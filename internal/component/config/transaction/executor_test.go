package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		{ID: "iface-add", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
		{ID: "peer-add", Owner: "bgp", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}},
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
		{ID: "iface-add", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
		{ID: "peer-add", Owner: "bgp", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}},
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

	ops := []ConfigOperation{{ID: "iface-add", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}}}
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
		{ID: "peer-add", Owner: "bgp", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1", Address: "192.0.2.1"}},
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

// TestExecuteRollsBackMixedTransaction covers the rollback of a transaction
// mixing decomposed operations with one coarse node. The decomposed
// operations replay their inverses in reverse order through
// config-operation-rollback. The coarse node does not: its inverse is the
// transaction-wide section rollback the orchestrator publishes when Execute
// returns this error, and its participant answers `config-rollback`, not
// `config-operation-rollback`.
//
// VALIDATES: AC-7 -- rollback reaches both node kinds, each through the callback its owner implements.
// PREVENTS: a coarse node's participant being sent an operation rollback it answers "unknown method" to.
func TestExecuteRollsBackMixedTransaction(t *testing.T) {
	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-mixed-rollback")
	executor.SetSectionDiffs(func(name string) ([]DiffSection, bool) {
		return []DiffSection{{Root: "static", Added: `{"static/route/10.0.0.0/8":{}}`}}, name == "static"
	})
	var sectionApplied []string
	var rolledBack []string

	gw.SubscribeConfigEvent(EventApplyFor("static"), func(payload []byte) {
		var ev ApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		sectionApplied = append(sectionApplied, ev.Diffs[0].Root)
		ack, err := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: "static", Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack, err := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("bgp"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack, err := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeError, Error: "bind failed"})
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyFailed, ack)
	})
	for _, owner := range []string{"iface", "static"} {
		gw.SubscribeConfigEvent(EventOperationRollbackFor(owner), func(payload []byte) {
			var ev ConfigOperationRollbackEvent
			require.NoError(t, json.Unmarshal(payload, &ev))
			require.Len(t, ev.Operations, 1)
			rolledBack = append(rolledBack, ev.Operations[0].ID)
			ack, err := json.Marshal(ConfigOperationRollbackAck{TransactionID: ev.TransactionID, Plugin: ev.Operations[0].Owner, OperationID: ev.Operations[0].ID, Status: CodeOK})
			require.NoError(t, err)
			gw.mustEmit(EventOperationRollbackOK, ack)
		})
	}

	ops := []ConfigOperation{
		{ID: "iface-add", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"}},
		{ID: "section-apply-static", Owner: "static", Type: OperationSectionApply},
		{ID: "peer-add", Owner: "bgp", Type: testOpAddPeer, Verb: VerbCreate, Target: ResourceRef{Kind: ResourcePeer, Peer: "203.0.113.1"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := executor.Execute(ctx, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind failed")
	assert.Equal(t, []string{"static"}, sectionApplied, "the coarse node applied its participant's section")
	assert.Equal(t, []string{"iface-add"}, rolledBack, "only the decomposed operation replays an inverse")
}

// TestExecuteRefusesCoarseNodeWithoutDiffSource fences the coarse node's
// payload. The executor holds no diffs of its own, so a sorted list carrying a
// coarse node with no diff source cannot be applied. It is refused, and never
// applied as an empty section that a participant would read as "delete
// everything I own".
//
// VALIDATES: a coarse node with no installed SectionDiffs aborts, naming the node.
// PREVENTS: an empty section apply passing for a real one.
func TestExecuteRefusesCoarseNodeWithoutDiffSource(t *testing.T) {
	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-exec-coarse-no-source")
	ops := []ConfigOperation{{ID: "section-apply-static", Owner: "static", Type: OperationSectionApply}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := executor.Execute(ctx, ops)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "section-apply-static")
	assert.Empty(t, gw.findEmitted(EventApplyFor("static")), "a node with no diffs emitted an apply anyway")
}

// TestOperationPathCarriesUnknownLabel verifies the operation path orders and
// runs an operation whose label no package in this repository names, and hands
// that label to its owner unchanged. The core reads the verb and the target
// kind; the label is the owner's own word for the work.
//
// VALIDATES: AC-5. A root joins the ordering without a core package learning
// its vocabulary.
// PREVENTS: The core silently dropping, rewriting, or refusing an operation it
// cannot name, which is what a central operation enumeration produced.
func TestOperationPathCarriesUnknownLabel(t *testing.T) {
	const provisionVIP ConfigOperationType = "provision-claim-vip"

	gw := newTestGateway()
	executor := NewOperationExecutor(gw, "tx-unknown-label")

	var verified, applied []ConfigOperationType
	gw.SubscribeConfigEvent(EventOperationVerifyFor("provision"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		verified = append(verified, ev.Operation.Type)
		ack, err := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventOperationVerifyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("provision"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		applied = append(applied, ev.Operation.Type)
		ack, err := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ack)
	})

	// The address the operation consumes is ordered before it by the graph,
	// so the pair also shows the core placing an unknown label by its verb
	// and its target alone.
	ops := []ConfigOperation{
		{ID: "iface-add-address", Owner: "iface", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1/32"}},
		{ID: "provision-claim", Owner: "provision", Type: provisionVIP, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceKind("vip"), Address: "192.0.2.1/32"}},
	}
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack, err := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventOperationApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		require.NoError(t, json.Unmarshal(payload, &ev))
		ack, err := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, Plugin: ev.Operation.Owner, OperationID: ev.Operation.ID, Status: CodeOK})
		require.NoError(t, err)
		gw.mustEmit(EventOperationVerifyOK, ack)
	})

	graph, err := BuildOperationGraph(ops, ConstraintRules())
	require.NoError(t, err)
	sorted, err := TopologicalSort(graph)
	require.NoError(t, err, "an operation the core cannot name is still orderable")
	require.Len(t, sorted, 2)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, executor.Verify(ctx, sorted))
	require.NoError(t, executor.Execute(ctx, sorted))

	assert.Equal(t, []ConfigOperationType{provisionVIP}, verified, "the owner verifies its own label, unchanged")
	assert.Equal(t, []ConfigOperationType{provisionVIP}, applied, "the owner applies its own label, unchanged")
}

// Undo keeps the actual execution order (including interleaved owners), has a
// fresh budget after apply expiry, and attempts earlier inverses after failure.
func TestOperationRollbackOrderBudgetAndErrors(t *testing.T) {
	gateway := newTestGateway()
	executor := NewOperationExecutor(gateway, "original")
	executor.SetDeadlineMS(time.Now().Add(-time.Second).UnixMilli())
	executor.SetRollbackBudget(time.Second)
	ops := []ConfigOperation{{ID: "a-first", Owner: "a"}, {ID: "b-middle", Owner: "b"}, {ID: "a-last", Owner: "a"}}
	var restored []string
	for _, owner := range []string{"a", "b"} {
		gateway.SubscribeConfigEvent(EventOperationRollbackFor(owner), func(payload []byte) {
			var event ConfigOperationRollbackEvent
			require.NoError(t, json.Unmarshal(payload, &event))
			require.Greater(t, event.DeadlineMS, time.Now().UnixMilli())
			require.Equal(t, "original", event.TransactionID)
			require.Len(t, event.Operations, 1)
			id := event.Operations[0].ID
			restored = append(restored, id)
			ack := ConfigOperationRollbackAck{TransactionID: event.TransactionID, Plugin: owner, OperationID: id, Status: CodeOK}
			topic := EventOperationRollbackOK
			if id == "a-last" {
				ack.Status, ack.Error = CodeError, "injected undo failure"
				topic = EventOperationRollbackFailed
			}
			raw, err := json.Marshal(ack)
			require.NoError(t, err)
			gateway.mustEmit(topic, raw)
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := executor.Rollback(ctx, ops, context.DeadlineExceeded)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorContains(t, err, "injected undo failure")
	require.Equal(t, []string{"a-last", "b-middle", "a-first"}, restored)
}

func TestOperationRollbackRestoresSectionsAfterUndo(t *testing.T) {
	for _, restored := range []bool{false, true} {
		t.Run(fmt.Sprintf("restored=%t", restored), func(t *testing.T) {
			gateway := newTestGateway()
			executor := NewOperationExecutor(gateway, "original")
			executor.SetRollbackBudget(time.Second)
			sectionsCalled := false
			undone := false
			sectionErr := errors.New("section restore failed")
			executor.afterRollback = func(ctx context.Context, cause error) error {
				require.NoError(t, ctx.Err())
				require.True(t, undone, "restore infrastructure before dependent sections")
				sectionsCalled = true
				if !restored {
					return errors.Join(cause, sectionErr)
				}
				return cause
			}
			gateway.SubscribeConfigEvent(EventOperationRollbackFor("infrastructure"), func(payload []byte) {
				require.False(t, sectionsCalled)
				undone = true
				var event ConfigOperationRollbackEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				raw, err := json.Marshal(ConfigOperationRollbackAck{
					TransactionID: event.TransactionID, Plugin: "infrastructure", OperationID: "infrastructure-change", Status: CodeOK,
				})
				require.NoError(t, err)
				gateway.mustEmit(EventOperationRollbackOK, raw)
			})
			err := executor.Rollback(t.Context(), []ConfigOperation{{ID: "infrastructure-change", Owner: "infrastructure"}}, context.DeadlineExceeded)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.True(t, undone)
			require.True(t, sectionsCalled)
			if !restored {
				require.ErrorIs(t, err, sectionErr)
			}
		})
	}
}

// VALIDATES: Infrastructure journal undo precedes dependent service restoration.
// PREVENTS: A coarse rollback installer binding before its old device exists.
func TestOperationRollbackRestoresInfrastructureBeforeService(t *testing.T) {
	for _, failure := range []string{"apply", "commit"} {
		t.Run(failure, func(t *testing.T) {
			gateway := newTestGateway()
			participants := []testParticipant{
				{name: "infrastructure", configRoots: []string{"infrastructure"}},
				{name: "service", configRoots: []string{"service"}},
			}
			coordinator := newTestOrchestrator(t, gateway, participants)
			coordinator.SetApplyDeadlineOverride(time.Second)
			ops := []ConfigOperation{
				{ID: "remove-old-device", Root: "infrastructure", Owner: "infrastructure", Type: "remove-device", Verb: VerbDestroy,
					Target: ResourceRef{Kind: ResourceInterface, Name: "old-device"}},
				{ID: "start-dependent-session", Root: "infrastructure", Owner: "infrastructure", Type: testOpAddPeer, Verb: VerbCreate,
					Target: ResourceRef{Kind: ResourcePeer, Peer: "dependent"}},
			}
			coordinator.SetOperationPlanner(func(context.Context, OperationPlanRequest) ([]ConfigOperation, error) {
				return ops, nil
			})
			emit := func(topic string, event any) {
				raw, err := json.Marshal(event)
				require.NoError(t, err)
				gateway.mustEmit(topic, raw)
			}
			for _, participant := range participants {
				gateway.SubscribeConfigEvent(EventVerifyFor(participant.name), func([]byte) {
					participant.respondVerify(gateway, coordinator.TransactionID())
				})
			}
			oldDevice, oldService, candidateService, session := true, true, false, false
			var undone []string
			var serviceRestores int
			gateway.SubscribeConfigEvent(EventOperationVerifyFor("infrastructure"), func(payload []byte) {
				var event ConfigOperationVerifyEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				emit(EventOperationVerifyOK, ConfigOperationVerifyAck{
					TransactionID: event.TransactionID, Plugin: "infrastructure", OperationID: event.Operation.ID, Status: CodeOK,
				})
			})
			gateway.SubscribeConfigEvent(EventOperationApplyFor("infrastructure"), func(payload []byte) {
				var event ConfigOperationApplyEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				ack := ConfigOperationApplyAck{TransactionID: event.TransactionID, Plugin: "infrastructure", OperationID: event.Operation.ID, Status: CodeOK}
				topic := EventOperationApplyOK
				if event.Operation.ID == "remove-old-device" {
					oldDevice = false
				} else {
					require.True(t, candidateService, "later failure must follow coarse service installation")
					if failure == "apply" {
						ack.Status, ack.Error = CodeError, "injected later apply failure"
						topic = EventOperationApplyFailed
					} else {
						session = true
					}
				}
				emit(topic, ack)
			})
			gateway.SubscribeConfigEvent(EventApplyFor("service"), func(payload []byte) {
				var event ApplyEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				require.False(t, oldDevice, "original infrastructure removal must have applied")
				oldService, candidateService = false, true
				emit(EventApplyOK, ApplyAck{TransactionID: event.TransactionID, Plugin: "service", Status: CodeOK})
			})
			gateway.SubscribeConfigEvent(EventOperationCommitFor("infrastructure"), func(payload []byte) {
				var event ConfigOperationCommitEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				emit(EventOperationCommitFailed, ConfigOperationCommitAck{
					TransactionID: event.TransactionID, Plugin: "infrastructure", Status: CodeError, Error: "injected later commit failure",
				})
			})
			gateway.SubscribeConfigEvent(EventOperationRollbackFor("infrastructure"), func(payload []byte) {
				var event ConfigOperationRollbackEvent
				require.NoError(t, json.Unmarshal(payload, &event))
				require.Equal(t, coordinator.TransactionID(), event.TransactionID)
				require.Len(t, event.Operations, 1)
				op := event.Operations[0]
				undone = append(undone, op.ID)
				if op.ID == "remove-old-device" {
					require.Equal(t, ops[0], op, "replay the original removal token")
					oldDevice = true
				} else {
					require.Equal(t, ops[1], op)
					session = false
				}
				emit(EventOperationRollbackOK, ConfigOperationRollbackAck{
					TransactionID: event.TransactionID, Plugin: "infrastructure", OperationID: op.ID, Status: CodeOK,
				})
			})
			gateway.SubscribeConfigEvent(EventRollback, func([]byte) {
				serviceRestores++
				assert.True(t, oldDevice, "old service installer requires its original device")
				candidateService = false
				oldService = oldDevice
				for _, participant := range participants {
					participant.respondRollback(gateway, coordinator.TransactionID())
				}
			})
			result := coordinator.Execute(t.Context(), map[string][]DiffSection{
				"infrastructure": {{Root: "infrastructure", Removed: `{"old-device":{}}`}},
				"service":        {{Root: "service", Changed: `{"binding":{"old":"old-device","new":"any"}}`}},
			})
			require.Equal(t, StateRolledBack, result.State)
			require.ErrorContains(t, result.Err, "injected later "+failure+" failure")
			assert.Equal(t, 1, serviceRestores)
			assert.True(t, oldDevice)
			assert.True(t, oldService, "rollback must reinstall the old running service")
			assert.False(t, candidateService)
			assert.False(t, session)
			wantUndo := []string{"remove-old-device"}
			if failure == "commit" {
				wantUndo = []string{"start-dependent-session", "remove-old-device"}
			}
			assert.Equal(t, wantUndo, undone)
		})
	}
}
