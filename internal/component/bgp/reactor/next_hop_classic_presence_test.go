// Design: docs/architecture/bgp/structural-forwarding.md -- classic presence after raw policy
// Related: forward_next_hop.go -- payloadNextHop and effectiveNextHop
package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// A raw policy base can carry classic NLRI without code 3. A configured IPv4
// replacement then creates code 3, so classic presence must survive parsing even
// when no original address existed. MP-only controls still ignore unused code 3.
func TestEffectiveNextHopClassicPresence(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.2")
	for _, tc := range []struct {
		name                string
		classic, code3, own bool
	}{
		{name: "classic-missing-code3", classic: true, own: true},
		{name: "classic-existing-code3", classic: true, code3: true, own: true},
		{name: "mp-only-missing-code3"},
		{name: "mp-only-unused-code3", code3: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := classicPresenceBody(t, tc.classic, tc.code3)
			original := bytes.Clone(body)
			base := payloadNextHop(body)
			facts := &peerForwardFacts{addr: peer, nhMode: nhModeExplicit4, nhLegacy: [4]byte{192, 0, 2, 2}}
			var mods filterapi.ModAccumulator
			applyFactsNextHop(facts, &mods, base.mpFamily)
			applyNextHopFamily(facts, &mods, base, body)
			require.Equal(t, tc.own, egressNextHopIsPeerOwn(facts, &mods, base))
			wantGate := withholdNone
			if tc.own {
				wantGate = withholdPeerOwn
			}
			dest := NewPeer(&PeerSettings{Address: peer})
			require.Equal(t, wantGate, egressNextHopWithheld(dest, facts, &mods, base, false, netip.Addr{}))
			output, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			update, err := message.UnpackUpdate(output)
			require.NoError(t, err)
			_, _, hop, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
			require.True(t, found, "the materializer inserts the configured code 3")
			require.Equal(t, peer.AsSlice(), hop)
			require.Equal(t, tc.own, aigpNextHop(output).has(peer), "only classic NLRI makes code 3 a forwarding address")
			_, _, reach, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
			require.True(t, found)
			require.Equal(t, classicPresenceReach(t), reach, "the zero-hop FlowSpec sibling survives")
			require.Equal(t, original, body)
		})
	}
}

func classicPresenceReach(t *testing.T) []byte {
	t.Helper()
	// RFC 8955 framing: a length-five destination-prefix component for
	// 192.0.2.0/24, with no forwarding next hop and one reserved octet.
	return append([]byte{0, 1, 133, 0, 0}, mustHex(t, "050118c00002")...)
}

func classicPresenceBody(t *testing.T, classic, code3 bool) []byte {
	t.Helper()
	attrs := mixedAttrs(mixedAttr(14, classicPresenceReach(t)))
	if code3 {
		attrs = append(attrs, 0x40, 3, 4, 192, 0, 2, 9)
	}
	var prefix []byte
	if classic {
		prefix = mixedV4Legacy
	}
	return buildUpdatePayload(attrs, prefix)
}
