package dnsresolver

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// scriptedUpstream answers from the replies function, which may change
// between queries.
func scriptedUpstream(t *testing.T, replies func(q *dns.Msg) *dns.Msg) *fakeUpstream {
	t.Helper()
	return newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return []fakeReply{{msg: replies(q)}}
	})
}

func ips(rrs []dns.RR) []string {
	var out []string
	for _, rr := range rrs {
		if a, ok := rr.(*dns.A); ok {
			out = append(out, a.A.String())
		}
	}
	return out
}

// TestCacheTTLDecrement proves AC-5: a repeat within the TTL is answered
// from the cache with no upstream query, its TTL reduced by the elapsed
// whole seconds.
func TestCacheTTLDecrement(t *testing.T) {
	up := aUpstream(t, 0)
	clock := newTestClock()
	r := newTestResolver(t, testConfig(up), clock)
	ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	clock.advance(3400 * time.Millisecond)
	reply := ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil || len(reply.Answer) != 1 {
		t.Fatalf("cached reply %v", reply)
	}
	if ttl := reply.Answer[0].Header().Ttl; ttl != 297 {
		t.Fatalf("TTL %d, want 297 (300 - 3 whole seconds)", ttl)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1", n)
	}
	if n := r.stats.hits.Load(); n != 1 {
		t.Fatalf("cache hits %d, want 1", n)
	}
}

// TestCacheCaseFold proves AC-27: another spelling of the name reaches the
// same entry, and the reply's question echoes the query's case.
func TestCacheCaseFold(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("WWW.Example.TEST", dns.TypeA), "127.0.0.1", false)
	if reply == nil || len(reply.Answer) != 1 {
		t.Fatalf("reply %v", reply)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: case made a second entry", n)
	}
	if got := reply.Question[0].Name; got != "WWW.Example.TEST." {
		t.Fatalf("question %q, want the query's case", got)
	}
}

// TestCacheReplaceNotMerge proves AC-6 (RFC 2181 Section 5.4): a new RRset
// after expiry replaces the cached one whole.
func TestCacheReplaceNotMerge(t *testing.T) {
	var second atomic.Bool
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		if second.Load() {
			return answerWith(q, "a.test. 60 IN A 192.0.2.3")
		}
		return answerWith(q, "a.test. 10 IN A 192.0.2.1", "a.test. 10 IN A 192.0.2.2")
	})
	clock := newTestClock()
	r := newTestResolver(t, testConfig(up), clock)
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	second.Store(true)
	clock.advance(11 * time.Second)
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.3" {
		t.Fatalf("cached RRset %v, want only 192.0.2.3", got)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2", n)
	}

	// The same rule at the cache: a put of the same key holds only the new set.
	c := newLRU(10)
	now := clock.now()
	k := keyOf("b.test", dns.TypeA, dns.ClassINET)
	c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("b.test. 60 IN A 192.0.2.1"), mustRR("b.test. 60 IN A 192.0.2.2")}, rank: rankAnswer, stored: now, ttl: 60}, now)
	c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("b.test. 60 IN A 192.0.2.9")}, rank: rankAnswer, stored: now, ttl: 60}, now)
	e, _, ok := c.get(k, now)
	if !ok || len(e.rrs) != 1 {
		t.Fatalf("entry %v, want the one new record", e)
	}
}

// TestTTLHighBit proves AC-21 (RFC 2181 Section 8): a TTL with the high bit
// set is treated as 0 and is not cached.
func TestTTLHighBit(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := answerWith(q, "a.test. 60 IN A 192.0.2.1")
		m.Answer[0].Header().Ttl = 0x80000001
		return m
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil || len(reply.Answer) != 1 || reply.Answer[0].Header().Ttl != 0 {
		t.Fatalf("reply %v, want the record with TTL 0", reply)
	}
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: a TTL-0 record was cached", n)
	}
}

// TestCacheMaxTTL proves the cache max-ttl cap on the reply and the cache.
func TestCacheMaxTTL(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg { return answerWith(q, "a.test. 90000 IN A 192.0.2.1") })
	cfg := testConfig(up)
	cfg.CacheMaxTTL = 100
	clock := newTestClock()
	r := newTestResolver(t, cfg, clock)
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply.Answer[0].Header().Ttl != 100 {
		t.Fatalf("TTL %d, want the cap 100", reply.Answer[0].Header().Ttl)
	}
	clock.advance(101 * time.Second)
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: the entry outlived max-ttl", n)
	}
}

// negativeUpstream answers nx.test with NXDOMAIN and nodata.test with
// NOERROR and no data, each with an SOA of the given TTL and MINIMUM.
func negativeUpstream(t *testing.T, soaTTL, minimum uint32) *fakeUpstream {
	t.Helper()
	return scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetReply(q)
		soa, ok := mustRR("test. 1 IN SOA ns.test. host.test. 1 3600 600 86400 1").(*dns.SOA)
		if !ok {
			t.Fatal("literal SOA did not parse as an SOA")
		}
		soa.Hdr.Ttl = soaTTL
		soa.Minttl = minimum
		m.Ns = []dns.RR{soa}
		if q.Question[0].Name == "nx.test." {
			m.Rcode = dns.RcodeNameError
		}
		return m
	})
}

// TestNegativeCacheTTL proves AC-7 and R-10 (RFC 2308 Section 5): NXDOMAIN
// is cached by name and class for min(SOA TTL, MINIMUM), NODATA per type,
// and max-negative-ttl caps a larger value.
func TestNegativeCacheTTL(t *testing.T) {
	for _, tc := range []struct {
		name            string
		soaTTL, minimum uint32
		maxNeg          uint32
		want            uint32
	}{
		{"minimum smaller", 300, 60, 3600, 60},
		{"soa ttl smaller", 20, 60, 3600, 20},
		{"capped", 300, 600, 30, 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up := negativeUpstream(t, tc.soaTTL, tc.minimum)
			cfg := testConfig(up)
			cfg.CacheMaxNegTTL = tc.maxNeg
			clock := newTestClock()
			r := newTestResolver(t, cfg, clock)
			reply := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
			if reply.Rcode != dns.RcodeNameError || len(reply.Ns) != 1 || reply.Ns[0].Header().Ttl != tc.want {
				t.Fatalf("reply %v, want NXDOMAIN with SOA TTL %d", reply, tc.want)
			}
			// NXDOMAIN covers every type of the name.
			if reply := ask(r, query("nx.test", dns.TypeMX), "127.0.0.1", false); reply.Rcode != dns.RcodeNameError {
				t.Fatalf("MX after NXDOMAIN: %v", reply)
			}
			if n := up.queries.Load(); n != 1 {
				t.Fatalf("upstream saw %d queries, want 1", n)
			}
			clock.advance(time.Duration(tc.want) * time.Second)
			ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
			if n := up.queries.Load(); n != 2 {
				t.Fatalf("upstream saw %d queries after expiry, want 2", n)
			}
		})
	}

	up := negativeUpstream(t, 300, 60)
	r := newTestResolver(t, testConfig(up), newTestClock())
	if reply := ask(r, query("nodata.test", dns.TypeAAAA), "127.0.0.1", false); reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 0 || len(reply.Ns) != 1 {
		t.Fatalf("NODATA reply %v", reply)
	}
	ask(r, query("nodata.test", dns.TypeAAAA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("NODATA not cached for its type: %d upstream queries", n)
	}
	ask(r, query("nodata.test", dns.TypeA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("NODATA for AAAA answered A: %d upstream queries, want 2", n)
	}
}

// TestNegativeWithoutSOANotCached proves RFC 2308 Section 5: a negative
// answer without an SOA is returned but not cached.
func TestNegativeWithoutSOANotCached(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetRcode(q, dns.RcodeNameError)
		return m
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	if reply := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeNameError {
		t.Fatalf("reply %v", reply)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2", n)
	}
}

// TestTrustRanking proves AC-20 (RFC 2181 Section 5.4.1): additional-section
// data is never returned as an answer, the answer-section RRset is what a
// later query returns, and lower-ranked data never replaces live
// authoritative data, both at the cache and from an upstream reply whose
// AA bit sets the rank.
func TestTrustRanking(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "a.test.":
			m := answerWith(q, "a.test. 300 IN A 192.0.2.1")
			m.Extra = append(m.Extra, mustRR("b.test. 300 IN A 198.51.100.9"))
			return m
		default:
			return answerWith(q, "b.test. 300 IN A 192.0.2.2")
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("b.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.2" {
		t.Fatalf("b.test answer %v, want the answer-section 192.0.2.2", got)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: additional data answered b.test", n)
	}

	// The AA bit reaches the rank: y.test's authoritative RRset, cached from
	// x.test's chain, survives a non-authoritative copy in z.test's chain.
	chains := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "x.test.":
			m := answerWith(q, "x.test. 300 IN CNAME y.test.", "y.test. 300 IN A 192.0.2.1")
			m.Authoritative = true
			return m
		default:
			return answerWith(q, "z.test. 300 IN CNAME y.test.", "y.test. 300 IN A 198.51.100.9")
		}
	})
	r = newTestResolver(t, testConfig(chains), newTestClock())
	ask(r, query("x.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("z.test", dns.TypeA), "127.0.0.1", false)
	reply = ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("y.test answer %v, want the authoritative 192.0.2.1", got)
	}
	if n := chains.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: y.test was not answered from the cache", n)
	}

	c := newLRU(10)
	now := newTestClock().now()
	k := keyOf("c.test", dns.TypeA, dns.ClassINET)
	c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("c.test. 60 IN A 192.0.2.1")}, rank: rankAnswerAuthoritative, stored: now, ttl: 60}, now)
	if c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("c.test. 60 IN A 192.0.2.9")}, rank: rankAnswer, stored: now, ttl: 60}, now) {
		t.Fatal("non-authoritative data replaced live authoritative data")
	}
	if !c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("c.test. 60 IN A 192.0.2.7")}, rank: rankAnswerAuthoritative, stored: now, ttl: 60}, now) {
		t.Fatal("authoritative data did not replace authoritative data")
	}
}

// TestSIGRRSetHandling proves AC-9 (RFC 2181 Section 5.3.1): an RRSIG query
// returns the entire RRSIG RRset as an ordinary RRset, and an RRSIG seen only
// in an authority or additional section is never returned for one.
func TestSIGRRSetHandling(t *testing.T) {
	sigA := "s.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA"
	sigAAAA := "s.test. 300 IN RRSIG AAAA 13 2 300 20261201000000 20261101000000 1234 test. AAAA"
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "s.test.":
			return answerWith(q, sigA, sigAAAA)
		default:
			m := answerWith(q, "o.test. 300 IN A 192.0.2.1")
			m.Ns = append(m.Ns, mustRR("o.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA"))
			m.Extra = append(m.Extra, mustRR("o.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA"))
			return m
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("s.test", dns.TypeRRSIG), "127.0.0.1", false)
	if reply == nil || len(reply.Answer) != 2 {
		t.Fatalf("RRSIG answer %v, want the whole set of 2", reply)
	}
	ask(r, query("o.test", dns.TypeA), "127.0.0.1", false)
	reply = ask(r, query("o.test", dns.TypeRRSIG), "127.0.0.1", false)
	for _, rr := range reply.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			t.Fatalf("RRSIG from authority or additional returned as an answer: %v", rr)
		}
	}
	if n := up.queries.Load(); n != 3 {
		t.Fatalf("upstream saw %d queries, want 3: the RRSIG query was answered from section data", n)
	}
}
