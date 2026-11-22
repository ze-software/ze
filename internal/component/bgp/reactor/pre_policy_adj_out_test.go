// Design: docs/guide/bmp.md -- pre-policy Adj-RIB-Out candidate emission in reactor
// Related: pre_policy_adj_out.go -- emitPrePolicyAdjRIBOut, prePolicyNextHopUnknownForFacts

package reactor

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

type prePolicyCaptureReceiver struct {
	mu   sync.Mutex
	sent []bgptypes.RawMessage
}

func (c *prePolicyCaptureReceiver) OnMessageReceived(_ *plugin.PeerInfo, _ bgptypes.RawMessage) int {
	return 0
}

func (c *prePolicyCaptureReceiver) OnMessageBatchReceived(_ *plugin.PeerInfo, _ []bgptypes.RawMessage) []int {
	return nil
}

func (c *prePolicyCaptureReceiver) OnMessageSent(_ *plugin.PeerInfo, msg bgptypes.RawMessage) {
	c.mu.Lock()
	c.sent = append(c.sent, msg)
	c.mu.Unlock()
}

func (c *prePolicyCaptureReceiver) snapshot() []bgptypes.RawMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]bgptypes.RawMessage, len(c.sent))
	copy(out, c.sent)
	return out
}

func testIPv4PrePolicyBody(nh [4]byte) []byte {
	attrs := []byte{
		byte(attribute.FlagTransitive), byte(attribute.AttrOrigin), 1, 0,
		byte(attribute.FlagTransitive), byte(attribute.AttrASPath), 6, 2, 1, 0, 0, 0xFD, 0xE9,
		byte(attribute.FlagTransitive), byte(attribute.AttrNextHop), 4, nh[0], nh[1], nh[2], nh[3],
	}
	nlri := []byte{24, 10, 0, 1}
	body := make([]byte, 4+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], nlri)
	return body
}

// TestExportFilterEmitsPrePolicyAdjRIBOutBeforePolicyRejection proves that the
// reactor emits EventKindUpdatePrePolicyOut (RawMessage.PrePolicyOut=true)
// before export policy runs, including for routes that export policy rejects,
// and marks PrePolicyNextHopUnknown according to the destination's next-hop
// rewrite mode.
func TestExportFilterEmitsPrePolicyAdjRIBOutBeforePolicyRejection(t *testing.T) {
	t.Run("nhModeSelf4 marks next hop unknown even when export policy rejects", func(t *testing.T) {
		capRecv := &prePolicyCaptureReceiver{}
		r := &Reactor{messageReceiver: capRecv}
		peer := NewPeer(&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.1"),
			LocalAS: 65000,
			PeerAS:  65001,
		})
		peer.fwdFacts.Store(&peerForwardFacts{
			addrStr:      "10.0.0.1",
			localAS:      65000,
			peerAS:       65001,
			isEBGP:       true,
			nhMode:       nhModeSelf4,
			exportPolicy: mustPolicy(t, nil, routepolicy.FinalReject),
		})

		body := testIPv4PrePolicyBody([4]byte{198, 51, 100, 1})
		suppress, _, _ := r.exportFilterForBody(peer, body)
		require.True(t, suppress, "export policy rejects the route post-policy")

		sent := capRecv.snapshot()
		require.Len(t, sent, 1, "pre-policy Adj-RIB-Out candidate must be emitted before export rejection")
		require.Equal(t, msgtype.TypeUPDATE, sent[0].Type)
		require.True(t, sent[0].PrePolicyOut)
		require.True(t, sent[0].PrePolicyNextHopUnknown, "nhModeSelf4 rewrites next hop post-policy so pre-policy next hop is unknown")
	})

	t.Run("nhModeNone with known next hop marks next hop known", func(t *testing.T) {
		capRecv := &prePolicyCaptureReceiver{}
		r := &Reactor{messageReceiver: capRecv}
		peer := NewPeer(&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.2"),
			LocalAS: 65000,
			PeerAS:  65000,
		})
		peer.fwdFacts.Store(&peerForwardFacts{
			addrStr:      "10.0.0.2",
			localAS:      65000,
			peerAS:       65000,
			isEBGP:       false,
			nhMode:       nhModeNone,
			exportPolicy: filterapi.Policy{},
		})

		body := testIPv4PrePolicyBody([4]byte{198, 51, 100, 1})
		suppress, _, _ := r.exportFilterForBody(peer, body)
		require.False(t, suppress)

		sent := capRecv.snapshot()
		require.Len(t, sent, 1)
		require.True(t, sent[0].PrePolicyOut)
		require.False(t, sent[0].PrePolicyNextHopUnknown, "preserved non-zero next hop is known at pre-policy completion")
	})

	t.Run("nhModeNone with all-zero body next hop marks next hop unknown", func(t *testing.T) {
		capRecv := &prePolicyCaptureReceiver{}
		r := &Reactor{messageReceiver: capRecv}
		peer := NewPeer(&PeerSettings{
			Address: netip.MustParseAddr("10.0.0.3"),
			LocalAS: 65000,
			PeerAS:  65000,
		})
		peer.fwdFacts.Store(&peerForwardFacts{
			addrStr:      "10.0.0.3",
			localAS:      65000,
			peerAS:       65000,
			nhMode:       nhModeNone,
			exportPolicy: filterapi.Policy{},
		})

		body := testIPv4PrePolicyBody([4]byte{0, 0, 0, 0})
		r.exportFilterForBody(peer, body)

		sent := capRecv.snapshot()
		require.Len(t, sent, 1)
		require.True(t, sent[0].PrePolicyOut)
		require.True(t, sent[0].PrePolicyNextHopUnknown, "0.0.0.0 body next hop is unknown at pre-policy completion")
	})
}
