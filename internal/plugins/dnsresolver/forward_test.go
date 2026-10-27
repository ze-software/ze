package dnsresolver

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestUpstreamQueryShape proves AC-8: a DO=1 client query goes upstream
// with RD=1, CD clear and an OPT of 1232 with DO clear; the reply carries an
// OPT, AA=0 and AD=0 (RFC 2181 Section 5.4.1, AA=0 on every resolver reply).
func TestUpstreamQueryShape(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	q := query("www.example.test", dns.TypeA)
	q.CheckingDisabled = true
	q.SetEdns0(4096, true)
	reply := ask(r, q, "127.0.0.1", false)
	sent := up.lastQuery()
	if sent == nil {
		t.Fatal("no upstream query")
	}
	opt := sent.IsEdns0()
	if opt == nil || opt.UDPSize() != upstreamUDPSize || opt.Do() {
		t.Fatalf("upstream OPT %v, want size %d with DO clear", opt, upstreamUDPSize)
	}
	if !sent.RecursionDesired || sent.CheckingDisabled {
		t.Fatalf("upstream rd=%v cd=%v, want rd only", sent.RecursionDesired, sent.CheckingDisabled)
	}
	if reply.IsEdns0() == nil {
		t.Fatal("reply has no OPT")
	}
	if reply.Authoritative || reply.AuthenticatedData {
		t.Fatalf("reply aa=%v ad=%v, want both clear", reply.Authoritative, reply.AuthenticatedData)
	}
	if len(reply.Answer) != 1 {
		t.Fatalf("answer %v, want only the upstream's record", reply.Answer)
	}
}

// TestReplyMatching proves AC-18 and A-4 (RFC 5452 Section 9.1): a reply
// with a mismatched ID, name, class or type, with QR clear, with two
// questions, or from another source port, is ignored and the resolver keeps waiting for the matching one.
func TestReplyMatching(t *testing.T) {
	bad := func(q *dns.Msg, mutate func(*dns.Msg)) *dns.Msg {
		m := answerWith(q, "a.test. 300 IN A 198.51.100.66")
		mutate(m)
		return m
	}
	forged := func(q *dns.Msg) []fakeReply {
		return []fakeReply{
			{msg: bad(q, func(m *dns.Msg) { m.Id++ })},
			{msg: bad(q, func(m *dns.Msg) { m.Question[0].Name = "b.test." })},
			{msg: bad(q, func(m *dns.Msg) { m.Question[0].Qtype = dns.TypeAAAA })},
			{msg: bad(q, func(m *dns.Msg) { m.Question[0].Qclass = dns.ClassCHAOS })},
			// A query, not a reply: QR clear.
			{msg: bad(q, func(m *dns.Msg) { m.Response = false })},
			// QDCOUNT 2, the first question the one asked.
			{msg: bad(q, func(m *dns.Msg) { m.Question = append(m.Question, m.Question[0]) })},
			{msg: bad(q, func(*dns.Msg) {}), fromOther: true},
		}
	}
	up := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply {
		return append(forged(q), fakeReply{msg: answerWith(q, "a.test. 300 IN A 192.0.2.1"), delay: 20 * time.Millisecond})
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("answer %v, want the matching reply's 192.0.2.1", got)
	}

	only := newFakeUpstream(t, func(q *dns.Msg, _ bool) []fakeReply { return forged(q) })
	r = newTestResolver(t, testConfig(only), newTestClock())
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("only forged replies: %v, want SERVFAIL after the timeout", reply)
	}
}

// TestBailiwick proves AC-19: answer-section records off the queried
// name's chain are neither returned nor cached.
func TestBailiwick(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		if q.Question[0].Name == "a.test." {
			return answerWith(q, "a.test. 300 IN A 192.0.2.1", "evil.test. 300 IN A 198.51.100.66")
		}
		return answerWith(q, "evil.test. 300 IN A 192.0.2.99")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.1" {
		t.Fatalf("answer %v, want only a.test's record", got)
	}
	reply = ask(r, query("evil.test", dns.TypeA), "127.0.0.1", false)
	if got := ips(reply.Answer); len(got) != 1 || got[0] != "192.0.2.99" {
		t.Fatalf("evil.test answer %v: the out-of-bailiwick record was cached", got)
	}
}

// TestFailoverAndTimeout proves AC-16, AC-17 and R-7: a silent first
// upstream is passed over for the second within the timeout and counted;
// with every upstream silent the answer is SERVFAIL within the timeout,
// nothing is cached, and the next query asks upstream again.
func TestFailoverAndTimeout(t *testing.T) {
	silent := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	good := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(silent, good), newTestClock())
	start := time.Now()
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 1 {
		t.Fatalf("reply %v, want the second upstream's answer", reply)
	}
	if took := time.Since(start); took > 600*time.Millisecond {
		t.Fatalf("answered after %v, beyond the timeout", took)
	}
	if n := r.stats.upstream[upstreamTimeout].Load(); n != 1 {
		t.Fatalf("timeouts counted %d, want 1", n)
	}

	silent2 := newFakeUpstream(t, func(*dns.Msg, bool) []fakeReply { return nil })
	r = newTestResolver(t, testConfig(silent, silent2), newTestClock())
	start = time.Now()
	reply = ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("all silent: %v, want SERVFAIL", reply)
	}
	if took := time.Since(start); took > 800*time.Millisecond {
		t.Fatalf("SERVFAIL after %v, want within the 600ms timeout", took)
	}
	before := silent.queries.Load()
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if silent.queries.Load() == before {
		t.Fatal("a timeout was cached: the next query did not go upstream")
	}
}

// TestUpstreamFailureRcode proves an upstream SERVFAIL or REFUSED is passed
// over, never returned as the answer while another upstream answers.
func TestUpstreamFailureRcode(t *testing.T) {
	failing := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		m := new(dns.Msg)
		m.SetRcode(q, dns.RcodeServerFailure)
		return m
	})
	good := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(failing, good), newTestClock())
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want the second upstream's answer", reply)
	}
}

// TestTCPRetryOnTC proves AC-15: an upstream reply with TC=1 is asked again
// over TCP and the whole RRset is answered.
func TestTCPRetryOnTC(t *testing.T) {
	up := newFakeUpstream(t, func(q *dns.Msg, tcp bool) []fakeReply {
		if !tcp {
			m := new(dns.Msg)
			m.SetReply(q)
			m.Truncated = true
			return []fakeReply{{msg: m}}
		}
		return []fakeReply{{msg: answerWith(q, "a.test. 300 IN A 192.0.2.1", "a.test. 300 IN A 192.0.2.2", "a.test. 300 IN A 192.0.2.3")}}
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if len(reply.Answer) != 3 {
		t.Fatalf("answer %v, want the 3 records from TCP", reply.Answer)
	}
	if n := up.tcp.Load(); n != 1 {
		t.Fatalf("TCP queries %d, want 1", n)
	}
}

// TestWorkBudget proves AC-11: a CNAME loop, a chain longer than 11
// restarts, and a resolution needing more than 32 upstream queries each end
// in SERVFAIL, and the upstream queries never exceed 32.
func TestWorkBudget(t *testing.T) {
	loop := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "a.test. 300 IN CNAME b.test.", "b.test. 300 IN CNAME a.test.")
	})
	r := newTestResolver(t, testConfig(loop), newTestClock())
	if reply := ask(r, query("a.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("CNAME loop: %v, want SERVFAIL", reply)
	}

	long := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		var rrs []string
		for i := range 13 {
			rrs = append(rrs, "c"+string(rune('a'+i))+".test. 300 IN CNAME c"+string(rune('a'+i+1))+".test.")
		}
		return answerWith(q, rrs...)
	})
	r = newTestResolver(t, testConfig(long), newTestClock())
	if reply := ask(r, query("ca.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("13-hop chain: %v, want SERVFAIL", reply)
	}

	// Seven upstreams refuse and the eighth answers each name with a CNAME
	// to a fresh name and nothing more: every restart costs 8 queries, so the
	// 32-query bound ends the resolution after 4.
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
	if reply := ask(r, query("q.test", dns.TypeA), "127.0.0.1", false); reply.Rcode != dns.RcodeServerFailure {
		t.Fatalf("query budget: %v, want SERVFAIL", reply)
	}
	var total int32
	for _, u := range ups {
		total += u.queries.Load()
	}
	if total > upstreamQueriesMax {
		t.Fatalf("%d upstream queries for one client query, want at most %d", total, upstreamQueriesMax)
	}
}
