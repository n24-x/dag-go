package dag

import (
	"fmt"
)

// Resolve returns the order of nodes which needed to build the given target resource.
func Resolve(g Graph, target ResourceID) ([]NodeID, error) {
	roots := g.Provide(target)
	if len(roots) == 0 {
		return nil, &MissingResourcesError{Resources: []ResourceID{target}}
	}
	return resolveFromRoots(g, roots)
}

// Resolve returns the order of nodes which needed to reach the given node.
func ResolveNode(g Graph, target NodeID) ([]NodeID, error) {
	return resolveFromRoots(g, []NodeID{target})
}

// resolveFromRoots returns the order of nodes needed to reach the given
// roots: a depth-first post-order walk in which each node is emitted once,
// after all of its dependencies.
func resolveFromRoots(g Graph, roots []NodeID) ([]NodeID, error) {
	if ok, cycle := IsAcyclic(g); !ok {
		return nil, &CycleError{Cycle: cycle}
	}

	const (
		unvisited = iota
		visiting  // on the current DFS stack
		done
	)
	state := make([]uint8, g.Count()+1) // indexed by NodeID; 0 unused

	var (
		order   []NodeID
		missing []ResourceID
		seen    = make(map[ResourceID]struct{})
	)

	var visit func(u NodeID) error
	visit = func(u NodeID) error {
		if u <= 0 || int(u) > g.Count() {
			return fmt.Errorf("dag: node %d out of range (graph has %d nodes)", u, g.Count())
		}
		switch state[u] {
		case done:
			return nil
		case visiting:
			// The entry check above already rejects cycles, so reaching a
			// visiting node means the graph changed mid-traversal or a custom
			// Graph implementation is inconsistent. Guard without a path.
			return &CycleError{}
		}

		state[u] = visiting

		// Collect precisely which of u's Requires have no provider yet.
		if node := g.At(u); node != nil {
			for _, r := range node.Requires() {
				if len(g.Provide(r)) == 0 {
					if _, ok := seen[r]; !ok {
						seen[r] = struct{}{}
						missing = append(missing, r)
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
		state[u] = done
		return nil
	}

	for _, root := range roots {
		if err := visit(root); err != nil {
			return nil, err
		}
	}
	if len(missing) > 0 {
		return nil, &MissingResourcesError{Resources: missing}
	}
	return order, nil
}
