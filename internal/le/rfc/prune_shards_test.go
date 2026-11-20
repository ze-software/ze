// Design: docs/architecture/core-design.md -- the rfc area, as one command
package rfc

import (
	"errors"
	"os"
	"slices"
	"testing"
)

// TestRemoveShardsToleratesAnAlreadyRemovedShard proves a shard that vanished
// between the listing and the removal is the outcome the removal wanted, not
// a failure of the whole render.
//
// Method: a temporary tree holds rfc9998's shard and not rfc9999's, as when
// another writer removed rfc9999's after the listing. removeShards over both
// stems answers no error, names only rfc9998 (this run deleted nothing for
// rfc9999), and leaves rfc9998's file gone.
func TestRemoveShardsToleratesAnAlreadyRemovedShard(t *testing.T) {
	tree := t.TempDir()
	if err := os.MkdirAll(treePath(tree, shardRelDir), 0o755); err != nil {
		t.Fatal(err)
	}
	present := treePath(tree, shardRel("rfc9998"))
	if err := os.WriteFile(present, []byte("# rfc9998\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeShards(tree, []string{"rfc9998", "rfc9999"})
	if err != nil {
		t.Fatalf("removeShards refused a shard already gone: %v", err)
	}
	if !slices.Equal(removed, []string{"rfc9998"}) {
		t.Fatalf("removed = %q, want exactly [rfc9998]", removed)
	}
	if _, statErr := os.Stat(present); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rfc9998's shard is still there after removal (stat: %v)", statErr)
	}
}
