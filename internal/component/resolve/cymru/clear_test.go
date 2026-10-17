package cymru

import (
	"context"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

// countingDNS answers every origin and AS name query and counts the origin
// queries, so a test can tell a cached origin from a fresh query.
func countingDNS(originQueries *atomic.Int32) TXTResolver {
	return func(_ context.Context, name string) ([]string, error) {
		if strings.HasSuffix(name, ".asn.cymru.com.") && strings.HasPrefix(name, "AS") {
			return []string{"65001 | US | arin | 2000-01-01 | TEST-AS - Test, US"}, nil
		}
		originQueries.Add(1)
		return []string{"65001 | 192.0.2.0/24 | US | arin | 2000-01-01"}, nil
	}
}

// VALIDATES: ClearCache empties one resolver and counts exactly the entries it
// held, and leaves another resolver alone; method: two resolvers cache an
// origin and an AS name, one is cleared, then looked up again.
// PREVENTS: clear cymru cache all counting a resolver nobody consults.
func TestClearCache_ExactCount(t *testing.T) {
	var queries atomic.Int32
	first := New(countingDNS(&queries), nil)
	second := New(countingDNS(&queries), nil)
	ctx := context.Background()

	for _, resolver := range []*CymruResolver{first, second} {
		if _, err := resolver.LookupOrigin(ctx, "192.0.2.1"); err != nil {
			t.Fatalf("LookupOrigin: %v", err)
		}
	}

	// Each resolver holds an origin and an AS name.
	if removed := first.ClearCache(); removed != 2 {
		t.Fatalf("ClearCache() = %d, want 2: the origin and the AS name", removed)
	}
	if second.cache.Len() != 2 {
		t.Fatal("clearing one resolver emptied another, want each owner to clear its own")
	}
	before := queries.Load()
	if _, err := first.LookupOrigin(ctx, "192.0.2.1"); err != nil {
		t.Fatalf("LookupOrigin after clear: %v", err)
	}
	if queries.Load() != before+1 {
		t.Fatal("the origin lookup after the clear was served from the cache, want a query")
	}
}

// VALIDATES: ClearCachedAddress removes one origin under any spelling of the
// address and leaves the others; method: cache two, clear one twice.
// PREVENTS: clear cymru cache address missing an entry cached as 2001:DB8::1.
func TestClearCachedAddress_OneEntry(t *testing.T) {
	var queries atomic.Int32
	resolver := New(countingDNS(&queries), nil)
	ctx := context.Background()

	for _, ip := range []string{"2001:DB8:0::1", "192.0.2.7"} {
		if _, err := resolver.LookupOrigin(ctx, ip); err != nil {
			t.Fatalf("LookupOrigin(%s): %v", ip, err)
		}
	}

	addr := netip.MustParseAddr("2001:db8::1")
	if removed := resolver.ClearCachedAddress(addr); removed != 1 {
		t.Fatalf("ClearCachedAddress(2001:db8::1) = %d, want 1 for an origin cached as 2001:DB8:0::1", removed)
	}
	if removed := resolver.ClearCachedAddress(addr); removed != 0 {
		t.Fatalf("ClearCachedAddress on the second call = %d, want 0", removed)
	}
	if removed := resolver.ClearCachedAddress(netip.MustParseAddr("198.51.100.1")); removed != 0 {
		t.Fatalf("ClearCachedAddress = %d for an address never looked up, want 0", removed)
	}

	before := queries.Load()
	if _, err := resolver.LookupOrigin(ctx, "192.0.2.7"); err != nil {
		t.Fatalf("LookupOrigin(192.0.2.7): %v", err)
	}
	if queries.Load() != before {
		t.Fatal("the address left in place was queried again, want a cached answer")
	}
	if _, err := resolver.LookupOrigin(ctx, "2001:db8::1"); err != nil {
		t.Fatalf("LookupOrigin(2001:db8::1): %v", err)
	}
	if queries.Load() != before+1 {
		t.Fatal("the cleared address was served from the cache, want a fresh query")
	}
}
