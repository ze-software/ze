// Design: docs/architecture/plugin/rib-storage-design.md -- effective received cleanup attributes.
// This observes the actual ingress producer; it does not implement pre-policy storage.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A policy-added NO_LLGR and a wire NO_LLGR reach the same effective received
// attributes on both actual delivery representations. The plain useful sibling
// keeps its NLRI/attributes and never acquires the cleanup predicate.
func TestRetainedCleanupSeesEffectiveImportNoLLGR(t *testing.T) {
	for _, kind := range []string{"wire", "policy", "plain-sibling"} {
		t.Run(kind, func(t *testing.T) {
			evaluate := policychain.Evaluator(func(_ *policyview.View, sets *policychain.Sets) (policychain.Decision, error) {
				if kind == "policy" {
					return policychain.DecisionPass, sets.AddCommunity(uint32(attribute.CommunityNoLLGR))
				}
				return policychain.DecisionPass, nil
			})
			registry, err := routepolicy.NewRegistry(routepolicy.Instance{Name: "cleanup-community", Type: libraryAtomicType, Config: map[string]any{"evaluate": evaluate}})
			require.NoError(t, err)
			chain, err := policychain.Compile(libraryAtomicAttachment, registry, []policychain.Member{{Name: "cleanup-community"}}, routepolicy.FinalAccept, policychain.Bases{})
			require.NoError(t, err)
			address := netip.MustParseAddr("192.0.2.50")
			source := NewPeer(&PeerSettings{Connection: ConnectionBoth, Address: address, LocalAS: 65000, GlobalLocalAS: 65000, PeerAS: 65001,
				ImportPolicy: mustPolicy(t, []routepolicy.Ref{{Name: "cleanup-community"}}, routepolicy.FinalAccept).WithLibrary(chain)})
			source.state.Store(int32(PeerStateEstablished))
			var messages []bgptypes.RawMessage
			r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000}, peers: map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
				recentUpdates: newRecentUpdateCache(100), attrModHandlers: attrModHandlersWithDefaults(),
				orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
				messageReceiver:     &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) { messages = append(messages, msg) }},
			}
			t.Cleanup(r.recentUpdates.Stop)
			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 50}
			if kind == "wire" {
				attrs = append(attrs, 0xc0, 8, 4, 0xff, 0xff, 0, 7)
			}
			ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
			require.NoError(t, err)
			nlri := []byte{24, 10, 99, 2}
			body := buildUpdatePayload(attrs, nlri)
			wire := wireu.NewWireUpdate(body, ctxID)
			wire.SetSourceID(source.SourceID())
			r.notifyMessageReceiver(address, msgtype.TypeUPDATE, body, wire, ctxID, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)
			require.Len(t, messages, 1, "the plain sibling remains useful and accepted too")
			msg := messages[0]
			want := kind != "plain-sibling"
			attrLen := int(binary.BigEndian.Uint16(msg.RawBytes[2:4]))
			require.Equal(t, nlri, msg.RawBytes[4+attrLen:])
			raw, err := msg.AttrsWire.GetRaw(attribute.AttrCommunity)
			require.NoError(t, err)
			require.Equal(t, want, bytes.Contains(raw, []byte{0xff, 0xff, 0, 7}), "typed received attributes carry the effective predicate")
			peer := plugin.PeerInfo{Address: address, AddressStr: address.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000}
			encoded := format.AppendMessage(nil, &peer, msg, bgptypes.ContentConfig{Encoding: plugin.EncodingJSON, Format: plugin.FormatFull})
			parsed, err := bgp.ParseEvent(encoded)
			require.NoError(t, err)
			require.Equal(t, msg.RawBytes[4:4+attrLen], parsed.GetRawAttributesBytes(), "full JSON carries those same effective received attributes")
		})
	}
}
