package dag

import (
	"errors"
	"slices"
	"testing"
)

// Test resources (mirroring designv3.md's user journey).
const (
	RConfig ResourceID = iota + 1
	RDB
	RServer
	RCache
	RAPI
)

func ids(ns ...NodeID) []NodeID            { return ns }
func resIDs(rs ...ResourceID) []ResourceID { return rs }

// ---- user-defined node types (any struct implementing Node) ----

type loadConfig struct{ path string }

func (loadConfig) Requires() []ResourceID { return nil }
func (loadConfig) Provides() []ResourceID { return resIDs(RConfig) }

type connectDB struct{ dsn string }

func (connectDB) Requires() []ResourceID { return resIDs(RConfig) }
func (connectDB) Provides() []ResourceID { return resIDs(RDB) }

type startServer struct{ port int }

func (startServer) Requires() []ResourceID { return resIDs(RDB) }
func (startServer) Provides() []ResourceID { return resIDs(RServer) }

// needProvide is a generic test node: requires `needs`, provides `provs`.
type needProvide struct {
	needs []ResourceID
	provs []ResourceID
}

func (n needProvide) Requires() []ResourceID { return n.needs }
func (n needProvide) Provides() []ResourceID { return n.provs }

func mustAdd(t *testing.T, g *DefaultGraph, n Node) NodeID {
	t.Helper()
	id, count, err := g.Add(n)
	if err != nil {
		t.Fatalf("Add(%T): %v", n, err)
	}
	if count != int(id) {
		t.Fatalf("Add(%T): count=%d but id=%d (IDs must start at 1, no gaps)", n, count, id)
	}
	return id
}

// ---- tests ----

// TestResolveChain is the user journey from designv3.md §6.
func TestResolveChain(t *testing.T) {
	g := New()

	id1, n1, err := g.Add(loadConfig{path: "app.yaml"})
	if err != nil || id1 != 1 || n1 != 1 {
		t.Fatalf("Add(loadConfig) = (%d, %d, %v), want (1, 1, nil)", id1, n1, err)
	}
	id2, n2, err := g.Add(connectDB{dsn: "postgres://db"})
	if err != nil || id2 != 2 || n2 != 2 {
		t.Fatalf("Add(connectDB) = (%d, %d, %v), want (2, 2, nil)", id2, n2, err)
	}
	id3, n3, err := g.Add(startServer{port: 8080})
	if err != nil || id3 != 3 || n3 != 3 {
		t.Fatalf("Add(startServer) = (%d, %d, %v), want (3, 3, nil)", id3, n3, err)
	}

	if g.Count() != 3 {
		t.Fatalf("Count() = %d, want 3", g.Count())
	}

	// Edges are derived on the fly: join Requires against the index.
	if deps := g.OutNeighbors(3); !slices.Equal(deps, ids(2)) {
		t.Fatalf("OutNeighbors(3) = %v, want [2]", deps)
	}
	if deps := g.OutNeighbors(2); !slices.Equal(deps, ids(1)) {
		t.Fatalf("OutNeighbors(2) = %v, want [1]", deps)
	}
	if deps := g.OutNeighbors(1); len(deps) != 0 {
		t.Fatalf("OutNeighbors(1) = %v, want empty", deps)
	}

	order, err := Resolve(g, RServer)
	if err != nil {
		t.Fatalf("Resolve(RServer): %v", err)
	}
	if !slices.Equal(order, ids(1, 2, 3)) {
		t.Fatalf("Resolve(RServer) = %v, want [1 2 3]", order)
	}
}

// TestResolveNode: pointing at a node directly yields the same order as
// resolving the resource it provides.
func TestResolveNode(t *testing.T) {
	g := New()
	mustAdd(t, g, loadConfig{})
	mustAdd(t, g, connectDB{})
	mustAdd(t, g, startServer{})

	byResource, err := Resolve(g, RServer)
	if err != nil {
		t.Fatalf("Resolve(RServer): %v", err)
	}
	byNode, err := ResolveNode(g, 3)
	if err != nil {
		t.Fatalf("ResolveNode(3): %v", err)
	}
	if !slices.Equal(byNode, byResource) {
		t.Fatalf("ResolveNode(3) = %v, want %v", byNode, byResource)
	}

	// Out-of-range node ID is an error, not a crash.
	if _, err := ResolveNode(g, 99); err == nil {
		t.Fatal("ResolveNode(99) should fail")
	}
}

// TestAddDanglingRequireIsLegal mirrors dig: missing dependencies are reported
// at Resolve time, not Add time.
func TestAddDanglingRequireIsLegal(t *testing.T) {
	g := New()
	mustAdd(t, g, loadConfig{})
	// startServer requires RDB, which nobody provides yet.
	if _, _, err := g.Add(startServer{}); err != nil {
		t.Fatalf("Add with dangling requires should be legal: %v", err)
	}

	if _, err := Resolve(g, RServer); err == nil {
		t.Fatal("Resolve(RServer) should report the missing RDB")
	}
}

// TestAddCycleRollback: adding a node that closes a cycle fails and leaves the
// graph untouched (dig: Snapshot/Rollback semantics).
func TestAddCycleRollback(t *testing.T) {
	g := New()
	mustAdd(t, g, loadConfig{})                                                // 1: provides RConfig
	mustAdd(t, g, needProvide{needs: resIDs(RConfig), provs: resIDs(RServer)}) // 2
	if g.Count() != 2 {
		t.Fatalf("Count() = %d, want 2", g.Count())
	}

	// Node 3 provides RConfig and requires RServer -> closes 2->3->2.
	id, count, err := g.Add(needProvide{needs: resIDs(RServer), provs: resIDs(RConfig)})
	if err == nil {
		t.Fatalf("Add should have failed, got id=%d count=%d", id, count)
	}
	var ce *CycleError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CycleError, got %T: %v", err, err)
	}
	if count != 2 || g.Count() != 2 {
		t.Fatalf("after rollback count=%d Count()=%d, want 2/2 (graph unchanged)", count, g.Count())
	}
	if got := g.Provide(RConfig); !slices.Equal(got, ids(1)) {
		t.Fatalf("Provide(RConfig) = %v, want [1] (index rolled back)", got)
	}
	if ok, _ := IsAcyclic(g); !ok {
		t.Fatal("graph must be acyclic after rollback")
	}

	// The same node through AddUnchecked succeeds, and Resolve then catches
	// the cycle at its entry check.
	id, count = g.AddUnchecked(needProvide{needs: resIDs(RServer), provs: resIDs(RConfig)})
	if id != 3 || count != 3 {
		t.Fatalf("AddUnchecked = (%d, %d), want (3, 3)", id, count)
	}
	if _, err := Resolve(g, RServer); err == nil {
		t.Fatal("Resolve on the cyclic graph should fail")
	} else {
		var ce2 *CycleError
		if !errors.As(err, &ce2) {
			t.Fatalf("expected *CycleError from Resolve, got %T: %v", err, err)
		}
	}
}

// TestResolveMissing: only reachable missing resources are reported.
func TestResolveMissing(t *testing.T) {
	g := New()
	// startServer requires RDB; nobody provides RDB, but RServer's provider
	// chain makes RDB reachable.
	mustAdd(t, g, startServer{})

	_, err := Resolve(g, RServer)
	var missing *MissingResourcesError
	if !errors.As(err, &missing) {
		t.Fatalf("expected *MissingResourcesError, got %T: %v", err, err)
	}
	if !slices.Equal(missing.Resources, resIDs(RDB)) {
		t.Fatalf("missing = %v, want [RDB]", missing.Resources)
	}

	// Resolving a resource nobody provides reports just that resource.
	_, err = Resolve(g, RConfig)
	if !errors.As(err, &missing) {
		t.Fatalf("expected *MissingResourcesError, got %T: %v", err, err)
	}
	if !slices.Equal(missing.Resources, resIDs(RConfig)) {
		t.Fatalf("missing = %v, want [RConfig]", missing.Resources)
	}
}

// TestMultipleProviders: every provider of a resource is a candidate edge
// (value-group semantics); picking one winner is an upper-layer policy.
func TestMultipleProviders(t *testing.T) {
	g := New()
	mustAdd(t, g, needProvide{provs: resIDs(RConfig)}) // 1
	mustAdd(t, g, needProvide{provs: resIDs(RConfig)}) // 2, second provider
	mustAdd(t, g, connectDB{})                         // 3, requires RConfig

	if deps := g.OutNeighbors(3); !slices.Equal(deps, ids(1, 2)) {
		t.Fatalf("OutNeighbors(3) = %v, want [1 2]", deps)
	}

	order, err := Resolve(g, RDB)
	if err != nil {
		t.Fatalf("Resolve(RDB): %v", err)
	}
	if !slices.Equal(order, ids(1, 2, 3)) {
		t.Fatalf("Resolve(RDB) = %v, want [1 2 3]", order)
	}
}

// TestResolveMemoizes: a diamond dependency emits the shared node exactly once
// (dig's "called" memoization).
func TestResolveMemoizes(t *testing.T) {
	g := New()
	mustAdd(t, g, needProvide{provs: resIDs(RConfig)})                           // 1
	mustAdd(t, g, needProvide{needs: resIDs(RConfig), provs: resIDs(RDB)})       // 2
	mustAdd(t, g, needProvide{needs: resIDs(RConfig), provs: resIDs(RServer)})   // 3
	mustAdd(t, g, needProvide{needs: resIDs(RDB, RServer), provs: resIDs(RAPI)}) // 4

	order, err := Resolve(g, RAPI)
	if err != nil {
		t.Fatalf("Resolve(RAPI): %v", err)
	}
	if !slices.Equal(order, ids(1, 2, 3, 4)) {
		t.Fatalf("Resolve(RAPI) = %v, want [1 2 3 4]", order)
	}
}

// TestIsAcyclicDirect: the primitive on a small cycle reports its path. Add
// rejects cycles, so build the cyclic graph through AddUnchecked.
func TestIsAcyclicDirect(t *testing.T) {
	g := New()
	if _, _, err := g.Add(needProvide{provs: resIDs(RConfig)}); err != nil {
		t.Fatal(err)
	}
	g.AddUnchecked(needProvide{needs: resIDs(RConfig), provs: resIDs(RServer)}) // 2
	g.AddUnchecked(needProvide{needs: resIDs(RServer), provs: resIDs(RConfig)}) // 3 => cycle 2->3->2

	ok, cycle := IsAcyclic(g)
	if ok {
		t.Fatal("expected a cycle")
	}
	if len(cycle) < 3 {
		t.Fatalf("cycle path = %v, want >= 3 entries", cycle)
	}
	t.Logf("cycle path: %v", cycle)
}
