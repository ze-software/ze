// Design: docs/architecture/dns/lan-resolver.md -- RFC 2181 Section 10: an alias's label and its value on the chain walk

package dnsresolver

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC2181AliasLabelAndValueKeptApart proves RFC 2181 Section 10.1.1,
// "Care must therefore be taken to be very clear whether the label, or the
// value (the canonical name) of a CNAME resource record is intended.", on
// the chain walk the forwarder runs (absorb).
//
// Method: an upstream answers an A query for www.test with www.test's CNAME
// to host.test and host.test's A record. The walk must look the alias up by
// its label (www.test, the name asked) and continue at its value (host.test,
// the canonical name), so the client gets both records and the A record is
// the one host.test owns. The negative turns the record round: the reply
// carries host.test CNAME www.test, whose label is host.test and whose value
// is the name asked. www.test is then a canonical name, not an alias, so the
// walk follows nothing: the answer is NOERROR with an empty Answer section
// and no second upstream query.
func TestRFC2181AliasLabelAndValueKeptApart(t *testing.T) {
	forward := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "www.test. 300 IN CNAME host.test.", "host.test. 300 IN A 192.0.2.1")
	})
	r := newTestResolver(t, testConfig(forward), newTestClock())
	reply := ask(r, query("www.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC2181-10.1.1-1 positive -- for an A query of www.test, the alias is the record owned by the label www.test and the walk goes on at its value host.test: the reply is NOERROR with exactly www.test CNAME host.test then host.test A 192.0.2.1.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 2 {
		t.Fatalf("reply %v, want NOERROR with the CNAME and the A record", reply)
	}
	alias, ok := reply.Answer[0].(*dns.CNAME)
	if !ok || alias.Hdr.Name != "www.test." || alias.Target != "host.test." {
		t.Errorf("first answer %v, want www.test. CNAME host.test.", reply.Answer[0])
	}
	a, ok := reply.Answer[1].(*dns.A)
	if !ok || a.Hdr.Name != "host.test." || a.A.String() != "192.0.2.1" {
		t.Errorf("second answer %v, want host.test. A 192.0.2.1, owned by the CNAME's value", reply.Answer[1])
	}

	reversed := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q, "host.test. 300 IN CNAME www.test.", "host.test. 300 IN A 192.0.2.1")
	})
	r = newTestResolver(t, testConfig(reversed), newTestClock())
	reply = ask(r, query("www.test", dns.TypeA), "127.0.0.1", false)

	// RFC requirement: RFC2181-10.1.1-1 negative -- a CNAME whose value, not its label, is the name asked does not make that name an alias: the A query of www.test against host.test CNAME www.test is answered NOERROR with an empty Answer section after one upstream query.
	if reply == nil || reply.Rcode != dns.RcodeSuccess {
		t.Fatalf("reply %v, want NOERROR", reply)
	}
	if len(reply.Answer) != 0 {
		t.Errorf("answer %v, want none: www.test is the CNAME's value, not its label", reply.Answer)
	}
	if n := reversed.queries.Load(); n != 1 {
		t.Errorf("upstream saw %d queries, want 1: nothing to follow", n)
	}
}

// TestRFC2181PTRLookupMayMeetAnAlias proves the resolver half of RFC 2181
// Section 10.2, "Note that while the value of a PTR record must not be an
// alias, there is no requirement that the process of resolving a PTR record
// not encounter any aliases.": the label looked up for a PTR may be an
// alias, and the PTR found at its canonical name is the answer.
//
// Method: a PTR query for 1.2.0.192.in-addr.arpa, which an RFC 2317
// classless delegation makes an alias of 1.0-25.2.0.192.in-addr.arpa, and
// an upstream that answers with that CNAME and the PTR at its target. The
// alias on the way is followed, never refused, and the reply carries both
// records. This is the case just outside the clause: the alias stands at the
// looked-up label, not in the PTR's value.
func TestRFC2181PTRLookupMayMeetAnAlias(t *testing.T) {
	up := scriptedUpstream(t, func(q *dns.Msg) *dns.Msg {
		return answerWith(q,
			"1.2.0.192.in-addr.arpa. 300 IN CNAME 1.0-25.2.0.192.in-addr.arpa.",
			"1.0-25.2.0.192.in-addr.arpa. 300 IN PTR host.example.")
	})
	r := newTestResolver(t, testConfig(up), newTestClock())
	reply := ask(r, query("1.2.0.192.in-addr.arpa", dns.TypePTR), "127.0.0.1", false)

	// RFC requirement: RFC2181-10.2-1 negative -- a PTR lookup whose label is an alias is followed to its canonical name: the PTR query of 1.2.0.192.in-addr.arpa is answered NOERROR with the CNAME to 1.0-25.2.0.192.in-addr.arpa and the PTR host.example. found there.
	if reply == nil || reply.Rcode != dns.RcodeSuccess || len(reply.Answer) != 2 {
		t.Fatalf("reply %v, want NOERROR with the CNAME and the PTR", reply)
	}
	if _, ok := reply.Answer[0].(*dns.CNAME); !ok {
		t.Errorf("first answer %v, want the CNAME met on the way", reply.Answer[0])
	}
	ptr, ok := reply.Answer[1].(*dns.PTR)
	if !ok || ptr.Hdr.Name != "1.0-25.2.0.192.in-addr.arpa." || ptr.Ptr != "host.example." {
		t.Errorf("second answer %v, want 1.0-25.2.0.192.in-addr.arpa. PTR host.example.", reply.Answer[1])
	}
}
