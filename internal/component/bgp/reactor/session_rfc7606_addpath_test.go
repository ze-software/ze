// RFC: rfc/short/rfc7606.md — revised UPDATE error handling
// RFC: rfc/short/rfc7911.md — ADD-PATH (4-octet Path Identifier per NLRI)
// Overview: session_validation.go — enforceRFC7606 ADD-PATH awareness
//
// The §5.3 NLRI syntax check is ADD-PATH-aware. When ADD-PATH receive is negotiated for a
// family (RFC 7911 Section 3) every NLRI on the wire is prefixed with a 4-octet Path
// Identifier. The check must skip it before reading the prefix length; an ADD-PATH-blind walk
// misreads a path-id byte as a prefix length and spuriously session-resets a conforming
// UPDATE. These tests drive the real session entry point (enforceRFC7606) with ADD-PATH
// negotiated in the receive encoding context, proving the fix at the level where the
// negotiated state is known — the message-level unit tests cannot show the session actually
// consults it.

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/wireu"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/core/bgp/context"
)

// newAddPathSession builds a validation session with ADD-PATH negotiated (send-receive) for
// the given families and a registered receive encoding context, so the AddPathFor lookup in
// enforceRFC7606 reports true for them — mirroring Peer.setEncodingContexts (peer.go).
func newAddPathSession(t *testing.T, fams ...capability.Family) *Session {
	t.Helper()
	s := newValidateSession()

	local := make([]capability.Capability, 0, len(fams)+1)
	remote := make([]capability.Capability, 0, len(fams)+1)
	apFams := make([]capability.AddPathFamily, 0, len(fams))
	for _, f := range fams {
		local = append(local, &capability.Multiprotocol{AFI: f.AFI, SAFI: f.SAFI})
		remote = append(remote, &capability.Multiprotocol{AFI: f.AFI, SAFI: f.SAFI})
		apFams = append(apFams, capability.AddPathFamily{AFI: f.AFI, SAFI: f.SAFI, Mode: capability.AddPathBoth})
	}
	local = append(local, &capability.AddPath{Families: apFams})
	remote = append(remote, &capability.AddPath{Families: apFams})

	s.negotiated = capability.Negotiate(local, remote, 65001, 65002)

	ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(s.negotiated))
	require.NoError(t, err, "receive encoding context must register")
	s.SetRecvCtxID(ctxID)
	return s
}

// TestEnforceRFC7606_MPAddPathLargePathIDAccepted proves the fix for MP_REACH_NLRI: with
// ADD-PATH negotiated for IPv6 unicast, an MP UPDATE whose inner NLRI carries a path-id with a
// byte > 128 (which an ADD-PATH-blind walk would misread as an out-of-range prefix length)
// followed by a valid prefix is ACCEPTED with no session reset.
//
// VALIDATES: the §5.3-4 MP inner-NLRI syntax check skips the RFC 7911 4-octet path-id when
// ADD-PATH receive is negotiated, so a conforming ADD-PATH MP UPDATE is accepted.
// PREVENTS: the ADD-PATH-blind regression that read a path-id byte as a prefix length and
// spuriously session-reset a conforming multiprotocol UPDATE.
//
// rfc-test-change-approved: 2026-07-17 add-path-aware §5.3 NLRI validation fix (spec-rfc-requirement-coverage REV-1, user-approved)
//
// The contrast case (a session with NO ADD-PATH negotiated, sending the identical bytes) is
// asserted to session-reset: that is the correct interpretation when the leading octets are
// NLRI rather than a path-id, and it proves the acceptance above is due to ADD-PATH awareness
// and not because the check was simply weakened. If the fix regressed to an ADD-PATH-blind
// walk, the ADD-PATH session would reset too and the acceptance assertion would fail.
//
// Untagged guard: the RFC7606-5.3-4 coverage tags live on the message-level pair (see
// rfc7606_withdraw_test.go); this adds the session-level ADD-PATH proof without re-staling
// their audited fingerprints in rfc/audit/rfc7606.json.
// RFC requirement: RFC7911-5-5 positive -- with ADD-PATH receive negotiated for IPv6 unicast, enforceRFC7606 skips the 4-octet Path Identifier before reading the inner MP_REACH prefix length, so the negotiated family's NLRI is parsed with the path id and a conforming ADD-PATH UPDATE is accepted (RFC7606ActionNone).
// RFC requirement: RFC7911-5-5 negative -- with no ADD-PATH negotiated the identical bytes are parsed as plain NLRI, the leading path-id octet reads as an out-of-range prefix length, and the session resets, proving the parse consults the per-family negotiated ADD-PATH state.
func TestEnforceRFC7606_MPAddPathLargePathIDAccepted(t *testing.T) {
	ipv6 := capability.Family{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}

	// MP_REACH: AFI=2 SAFI=1 NHLen=16 NH[16] Reserved=0, then path-id 0x00000081 (129) and
	// 2001:db8::/32. Add-path-blind, the 0x81 path-id byte reads as prefix length 129 > 128.
	value := []byte{0x00, 0x02, 0x01, 0x10}
	value = append(value, make([]byte, 16)...) // valid 16-octet IPv6 next hop
	// Reserved, then ADD-PATH path-id = 129, then 2001:db8::/32 (4 prefix octets, exact fit).
	value = append(value, 0x00, 0x00, 0x00, 0x00, 0x81, 32, 0x20, 0x01, 0x0d, 0xb8)

	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
	}
	pathAttrs = append(pathAttrs, 0x80, 0x0E, byte(len(value))) // MP_REACH_NLRI, optional non-transitive
	pathAttrs = append(pathAttrs, value...)

	body := makeUpdateBody(nil, pathAttrs, nil)

	// ADD-PATH negotiated for IPv6 unicast: the path-id is skipped and the /32 accepted.
	s := newAddPathSession(t, ipv6)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a conforming ADD-PATH MP UPDATE must not reset the session")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"the 4-octet path-id must be skipped before the prefix length is read")

	// No ADD-PATH negotiated: the same bytes are plain NLRI, so 0x81 is a prefix length
	// 129 > 128 and the field is syntactically incorrect (§5.3 / §3(j) session reset). This
	// isolates the acceptance above to ADD-PATH awareness.
	sBlind := newValidateSession()
	_, blindAction, blindErr := sBlind.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, blindErr, "add-path-blind, the path-id byte is an out-of-range prefix length")
	assert.Equal(t, message.RFC7606ActionSessionReset, blindAction)
}

// TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted is the IPv4 unicast body-NLRI analog:
// with ADD-PATH negotiated for IPv4 unicast, a body NLRI whose path-id carries a byte > 32 and
// a valid prefix is ACCEPTED, and the same bytes reset a non-ADD-PATH session.
//
// VALIDATES: the §5.3 IPv4 body-NLRI syntax check in enforceRFC7606 consults the receive
// context's ADD-PATH state, so the RFC 7911 path-id is skipped and a conforming ADD-PATH
// NLRI is accepted.
// PREVENTS: the ADD-PATH-blind regression on the IPv4 body NLRI path.
//
// rfc-test-change-approved: 2026-07-17 add-path-aware §5.3 NLRI validation fix (spec-rfc-requirement-coverage REV-1, user-approved)
//
// Untagged guard: the RFC7606-5.3-1 coverage tags live on the message-level tests; this adds
// the session-level ADD-PATH proof without re-staling their audited fingerprints.
// RFC requirement: RFC7911-5-5 positive -- with ADD-PATH receive negotiated for IPv4 unicast, enforceRFC7606 skips the 4-octet Path Identifier before reading the body NLRI prefix length, so the negotiated family's NLRI is parsed with the path id and a conforming ADD-PATH UPDATE is accepted (RFC7606ActionNone).
// RFC requirement: RFC7911-5-5 negative -- with no ADD-PATH negotiated the identical IPv4 body bytes are parsed as plain NLRI, the leading path-id octet reads as an out-of-range prefix length, and the session resets, isolating acceptance to the per-family ADD-PATH state.
func TestEnforceRFC7606_IPv4BodyAddPathLargePathIDAccepted(t *testing.T) {
	ipv4 := capability.Family{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}

	pathAttrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	}
	// Body NLRI: path-id 0x00000021 (33) then 10.0.0.0/24. Add-path-blind, the 0x21 path-id
	// byte reads as prefix length 33 > 32.
	nlri := []byte{0x00, 0x00, 0x00, 0x21, 24, 10, 0, 0}
	body := makeUpdateBody(nil, pathAttrs, nlri)

	s := newAddPathSession(t, ipv4)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err, "a conforming ADD-PATH IPv4 body NLRI must not reset the session")
	assert.Equal(t, message.RFC7606ActionNone, action,
		"the 4-octet path-id must be skipped before the prefix length is read")

	sBlind := newValidateSession()
	_, blindAction, blindErr := sBlind.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.Error(t, blindErr, "add-path-blind, the path-id byte is an out-of-range prefix length")
	assert.Equal(t, message.RFC7606ActionSessionReset, blindAction)
}
