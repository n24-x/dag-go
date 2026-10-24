package dag

import (
	"fmt"
	"slices"
)

// DefaultGraph is the default Graph implementation.
type DefaultGraph[ResourceType comparable] struct {
	// nodes holds registered nodes; the slice index is NodeID-1 because
	// NodeIDs start at 1 (0 is the sentinel).
	nodes []Node[ResourceType]

	// providers maps a resource to the node IDs that provide it, in
	// registration order. A resource may be provided by several nodes.
	providers map[ResourceType][]NodeID

	// consumers maps a resource to the node IDs that require it, in
	// registration order. It is the reverse of providers, and lets Add find the
	// nodes that could close a cycle through a newly added node without
	// scanning the whole graph.
	consumers map[ResourceType][]NodeID

	// nextID is the NodeID to hand out on the next Add.
	// It MUST be initialized to 1: NodeIDs are 1-based, and 0 is reserved as
	// the sentinel for "not found". It is incremented on every Add and
	// decremented on rollback, so it never skips an ID.
	nextID NodeID

	// unverified records that AddUnchecked has run since the graph was last
	// known to be acyclic.
	//
	// Add's cycle check only looks at cycles through the node being added, which
	// is exact only while everything already in the graph is acyclic.
	// AddUnchecked breaks that premise on purpose, so the next Add falls back to
	// a whole-graph scan to re-establish it.
	unverified bool
}

// New returns an empty graph ready for use.
//
// The type parameter must be supplied explicitly since there are no arguments
// to infer it from: New[MyKey]().
func New[ResourceType comparable]() *DefaultGraph[ResourceType] {
	return &DefaultGraph[ResourceType]{
		providers: make(map[ResourceType][]NodeID),
		consumers: make(map[ResourceType][]NodeID),
		nextID:    1,
	}
}

// Add registers a node transactionally:
//
//  1. assign the next [NodeID], append the node, and index it under the
//     resources it provides and the ones it requires (caching the previous
//     entries for rollback)
//  2. check for a cycle
//  3. on failure, roll the node and both indexes back and report the
//     [CycleError] that the check returned
//
// Step 2 is incremental: adding a node to an acyclic graph can only close a
// cycle that runs through that node, because every edge a new node contributes
// is incident to it. So the check asks "can the new node reach a node that
// depends on it" rather than "is the whole graph still acyclic". That costs
// one map lookup per provided resource when nothing consumes what the new node
// provides - the common case for a graph built in dependency order - instead of
// a full traversal on every Add.
//
// If [DefaultGraph.AddUnchecked] ran since the last verified Add, the graph may
// already hold a cycle that the incremental check would walk past; Add then
// scans the whole graph once and clears the flag on success.
//
// The CycleError snapshots the cycle's Nodes at detection time, i.e. before
// the rollback: the node that closed the cycle is present in Nodes even
// though it is removed from the graph by the rollback.
//
// On success it returns the new node's ID and the total node count after the
// add. On failure it returns (0, count after rollback, err).
func (g *DefaultGraph[ResourceType]) Add(n Node[ResourceType]) (NodeID, int, error) {
	return g.add(n, false)
}

// AddUnchecked registers a node without running cycle detection. It assigns
// the ID and registers Provided and Required resources exactly like [Add].
//
// Use it only when you know the registration cannot close a cycle, or when you
// intend to verify the graph once at the end. Either way the graph is marked
// unverified: the next [DefaultGraph.Add] scans the whole graph before trusting
// its incremental check again, and [DefaultGraph.VerifyAcyclic] does that scan
// directly. [Resolve] keeps checking on its own and needs no flag.
func (g *DefaultGraph[ResourceType]) AddUnchecked(n Node[ResourceType]) (NodeID, int) {
	id, count, _ := g.add(n, true)
	return id, count
}

// add implements both Add (unchecked=false) and AddUnchecked (unchecked=true).
func (g *DefaultGraph[ResourceType]) add(n Node[ResourceType], unchecked bool) (NodeID, int, error) {
	id := g.nextID
	g.nextID++
	g.nodes = append(g.nodes, n)

	// Cache previous index entries so we can restore them on rollback. One entry
	// per distinct resource: the same resource can show up in both indexes (a
	// node may provide what another requires, and even what it requires
	// itself), and re-reading it twice would lose the first snapshot.
	type oldEntry struct {
		resource  ResourceType
		providers []NodeID
		consumers []NodeID
	}
	olds := make([]oldEntry, 0, 4)
	seen := make(map[ResourceType]struct{}, 4)

	remember := func(typ ResourceType) {
		if _, ok := seen[typ]; ok {
			return
		}
		seen[typ] = struct{}{}
		olds = append(olds, oldEntry{
			resource:  typ,
			providers: g.providers[typ],
			consumers: g.consumers[typ],
		})
	}

	for _, typ := range n.Provides() {
		remember(typ)
		g.providers[typ] = append(g.providers[typ], id)
	}
	for _, typ := range n.Requires() {
		remember(typ)
		g.consumers[typ] = append(g.consumers[typ], id)
	}

	rollback := func() {
		g.nodes = g.nodes[:len(g.nodes)-1]
		for _, o := range olds {
			g.providers[o.resource] = o.providers
			g.consumers[o.resource] = o.consumers
		}
		g.nextID--
	}

	if unchecked {
		g.unverified = true
		return id, len(g.nodes), nil
	}

	if g.unverified {
		// The premise of the incremental check may not hold, so only a
		// whole-graph scan can re-establish it. It covers the node registered
		// above as well.
		if ok, cycle := IsAcyclic(g); !ok {
			rollback()
			return 0, len(g.nodes), cycle
		}
		g.unverified = false
		return id, len(g.nodes), nil
	}

	if cycle := g.cycleThrough(id); cycle != nil {
		rollback()
		return 0, len(g.nodes), cycle
	}

	return id, len(g.nodes), nil
}

// VerifyAcyclic scans the whole graph and reports a cycle if there is one.
//
// It is the check that [DefaultGraph.AddUnchecked] defers. On success the graph
// is marked verified, so the incremental check in [DefaultGraph.Add] is trusted
// again.
func (g *DefaultGraph[ResourceType]) VerifyAcyclic() error {
	if ok, cycle := IsAcyclic(g); !ok {
		return cycle
	}
	g.unverified = false
	return nil
}

// cycleThrough reports a cycle that contains id, or nil if there is none.
//
// It assumes the rest of the graph is acyclic, which is what makes the search
// sound: a cycle containing id must be id -> ... -> x -> id, so it is enough to
// walk forward from id and look for a node that depends on it. Those nodes come
// from the consumers index rather than from a scan, so a node that nothing
// consumes costs one map lookup per resource it provides.
//
// The node must already be indexed: Add calls this after registering it, so
// g.At(id) is still in the graph and can be snapshotted into the CycleError.
func (g *DefaultGraph[ResourceType]) cycleThrough(id NodeID) *CycleError[ResourceType] {
	// Nodes that would close a cycle through id: the ones that require
	// something id provides. id itself is left out because it can only close a
	// cycle with a self edge, which the walk below finds as v == id.
	dependents := make(map[NodeID]struct{}, 2)
	for _, typ := range g.nodes[id-1].Provides() {
		for _, c := range g.consumers[typ] {
			if c != id {
				dependents[c] = struct{}{}
			}
		}
	}
	if len(dependents) == 0 {
		// Nothing depends on id, so the only remaining possibility is a node
		// that requires what it provides.
		if slices.Contains(g.OutNeighbors(id), id) {
			return g.newCycleError([]NodeID{id, id})
		}
		return nil
	}

	visited := make([]bool, len(g.nodes)+1)
	var path []NodeID

	var walk func(u NodeID) []NodeID
	walk = func(u NodeID) []NodeID {
		visited[u] = true
		path = append(path, u)

		for _, v := range g.OutNeighbors(u) {
			switch {
			case v == id:
				// Back at the start: id requires something it provides.
				return append(slices.Clone(path), id)
			case isDependent(dependents, v):
				return append(slices.Clone(path), v, id)
			case !visited[v]:
				if cycle := walk(v); cycle != nil {
					return cycle
				}
			}
		}

		path = path[:len(path)-1]
		return nil
	}

	if cycle := walk(id); cycle != nil {
		return g.newCycleError(cycle)
	}
	return nil
}

// isDependent reports whether id is a key of set.
func isDependent(set map[NodeID]struct{}, id NodeID) bool {
	_, ok := set[id]
	return ok
}

// newCycleError snapshots the nodes along cycle.
func (g *DefaultGraph[ResourceType]) newCycleError(cycle []NodeID) *CycleError[ResourceType] {
	nodes := make([]Node[ResourceType], len(cycle))
	for i, id := range cycle {
		nodes[i] = g.At(id)
	}
	return &CycleError[ResourceType]{Cycle: cycle, Nodes: nodes}
}

// Count returns the total number of nodes in the graph.
func (g *DefaultGraph[ResourceType]) Count() int { return len(g.nodes) }

// OutNeighbors returns the node IDs that node u depends on.
func (g *DefaultGraph[ResourceType]) OutNeighbors(u NodeID) []NodeID {
	if u <= 0 || int(u) > len(g.nodes) {
		return nil
	}
	var deps []NodeID
	for _, typ := range g.nodes[u-1].Requires() {
		deps = append(deps, g.providers[typ]...)
	}
	return deps
}

// Provide returns the node IDs that provide the given resource, in
// registration order.
func (g *DefaultGraph[ResourceType]) Provide(typ ResourceType) []NodeID {
	return g.providers[typ]
}

// At returns the node registered under the given ID, or nil if the ID is out
// of range.
func (g *DefaultGraph[ResourceType]) At(id NodeID) Node[ResourceType] {
	if id <= 0 || int(id) > len(g.nodes) {
		return nil
	}
	return g.nodes[id-1]
}

// compile-time check that DefaultGraph satisfies Graph for any comparable ResourceType.
var _ Graph[string] = (*DefaultGraph[string])(nil)

// String renders a compact description of the graph for debugging.
func (g *DefaultGraph[ResourceType]) String() string {
	return fmt.Sprintf("dag.DefaultGraph{nodes=%d, resources=%d}", g.Count(), len(g.providers))
}
