// Design: docs/architecture/plugin/rib-storage-design.md -- explicit test source identities.
package rr

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"testing"
	"time"
)

// rrOriginalFixture constructs one explicit original physical source for tests.
// Its owner and connection are fixture inputs, never inferred from an address.
func rrOriginalFixture(t *testing.T, address string, owner, connection uint64) peerIdentity {
	t.Helper()
	original, err := newPeerIdentity(address, owner, connection)
	require.NoError(t, err)
	return original
}

// rrOriginalWire applies the inventory question's native bytes with explicit
// original provenance. It does not start a source, grant a lease or dispatch
// output, so inventory-only tests remain questions about native parsing.
func rrOriginalWire(t *testing.T, rr *routeReflector, original peerIdentity, receipt, id uint64, msg *bgptypes.RawMessage) {
	t.Helper()
	msg.SourceOwner = original.owner
	msg.SourceReceipt = receipt
	msg.MessageID = id
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	require.NoError(t, rr.updateWithdrawalMapWire(original.address.String(), msg, original.connection))
}

// rrFixtureLease answers the lease protocol for these test engine instances.
// Tests still acquire through the actual role API before relying on a lease.
func rrFixtureLease(bridge *rpc.DirectBridge) {
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		result, handled, err := rrFixtureLeaseRPC(method, params)
		if handled {
			return result, err
		}
		if method == rpc.MethodDispatchCommand {
			var input rpc.DispatchCommandInput
			if err := json.Unmarshal(params, &input); err != nil {
				return nil, err
			}
			return json.Marshal(rpc.DispatchCommandOutput{Status: statusDone, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS})
		}
		return nil, errors.New("unexpected fixture engine RPC " + method)
	})
}

func rrFixtureLeaseRPC(method string, params json.RawMessage) (json.RawMessage, bool, error) {
	if method == rpc.MethodReportIssue {
		return json.RawMessage(`{}`), true, nil
	}
	if method != rpc.MethodSourceLease {
		return nil, false, nil
	}
	var input rpc.SourceLeaseInput
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, true, err
	}
	result, err := json.Marshal(rpc.SourceLeaseOutput{PeerOwner: input.PeerOwner, SourceReceipt: input.SourceReceipt, Action: input.Action})
	return result, true, err
}

func rrFixtureStart(t *testing.T, rr *routeReflector, original peerIdentity, receipt, generation, session uint64) {
	t.Helper()
	require.NoError(t, rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: original.address.String(), PeerOwner: original.owner,
		ConnectionID: original.connection, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached,
		SessionGeneration: generation, DestinationSession: session}))
	if receipt == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.NoError(t, rr.acquireSourceLease(ctx, original, receipt))
}

// rrFixtureInventory supplies a complete row for tests about replay/validation,
// preserving the actual parser and semantic native key used by production.
func rrFixtureInventory(t *testing.T, rr *routeReflector, original peerIdentity, route rpc.StoredRoute) withdrawalKey {
	t.Helper()
	route.SourcePeer = original.address.String()
	route.SourceOwner = original.owner
	route.SourceConnectionID = original.connection
	route.NLRIFraming = rpc.NLRIFramingPrefixOnly
	key, info, _, err := storedInventory(route)
	require.NoError(t, err)
	rr.withdrawalMu.Lock()
	defer rr.withdrawalMu.Unlock()
	rr.putWithdrawalLocked(route.SourcePeer, key, info)
	return key
}
