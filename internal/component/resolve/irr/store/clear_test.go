package store

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/peeringdb"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func clearInput(cache, action, key string) rpc.ResolverCacheClearInput {
	return rpc.ResolverCacheClearInput{Cache: cache, Action: action, Key: key}
}

// mustClear runs input on s and requires removed entries.
func mustClear(t *testing.T, s *PrefixStore, input rpc.ResolverCacheClearInput, removed int) {
	t.Helper()
	got, err := s.ClearResolverCache(input)
	if err != nil {
		t.Fatalf("ClearResolverCache(%+v): %v", input, err)
	}
	if got != removed {
		t.Fatalf("ClearResolverCache(%+v) = %d, want %d", input, got, removed)
	}
}

// VALIDATES: the store clears exactly the clients it uses now, and a client
// UseClients replaced is no longer counted; method: cache through the first
// clients, clear one key, replace the clients, clear all.
// PREVENTS: clear irr cache counting the client a reload replaced.
func TestClearResolverCache_CurrentClientsOnly(t *testing.T) {
	ctx := context.Background()
	whois := fakeIRR(t, map[string]string{"AS-X": "192.0.2.0/24"}, nil)
	pdbURL := fakePeeringDB(t, map[string]string{"65001": "AS-X"})
	oldIRR, oldPDB := irr.NewIRR(whois), peeringdb.NewPeeringDB(pdbURL)
	s := New(oldIRR, oldPDB, nil)

	if _, err := oldIRR.LookupPrefixes(ctx, "AS-X"); err != nil {
		t.Fatalf("LookupPrefixes: %v", err)
	}
	if _, err := oldPDB.LookupASSet(ctx, 65001); err != nil {
		t.Fatalf("LookupASSet: %v", err)
	}
	mustClear(t, s, clearInput(rpc.ResolverCacheIRR, rpc.ResolverCacheDeleteEntry, "AS-X"), 1)
	mustClear(t, s, clearInput(rpc.ResolverCacheIRR, rpc.ResolverCacheDeleteEntry, "AS-X"), 0)

	if _, err := oldIRR.LookupPrefixes(ctx, "AS-X"); err != nil {
		t.Fatalf("LookupPrefixes: %v", err)
	}
	newIRR, newPDB := irr.NewIRR(whois), peeringdb.NewPeeringDB(pdbURL)
	s.UseClients(newIRR, newPDB)
	mustClear(t, s, clearInput(rpc.ResolverCacheIRR, rpc.ResolverCacheClearAll, ""), 0)
	mustClear(t, s, clearInput(rpc.ResolverCachePeeringDB, rpc.ResolverCacheClearAll, ""), 0)

	if _, err := newPDB.LookupASSet(ctx, 65001); err != nil {
		t.Fatalf("LookupASSet: %v", err)
	}
	mustClear(t, s, clearInput(rpc.ResolverCachePeeringDB, rpc.ResolverCacheDeleteEntry, "65001"), 1)
}

// VALIDATES: a key, a cache or an action the store cannot apply is an error,
// and a store with no PeeringDB client holds no PeeringDB entry.
// PREVENTS: a malformed request answered as removed 0.
func TestClearResolverCache_Refusals(t *testing.T) {
	s := New(irr.NewIRR("127.0.0.1:1"), nil, nil)
	mustClear(t, s, clearInput(rpc.ResolverCachePeeringDB, rpc.ResolverCacheClearAll, ""), 0)
	for _, input := range []rpc.ResolverCacheClearInput{
		clearInput(rpc.ResolverCachePeeringDB, rpc.ResolverCacheDeleteEntry, "AS-FOO"),
		clearInput(rpc.ResolverCacheCymru, rpc.ResolverCacheClearAll, ""),
		clearInput(rpc.ResolverCacheIRR, "flush", ""),
	} {
		if _, err := s.ClearResolverCache(input); err == nil {
			t.Errorf("ClearResolverCache(%+v) succeeded, want an error", input)
		}
	}
}
