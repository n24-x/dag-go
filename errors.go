package dag

import (
	"fmt"
	"strings"
)

// CycleError reports that the graph contains a cycle, along with the cyclic
// path of NodeIDs.
type CycleError struct {
	// Cycle is the path of NodeIDs forming the cycle, e.g. [1 2 3 1].
	Cycle []NodeID
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dag: cycle detected in dependency graph: %v", e.Cycle)
}

// MissingResourcesError reports resources that were requested while resolving
// a target but have no provider registered in the graph. It is the generic
// counterpart of dig's errMissingDependencies, which dig raises at Invoke time
// rather than at Provide time — here it surfaces at Resolve time.
type MissingResourcesError struct {
	// Resources lists the resources that nothing provides, in the order they
	// were first encountered during the traversal.
	Resources []ResourceID
}

func (e *MissingResourcesError) Error() string {
	parts := make([]string, len(e.Resources))
	for i, r := range e.Resources {
		parts[i] = fmt.Sprintf("%v", r)
	}
	return fmt.Sprintf("dag: missing dependencies: %s", strings.Join(parts, ", "))
}
