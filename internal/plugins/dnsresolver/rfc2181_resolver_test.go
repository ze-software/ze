package dnsresolver

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/dnsserver"
)

// TestRFC2181TruncatedWhenRRSetDoesNotFit proves AC-13 (RFC 2181 Section
// 5.1): a 100-record A RRset that cannot fit a 1232-octet UDP reply is sent
// marked truncated, while a reply that fits, and the same RRset over TCP,
// go out whole with TC clear. Method: an upstream that answers UDP with TC
// and TCP with the whole set, then the client asks over UDP and over TCP.
func TestRFC2181TruncatedWhenRRSetDoesNotFit(t *testing.T) {
	var big []string
	for i := 1; i <= 100; i++ {
		big = append(big, "big.test. 300 IN A 192.0.2."+strconv.Itoa(i))
	}
	up := newFakeUpstream(t, func(q *dns.Msg, tcp bool) []fakeReply {
		if q.Question[0].Name == "small.test." {
			return []fakeReply{{msg: answerWith(q, "small.test. 300 IN A 192.0.2.1")}}
		}
		if !tcp {
			m := new(dns.Msg)
			m.SetReply(q)
			m.Truncated = true
			return []fakeReply{{msg: m}}
		}
		return []fakeReply{{msg: answerWith(q, big...)}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())

	q := query("big.test", dns.TypeA)
	q.SetEdns0(dnsserver.RecursiveUDPSize, false)
	reply := ask(r, q, "127.0.0.1", false)
	if reply == nil {
		t.Fatal("no UDP reply for big.test")
	}
	wire, err := reply.Pack()
	if err != nil {
		t.Fatalf("pack UDP reply: %v", err)
	}
	// RFC requirement: RFC2181-5.1-1 positive -- the UDP reply to a query
	// offering 1232 octets, whose 100-record A RRset does not fit, has TC set,
	// packs to at most 1232 octets and carries an OPT.
	if !reply.Truncated {
		t.Fatalf("UDP reply of %d octets with TC clear, want TC set", len(wire))
	}
	if len(wire) > dnsserver.RecursiveUDPSize {
		t.Fatalf("UDP reply of %d octets, want at most %d", len(wire), dnsserver.RecursiveUDPSize)
	}
	if reply.IsEdns0() == nil {
		t.Fatal("UDP reply has no OPT")
	}

	// RFC requirement: RFC2181-5.1-1 negative -- a UDP reply whose one-record
	// RRset fits has TC clear and carries the record, and the 100-record RRset
	// asked over TCP, where it fits, comes back whole with TC clear.
	small := query("small.test", dns.TypeA)
	small.SetEdns0(dnsserver.RecursiveUDPSize, false)
	reply = ask(r, small, "127.0.0.1", false)
	if reply == nil || reply.Truncated || len(reply.Answer) != 1 {
		t.Fatalf("UDP reply for small.test %v, want one record with TC clear", reply)
	}
	reply = ask(r, query("big.test", dns.TypeA), "127.0.0.1", true)
	if reply == nil {
		t.Fatal("no TCP reply for big.test")
	}
	if reply.Truncated || len(reply.Answer) != len(big) {
		t.Fatalf("TCP reply tc=%v with %d records, want TC clear and %d", reply.Truncated, len(reply.Answer), len(big))
	}
}

// TestRFC2181LowerTrustNeverDisplacesHigher proves AC-20's ranking half (RFC
// 2181 Section 5.4.1): data ranked lower never displaces live data ranked
// higher. Method: an authoritative reply to a y.test query caches y.test's
// RRset at the authoritative rank, a non-authoritative reply to a z.test
// query then carries another y.test RRset on z.test's CNAME chain, and a
// y.test query is answered from the cache; the cache itself is then offered
// a lower-ranked put directly.
func TestRFC2181LowerTrustNeverDisplacesHigher(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "y.test.":
			m := answerWith(q, "y.test. 300 IN A 192.0.2.1")
			m.Authoritative = true
			return m
		default:
			return answerWith(q, "z.test. 300 IN CNAME y.test.", "y.test. 300 IN A 198.51.100.9")
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("z.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2181-5.4.1-3 positive -- lower-ranked data never
	// displaces live higher-ranked data: the y.test RRset of an authoritative
	// (AA=1) reply to a y.test query is still what a y.test query returns,
	// from the cache with no third upstream query, after a non-authoritative
	// reply to a z.test query carried another y.test RRset on its CNAME chain,
	// and at the cache a non-authoritative put over a live authoritative entry
	// is refused.
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("y.test answer %v, want the authoritative 192.0.2.1", got)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: y.test was not answered from the cache", n)
	}
	c := newLRU(10)
	now := newTestClock().now()
	k := keyOf("c.test", dns.TypeA, dns.ClassINET)
	c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("c.test. 60 IN A 192.0.2.1")}, rank: rankAnswerAuthoritative, stored: now, ttl: 60}, now)
	if c.put(&cacheEntry{key: k, rrs: []dns.RR{mustRR("c.test. 60 IN A 192.0.2.9")}, rank: rankAnswer, stored: now, ttl: 60}, now) {
		t.Fatal("non-authoritative data replaced live authoritative data")
	}
}

// TestRFC2181HigherTrustReplacesLower proves the other half of AC-20 (RFC
// 2181 Section 5.4.1): the ranking never keeps lower-ranked data against
// higher-ranked data. Method: a non-authoritative reply to a y.test query
// caches y.test, a w.test query then meets an alias to y.test with nothing
// about its target, so the resolver asks for y.test itself and gets an
// authoritative reply, and a y.test query is answered from the cache.
func TestRFC2181HigherTrustReplacesLower(t *testing.T) {
	var asked atomic.Int32
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "y.test.":
			if asked.Add(1) == 1 {
				return answerWith(q, "y.test. 300 IN A 198.51.100.9")
			}
			m := answerWith(q, "y.test. 300 IN A 192.0.2.1")
			m.Authoritative = true
			return m
		default:
			return answerWith(q, "w.test. 300 IN CNAME y.test.")
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("w.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2181-5.4.1-3 negative -- live non-authoritative data
	// does not hold its place against an authoritative (AA=1) reply: after
	// y.test was cached from a non-authoritative reply to a y.test query, the
	// y.test RRset of an authoritative reply to a second y.test query replaces
	// it, and a y.test query answers that RRset from the cache with no fourth
	// upstream query.
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("y.test answer %v, want the authoritative 192.0.2.1", got)
	}
	if n := up.queries.Load(); n != 3 {
		t.Fatalf("upstream saw %d queries, want 3: y.test was not answered from the cache", n)
	}
}

// TestRFC2181AliasTargetNotAuthoritative proves RFC 2181 Section 5.4.1's
// alias rule in the ranking: in an authoritative reply to a query for an
// alias, only the alias record ranks as authoritative, and the data at the
// alias target ranks as a non-authoritative answer. Method: an authoritative
// reply to x.test holds x.test's CNAME to y.test and y.test's A RRset; the
// cache entries' ranks are read, then a non-authoritative reply to z.test
// carries another y.test RRset on its own chain, and a y.test query is
// answered from the cache.
func TestRFC2181AliasTargetNotAuthoritative(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		switch q.Question[0].Name {
		case "x.test.":
			m := answerWith(q, "x.test. 300 IN CNAME y.test.", "y.test. 300 IN A 192.0.2.1")
			m.Authoritative = true
			return m
		default:
			return answerWith(q, "z.test. 300 IN CNAME y.test.", "y.test. 300 IN A 198.51.100.9")
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("x.test", dns.TypeA), "127.0.0.1", false)
	alias, target := cachedRank(r, "x.test", dns.TypeCNAME), cachedRank(r, "y.test", dns.TypeA)
	ask(r, query("z.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("y.test", dns.TypeA), "127.0.0.1", false)
	// RFC requirement: RFC2181-5.4.1-3 positive -- in an authoritative (AA=1)
	// reply to an x.test query whose answer holds x.test's CNAME to y.test and
	// y.test's A RRset, the CNAME is cached at the authoritative rank and the
	// y.test RRset at the non-authoritative answer rank, so the y.test RRset a
	// later non-authoritative reply carries replaces it, and a y.test query
	// answers that RRset from the cache with no third upstream query.
	if alias != rankAnswerAuthoritative {
		t.Errorf("x.test CNAME cached at rank %d, want the authoritative rank %d", alias, rankAnswerAuthoritative)
	}
	if target != rankAnswer {
		t.Errorf("y.test A cached at rank %d, want the non-authoritative answer rank %d", target, rankAnswer)
	}
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "198.51.100.9" {
		t.Fatalf("y.test answer %v, want 198.51.100.9, which replaced the alias target data", got)
	}
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: y.test was not answered from the cache", n)
	}
}

// cachedRank is the rank of the answer-cache entry for name and qtype, or
// rankUnspecified when there is none.
func cachedRank(r *resolver, name string, qtype uint16) rank {
	r.answers.mu.Lock()
	defer r.answers.mu.Unlock()
	e, ok := r.answers.entries[keyOf(name, qtype, dns.ClassINET)]
	if !ok {
		return rankUnspecified
	}
	return e.rank
}

// TestRFC2181WholeSIGRRSet proves AC-9's first half (RFC 2181 Section
// 5.3.1): an RRSIG query is answered with the entire RRSIG RRset, as for any
// other type, both from the upstream and from the cache. Method: an upstream
// whose RRSIG answer holds three records, asked twice.
func TestRFC2181WholeSIGRRSet(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q,
			"s.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA",
			"s.test. 300 IN RRSIG AAAA 13 2 300 20261201000000 20261101000000 1234 test. AAAA",
			"s.test. 300 IN RRSIG MX 13 2 300 20261201000000 20261101000000 1234 test. AAAA")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	forwarded := ask(r, query("s.test", dns.TypeRRSIG), "127.0.0.1", false)
	cached := ask(r, query("s.test", dns.TypeRRSIG), "127.0.0.1", false)
	if forwarded == nil || cached == nil {
		t.Fatalf("replies forwarded=%v cached=%v, want two", forwarded, cached)
	}
	// RFC requirement: RFC2181-5.3.1-3 negative -- an RRSIG RRset the upstream
	// reply to an RRSIG query carries in its answer section is taken as the
	// RRSIG RRset: the forwarded answer holds the three RRSIG records.
	sigs := 0
	for _, rr := range forwarded.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			sigs++
		}
	}
	if sigs != 3 {
		t.Fatalf("forwarded RRSIG answer %v, want the answer section's 3 RRSIG records", forwarded.Answer)
	}
	// RFC requirement: RFC2181-5.3.1-2 positive -- an RRSIG query whose
	// upstream answer section holds three RRSIG records, covering A, AAAA and
	// MX, is answered with all three, and so is the same query answered from
	// the cache.
	if len(forwarded.Answer) != 3 {
		t.Fatalf("forwarded RRSIG answer %v, want the whole set of 3", forwarded.Answer)
	}
	if len(cached.Answer) != 3 {
		t.Fatalf("cached RRSIG answer %v, want the whole set of 3", cached.Answer)
	}
	// RFC requirement: RFC2181-5.3.1-4 positive -- an RRSIG RRset that arrived
	// in the answer section of the reply to an RRSIG query is cached where a
	// later RRSIG query reads it: the second query is answered from the cache,
	// one upstream query in all.
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: the second answer did not come from the cache", n)
	}
}

// TestRFC2181SectionSIGNotTheRRSet proves AC-9's second half (RFC 2181
// Section 5.3.1): an RRSIG seen only in the authority or additional section
// is never taken as the RRSIG RRset. Method: an upstream whose every reply,
// the reply to the RRSIG query included, carries an RRSIG only in those
// sections.
func TestRFC2181SectionSIGNotTheRRSet(t *testing.T) {
	sectionSIG := "o.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA"
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := answerWith(q, "o.test. 300 IN A 192.0.2.1")
		m.Ns = append(m.Ns, mustRR(sectionSIG))
		m.Extra = append(m.Extra, mustRR(sectionSIG))
		return m
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("o.test", dns.TypeRRSIG), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("no reply to the RRSIG query")
	}
	// RFC requirement: RFC2181-5.3.1-3 positive -- an RRSIG that the upstream
	// reply to an RRSIG query carries only in its authority and additional
	// sections is not taken as the RRSIG RRset: the answer holds no RRSIG.
	for _, rr := range reply.Answer {
		if rr.Header().Rrtype == dns.TypeRRSIG {
			t.Fatalf("RRSIG from authority or additional returned as an answer: %v", rr)
		}
	}
	// RFC requirement: RFC2181-5.3.1-2 negative -- the reply to an RRSIG query
	// whose upstream answer section holds no RRSIG is never answered with part
	// of an RRSIG RRset taken from the additional section: its answer section
	// is empty.
	if len(reply.Answer) != 0 {
		t.Fatalf("RRSIG answer %v, want none: the upstream answered no RRSIG RRset", reply.Answer)
	}
}

// TestRFC2181SectionSIGNotCached proves AC-9's cache half (RFC 2181 Section
// 5.3.1): an RRSIG that arrived in the authority and additional sections of
// an A reply is not cached where an RRSIG query could read it. Method: the
// upstream answers the later RRSIG query with a different RRSIG (key tag
// 999), so an answer from the cache is told apart from the upstream's.
func TestRFC2181SectionSIGNotCached(t *testing.T) {
	sectionSIG := "o.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA"
	answerSIG := "o.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 999 test. AAAA"
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		if q.Question[0].Qtype == dns.TypeRRSIG {
			return answerWith(q, answerSIG)
		}
		m := answerWith(q, "o.test. 300 IN A 192.0.2.1")
		m.Ns = append(m.Ns, mustRR(sectionSIG))
		m.Extra = append(m.Extra, mustRR(sectionSIG))
		return m
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	ask(r, query("o.test", dns.TypeA), "127.0.0.1", false)
	reply := ask(r, query("o.test", dns.TypeRRSIG), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("no reply to the RRSIG query")
	}
	// RFC requirement: RFC2181-5.3.1-4 negative -- the RRSIG (key tag 1234)
	// carried only in the authority and additional sections of o.test's A
	// reply is not returned from the cache to a later o.test RRSIG query: that
	// query goes upstream (two upstream queries in all) and is answered with
	// the upstream's one RRSIG, key tag 999.
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: the RRSIG query was answered from the cache", n)
	}
	if len(reply.Answer) != 1 {
		t.Fatalf("RRSIG answer %v, want the upstream's one RRSIG", reply.Answer)
	}
	sig, ok := reply.Answer[0].(*dns.RRSIG)
	if !ok || sig.KeyTag != 999 {
		t.Fatalf("RRSIG answer %v, want key tag 999, not the cached section data", reply.Answer[0])
	}
}

// TestRFC2181ReplyAAClear proves AC-10's AA half (RFC 2181 Section 5.4.1):
// the resolver holds no zone, so every reply it sends has AA clear, whatever
// the upstream said. Method: an upstream that sets AA, then a forwarded
// answer, the same answer from the cache, and a refusal.
func TestRFC2181ReplyAAClear(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	forwarded := ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	cached := ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	notRecursive := query("www.example.test", dns.TypeA)
	notRecursive.RecursionDesired = false
	refused := ask(r, notRecursive, "127.0.0.1", false)
	if forwarded == nil || cached == nil || refused == nil {
		t.Fatalf("replies forwarded=%v cached=%v refused=%v, want three", forwarded, cached, refused)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: the second answer did not come from the cache", n)
	}
	if refused.Rcode != dns.RcodeRefused {
		t.Fatalf("RD=0 reply rcode %s, want REFUSED", dns.RcodeToString[refused.Rcode])
	}
	// RFC requirement: RFC2181-5.4.1-9 negative -- the resolver, an authority
	// for no name, never sets AA where it is not correct: AA is clear on an
	// answer forwarded from an upstream that set AA, on the same answer served
	// from the cache, and on the refusal of an RD=0 query.
	replies := []struct {
		name  string
		reply *dns.Msg
	}{{"forwarded", forwarded}, {"cached", cached}, {"refused", refused}}
	for _, c := range replies {
		if c.reply.Authoritative {
			t.Errorf("%s reply has AA set, want clear", c.name)
		}
	}
}
