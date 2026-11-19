// Design: docs/architecture/dns/lan-resolver.md -- the negative cache, and the
// server failure that is never cached
// RFC: rfc/short/rfc2308.md -- negative caching (Sections 5, 6, 7.1, 7.2, 8)
// Related: cache_test.go -- negativeUpstream, scriptedUpstream, ips

package dnsresolver

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

// rfc2308SOA is the in-bailiwick SOA the scripted upstreams of this file put
// in the authority section: owner test., TTL 60, MINIMUM 60.
const rfc2308SOA = "test. 60 IN SOA ns.test. host.test. 1 3600 600 86400 60"

// soaOf returns the single SOA of rrs, or nil when rrs is not exactly one SOA.
func soaOf(rrs []dns.RR) *dns.SOA {
	if len(rrs) != 1 {
		return nil
	}
	soa, ok := rrs[0].(*dns.SOA)
	if !ok {
		return nil
	}
	return soa
}

// TestRFC2308NegativeEntryExpires proves RFC 2308 Section 5 on the resolver
// path: a cached negative answer is used while its TTL lasts, and once the
// TTL has decremented to zero it is never used again. Method: nx.test draws
// NXDOMAIN with an SOA of TTL 60 and MINIMUM 60, so the entry lives 60
// seconds; the test clock is moved to 59 seconds, then to 60.
func TestRFC2308NegativeEntryExpires(t *testing.T) {
	up := negativeUpstream(t, 60, 60)
	clock := newTestClock()
	r := newTestResolver(t, testConfig(up), clock)
	if first := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false); first == nil || first.Rcode != dns.RcodeNameError {
		t.Fatalf("reply %v, want NXDOMAIN", first)
	}

	clock.advance(59 * time.Second)
	live := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-5-1 negative -- a cached NXDOMAIN of TTL 60 is
	// still used 59 seconds after it was stored: the query is answered
	// NXDOMAIN from the cache, no upstream query is made, and the SOA it
	// returns carries the decremented TTL 1.
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries at 59s, want 1: the live negative entry was not used", n)
	}
	if live == nil || live.Rcode != dns.RcodeNameError {
		t.Fatalf("reply %v at 59s, want NXDOMAIN from the cache", live)
	}
	if soa := soaOf(live.Ns); soa == nil || soa.Hdr.Ttl != 1 {
		t.Fatalf("authority %v at 59s, want the cached SOA with TTL 1", live.Ns)
	}

	clock.advance(time.Second)
	ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-5-1 positive -- once the TTL has reached zero,
	// 60 seconds after the entry was stored, the cached NXDOMAIN is not used
	// again: the same query reaches the upstream.
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries at 60s, want 2: the negative entry was used at TTL 0", n)
	}
}

// TestRFC2308CachedSOATTLDecremented proves RFC 2308 Section 6 on the
// resolver path: a negative answer served from the cache carries the cached
// SOA in the authority section with its TTL decremented by the time it was
// stored, and a negative answer fresh from the upstream carries the SOA's
// whole TTL however long the resolver ran before it. Method: the test clock
// runs 100 seconds before nx.test (NXDOMAIN) and nodata.test AAAA (NODATA)
// are first asked, each with an SOA of TTL 60, then 25 seconds more before
// each is asked again.
func TestRFC2308CachedSOATTLDecremented(t *testing.T) {
	up := negativeUpstream(t, 60, 60)
	clock := newTestClock()
	r := newTestResolver(t, testConfig(up), clock)
	clock.advance(100 * time.Second)

	freshNX := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	freshNoData := ask(r, query("nodata.test", dns.TypeAAAA), "127.0.0.1", false)
	// RFC requirement: RFC2308-6-1 negative -- an answer that did not come
	// from the cache is not decremented: the NXDOMAIN and the NODATA fresh
	// from the upstream, asked 100 seconds after the resolver started, each
	// carry the SOA with its whole TTL 60.
	if freshNX == nil || freshNX.Rcode != dns.RcodeNameError {
		t.Fatalf("reply %v, want NXDOMAIN", freshNX)
	}
	if soa := soaOf(freshNX.Ns); soa == nil || soa.Hdr.Ttl != 60 {
		t.Fatalf("fresh NXDOMAIN authority %v, want the SOA with TTL 60", freshNX.Ns)
	}
	if freshNoData == nil || freshNoData.Rcode != dns.RcodeSuccess || len(freshNoData.Answer) != 0 {
		t.Fatalf("reply %v, want NODATA", freshNoData)
	}
	if soa := soaOf(freshNoData.Ns); soa == nil || soa.Hdr.Ttl != 60 {
		t.Fatalf("fresh NODATA authority %v, want the SOA with TTL 60", freshNoData.Ns)
	}

	clock.advance(25 * time.Second)
	cachedNX := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	cachedNoData := ask(r, query("nodata.test", dns.TypeAAAA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: a negative answer was not served from the cache", n)
	}
	// RFC requirement: RFC2308-6-1 positive -- a cached negative answer, an
	// NXDOMAIN and a NODATA each stored 25 seconds earlier with an SOA of TTL
	// 60, is answered with the cached SOA in the authority section at TTL 35.
	if cachedNX == nil || cachedNX.Rcode != dns.RcodeNameError {
		t.Fatalf("cached reply %v, want NXDOMAIN", cachedNX)
	}
	if soa := soaOf(cachedNX.Ns); soa == nil || soa.Hdr.Name != "test." || soa.Hdr.Ttl != 35 {
		t.Fatalf("cached NXDOMAIN authority %v, want the test. SOA with TTL 35", cachedNX.Ns)
	}
	if cachedNoData == nil || cachedNoData.Rcode != dns.RcodeSuccess || len(cachedNoData.Answer) != 0 {
		t.Fatalf("cached reply %v, want NODATA", cachedNoData)
	}
	if soa := soaOf(cachedNoData.Ns); soa == nil || soa.Hdr.Name != "test." || soa.Hdr.Ttl != 35 {
		t.Fatalf("cached NODATA authority %v, want the test. SOA with TTL 35", cachedNoData.Ns)
	}
}

// TestRFC2308AuthoritySOACached proves RFC 2308 Section 8 on the resolver
// path: the SOA of a negative answer's authority section is cached when it
// owns the name asked or one of its ancestors, and an SOA for an unrelated
// zone is not. Method: one upstream answers nx.test NXDOMAIN with the test.
// SOA, another answers it NXDOMAIN with an SOA owned by other.example.; each
// is asked twice.
func TestRFC2308AuthoritySOACached(t *testing.T) {
	inside := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetRcode(q, dns.RcodeNameError)
		m.Ns = []dns.RR{mustRR(rfc2308SOA)}
		return m
	})
	r := newTestResolver(t, testConfig(inside), newTestClock())
	ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	cached := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-8-1 positive -- the authority-section SOA of
	// an NXDOMAIN for nx.test, owned by its ancestor test., is cached: the
	// second query is answered from the cache, with that SOA in the
	// authority section.
	if n := inside.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: the SOA was not cached", n)
	}
	if cached == nil || cached.Rcode != dns.RcodeNameError {
		t.Fatalf("cached reply %v, want NXDOMAIN", cached)
	}
	if soa := soaOf(cached.Ns); soa == nil || soa.Hdr.Name != "test." {
		t.Fatalf("cached authority %v, want the test. SOA", cached.Ns)
	}

	outside := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetRcode(q, dns.RcodeNameError)
		m.Ns = []dns.RR{mustRR("other.example. 60 IN SOA ns.other.example. host.other.example. 1 3600 600 86400 60")}
		return m
	})
	r = newTestResolver(t, testConfig(outside), newTestClock())
	first := ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("nx.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-8-1 negative -- an authority-section SOA owned
	// by other.example., which is neither nx.test nor an ancestor of it, is
	// not cached and not returned: the NXDOMAIN reaches the client with an
	// empty authority section, and the second query reaches the upstream.
	if first == nil || first.Rcode != dns.RcodeNameError || len(first.Ns) != 0 {
		t.Fatalf("reply %v, want NXDOMAIN with no authority record", first)
	}
	if n := outside.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: an out-of-bailiwick SOA was cached", n)
	}
}

// serverFailureUpstream answers fail.test A with SERVFAIL, carrying the
// test. SOA in the authority section so a resolver that took the failure
// for a negative answer would have an SOA to cache it by, and answers
// fail.test AAAA with 2001:db8::1.
func serverFailureUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	return scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		if q.Question[0].Qtype == dns.TypeAAAA {
			return answerWith(q, "fail.test. 60 IN AAAA 2001:db8::1")
		}
		m := new(dns.Msg)
		m.SetRcode(q, dns.RcodeServerFailure)
		m.Ns = []dns.RR{mustRR(rfc2308SOA)}
		return m
	})
}

// TestRFC2308ServerFailureNotCached proves RFC 2308 Section 7.1 on the
// resolver path, whose OPTIONAL server-failure cache Ze does not keep: a
// server failure is cached for no time at all, so an identical query made
// at once is forwarded again. Method: fail.test A draws SERVFAIL from the
// only upstream, then is asked again with the test clock unmoved.
func TestRFC2308ServerFailureNotCached(t *testing.T) {
	up := serverFailureUpstream(t)
	r := newTestResolver(t, testConfig(up), newTestClock())
	first := ask(r, query("fail.test", dns.TypeA), "127.0.0.1", false)
	again := ask(r, query("fail.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-7.1-1 negative -- a SERVFAIL is not cached:
	// the identical fail.test A query made at once, with no time elapsed,
	// reaches the upstream again and is answered SERVFAIL again.
	if first == nil || first.Rcode != dns.RcodeServerFailure {
		t.Fatalf("reply %v, want SERVFAIL", first)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: the server failure was cached", n)
	}
	if again == nil || again.Rcode != dns.RcodeServerFailure {
		t.Fatalf("second reply %v, want SERVFAIL from the upstream", again)
	}
}

// TestRFC2308ServerFailureHeldAgainstNoTuple proves RFC 2308 Section 7.1 on
// the resolver path: no cached server failure exists, so none reaches the
// tuple that drew it nor any other. Method: fail.test A draws SERVFAIL, then
// fail.test A and fail.test AAAA, the same name and class under another type,
// are each asked.
func TestRFC2308ServerFailureHeldAgainstNoTuple(t *testing.T) {
	up := serverFailureUpstream(t)
	r := newTestResolver(t, testConfig(up), newTestClock())
	if first := ask(r, query("fail.test", dns.TypeA), "127.0.0.1", false); first == nil || first.Rcode != dns.RcodeServerFailure {
		t.Fatalf("reply %v, want SERVFAIL", first)
	}
	ask(r, query("fail.test", dns.TypeA), "127.0.0.1", false)
	other := ask(r, query("fail.test", dns.TypeAAAA), "127.0.0.1", false)
	// RFC requirement: RFC2308-7.1-2 negative -- after a SERVFAIL for
	// <fail.test, A, IN, the upstream>, the same tuple asked again reaches the
	// upstream, and so does <fail.test, AAAA, IN, the upstream>, which is
	// answered 2001:db8::1.
	if n := up.queries.Load(); n != 3 {
		t.Fatalf("upstream saw %d queries, want 3: a server failure was held against a tuple", n)
	}
	if other == nil || other.Rcode != dns.RcodeSuccess || len(other.Answer) != 1 {
		t.Fatalf("AAAA reply %v, want the upstream's 2001:db8::1", other)
	}
	if aaaa, ok := other.Answer[0].(*dns.AAAA); !ok || aaaa.AAAA.String() != "2001:db8::1" {
		t.Fatalf("AAAA answer %v, want 2001:db8::1", other.Answer[0])
	}
}

// silentUpstream never answers silent.test and answers every other name
// with 192.0.2.1.
func silentUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	return newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		if q.Question[0].Name == "silent.test." {
			return nil
		}
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 60 IN A 192.0.2.1")}}
	})
}

// TestRFC2308SilentUpstreamAskedAgain proves RFC 2308 Section 7.2 on the
// resolver path, whose OPTIONAL dead-server indication Ze does not keep: an
// upstream that did not answer is deemed dead for no time at all, so the
// next query is sent to it again. Method: the only upstream stays silent
// for silent.test A, which is asked twice with the test clock unmoved; the
// timeout is shortened to keep the test quick.
func TestRFC2308SilentUpstreamAskedAgain(t *testing.T) {
	up := silentUpstream(t)
	cfg := testConfig(up)
	cfg.Timeout = 300 * time.Millisecond
	r := newTestResolver(t, cfg, newTestClock())
	first := ask(r, query("silent.test", dns.TypeA), "127.0.0.1", false)
	again := ask(r, query("silent.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-7.2-1 negative -- an upstream that did not
	// answer silent.test A is not deemed dead: the next query, made at once,
	// is sent to it again, and each draws SERVFAIL when it stays silent.
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: the silent upstream was deemed dead", n)
	}
	if first == nil || first.Rcode != dns.RcodeServerFailure || again == nil || again.Rcode != dns.RcodeServerFailure {
		t.Fatalf("replies %v and %v, want SERVFAIL for both", first, again)
	}
}

// TestRFC2308SilentUpstreamNotDeemedDeadBeyondTheTuple proves RFC 2308
// Section 7.2 on the resolver path: no dead-server indication is stored, so
// an upstream that was silent for one query is asked both for another tuple
// and for the same tuple again. Method: the only upstream stays silent for
// silent.test A and answers live.test A; silent.test, live.test and
// silent.test are asked in turn.
func TestRFC2308SilentUpstreamNotDeemedDeadBeyondTheTuple(t *testing.T) {
	up := silentUpstream(t)
	cfg := testConfig(up)
	cfg.Timeout = 300 * time.Millisecond
	r := newTestResolver(t, cfg, newTestClock())
	if first := ask(r, query("silent.test", dns.TypeA), "127.0.0.1", false); first == nil || first.Rcode != dns.RcodeServerFailure {
		t.Fatalf("reply %v, want SERVFAIL from the silent upstream", first)
	}
	live := ask(r, query("live.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("silent.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2308-7.2-2 negative -- after the upstream was
	// silent for <silent.test, A, IN, the upstream>, it is asked for
	// <live.test, A, IN, the upstream> and answers 192.0.2.1, and it is asked
	// for <silent.test, A, IN, the upstream> again.
	if live == nil || live.Rcode != dns.RcodeSuccess {
		t.Fatalf("live.test reply %v, want NOERROR from the upstream", live)
	}
	if got := ips(live.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("live.test answer %v, want 192.0.2.1", got)
	}
	if n := up.queries.Load(); n != 3 {
		t.Fatalf("upstream saw %d queries, want 3: the silent upstream was deemed dead", n)
	}
}
