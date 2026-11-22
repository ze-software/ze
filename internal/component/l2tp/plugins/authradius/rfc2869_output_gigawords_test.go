// RFC 2869 Section 5.2 (Acct-Output-Gigawords) NAS-side presence.
//
// VALIDATES: RFC2869-5.2-1 -- an Interim-Update whose output octet counter has wrapped
// carries Acct-Output-Gigawords with the wrap count, and a Start never carries it.
// The Section 5.1 twin, RFC2869-x-2, lives in rfc2869_acct_test.go and
// rfc2869_accounting_test.go.
//
// Producer: buildAcctPacket (acct.go) -- the Gigawords split by splitGigawords, appended
// inside the Stop or Interim-Update guard.

package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// TestRFC2869OutputGigawordsOnInterimUpdate: an Interim-Update whose output
// octet counter has wrapped carries Acct-Output-Gigawords with the wrap count.
// It stubs the original PPP unit's counters at three wraps plus 200 octets on transmit and
// reads the attribute out of the built Accounting-Request.
//
// RFC requirement: RFC2869-5.2-1 positive -- an Accounting-Request with Acct-Status-Type
// Interim-Update, whose output octet counter has wrapped around 2^32 three times,
// carries Acct-Output-Gigawords holding 3 beside Acct-Output-Octets holding the remainder.
func TestRFC2869OutputGigawordsOnInterimUpdate(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "heidi",
		acctSessID: "1-8-1",
	}
	bindAcctUnit(sess, newAcctUnitCapture(fixedAcctUnit(unitCounters(100, 0x300000000+200, 0, 0), nil)))

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusInterimUpdate, 60)

	assertAttrUint32(t, pkt, radius.AttrAcctOutputOctets, 200)
	assertAttrUint32(t, pkt, radius.AttrAcctOutputGigawords, 3)
}

// TestRFC2869OutputGigawordsAbsentOnStart: a Start never carries
// Acct-Output-Gigawords, whatever the transmit counter holds. It stubs the
// counters at three wraps on transmit and builds a Start.
//
// RFC requirement: RFC2869-5.2-1 negative -- an Accounting-Request with Acct-Status-Type
// Start carries no Acct-Output-Gigawords even when the output octet counter has wrapped
// around 2^32.
func TestRFC2869OutputGigawordsAbsentOnStart(t *testing.T) {
	acct := newRADIUSAcct()
	sess := &acctSession{
		username:   "ivan",
		acctSessID: "1-9-1",
	}
	bindAcctUnit(sess, newAcctUnitCapture(fixedAcctUnit(unitCounters(100, 0x300000000+200, 0, 0), nil)))

	pkt := acct.buildAcctPacket(sess, "nas1", nil, radius.AcctStatusStart, 0)

	if v := pkt.FindAttr(radius.AttrAcctOutputGigawords); v != nil {
		t.Error("Start packet must not carry Acct-Output-Gigawords")
	}
}
