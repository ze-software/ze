package dnsresolver

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// aUpstream answers every A query for its name with one A record, TTL 300,
// and sets AA and AD so the tests can see the resolver clear them.
func aUpstream(t *testing.T, delay time.Duration) *fakeUpstream {
	t.Helper()
	return newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		m := answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1")
		m.Authoritative = true
		m.AuthenticatedData = true
		return []fakeReply{{msg: m, delay: delay}}
	})
}

// TestForwardAnswer proves AC-1 and AC-10: an allowed RD=1 query is
// answered with the upstream's records, RA=1, AA=0, AD=0, Z=0, and a TTL no
// greater than the upstream's, even when the upstream set AA and AD.
func TestForwardAnswer(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false)
	if reply == nil {
		t.Fatal("no reply")
	}
	if reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("rcode %s, want NOERROR", dns.RcodeToString[reply.Rcode])
	}
	if !reply.RecursionAvailable || reply.Authoritative || reply.AuthenticatedData || reply.Zero {
		t.Fatalf("flags ra=%v aa=%v ad=%v z=%v, want ra only", reply.RecursionAvailable, reply.Authoritative, reply.AuthenticatedData, reply.Zero)
	}
	if len(reply.Answer) != 1 {
		t.Fatalf("answer %v, want one A", reply.Answer)
	}
	a, ok := reply.Answer[0].(*dns.A)
	if !ok || a.A.String() != "192.0.2.1" || a.Hdr.Ttl > 300 {
		t.Fatalf("answer %v, want 192.0.2.1 with TTL at most 300", reply.Answer[0])
	}
	// The same question over the TCP listener is answered the same way.
	reply = ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", true)
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("TCP reply %v", reply)
	}
}

// TestACLRefused proves AC-2: a source outside allow-from gets REFUSED and
// no upstream query is sent.
func TestACLRefused(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("www.example.test", dns.TypeA), "10.9.9.9", false)
	if reply == nil || reply.Rcode != dns.RcodeRefused {
		t.Fatalf("reply %v, want REFUSED", reply)
	}
	if reply.Authoritative || len(reply.Answer) != 0 {
		t.Fatalf("refusal carries AA or data: %v", reply)
	}
	if n := up.queries.Load(); n != 0 {
		t.Fatalf("upstream saw %d queries, want 0", n)
	}
	if n := r.stats.refusedACL.Load(); n != 1 {
		t.Fatalf("refused acl %d, want 1", n)
	}
}

// TestRDZeroRefused proves AC-12: RD=0 is REFUSED whether or not the name
// is cached, and sends nothing upstream.
func TestRDZeroRefused(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	for _, cached := range []bool{false, true} {
		if cached {
			if reply := ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false); reply == nil || reply.Rcode != dns.RcodeSuccess {
				t.Fatalf("priming query: %v", reply)
			}
		}
		before := up.queries.Load()
		q := query("www.example.test", dns.TypeA)
		q.RecursionDesired = false
		reply := ask(r, q, "127.0.0.1", false)
		if reply == nil || reply.Rcode != dns.RcodeRefused || len(reply.Answer) != 0 {
			t.Fatalf("cached=%v: reply %v, want REFUSED with no answer", cached, reply)
		}
		if up.queries.Load() != before {
			t.Fatalf("cached=%v: RD=0 query went upstream", cached)
		}
	}
	if n := r.stats.refusedRD.Load(); n != 2 {
		t.Fatalf("refused rd-clear %d, want 2", n)
	}
}

// TestQDCount proves AC-25: QDCOUNT 0 or 2 is FORMERR with no upstream
// query, whether RD is set or clear. Method: ask each count with RD=1 and
// with RD=0; the RD=0 cases fail when the RD check runs first, since it
// answers REFUSED and counts an RD-clear refusal.
func TestQDCount(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	for _, rd := range []bool{true, false} {
		zero := query("a.test", dns.TypeA)
		zero.Question = nil
		zero.RecursionDesired = rd
		two := query("a.test", dns.TypeA)
		two.Question = append(two.Question, dns.Question{Name: "b.test.", Qtype: dns.TypeA, Qclass: dns.ClassINET})
		two.RecursionDesired = rd
		for name, q := range map[string]*dns.Msg{"zero": zero, "two": two} {
			reply := ask(r, q, "127.0.0.1", false)
			if reply == nil || reply.Rcode != dns.RcodeFormatError {
				t.Fatalf("%s questions, RD=%v: reply %v, want FORMERR", name, rd, reply)
			}
		}
	}
	if n := up.queries.Load(); n != 0 {
		t.Fatalf("upstream saw %d queries, want 0", n)
	}
	if n := r.stats.refusedRD.Load(); n != 0 {
		t.Fatalf("refused rd-clear %d, want 0: a malformed query is not an RD refusal", n)
	}
}

// TestCoalescing proves AC-22: two clients asking the same uncached
// question at once cause one upstream query and both are answered.
func TestCoalescing(t *testing.T) {
	up := aUpstream(t, 200*time.Millisecond)
	r := newTestResolver(t, testConfig(up), newTestClock())
	var wg sync.WaitGroup
	replies := make([]*dns.Msg, 2)
	for i := range replies {
		wg.Go(func() {
			replies[i] = ask(r, query("same.test", dns.TypeA), "127.0.0.1", false)
		})
	}
	wg.Wait()
	for i, reply := range replies {
		if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
			t.Fatalf("client %d: reply %v, want the answer", i, reply)
		}
	}
	if n := up.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries, want 1", n)
	}
}

// TestSlowUpstreamDoesNotBlockCache proves A-8 at the answer function: a
// resolution blocked on a slow upstream does not delay a cached answer.
func TestSlowUpstreamDoesNotBlockCache(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		delay := time.Duration(0)
		if q.Question[0].Name == "slow.test." {
			delay = 400 * time.Millisecond
		}
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1"), delay: delay}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	if reply := ask(r, query("fast.test", dns.TypeA), "127.0.0.1", false); reply == nil {
		t.Fatal("priming query dropped")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ask(r, query("slow.test", dns.TypeA), "127.0.0.1", false)
	}()
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	reply := ask(r, query("fast.test", dns.TypeA), "127.0.0.1", false)
	if took := time.Since(start); took > 100*time.Millisecond {
		t.Fatalf("cached answer took %v while a resolution was in flight", took)
	}
	if reply == nil || len(reply.Answer) != 1 {
		t.Fatalf("cached reply %v", reply)
	}
	<-done
}

// TestOutstandingBound proves AC-23: with limit outstanding 1, a second
// distinct uncached query while the first is in flight is dropped and
// counted, and a cached query still answers.
func TestOutstandingBound(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		delay := time.Duration(0)
		if q.Question[0].Name == "slow.test." {
			delay = 400 * time.Millisecond
		}
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.1"), delay: delay}}
	})
	cfg := testConfig(up)
	cfg.LimitOutstanding = 1
	r := newTestResolver(t, cfg, newTestClock())
	if reply := ask(r, query("cached.test", dns.TypeA), "127.0.0.1", false); reply == nil {
		t.Fatal("priming query dropped")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ask(r, query("slow.test", dns.TypeA), "127.0.0.1", false)
	}()
	time.Sleep(100 * time.Millisecond)
	if reply := ask(r, query("other.test", dns.TypeA), "127.0.0.1", false); reply != nil {
		t.Fatalf("second resolution answered %v, want dropped", reply)
	}
	if n := r.stats.dropped.Load(); n != 1 {
		t.Fatalf("dropped %d, want 1", n)
	}
	// The priming query is the only one answered so far; the dropped one is
	// in dropped_total and not in queries_total.
	if n := r.stats.queries.Load(); n != 1 {
		t.Fatalf("queries counted %d, want 1: only the priming query was answered", n)
	}
	if reply := ask(r, query("cached.test", dns.TypeA), "127.0.0.1", false); reply == nil || len(reply.Answer) != 1 {
		t.Fatalf("cached query while bound reached: %v", reply)
	}
	<-done
}

// TestRateLimit proves AC-24: limit per-client-rate 5 and 50 queries in one
// second from one source answer 5 plus one in ten of the 45 excess (4), the
// rest are dropped and counted, and another source is unaffected.
func TestRateLimit(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	cfg.LimitPerClientRate = 5
	clock := newTestClock()
	r := newTestResolver(t, cfg, clock)
	answered := 0
	for range 50 {
		if ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false) != nil {
			answered++
		}
	}
	if answered != 9 {
		t.Fatalf("answered %d of 50, want 9 (5 + 4)", answered)
	}
	if n := r.stats.rateLimited.Load(); n != 41 {
		t.Fatalf("rate limited %d, want 41", n)
	}
	// queries_total counts answered queries only: a dropped one is in
	// rate_limited_total, never in both.
	if n := r.stats.queries.Load(); n != 9 {
		t.Fatalf("queries counted %d, want the 9 answered", n)
	}
	if ask(r, query("www.example.test", dns.TypeA), "127.0.0.2", false) == nil {
		t.Fatal("another source was rate-limited")
	}
	clock.advance(time.Second)
	if ask(r, query("www.example.test", dns.TypeA), "127.0.0.1", false) == nil {
		t.Fatal("the next second still rate-limited")
	}
}

// TestAllowFromReload proves AC-30 at the query path: a new allow-from
// applies to the next query.
func TestAllowFromReload(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	r := newTestResolver(t, cfg, newTestClock())
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply == nil || reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("before reload: %v", reply)
	}
	cfg.AllowFrom = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	r.apply(&cfg)
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply == nil || reply.Rcode != dns.RcodeRefused {
		t.Fatalf("after reload: %v, want REFUSED", reply)
	}
}
