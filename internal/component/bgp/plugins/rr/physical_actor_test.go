// Design: docs/architecture/plugin/rib-storage-design.md -- original reflector sessions.
package rr

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRRPhysicalStateKeepsSiblingAndSuccessorBothInputs delivers original,
// same-address sibling and physical successor through the real typed/native
// TEXT state producers. An original DOWN leaves the useful current actors up.
func TestRRPhysicalStateKeepsSiblingAndSuccessorBothInputs(t *testing.T) {
	for _, textInput := range []bool{false, true} {
		t.Run(fmt.Sprint(textInput), func(t *testing.T) {
			rr := &routeReflector{peers: make(map[peerIdentity]*peerState), withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
			t.Cleanup(rr.stopDepartures)
			feed := func(owner, connection uint64, state rpc.SessionState) {
				if textInput {
					rr.dispatchText(fmt.Sprintf("peer 192.0.2.1 remote as 65001 state %s peer-owner %d connection-id %d destination-session %d session-generation %d reason attached is-ibgp true rr-client true", state, owner, connection, connection+100, connection+1000))
					return
				}
				rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: "192.0.2.1", PeerOwner: owner, ConnectionID: connection,
					DestinationSession: connection + 100, SessionGeneration: connection + 1000, State: state, Reason: rpc.ReasonAttached, IsIBGP: true, RouteReflectorClient: true})
			}
			original, err := newPeerIdentity("192.0.2.1", 701, 41)
			require.NoError(t, err)
			successor, err := newPeerIdentity("192.0.2.1", 701, 42)
			require.NoError(t, err)
			sibling, err := newPeerIdentity("192.0.2.1", 702, 43)
			require.NoError(t, err)
			feed(701, 41, rpc.SessionStateUp)
			feed(702, 43, rpc.SessionStateUp)
			feed(701, 42, rpc.SessionStateUp)
			feed(701, 41, rpc.SessionStateDown)
			settled, finish := context.WithTimeout(t.Context(), time.Second)
			defer finish()
			require.NoError(t, rr.awaitDepartures(settled, 701))
			rr.mu.RLock()
			old := rr.peers[original]
			oldUp := old != nil && old.Up
			rr.mu.RUnlock()
			require.False(t, oldUp)
			require.True(t, rr.peers[successor].Up)
			require.True(t, rr.peers[sibling].Up)
			require.Equal(t, uint64(142), rr.peers[successor].DestinationSession)
			targets := rr.targets(peerIdentity{owner: 999, address: netip.MustParseAddr("192.0.2.9"), connection: 99}, family.Family{})
			require.Len(t, targets, 2)
			feed(701, 40, rpc.SessionStateUp)
			require.NotContains(t, rr.peers, peerIdentity{owner: 701, connection: 40, address: original.address})
		})
	}
}

// TestRRCleanupKeepsExactRetainedRouteWithSharedReceipt removes only original
// B while A keeps the Session-wide receipt. Family, NO_LLGR, sibling owner,
// replacement physical session and another prefix are independent controls.
func TestRRCleanupKeepsExactRetainedRouteWithSharedReceipt(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	const address = "192.0.2.1"
	entries := map[withdrawalKey]withdrawalInfo{}
	add := func(owner, connection uint64, prefix string, fam family.Family, noLLGR bool) rpc.StoredRoute {
		info := withTextNative(withdrawalInfo{
			Family: fam, Prefix: "prefix " + prefix, SourceOwner: owner, ConnectionID: connection,
			SourceReceipt: 70, MsgID: 80, NoLLGR: noLLGR, Retained: true})
		entries[withdrawalKey{owner: owner, connection: connection, route: fam.String() + "|" + prefix}] = info
		return rpc.StoredRoute{SourcePeer: address, SourceOwner: owner, SourceConnectionID: connection,
			SourceReceipt: 70, MsgID: 80, Family: fam.String(), NLRIHex: info.NativeHex}
	}
	a := add(701, 41, "10.0.0.0/24", family.IPv4Unicast, false)
	b := add(701, 41, "10.0.1.0/24", family.IPv4Unicast, true)
	sibling := add(702, 41, "10.0.1.0/24", family.IPv4Unicast, true)
	successor := add(701, 42, "10.0.1.0/24", family.IPv4Unicast, true)
	otherFamily := add(701, 41, "2001:db8::/32", family.IPv6Unicast, true)
	rr.withdrawals[address] = entries
	payload, err := retention.EncodeCleanup(701, 41, address, []family.Family{family.IPv4Unicast}, true)
	require.NoError(t, err)
	require.NoError(t, rr.cleanupSource(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: []byte(payload)}))
	require.False(t, rr.storedMemberLocked(b), "escaped B cannot borrow retained A's session receipt")
	for _, route := range []rpc.StoredRoute{a, sibling, successor, otherFamily} {
		require.True(t, rr.storedMemberLocked(route))
	}
	work, finish := context.WithTimeout(t.Context(), time.Second)
	defer finish()
	require.NoError(t, rr.settleEvents(work))
	require.Len(t, rr.withdrawals[address], 4)
}

// TestRRCleanupSettlementRequiresJoinedForward leaves an actual admitted live
// forward held at the production gate. Cleanup cannot acknowledge it merely
// because the route's inventory row is already gone. The second call proves
// useful settlement once that original work finishes.
func TestRRCleanupSettlementRequiresJoinedForward(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	payload, err := retention.EncodeCleanup(701, 41, "192.0.2.1", nil, false)
	require.NoError(t, err)
	rr.forwards.enter()
	require.NoError(t, rr.cleanupSource(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: []byte(payload)}))
	deadline, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	require.Error(t, rr.settleEvents(deadline))
	require.Len(t, rr.cleanupPending, 1, "a failed application must keep the lease obligation pending")
	rr.forwards.leave()
	work, finish := context.WithTimeout(t.Context(), time.Second)
	defer finish()
	require.NoError(t, rr.settleEvents(work))
	require.Empty(t, rr.cleanupPending)
}
