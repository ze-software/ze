package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// These tests drive the mup plugin's recognizer (mup_test.go links the plugin into
// this binary) through the receive path's RFC 7606 pass, whose output is what the
// Adj-RIB-In holds and every relay forwards.

// TestMUPT1ST02TreatAsWithdraw proves a received draft-mpmz-bess-mup-safi-02 Type 1
// ST route, which ends at its Endpoint Address with no Source Address Length octet,
// is treated as withdrawn rather than installed or relayed.
//
// VALIDATES: draft-ietf-bess-mup-safi Section 3.1.3.1, "The Source Address Length has
// a fixed length of 1 octet.", and "< 0: the mandatory fields exceed the declared
// Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw.": after a
// well-formed generation K0 of the route, the -02 route leaves MP_REACH_NLRI and the
// route is withdrawn by K0's bytes, while a current-form sibling is kept byte for
// byte; no session reset.
// PREVENTS: a route short of a mandatory field being held in the Adj-RIB-In or relayed,
// in its received form or rewritten.
func TestMUPT1ST02TreatAsWithdraw(t *testing.T) {
	old := mupWireNLRI(3, t1stBody(2)...)
	k0 := mupWireNLRI(3, t1stBody(2, 0x00)...)
	sibling := mupWireNLRI(3, t1stBody(1, 0x00, 0xf0, 0x02, 0xaa, 0xbb)...)

	s := nlriTypeTestSession()
	holdT1STGeneration(t, s, mupFam, k0)
	nlri := append(append([]byte{}, old...), sibling...)
	wu, action, validation := applyRFC7606Body(t, s, makeUpdateBody(nil, mpReachAttrs(mupFam, nlri), nil))
	assert.Equal(t, message.RFC7606ActionNone, action, "a malformed T1ST is never a session reset")
	assert.Nil(t, validation.typedWithdrawBodies, "no MP_UNREACH was present, so the withdrawal joins the UPDATE")

	kept, found := mpReachNLRIOf(t, wu.Payload())
	require.True(t, found)
	assert.Equal(t, sibling, kept, "the -02 route leaves the announcement, the sibling stays as received")
	withdrawn, found := mpUnreachNLRIOf(t, wu.Payload())
	require.True(t, found, "the -02 route is withdrawn, not installed")
	assert.Equal(t, k0, withdrawn, "the route is withdrawn by the generation held for its key")
	requireDecodable(t, mupFam, withdrawn)
}
