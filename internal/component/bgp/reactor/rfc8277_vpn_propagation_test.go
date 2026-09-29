// Design: docs/architecture/core-design.md -- labeled unicast propagation
// Related: rfc8277_test.go -- the SAFI 4 case of the same rule

package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// TestLabeledVPNPropagationUnchangedNextHopKeepsLabels pins the SAFI 128 half of RFC 8277
// Section 3.2.1 on the forward rail.
//
// RFC 8277 Section 3.2.1: "When a SAFI-4 or SAFI-128 route is propagated, if the Network
// Address of Next Hop field is left unchanged, the Label field(s) MUST also be left
// unchanged."
//
// Method: a peer configured next-hop-unchanged produces no next-hop modification op, and
// the MP_REACH handler then plans the received VPN-IPv4 (AFI 1, SAFI 128) attribute. Its
// Next Hop (an 8-octet zero Route Distinguisher and 10.0.0.1), the label entry (label 20,
// S bit set), the Route Distinguisher and the prefix must all come out byte for byte.
//
// VALIDATES: a propagated VPN-IPv4 route with an unchanged next hop keeps its label.
// PREVENTS: a VPN label rewrite that the SAFI 4 case alone would not notice.
//
// RFC requirement: RFC8277-3.2.1-1 positive -- propagating a SAFI-128 VPN-IPv4 route with an unchanged Next Hop leaves the Next Hop and the NLRI label entry identical to the ones received.
func TestLabeledVPNPropagationUnchangedNextHopKeepsLabels(t *testing.T) {
	t.Parallel()

	dest := &PeerSettings{NextHopMode: NextHopUnchanged}
	var mods filterapi.ModAccumulator
	applyNextHopMod(dest, &mods)
	require.Empty(t, mods.Ops(), "next-hop-unchanged emits no next-hop rewrite op")

	vpnNextHop := []byte{0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1} // RD 0, then 10.0.0.1.
	vpnNLRI := []byte{
		112,              // 24 label bits + 64 RD bits + 24 prefix bits.
		0x00, 0x01, 0x41, // Label 20, S bit set.
		0, 0, 0xfd, 0xe9, 0, 0, 0, 1, // RD type 0, 65001:1.
		10, 1, 2, // Prefix 10.1.2.0/24.
	}
	src := buildMPReachSource(1 /*AFI IPv4*/, 128 /*SAFI VPN*/, vpnNextHop, vpnNLRI)

	out, ok := planHandlerBytes(mpReachNextHopHandler(), 14, src, mods.Ops())
	require.True(t, ok, "with no op the handler plans a verbatim copy")
	require.Equal(t, src, out, "the whole VPN MP_REACH, labels included, is unchanged")

	val := out[3:]
	require.Equal(t, vpnNextHop, val[4:16], "the Next Hop is the one received")
	require.Equal(t, vpnNLRI, val[17:], "the label entry, RD and prefix survive byte for byte")
	require.Equal(t, []byte{0x00, 0x01, 0x41}, val[18:21], "the label entry itself is untouched")
}
