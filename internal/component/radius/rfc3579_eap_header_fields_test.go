// Related: authenticator_eap.go -- processEAPMessage and
//   checkEAPResponseHeader, the two halves of the header validation
// Related: authenticator_eap_test.go -- eapMockServer, the RADIUS server these
//   units talk to
// RFC: rfc/short/rfc3579.md -- Section 2.2
//
// VALIDATES: the Code and Identifier halves of RFC 3579 Section 2.2, which the
// Length-only units in rfc3579_nas_obligations_test.go leave open. From the
// server, an Access-Challenge's EAP packet must be a Request (Code 1); toward
// the server, the Response must be a Response (Code 2) carrying the Identifier
// of the most recently validated Request.
// PREVENTS: a NAS that hands the peer whatever Code the server encapsulated, and
// one that forwards a Response the server's outstanding Request cannot match.

package radius

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// TestRFC3579NASForwardsTheCurrentIdentifierBothWays is the conforming side.
//
// VALIDATES: over a complete EAP-MSCHAPv2 login, every EAP packet the server put
// in an Access-Challenge is a Request, and every EAP packet ze forwards in the
// next Access-Request is a Response whose Identifier is that Request's. The
// check itself admits the matching pair.
// PREVENTS: an Identifier drawn anywhere but from the Request the Response
// answers.
//
// RFC requirement: RFC3579-2.2-1 positive -- a Challenge carrying Code 1 is
// forwarded and each Response ze sends carries Code 2 and the Identifier of the
// server's most recent Request (authenticator_eap.go processEAPMessage and
// checkEAPResponseHeader).
func TestRFC3579NASForwardsTheCurrentIdentifierBothWays(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	srv.mu.Lock()
	sent := append([][]byte{}, srv.sentEAP...)
	srv.mu.Unlock()
	require.GreaterOrEqual(t, len(captured), 3, "MS-CHAPv2 runs identity, challenge and success rounds")
	require.Len(t, sent, len(captured), "one server reply per Access-Request")

	for i := 1; i < len(captured); i++ {
		request, err := eap.DecodePacket(sent[i-1])
		require.NoError(t, err)
		require.Equal(t, eap.CodeRequest, request.Code, "reply %d is an Access-Challenge", i-1)

		response := eapPacketOf(t, captured[i])
		assert.Equal(t, eap.CodeResponse, response.Code, "Access-Request %d", i)
		assert.Equal(t, request.Identifier, response.Identifier,
			"Access-Request %d answers the Request the server sent before it", i)
	}

	require.NoError(t, checkEAPResponseHeader(
		&eap.Packet{Code: eap.CodeRequest, Identifier: 7, Type: eap.TypeMSCHAPv2},
		&eap.Packet{Code: eap.CodeResponse, Identifier: 7, Type: eap.TypeMSCHAPv2}))
}

// TestRFC3579NASRefusesANonRequestCodeInAChallenge is the Code check on the
// server's side.
//
// VALIDATES: an Access-Challenge whose EAP packet carries any Code but 1 is
// refused before the peer reads it: the login ends as an infrastructure error
// naming the server's EAP header, and no Access-Request follows it.
// PREVENTS: a Response, Success, Failure or unknown Code reaching the peer as if
// the server had asked it something.
//
// RFC requirement: RFC3579-2.2-1 negative -- the Code of a server packet inside
// an Access-Challenge is checked against 1 before forwarding
// (authenticator_eap.go processEAPMessage).
func TestRFC3579NASRefusesANonRequestCodeInAChallenge(t *testing.T) {
	codes := []uint8{0, eap.CodeResponse, eap.CodeSuccess, eap.CodeFailure, 5, 255}
	for _, code := range codes {
		secret := []byte("testing123")
		srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello", nil)
		srv.mu.Lock()
		srv.challengeEAPCode, srv.rewriteChallengeEAPCode = code, true
		srv.mu.Unlock()

		a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
		res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})

		require.Error(t, err, "code %d", code)
		assert.ErrorContains(t, err, "EAP header from the server", "code %d", code)
		assert.NotErrorIs(t, err, aaa.ErrAuthRejected, "code %d", code)
		assert.False(t, res.Authenticated, "code %d", code)
		assert.Equal(t, 1, srv.requestCount(), "code %d: nothing was forwarded after the challenge", code)
	}
}

// TestRFC3579NASRefusesAResponseOffTheCurrentIdentifier is the check on the
// peer's side.
//
// VALIDATES: a Response whose Identifier is not the current Request's, or a
// packet that is not a Response at all, is refused and so never forwarded.
// PREVENTS: forwarding what RFC 3579 Section 2.2 says the NAS silently discards.
//
// RFC requirement: RFC3579-2.2-1 negative -- a Response whose Code is not 2 or
// whose Identifier does not match the current Request is refused
// (authenticator_eap.go checkEAPResponseHeader).
func TestRFC3579NASRefusesAResponseOffTheCurrentIdentifier(t *testing.T) {
	request := &eap.Packet{Code: eap.CodeRequest, Identifier: 7, Type: eap.TypeMSCHAPv2}
	responses := []*eap.Packet{
		{Code: eap.CodeResponse, Identifier: 6, Type: eap.TypeMSCHAPv2},
		{Code: eap.CodeResponse, Identifier: 8, Type: eap.TypeMSCHAPv2},
		{Code: eap.CodeRequest, Identifier: 7, Type: eap.TypeMSCHAPv2},
		{Code: eap.CodeFailure, Identifier: 7},
	}
	for _, response := range responses {
		err := checkEAPResponseHeader(request, response)
		assert.Error(t, err, "code %d identifier %d", response.Code, response.Identifier)
	}
}
