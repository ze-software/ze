// Related: authenticator_eap.go -- eapCredential, which appends the State
// RFC: rfc/short/rfc2865.md -- Section 5.24 State
//
// VALIDATES: the State obligation of a client that DOES support
// challenge/response. Ze acquired that role in plan/spec-radius-admin-eap.md:
// before it, every Access-Challenge was a rejection and there was no second
// request for a State to ride in.
// PREVENTS: a challenge loop that reads, rewrites, drops or manufactures the
// server's State, each of which breaks a server that keeps its conversation
// state in the attribute rather than in a session table.

package radius

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/eap"
)

// TestRadiusAdminStateIsReturnedUnmodified is the obligation itself.
//
// VALIDATES: every Access-Request that answers an Access-Challenge carries the
// State that challenge sent, byte for byte and at full length.
// PREVENTS: a loop that re-encodes, truncates or re-derives the value, all of
// which look identical to ze and are unusable to the server that issued it.
//
// RFC requirement: RFC2865-5.24-1 positive -- "This Attribute is available to
// be sent by the server to the client in an Access-Challenge and MUST be sent
// unmodified from the client to the server in the new Access-Request reply to
// that challenge" (authenticator_eap.go eapCredential, which appends the value
// authenticateEAP read from the challenge with resp.FindAttr(AttrState)).
func TestRadiusAdminStateIsReturnedUnmodified(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated)

	echoed := srv.states()
	require.NotEmpty(t, echoed, "the exchange ran at least one challenge round")

	// The server issues a distinct State per challenge round, so an echo that
	// merely repeated the first one is a different failure from one that dropped
	// it, and both are caught here.
	for round, got := range echoed {
		want := []byte{'z', 'e', byte(round), 0xa5}
		assert.Equalf(t, want, got,
			"the request answering challenge %d carries that challenge's State, unmodified", round)
	}
}

// TestRadiusAdminStateIsNotManufactured is the case that pins the attribute to
// the server rather than to ze.
//
// VALIDATES: when a challenge carries no State, the Access-Request answering it
// carries no State attribute either.
// PREVENTS: a client that invents a State, or that carries a stale one forward
// from an earlier round. RFC 2865 Section 5.24 gives a packet "only zero or one
// State Attribute", and the value is the server's to choose.
//
// RFC requirement: RFC2865-5.24-1 negative -- the same sentence read for the
// absent case: there is no State to send unmodified, so none is sent
// (authenticator_eap.go eapCredential, whose append is guarded by len(state) > 0).
func TestRadiusAdminStateIsNotManufactured(t *testing.T) {
	secret := []byte("testing123")
	srv := newEAPMockServer(t, secret, eap.TypeMSCHAPv2, "Hello",
		[]Attr{{Type: AttrFilterID, Value: []byte("admin")}})
	srv.mu.Lock()
	srv.dropState = true
	srv.mu.Unlock()

	a := eapAuthenticator(t, srv.addr, secret, AuthMethodEAPMSCHAPv2)
	res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "Hello"})
	require.NoError(t, err)
	require.True(t, res.Authenticated, "a challenge without State is answered normally")

	captured := srv.captured(t)
	require.Greater(t, len(captured), 1, "the exchange ran at least one challenge round")
	for index, pkt := range captured {
		assert.Nilf(t, pkt.FindAttr(AttrState),
			"request %d carries no State, because no challenge sent one", index)
	}
}
