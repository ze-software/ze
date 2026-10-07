// VALIDATES: the other AIGP TLVs of a received AIGP attribute reach the next
// peer unchanged, whether the metric is carried or accumulated (RFC 7311
// Section 3).
// PREVENTS: a forwarding rail or policy replacement that strips or rewrites
// the TLVs after the first.

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
)

// aigpOtherTLVsValue is an AIGP attribute value holding the AIGP TLV (metric
// 100), a second Type-1 TLV (metric 7) and an unknown Type-9 TLV. RFC 7311
// Section 3 calls everything after the first AIGP TLV "any other AIGP TLVs".
func aigpOtherTLVsValue() []byte {
	var value [27]byte
	attribute.WriteAIGPMetric(value[:], 0, 100)
	attribute.WriteAIGPMetric(value[:], 11, 7)
	copy(value[22:], []byte{9, 0, 5, 0xab, 0xcd})
	return value[:]
}

func aigpOtherTLVsBody(value []byte) []byte {
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x40, 3, 4, 192, 0, 2, 1, 0x40, 5, 4, 0, 0, 0, 100}
	attrs = append(attrs, 0x80, byte(attribute.AttrAIGP), byte(len(value)))
	attrs = append(attrs, value...)
	return makeUpdateBody(nil, attrs, []byte{24, 10, 20, 0})
}

func aigpWireValue(t *testing.T, body []byte) []byte {
	t.Helper()
	sections, err := wire.ParseUpdateSections(body)
	require.NoError(t, err)
	_, _, value, present := attribute.AttrFind(sections.Attrs(body), attribute.AttrAIGP)
	require.True(t, present, "the AIGP attribute must be passed along")
	return value
}

// TestRFC7311OtherAIGPTLVsPassedAlongUnchanged drives a received AIGP attribute
// with a second AIGP TLV and an unknown TLV through the forwarding rail to the
// socket, and compares the octets after the first TLV with what was received.
// Method: the transparent case forwards with the next hop unchanged; the
// accumulating case sets next-hop-self so Ze rewrites the first metric. In both,
// a policy replaces AIGP with a single-TLV value that drops the other TLVs,
// pushing the input toward the violation: the wire must still carry them.
//
// RFC requirement: RFC7311-3-3 positive -- when Ze passes the AIGP attribute along, the second Type-1 TLV and the unknown TLV reach the wire octet for octet, both when the next hop is unchanged and when next-hop-self rewrites the first metric.
// RFC requirement: RFC7311-3-3 negative -- a policy replacement that drops the other TLVs does not reach the wire: the forwarded attribute still carries them unchanged.
func TestRFC7311OtherAIGPTLVsPassedAlongUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name   string
		self   bool
		metric uint64
	}{
		{"transparent", false, 100},
		{"next-hop-self", true, 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			igpcost.Set(func(netip.Addr) igpcost.Distance {
				return igpcost.Distance{Cost: 30, Resolved: true}
			})
			t.Cleanup(func() { igpcost.Set(nil) })
			received := aigpOtherTLVsValue()
			body := aigpOtherTLVsBody(received)
			facts := &peerForwardFacts{aigpEnabled: true, localAddr: netip.MustParseAddr("192.0.2.9")}
			var mods filterapi.ModAccumulator
			var replacement [11]byte
			attribute.WriteAIGPMetric(replacement[:], 0, 100)
			mods.Op(uint8(attribute.AttrAIGP), filterapi.AttrModSet, replacement[:])
			if tc.self {
				mods.Op(uint8(attribute.AttrNextHop), filterapi.AttrModSet, []byte{192, 0, 2, 9})
			}
			applyFactsAIGP(facts, payloadAIGP(body), payloadNextHop(body), body, netip.MustParseAddr("192.0.2.1"), 0, &mods)
			rebuilt, _, failure := buildModifiedPayload(body, &mods, attrModHandlersWithDefaults(), nil, nil)
			require.Equal(t, modifyFailureNone, failure)
			require.NotNil(t, rebuilt)
			peer, conn := newAnnouncePeer(t, "192.0.2.2")
			source, _ := newAnnouncePeer(t, "192.0.2.1")
			for _, established := range []*Peer{source, peer} {
				established.session.localOpen = &message.Open{MyAS: 65000, HoldTime: 90}
				established.session.peerOpen = &message.Open{MyAS: 65001, HoldTime: 90}
				established.session.negotiateWith(nil, nil)
				established.setEncodingContexts(established.session.negotiated)
				t.Cleanup(established.clearEncodingContexts)
			}
			enabled := true
			peer.session.settings.AIGPSession = &enabled
			fwdBatchHandler(fwdKey{}, []fwdItem{{
				peer: peer, session: peer.currentSession(), authority: adjOutForwarded,
				rawBodies: [][]byte{rebuilt}, sourceMessageID: 1,
				receivedPeer: source, receivedGeneration: source.forwardGeneration.Load(),
				sourcePeerStr: source.addrString,
			}})
			sent := aigpWireValue(t, conn.written()[message.HeaderLen:])
			require.Len(t, sent, len(received), "the other AIGP TLVs must not be dropped")
			require.True(t, bytes.Equal(received[11:], sent[11:]), "the other AIGP TLVs must be passed along unchanged: got %x want %x", sent[11:], received[11:])
			metric, present := aigpReceivedMetric(t, conn.written()[message.HeaderLen:])
			require.True(t, present)
			require.Equal(t, tc.metric, metric, "only the first AIGP TLV carries the accumulated value")
		})
	}
}
