package dns

import (
	"slices"
	"testing"
)

// VALIDATES: RFC 2181 section 5.4 -- response data that forms an RRSet with
// cached data (same name, same type) discards the WHOLE cached RRSet: a
// response sharing one record with a three-record cached set leaves only the
// response's records, never the union and never the cached remainder. Data for
// another type or another name forms no RRSet with it and leaves it whole.
// PREVENTS: a cache that merges response RRs into a cached RRSet, or that
// discards the wrong set.
func TestRFC2181FormingAnRRSetDiscardsTheWholeCachedSet(t *testing.T) {
	c := newCache(100, 3600)
	c.put("www.example.", 1, []string{"192.0.2.1", "192.0.2.2", "192.0.2.3"}, 300)
	c.put("www.example.", 28, []string{"2001:db8::1"}, 300)
	c.put("mail.example.", 1, []string{"192.0.2.9"}, 300)

	// RFC requirement: RFC2181-5.4-2 positive -- a response {192.0.2.2} forming an RRSet with the cached A set {192.0.2.1, 192.0.2.2, 192.0.2.3} replaces it entirely: the A set read back is exactly {192.0.2.2}, with neither the union nor any cached remainder.
	c.put("www.example.", 1, []string{"192.0.2.2"}, 300)
	records, ok := c.get("www.example.", 1)
	if !ok {
		t.Fatal("the response RRSet was not cached")
	}
	if !slices.Equal(records, []string{"192.0.2.2"}) {
		t.Errorf("A RRSet after the response = %v, want exactly [192.0.2.2]", records)
	}

	// RFC requirement: RFC2181-5.4-2 negative -- data that forms no RRSet with a cached set (the AAAA set of the same name, the A set of another name) does not discard it: both read back unchanged after the A response.
	if aaaa, ok := c.get("www.example.", 28); !ok || !slices.Equal(aaaa, []string{"2001:db8::1"}) {
		t.Errorf("AAAA RRSet of the same name = %v (cached %v), want [2001:db8::1] kept", aaaa, ok)
	}
	if other, ok := c.get("mail.example.", 1); !ok || !slices.Equal(other, []string{"192.0.2.9"}) {
		t.Errorf("A RRSet of another name = %v (cached %v), want [192.0.2.9] kept", other, ok)
	}
}
