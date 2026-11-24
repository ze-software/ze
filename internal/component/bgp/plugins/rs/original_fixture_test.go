// Design: docs/architecture/plugin/rib-storage-design.md -- original RS test actors.
package rs

import (
	"context"
	"fmt"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rsFixtureKey constructs only from the original identities written at the
// fixture call site. An address carries no authority and cannot select IDs.
func rsFixtureKey(address string, owner, physical uint64) workerKey {
	key, err := newWorkerKey(address, owner, physical)
	if err != nil {
		panic("BUG: invalid explicit RS fixture actor: " + err.Error())
	}
	return key
}

func rsFixtureTarget(address string, owner, session uint64) rpc.ForwardTarget {
	return rpc.ForwardTarget{Destination: address, DestinationOwner: owner, DestinationSession: session}
}

// rsFixtureAddresses keeps old endpoint-only assertions readable while the
// production selector returns its complete captured output tuples.
func rsFixtureAddresses(targets []rpc.ForwardTarget) []string {
	addresses := make([]string, 0, len(targets))
	for _, target := range targets {
		addresses = append(addresses, target.Destination)
	}
	return addresses
}

func rsFixtureUpdate(key workerKey, receipt, epoch, msgID uint64, body string) string {
	return fmt.Sprintf("peer %s remote as 65001 received update %d peer-owner %d connection-id %d source-receipt %d source-policy-epoch %d %s", key.address, msgID, key.owner, key.connectionID, receipt, epoch, body)
}

// rsFixtureRetainedRoute creates a known native row for a unit replay question.
// The caller provides every original numeric fact; the existing SDK lease
// operation runs against the test's evaluator before any retained state exists.
func rsFixtureRetainedRoute(t *testing.T, rs *routeServer, key workerKey, receipt, epoch, message uint64, nlri string) rpc.StoredRoute {
	t.Helper()
	rs.mu.Lock()
	if rs.peers[key] == nil {
		rs.peers[key] = &PeerState{Address: key.address.String(), Up: true, RSClient: true, StateSeen: true}
	}
	rs.mu.Unlock()
	if err := rs.acquireSource(key, receipt); err != nil {
		t.Fatal(err)
	}
	row := rpc.StoredRoute{SourcePeer: key.address.String(), SourceOwner: key.owner, SourceConnectionID: key.connectionID, SourceReceipt: receipt, SourcePolicyEpoch: epoch, MsgID: message, Family: family.IPv4Unicast.String(), NLRIFraming: rpc.NLRIFramingPrefixOnly, NLRIHex: nlri}
	native, err := storedKey(row, false)
	if err != nil {
		t.Fatal(err)
	}
	rs.withdrawalMu.Lock()
	if rs.withdrawals[key.address.String()] == nil {
		rs.withdrawals[key.address.String()] = make(map[withdrawalKey]withdrawalEntry)
	}
	rs.withdrawals[key.address.String()][native] = withdrawalEntry{owner: key.owner, connectionID: key.connectionID, sourceReceipt: receipt, sourcePolicyEpoch: epoch, msgID: message, retained: true}
	rs.withdrawalMu.Unlock()
	rs.mu.Lock()
	rs.peers[key].Up = false
	rs.peers[key].sourceRetained = true
	rs.mu.Unlock()
	return row
}

func rsFixtureSourceLeaseEvaluator(bridge *rpc.DirectBridge) {
	bridge.SetDispatchNative(rsFixtureNativeOperation)
}

func rsFixtureSourceProgress(route ribevents.ValidationRoute, minimum uint64) (bool, error) {
	if route.PeerOwner != 1 {
		return false, fmt.Errorf("unit provider has no original owner %d", route.PeerOwner)
	}
	if route.ConnectionID != 101 {
		return false, fmt.Errorf("unit provider has no original physical connection %d", route.ConnectionID)
	}
	return minimum <= 1000, nil
}

func rsFixtureCommandInput(value any) (rpc.DispatchCommandInput, error) {
	switch input := value.(type) {
	case rpc.DispatchCommandInput:
		return input, nil
	case *rpc.DispatchCommandInput:
		if input != nil {
			return *input, nil
		}
	}
	return rpc.DispatchCommandInput{}, fmt.Errorf("unit bound command input has shape %T", value)
}

func rsFixtureBoundCommand(input rpc.DispatchCommandInput) *rpc.DispatchCommandOutput {
	return &rpc.DispatchCommandOutput{Status: statusDone, PeerOwner: input.PeerOwner, ConnectionID: input.ConnectionID, DestinationOwner: input.DestinationOwner, DestinationSession: input.DestinationSession, DeadlineMS: input.DeadlineMS}
}

func rsFixtureNativeOperation(_ context.Context, method string, value any) (any, bool, error) {
	switch method {
	case rpc.MethodSourceLease:
		input, ok := value.(rpc.SourceLeaseInput)
		if !ok {
			return nil, true, fmt.Errorf("unit source lease input has shape %T", value)
		}
		return &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}, true, nil
	case rpc.MethodDispatchCommand:
		input, err := rsFixtureCommandInput(value)
		if err != nil {
			return nil, true, err
		}
		return rsFixtureBoundCommand(input), true, nil
	default:
		return nil, false, nil
	}
}

// rsFixtureNativeActors supplies the original source and its actual useful
// recipient before direct native inventory operations. No address selects IDs.
func rsFixtureNativeActors(rs *routeServer, source workerKey, target workerKey, session uint64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.peers[source] = &PeerState{Address: source.address.String(), Up: true, RSClient: true, StateSeen: true}
	rs.peers[target] = &PeerState{Address: target.address.String(), Up: true, RSClient: true, StateSeen: true, DestinationSession: session, SessionGeneration: 3}
}

func rsFixtureOriginalRecords(records *nlriRecords, source workerKey, receipt, epoch uint64) {
	records.sourceOwner = source.owner
	records.sourceConnection = source.connectionID
	records.sourceReceipt = receipt
	records.sourcePolicyEpoch = epoch
}

// rsFixtureCurrentFlowSpec is the original current receive membership for a
// validation-order question. It acquires the SDK lease and records the exact
// original path; it does not manufacture a cold provider's positive answer.
func rsFixtureCurrentFlowSpec(t *testing.T, rs *routeServer, source workerKey, receipt, epoch, message uint64, fam family.Family, nlri string) {
	t.Helper()
	if err := rs.acquireSource(source, receipt); err != nil {
		t.Fatal(err)
	}
	route := rpc.StoredRoute{SourcePeer: source.address.String(), SourceOwner: source.owner, SourceConnectionID: source.connectionID, SourceReceipt: receipt, SourcePolicyEpoch: epoch, MsgID: message, Family: fam.String(), NLRIFraming: rpc.NLRIFramingPrefixOnly, NLRIHex: nlri}
	key, err := storedKey(route, false)
	if err != nil {
		t.Fatal(err)
	}
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	if rs.withdrawals[source.address.String()] == nil {
		rs.withdrawals[source.address.String()] = make(map[withdrawalKey]withdrawalEntry)
	}
	rs.withdrawals[source.address.String()][key] = withdrawalEntry{owner: source.owner, connectionID: source.connectionID, sourceReceipt: receipt, sourcePolicyEpoch: epoch, msgID: message}
}
