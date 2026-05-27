package transaction

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSettlementOperation ConfigOperationType = "test-settlement-operation"

// TestExecutorOrdering verifies that the operation executor emits one apply
// event at a time and waits for the matching operation ack before continuing.
//
// VALIDATES: Operations execute in the sorted order supplied to the executor.
// PREVENTS: Operation apply events being fired as an unordered batch.
func TestExecutorOrdering(t *testing.T) {
	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-order")
	var applied []string

	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		applied = append(applied, ev.Operation.ID)
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("bgp"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		applied = append(applied, ev.Operation.ID)
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})

	ops := []ConfigOperation{
		{ID: "iface-add", Owner: "iface", Type: OperationAddInterface, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
		{ID: "peer-add", Owner: "bgp", Type: OperationAddPeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, executor.Execute(ctx, ops))
	assert.Equal(t, []string{"iface-add", "peer-add"}, applied)
}

// TestExecutorRollback verifies that a failed operation triggers operation
// rollback for previously applied operations in reverse execution order.
//
// VALIDATES: Operation failure rolls back already-applied operations in reverse order.
// PREVENTS: Failed operation execution leaving earlier operations applied.
func TestExecutorRollback(t *testing.T) {
	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-rollback")
	var rolledBack []string

	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("bgp"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeError, Error: "bind failed"}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyFailed, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationRollbackFor("iface"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		require.Len(t, ev.Operations, 1)
		rolledBack = append(rolledBack, ev.Operations[0].ID)
		ack := ConfigOperationRollbackAck{TransactionID: ev.TransactionID, Plugin: ev.Operations[0].Owner, OperationID: ev.Operations[0].ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationRollbackOK, ackPayload)
	})

	ops := []ConfigOperation{
		{ID: "iface-add", Owner: "iface", Type: OperationAddInterface, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
		{ID: "peer-add", Owner: "bgp", Type: OperationAddPeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := executor.Execute(ctx, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
	assert.Equal(t, []string{"iface-add"}, rolledBack)
}

// TestExecutorIgnoresStrayApplyAck verifies stale or unrelated operation acks
// cannot fill the executor's ack channel before the matching ack arrives.
//
// VALIDATES: Apply ack subscribers accept only the currently awaited operation ID.
// PREVENTS: A stale ack deadlocking the synchronous event bridge before the real ack is sent.
func TestExecutorIgnoresStrayApplyAck(t *testing.T) {
	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-stray-ack")
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		stray := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: "old-op", Status: CodeOK}
		strayPayload, err := json.Marshal(stray)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, strayPayload)

		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})

	ops := []ConfigOperation{{ID: "iface-add", Owner: "iface", Type: OperationAddInterface, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resultCh := make(chan error, 1)
	go func() { resultCh <- executor.Execute(ctx, ops) }()

	select {
	case err := <-resultCh:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for executor")
	}
}

// TestExecutorSettlement verifies that the executor arms settlement waiters
// before applying an operation and does not advance until readiness arrives.
//
// VALIDATES: Settlement waiters block dependent operation execution until the readiness event is observed.
// PREVENTS: Removing or adding dependent resources before async netlink/BGP side effects have settled.
func TestExecutorSettlement(t *testing.T) {
	registerTestSettlementRule(t)

	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-settlement")
	settled := atomic.Bool{}
	peerAppliedBeforeSettlement := atomic.Bool{}

	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
		go func() {
			time.Sleep(25 * time.Millisecond)
			settled.Store(true)
			gw.emitEvent("interface", "addr-added", `{"address":"192.0.2.1"}`)
		}()
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("bgp"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		if !settled.Load() {
			peerAppliedBeforeSettlement.Store(true)
		}
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})

	ops := []ConfigOperation{
		{ID: "addr-add", Owner: "iface", Type: testSettlementOperation, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1/32"}, Params: ConfigOperationParams{Interface: "eth0", CIDR: "192.0.2.1/32"}},
		{ID: "peer-add", Owner: "bgp", Type: OperationAddPeer, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1", Address: "192.0.2.1"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, executor.Execute(ctx, ops))
	assert.False(t, peerAppliedBeforeSettlement.Load())
}

// TestExecutorSettlementTimeout verifies that an operation whose apply ack
// succeeds is still rolled back when its readiness event never arrives.
//
// VALIDATES: Settlement timeout fails the commit and rolls back completed operations.
// PREVENTS: A successful apply ack promoting config while async side effects never settled.
func TestExecutorSettlementTimeout(t *testing.T) {
	registerTestSettlementRule(t)

	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-settlement-timeout")
	var rolledBack []string

	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationRollbackFor("iface"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		require.Len(t, ev.Operations, 1)
		rolledBack = append(rolledBack, ev.Operations[0].ID)
		ack := ConfigOperationRollbackAck{TransactionID: ev.TransactionID, Plugin: ev.Operations[0].Owner, OperationID: ev.Operations[0].ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationRollbackOK, ackPayload)
	})

	ops := []ConfigOperation{{
		ID:     "addr-add-timeout",
		Owner:  "iface",
		Type:   testSettlementOperation,
		Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.2/32"},
		Params: ConfigOperationParams{Interface: "eth0", CIDR: "192.0.2.2/32"},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := executor.Execute(ctx, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "settlement timeout")
	assert.Equal(t, []string{"addr-add-timeout"}, rolledBack)
}

// TestExecutorSettlementSkipsMissingResource verifies resource-derived
// settlement rules do not wait when an operation has no resource to match.
//
// VALIDATES: Settlement rules with empty derived resources are skipped.
// PREVENTS: BGP peers using local-address auto timing out waiting for an impossible listener-ready match.
func TestExecutorSettlementSkipsMissingResource(t *testing.T) {
	registerTestSettlementRule(t)

	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-settlement-empty-resource")
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, err := json.Marshal(ack)
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})

	ops := []ConfigOperation{{
		ID:     "addr-add-auto",
		Owner:  "iface",
		Type:   testSettlementOperation,
		Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0"},
		Params: ConfigOperationParams{Interface: "eth0"},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, executor.Execute(ctx, ops))
}

func registerTestSettlementRule(t *testing.T) {
	t.Helper()
	err := RegisterSettlementRule(SettlementRule{
		ID:           "test-executor-settlement-add-address",
		Operation:    OperationSelector{Type: testSettlementOperation, ResourceKind: ResourceAddress},
		Readiness:    ConfigOperationReadiness{Namespace: "interface", EventType: "addr-added"},
		ResourceFrom: SettlementResourceAddress,
		Timeout:      75 * time.Millisecond,
	})
	if err != nil {
		assert.Contains(t, err.Error(), "already registered")
	}
}
