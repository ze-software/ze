package dnsresolver

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC2181ResolverReplacesRRsetWhole proves RFC 2181 Section 5.4 on the
// resolver path: an RRset arriving in a response for a name and type the
// cache holds live replaces the cached RRset whole. Method: a.test's A RRset
// {192.0.2.1, 192.0.2.2} is cached, then a query for alias.test draws a reply
// carrying a CNAME to a.test and a.test's A RRset {192.0.2.3}; a.test is then
// asked again, from the cache.
func TestRFC2181ResolverReplacesRRsetWhole(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		if q.Question[0].Name == "alias.test." {
			return answerWith(q, "alias.test. 300 IN CNAME a.test.", "a.test. 300 IN A 192.0.2.3")
		}
		return answerWith(q, "a.test. 300 IN A 192.0.2.1", "a.test. 300 IN A 192.0.2.2")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	if first := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); first == nil || len(first.Answer) != 2 {
		t.Fatalf("reply %v, want the two-record RRset", first)
	}
	if alias := ask(r, query("alias.test", dns.TypeA), "127.0.0.1", false); alias == nil || len(alias.Answer) != 2 {
		t.Fatalf("reply %v, want the CNAME and one A record", alias)
	}
	cached := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: a.test was not answered from the cache", n)
	}
	// RFC requirement: RFC2181-5.4-1 positive -- "Servers must never merge RRs
	// from a response with RRs in their cache to form an RRSet." After the
	// alias.test reply carried a.test A {192.0.2.3} while {192.0.2.1,
	// 192.0.2.2} was cached, a.test is answered from the cache with
	// 192.0.2.3 alone.
	if cached == nil {
		t.Fatal("no reply for a.test")
	}
	if got := ips(cached.Answer); len(got) != 1 || got[0] != "192.0.2.3" {
		t.Fatalf("cached a.test RRset %v, want only 192.0.2.3", got)
	}
}

// TestRFC2181ResolverCacheIgnoresOrDiscards proves RFC 2181 Section 5.4 at
// the resolver cache: data that would form an RRset with a cached one is
// either ignored, leaving the cached RRset as it was, or replaces the
// cached RRset whole; data for another type at the same name forms no RRset
// with it and leaves it alone. Method: puts into the resolver's LRU, ranked
// as the resolver ranks answers, read back with get.
func TestRFC2181ResolverCacheIgnoresOrDiscards(t *testing.T) {
	now := newTestClock().now()
	c := newLRU(10)
	k := keyOf("r.test", dns.TypeA, dns.ClassINET)
	entry := func(rk rank, rrs ...string) *cacheEntry {
		e := &cacheEntry{key: k, rank: rk, stored: now, ttl: 300}
		for _, s := range rrs {
			e.rrs = append(e.rrs, mustRR(s))
		}
		return e
	}

	c.put(entry(rankAnswerAuthoritative, "r.test. 300 IN A 192.0.2.1", "r.test. 300 IN A 192.0.2.2"), now)
	c.put(entry(rankAnswer, "r.test. 300 IN A 192.0.2.9"), now)
	held, _, ok := c.get(k, now)
	// RFC requirement: RFC2181-5.4-2 positive -- "If a response contains data
	// that would form an RRSet with data in a server's cache the server must
	// either ignore the RRs in the response, or discard the entire RRSet
	// currently in the cache, as appropriate." A lower-ranked A RRset
	// {192.0.2.9} put over a live authoritative {192.0.2.1, 192.0.2.2} is
	// ignored: the cache holds {192.0.2.1, 192.0.2.2} exactly. An authoritative
	// {192.0.2.7} put after it discards that RRset: the cache holds 192.0.2.7
	// alone.
	if !ok {
		t.Fatal("the authoritative RRset is gone")
	}
	if got := ips(held.rrs); len(got) != 2 || got[0] != "192.0.2.1" || got[1] != "192.0.2.2" {
		t.Fatalf("cached RRset %v, want 192.0.2.1 and 192.0.2.2 alone", got)
	}
	c.put(entry(rankAnswerAuthoritative, "r.test. 300 IN A 192.0.2.7"), now)
	held, _, ok = c.get(k, now)
	if !ok {
		t.Fatal("the replacing RRset was not stored")
	}
	if got := ips(held.rrs); len(got) != 1 || got[0] != "192.0.2.7" {
		t.Fatalf("cached RRset %v, want 192.0.2.7 alone", got)
	}

	other := &cacheEntry{key: keyOf("r.test", dns.TypeAAAA, dns.ClassINET), rank: rankAnswerAuthoritative, stored: now, ttl: 300,
		rrs: []dns.RR{mustRR("r.test. 300 IN AAAA 2001:db8::1")}}
	c.put(other, now)
	held, _, ok = c.get(k, now)
	// RFC requirement: RFC2181-5.4-2 negative -- only data that would form an
	// RRSet with cached data discards it: an AAAA RRset put for r.test leaves
	// the cached A RRset 192.0.2.7 in place.
	if !ok {
		t.Fatal("the A RRset is gone after an AAAA put")
	}
	if got := ips(held.rrs); len(got) != 1 || got[0] != "192.0.2.7" {
		t.Fatalf("cached A RRset %v, want 192.0.2.7 alone", got)
	}
}

// TestRFC2181ResolverTTLBounds proves RFC 2181 Section 8 on the resolver
// path: a received TTL with the most significant bit set is treated as zero,
// and the largest TTL Section 8 allows is kept. Method: cache max-ttl raised
// to 2147483647, so the configured cap does not decide; one upstream record
// with TTL 0x80000001 and one with TTL 2147483647, each asked twice.
func TestRFC2181ResolverTTLBounds(t *testing.T) {
	const ttlMax = 2147483647
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")
		m.Answer[0].Header().Ttl = ttlMax
		if q.Question[0].Name == "high.test." {
			m.Answer[0].Header().Ttl = 0x80000001
		}
		return m
	})
	cfg := testConfig(up)
	cfg.CacheMaxTTL = ttlMax
	r := newTestResolver(t, cfg, newTestClock())

	high := ask(r, query("high.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("high.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2181-8-1 positive -- "It is hereby specified that a
	// TTL value is an unsigned number, with a minimum value of 0, and a maximum
	// value of 2147483647." A record received with TTL 0x80000001 is answered
	// with TTL 0 and is not cached: the second query for it reaches the
	// upstream again.
	if high == nil || len(high.Answer) != 1 || high.Answer[0].Header().Ttl != 0 {
		t.Fatalf("reply %v, want the record with TTL 0", high)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries for high.test, want 2: a TTL with the high bit set was cached", n)
	}

	top := ask(r, query("top.test", dns.TypeA), "127.0.0.1", false)
	again := ask(r, query("top.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2181-8-1 negative -- 2147483647 is the maximum TTL,
	// not past it: a record received with TTL 2147483647 is answered with that
	// TTL and cached, so the second query for it is answered from the cache
	// with TTL 2147483647.
	if top == nil || len(top.Answer) != 1 || top.Answer[0].Header().Ttl != ttlMax {
		t.Fatalf("reply %v, want the record with TTL %d", top, ttlMax)
	}
	if again == nil || len(again.Answer) != 1 || again.Answer[0].Header().Ttl != ttlMax {
		t.Fatalf("cached reply %v, want the record with TTL %d", again, ttlMax)
	}
	if n := up.queries.Load(); n != 3 {
		t.Fatalf("upstream saw %d queries, want 3: the TTL-2147483647 record was not cached", n)
	}
}
