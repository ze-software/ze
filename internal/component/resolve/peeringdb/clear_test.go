package peeringdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// countingServer answers like fakeHandler and counts the requests it serves,
// so a test can tell a cached answer from a fresh query.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fakeHandler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// VALIDATES: ClearCache empties one client and counts exactly the entries it
// held, and leaves another client alone; method: two clients each cache two
// entries, one is cleared, then each is looked up again.
// PREVENTS: clear peeringdb cache all reporting entries of a client its owner
// does not use, or missing the next query.
func TestClearCache_ExactCount(t *testing.T) {
	srv, hits := countingServer(t)
	first := NewPeeringDB(srv.URL)
	second := NewPeeringDB(srv.URL)
	ctx := context.Background()

	for _, client := range []*PeeringDB{first, second} {
		if _, err := client.LookupASN(ctx, 65001); err != nil {
			t.Fatalf("LookupASN: %v", err)
		}
		if _, err := client.LookupASSet(ctx, 65001); err != nil {
			t.Fatalf("LookupASSet: %v", err)
		}
	}
	if got := hits.Load(); got != 4 {
		t.Fatalf("server served %d requests before the clear, want 4", got)
	}

	if removed := first.ClearCache(); removed != 2 {
		t.Fatalf("ClearCache() = %d, want 2: the prefix counts and the AS-SET names of 65001", removed)
	}
	if removed := first.ClearCache(); removed != 0 {
		t.Fatalf("ClearCache() on an empty cache = %d, want 0", removed)
	}
	if second.prefixCache.Len() != 1 || second.asSetCache.Len() != 1 {
		t.Fatal("clearing one client emptied another, want each owner to clear its own")
	}

	if _, err := first.LookupASN(ctx, 65001); err != nil {
		t.Fatalf("LookupASN after clear: %v", err)
	}
	if got := hits.Load(); got != 5 {
		t.Fatalf("server served %d requests, want 5: the lookup after the clear must query again", got)
	}
}

// VALIDATES: ClearCachedASN removes one ASN and leaves the others cached, and
// counts the entries it removed; method: cache two ASNs, clear one twice.
// PREVENTS: clear peeringdb cache asn emptying the whole cache, or claiming a find.
func TestClearCachedASN_OneEntry(t *testing.T) {
	srv, hits := countingServer(t)
	client := NewPeeringDB(srv.URL)
	ctx := context.Background()

	for _, asn := range []uint32{64601, 64602} {
		if _, err := client.LookupASSet(ctx, asn); err != nil {
			t.Fatalf("LookupASSet(%d): %v", asn, err)
		}
	}

	if removed := client.ClearCachedASN(64601); removed != 1 {
		t.Fatalf("ClearCachedASN(64601) = %d, want 1: only its AS-SET names were cached", removed)
	}
	if removed := client.ClearCachedASN(64601); removed != 0 {
		t.Fatalf("ClearCachedASN(64601) on the second call = %d, want 0", removed)
	}
	if removed := client.ClearCachedASN(64699); removed != 0 {
		t.Fatalf("ClearCachedASN(64699) = %d for an ASN never looked up, want 0", removed)
	}

	before := hits.Load()
	if _, err := client.LookupASSet(ctx, 64602); err != nil {
		t.Fatalf("LookupASSet(64602): %v", err)
	}
	if hits.Load() != before {
		t.Fatal("the ASN left in place was queried again, want a cached answer")
	}
	if _, err := client.LookupASSet(ctx, 64601); err != nil {
		t.Fatalf("LookupASSet(64601): %v", err)
	}
	if hits.Load() != before+1 {
		t.Fatal("the cleared ASN was served from the cache, want a fresh query")
	}
}
