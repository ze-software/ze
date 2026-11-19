// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4.2, re-establishment without the Graceful Restart Capability
// Overview: gr.go -- handleStructuredOpen and handleStructuredState, the DirectBridge path
// Related: rfc9494_gr_event_test.go -- buildOpenBody, buildCapabilityParam, buildGRCapTLV

package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath runs a whole restart
// through handleStructuredEvent: an OPEN with the Graceful Restart Capability (F set for
// IPv4 and IPv6 unicast), a TCP failure, then an OPEN that carries no capability 64 and the
// session coming back up.
//
// VALIDATES: RFC 4724 Section 4.2 -- "if the Graceful Restart Capability is not received in
// the re-established session at all, then the Receiving Speaker MUST immediately remove all
// the stale routes from the peer that it is retaining for that address family."
// handleStructuredOpen forgets the previous session's capability when the new OPEN lacks
// one, so the "up" event purges both retained families.
// PREVENTS: the previous session's capability surviving an OPEN without it, which keeps
// every stale route until End-of-RIB or Restart Time expiry.
//
// RFC requirement: RFC4724-4.2-8 positive -- on the structured event path, after a TCP failure of a session whose OPEN carried capability 64 with F set for IPv4 and IPv6 unicast, a re-established OPEN with only capability 65, or with no optional parameters, makes the "up" event dispatch purge-stale for both families.
func TestRFC4724ReestablishedWithoutCapabilityPurgesOnTheStructuredPath(t *testing.T) {
	purgeIPv4 := "request bgp rib source purge-stale owner 701 connection 41 family " + family.IPv4Unicast.String()
	purgeIPv6 := "request bgp rib source purge-stale owner 701 connection 41 family " + family.IPv6Unicast.String()
	grOpen := buildOpenBody(buildCapabilityParam(buildGRCapTLV(0, 120, [][4]byte{
		{0x00, 0x01, 0x01, 0x80}, // IPv4 unicast, F set
		{0x00, 0x02, 0x01, 0x80}, // IPv6 unicast, F set
	})))
	cases := []struct {
		name string
		open []byte
	}{
		{"only capability 65", buildOpenBody(buildCapabilityParam([]byte{65, 4, 0x00, 0x00, 0xFD, 0xE9}))},
		{"no optional parameters", buildOpenBody(nil)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gp, rec := newRecordedGRPlugin()
			connection := uint64(41)
			t.Cleanup(func() { gp.state.removePeer(grFixtureSource(t, testPeer, 701, 41).grPeer) })
			deliver := func(se *rpc.StructuredEvent) {
				se.PeerAddress = testPeer
				se.PeerOwner = 701
				se.ConnectionID = connection
				require.NoError(t, gp.handleStructuredEvent(se))
			}
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindOpen, Direction: rpc.DirectionSent, RawMessage: &bgptypes.RawMessage{RawBytes: grOpen}})
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindOpen, Direction: rpc.DirectionReceived,
				RawMessage: &bgptypes.RawMessage{RawBytes: grOpen}})
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindState, State: rpc.SessionStateDown, Reason: "tcp-failure"})
			before := len(rec.all())
			assert.Equal(t, []string{"request bgp rib source retain-routes owner 701 connection 41 on-down families ipv4/unicast ipv6/unicast", "request bgp rib source mark-stale owner 701 connection 41 restart-time 120"}, rec.all(), "the first session advertised both families; retain and mark its original connection")

			connection = 42
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindOpen, Direction: rpc.DirectionSent, RawMessage: &bgptypes.RawMessage{RawBytes: tc.open}})
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindOpen, Direction: rpc.DirectionReceived,
				RawMessage: &bgptypes.RawMessage{RawBytes: tc.open}})
			deliver(&rpc.StructuredEvent{EventType: rpc.EventKindState, State: rpc.SessionStateUp})

			assert.ElementsMatch(t, []string{purgeIPv4, purgeIPv6}, rec.all()[before:],
				"no Graceful Restart Capability in the re-established session: every retained family is purged")
		})
	}
}
