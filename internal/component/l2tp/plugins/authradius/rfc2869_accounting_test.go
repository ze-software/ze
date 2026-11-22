// RFC 2869 (RADIUS Gigaword accounting) NAS-side presence rules.
//
// VALIDATES: RFC 2869 Section 5.1-5.2 -- the Acct-Input/Output-Gigawords attributes
// appear only in Stop/Interim Accounting-Requests, never in a Start, and only when the
// wrap count is non-zero. The positive presence and non-zero cases live with the counter
// tests in rfc2869_acct_test.go (TestBuildAcctPacketGigawords, TestBuildAcctPacketWithCounters);
// this file pins the status-type gate with its own negative case.
//
// Producer: buildAcctPacket (acct.go) -- the gigaword append sits inside the
// `statusType == Stop || Interim` guard (acct.go:212), so a Start never reaches it.

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// RFC requirement: RFC2869-x-2 negative -- an Accounting-Start carries no Gigawords
// attribute even when the interface counters exceed 4GB; presence is gated on
// Acct-Status-Type being Stop or Interim-Update, not on the counter value.
func TestRFC2869GigawordsAbsentOnStart(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "grace",
		acctSessID: "1-7-1",
	}
	bindAcctUnit(sess, newAcctUnitCapture(fixedAcctUnit(unitCounters(0x200000000+100, 0x300000000+200, 0, 0), nil)))

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStart, 0)

	if v := pkt.FindAttr(radius.AttrAcctInputGigawords); v != nil {
		t.Error("Start packet must not carry Acct-Input-Gigawords")
	}
	if v := pkt.FindAttr(radius.AttrAcctOutputGigawords); v != nil {
		t.Error("Start packet must not carry Acct-Output-Gigawords")
	}
}
