package dag

import (
	"fmt"
	"strings"
)

// CycleError reports that the graph contains a cycle, along with the cyclic
// path of NodeIDs and the nodes on it.
//
// CycleError is built by IsAcyclic and reused by Add / Resolve / ResolveNode,
// so Cycle and Nodes always stay parallel.
type CycleError[ResourceType comparable] struct {
	// Cycle is the path of NodeIDs forming the cycle, e.g. [2 3 2].
	Cycle []NodeID

	// Nodes runs parallel to Cycle: Nodes[i] is the node under Cycle[i] as
	// snapshotted at detection time (g.At(Cycle[i])).
	//
	// The snapshot matters for Add: the node that closed the cycle is rolled
	// back out of the graph right after detection, so it is no longer
	// reachable through At once Add returns. Nodes keeps it available.
	Nodes []Node[ResourceType]
}

// TODO: build a human-readable message instead of printing raw NodeIDs. The
// indexes alone are hard to debug; the final form should describe the cycle
// with resources or node summaries, since Nodes is already available here.
func (e *CycleError[ResourceType]) Error() string {
	var msg []string
	for _, node := range e.Nodes {
		msg = append(msg, node.String())
	}
	return fmt.Sprintf("dag: cycle detected in graph: %s", strings.Join(msg, " -> "))

}

// MissingProviderError reports resources that no node provides. Resources are
// collected across all possible build paths, and Error reports every one of
// them.
type MissingProviderError[ResourceType comparable] struct {
	// Resources lists the resources whose providers are missing, in the order
	// they were first encountered during the traversal.
	Resources []ResourceType
}

func (e *MissingProviderError[ResourceType]) Error() string {
	parts := make([]string, len(e.Resources))
	for i, r := range e.Resources {
		parts[i] = fmt.Sprintf("%v", r)
	}
	return fmt.Sprintf("dag: dependencies: %s missing provider", strings.Join(parts, ", "))
}
