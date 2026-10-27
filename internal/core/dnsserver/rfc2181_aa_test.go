// Design: docs/architecture/dns/server-harness.md -- shapeAuthoritative sets AA
// on every answer an authoritative Ze DNS server gives.
// RFC: rfc/short/rfc2181.md -- RFC2181-5.4.1-9, the AA bit set correctly

package dnsserver

import (
	"testing"

	"github.com/miekg/dns"
)

// TestRFC2181AuthoritativeReplySetsAA proves the positive side of RFC 2181
// Section 5.4.1's AA rule: a Ze server that is the authority for the name it
// answers sets AA on the reply, whatever its answer function left in the
// header. Method: an authoritative handler whose answer function clears AA
// answers one query, and the packed header octets are read.
func TestRFC2181AuthoritativeReplySetsAA(t *testing.T) {
	t.Parallel()

	w := udpWriter()
	Authoritative(nil, func(msg, r *dns.Msg, p Peer) bool {
		msg.Authoritative = false
		return true
	})(w, questionFor("aa.test"))
	if w.written == nil {
		t.Fatal("no reply written")
	}
	// RFC requirement: RFC2181-5.4.1-9 positive -- an authoritative handler
	// whose answer function cleared AA sends its answer with the AA bit set in
	// the packed header.
	h := headerOctets(t, w.written)
	if h[flagOctetAA]&maskAA == 0 {
		t.Errorf("flag octet %d = %#02x, AA clear; an answer from the name's authority must set it", flagOctetAA, h[flagOctetAA])
	}
}
