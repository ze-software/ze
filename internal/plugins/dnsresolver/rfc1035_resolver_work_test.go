// Design: docs/architecture/dns/lan-resolver.md -- RFC 1035 Section 7: the resolver's requests, work bound, upstream timeouts and reply matching

package dnsresolver

import (
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestRFC1035ResolverMultiplexesRequests proves RFC 1035 Section 7.1,
// "Since a resolver must be able to multiplex multiple requests if it is to
// perform its function efficiently, each pending request is usually
// represented in some block of state information.".
//
// Method: one upstream that never answers slow.test and answers fast.test at
// once. While the slow.test request is pending on its upstream, a fast.test
// request is answered, so one pending request does not hold the resolver.
// The negative takes the case just outside the clause: two identical
// questions in flight are one request, not two, so they share one state
// block and the upstream sees one query.
func TestRFC1035ResolverMultiplexesRequests(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		if q.Question[0].Name == "slow.test." {
			return nil
		}
		return []fakeReply{{msg: answerWith(q, q.Question[0].Name+" 300 IN A 192.0.2.7")}}
	})
	cfg := testConfig(up)
	cfg.Timeout = 3 * time.Second
	r := newTestResolver(t, cfg, newTestClock())

	slowDone := make(chan *dns.Msg, 1)
	go func() { slowDone <- ask(r, query("slow.test", dns.TypeA), "127.0.0.1", false) }()
	pendingBy := time.Now().Add(2 * time.Second)
	for up.queries.Load() == 0 {
		if time.Now().After(pendingBy) {
			t.Fatal("the slow.test request never reached the upstream")
		}
		time.Sleep(5 * time.Millisecond)
	}
	fast := ask(r, query("fast.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.1-1 positive -- while a slow.test request is pending on a silent upstream, a fast.test request is answered NOERROR with its own A record 192.0.2.7, and the slow.test request is still unanswered when it is.
	select {
	case reply := <-slowDone:
		t.Fatalf("slow.test answered %v before fast.test: the requests were not multiplexed", reply)
	default:
	}
	if fast == nil || fast.Rcode != dns.RcodeSuccess || len(fast.Answer) != 1 || fast.Answer[0].Header().Name != "fast.test." {
		t.Fatalf("fast.test reply %v, want its own A record while slow.test is pending", fast)
	}
	if slow := <-slowDone; slow == nil || slow.Rcode != dns.RcodeServerFailure {
		t.Fatalf("slow.test reply %v, want SERVFAIL after the timeout", slow)
	}

	same := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return []fakeReply{{msg: answerWith(q, "same.test. 300 IN A 192.0.2.8"), delay: 200 * time.Millisecond}}
	})
	r = newTestResolver(t, testConfig(same), newTestClock())
	var wg sync.WaitGroup
	replies := make([]*dns.Msg, 2)
	for i := range replies {
		wg.Go(func() { replies[i] = ask(r, query("same.test", dns.TypeA), "127.0.0.1", false) })
	}
	wg.Wait()

	// RFC requirement: RFC1035-7.1-1 negative -- two identical same.test questions in flight are one pending request: both clients get the A record 192.0.2.8 and the upstream sees one query.
	for i, reply := range replies {
		if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
			t.Fatalf("client %d: reply %v, want the A record", i, reply)
		}
	}
	if n := same.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries for two identical questions in flight, want 1", n)
	}
}

// TestRFC1035ResolverWorkIsLimited proves RFC 1035 Section 7.1, "The amount
// of work which a resolver will do in response to a client request must be
// limited to guard against errors in the database, such as circular CNAME
// references, and operational problems, such as network partition which
// prevents the resolver from accessing the name servers it needs.".
//
// Method: a CNAME loop, and a database error the loop check cannot see: an
// upstream that answers every name with a CNAME to a fresh name and nothing
// more, behind seven upstreams that refuse. Each ends in SERVFAIL, and the
// second never costs more than the 32 upstream queries the budget allows.
// The negative is work inside the bound: a three-record chain is answered
// in full.
func TestRFC1035ResolverWorkIsLimited(t *testing.T) {
	loop := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "a.test. 300 IN CNAME b.test.", "b.test. 300 IN CNAME a.test.")
	})
	r := newTestResolver(t, testConfig(loop), newTestClock())
	looped := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)

	var ups []*fakeUpstream
	for range 7 {
		ups = append(ups, scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
			m := new(dns.Msg)
			m.SetRcode(q, dns.RcodeRefused)
			return m
		}))
	}
	ups = append(ups, scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		name := q.Question[0].Name
		return answerWith(q, name+" 300 IN CNAME x"+name)
	}))
	cfg := testConfig(ups...)
	cfg.Timeout = 5 * time.Second
	r = newTestResolver(t, cfg, newTestClock())
	endless := ask(r, query("q.test", dns.TypeA), "127.0.0.1", false)
	var total int32
	for _, u := range ups {
		total += u.queries.Load()
	}

	// RFC requirement: RFC1035-7.1-2 positive -- a circular CNAME reference ends in SERVFAIL, and a chain of fresh CNAMEs behind seven refusing upstreams ends in SERVFAIL after at most 32 upstream queries.
	if looped == nil || looped.Rcode != dns.RcodeServerFailure {
		t.Fatalf("CNAME loop: %v, want SERVFAIL", looped)
	}
	if endless == nil || endless.Rcode != dns.RcodeServerFailure {
		t.Fatalf("endless chain: %v, want SERVFAIL", endless)
	}
	if total > upstreamQueriesMax {
		t.Fatalf("%d upstream queries for one client request, want at most %d", total, upstreamQueriesMax)
	}

	chain := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "c1.test. 300 IN CNAME c2.test.", "c2.test. 300 IN CNAME c3.test.", "c3.test. 300 IN A 192.0.2.3")
	})
	r = newTestResolver(t, testConfig(chain), newTestClock())
	reply := ask(r, query("c1.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.1-2 negative -- the limit leaves work inside it alone: the two-alias chain c1.test to c3.test is answered NOERROR with both CNAMEs and c3.test A 192.0.2.3.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 3 {
		t.Fatalf("three-record chain: %v, want NOERROR with all three records", reply)
	}
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.3" {
		t.Fatalf("chain answer addresses %v, want [192.0.2.3]", got)
	}
}

// TestRFC1035AddressWithoutHistoryGetsAShare proves RFC 1035 Section 7.2,
// "Part of this step must deal with addresses which have no such history;
// in this case an expected round trip time of 5-10 seconds should be the
// worst case, with lower estimates for the same local network, etc.".
//
// The forwarder keeps no response-time history, so every upstream address
// is one with none, and forward deals with each by giving the attempt an
// equal share of the time left (the deadline split over the addresses still
// to try). Method: two upstreams under a 2 second timeout. A silent first
// address is abandoned at its share, in time for the second to answer. The
// negative: a first address that answers within its share, 300 ms here, is
// the answer, and the second is never asked.
func TestRFC1035AddressWithoutHistoryGetsAShare(t *testing.T) {
	silent := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	second := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg { return answerWith(q, "h.test. 300 IN A 192.0.2.2") })
	cfg := testConfig(silent, second)
	cfg.Timeout = 2 * time.Second
	r := newTestResolver(t, cfg, newTestClock())
	reply := ask(r, query("h.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.2-1 positive -- with no history for either of two upstreams, the silent first is given a share of the 2 second timeout and abandoned within it, so the second answers: the reply is NOERROR with its A record 192.0.2.2.
	if reply == nil || reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want the second upstream's answer", reply)
	}
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.2" {
		t.Fatalf("answer %v, want the second upstream's 192.0.2.2", got)
	}

	slowFirst := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return []fakeReply{{msg: answerWith(q, "h.test. 300 IN A 192.0.2.1"), delay: 300 * time.Millisecond}}
	})
	unasked := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg { return answerWith(q, "h.test. 300 IN A 192.0.2.2") })
	cfg = testConfig(slowFirst, unasked)
	cfg.Timeout = 2 * time.Second
	r = newTestResolver(t, cfg, newTestClock())
	reply = ask(r, query("h.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.2-1 negative -- a first upstream answering after 300 ms, inside its share of the 2 second timeout, is the answer: the reply carries its 192.0.2.1 and the second upstream sees no query.
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("answer %v, want the first upstream's 192.0.2.1", got)
	}
	if n := unasked.queries.Load(); n != 0 {
		t.Fatalf("second upstream saw %d queries, want 0: the first answered within its share", n)
	}
}

// TestRFC1035ResponseMatchedToItsTransmission proves RFC 1035 Section 7.3,
// "However, if it is using the response to sample the round trip time to
// access the name server, it must be able to determine which transmission
// matches the response (and keep transmission times for each outgoing
// message), or only calculate round trip times based on initial
// transmissions.", in its first alternative: the forwarder tells which
// transmission a response answers, by the ID each transmission carries.
//
// Method: an upstream that first sends a reply to the same question with
// another transmission's ID, then the reply bearing this one's. Only the
// second is taken. The negative: the reply bearing the transmission's own
// ID, sent alone, is taken at once.
func TestRFC1035ResponseMatchedToItsTransmission(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		other := answerWith(q, "m.test. 300 IN A 198.51.100.66")
		other.Id = q.Id + 1
		return []fakeReply{
			{msg: other},
			{msg: answerWith(q, "m.test. 300 IN A 192.0.2.1"), delay: 20 * time.Millisecond},
		}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("m.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.3-1 positive -- a reply to m.test carrying another transmission's ID (198.51.100.66) is not taken as the response; the reply bearing this transmission's ID is, so the answer is 192.0.2.1.
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("answer %v, want 192.0.2.1 from the reply that matches the transmission", got)
	}

	own := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg { return answerWith(q, "m.test. 300 IN A 192.0.2.1") })
	r = newTestResolver(t, testConfig(own), newTestClock())
	reply = ask(r, query("m.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-7.3-1 negative -- the reply bearing the transmission's own ID, sent alone, is taken: the answer is NOERROR with 192.0.2.1.
	if reply == nil || reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want NOERROR", reply)
	}
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("answer %v, want 192.0.2.1", got)
	}
}
