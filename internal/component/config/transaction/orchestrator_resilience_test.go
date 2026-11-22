// Design: docs/architecture/config/transaction-protocol.md -- deadline enforcement, detached rollback, ack deduplication
// Related: orchestrator.go -- runOperationPath, rollBack, runVerify, runApply

package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestOperationPathEnforcesVerifyDeadline proves runOperationPath bounds
// executor.Verify with computeVerifyDeadline even when the caller context has
// a much longer timeout.
//
// VALIDATES: operation verify aborts on the computed verify deadline.
// PREVENTS: a silent operation verify handler hanging until caller timeout.
func TestOperationPathEnforcesVerifyDeadline(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetVerifyDeadline(120 * time.Millisecond)
	orch.SetOperationPlanner(func(context.Context, OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:     "op-verify-hang",
			Root:   "interface",
			Owner:  "iface",
			Type:   testOpAddInterface,
			Verb:   VerbCreate,
			Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
		}}, nil
	})
	gw.SubscribeConfigEvent(EventVerifyFor("iface"), func([]byte) {
		p1.respondVerify(gw, orch.TransactionID())
	})
	// Intentionally register no responder for EventOperationVerifyFor("iface").

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result := orch.Execute(ctx, map[string][]DiffSection{"interface": {{Root: "interface"}}})
	elapsed := time.Since(start)

	if result.State != StateAborted {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateAborted)
	}
	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", result.Err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("operation verify took %v, expected abort near 120ms deadline", elapsed)
	}
	if len(gw.findEmitted(EventVerifyAbort)) == 0 {
		t.Fatal("verify-abort was not emitted after operation verify timeout")
	}
}

// TestOperationPathEnforcesApplyDeadline proves runOperationPath bounds
// executor.Execute with computeApplyDeadline and triggers rollback on expiry.
//
// VALIDATES: operation apply times out on the computed apply deadline and rolls back.
// PREVENTS: a silent operation apply handler wedging the transaction indefinitely.
func TestOperationPathEnforcesApplyDeadline(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetApplyDeadlineOverride(120 * time.Millisecond)
	orch.SetOperationPlanner(func(context.Context, OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:     "op-apply-hang",
			Root:   "interface",
			Owner:  "iface",
			Type:   testOpAddInterface,
			Verb:   VerbCreate,
			Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
		}}, nil
	})
	gw.SubscribeConfigEvent(EventVerifyFor("iface"), func([]byte) {
		p1.respondVerify(gw, orch.TransactionID())
	})
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		raw, err := json.Marshal(ConfigOperationVerifyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		gw.mustEmit(EventOperationVerifyOK, raw)
	})
	// Intentionally register no responder for EventOperationApplyFor("iface").
	gw.SubscribeConfigEvent(EventRollback, func([]byte) {
		p1.respondRollback(gw, orch.TransactionID())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result := orch.Execute(ctx, map[string][]DiffSection{"interface": {{Root: "interface"}}})
	elapsed := time.Since(start)

	if result.State != StateRolledBack {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateRolledBack)
	}
	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", result.Err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("operation apply took %v, expected timeout near 120ms deadline", elapsed)
	}
	if len(gw.findEmitted(EventRollback)) == 0 {
		t.Fatal("rollback was not emitted after operation apply timeout")
	}
}

// TestOperationPathEnforcesCommitDeadline proves runOperationPath bounds
// executor.Commit with computeApplyDeadline and rolls back applied operations
// when commit does not acknowledge in time.
//
// VALIDATES: operation commit timeout triggers operation and section rollback.
// PREVENTS: a silent commit handler blocking past the apply deadline.
func TestOperationPathEnforcesCommitDeadline(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetApplyDeadlineOverride(150 * time.Millisecond)
	orch.SetOperationPlanner(func(context.Context, OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:     "op-commit-hang",
			Root:   "interface",
			Owner:  "iface",
			Type:   testOpAddInterface,
			Verb:   VerbCreate,
			Target: ResourceRef{Kind: ResourceInterface, Name: "eth0"},
		}}, nil
	})
	gw.SubscribeConfigEvent(EventVerifyFor("iface"), func([]byte) {
		p1.respondVerify(gw, orch.TransactionID())
	})
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		raw, err := json.Marshal(ConfigOperationVerifyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		gw.mustEmit(EventOperationVerifyOK, raw)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		raw, err := json.Marshal(ConfigOperationApplyAck{
			TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK,
		})
		if err != nil {
			return
		}
		gw.mustEmit(EventOperationApplyOK, raw)
	})
	// Intentionally register no responder for EventOperationCommitFor("iface").
	var opRolledBack []string
	gw.SubscribeConfigEvent(EventOperationRollbackFor("iface"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		for _, op := range ev.Operations {
			opRolledBack = append(opRolledBack, op.ID)
			raw, err := json.Marshal(ConfigOperationRollbackAck{
				TransactionID: ev.TransactionID, Plugin: "iface", OperationID: op.ID, Status: CodeOK,
			})
			if err != nil {
				return
			}
			gw.mustEmit(EventOperationRollbackOK, raw)
		}
	})
	gw.SubscribeConfigEvent(EventRollback, func([]byte) {
		p1.respondRollback(gw, orch.TransactionID())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result := orch.Execute(ctx, map[string][]DiffSection{"interface": {{Root: "interface"}}})
	elapsed := time.Since(start)

	if result.State != StateRolledBack {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateRolledBack)
	}
	if !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", result.Err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("operation commit took %v, expected timeout near 150ms deadline", elapsed)
	}
	if !slices.Equal(opRolledBack, []string{"op-commit-hang"}) {
		t.Fatalf("operation rollback = %v, want [op-commit-hang]", opRolledBack)
	}
}

// TestRollbackCollectsAcksAndRestartsBrokenPluginAfterCallerCancel proves that
// when caller cancellation ends runApply (non-shutdown), rollBack detaches the
// canceled context under computeRollbackDeadline, still waits for rollback
// acks, and still restarts CodeBroken participants.
//
// VALIDATES: rollBack detaches caller cancellation before collectRollbackAcks.
// PREVENTS: an apply timeout or caller cancellation skipping rollback ack
// collection and leaving a CodeBroken plugin un-restarted.
func TestRollbackCollectsAcksAndRestartsBrokenPluginAfterCallerCancel(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, rollbackCode: CodeBroken}

	var mu sync.Mutex
	var restarted []string
	restartFn := func(name string) error {
		mu.Lock()
		restarted = append(restarted, name)
		mu.Unlock()
		return nil
	}
	orch, err := NewTxCoordinator(gw, []Participant{
		{Name: "bgp", ConfigRoots: []string{"bgp"}},
		{Name: "iface", ConfigRoots: []string{"interface"}},
	}, restartFn)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}
	orch.SetApplyDeadlineOverride(time.Second)

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("bgp"))
	cancel()

	waitForEmit(t, gw, EventRollback)
	// Deliver rollback acks asynchronously after a brief delay so a buggy
	// collectRollbackAcks honoring the canceled ctx would already have exited.
	time.Sleep(30 * time.Millisecond)
	p1.respondRollback(gw, orch.TransactionID())
	p2.respondRollback(gw, orch.TransactionID())

	var result *TxResult
	select {
	case result = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not finish rollback")
	}

	if result.State != StateRolledBack {
		t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateRolledBack)
	}
	if !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("err = %v, want it to wrap context.Canceled", result.Err)
	}
	if !strings.Contains(result.Err.Error(), "iface could not put its running config back") {
		t.Fatalf("err = %q, want rollback outcome naming iface", result.Err.Error())
	}
	mu.Lock()
	gotRestarted := append([]string(nil), restarted...)
	mu.Unlock()
	if !slices.Equal(gotRestarted, []string{"iface"}) {
		t.Fatalf("restarted = %v, want [iface]", gotRestarted)
	}
}

// TestRunVerifyDeduplicatesParticipantAcks proves that duplicate verify-ok
// acks from one participant cannot complete runVerify while another active
// participant has not answered.
//
// VALIDATES: runVerify tracks an expected participant set rather than a raw counter.
// PREVENTS: duplicate acks from plugin A letting apply start while plugin B is
// still verifying or hung.
func TestRunVerifyDeduplicatesParticipantAcks(t *testing.T) {
	t.Run("duplicate does not finish without second participant", func(t *testing.T) {
		gw := newTestGateway()
		p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
		p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
		orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})
		orch.SetVerifyDeadline(150 * time.Millisecond)

		diffs := map[string][]DiffSection{
			"bgp":       {{Root: "bgp"}},
			"interface": {{Root: "interface"}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resultCh := make(chan *TxResult, 1)
		go func() { resultCh <- orch.Execute(ctx, diffs) }()

		waitForEmit(t, gw, EventVerifyFor("bgp"))
		// Send three duplicate verify-ok acks from p1 and none from p2.
		p1.respondVerify(gw, orch.TransactionID())
		p1.respondVerify(gw, orch.TransactionID())
		p1.respondVerify(gw, orch.TransactionID())

		var result *TxResult
		select {
		case result = <-resultCh:
		case <-ctx.Done():
			t.Fatal("timed out waiting for verify timeout")
		}

		if result.State != StateAborted {
			t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateAborted)
		}
		if len(gw.findEmitted(EventApplyFor("bgp"))) != 0 {
			t.Fatal("apply was emitted even though iface never verified")
		}
	})

	t.Run("duplicate does not crowd out second participant", func(t *testing.T) {
		gw := newTestGateway()
		p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
		p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
		orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

		diffs := map[string][]DiffSection{
			"bgp":       {{Root: "bgp"}},
			"interface": {{Root: "interface"}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resultCh := make(chan *TxResult, 1)
		go func() { resultCh <- orch.Execute(ctx, diffs) }()

		waitForEmit(t, gw, EventVerifyFor("bgp"))
		// Send duplicate p1 acks followed by p2's real ack.
		p1.respondVerify(gw, orch.TransactionID())
		p1.respondVerify(gw, orch.TransactionID())
		p1.respondVerify(gw, orch.TransactionID())
		p2.respondVerify(gw, orch.TransactionID())

		waitForEmit(t, gw, EventApplyFor("bgp"))
		p1.respondApply(gw, orch.TransactionID())
		p2.respondApply(gw, orch.TransactionID())

		select {
		case result := <-resultCh:
			if result.State != StateCommitted {
				t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateCommitted)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for commit")
		}
	})
}

// TestRunApplyDeduplicatesParticipantAcks proves that duplicate apply-ok acks
// from one participant cannot commit runApply while another active participant
// has not finished applying.
//
// VALIDATES: runApply tracks an expected participant set rather than a raw counter.
// PREVENTS: duplicate acks from plugin A committing while plugin B is still
// applying or hung.
func TestRunApplyDeduplicatesParticipantAcks(t *testing.T) {
	t.Run("duplicate does not commit without second participant", func(t *testing.T) {
		gw := newTestGateway()
		p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
		p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
		orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})
		orch.SetApplyDeadlineOverride(150 * time.Millisecond)

		diffs := map[string][]DiffSection{
			"bgp":       {{Root: "bgp"}},
			"interface": {{Root: "interface"}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resultCh := make(chan *TxResult, 1)
		go func() { resultCh <- orch.Execute(ctx, diffs) }()

		waitForEmit(t, gw, EventVerifyFor("bgp"))
		p1.respondVerify(gw, orch.TransactionID())
		p2.respondVerify(gw, orch.TransactionID())

		waitForEmit(t, gw, EventApplyFor("bgp"))
		// Send three duplicate apply-ok acks from p1 and none from p2.
		p1.respondApply(gw, orch.TransactionID())
		p1.respondApply(gw, orch.TransactionID())
		p1.respondApply(gw, orch.TransactionID())

		waitForEmit(t, gw, EventRollback)
		p1.respondRollback(gw, orch.TransactionID())
		p2.respondRollback(gw, orch.TransactionID())

		var result *TxResult
		select {
		case result = <-resultCh:
		case <-ctx.Done():
			t.Fatal("timed out waiting for apply timeout rollback")
		}

		if result.State != StateRolledBack {
			t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateRolledBack)
		}
		if len(gw.findEmitted(EventCommitted)) != 0 {
			t.Fatal("committed was emitted even though iface never applied")
		}
	})

	t.Run("duplicate does not crowd out second participant", func(t *testing.T) {
		gw := newTestGateway()
		p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
		p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
		orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

		diffs := map[string][]DiffSection{
			"bgp":       {{Root: "bgp"}},
			"interface": {{Root: "interface"}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resultCh := make(chan *TxResult, 1)
		go func() { resultCh <- orch.Execute(ctx, diffs) }()

		waitForEmit(t, gw, EventVerifyFor("bgp"))
		p1.respondVerify(gw, orch.TransactionID())
		p2.respondVerify(gw, orch.TransactionID())

		waitForEmit(t, gw, EventApplyFor("bgp"))
		p1.respondApply(gw, orch.TransactionID())
		p1.respondApply(gw, orch.TransactionID())
		p1.respondApply(gw, orch.TransactionID())
		p2.respondApply(gw, orch.TransactionID())

		select {
		case result := <-resultCh:
			if result.State != StateCommitted {
				t.Fatalf("state = %s (err=%v), want %s", result.State, result.Err, StateCommitted)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for commit")
		}
	})
}
