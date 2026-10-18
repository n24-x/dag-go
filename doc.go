// Package dag is a declarative dependency-graph engine abstracted from the
// graph machinery of uber-go/dig. The model's first-class citizen is the
// resource: something nodes provide and other nodes require. A node is just a
// registration unit that says what it requires and
// what it provides.
//
// Resource types (the ResourceType type parameter) are user-chosen comparable
// values — strings, structs, pointers, whatever identifies a resource in the
// user's world. Providers and consumers match on exact equality of
// ResourceType, so there is no
// separate resource-ID space to translate into and no collision handling.
//
// ResourceType should implement [fmt.Stringer], because Error methods report
// resources in error messages. The type parameter is only constrained to
// comparable, so a type without String renders through its default %v form.
package dag
