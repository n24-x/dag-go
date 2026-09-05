// Package dag is a declarative dependency-graph engine abstracted from the
// graph machinery of uber-go/dig. The model's first-class citizen is the
// resource: something nodes provide and other nodes require. A node is just a
// registration unit that says what it requires and
// what it provides.
package dag
