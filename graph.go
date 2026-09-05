package dag

// NodeID uniquely identifies a node (one registered unit) inside a graph.
//
// NodeIDs are assigned by the graph at Add time, in registration order,
// starting from 1.
//
// The value 0 is reserved as the sentinel for "not found".
type NodeID int

// ResourceID uniquely identifies a resource: something that can be produced by
// nodes and required by other nodes.
type ResourceID int

// Node is the minimal registrable unit. It describes what the
// node needs (Requires) and what it provides (Provides).
type Node interface {
	// Requires returns the resources this node needs in order to run
	// (dig: constructor parameters / dig.In fields).
	Requires() []ResourceID
	// Provides returns the resources this node makes available after it runs
	// (dig: constructor results / dig.Out fields).
	Provides() []ResourceID
}

// Graph is the query contract visible to algorithms (IsAcyclic, Resolve,
// ResolveNode, ...). It deliberately exposes no notion of edges: edges are
// derived — the providers of a node's Requires — rather than stored.
//
// The contract is intentionally split from Node so that algorithms run on
// NodeIDs and never touch Node implementations. This mirrors dig's layering:
// internal/graph algorithms know only int node indices, and the dig-side
// graphHolder translates between the container's key space and node indices.
type Graph interface {
	// Count returns the total number of nodes in the graph.
	Count() int

	// At returns the node registered under the given ID, or nil if the ID is
	// out of range. It lets algorithms read a node's Requires back when they
	// need to report precisely which resources are missing (see Resolve).
	At(id NodeID) Node

	// OutNeighbors returns the node IDs that node u depends on — the
	// providers of every resource u requires, computed on the fly.
	//
	// The empty result means u has no dependencies (leaf) or that some of its
	// Requires have no provider yet; missing resources are reported precisely
	// by Resolve, which collects them into a MissingResourcesError.
	OutNeighbors(u NodeID) []NodeID

	// Provide returns the node IDs that provide the given resource.
	// Several nodes may provide the same resource; all are returned, in
	// registration order. A nil result means nobody provides r yet.
	Provide(r ResourceID) []NodeID
}
