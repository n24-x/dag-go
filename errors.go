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
type CycleError[ResourceKey comparable] struct {
	// Cycle is the path of NodeIDs forming the cycle, e.g. [2 3 2].
	Cycle []NodeID

	// Nodes runs parallel to Cycle: Nodes[i] is the node under Cycle[i] as
	// snapshotted at detection time (g.At(Cycle[i])).
	//
	// The snapshot matters for Add: the node that closed the cycle is rolled
	// back out of the graph right after detection, so it is no longer
	// reachable through At once Add returns. Nodes keeps it available.
	Nodes []Node[ResourceKey]
}

func (e *CycleError[ResourceKey]) Error() string {
	return fmt.Sprintf("dag: cycle detected in dependency graph: %v", e.Cycle)
}

// MissingResourcesError reports resources that were requested while resolving
// a target but have no provider registered in the graph. It is the generic
// counterpart of dig's errMissingDependencies, which dig raises at Invoke time
// rather than at Provide time — here it surfaces at Resolve time.
type MissingResourcesError[ResourceKey comparable] struct {
	// Resources lists the resources that nothing provides, in the order they
	// were first encountered during the traversal.
	Resources []ResourceKey
}

func (e *MissingResourcesError[ResourceKey]) Error() string {
	parts := make([]string, len(e.Resources))
	for i, r := range e.Resources {
		parts[i] = fmt.Sprintf("%v", r)
	}
	return fmt.Sprintf("dag: missing dependencies: %s", strings.Join(parts, ", "))
}
