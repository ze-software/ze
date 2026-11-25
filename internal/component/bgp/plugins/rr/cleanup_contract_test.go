// Design: docs/architecture/plugin/rib-storage-design.md -- cleanup admission and owed output.
package rr

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRRClosingInventoryRefusesEscapedLivePage seals original B while retained
// A still holds its Session receipt. An escaped newer copy cannot reopen B;
// another prefix, sibling owner and physical successor remain admissible.
func TestRRClosingInventoryRefusesEscapedLivePage(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	t.Cleanup(rr.stopDepartures)
	apply := func(row inventoryRecord) {
		rr.withdrawalMu.Lock()
		defer rr.withdrawalMu.Unlock()
		rr.applyInventoryLocked("192.0.2.1", row)
	}
	const address = "192.0.2.1"
	makeRow := func(owner, connection uint64, last byte) inventoryRecord {
		row, err := nativeInventory(withdrawalInfo{Family: family.IPv4Unicast, SourceOwner: owner, ConnectionID: connection, SourceReceipt: 70, MsgID: 80}, []byte{24, 10, 0, last}, false)
		require.NoError(t, err)
		return row
	}
	a, b := makeRow(701, 41, 0), makeRow(701, 41, 1)
	b.info.NoLLGR = true
	apply(a)
	apply(b)
	payload, err := retention.EncodeCleanup(701, 41, address, []family.Family{family.IPv4Unicast}, true)
	require.NoError(t, err)
	require.NoError(t, rr.cleanupSource(rpc.EventEnvelope{Namespace: retention.Namespace, Event: retention.CleanupEventType, Payload: []byte(payload)}))
	require.Len(t, rr.withdrawals[address], 2, "seal preserves an admitted batch's rows until actual output joins")
	escaped := b
	escaped.info.MsgID = 81
	rr.withdrawalMu.Lock()
	rr.putWithdrawalLocked(address, escaped.key, escaped.info)
	rr.withdrawalMu.Unlock()
	held := rr.withdrawals[address][b.key]
	require.True(t, held.Closing, "late live page cannot re-adopt the closed original route")
	require.Equal(t, uint64(80), held.MsgID)
	sibling, successor := makeRow(702, 41, 1), makeRow(701, 42, 1)
	apply(sibling)
	apply(successor)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.awaitDepartures(ctx, 701))
	require.Contains(t, rr.withdrawals[address], a.key)
	require.Contains(t, rr.withdrawals[address], sibling.key)
	require.Contains(t, rr.withdrawals[address], successor.key)
}

// TestRRUnsettledOutputKeepsCleanupOwed proves exiting a local waiter cannot
// manufacture an applied cleanup. An independent healthy owner still settles.
// This is a settlement contract question, not transport/wire evidence.
func TestRRUnsettledOutputKeepsCleanupOwed(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	source, err := newPeerIdentity("192.0.2.1", 701, 41)
	require.NoError(t, err)
	rr.oweCleanup(source)
	rr.noteWorkFailure("held original relay", errors.New("remote cancellation has no settlement receipt"))
	work, finish := context.WithTimeout(t.Context(), time.Second)
	defer finish()
	require.ErrorContains(t, rr.settleEvents(work), "held original relay remains owed")
	require.Contains(t, rr.cleanupPending, source)
	healthy := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	healthy.oweCleanup(source)
	require.NoError(t, healthy.settleEvents(work))
	require.Empty(t, healthy.cleanupPending)
}

// TestRRNativeOpenKeepsOriginalFamiliesAndRefusesDuplicateMetadata uses the
// native formatter grammar. Malformed metadata cannot mutate either actor.
func TestRRNativeOpenKeepsOriginalFamiliesAndRefusesDuplicateMetadata(t *testing.T) {
	rr := &routeReflector{peers: make(map[peerIdentity]*peerState)}
	require.NoError(t, rr.dispatchText("peer 192.0.2.1 remote as 65001 received open 9 peer-owner 701 connection-id 41 asn 65001 router-id 192.0.2.9 hold-time 90 cap 1 multiprotocol ipv6/unicast cap 65 asn4 65001"))
	source, err := newPeerIdentity("192.0.2.1", 701, 41)
	require.NoError(t, err)
	require.True(t, rr.peers[source].Families[family.IPv6Unicast])
	require.False(t, rr.peers[source].Families[family.IPv4Unicast])
	require.Error(t, rr.dispatchText("peer 192.0.2.1 remote as 65001 received open 10 peer-owner 701 connection-id 41 connection-id 42 asn 65001"))
	require.Error(t, rr.dispatchText("peer 192.0.2.1 remote as 65001 state up peer-owner 701 connection-id 41 session-generation 1041 destination-session 141 reason attached peer-owner 702"))
	require.Len(t, rr.peers, 1)
	require.True(t, rr.peers[source].Families[family.IPv6Unicast])
	require.False(t, rr.peers[source].Up)
}

// TestRRNativeOpenReadsEveryRFC5396ASNSpelling proves the OPEN's `asn` field
// and its four-octet AS capability are read with asn.Parse. Method: dispatch
// one OPEN per field in asdot and check the recorded peer AS, then one whose
// AS is not a number in either spelling. PREVENTS: a decimal-only parse that
// refuses an asdot OPEN, or one that accepts a malformed AS.
func TestRRNativeOpenReadsEveryRFC5396ASNSpelling(t *testing.T) {
	source, err := newPeerIdentity("192.0.2.1", 701, 41)
	require.NoError(t, err)
	for _, line := range []string{
		"peer 192.0.2.1 remote as 65546 received open 9 peer-owner 701 connection-id 41 asn 1.10 router-id 192.0.2.9 hold-time 90",
		"peer 192.0.2.1 remote as 65546 received open 9 peer-owner 701 connection-id 41 asn 23456 router-id 192.0.2.9 hold-time 90 cap 65 asn4 1.10",
	} {
		rr := &routeReflector{peers: make(map[peerIdentity]*peerState)}
		require.NoError(t, rr.dispatchText(line), line)
		require.Equal(t, uint32(65546), rr.peers[source].ASN, line)
	}
	rr := &routeReflector{peers: make(map[peerIdentity]*peerState)}
	require.Error(t, rr.dispatchText("peer 192.0.2.1 remote as 65546 received open 9 peer-owner 701 connection-id 41 asn 1.x router-id 192.0.2.9 hold-time 90"))
	require.Empty(t, rr.peers)
}

// TestRRReplacementInventoryUsesSemanticNativeKey changes only the label
// stack and physical source; that replaces one VPN path. Another original
// owner and a different RD remain useful independent paths.
func TestRRReplacementInventoryUsesSemanticNativeKey(t *testing.T) {
	rr := &routeReflector{withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo)}
	t.Cleanup(rr.stopDepartures)
	apply := func(row inventoryRecord) {
		rr.withdrawalMu.Lock()
		defer rr.withdrawalMu.Unlock()
		rr.applyInventoryLocked("192.0.2.1", row)
	}
	const address = "192.0.2.1"
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	raw := []byte{112, 0, 16, 1, 0, 0, 253, 232, 0, 0, 0, 1, 10, 0, 0}
	record := func(owner, connection uint64, native []byte) inventoryRecord {
		row, err := nativeInventory(withdrawalInfo{Family: fam, SourceOwner: owner, ConnectionID: connection, SourceReceipt: 70, MsgID: 80}, native, false)
		require.NoError(t, err)
		return row
	}
	original := record(701, 41, raw)
	apply(original)
	sibling := record(702, 41, raw)
	apply(sibling)
	raw[2] = 32
	successor := record(701, 42, raw)
	apply(successor)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.awaitDepartures(ctx, 701))
	require.NotContains(t, rr.withdrawals[address], original.key)
	require.Contains(t, rr.withdrawals[address], successor.key)
	require.Contains(t, rr.withdrawals[address], sibling.key)
	raw[11] = 2
	otherRD := record(701, 42, raw)
	apply(otherRD)
	require.Len(t, rr.withdrawals[address], 3)
}

// TestRRCompletedCachedRefusalKeepsWholeTargetSet proves one cached request
// owns every captured destination and its known completed refusal does not
// become unresolved history. This uses the real SDK boundary with a recording
// evaluator; final carrier/permission behavior remains a separate writer proof.
func TestRRCompletedCachedRefusalKeepsWholeTargetSet(t *testing.T) {
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { require.NoError(t, engineEnd.Close()) })
	plugin := sdk.NewWithConn("rr-completed-forward", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { require.NoError(t, plugin.Close()) })
	source, err := newPeerIdentity("192.0.2.1", 701, 41)
	require.NoError(t, err)
	first, err := newPeerIdentity("192.0.2.2", 702, 42)
	require.NoError(t, err)
	second, err := newPeerIdentity("192.0.2.2", 703, 43)
	require.NoError(t, err)
	rr := &routeReflector{plugin: plugin, peers: map[peerIdentity]*peerState{
		source: {Identity: source, Up: true, Internal: true, RRClient: true, DestinationSession: 141},
		first:  {Identity: first, Up: true, Internal: true, RRClient: true, DestinationSession: 142},
		second: {Identity: second, Up: true, Internal: true, RRClient: true, DestinationSession: 143},
	}}
	calls := 0
	bridge.SetForwardCached(func(_ context.Context, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
		calls++
		require.Equal(t, []uint64{80}, input.IDs)
		require.Len(t, input.Targets, 2)
		owners := map[uint64]uint64{}
		for _, target := range input.Targets {
			owners[target.DestinationOwner] = target.DestinationSession
		}
		require.Equal(t, map[uint64]uint64{702: 142, 703: 143}, owners)
		return rpc.CompletedCachedWrite(input.DeadlineMS, errors.New("original destination refused after every target joined")), nil
	})
	bridge.SetReady()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	err = rr.forwardUpdate(ctx, source, 80)
	var joined *rpc.CachedWriteJoinedError
	require.ErrorAs(t, err, &joined, "completed writer failure must preserve its cause")
	rr.noteForwardFailure("ordinary cached reflection", err)
	require.Equal(t, 1, calls, "one whole target set owns this cached ID's single ACK")
	require.NoError(t, rr.unsettledWork())
}
