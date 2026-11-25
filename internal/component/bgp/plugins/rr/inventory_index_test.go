// Design: docs/architecture/plugin/rib-storage-design.md -- Route-reflector indexed withdrawal inventory.
// Related: stored_membership.go -- O(1) route-connection and native-NLRI lookup indexes.
package rr

import (
	"context"
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: putWithdrawalLocked, applyInventoryLocked, storedMemberLocked,
// fillValidationReceipt, and sourceMemberLocked stay synchronized via their O(1)
// indexes across bulk inserts, NativeHex replacements, connection supersession,
// and withdrawals.
// PREVENTS: quadratic full-map scans per NLRI on ingest, withdrawal, or replay,
// and stale nativeIndex entries after a route replaces its wire bytes.
func TestWithdrawalIndexTracksPutSupersedeWithdrawAndLookup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rr := &routeReflector{
		peers:       make(map[peerIdentity]*peerState),
		withdrawals: make(map[string]map[withdrawalKey]withdrawalInfo),
		downCtx:     ctx,
	}
	defer rr.stopDepartures()

	const addr = "192.0.2.10"
	const owner uint64 = 71
	oldSource, err := newPeerIdentity(addr, owner, 10)
	require.NoError(t, err)
	newSource, err := newPeerIdentity(addr, owner, 11)
	require.NoError(t, err)

	rr.withdrawalMu.Lock()

	// Bulk insert 1000 routes on oldSource via the indexed path.
	const total = 1000
	for i := range total {
		raw := []byte{24, 10, byte(i >> 8), byte(i)}
		nativeHex := hex.EncodeToString(raw)
		routeKey := "ipv4/unicast|0|" + nativeHex
		key := withdrawalKey{owner: owner, connection: oldSource.connection, route: routeKey}
		info := withdrawalInfo{
			Family:            family.IPv4Unicast,
			NativeHex:         nativeHex,
			SourceOwner:       owner,
			ConnectionID:      oldSource.connection,
			SourceReceipt:     501,
			SourcePolicyEpoch: 3,
			MsgID:             uint64(i + 1),
		}
		rr.putWithdrawalLocked(addr, key, info)
	}
	require.Equal(t, total, rr.addressTracked[addr])
	require.True(t, rr.sourceMemberLocked(oldSource))
	require.False(t, rr.sourceMemberLocked(newSource))

	// Replace route 0 in-place with different NativeHex (e.g., relabeled route):
	// old NativeHex must leave nativeIndex and new NativeHex must resolve.
	route0RawOld := []byte{24, 10, 0, 0}
	route0HexOld := hex.EncodeToString(route0RawOld)
	route0HexNew := hex.EncodeToString([]byte{24, 10, 250, 0})
	key0 := withdrawalKey{owner: owner, connection: oldSource.connection, route: "ipv4/unicast|0|" + route0HexOld}
	rr.putWithdrawalLocked(addr, key0, withdrawalInfo{
		Family:            family.IPv4Unicast,
		NativeHex:         route0HexNew,
		SourceOwner:       owner,
		ConnectionID:      oldSource.connection,
		SourceReceipt:     501,
		SourcePolicyEpoch: 4,
		MsgID:             2000,
	})
	require.Equal(t, total, rr.addressTracked[addr])
	require.False(t, rr.storedMemberLocked(rpc.StoredRoute{
		SourcePeer: addr, SourceOwner: owner, SourceConnectionID: oldSource.connection,
		SourceReceipt: 501, MsgID: 1, Family: family.IPv4Unicast.String(), NLRIHex: route0HexOld,
	}))
	require.True(t, rr.storedMemberLocked(rpc.StoredRoute{
		SourcePeer: addr, SourceOwner: owner, SourceConnectionID: oldSource.connection,
		SourceReceipt: 501, MsgID: 2000, Family: family.IPv4Unicast.String(), NLRIHex: route0HexNew,
	}))

	// Newer connection supersedes route 1; older connection cannot overwrite it back.
	route1Hex := hex.EncodeToString([]byte{24, 10, 0, 1})
	route1Key := "ipv4/unicast|0|" + route1Hex
	newKey1 := withdrawalKey{owner: owner, connection: newSource.connection, route: route1Key}
	rr.putWithdrawalLocked(addr, newKey1, withdrawalInfo{
		Family:            family.IPv4Unicast,
		NativeHex:         route1Hex,
		SourceOwner:       owner,
		ConnectionID:      newSource.connection,
		SourceReceipt:     502,
		SourcePolicyEpoch: 5,
		MsgID:             3001,
	})
	// Attempting to re-put on oldSource must be ignored.
	oldKey1 := withdrawalKey{owner: owner, connection: oldSource.connection, route: route1Key}
	rr.putWithdrawalLocked(addr, oldKey1, withdrawalInfo{
		Family: family.IPv4Unicast, NativeHex: route1Hex,
		SourceOwner: owner, ConnectionID: oldSource.connection, SourceReceipt: 501, MsgID: 9999,
	})
	require.Equal(t, total, rr.addressTracked[addr])
	require.True(t, rr.sourceMemberLocked(newSource))
	require.True(t, rr.storedMemberLocked(rpc.StoredRoute{
		SourcePeer: addr, SourceOwner: owner, SourceConnectionID: newSource.connection,
		SourceReceipt: 502, MsgID: 3001, Family: family.IPv4Unicast.String(), NLRIHex: route1Hex,
	}))

	// fillValidationReceipt must resolve via the O(1) nativeIndex.
	rr.withdrawalMu.Unlock()
	stored := rpc.StoredRoute{
		SourcePeer: addr, SourceOwner: owner, SourceConnectionID: newSource.connection,
		Family: family.IPv4Unicast.String(), NLRIHex: route1Hex,
	}
	rr.fillValidationReceipt(&stored)
	require.Equal(t, uint64(502), stored.SourceReceipt)
	require.Equal(t, uint64(3001), stored.MsgID)
	require.Equal(t, uint64(5), stored.SourcePolicyEpoch)

	// applyInventoryLocked withdrawal: stale MsgID ignored, current MsgID deletes.
	rr.withdrawalMu.Lock()
	rr.applyInventoryLocked(addr, inventoryRecord{
		key:      newKey1,
		info:     withdrawalInfo{MsgID: 3000},
		withdraw: true,
	})
	require.Contains(t, rr.withdrawals[addr], newKey1)

	rr.applyInventoryLocked(addr, inventoryRecord{
		key:      newKey1,
		info:     withdrawalInfo{MsgID: 3001},
		withdraw: true,
	})
	require.NotContains(t, rr.withdrawals[addr], newKey1)
	require.False(t, rr.sourceMemberLocked(newSource))
	require.Equal(t, total-1, rr.addressTracked[addr])

	// Withdraw all remaining oldSource routes via applyInventoryLocked.
	for i := range total {
		if i == 1 {
			continue
		}
		raw := []byte{24, 10, byte(i >> 8), byte(i)}
		routeKey := "ipv4/unicast|0|" + hex.EncodeToString(raw)
		key := withdrawalKey{owner: owner, connection: oldSource.connection, route: routeKey}
		rr.applyInventoryLocked(addr, inventoryRecord{
			key:      key,
			info:     withdrawalInfo{MsgID: 5000 + uint64(i)},
			withdraw: true,
		})
	}
	require.Empty(t, rr.withdrawals[addr], "bucket should clear after withdrawing all "+strconv.Itoa(total)+" routes")
	require.Zero(t, rr.addressTracked[addr])
	require.False(t, rr.sourceMemberLocked(oldSource))
	rr.withdrawalMu.Unlock()
}
