package dnsresolver

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestAnyReturnsEveryOwnedRRset proves an ANY query is answered with every
// answer-section RRset the name owns, its whole RRSIG RRset included (RFC
// 2181 Section 5.3.1: "a query for all records associated with a name
// (type=ANY) the entire SIG RRSet must be included"). Method: an upstream
// answers the ANY query with an A, an AAAA, two RRSIGs and a record of
// another name; the ANY answer must hold the five records n.test owns and
// nothing else, a later A query is answered from the cache, and a second ANY
// query is forwarded again.
func TestAnyReturnsEveryOwnedRRset(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q,
			"n.test. 300 IN A 192.0.2.1",
			"n.test. 300 IN AAAA 2001:db8::1",
			"n.test. 300 IN RRSIG A 13 2 300 20261201000000 20261101000000 1234 test. AAAA",
			"n.test. 300 IN RRSIG AAAA 13 2 300 20261201000000 20261101000000 1234 test. AAAA",
			"n.test. 300 IN MX 10 mail.n.test.",
			"planted.test. 300 IN A 198.51.100.66")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("n.test", dns.TypeANY), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("ANY query dropped")
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("ANY rcode %s, want NOERROR", dns.RcodeToString[reply.Rcode])
	}
	byType := map[uint16]int{}
	for _, rr := range reply.Answer {
		if rr.Header().Name != "n.test." {
			t.Fatalf("ANY answer carries %v, a record n.test does not own", rr)
		}
		byType[rr.Header().Rrtype]++
	}
	want := map[uint16]int{dns.TypeA: 1, dns.TypeAAAA: 1, dns.TypeRRSIG: 2, dns.TypeMX: 1}
	for qt, n := range want {
		if byType[qt] != n {
			t.Fatalf("ANY answer %v holds %d %s, want %d", reply.Answer, byType[qt], dns.TypeToString[qt], n)
		}
	}
	if len(reply.Answer) != 5 {
		t.Fatalf("ANY answer %v, want the 5 records n.test owns", reply.Answer)
	}

	a := ask(r, query("n.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(a.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("A answer %v after ANY, want 192.0.2.1", a.Answer)
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: the A RRset of the ANY answer was not cached", n)
	}
	ask(r, query("n.test", dns.TypeANY), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: a second ANY query must be forwarded", n)
	}
}

// TestAnyDoesNotFollowAlias proves an ANY query at an alias is answered with
// the CNAME and no restart: the CNAME matches ANY as any type at the node
// does (RFC 1034 Section 4.3.2). Method: the upstream answers with the CNAME
// alone; the reply is that CNAME, and only one upstream query is sent.
func TestAnyDoesNotFollowAlias(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "alias.test. 300 IN CNAME target.test.")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("alias.test", dns.TypeANY), "127.0.0.1", false)
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("ANY at an alias: %v, want NOERROR with the CNAME", reply)
	}
	if _, ok := reply.Answer[0].(*dns.CNAME); !ok {
		t.Fatalf("ANY at an alias answered %v, want the CNAME", reply.Answer[0])
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1: ANY must not restart at the target", n)
	}
}

// TestRefusedLowerRankAnswersCachedSet proves a reply whose RRset the cache
// refuses, because it holds a live higher-ranked one, is answered with the
// cached RRset. Method: the cache holds t.test's authoritative A; a query
// for alias.test meets a non-authoritative reply carrying the CNAME and
// another t.test A; the client must be given the cached 192.0.2.1.
func TestRefusedLowerRankAnswersCachedSet(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "alias.test. 300 IN CNAME t.test.", "t.test. 300 IN A 198.51.100.9")
	})
	clock := newTestClock()
	r := newTestResolver(t, testConfig(up), clock)
	now := clock.now()
	r.answers.put(&cacheEntry{
		key: keyOf("t.test", dns.TypeA, dns.ClassINET), rrs: []dns.RR{mustRR("t.test. 300 IN A 192.0.2.1")},
		rank: rankAnswerAuthoritative, stored: now, ttl: 300,
	}, now)
	clock.advance(10 * time.Second)
	reply := ask(r, query("alias.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("query dropped")
	}
	got := ips(reply.Answer)
	if len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("alias.test answer %v, want the cached authoritative 192.0.2.1", reply.Answer)
	}
	for _, rr := range reply.Answer {
		if _, ok := rr.(*dns.A); ok && rr.Header().Ttl != 290 {
			t.Fatalf("cached A answered with TTL %d, want its remaining 290", rr.Header().Ttl)
		}
	}
}

// TestCoalescedWaitersBounded proves the queries waiting on one resolution
// in flight are bounded by limit outstanding: one beyond it is dropped and
// counted. Method: limit outstanding 1 and an upstream that holds its reply
// until released; one query leads, a second waits on it, and a third
// identical query is dropped at once with dropped_total +1; once released,
// the leader and the waiter both get the answer.
func TestCoalescedWaitersBounded(t *testing.T) {
	release := make(chan struct{})
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		<-release
		return []fakeReply{{msg: answerWith(q, "same.test. 300 IN A 192.0.2.1")}}
	})
	cfg := testConfig(up)
	cfg.LimitOutstanding = 1
	cfg.Timeout = 5 * time.Second
	r := newTestResolver(t, cfg, newTestClock())

	var wg sync.WaitGroup
	replies := make([]*dns.Msg, 2)
	wg.Go(func() { replies[0] = ask(r, query("same.test", dns.TypeA), "127.0.0.1", false) })
	waitFor(t, "the leader's resolution", func() bool { return r.limits.outstanding.Load() == 1 })
	wg.Go(func() { replies[1] = ask(r, query("same.test", dns.TypeA), "127.0.0.1", false) })
	waitFor(t, "the first waiter", func() bool { return r.limits.waiters.Load() == 1 })

	if reply := ask(r, query("same.test", dns.TypeA), "127.0.0.1", false); reply != nil {
		t.Fatalf("a waiter beyond limit outstanding was answered %v, want dropped", reply)
	}
	if n := r.stats.dropped.Load(); n != 1 {
		t.Fatalf("dropped %d, want 1", n)
	}
	close(release)
	wg.Wait()
	for i, reply := range replies {
		if reply == nil || len(reply.Answer) != 1 {
			t.Fatalf("client %d: %v, want the answer", i, reply)
		}
	}
	if n := r.limits.waiters.Load(); n != 0 {
		t.Fatalf("waiters %d after every query finished, want 0", n)
	}
}

// TestLeaderPanicReleasesFlight proves a panic in a resolution still
// finishes its flight and returns its outstanding slot. Method: a clock
// that panics once the upstream reply is absorbed; after the panic the
// in-flight table is empty, the slot is free, and the same question is
// resolved by a new leader.
func TestLeaderPanicReleasesFlight(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	cfg.LimitOutstanding = 1
	clock := newTestClock()
	r := newTestResolver(t, cfg, clock)
	r.now = func() time.Time { panic("test: the clock fails") }
	q := dns.Question{Name: "p.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("resolution did not panic")
			}
		}()
		r.resolveShared(r.policy.Load(), q)
	}()
	r.flights.mu.Lock()
	inFlight := len(r.flights.flights)
	r.flights.mu.Unlock()
	if inFlight != 0 {
		t.Fatalf("%d flights left in the table after a panic, want 0", inFlight)
	}
	if n := r.limits.outstanding.Load(); n != 0 {
		t.Fatalf("outstanding %d after a panic, want 0", n)
	}
	r.now = clock.now
	if res := r.resolveShared(r.policy.Load(), q); res.outcome != outcomeAnswered || len(res.answer) != 1 {
		t.Fatalf("after the panic: %+v, want the answer", res)
	}
}

// TestIntersectPrefixes proves the reload window's allow-from holds exactly
// the addresses both lists admit.
func TestIntersectPrefixes(t *testing.T) {
	p := netip.MustParsePrefix
	got := intersectPrefixes(
		[]netip.Prefix{p("10.0.0.0/8"), p("192.0.2.0/24")},
		[]netip.Prefix{p("10.1.0.0/16"), p("192.0.0.0/16"), p("198.51.100.0/24")})
	want := []netip.Prefix{p("10.1.0.0/16"), p("192.0.2.0/24")}
	if len(got) != len(want) {
		t.Fatalf("intersection %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("intersection %v, want %v", got, want)
		}
	}
}

// waitFor polls cond until it holds, failing after two seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		// sleep(poll): no channel reports the resolver's counters; 1ms bounds the poll cost.
		time.Sleep(time.Millisecond)
	}
}
