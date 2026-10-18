// Design: docs/architecture/meta/README.md -- route metadata on a received event
// RFC: rfc/short/rfc9494.md -- Section 4.3, processing a received LLGR_STALE route
// Related: reactor_notify.go -- notifyMessageReceiver, the ingress pass and msg.Meta
// Related: session_negotiate.go -- retainAdvertisedLLGR, localLLGR from Ze's own OPEN

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// llgrOpenParams is one capabilities optional parameter holding an LLGR
// Capability for ipv4/unicast with a long-lived stale time of 3600 seconds.
var llgrOpenParams = []byte{
	0x02, 0x09, // optional parameter: capabilities, length 9
	0x47, 0x07, // capability code 71 (LLGR), length 7
	0x00, 0x01, 0x01, 0x00, 0x00, 0x0e, 0x10, // AFI 1, SAFI 1, flags 0, LLST 3600
}

// llgrMetaStep stands in for the GR plugin's LLGRIngressFilter, which the reactor
// may not import: it writes stale level 2 when the session advertised LLGR, and
// records what it was told.
func llgrMetaStep(seen *[]bool) filterapi.IngressFilterFunc {
	return func(src filterapi.PeerFilterInfo, _ []byte, meta map[string]any) (bool, []byte) {
		*seen = append(*seen, src.LLGRAdvertised)
		if src.LLGRAdvertised {
			meta["stale"] = uint8(2)
		}
		return true, nil
	}
}

// receiveThroughIngress delivers one received UPDATE from a peer whose session's
// own OPEN did (localLLGR true) or did not carry the LLGR Capability, through one
// in-process ingress step, and returns the dispatched event and what the step saw.
func receiveThroughIngress(t *testing.T, localLLGR bool) (bgptypes.RawMessage, []bool) {
	t.Helper()
	addr := netip.MustParseAddr("203.0.113.71")
	source := NewPeer(&PeerSettings{
		Connection:    ConnectionBoth,
		Address:       addr,
		LocalAS:       65000,
		GlobalLocalAS: 65000,
		PeerAS:        65001,
		RouterID:      0xcb007147,
	})
	source.state.Store(int32(PeerStateEstablished))
	session := NewSession(source.Settings())
	session.localLLGR.Store(localLLGR)
	source.session = session

	var seen []bool
	var dispatched []bgptypes.RawMessage
	r := &Reactor{
		clock:           source.clock,
		config:          &Config{LocalAS: 65000},
		peers:           map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
		recentUpdates:   newRecentUpdateCache(100),
		attrModHandlers: attrModHandlersWithDefaults(),
		messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
			dispatched = append(dispatched, msg)
		}},
		orderedIngressSteps: []orderedIngressStep{{name: "llgr-ingress", inproc: llgrMetaStep(&seen)}},
	}

	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0, 0, 0xFD, 0xE9, // AS_PATH [65001]
		0x40, 0x03, 0x04, 203, 0, 113, 71, // NEXT_HOP
		0xc0, 0x08, 0x04, 0xff, 0xff, 0x00, 0x06, // COMMUNITIES LLGR_STALE
	}
	body := buildUpdatePayload(attrs, []byte{24, 192, 0, 2})
	wu := wireu.NewWireUpdate(body, 0)
	wu.SetSourceID(source.SourceID())

	r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived, BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)

	require.Len(t, dispatched, 1, "the UPDATE is dispatched to the RIB plugin")
	return dispatched[0], seen
}

// TestRFC9494ReceivedEventCarriesIngressStale verifies the stale level an
// ingress step decides reaches the received event the RIB consumes.
//
// VALIDATES: AC-13: the reactor hands the ingress metadata to the received event
// as msg.Meta, and the step is told the session advertised LLGR.
// PREVENTS: the ingress step deciding stale level 2 and the RIB never seeing it,
// so a received LLGR_STALE route competes as a fresh one.
//
// RFC requirement: RFC9494-4.3-1 positive -- a received UPDATE carrying LLGR_STALE on a session whose own OPEN carried the LLGR Capability reaches the RIB plugin as an event whose Meta["stale"] is uint8(2).
// MUTATION: drop `msg.Meta = routeMeta` in notifyMessageReceiver, and this goes red.
func TestRFC9494ReceivedEventCarriesIngressStale(t *testing.T) {
	msg, seen := receiveThroughIngress(t, true)

	require.Equal(t, []bool{true}, seen, "the step is told the session advertised LLGR")
	require.Equal(t, uint8(2), msg.Meta["stale"], "the received event carries the stale level")
}

// TestRFC9494ReceivedEventNoLLGRSessionStaysFresh verifies the same UPDATE on a
// session whose own OPEN carried no LLGR Capability reaches the RIB with no stale
// level.
//
// VALIDATES: AC-14: the step is told the session did not advertise LLGR.
// PREVENTS: a flag that reads true for every session.
//
// RFC requirement: RFC9494-4.3-1 negative -- the same LLGR_STALE UPDATE on a session whose own OPEN carried no LLGR Capability reaches the RIB plugin with no stale level in Meta.
// MUTATION: set LLGRAdvertised to true in notifyMessageReceiver, and this goes red.
func TestRFC9494ReceivedEventNoLLGRSessionStaysFresh(t *testing.T) {
	msg, seen := receiveThroughIngress(t, false)

	require.Equal(t, []bool{false}, seen, "the step is told the session did not advertise LLGR")
	require.NotContains(t, msg.Meta, "stale", "the received event carries no stale level")
}

// TestRetainAdvertisedLLGR verifies retainAdvertisedLLGR records whether Ze's
// own OPEN carried the LLGR Capability, and forgets it on a session that did not.
//
// VALIDATES: localLLGR is derived from Ze's OPEN when the peer OPEN is processed, before any UPDATE.
// PREVENTS: a stale flag from the previous session surviving a renegotiation.
func TestRetainAdvertisedLLGR(t *testing.T) {
	llgrCaps, err := capability.ParseFromOptionalParams(llgrOpenParams, false)
	require.NoError(t, err)

	s := NewSession(&PeerSettings{LocalAS: 65000, PeerAS: 65001, Address: netip.MustParseAddr("203.0.113.72")})

	s.retainAdvertisedLLGR(llgrCaps)
	require.True(t, s.localLLGR.Load(), "an OPEN carrying code 71 advertised LLGR")

	s.retainAdvertisedLLGR(nil)
	require.False(t, s.localLLGR.Load(), "an OPEN without code 71 did not")
}
