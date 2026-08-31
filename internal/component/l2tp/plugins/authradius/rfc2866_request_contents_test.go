// RFC 2866 (RADIUS Accounting) obligations over what one Accounting-Request carries.
//
// VALIDATES: the attribute set of an Accounting-Request -- the four attributes
// Section 4.1 forbids, the NAS identity Section 4.1 requires, the zero-length
// text Section 5 forbids, and the Acct-Session-Id Section 5.5 requires on every
// record of a session.
// PREVENTS: an Accounting-Request that names no NAS, one carrying a credential
// or a reply attribute, one carrying text of length zero, and a Stop record a
// server cannot join to its Start.
//
// ze is the RADIUS accounting client (NAS) and never the accounting server, so
// every requirement here binds the packet ze builds. The producers exercised
// are buildAcctPacket and appendNASIdentity.

package l2tpauthradius

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// attrState is the State attribute type (RFC 2865 Section 5.24). ze neither
// sends nor reads it, so the radius dictionary declares no constant for it and
// this file names the code the table forbids.
const attrState = 24

// acctStatuses is the Acct-Status-Type set ze sends, so a case that must hold
// for every record of a session runs over all three.
var acctStatuses = []uint8{
	radius.AcctStatusStart,
	radius.AcctStatusInterimUpdate,
	radius.AcctStatusStop,
}

// RFC requirement: RFC2866-4.1-2 positive -- no Accounting-Request ze builds
// carries User-Password, CHAP-Password, Reply-Message or State, for any record
// of the session. CHAP-Challenge is checked with them: the Section 5.13 table
// marks it 0 in an Accounting-Request, which the legend reads as MUST NOT be
// present.
func TestRFC2866AcctForbiddenAttributesAbsent(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1", peerAddr: "192.0.2.10"}

	forbidden := []struct {
		name string
		attr uint8
	}{
		{"User-Password", radius.AttrUserPassword},
		{"CHAP-Password", radius.AttrCHAPPassword},
		{"Reply-Message", radius.AttrReplyMessage},
		{"State", attrState},
		{"CHAP-Challenge", radius.AttrCHAPChallenge},
	}

	for _, status := range acctStatuses {
		pkt := acct.buildAcctPacket(sess, "nas1", net.IPv4(198, 51, 100, 7), status, 60)
		for _, f := range forbidden {
			if pkt.FindAttr(f.attr) != nil {
				t.Errorf("status %d: Accounting-Request carries %s, which MUST NOT be present", status, f.name)
			}
		}
	}
}

// RFC requirement: RFC2866-4.1-2 negative -- the absence above is the absence of
// those four attributes and not of every attribute: the same packet carries the
// User-Name, Acct-Session-Id and Acct-Status-Type an Accounting-Request owes, so
// a builder that emitted nothing at all would fail this test.
func TestRFC2866AcctForbiddenAttributesDoNotEmptyTheRequest(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1", peerAddr: "192.0.2.10"}

	pkt := acct.buildAcctPacket(sess, "nas1", net.IPv4(198, 51, 100, 7), radius.AcctStatusStart, 0)
	for _, want := range []struct {
		name string
		attr uint8
	}{
		{"User-Name", radius.AttrUserName},
		{"Acct-Session-Id", radius.AttrAcctSessionID},
		{"Acct-Status-Type", radius.AttrAcctStatusType},
	} {
		if pkt.FindAttr(want.attr) == nil {
			t.Errorf("Accounting-Request is missing %s", want.name)
		}
	}
}

// RFC requirement: RFC2866-4.1-3 positive -- every Accounting-Request names this
// NAS. Both leaves that name it are optional, so the case that matters is the
// configuration that sets neither: the packet still carries a NAS-Identifier,
// and it is text of non-zero length.
func TestRFC2866AcctNASIdentityAlwaysPresent(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1"}

	cases := []struct {
		name       string
		nasID      string
		sourceAddr net.IP
	}{
		{"neither leaf set", "", nil},
		{"nas-identifier only", "lns1", nil},
		{"source-address only", "", net.IPv4(198, 51, 100, 7)},
		{"both leaves set", "lns1", net.IPv4(198, 51, 100, 7)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, status := range acctStatuses {
				pkt := acct.buildAcctPacket(sess, tc.nasID, tc.sourceAddr, status, 0)
				ipAddr := pkt.FindAttr(radius.AttrNASIPAddress)
				nasID := pkt.FindAttr(radius.AttrNASIdentifier)
				if ipAddr == nil && nasID == nil {
					t.Fatalf("status %d: Accounting-Request carries neither NAS-IP-Address nor NAS-Identifier", status)
				}
				if nasID != nil && len(nasID) == 0 {
					t.Fatalf("status %d: NAS-Identifier is text of length zero", status)
				}
			}
		})
	}
}

// RFC requirement: RFC2866-4.1-3 negative -- the fallback fires only where the
// obligation would otherwise break. A configured NAS-Identifier reaches the wire
// unchanged rather than being replaced by the host name, and a configuration
// that names the NAS by address alone is not padded with one.
func TestRFC2866AcctNASIdentityFallbackIsNarrow(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1"}

	configured := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	if got := string(configured.FindAttr(radius.AttrNASIdentifier)); got != "lns1" {
		t.Fatalf("NAS-Identifier: got %q, want %q", got, "lns1")
	}

	byAddress := acct.buildAcctPacket(sess, "", net.IPv4(198, 51, 100, 7), radius.AcctStatusStart, 0)
	if byAddress.FindAttr(radius.AttrNASIPAddress) == nil {
		t.Fatal("a configured source-address MUST reach the wire as NAS-IP-Address")
	}
	if byAddress.FindAttr(radius.AttrNASIdentifier) != nil {
		t.Fatal("NAS-IP-Address already names this NAS, so no NAS-Identifier is invented")
	}
}

// RFC requirement: RFC2866-5-3 positive -- a session that never authenticated
// carries no username, and text of length zero is not sent: the User-Name
// attribute is omitted, and no attribute of the packet is empty.
func TestRFC2866AcctZeroLengthTextOmitted(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "", acctSessID: "1-2-1"}

	for _, status := range acctStatuses {
		pkt := acct.buildAcctPacket(sess, "lns1", nil, status, 0)
		if pkt.FindAttr(radius.AttrUserName) != nil {
			t.Errorf("status %d: a zero-length User-Name MUST be omitted, not sent empty", status)
		}
		for _, attr := range pkt.Attrs {
			if len(attr.Value) == 0 {
				t.Errorf("status %d: attribute type %d is text of length zero", status, attr.Type)
			}
		}
	}
}

// RFC requirement: RFC2866-5-3 negative -- the omission is conditional on the
// text being empty: an authenticated session still carries its User-Name, so a
// builder that dropped the attribute outright would fail here.
func TestRFC2866AcctNonEmptyTextIsSent(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: "1-2-1"}

	pkt := acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0)
	if got := string(pkt.FindAttr(radius.AttrUserName)); got != "alice" {
		t.Fatalf("User-Name: got %q, want %q", got, "alice")
	}
}

// RFC requirement: RFC2866-5.5-2 positive -- the Start, Interim-Update and Stop
// records of one session all carry the same Acct-Session-Id, which is what lets
// a server join them.
func TestRFC2866AcctSessionIDSameAcrossRecords(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{username: "alice", acctSessID: acct.genSessionID(1, 2)}

	start := string(acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStart, 0).FindAttr(radius.AttrAcctSessionID))
	interim := string(acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusInterimUpdate, 30).FindAttr(radius.AttrAcctSessionID))
	stop := string(acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusStop, 60).FindAttr(radius.AttrAcctSessionID))

	if start != stop {
		t.Fatalf("Start and Stop carry different Acct-Session-Id: %q and %q", start, stop)
	}
	if interim != stop {
		t.Fatalf("Interim-Update and Stop carry different Acct-Session-Id: %q and %q", interim, stop)
	}
}

// RFC requirement: RFC2866-5.5-2 negative -- the equality above is a property of
// one session and not of the builder: two sessions of the same NAS carry
// different Acct-Session-Ids, so a builder that wrote one constant id into every
// record would fail here.
func TestRFC2866AcctSessionIDDiffersBetweenSessions(t *testing.T) {
	acct := newRADIUSAcct()
	first := &acctSession{username: "alice", acctSessID: acct.genSessionID(1, 2)}
	second := &acctSession{username: "bob", acctSessID: acct.genSessionID(1, 3)}

	firstID := string(acct.buildAcctPacket(first, "lns1", nil, radius.AcctStatusStart, 0).FindAttr(radius.AttrAcctSessionID))
	secondID := string(acct.buildAcctPacket(second, "lns1", nil, radius.AcctStatusStart, 0).FindAttr(radius.AttrAcctSessionID))
	if firstID == secondID {
		t.Fatalf("two sessions share Acct-Session-Id %q", firstID)
	}
}

// RFC requirement: RFC2866-5.5-3 positive -- every Accounting-Request ze builds
// carries exactly one Acct-Session-Id, and it holds the id the session was
// given.
func TestRFC2866AcctSessionIDPresentOnEveryRequest(t *testing.T) {
	acct := newRADIUSAcct()
	want := acct.genSessionID(4, 5)
	sess := &acctSession{username: "alice", acctSessID: want}

	for _, status := range acctStatuses {
		pkt := acct.buildAcctPacket(sess, "lns1", nil, status, 0)
		vals := pkt.FindAllAttr(radius.AttrAcctSessionID)
		if len(vals) != 1 {
			t.Fatalf("status %d: Acct-Session-Id count: got %d, want 1", status, len(vals))
		}
		if string(vals[0]) != want {
			t.Fatalf("status %d: Acct-Session-Id: got %q, want %q", status, vals[0], want)
		}
	}
}

// RFC requirement: RFC2866-5.5-3 negative -- the id a session accounts under is
// the one genSessionID answered, and it is never empty, so no record can reach a
// server with an Acct-Session-Id a billing system cannot join on.
func TestRFC2866AcctSessionIDNeverEmpty(t *testing.T) {
	acct := newRADIUSAcct()
	for range 64 {
		if got := acct.genSessionID(1, 2); got == "" {
			t.Fatal("genSessionID answered an empty Acct-Session-Id")
		}
	}
}
