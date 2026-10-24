package dag

import (
	"fmt"
)

// Resolve returns all build paths for target at once: building the nodes in
// this order, one after another, guarantees that target gets a provider.
//
// It returns *MissingProviderError if a needed resource has no provider, and
// *CycleError if the graph contains a cycle.
func Resolve[ResourceType comparable](g Graph[ResourceType], target ResourceType) ([]NodeID, error) {
	roots := g.Provide(target)
	if len(roots) == 0 {
		return nil, &MissingProviderError[ResourceType]{Resources: []ResourceType{target}}
	}
	return resolveFromRoots(g, roots)
}

// ResolveNode is like Resolve but takes a node ID instead of a resource, and
// returns build path for that node.
func ResolveNode[ResourceType comparable](g Graph[ResourceType], target NodeID) ([]NodeID, error) {
	return resolveFromRoots(g, []NodeID{target})
}

// resolveFromRoots returns the merge build path of the given roots: a
// depth-first post-order walk in which each node is emitted once, after all of
// its dependencies.
func resolveFromRoots[ResourceType comparable](g Graph[ResourceType], roots []NodeID) ([]NodeID, error) {
	if ok, cycle := IsAcyclic(g); !ok {
		return nil, cycle
	}

	state := make([]nodeColor, g.Count()+1) // indexed by NodeID; 0 unused

	var (
		order   []NodeID
		missing []ResourceType
		seen    = make(map[ResourceType]struct{})
	)

	var visit func(u NodeID) error
	visit = func(u NodeID) error {
		if u <= 0 || int(u) > g.Count() {
			return fmt.Errorf("dag: node %d out of range (graph has %d nodes)", u, g.Count())
		}
		switch state[u] {
		case black:
			return nil
		case gray: // still on the current path: a back edge
			if ok, cycle := IsAcyclic(g); !ok {
				return fmt.Errorf("dag: graph became cyclic during resolve: %w", cycle)
			}
			return fmt.Errorf("dag: graph changes during resolve introduced a cycle")
		case white:
			// Not visited yet: fall through to the walk below.
		}

		state[u] = gray

		// Record which resources are unavailable.
		if node := g.At(u); node != nil {
			for _, typ := range node.Requires() {
				if len(g.Provide(typ)) == 0 {
					if _, ok := seen[typ]; !ok {
						seen[typ] = struct{}{}
						missing = append(missing, typ)
					}
				}
			}
		}

		for _, v := range g.OutNeighbors(u) {
			if err := visit(v); err != nil {
				return err
			}
		}
		order = append(order, u)
		state[u] = black
		return nil
	}

	for _, root := range roots {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	if len(missing) > 0 {
		return nil, &MissingProviderError[ResourceType]{Resources: missing}
	}
	return order, nil
}
