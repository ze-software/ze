// Design: docs/architecture/dns/lan-resolver.md -- RFC 1035 Section 4.2.1: retransmitting a lost UDP query

package dnsresolver

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestRFC1035LostQueryIsRetransmitted proves RFC 1035 Section 4.2.1,
// "Queries sent using UDP may be lost, and hence a retransmission strategy
// is required.".
//
// Method: one upstream under a 4 second timeout, which holds two attempts of
// the 2 second minimum retransmission interval. The upstream drops the first
// query and answers the next: the resolver sends the question again and
// answers with it. The negative takes the case just outside the clause, a
// query that is answered: an upstream that answers the first query sees one
// query and no retransmission.
func TestRFC1035LostQueryIsRetransmitted(t *testing.T) {
	var asked atomic.Int32
	lossy := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		if asked.Add(1) == 1 {
			return nil
		}
		return []fakeReply{{msg: answerWith(q, "lost.test. 300 IN A 192.0.2.4")}}
	})
	cfg := testConfig(lossy)
	cfg.Timeout = 4 * time.Second
	r := newTestResolver(t, cfg, newTestClock())
	reply := ask(r, query("lost.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-4.2.1-4 positive -- a single upstream that drops the first UDP query for lost.test is sent it again, and the reply is NOERROR with its A record 192.0.2.4 after two upstream queries.
	if reply == nil || reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want NOERROR from the retransmitted query", reply)
	}
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.4" {
		t.Fatalf("answer %v, want 192.0.2.4", got)
	}
	if n := lossy.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: the lost one and its retransmission", n)
	}

	prompt := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg { return answerWith(q, "kept.test. 300 IN A 192.0.2.5") })
	cfg = testConfig(prompt)
	cfg.Timeout = 4 * time.Second
	r = newTestResolver(t, cfg, newTestClock())
	reply = ask(r, query("kept.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-4.2.1-4 negative -- a query the upstream answers is not retransmitted: kept.test is answered NOERROR with 192.0.2.5 and the upstream sees one query.
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.5" {
		t.Fatalf("answer %v, want 192.0.2.5", got)
	}
	if n := prompt.queries.Load(); n != 1 {
		t.Fatalf("upstream saw %d queries for an answered question, want 1", n)
	}
}

// TestRFC1035RetransmissionIsBounded proves the bound on the retransmission
// strategy of RFC 1035 Section 4.2.1, "Queries sent using UDP may be lost,
// and hence a retransmission strategy is required.", read with its
// "Depending on how well connected the client is to its expected servers,
// the minimum retransmission interval should be 2-5 seconds.": the resend
// stops at the deadline, never more often than every 2 seconds.
//
// Method: a silent single upstream. Under a 4 second timeout it is asked
// twice, 2 seconds apart, and the client gets SERVFAIL within the timeout.
// Under a 1 second timeout, shorter than one minimum interval, it is asked
// once.
func TestRFC1035RetransmissionIsBounded(t *testing.T) {
	silent := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	cfg := testConfig(silent)
	cfg.Timeout = 4 * time.Second
	r := newTestResolver(t, cfg, newTestClock())
	start := time.Now()
	reply := ask(r, query("gone.test", dns.TypeA), "127.0.0.1", false)
	took := time.Since(start)
	sent := silent.queries.Load()

	short := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	cfg = testConfig(short)
	cfg.Timeout = time.Second
	r = newTestResolver(t, cfg, newTestClock())
	shortReply := ask(r, query("gone.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC1035-4.2.1-4 negative -- retransmission is bounded by the deadline at the 2 second minimum interval: a silent upstream is asked exactly twice under a 4 second timeout and the client gets SERVFAIL within it, and exactly once under a 1 second timeout, which also ends in SERVFAIL.
	if reply == nil || reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("4s timeout: reply %v, want SERVFAIL", reply)
	}
	if sent != 2 {
		t.Fatalf("4s timeout: upstream saw %d queries, want 2", sent)
	}
	if took > 5*time.Second {
		t.Fatalf("4s timeout: SERVFAIL after %v, want within the timeout", took)
	}
	if shortReply == nil || shortReply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("1s timeout: reply %v, want SERVFAIL", shortReply)
	}
	if n := short.queries.Load(); n != 1 {
		t.Fatalf("1s timeout: upstream saw %d queries, want 1", n)
	}
}
