package dag

// Graph represents a simple interface for representation
// of a directed graph.
//
// It is assumed that each node in the graph is uniquely
// identified with an incremental positive integer (i.e. 1, 2, 3...).
// A value of 0 for a node represents a sentinel error value.
//
// ResourceKey is the user's resource key: it can be any comparable type
// (string, a struct, a pointer, ...).
type Graph[ResourceKey comparable] interface {
	// return the total number of nodes in the graph.
	Count() int

	// return the node registered under the given ID, or nil if the ID is
	// out of range.
	At(id NodeID) Node[ResourceKey]

	// return a list of integers where each
	// represents a node that has an edge from node u.
	OutNeighbors(u NodeID) []NodeID

	// return the node IDs that provide the given k.
	// Several nodes may provide the same resource; all are returned, in
	// registration order. A nil result means nobody provides k yet.
	Provide(k ResourceKey) []NodeID
}

// NodeID uniquely identifies a node (one registered unit) inside a graph.
//
// NodeIDs are assigned by the graph at Add time, in registration order,
// starting from 1.
//
// The value 0 is reserved as the sentinel for "not found".
type NodeID int

// Node is the minimal registrable unit. It describes what the
// node needs (Requires) and what it provides (Provides).
type Node[ResourceKey comparable] interface {
	// return the resources this node depends on, i.e. the resources
	// other nodes must provide for this node to be usable
	Requires() []ResourceKey
	// return the resources this node produces, i.e. the resources
	// other nodes can depend on
	Provides() []ResourceKey
}
