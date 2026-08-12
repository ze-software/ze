// Design: docs/architecture/dns/server-harness.md -- shapeAuthoritative owns the
// reply header shape for every Ze DNS server. This file pins the two RFC 1035
// section 4.1.1 header fields it produces, read off the packed octets rather
// than off the struct, because the octets are what a resolver sees.
// RFC: rfc/short/rfc1035.md -- header fields (Z, AA)

package dnsserver

import (
	"testing"

	"github.com/miekg/dns"
)

// Header flag masks, RFC 1035 section 4.1.1. Octet 2 carries QR, OPCODE, AA, TC
// and RD; octet 3 carries RA, the three-bit Z field, and RCODE.
const (
	flagOctetAA = 2
	maskAA      = 0x04
	maskRD      = 0x01

	flagOctetZ = 3
	maskRA     = 0x80
	// maskZReserved is the one bit of RFC 1035's three-bit Z field that is still
	// reserved. RFC 4035 reassigned the other two: 0x20 is AD and 0x10 is CD.
	maskZReserved = 0x40
	maskAD        = 0x20
	maskCD        = 0x10
)

// headerOctets packs msg and returns its first four octets, which hold the ID
// and both flag octets. Asserting on these rather than on the dns.Msg fields is
// what makes the test measure the wire form the obligation is written about.
func headerOctets(t *testing.T, msg *dns.Msg) []byte {
	t.Helper()
	wire, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack reply: %v", err)
	}
	if len(wire) < 12 {
		t.Fatalf("packed reply is %d octets, shorter than the 12-octet header", len(wire))
	}
	return wire[:4]
}

// VALIDATES: the reserved bit of the Z field is zero in the octets Ze writes,
// even when the answer func set it and even when the query carried it. The AD
// bit, which RFC 4035 took from the same field, is never asserted either.
// PREVENTS: an answer func leaking a reserved bit into every reply of a zone.
// The harness exists to make the reply shape an invariant no answer policy can
// break, and Z is part of that shape: a responder that sets it spends the one
// signal a later protocol extension has for detecting an old implementation.
func TestRFC1035_ReservedZFieldIsZero(t *testing.T) {
	t.Parallel()

	// The answer func sets Z and clears nothing else. This is the half that goes
	// red if the harness stops holding the field down, because SetReply never
	// copies Z, so a query alone cannot put it on the wire.
	dirtyQuery := questionFor("zero.test")
	dirty := udpWriter()
	Authoritative(func(msg, r *dns.Msg, p Peer) bool {
		msg.Zero = true
		msg.AuthenticatedData = true
		return true
	}, nil)(dirty, dirtyQuery)
	if dirty.written == nil {
		t.Fatal("no reply written")
	}
	// RFC requirement: RFC1035-4.1.1-1 positive -- "Z               Reserved for
	// future use.  Must be zero in all queries and responses." RFC 1035 carries
	// no capitalised RFC 2119 keyword anywhere, so this quoted sentence is the
	// whole anchor: a grep for MUST finds nothing in the document.
	h := headerOctets(t, dirty.written)
	if h[flagOctetZ]&maskZReserved != 0 {
		t.Errorf("flag octet %d = %#02x, reserved Z bit %#02x set; it must be zero in every response", flagOctetZ, h[flagOctetZ], maskZReserved)
	}
	if h[flagOctetZ]&maskAD != 0 {
		t.Errorf("flag octet %d = %#02x, AD bit set; Ze validates nothing and must never claim authenticated data", flagOctetZ, h[flagOctetZ])
	}
	// Zeroing Z must not blank the neighboring header. A handler that replaced
	// the flag octets wholesale would pass the two assertions above and fail
	// this one.
	if !dirty.written.Authoritative {
		t.Error("holding Z down cleared the authoritative bit")
	}
	if dirty.written.Id != dirtyQuery.Id {
		t.Errorf("reply ID = %d, want the query's %d; the header was rewritten rather than shaped", dirty.written.Id, dirtyQuery.Id)
	}

	// A query that arrives with Z set draws a reply with Z clear: the field is
	// not echoed. CD, which RFC 4035 took from the same three bits, IS echoed,
	// which is what shows the reply header is copied selectively rather than
	// zeroed wholesale.
	q := questionFor("echo.test")
	q.Zero = true
	q.CheckingDisabled = true
	echoed := udpWriter()
	Authoritative(func(msg, r *dns.Msg, p Peer) bool { return true }, nil)(echoed, q)
	if echoed.written == nil {
		t.Fatal("no reply written for the Z=1 query")
	}
	// RFC requirement: RFC1035-4.1.1-1 negative -- "Must be zero in all queries
	// and responses" binds the response independently of the query, so a query
	// that arrives with the bit set is answered with it clear rather than echoed.
	e := headerOctets(t, echoed.written)
	if e[flagOctetZ]&maskZReserved != 0 {
		t.Errorf("flag octet %d = %#02x, the query's Z bit was echoed into the reply", flagOctetZ, e[flagOctetZ])
	}
	if e[flagOctetZ]&maskCD == 0 {
		t.Error("the query's CD bit was dropped, so this reply header was blanked rather than shaped (RFC 4035)")
	}
}

// VALIDATES: every reply carries AA set, including one an answer func tried to
// leave non-authoritative, while the bits around it still track the query. RD
// is copied and RA stays clear, so the harness is shaping named fields rather
// than setting every flag it can reach.
// PREVENTS: an answer policy emitting a reply that does not claim authority for
// the zone Ze serves. A resolver reading AA=0 from an authoritative server
// treats the answer as a referral or a cached copy and may re-query elsewhere.
func TestRFC1035_AuthoritativeAnswerBitOnEveryReply(t *testing.T) {
	t.Parallel()

	// An answer func that clears AA and advertises recursion. Both are
	// re-asserted after it runs.
	w := udpWriter()
	rdQuery := questionFor("aa.test")
	rdQuery.RecursionDesired = true
	Authoritative(func(msg, r *dns.Msg, p Peer) bool {
		msg.Authoritative = false
		msg.RecursionAvailable = true
		return true
	}, nil)(w, rdQuery)
	if w.written == nil {
		t.Fatal("no reply written")
	}
	// RFC requirement: RFC1035-4.1.1-2 positive -- "AA              Authoritative
	// Answer - this bit is valid in responses, and specifies that the responding
	// name server is an authority for the domain name in question section."
	h := headerOctets(t, w.written)
	if h[flagOctetAA]&maskAA == 0 {
		t.Errorf("flag octet %d = %#02x, AA clear; a reply from the zone's authority must set it", flagOctetAA, h[flagOctetAA])
	}
	if h[flagOctetZ]&maskRA != 0 {
		t.Errorf("flag octet %d = %#02x, RA set; Ze serves no recursion and must not advertise it", flagOctetZ, h[flagOctetZ])
	}
	if h[flagOctetAA]&maskRD == 0 {
		t.Error("RD was dropped from a query that set it, so the flag octet was written wholesale rather than shaped")
	}

	// The same handler answering a query with RD clear. AA is set in both, RD in
	// neither: two inputs, two different flag octets, which is what stops the
	// assertions above passing against a hard-coded octet. dns.SetQuestion turns
	// RD on, so the second query clears it explicitly.
	noRD := questionFor("aa.test")
	noRD.RecursionDesired = false
	plain := udpWriter()
	Authoritative(func(msg, r *dns.Msg, p Peer) bool { return true }, nil)(plain, noRD)
	if plain.written == nil {
		t.Fatal("no reply written for the RD=0 query")
	}
	// RFC requirement: RFC1035-4.1.1-2 negative -- "this bit is valid in
	// responses" scopes AA to the AA position alone. The neighboring flags are
	// not swept along with it: RD still tracks the query, and this reply's flag
	// octet differs from the one above because of it.
	p := headerOctets(t, plain.written)
	if p[flagOctetAA]&maskAA == 0 {
		t.Errorf("flag octet %d = %#02x, AA clear on the second reply", flagOctetAA, p[flagOctetAA])
	}
	if p[flagOctetAA]&maskRD != 0 {
		t.Errorf("flag octet %d = %#02x, RD set though the query cleared it", flagOctetAA, p[flagOctetAA])
	}
	if p[flagOctetAA] == h[flagOctetAA] {
		t.Errorf("both replies carry flag octet %#02x, so the octet does not track the query and this test proved nothing", p[flagOctetAA])
	}
}
