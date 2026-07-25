// Design: docs/architecture/core-design.md -- labeled unicast propagation
// RFC: rfc/short/rfc8277.md -- label handling when a route is propagated
// Related: filter_delta_handlers.go -- mpReachNextHopHandler, applyNextHopMod

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// labeledMPReachNLRI is one RFC 8277 labeled unicast NLRI: Length = 24 label
// bits + 24 prefix bits, label 100 with the S bit set, then 10.0.0.0/24.
var labeledMPReachNLRI = []byte{48, 0x00, 0x06, 0x41, 10, 0, 0}

// TestLabeledPropagationUnchangedNextHopKeepsLabels pins RFC 8277 Section
// 3.2.1: when a labeled unicast route is propagated with the Next Hop left
// unchanged, the Label field(s) must be left unchanged too. ze expresses
// "unchanged" as an empty next-hop modification set, and the MP_REACH handler
// copies the source attribute verbatim when no Set op reaches it, so the label
// octets inside the NLRI are the ones that were received.
//
// VALIDATES: applyNextHopMod emits no op for NextHopUnchanged, and
// mpReachNextHopHandler with no op reproduces the labeled MP_REACH byte for
// byte (AFI/SAFI, next-hop, reserved octet, label entry and prefix).
// PREVENTS: a propagated labeled route carrying a label the downstream router
// never bound, which would blackhole the MPLS path.
//
// RFC requirement: RFC8277-3.2.1-1 positive -- propagating with an unchanged Next Hop leaves the NLRI label octets identical to the ones received.
func TestLabeledPropagationUnchangedNextHopKeepsLabels(t *testing.T) {
	t.Parallel()

	// A peer configured next-hop-unchanged produces no next-hop modification.
	dest := &PeerSettings{NextHopMode: NextHopUnchanged}
	var mods filterapi.ModAccumulator
	applyNextHopMod(dest, &mods)
	require.Empty(t, mods.Ops(), "next-hop-unchanged emits no next-hop rewrite op")

	// With no op, the labeled MP_REACH is copied verbatim.
	src := buildMPReachSource(1 /*AFI IPv4*/, 4, /*SAFI labeled unicast*/
		[]byte{10, 0, 0, 1}, labeledMPReachNLRI)

	buf := make([]byte, 256)
	n := mpReachNextHopHandler()(src, mods.Ops(), buf, 0)
	require.Equal(t, len(src), n)
	assert.Equal(t, src, buf[:n], "the whole MP_REACH, labels included, is unchanged")

	val := buf[3:n]
	assert.Equal(t, labeledMPReachNLRI, val[9:],
		"the label entry and prefix survive propagation byte for byte")
	assert.Equal(t, byte(0x01), val[9+3]&0x01,
		"the S bit of the propagated label entry is untouched")
}

// TestLabeledPropagationChangedNextHopReusesReceivedLabelGap documents RFC 8277
// Section 3.2.2 as ze implements it today: when the Next Hop is changed on
// propagation, the NLRI must carry the label(s) bound to the prefix AT THE NEW
// next hop. ze allocates no local label for labeled unicast, and the MP_REACH
// handler patches only the next-hop field, so the received upstream label is
// re-advertised alongside a next hop that never bound it.
//
// VALIDATES: the exact observable behavior behind the RFC8277-3.2.2-1 gap.
// PREVENTS: the gap being closed silently, or being mis-recorded as closed.
func TestLabeledPropagationChangedNextHopReusesReceivedLabelGap(t *testing.T) {
	t.Parallel()

	src := buildMPReachSource(1, 4, []byte{10, 0, 0, 1}, labeledMPReachNLRI)
	ops := []filterapi.AttrOp{
		{Code: 14, Action: filterapi.AttrModSet, Buf: []byte{10, 0, 0, 2}},
	}

	buf := make([]byte, 256)
	n := mpReachNextHopHandler()(src, ops, buf, 0)
	require.Equal(t, len(src), n)

	val := buf[3:n]
	assert.Equal(t, []byte{10, 0, 0, 2}, val[4:8], "the next hop is rewritten")
	assert.Equal(t, labeledMPReachNLRI, val[9:],
		"gap RFC8277-3.2.2-1: the upstream label rides along unchanged, with no label bound at the new next hop")
}
