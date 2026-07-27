package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// dynamicOpenConfirmSession returns a session in the state a DYNAMIC peer is in when
// collision resolution runs: no configured peer AS, but the peer's OPEN already received
// and stored, carrying the AS it advertises.
//
// It reaches OpenConfirm through the normal OPEN exchange with advertisedAS configured,
// then clears PeerAS. That reproduces the production state exactly --
// buildDynamicPeerSettings sets PeerAS to 0 and resolveDynamicPeerSettings only fills it
// on the Established transition, which is strictly after OpenConfirm -- without
// duplicating the pipe plumbing. The OPEN path itself is unaffected: with the AS
// configured to the same value it advertises, validateOpenIdentifier reads the identical
// number either way.
func dynamicOpenConfirmSession(t *testing.T, localAS, advertisedAS, localID uint32) *Session {
	t.Helper()

	session := openConfirmSessionWithAS(t, localAS, advertisedAS, localID)
	session.settings.PeerAS = 0
	session.settings.IsDynamic = true
	return session
}

// TestDetectCollisionEqualIdentifierDynamicPeerUsesAdvertisedAS verifies that the
// RFC 6286 Section 2.3 tie-break reads the AS a peer ADVERTISES when none is configured.
//
// A dynamic peer's PeerAS is 0 in OpenConfirm -- the only state this branch runs in --
// so comparing it raw made `PeerAS > LocalAS` false against any real LocalAS, and the
// tie-break always preserved the local connection whatever the peer's actual AS. The
// zero silently selected one branch instead of being recognized as "unknown"
// (ai/rules/fail-closed-guards.md). validateOpenIdentifier one function away already
// falls back to openAdvertisedAS for exactly this reason; this sibling call site was
// missed (ai/rules/before-writing-code.md, Sibling Call-Site Audit).
//
// Not an interop change today: dynamic peers are ConnectionPassive, so both colliding
// connections are peer-initiated and Section 2.3's "connection initiated by the speaker
// with the larger AS number" is degenerate. The guard is pinned here so it stays correct
// if a dynamic peer ever initiates.
//
// VALIDATES: advertised AS larger than local -> the pending (remote) connection wins.
// VALIDATES: advertised AS smaller than local -> the existing (local) connection is kept.
// PREVENTS: the configured-AS zero value deciding the tie-break instead of the peer's
// real AS number.
func TestDetectCollisionEqualIdentifierDynamicPeerUsesAdvertisedAS(t *testing.T) {
	const localID uint32 = 0x01020304

	tests := []struct {
		name         string
		localAS      uint32
		advertisedAS uint32
		wantAccept   bool
		wantClose    bool
		description  string
	}{
		{
			name:    "advertised AS larger than local wins",
			localAS: 65001, advertisedAS: 65002,
			wantAccept: true, wantClose: true,
			description: "the peer advertises the larger AS, so its connection is preserved",
		},
		{
			name:    "advertised AS smaller than local loses",
			localAS: 65002, advertisedAS: 65001,
			wantAccept: false, wantClose: false,
			description: "this speaker has the larger AS, so its own connection is preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := dynamicOpenConfirmSession(t, tt.localAS, tt.advertisedAS, localID)

			accept, closeExisting := session.DetectCollision(localID)

			assert.Equal(t, tt.wantAccept, accept, tt.description)
			assert.Equal(t, tt.wantClose, closeExisting, tt.description)
		})
	}
}

// TestCollisionPeerASResolutionOrder verifies collisionPeerAS resolution order.
//
// VALIDATES: a configured PeerAS is used verbatim and the OPEN is not consulted.
// VALIDATES: with no configured AS and no OPEN yet, the result is 0 rather than a guess.
// PREVENTS: the advertised AS overriding an operator-configured peer AS.
func TestCollisionPeerASResolutionOrder(t *testing.T) {
	t.Run("configured AS wins over advertised", func(t *testing.T) {
		session := openConfirmSessionWithAS(t, 65001, 65002, 0x01020304)

		assert.Equal(t, uint32(65002), session.collisionPeerAS(),
			"a configured PeerAS must be used as-is")
	})

	t.Run("no configured AS and no OPEN yields zero", func(t *testing.T) {
		settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 0, 0x01020304)
		session := NewSession(settings)

		assert.Equal(t, uint32(0), session.collisionPeerAS(),
			"with nothing to read, the answer is unknown, not invented")
	})
}
