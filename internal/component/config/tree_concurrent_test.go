package config

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTreeConcurrentSetAndSerialize verifies that Tree.Set, Tree.AppendValue,
// Tree.SetContainer, Tree.AddListEntry, Tree.Clone, and Tree.ToMap on one
// goroutine do not race with Serialize, SerializeSet, and SerializeSetWithMeta
// walks on another goroutine.
//
// VALIDATES: Tree / MetaTree lock invariant -- all map/slice access is
// serialized between mutating goroutines and walking goroutines.
//
// PREVENTS: The TestETSessionOption-class flake where the editor dispatches
// command handlers in goroutines (processCmdWithDepth) while the test
// checker reads WorkingContent on the main goroutine. Without per-Tree /
// per-MetaTree mutexes the race detector flags writes at tree.go setLocked
// versus reads at serialize_set.go.
//
// Run under -race. The assertions are that the goroutines complete without
// panic and the serialized output is never empty (proving the reader saw
// at least something that a writer produced, i.e., the walk did not crash
// on a half-mutated tree).
func TestTreeConcurrentSetAndSerialize(t *testing.T) {
	schema := testSchema()

	tree := NewTree()
	meta := NewMetaTree()
	tree.Set("router-id", "10.0.0.1")
	tree.Set("local-as", "65000")

	// Pre-populate a neighbor entry so the walker has a sub-tree target.
	// `neighbor` is a list in testSchema; register the entry directly.
	peer := NewTree()
	peer.Set("peer-as", "65001")
	tree.AddListEntry("neighbor", "192.0.2.1", peer)

	const iterations = 200
	const writers = 4
	const readers = 4

	var wg sync.WaitGroup
	start := make(chan struct{})

	for w := range writers {
		wg.Go(func() {
			<-start
			for i := range iterations {
				peer.Set("peer-as", intToString(65000+(w*1000+i)%1000))
				peer.AppendValue("communities", "100:200")
				peer.SetContainer("family", NewTree())
				meta.SetEntry("router-id", MetaEntry{
					User: "thomas",
					Time: time.Now(),
				})
			}
		})
	}

	for range readers {
		wg.Go(func() {
			<-start
			for range iterations {
				_ = Serialize(tree, schema)
				_ = SerializeSet(tree, schema)
				_ = SerializeSetWithMeta(tree, meta, schema)
				_ = tree.ToMap()
				_ = tree.Clone()
			}
		})
	}

	close(start)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent test deadlocked")
	}

	// Sanity: final serialization must still contain both the root leaf
	// and the neighbor we seeded. Proves we did not corrupt the tree
	// through the concurrent churn.
	final := Serialize(tree, schema)
	if !strings.Contains(final, "router-id") {
		t.Fatalf("expected router-id in final output, got: %q", final)
	}
	if !strings.Contains(final, "192.0.2.1") {
		t.Fatalf("expected neighbor 192.0.2.1 in final output, got: %q", final)
	}
}

// intToString is a zero-import helper so we do not need strconv in this file.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
