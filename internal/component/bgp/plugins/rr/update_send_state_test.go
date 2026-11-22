// Design: docs/architecture/plugin/rib-storage-design.md -- route-reflector original output snapshots.
// Related: actor.go -- receive-only UP still creates a source actor while skipping output targets.
package rr

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRRUpdateSendDenialKeepsSource proves that a peer attached receive-only
// (UpdateSendDenied=true) stays tracked as a live source actor over both
// structured and native text transports, while owing no autonomous replay and
// never being selected as a reflection or departure target.
func TestRRUpdateSendDenialKeepsSource(t *testing.T) {
	for _, transport := range []struct {
		name string
		feed func(rr *routeReflector) error
	}{
		{
			name: "structured",
			feed: func(rr *routeReflector) error {
				return rr.handleStructuredState(&rpc.StructuredEvent{
					PeerAddress:        "192.0.2.1",
					State:              rpc.SessionStateUp,
					PeerOwner:          701,
					ConnectionID:       41,
					DestinationSession: 141,
					SessionGeneration:  1041,
					InitialReplay:      141,
					UpdateSendDenied:   true,
				})
			},
		},
		{
			name: "native text",
			feed: func(rr *routeReflector) error {
				return rr.dispatchText("peer 192.0.2.1 remote as 65001 state up peer-owner 701 connection-id 41 destination-session 141 session-generation 1041 initial-replay 141 update-send-denied true")
			},
		},
	} {
		t.Run(transport.name, func(t *testing.T) {
			rr := &routeReflector{}
			require.NoError(t, transport.feed(rr))

			denied := rrOriginalFixture(t, "192.0.2.1", 701, 41)
			state := rr.peers[denied]
			require.NotNil(t, state)
			require.True(t, state.Up)
			require.True(t, state.UpdateSendDenied)
			require.Equal(t, uint64(141), state.DestinationSession)
			require.Zero(t, state.ReplayGen, "receive-only peer owes no autonomous UP replay")

			permitted := rrOriginalFixture(t, "192.0.2.2", 702, 42)
			require.NoError(t, rr.handleStructuredState(&rpc.StructuredEvent{
				PeerAddress:        "192.0.2.2",
				State:              rpc.SessionStateUp,
				Reason:             rpc.ReasonAttached,
				PeerOwner:          702,
				ConnectionID:       42,
				DestinationSession: 142,
				SessionGeneration:  1042,
			}))

			third := rrOriginalFixture(t, "192.0.2.3", 703, 43)
			targets := rr.targets(third, family.Family{})
			require.Len(t, targets, 1)
			require.Equal(t, permitted, targets[0].identity)

			require.False(t, rr.targetCurrent(peerTarget{identity: denied, session: 141, generation: 0}))
			require.True(t, rr.targetCurrent(peerTarget{identity: permitted, session: 142, generation: 0}))
		})
	}
}
