package dag

import "fmt"

// DefaultGraph is the default Graph implementation.
type DefaultGraph[ResourceKey comparable] struct {
	// nodes holds registered nodes; the slice index is NodeID-1 because
	// NodeIDs start at 1 (0 is the sentinel).
	nodes []Node[ResourceKey]

	// providers maps a resource to the node IDs that provide it, in
	// registration order. A resource may be provided by several nodes.
	providers map[ResourceKey][]NodeID

	// nextID is the NodeID to hand out on the next Add.
	// It MUST be initialized to 1: NodeIDs are 1-based, and 0 is reserved as
	// the sentinel for "not found". It is incremented on every Add and
	// decremented on rollback, so it never skips an ID.
	nextID NodeID
}

// New returns an empty graph ready for use.
//
// The type parameter must be supplied explicitly since there are no arguments
// to infer it from: New[MyKey]().
func New[ResourceKey comparable]() *DefaultGraph[ResourceKey] {
	return &DefaultGraph[ResourceKey]{providers: make(map[ResourceKey][]NodeID), nextID: 1}
}

// Add registers a node transactionally:
//
//  1. assign the next [NodeID] and append the node
//  2. register each Provided resource in the index (caching previous entries
//     for rollback)
//  3. run [IsAcyclic] over the whole graph
//  4. on failure, roll the node and the index back and report the
//     [CycleError] that IsAcyclic returned
//
// The CycleError snapshots the cycle's Nodes at detection time, i.e. before
// the rollback: the node that closed the cycle is present in Nodes even
// though it is removed from the graph by the rollback.
//
// On success it returns the new node's ID and the total node count after the
// add. On failure it returns (0, count after rollback, err).
func (g *DefaultGraph[ResourceKey]) Add(n Node[ResourceKey]) (NodeID, int, error) {
	return g.add(n, false)
}

// AddUnchecked registers a node without running cycle detection. It assigns
// the ID and registers Provided resources exactly like [Add]. Use it
// only when you know the registration cannot close a cycle — Resolve checks
// for cycles before traversing, so an AddUnchecked graph is still caught
// there.
func (g *DefaultGraph[ResourceKey]) AddUnchecked(n Node[ResourceKey]) (NodeID, int) {
	id, count, _ := g.add(n, true)
	return id, count
}

// add implements both Add (check=true) and AddUnchecked (check=false).
func (g *DefaultGraph[ResourceKey]) add(n Node[ResourceKey], unchecked bool) (NodeID, int, error) {
	id := g.nextID
	g.nextID++
	g.nodes = append(g.nodes, n)

	// Cache previous index entries so we can restore them on rollback
	type oldEntry struct {
		k    ResourceKey
		prev []NodeID
	}
	var olds []oldEntry
	for _, k := range n.Provides() {
		olds = append(olds, oldEntry{k: k, prev: g.providers[k]})
		g.providers[k] = append(g.providers[k], id)
	}

	if unchecked {
		return id, len(g.nodes), nil
	}

	if ok, cycle := IsAcyclic(g); !ok {
		// Rollback: drop the node and restore the index
		g.nodes = g.nodes[:len(g.nodes)-1]
		for _, o := range olds {
			g.providers[o.k] = o.prev
		}
		g.nextID--
		return 0, len(g.nodes), cycle
	}

	return id, len(g.nodes), nil
}

// Count returns the total number of nodes in the graph.
func (g *DefaultGraph[ResourceKey]) Count() int { return len(g.nodes) }

// OutNeighbors returns the node IDs that node u depends on.
func (g *DefaultGraph[ResourceKey]) OutNeighbors(u NodeID) []NodeID {
	if u <= 0 || int(u) > len(g.nodes) {
		return nil
	}
	var deps []NodeID
	for _, k := range g.nodes[u-1].Requires() {
		deps = append(deps, g.providers[k]...)
	}
	return deps
}

// Provide returns the node IDs that provide the given resource, in
// registration order.
func (g *DefaultGraph[ResourceKey]) Provide(k ResourceKey) []NodeID {
	return g.providers[k]
}

// At returns the node registered under the given ID, or nil if the ID is out
// of range.
func (g *DefaultGraph[ResourceKey]) At(id NodeID) Node[ResourceKey] {
	if id <= 0 || int(id) > len(g.nodes) {
		return nil
	}
	return g.nodes[id-1]
}

// compile-time check that DefaultGraph satisfies Graph for any comparable ResourceKey.
var _ Graph[string] = (*DefaultGraph[string])(nil)

// String renders a compact description of the graph for debugging.
func (g *DefaultGraph[ResourceKey]) String() string {
	return fmt.Sprintf("dag.DefaultGraph{nodes=%d, resources=%d}", g.Count(), len(g.providers))
}
