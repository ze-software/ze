package irr

import (
	"context"
	"sync/atomic"
	"testing"
)

// VALIDATES: ClearCache empties one client and counts exactly the entries it
// held, and leaves another client alone; method: two clients cache AS-TEST's
// members and prefixes, one is cleared, then looked up again.
// PREVENTS: clear irr cache all counting a client its owner does not use.
func TestClearCache_ExactCount(t *testing.T) {
	var hits atomic.Int32
	addr, cleanup := countingIRRServer(t, &hits, handleASSetQuery)
	defer cleanup()
	first := NewIRR(addr)
	second := NewIRR(addr)
	ctx := context.Background()

	for _, client := range []*IRR{first, second} {
		if _, err := client.ResolveASSet(ctx, "AS-TEST"); err != nil {
			t.Fatalf("ResolveASSet: %v", err)
		}
		if _, err := client.LookupPrefixes(ctx, "AS-TEST"); err != nil {
			t.Fatalf("LookupPrefixes: %v", err)
		}
	}

	if removed := second.ClearCache(); removed != 2 {
		t.Fatalf("ClearCache() = %d, want 2: the members and the prefix list of AS-TEST", removed)
	}
	if first.asSetCache.Len() != 1 || first.prefixCache.Len() != 1 {
		t.Fatal("clearing one client emptied another, want each owner to clear its own")
	}

	before := hits.Load()
	if _, err := second.ResolveASSet(ctx, "AS-TEST"); err != nil {
		t.Fatalf("ResolveASSet after clear: %v", err)
	}
	if hits.Load() == before {
		t.Fatal("the lookup after the clear was served from the cache, want a query")
	}
}

// VALIDATES: ClearCachedASSet removes one AS-SET and leaves the others, and
// counts the entries it removed; method: cache two sets, clear one twice.
// PREVENTS: clear irr cache as-set emptying the whole cache, or claiming a find.
func TestClearCachedASSet_OneEntry(t *testing.T) {
	var hits atomic.Int32
	addr, cleanup := countingIRRServer(t, &hits, handleASSetQuery)
	defer cleanup()
	client := NewIRR(addr)
	ctx := context.Background()

	for _, name := range []string{"AS-TEST", "AS-CHILD"} {
		if _, err := client.ResolveASSet(ctx, name); err != nil {
			t.Fatalf("ResolveASSet(%s): %v", name, err)
		}
	}

	if removed := client.ClearCachedASSet("AS-TEST"); removed != 1 {
		t.Fatalf("ClearCachedASSet(AS-TEST) = %d, want 1: only its members were cached", removed)
	}
	if removed := client.ClearCachedASSet("AS-TEST"); removed != 0 {
		t.Fatalf("ClearCachedASSet(AS-TEST) on the second call = %d, want 0", removed)
	}
	if removed := client.ClearCachedASSet("AS-NEVER-LOOKED-UP"); removed != 0 {
		t.Fatalf("ClearCachedASSet = %d for a set never looked up, want 0", removed)
	}

	before := hits.Load()
	if _, err := client.ResolveASSet(ctx, "AS-CHILD"); err != nil {
		t.Fatalf("ResolveASSet(AS-CHILD): %v", err)
	}
	if hits.Load() != before {
		t.Fatal("the set left in place was queried again, want a cached answer")
	}
	if _, err := client.ResolveASSet(ctx, "AS-TEST"); err != nil {
		t.Fatalf("ResolveASSet(AS-TEST): %v", err)
	}
	if hits.Load() == before {
		t.Fatal("the cleared set was served from the cache, want a fresh query")
	}
}
