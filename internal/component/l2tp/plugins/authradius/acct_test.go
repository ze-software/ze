// Goal: prove RADIUS accounting stops reporting an IPv4 address the session
// released while it stays up, and reports it again after the reinstall.
// Method: a tracked accounting session receives (l2tp, session-ip-released)
// and then (l2tp, session-ip-assigned) through the handlers the bus
// subscription calls, and buildAcctPacket, the producer of every record,
// builds an Interim-Update after each.

package l2tpauthradius

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestAcctInterimOmitsReleasedIPv4 (AC-4, AC-8). RFC 2866 Section 4.1: the
// Framed-IP-Address "MUST contain the actual IP address assigned or
// negotiated." Fails before the release handler: peerAddr kept the address.
func TestAcctInterimOmitsReleasedIPv4(t *testing.T) {
	acct := newRADIUSAcct()
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatalf("birth: %v", err)
	}
	key := sessionKey{birth: birth, tunnelID: 1, sessionID: 2}
	sess := &acctSession{birth: birth, tunnelID: 1, sessionID: 2, username: "alice", acctSessID: "1-2-1", peerAddr: "10.100.0.2"}
	acct.mu.Lock()
	acct.sessions[key] = sess
	acct.accepting = true
	acct.endpoint = &acctEndpoint{}
	acct.mu.Unlock()
	framed := func() [][]byte {
		return acct.buildAcctPacket(sess, "lns1", nil, radius.AcctStatusInterimUpdate, 0).FindAllAttr(radius.AttrFramedIPAddress)
	}

	acct.onSessionIPReleased(&events.SessionIPReleasedPayload{Birth: birth, SubscriberID: birth.String(),
		TunnelID: 1, SessionID: 2, Family: events.AddressFamilyIPv6})
	if got := framed(); len(got) != 1 {
		t.Fatalf("after an IPv6 release: Framed-IP-Address count %d, want 1", len(got))
	}

	acct.onSessionIPReleased(&events.SessionIPReleasedPayload{Birth: birth, SubscriberID: birth.String(),
		TunnelID: 1, SessionID: 2, Family: events.AddressFamilyIPv4})
	if got := framed(); len(got) != 0 {
		t.Fatalf("after an IPv4 release: Framed-IP-Address count %d, want 0", len(got))
	}

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{Birth: birth, SubscriberID: birth.String(),
		TunnelID: 1, SessionID: 2, PeerAddr: "10.100.0.2"})
	got := framed()
	if len(got) != 1 {
		t.Fatalf("after the reinstall: Framed-IP-Address count %d, want 1", len(got))
	}
	if addr := net.IP(got[0]).String(); addr != "10.100.0.2" {
		t.Fatalf("after the reinstall: Framed-IP-Address = %s, want 10.100.0.2", addr)
	}
}
