// Related: extract.go -- extractAuthMetadata, which reads the Access-Accept's
//   Framed-IP-Address
// Related: acct.go -- buildAcctPacket, which reports the address in accounting
// RFC: rfc/short/rfc2866.md -- Section 4.1
//
// VALIDATES: the second clause of RFC 2866 Section 4.1. RFC 2865 Section 5.8
// gives Framed-IP-Address two special values: 0xFFFFFFFF, "the NAS Should allow
// the user to select an address (e.g. Negotiated)", and 0xFFFFFFFE, "the NAS
// should select an address for the user (e.g. Assigned from a pool of addresses
// kept by the NAS)". An Access-Accept carrying either one names no address, so
// the address the Accounting-Request reports is the one the NAS then assigned.
// PREVENTS: a special value taken as the literal subscriber address, which the
// pool then pins (l2tp/plugins/pool register.go) and accounting reports.

package l2tpauthradius

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// specialFramedIPValues are the two RFC 2865 Section 5.8 values that ask the
// NAS to assign or negotiate the address.
var specialFramedIPValues = [][]byte{
	{0xff, 0xff, 0xff, 0xfe},
	{0xff, 0xff, 0xff, 0xff},
}

// TestRFC2866AccountingReportsTheAddressTheNASAssigned is the conforming side.
//
// VALIDATES: an Access-Accept carrying a special Framed-IP-Address and a
// Framed-Pool leaves the address to the NAS's pool, and the Start, Interim and
// Stop records then carry the address the session was actually assigned.
// PREVENTS: an Accept that asked the NAS to choose producing an accounting
// record that reports anything but the NAS's choice.
//
// RFC requirement: RFC2866-4.1-1 positive -- after an Access-Accept carrying
// 0xFFFFFFFE or 0xFFFFFFFF, the Accounting-Request's Framed-IP-Address is the
// address assigned (extract.go extractAuthMetadata leaves the choice to the
// pool; acct.go buildAcctPacket reports the assignment).
func TestRFC2866AccountingReportsTheAddressTheNASAssigned(t *testing.T) {
	for _, special := range specialFramedIPValues {
		resp := &radius.Packet{Code: radius.CodeAccessAccept, Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: special},
			{Type: radius.AttrFramedPool, Value: []byte("subscribers")},
		}}
		meta := extractAuthMetadata(resp)
		if meta == nil {
			t.Fatalf("%v: the Accept's Framed-Pool was dropped", net.IP(special))
		}
		if meta.FramedPool != "subscribers" {
			t.Fatalf("%v: Framed-Pool = %q, want subscribers", net.IP(special), meta.FramedPool)
		}
		if meta.FramedIP.IsValid() {
			t.Fatalf("%v: the Accept pinned %s; the special value asks the NAS to choose", net.IP(special), meta.FramedIP)
		}

		// The pool chose 100.64.0.9; the reactor reports it on session-ip-assigned.
		acct := newRADIUSAcct()
		sess := &acctSession{tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "100.64.0.9"}
		for _, status := range []uint8{radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop} {
			vals := acct.buildAcctPacket(sess, "lns1", nil, status, 0).FindAllAttr(radius.AttrFramedIPAddress)
			if len(vals) != 1 {
				t.Fatalf("%v status %d: Framed-IP-Address count %d, want 1", net.IP(special), status, len(vals))
			}
			if got := net.IP(vals[0]).String(); got != "100.64.0.9" {
				t.Fatalf("%v status %d: Framed-IP-Address = %s, want the assigned 100.64.0.9", net.IP(special), status, got)
			}
		}
	}
}

// TestRFC2866SpecialFramedIPValueIsNeverTheSubscriberAddress is the refusal.
//
// VALIDATES: neither special value becomes the subscriber's address, with or
// without a Framed-Pool beside it, so neither can reach the pool as a pinned
// address and from there the Accounting-Request.
// PREVENTS: 255.255.255.254 read as an ordinary unicast address.
//
// RFC requirement: RFC2866-4.1-1 negative -- an Access-Accept's special
// Framed-IP-Address is never taken as the user's address (extract.go
// extractAuthMetadata).
func TestRFC2866SpecialFramedIPValueIsNeverTheSubscriberAddress(t *testing.T) {
	for _, special := range specialFramedIPValues {
		alone := extractAuthMetadata(&radius.Packet{Code: radius.CodeAccessAccept, Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: special},
		}})
		if alone != nil && alone.FramedIP.IsValid() {
			t.Errorf("%v alone: pinned %s as the subscriber address", net.IP(special), alone.FramedIP)
		}
		withTimeout := extractAuthMetadata(&radius.Packet{Code: radius.CodeAccessAccept, Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: special},
			{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(3600)},
		}})
		if withTimeout == nil {
			t.Fatalf("%v: the Accept's Session-Timeout was dropped", net.IP(special))
		}
		if withTimeout.FramedIP.IsValid() {
			t.Errorf("%v with Session-Timeout: pinned %s as the subscriber address", net.IP(special), withTimeout.FramedIP)
		}
	}
}
