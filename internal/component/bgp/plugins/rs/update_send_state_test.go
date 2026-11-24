// Design: docs/architecture/plugin/rib-storage-design.md -- route-server original output snapshots.
// Related: server_handlers.go -- receive-only UP still creates a source actor.
package rs

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRSUpdateSendDenialKeepsSource checks both state representations. The
// receive-only actor can acquire its original route lease and send useful
// routes through an authorized recipient. It owes no autonomous replay, live
// delivery, or departure as a recipient. Explicit engine handback still joins.
func TestRSUpdateSendDenialKeepsSource(t *testing.T) {
	for _, transport := range []struct {
		name  string
		event *Event
	}{
		{"structured", parseStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.1",
			State: rpc.SessionStateUp, PeerOwner: 701, ConnectionID: 41, DestinationSession: 141,
			SessionGeneration: 1041, InitialReplay: 141, UpdateSendDenied: true, RSClient: true})},
		{"native text", parseTextState("peer 192.0.2.1 remote as 65001 state up peer-owner 701 connection-id 41" +
			" destination-session 141 session-generation 1041 initial-replay 141 update-send-denied true rs-client true")},
	} {
		t.Run(transport.name, func(t *testing.T) {
			require.NotNil(t, transport.event)
			var lease rpc.SourceLeaseInput
			rs := &routeServer{peers: make(map[workerKey]*PeerState),
				sourceLeaseHook: func(_ context.Context, input rpc.SourceLeaseInput) error {
					lease = input
					return nil
				}}
			rs.handleState(transport.event)
			origin := rsFixtureKey("192.0.2.1", 701, 41)
			require.True(t, rs.peers[origin].Up)
			require.Equal(t, uint64(141), rs.peers[origin].DestinationSession)
			require.Empty(t, rs.replays.snapshot())
			require.NoError(t, rs.acquireSource(origin, 1001))
			require.Equal(t, rpc.SourceLeaseInput{SourceReceipt: 1001, PeerOwner: 701, Action: rpc.SourceLeaseAcquire}, lease)

			rs.handleState(&Event{PeerAddr: "192.0.2.2", State: "up", Reason: rpc.ReasonAttached, RSClient: true,
				PeerOwner: 702, ConnectionID: 42, DestinationSession: 142, SessionGeneration: 1042})
			permitted := rpc.ForwardTarget{Destination: "192.0.2.2", DestinationOwner: 702, DestinationSession: 142}
			denied := rpc.ForwardTarget{Destination: "192.0.2.1", DestinationOwner: 701, DestinationSession: 141}
			third := rsFixtureKey("192.0.2.3", 703, 43)
			families := map[family.Family]bool{family.IPv4Unicast: true}
			require.Equal(t, []rpc.ForwardTarget{permitted}, rs.selectForwardTargets(nil, origin, 5, families))
			require.Equal(t, []rpc.ForwardTarget{permitted}, rs.selectForwardTargets(nil, third, 5, families))
			require.True(t, rs.fastPathOwedTarget(denied, third, 5, families), "explicit engine handback keeps its definitive result path")

			rs.peers[third] = &PeerState{}
			key := withdrawalKey{owner: 703, fam: family.IPv4Unicast, prefix: netip.MustParsePrefix("192.0.2.0/24")}
			rs.recordDeparture(third, map[withdrawalKey]withdrawalEntry{key: {connectionID: 43, owner: 703}})
			require.Equal(t, map[rpc.ForwardTarget]error{permitted: nil}, rs.peers[third].departure.targets)
			require.NoError(t, rs.unknownOutputFailure())
		})
	}
}
