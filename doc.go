// Package dag is a declarative dependency-graph engine abstracted from the
// graph machinery of uber-go/dig. The model's first-class citizen is the
// resource: something nodes provide and other nodes require. A node is just a
// registration unit that says what it requires and
// what it provides.
//
// Resource keys (the ResourceKey type parameter) are user-chosen comparable
// values — strings, structs, pointers, whatever identifies a resource in the
// user's world. Providers and consumers match on exact equality of
// ResourceKey, so there is no
// separate resource-ID space to translate into and no collision handling.
package dag
