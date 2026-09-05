package dag

// DFS node colors used by the three-color cycle detection below.
const (
	white = iota // node has not been visited
	gray         // node is on the current DFS recursion stack
	black        // node fully processed; no cycle through it
)

// IsAcyclic uses DFS with three-color marking to detect cycles in a graph
// represented by the Graph interface. Node IDs start at 1; 0 is the sentinel
// "not found" and is never a real node.
//
// If the graph is acyclic it returns (true, nil). Otherwise it returns
// (false, cycle), where cycle is the path of NodeIDs forming the cycle,
// e.g. [1 2 3 1] for 1 -> 2 -> 3 -> 1.
func IsAcyclic(g Graph) (bool, []NodeID) {
	// state is indexed by NodeID; index 0 (the sentinel) is unused.
	state := make([]int, g.Count()+1)

	for u := NodeID(1); u <= NodeID(g.Count()); u++ {
		// Start a DFS from each unvisited node to cover disconnected graphs.
		if state[u] == white {
			cycle := detectCycle(g, u, state, nil /* cycle path */)
			if len(cycle) > 0 {
				return false, cycle
			}
		}
	}

	return true, nil
}

// detectCycle performs a DFS starting from u and returns the first cycle
// found, or nil if no cycle is reachable from u.
func detectCycle(g Graph, u NodeID, state []int, path []NodeID) []NodeID {
	// Mark the current node as being explored and add it to the current path.
	state[u] = gray
	path = append(path, u)

	for _, v := range g.OutNeighbors(u) {
		switch state[v] {
		case white:
			// Continue the DFS from an unvisited neighbor.
			if cycle := detectCycle(g, v, state, path); len(cycle) > 0 {
				return cycle
			}

		case gray:
			// v is still on the recursion stack: a back edge closes a cycle
			// starting at v. Prune the path down to the cyclic nodes and
			// close the cycle by appending v.
			for i := len(path) - 1; i >= 0; i-- {
				if path[i] == v {
					cycleLen := len(path) - i
					cycle := make([]NodeID, cycleLen+1)
					copy(cycle, path[i:])
					cycle[cycleLen] = v
					return cycle
				}
			}
		}
		// Black nodes have already been fully processed.
	}

	// Mark the current node as fully processed.
	state[u] = black
	return nil
}
