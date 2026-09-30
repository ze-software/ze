// Design: docs/guide/l2tp.md -- RADIUS accounting
// Related: acct.go -- buildAcctPacket, which builds every Accounting-Request
// Related: exclude.go -- parseAttributeExclusions, the only operator control
//   over which attributes an Accounting-Request carries
// RFC: rfc/short/rfc2866.md -- Section 5.13
//
// VALIDATES: the two obligation-bearing lines of the RFC 2866 Section 5.13
// legend, over the Accounting-Request table. "1 Exactly one instance of this
// attribute MUST be present." binds the table's two 1 rows, Acct-Status-Type
// and Acct-Session-Id. "0 This attribute MUST NOT be present" binds its five 0
// rows: User-Password, CHAP-Password, Reply-Message, State and CHAP-Challenge.
// The 0+ and 0-1 lines are permissions and oblige nothing.
// METHOD: every packet is encoded to its wire form and decoded again, so the
// assertion reads what a server receives, not the builder's attribute list.
// PREVENTS: an operator exclusion that removes a mandatory attribute, and
// subscriber-supplied text that smuggles a forbidden attribute onto the wire.

package l2tpauthradius

import (
	"net"
	"slices"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// zeroMarkedAttrs are the attributes the Section 5.13 table marks 0 for an
// Accounting-Request.
var zeroMarkedAttrs = []struct {
	name string
	attr uint8
}{
	{"User-Password", radius.AttrUserPassword},
	{"CHAP-Password", radius.AttrCHAPPassword},
	{"Reply-Message", radius.AttrReplyMessage},
	{"State", attrState},
	{"CHAP-Challenge", radius.AttrCHAPChallenge},
}

// oneMarkedAttrs are the attributes the Section 5.13 table marks 1.
var oneMarkedAttrs = []struct {
	name string
	attr uint8
}{
	{"Acct-Status-Type", radius.AttrAcctStatusType},
	{"Acct-Session-Id", radius.AttrAcctSessionID},
}

// onTheWire encodes pkt as ze sends it and decodes the octets back, so a test
// reads the attributes a server would parse.
func onTheWire(t *testing.T, pkt *radius.Packet) *radius.Packet {
	t.Helper()
	buf := make([]byte, radius.MaxPacketLen)
	n, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("encode Accounting-Request: %v", err)
	}
	decoded, err := radius.Decode(buf[:n])
	if err != nil {
		t.Fatalf("decode Accounting-Request: %v", err)
	}
	return decoded
}

// assertOneMarkedExactlyOnce fails unless decoded carries exactly one instance
// of each 1-marked attribute, and that instance is not empty.
func assertOneMarkedExactlyOnce(t *testing.T, where string, decoded *radius.Packet) {
	t.Helper()
	for _, one := range oneMarkedAttrs {
		vals := decoded.FindAllAttr(one.attr)
		if len(vals) != 1 {
			t.Errorf("%s: %s count %d, want exactly 1", where, one.name, len(vals))
			continue
		}
		if len(vals[0]) == 0 {
			t.Errorf("%s: %s is empty", where, one.name)
		}
	}
}

// TestRFC2866EveryRecordCarriesOneOfEachOneMarkedAttribute is the conforming
// side of the 1 legend line.
//
// VALIDATES: the Start, Interim-Update and Stop records of one session each
// carry exactly one Acct-Status-Type and exactly one Acct-Session-Id on the
// wire, and the session id is the same one in every record.
// PREVENTS: a record that loses or duplicates either attribute between the
// builder and the socket.
//
// RFC requirement: RFC2866-5-1 positive -- every Accounting-Request ze sends
// carries exactly one Acct-Status-Type and exactly one Acct-Session-Id
// (acct.go buildAcctPacket).
func TestRFC2866EveryRecordCarriesOneOfEachOneMarkedAttribute(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "192.0.2.10"}

	for _, status := range acctStatuses {
		decoded := onTheWire(t, acct.buildAcctPacket(sess, "nas1", net.IPv4(198, 51, 100, 7), status, 60))
		assertOneMarkedExactlyOnce(t, "status "+strconv.Itoa(int(status)), decoded)
		if got := string(decoded.FindAttr(radius.AttrAcctSessionID)); got != "1-2-1" {
			t.Errorf("status %d: Acct-Session-Id %q, want the session's 1-2-1", status, got)
		}
	}
}

// TestRFC2866ExclusionsCannotRemoveAOneMarkedAttribute is the refusal side of
// the 1 legend line.
//
// VALIDATES: the `attributes exclude` container refuses both 1-marked names,
// and an operator who excludes every name it does accept, from every packet
// kind, still sends exactly one Acct-Status-Type and one Acct-Session-Id in
// each record.
// METHOD: the excluded set is every key of excludableAttributes, so a name
// added to that map later is covered without an edit here.
// PREVENTS: an operator control that turns a mandatory attribute off.
//
// RFC requirement: RFC2866-5-1 negative -- an exclusion naming Acct-Status-Type
// or Acct-Session-Id is refused, and the widest accepted exclusion leaves both
// present exactly once (exclude.go parseAttributeExclusions).
func TestRFC2866ExclusionsCannotRemoveAOneMarkedAttribute(t *testing.T) {
	for _, name := range []string{"acct-status-type", "acct-session-id"} {
		block := map[string]any{"attributes": map[string]any{"exclude": map[string]any{name: "true"}}}
		if _, err := parseAttributeExclusions(block); err == nil {
			t.Errorf("attributes exclude %s was accepted; RFC 2866 Section 5.13 makes it mandatory", name)
		}
	}

	everything := map[string]any{}
	for name := range excludableAttributes {
		everything[name] = "true"
	}
	exclusions, err := parseAttributeExclusions(map[string]any{"attributes": map[string]any{"exclude": everything}})
	if err != nil {
		t.Fatalf("excluding every accepted name: %v", err)
	}
	if len(exclusions) != len(excludableAttributes) {
		t.Fatalf("parsed %d exclusion(s), want %d", len(exclusions), len(excludableAttributes))
	}

	acct := newRADIUSAcct()
	acct.setExclusions(exclusions)
	sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "192.0.2.10"}
	for _, status := range acctStatuses {
		pkt := acct.buildAcctPacket(sess, "nas1", net.IPv4(198, 51, 100, 7), status, 60)
		assertOneMarkedExactlyOnce(t, "all exclusions, status "+strconv.Itoa(int(status)), onTheWire(t, pkt))
	}
}

// TestRFC2866AccountingRequestOnTheWireCarriesNoZeroMarkedAttribute is the
// conforming side of the 0 legend line.
//
// VALIDATES: the Start, Interim-Update and Stop records of a session, as a
// server decodes them, carry none of the five 0-marked attributes, while the
// records are not empty.
// PREVENTS: a credential, a reply text or a challenge state copied into
// accounting.
//
// RFC requirement: RFC2866-5.13-1 positive -- no Accounting-Request ze sends
// carries User-Password, CHAP-Password, Reply-Message, State or CHAP-Challenge
// (acct.go buildAcctPacket).
func TestRFC2866AccountingRequestOnTheWireCarriesNoZeroMarkedAttribute(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1",
		peerAddr: "192.0.2.10", callingStationID: "02:00:00:00:00:01", nasPortID: "lns1:1:2",
	}

	for _, status := range acctStatuses {
		decoded := onTheWire(t, acct.buildAcctPacket(sess, "nas1", net.IPv4(198, 51, 100, 7), status, 60))
		if len(decoded.Attrs) == 0 {
			t.Fatalf("status %d: the Accounting-Request carries no attribute", status)
		}
		for _, zero := range zeroMarkedAttrs {
			if decoded.FindAttr(zero.attr) != nil {
				t.Errorf("status %d: Accounting-Request carries %s, which MUST NOT be present", status, zero.name)
			}
		}
	}
}

// TestRFC2866SubscriberTextNeverBecomesAZeroMarkedAttribute is the refusal
// side of the 0 legend line.
//
// VALIDATES: text the subscriber or the operator chooses (User-Name,
// Calling-Station-Id, NAS-Port-Id, NAS-Identifier) that is itself the octets of
// a 0-marked attribute stays inside its own attribute's value on the wire: the
// decoded request holds none of the five, and each smuggled value comes back
// whole in the attribute that carried it.
// METHOD: each value is a well-formed type-length-value of a forbidden
// attribute, the shape that would appear on the wire if the builder or the
// encoder copied text without its own header.
// PREVENTS: a forbidden attribute reaching the server through text ze relays.
//
// RFC requirement: RFC2866-5.13-1 negative -- attribute-shaped text naming
// User-Password, CHAP-Password, Reply-Message, State or CHAP-Challenge in every
// relayed text field never yields one of them in the Accounting-Request
// (acct.go buildAcctPacket, radius Packet.EncodeTo).
func TestRFC2866SubscriberTextNeverBecomesAZeroMarkedAttribute(t *testing.T) {
	for _, zero := range zeroMarkedAttrs {
		smuggled := string([]byte{zero.attr, 6, 'x', 'y', 'z', 'w'})
		acct := newRADIUSAcct()
		sess := &acctSession{
			tunnelID: 1, sessionID: 2, username: smuggled, acctSessID: "1-2-1",
			peerAddr: "192.0.2.10", callingStationID: smuggled, nasPortID: smuggled,
		}
		for _, status := range acctStatuses {
			decoded := onTheWire(t, acct.buildAcctPacket(sess, smuggled, nil, status, 60))
			if decoded.FindAttr(zero.attr) != nil {
				t.Errorf("%s status %d: relayed text became a %s attribute", zero.name, status, zero.name)
			}
			for _, carrier := range []uint8{
				radius.AttrUserName, radius.AttrCallingStationID, radius.AttrNASPortID, radius.AttrNASIdentifier,
			} {
				vals := decoded.FindAllAttr(carrier)
				if !slices.ContainsFunc(vals, func(v []byte) bool { return string(v) == smuggled }) {
					t.Errorf("%s status %d: attribute %d does not carry the relayed text whole", zero.name, status, carrier)
				}
			}
		}
	}
}
