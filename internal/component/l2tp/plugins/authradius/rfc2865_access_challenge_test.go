// RFC: rfc/short/rfc2865.md -- RFC2865-4.4-1 on the L2TP subscriber login
// Related: handler.go -- doRADIUS, the switch over the reply code
// Related: handler_test.go -- setupAuthWithAttrs, fakeResponder

// The admin login already treats an Access-Challenge as an Access-Reject
// (internal/component/radius). The L2TP LNS is a second NAS role in the same
// daemon, with its own reply switch, so the rule is proven here again for its
// two credential-bearing methods.
package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// challengeLogins are the PAP and CHAP-MD5 subscriber logins each test drives.
func challengeLogins() map[string]ppp.EventAuthRequest {
	return map[string]ppp.EventAuthRequest{
		"pap": {TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP, Username: "alice", Response: []byte("pw")},
		"chap-md5": {
			TunnelID: 1, SessionID: 3, Method: ppp.AuthMethodCHAPMD5, Identifier: 7,
			Username: "bob", Challenge: make([]byte, 16), Response: make([]byte, 16),
		},
	}
}

// RFC requirement: RFC2865-4.4-1 positive -- an Access-Challenge carrying State
// and a Reply-Message, answered to a PAP and to a CHAP-MD5 subscriber login, is
// treated as an Access-Reject: the login is refused, with exactly one answer to
// the PPP layer (handler.go doRADIUS).
func TestRFC2865SubscriberAccessChallengeIsTreatedAsReject(t *testing.T) {
	key := []byte("testing123")
	challenge := []radius.Attr{
		{Type: radius.AttrState, Value: []byte{0x01, 0x02, 0x03, 0x04}},
		{Type: radius.AttrReplyMessage, Value: radius.AttrString("enter the token")},
	}
	for name, login := range challengeLogins() {
		a, resp, cleanup := setupAuthWithAttrs(t, key, radius.CodeAccessChallenge, challenge)
		a.handle(login, resp.respond)
		call := resp.waitOne(t)
		cleanup()
		if call.accept {
			t.Fatalf("%s: an Access-Challenge admitted the subscriber", name)
		}
		resp.mu.Lock()
		answers := len(resp.calls)
		resp.mu.Unlock()
		if got := answers; got != 1 {
			t.Errorf("%s: answers to the PPP layer: got %d, want 1", name, got)
		}
	}
}

// RFC requirement: RFC2865-4.4-1 negative -- the same PAP and CHAP-MD5 logins
// answered with an Access-Accept naming Framed-User are admitted, so the refusal
// above is specific to the Access-Challenge code (handler.go doRADIUS).
func TestRFC2865SubscriberAccessAcceptIsNotTreatedAsReject(t *testing.T) {
	key := []byte("testing123")
	framed := []radius.Attr{{Type: radius.AttrServiceType, Value: radius.AttrUint32(radius.ServiceTypeFramed)}}
	for name, login := range challengeLogins() {
		a, resp, cleanup := setupAuthWithAttrs(t, key, radius.CodeAccessAccept, framed)
		a.handle(login, resp.respond)
		call := resp.waitOne(t)
		cleanup()
		if !call.accept {
			t.Fatalf("%s: an Access-Accept for Framed-User was refused: %q", name, call.message)
		}
	}
}
