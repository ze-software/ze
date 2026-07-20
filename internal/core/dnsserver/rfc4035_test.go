// Design: plan/learned/1027-dns-server-harness.md -- the authoritative wrapper
// owns the reply's header shape for every ze DNS server
// RFC: rfc/short/rfc4035.md -- DNSSEC protocol modifications, name-server header
// bits (CD copied from the query, AD ignored on the query and never asserted)

package dnsserver

import (
	"testing"

	"github.com/miekg/dns"
)

// answerOne is a minimal AnswerFunc putting one A record in the Answer section,
// so a test can compare two replies for equality of the answer itself.
func answerOne(msg, r *dns.Msg, _ Peer) bool {
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
		A:   []byte{192, 0, 2, 10},
	})
	return true
}

// VALIDATES: every reply the ze DNS-server harness writes carries the query's CD
// bit verbatim, ignores the query's AD bit, and never asserts AD itself.
// PREVENTS: a CD bit dropped on the floor (which would silently change what a
// downstream validating resolver was asked to do), a query's AD setting being
// echoed back as though ze had authenticated the data, or ze claiming
// authenticated data it never validated.
func TestRFC4035_CDCopiedADIgnored(t *testing.T) {
	handler := Authoritative(answerOne, nil)

	set := new(dns.Msg)
	set.SetQuestion("example.test.", dns.TypeA)
	set.CheckingDisabled = true
	set.AuthenticatedData = true
	fwSet := &fakeResponseWriter{}
	handler(fwSet, set)
	if fwSet.written == nil {
		t.Fatal("no reply written for the CD=1/AD=1 query")
	}

	plainQ := new(dns.Msg)
	plainQ.SetQuestion("example.test.", dns.TypeA)
	fwClear := &fakeResponseWriter{}
	handler(fwClear, plainQ)
	if fwClear.written == nil {
		t.Fatal("no reply written for the CD=0/AD=0 query")
	}

	// RFC requirement: RFC4035-3-8 positive -- a query with CD set draws a reply
	// with CD set (handler.go msg.SetReply(r) copies the CD bit).
	if !fwSet.written.CheckingDisabled {
		t.Error("reply to a CD=1 query has CD clear; the CD bit must be copied from the query")
	}
	// RFC requirement: RFC4035-3-8 negative -- a query with CD clear draws a reply
	// with CD clear, so the bit is genuinely copied rather than always asserted.
	if fwClear.written.CheckingDisabled {
		t.Error("reply to a CD=0 query has CD set; the CD bit must be copied, not invented")
	}

	// RFC requirement: RFC4035-3-9 positive -- the query's AD setting is ignored:
	// the AD=1 query is answered exactly as the AD=0 query is.
	if len(fwSet.written.Answer) != 1 || len(fwClear.written.Answer) != 1 ||
		fwSet.written.Answer[0].String() != fwClear.written.Answer[0].String() {
		t.Errorf("AD=1 query answered differently from the AD=0 query: %v vs %v",
			fwSet.written.Answer, fwClear.written.Answer)
	}
	// RFC requirement: RFC4035-3-9 negative -- the AD bit of the query is not
	// honored: the reply to an AD=1 query does not come back with AD set.
	if fwSet.written.AuthenticatedData {
		t.Error("reply to an AD=1 query has AD set; the query's AD bit must be ignored, never echoed")
	}
	if fwClear.written.AuthenticatedData {
		t.Error("reply has AD set though ze validated nothing")
	}
}
