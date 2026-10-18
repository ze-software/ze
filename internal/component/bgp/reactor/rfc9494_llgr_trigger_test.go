// Design: docs/architecture/core-design.md -- BGP capability negotiation
// RFC: rfc/short/rfc9494.md -- Section 4.3, the trigger is the capability Ze advertised
// Related: session_negotiate.go -- negotiateWith, retainAdvertisedLLGR, advertisedLLGR

package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// newOpenedSession is a session that has both OPENs, which is the state
// negotiateWith requires before it negotiates anything.
func newOpenedSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession(&PeerSettings{
		LocalAS:         65000,
		PeerAS:          65001,
		Address:         netip.MustParseAddr("203.0.113.73"),
		ReceiveHoldTime: 90 * time.Second,
	})
	s.localOpen = &message.Open{Version: 4, MyAS: 65000, HoldTime: 90, BGPIdentifier: 0x01020304}
	s.peerOpen = &message.Open{Version: 4, MyAS: 65001, HoldTime: 90, BGPIdentifier: 0x05060708}
	return s
}

// TestRFC9494NegotiateWithKeysOnAdvertisedLLGR verifies negotiation decides
// whether the session advertised LLGR from Ze's OWN capabilities, and never from
// the capabilities the peer sent.
//
// Method: negotiate one session with the LLGR Capability on Ze's side only, and
// another with it on the peer's side only, then ask advertisedLLGR, the
// accessor the ingress step and the relay rail read.
//
// VALIDATES: RFC 9494 Section 4.3 is triggered by "has advertised the Long-Lived
// Graceful Restart Capability to a neighbor", which is the OPEN Ze sent.
// PREVENTS: a session treating received LLGR_STALE routes as least preferred
// because the PEER sent LLGR while Ze did not.
//
// RFC requirement: RFC9494-4.3-1 positive -- negotiateWith with the LLGR Capability among Ze's own capabilities and none from the peer leaves advertisedLLGR true.
//
// RFC requirement: RFC9494-4.3-1 negative -- negotiateWith with the LLGR Capability only among the peer's capabilities leaves advertisedLLGR false.
//
// MUTATION: pass peerCaps instead of localCaps to retainAdvertisedLLGR in
// negotiateWith (session_negotiate.go), and both cases go red.
func TestRFC9494NegotiateWithKeysOnAdvertisedLLGR(t *testing.T) {
	llgrCaps, err := capability.ParseFromOptionalParams(llgrOpenParams, false)
	require.NoError(t, err)
	require.NotEmpty(t, llgrCaps, "the fixture holds the LLGR Capability")

	t.Run("advertised", func(t *testing.T) {
		s := newOpenedSession(t)
		s.negotiateWith(llgrCaps, nil)
		require.NotNil(t, s.negotiated, "negotiation ran")
		require.True(t, s.advertisedLLGR(), "Ze's own OPEN carried LLGR")
	})

	t.Run("received", func(t *testing.T) {
		s := newOpenedSession(t)
		s.negotiateWith(nil, llgrCaps)
		require.NotNil(t, s.negotiated, "negotiation ran")
		require.False(t, s.advertisedLLGR(), "only the peer's OPEN carried LLGR")
	})
}
