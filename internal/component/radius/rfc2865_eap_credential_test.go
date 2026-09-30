// Related: authenticator_eap.go -- authenticateEAP and eapCredential, the EAP
//   Access-Request builder
// RFC: rfc/short/rfc2865.md -- Section 4.1
//
// VALIDATES: the extension clause of RFC 2865 Section 4.1 on the admin EAP
// login. "If future extensions allow other kinds of authentication information
// to be conveyed, the attribute for that can be used in an Access-Request
// instead of User-Password or CHAP-Password." RFC 3579 is such an extension:
// EAP-Message carries the authentication information, and an EAP
// Access-Request carries neither of the RFC 2865 credentials.
// PREVENTS: an EAP login that also sends the operator's password as a
// User-Password, and one that sends an Access-Request carrying no
// authentication information at all.

package radius

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// TestRFC2865EAPAccessRequestCarriesTheExtensionCredential drives a full
// EAP-MSCHAPv2 login and reads every Access-Request the server received.
//
// VALIDATES: each Access-Request carries EAP-Message, the extension's
// authentication information, and neither User-Password nor CHAP-Password;
// every request after the first also carries the server's State.
// PREVENTS: an EAP round sent with no authentication information, or with an
// RFC 2865 credential beside the EAP one.
//
// RFC requirement: RFC2865-4.1-3 positive -- every EAP Access-Request carries
// EAP-Message in place of User-Password or CHAP-Password, and each request
// answering a challenge carries State (authenticator_eap.go authenticateEAP,
// eapCredential).
func TestRFC2865EAPAccessRequestCarriesTheExtensionCredential(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	captured := srv.captured(t)
	require.GreaterOrEqual(t, len(captured), 2, "identity round, then at least one challenge round")
	for i, req := range captured {
		assert.Equal(t, uint8(CodeAccessRequest), req.Code, "request %d", i)
		assert.NotEmpty(t, req.FindAllAttr(AttrEAPMessage), "request %d carries the EAP credential", i)
		assert.Empty(t, req.FindAllAttr(AttrUserPassword), "request %d carries no User-Password", i)
		assert.Empty(t, req.FindAllAttr(AttrCHAPPassword), "request %d carries no CHAP-Password", i)
		if i > 0 {
			assert.Len(t, req.FindAllAttr(AttrState), 1, "request %d answers a challenge and carries its State", i)
		}
	}
}
