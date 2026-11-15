// Design: docs/architecture/bgp/structural-forwarding.md -- relay the received NLRI
// Related: rfc7311_aigp_readvertise_test.go -- live writer and cache fixture
// Related: relay_proof_helpers_test.go -- common socket relay driver
// RFC: rfc/short/rfc4271.md -- Section 5.1.6 and Section 9.1.4

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Rewriting NEXT_HOP and MED forces a rebuild, so the assertion covers more
// than the unchanged-wire shortcut. It reads the final Session socket bytes.
// RFC requirement: RFC4271-5.1.6-1 positive -- general and route-server relay
// preserve the exact received /17 and /24 NLRI with ATOMIC_AGGREGATE present,
// including when the destination's attribute policy rebuilds the UPDATE.
func TestRFC4271AtomicAggregateRelayPreservesExactNLRI(t *testing.T) {
	want := []byte{17, 10, 128, 0, 24, 192, 0, 2}
	for _, rs := range []bool{false, true} {
		f := newAIGPReplayFixture(t, nil)
		exportLibraryAnswer(t, f.r, medLibraryAnswer(27), f.destination)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, true, 10), rs)
		bodies := aigpSocketBodies(t, f.conn)
		require.Len(t, bodies, 1, "rs=%v: one live announcement", rs)
		update, err := message.UnpackUpdate(bodies[0])
		require.NoError(t, err)
		require.Equal(t, want, update.NLRI, "rs=%v: no deaggregation or prefix loss", rs)
		require.Empty(t, update.WithdrawnRoutes)
		_, _, value, present := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
		require.True(t, present, "rs=%v: the atomic aggregate remains identifiable", rs)
		require.Empty(t, value)
		med := payloadMED(bodies[0])
		require.True(t, med.present)
		require.Len(t, med.raw, 4)
		require.Equal(t, uint32(27), binary.BigEndian.Uint32(med.raw), "rs=%v: the policy actually rebuilt the route", rs)
	}
}

// The prohibition does not ban independently received more-specific routes.
// The input is a different received UPDATE, not a deaggregation of the first.
// RFC requirement: RFC4271-5.1.6-1 negative -- independently received /25 and
// /32 routes without ATOMIC_AGGREGATE are still advertised byte-for-byte on
// both relay rails; the preservation rule does not clamp every route to /24.
func TestRFC4271IndependentSpecificRoutesRemainAdvertisable(t *testing.T) {
	want := []byte{25, 198, 51, 100, 128, 32, 203, 0, 113, 9}
	for _, rs := range []bool{false, true} {
		f := newAIGPReplayFixture(t, nil)
		exportLibraryAnswer(t, f.r, medLibraryAnswer(27), f.destination)
		rfc4271ProofRelay(t, f, rfc4271ProofRelayBody(want, false, 10), rs)
		bodies := aigpSocketBodies(t, f.conn)
		require.Len(t, bodies, 1)
		update, err := message.UnpackUpdate(bodies[0])
		require.NoError(t, err)
		require.Equal(t, want, update.NLRI, "rs=%v: independent specific routes survive", rs)
		require.Empty(t, update.WithdrawnRoutes)
		_, _, _, present := attribute.AttrFind(update.PathAttributes, attribute.AttrAtomicAggregate)
		require.False(t, present)
	}
}

// The basic received attributes contain no AIGP, so this fixture's AIGP
// configuration cannot make an unknown link metric suppress the proof route.
func rfc4271ProofRelayBody(nlri []byte, atomic bool, med byte) []byte {
	var attrs []byte
	attrs = append(attrs, makeAttr(0x40, 1, []byte{0})...)
	attrs = append(attrs, makeAttr(0x40, 2, []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
	attrs = append(attrs, makeAttr(0x40, 3, []byte{198, 18, 231, 1})...)
	attrs = append(attrs, makeAttr(0x80, 4, []byte{0, 0, 0, med})...)
	if atomic {
		attrs = append(attrs, makeAttr(0x40, 6, nil)...)
	}
	return buildModTestPayload(attrs, nlri)
}
