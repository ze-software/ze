package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/report"
)

// findReportError returns the first active report.Issue matching the given
// code. Tests use it to assert the orchestrator pushed a report-bus entry
// alongside the stream event. The helper filters on the subject as well, so a
// test with several transactions stays deterministic. Re-runs that overlap
// within the package's one process-wide report store do too.
//
// The source is always reportSourceConfig: this package raises under no other
// one, so taking it as a parameter only gave every call site the same word to
// repeat.
func findReportError(code, subject string) *report.Issue {
	issues := report.Errors(0)
	for i := range issues {
		if issues[i].Source == reportSourceConfig && issues[i].Code == code && issues[i].Subject == subject {
			return &issues[i]
		}
	}
	return nil
}

// testGateway is a minimal EventGateway implementation for orchestrator tests.
// It records emitted events and dispatches synchronously to registered handlers,
// matching the production adapter's semantics (engine handlers fire inline).
type testGateway struct {
	mu            sync.Mutex
	emitted       []emittedEvent
	handlers      map[string][]func(payload []byte)
	eventHandlers map[testEventKey][]func(payload any)
}

type testEventKey struct {
	Namespace string
	EventType string
}

type emittedEvent struct {
	EventType string
	Payload   []byte
}

func newTestGateway() *testGateway {
	return &testGateway{
		handlers:      make(map[string][]func([]byte)),
		eventHandlers: make(map[testEventKey][]func(any)),
	}
}

// EmitConfigEvent records the emission and dispatches to all registered
// handlers for the event type. Synchronous, like the production adapter.
// The fake never returns an error.
func (g *testGateway) EmitConfigEvent(eventType string, payload []byte) (int, error) {
	g.mu.Lock()
	g.emitted = append(g.emitted, emittedEvent{EventType: eventType, Payload: payload})
	handlers := append([]func([]byte){}, g.handlers[eventType]...)
	g.mu.Unlock()

	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

// SubscribeConfigEvent registers a handler for an event type and returns
// an unsubscribe function. nil handlers return a no-op unsubscribe.
func (g *testGateway) SubscribeConfigEvent(eventType string, handler func(payload []byte)) func() {
	if handler == nil {
		return func() {}
	}
	g.mu.Lock()
	g.handlers[eventType] = append(g.handlers[eventType], handler)
	idx := len(g.handlers[eventType]) - 1
	g.mu.Unlock()

	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		hs := g.handlers[eventType]
		if idx < len(hs) {
			g.handlers[eventType] = append(hs[:idx], hs[idx+1:]...)
		}
	}
}

// SubscribeEvent registers a handler for non-config settlement events.
func (g *testGateway) SubscribeEvent(namespace, eventType string, handler func(payload any)) func() {
	if handler == nil {
		return func() {}
	}
	key := testEventKey{Namespace: namespace, EventType: eventType}
	g.mu.Lock()
	g.eventHandlers[key] = append(g.eventHandlers[key], handler)
	idx := len(g.eventHandlers[key]) - 1
	g.mu.Unlock()

	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		hs := g.eventHandlers[key]
		if idx < len(hs) {
			g.eventHandlers[key] = append(hs[:idx], hs[idx+1:]...)
		}
	}
}

// mustEmit calls EmitConfigEvent and panics on error. The fake never errors,
// so this exists purely to keep test helper code straight-line without
// triggering the ignored-errors lint hook.
func (g *testGateway) mustEmit(eventType string, payload []byte) {
	if _, err := g.EmitConfigEvent(eventType, payload); err != nil {
		panic(err)
	}
}

func (g *testGateway) emitEvent(namespace, eventType string, payload any) {
	g.mu.Lock()
	handlers := append([]func(any){}, g.eventHandlers[testEventKey{Namespace: namespace, EventType: eventType}]...)
	g.mu.Unlock()

	for _, h := range handlers {
		h(payload)
	}
}

func (g *testGateway) findEmitted(eventType string) []emittedEvent {
	g.mu.Lock()
	defer g.mu.Unlock()
	var result []emittedEvent
	for _, ev := range g.emitted {
		if ev.EventType == eventType {
			result = append(result, ev)
		}
	}
	return result
}

// waitForEmit polls until at least one event is emitted with the given
// event type. Replaces time.Sleep for deterministic synchronization.
func waitForEmit(t *testing.T, gw *testGateway, eventType string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(gw.findEmitted(eventType)) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for emit of %q", eventType)
}

// testParticipant simulates a plugin responding to transaction events.
type testParticipant struct {
	name         string
	configRoots  []string
	wantsConfig  []string
	verifyBudget int
	applyBudget  int
	verifyErr    string // non-empty to simulate verify failure
	applyErr     string // non-empty to simulate apply failure
	applyCode    string // override code in apply ack (e.g., CodeBroken)
	rollbackCode string // override code in rollback ack
}

func (tp *testParticipant) respondVerify(gw *testGateway, txID string) {
	ack := VerifyAck{
		TransactionID:   txID,
		Plugin:          tp.name,
		ApplyBudgetSecs: tp.applyBudget,
	}
	if tp.verifyErr != "" {
		ack.Status = CodeError
		ack.Error = tp.verifyErr
		payload, _ := json.Marshal(ack)
		gw.mustEmit(EventVerifyFailed, payload)
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		gw.mustEmit(EventVerifyOK, payload)
	}
}

func (tp *testParticipant) respondApply(gw *testGateway, txID string) {
	ack := ApplyAck{
		TransactionID:    txID,
		Plugin:           tp.name,
		VerifyBudgetSecs: tp.verifyBudget,
		ApplyBudgetSecs:  tp.applyBudget,
	}
	if tp.applyErr != "" {
		ack.Status = CodeError
		if tp.applyCode != "" {
			ack.Status = tp.applyCode
		}
		ack.Error = tp.applyErr
		payload, _ := json.Marshal(ack)
		gw.mustEmit(EventApplyFailed, payload)
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		gw.mustEmit(EventApplyOK, payload)
	}
}

func (tp *testParticipant) respondRollback(gw *testGateway, txID string) {
	code := CodeOK
	if tp.rollbackCode != "" {
		code = tp.rollbackCode
	}
	ack := RollbackAck{
		TransactionID: txID,
		Plugin:        tp.name,
		Code:          code,
	}
	payload, _ := json.Marshal(ack)
	gw.mustEmit(EventRollbackOK, payload)
}

// newTestOrchestrator creates an orchestrator with the given participants.
// Fails the test if NewTxCoordinator returns an error (which only happens
// for nil gateway or reserved participant names; tests that exercise those
// paths must call NewTxCoordinator directly instead).
func newTestOrchestrator(t *testing.T, gw *testGateway, participants []testParticipant) *TxCoordinator {
	t.Helper()
	pp := make([]Participant, len(participants))
	for i := range participants {
		pp[i] = Participant{
			Name:         participants[i].name,
			ConfigRoots:  participants[i].configRoots,
			WantsConfig:  participants[i].wantsConfig,
			VerifyBudget: participants[i].verifyBudget,
			ApplyBudget:  participants[i].applyBudget,
		}
	}
	orch, err := NewTxCoordinator(gw, pp, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}
	return orch
}

// VALIDATES: AC-1/AC-2 - All verify/ok triggers apply phase.
// PREVENTS: Orchestrator stuck in verify when all plugins accept.
func TestOrchestratorVerifyAllOk(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyBudget: 10}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyBudget: 5}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
		"interface": {{Root: "interface", Changed: `{"eth0":"up"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run transaction in background.
	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	// Wait for verify events, then respond.
	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	// Wait for apply events, then respond.
	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if result.State != StateCommitted {
			t.Fatalf("state = %s, want %s", result.State, StateCommitted)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	// Verify committed was emitted.
	committed := gw.findEmitted(EventCommitted)
	if len(committed) == 0 {
		t.Fatal("config/committed not emitted")
	}
}

// VALIDATES: AC-3 - Any verify/failed triggers abort.
// PREVENTS: Apply sent when a plugin rejected verify.
func TestOrchestratorVerifyFailed(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, verifyErr: "bad config"}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateAborted {
			t.Fatalf("state = %s, want %s", result.State, StateAborted)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	// Verify abort was emitted and no apply was sent.
	aborts := gw.findEmitted(EventVerifyAbort)
	if len(aborts) == 0 {
		t.Fatal("config/verify-abort not emitted")
	}
	applies := gw.findEmitted(EventApplyFor("bgp"))
	if len(applies) != 0 {
		t.Fatal("apply emitted after verify failure")
	}
}

// VALIDATES: AC-15 - Missing verify ack triggers abort after deadline.
// PREVENTS: Orchestrator hanging forever waiting for a dead plugin.
func TestOrchestratorVerifyTimeout(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, verifyBudget: 1}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})

	// Override verify deadline to be short for test.
	orch.SetVerifyDeadline(200 * time.Millisecond)

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := orch.Execute(ctx, diffs)
	// Plugin never responds -> timeout -> abort.
	if result.State != StateAborted {
		t.Fatalf("state = %s, want %s", result.State, StateAborted)
	}
}

// VALIDATES: AC-2/AC-10 - Apply deadline computed from max budget.
// PREVENTS: Wrong deadline calculation.
func TestOrchestratorVerifyToApply(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyBudget: 5}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyBudget: 30}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	// p2 provides a larger budget in verify ack.
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("bgp"))
	// Verify apply deadline is from max budget (30s from p2).
	applyDeadline := orch.ApplyDeadline()
	if applyDeadline < 25*time.Second || applyDeadline > 35*time.Second {
		t.Fatalf("apply deadline = %v, want ~30s", applyDeadline)
	}

	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}
}

// VALIDATES: AC-4 - All apply/ok triggers committed + applied.
// PREVENTS: Config file write or notification skipped.
func TestOrchestratorApplyAllOk(t *testing.T) {
	gw := newTestGateway()
	writerCalled := false
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetConfigWriter(func() error {
		writerCalled = true
		return nil
	})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if !result.Saved {
			t.Fatal("Saved = false, want true")
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	if !writerCalled {
		t.Fatal("config writer not called")
	}

	applied := gw.findEmitted(EventApplied)
	if len(applied) == 0 {
		t.Fatal("config/applied not emitted")
	}
}

// VALIDATES: Operation-sensitive transactions use operation verify/apply/commit instead of legacy apply.
// PREVENTS: Operation planning being built but never wired into TxCoordinator.Execute.
func TestOrchestratorUsesOperationPathWhenPlannerReturnsOperations(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetOperationPlanner(func(_ context.Context, req OperationPlanRequest) ([]ConfigOperation, error) {
		if req.TransactionID != orch.TransactionID() {
			t.Fatalf("planner tx = %q, want %q", req.TransactionID, orch.TransactionID())
		}
		return []ConfigOperation{{ID: "addr-add", Root: "interface", Owner: "iface", Type: testOpAddAddress, Verb: VerbCreate, Target: ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1/32"}}}, nil
	})

	var operationEvents []string
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		operationEvents = append(operationEvents, EventOperationVerifyFor("iface"))
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal operation verify: %v", err)
		}
		ack := ConfigOperationVerifyAck{TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, _ := json.Marshal(ack)
		gw.mustEmit(EventOperationVerifyOK, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		operationEvents = append(operationEvents, EventOperationApplyFor("iface"))
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal operation apply: %v", err)
		}
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK}
		ackPayload, _ := json.Marshal(ack)
		gw.mustEmit(EventOperationApplyOK, ackPayload)
	})
	gw.SubscribeConfigEvent(EventOperationCommitFor("iface"), func(payload []byte) {
		operationEvents = append(operationEvents, EventOperationCommitFor("iface"))
		var ev ConfigOperationCommitEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			t.Fatalf("unmarshal operation commit: %v", err)
		}
		ack := ConfigOperationCommitAck{TransactionID: ev.TransactionID, Plugin: "iface", Status: CodeOK}
		ackPayload, _ := json.Marshal(ack)
		gw.mustEmit(EventOperationCommitOK, ackPayload)
	})

	diffs := map[string][]DiffSection{"interface": {{Root: "interface", Added: `{"interface/eth0/address/192.0.2.1/32":true}`}}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("iface"))
	p1.respondVerify(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if result.State != StateCommitted {
			t.Fatalf("state = %s, want %s", result.State, StateCommitted)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	if applies := gw.findEmitted(EventApplyFor("iface")); len(applies) != 0 {
		t.Fatalf("legacy apply emitted in operation path: %d", len(applies))
	}
	if got, want := operationEvents, []string{EventOperationVerifyFor("iface"), EventOperationApplyFor("iface"), EventOperationCommitFor("iface")}; !slices.Equal(got, want) {
		t.Fatalf("operation events = %v, want %v", got, want)
	}
}

// VALIDATES: AC-5 - Apply/failed triggers rollback, collects acks.
// PREVENTS: Rollback skipped when a plugin fails apply.
func TestOrchestratorRollbackOnFailure(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyErr: "disk full"}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID())

	// Orchestrator emits rollback -> respond.
	waitForEmit(t, gw, EventRollback)
	p1.respondRollback(gw, orch.TransactionID())
	p2.respondRollback(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	rollbacks := gw.findEmitted(EventRollback)
	if len(rollbacks) == 0 {
		t.Fatal("config/rollback not emitted")
	}
}

// VALIDATES: AC-16 - Apply deadline exceeded triggers rollback.
// PREVENTS: Orchestrator hanging when plugin stops responding during apply.
func TestOrchestratorRollbackOnTimeout(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetApplyDeadlineOverride(200 * time.Millisecond)

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())

	// Don't respond to apply -> timeout -> rollback emitted.
	// Respond to rollback.
	waitForEmit(t, gw, EventRollback)
	p1.respondRollback(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}
}

// VALIDATES: AC-7 - Write failure produces applied with saved=false.
// PREVENTS: Rollback triggered by disk write failure.
func TestOrchestratorFileWriteFailure(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	orch.SetConfigWriter(func() error {
		return errConfigWriteFailed
	})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateCommitted {
			t.Fatalf("state = %s, want %s", result.State, StateCommitted)
		}
		if result.Saved {
			t.Fatal("Saved = true, want false (write failed)")
		}
		if result.Err != nil {
			t.Fatalf("Err should be nil (write failure is warning, not error), got: %v", result.Err)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	// Applied event should have saved=false.
	applied := gw.findEmitted(EventApplied)
	if len(applied) == 0 {
		t.Fatal("config/applied not emitted")
	}
	var ev AppliedEvent
	if err := json.Unmarshal(applied[0].Payload, &ev); err != nil {
		t.Fatalf("unmarshal applied: %v", err)
	}
	if ev.Saved {
		t.Fatal("AppliedEvent.Saved = true, want false")
	}
}

// VALIDATES: AC-1 - Plugin receives only declared roots.
// PREVENTS: Plugin receiving diffs for roots it did not declare.
func TestPerPluginDiffFiltering(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
		"interface": {{Root: "interface", Changed: `{"eth0":"up"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))

	// Check that bgp got only bgp diffs.
	bgpVerify := gw.findEmitted(EventVerifyFor("bgp"))
	if len(bgpVerify) != 1 {
		t.Fatalf("bgp verify events = %d, want 1", len(bgpVerify))
	}
	var bgpEv VerifyEvent
	if err := json.Unmarshal(bgpVerify[0].Payload, &bgpEv); err != nil {
		t.Fatalf("unmarshal bgp verify: %v", err)
	}
	if len(bgpEv.Diffs) != 1 || bgpEv.Diffs[0].Root != "bgp" {
		t.Fatalf("bgp got wrong diffs: %+v", bgpEv.Diffs)
	}

	// Check that iface got only interface diffs.
	ifaceVerify := gw.findEmitted(EventVerifyFor("iface"))
	if len(ifaceVerify) != 1 {
		t.Fatalf("iface verify events = %d, want 1", len(ifaceVerify))
	}
	var ifaceEv VerifyEvent
	if err := json.Unmarshal(ifaceVerify[0].Payload, &ifaceEv); err != nil {
		t.Fatalf("unmarshal iface verify: %v", err)
	}
	if len(ifaceEv.Diffs) != 1 || ifaceEv.Diffs[0].Root != "interface" {
		t.Fatalf("iface got wrong diffs: %+v", ifaceEv.Diffs)
	}

	// Complete to avoid leak.
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID())
	<-resultCh
}

// VALIDATES: AC-9 - WantsConfig plugin receives other plugin's diffs.
// PREVENTS: Read-only config interest silently ignored.
func TestWantsConfigDiffDelivery(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "dhcp", wantsConfig: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("dhcp"))

	// dhcp should get bgp diffs via WantsConfig.
	dhcpVerify := gw.findEmitted(EventVerifyFor("dhcp"))
	if len(dhcpVerify) != 1 {
		t.Fatalf("dhcp verify events = %d, want 1", len(dhcpVerify))
	}
	var dhcpEv VerifyEvent
	if err := json.Unmarshal(dhcpVerify[0].Payload, &dhcpEv); err != nil {
		t.Fatalf("unmarshal dhcp verify: %v", err)
	}
	if len(dhcpEv.Diffs) != 1 || dhcpEv.Diffs[0].Root != "bgp" {
		t.Fatalf("dhcp got wrong diffs: %+v", dhcpEv.Diffs)
	}

	// Complete.
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID())
	<-resultCh
}

// VALIDATES: AC-6 - Broken code triggers plugin restart.
// PREVENTS: Broken plugin left in corrupt state after rollback.
func TestOrchestratorBrokenRecovery(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyErr: "crash", rollbackCode: CodeBroken}

	var restarted []string
	restartFn := func(name string) error {
		restarted = append(restarted, name)
		return nil
	}

	pp := make([]Participant, 2)
	pp[0] = Participant{Name: "bgp", ConfigRoots: []string{"bgp"}}
	pp[1] = Participant{Name: "iface", ConfigRoots: []string{"interface"}}
	orch, err := NewTxCoordinator(gw, pp, restartFn)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, orch.TransactionID())
	p2.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, orch.TransactionID())
	p2.respondApply(gw, orch.TransactionID()) // apply fails

	// Rollback emitted -> respond with broken.
	waitForEmit(t, gw, EventRollback)
	p1.respondRollback(gw, orch.TransactionID())
	p2.respondRollback(gw, orch.TransactionID()) // code=broken

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	// restartFn should have been called for the broken plugin.
	if len(restarted) != 1 || restarted[0] != "iface" {
		t.Fatalf("restarted = %v, want [iface]", restarted)
	}
}

// VALIDATES: AC-11 - Updated budgets from apply/ok used for next tx.
// PREVENTS: Stale budgets used after plugin self-corrects.
func TestBudgetUpdatesStored(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, verifyBudget: 5, applyBudget: 10}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))

	// Send verify ack with updated apply budget.
	ack := VerifyAck{
		TransactionID:   orch.TransactionID(),
		Plugin:          "bgp",
		Status:          CodeOK,
		ApplyBudgetSecs: 20, // Updated from 10 to 20.
	}
	payload, _ := json.Marshal(ack)
	gw.mustEmit(EventVerifyOK, payload)

	waitForEmit(t, gw, EventApplyFor("bgp"))

	// Send apply ack with updated budgets.
	applyAck := ApplyAck{
		TransactionID:    orch.TransactionID(),
		Plugin:           "bgp",
		Status:           CodeOK,
		VerifyBudgetSecs: 8, // Updated from 5 to 8.
		ApplyBudgetSecs:  25,
	}
	applyPayload, _ := json.Marshal(applyAck)
	gw.mustEmit(EventApplyOK, applyPayload)

	<-resultCh

	// Check budgets were updated.
	budgets := orch.ParticipantBudgets("bgp")
	if budgets.VerifyBudget != 8 {
		t.Fatalf("verify budget = %d, want 8", budgets.VerifyBudget)
	}
	if budgets.ApplyBudget != 25 {
		t.Fatalf("apply budget = %d, want 25", budgets.ApplyBudget)
	}
}

// VALIDATES: AC-14 - Plugin with neither ConfigRoots nor WantsConfig does not receive events.
// PREVENTS: Uninvolved plugins receiving transaction events.
func TestNoConfigPluginExcluded(t *testing.T) {
	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "observer"} // No ConfigRoots, no WantsConfig.
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))

	// observer should NOT have received a verify event.
	observerVerify := gw.findEmitted(EventVerifyFor("observer"))
	if len(observerVerify) != 0 {
		t.Fatalf("observer got %d verify events, want 0", len(observerVerify))
	}

	// Only bgp needs to respond (observer is excluded from active count).
	p1.respondVerify(gw, orch.TransactionID())
	waitForEmit(t, gw, EventApplyFor("bgp"))

	observerApply := gw.findEmitted(EventApplyFor("observer"))
	if len(observerApply) != 0 {
		t.Fatalf("observer got %d apply events, want 0", len(observerApply))
	}

	p1.respondApply(gw, orch.TransactionID())
	<-resultCh
}

// VALIDATES: Boundary - Budget at MaxBudgetSeconds (600) is valid, 601+ is capped.
// PREVENTS: Unbounded deadlines from malicious or buggy plugin budgets.
func TestBudgetBoundary(t *testing.T) {
	// capBudget at boundary values.
	if got := capBudget(600); got != 600 {
		t.Fatalf("capBudget(600) = %d, want 600", got)
	}
	if got := capBudget(601); got != MaxBudgetSeconds {
		t.Fatalf("capBudget(601) = %d, want %d", got, MaxBudgetSeconds)
	}
	if got := capBudget(999999); got != MaxBudgetSeconds {
		t.Fatalf("capBudget(999999) = %d, want %d", got, MaxBudgetSeconds)
	}
	if got := capBudget(0); got != 0 {
		t.Fatalf("capBudget(0) = %d, want 0", got)
	}
	if got := capBudget(-1); got != -1 {
		t.Fatalf("capBudget(-1) = %d, want -1 (no lower bound needed)", got)
	}
}

// withTierFn temporarily overrides tierFn for the duration of the test and
// restores the previous value via t.Cleanup. Tests that need specific tier
// shapes use this to inject a fake registry.TopologicalTiers without touching
// the global plugin registry.
func withTierFn(t *testing.T, fn func(names []string) ([][]string, error)) {
	t.Helper()
	prev := tierFn
	tierFn = fn
	t.Cleanup(func() { tierFn = prev })
}

// VALIDATES: rollback ack collection drains in reverse dependency-tier order.
// PREVENTS: A dependency rolling back while a dependent is still mid-rollback.
//
// Setup: three participants leaf, middle, root with dependencies
// leaf -> middle -> root, expressed via a fake tierFn that returns
// [[root], [middle], [leaf]] (lowest tier first). Rollback order must be
// leaf, middle, root (reverse of dependency order). The test detects order
// via restartFn: every plugin reports CodeBroken so restartFn is called once
// per plugin, and the call order is captured.
func TestOrchestratorRollbackReverseTier(t *testing.T) {
	withTierFn(t, func(names []string) ([][]string, error) {
		// Build the tier structure leaf depends on middle depends on root.
		// Tier 0 (no deps) = root, tier 1 = middle, tier 2 = leaf.
		tiers := make([][]string, 3)
		for _, n := range names {
			switch n {
			case "root":
				tiers[0] = append(tiers[0], n)
			case "middle":
				tiers[1] = append(tiers[1], n)
			case "leaf":
				tiers[2] = append(tiers[2], n)
			}
		}
		return tiers, nil
	})

	gw := newTestGateway()

	var restartedMu sync.Mutex
	var restarted []string
	restartFn := func(name string) error {
		restartedMu.Lock()
		defer restartedMu.Unlock()
		restarted = append(restarted, name)
		return nil
	}

	pp := []Participant{
		{Name: "root", ConfigRoots: []string{"root-cfg"}},
		{Name: "middle", ConfigRoots: []string{"middle-cfg"}},
		{Name: "leaf", ConfigRoots: []string{"leaf-cfg"}},
	}
	orch, err := NewTxCoordinator(gw, pp, restartFn)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	diffs := map[string][]DiffSection{
		"root-cfg":   {{Root: "root-cfg"}},
		"middle-cfg": {{Root: "middle-cfg"}},
		"leaf-cfg":   {{Root: "leaf-cfg", Added: "trigger-fail"}}, // triggers apply failure
	}

	// Make leaf fail apply so the orchestrator enters rollback.
	leaf := testParticipant{name: "leaf", configRoots: []string{"leaf-cfg"}, applyErr: "boom"}
	root := testParticipant{name: "root", configRoots: []string{"root-cfg"}}
	middle := testParticipant{name: "middle", configRoots: []string{"middle-cfg"}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("root"))
	root.respondVerify(gw, orch.TransactionID())
	middle.respondVerify(gw, orch.TransactionID())
	leaf.respondVerify(gw, orch.TransactionID())

	waitForEmit(t, gw, EventApplyFor("root"))
	root.respondApply(gw, orch.TransactionID())
	middle.respondApply(gw, orch.TransactionID())
	leaf.respondApply(gw, orch.TransactionID()) // failure

	waitForEmit(t, gw, EventRollback)
	// Send rollback acks in tier-0 order to prove the orchestrator buffers
	// them and processes only the deepest tier first. Every ack reports
	// CodeBroken so restartFn captures the processing order.
	root.rollbackCode = CodeBroken
	middle.rollbackCode = CodeBroken
	leaf.rollbackCode = CodeBroken
	root.respondRollback(gw, orch.TransactionID())
	middle.respondRollback(gw, orch.TransactionID())
	leaf.respondRollback(gw, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	restartedMu.Lock()
	got := append([]string(nil), restarted...)
	restartedMu.Unlock()

	want := []string{"leaf", "middle", "root"}
	if len(got) != len(want) {
		t.Fatalf("restart count = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("restart order = %v, want %v", got, want)
		}
	}
}

// TestApplyDeadlineSumsEveryParticipantBudget fences the size of the budget
// against the way the phase actually runs. Every participant applies in turn,
// one after another, so the phase can cost the SUM of what they declared and
// the deadline has to cover it.
//
// The tier shape below is the one the deleted per-tier max read: bgp and rib
// in tier 0, fib-kernel and fib-p4 in tier 1. It no longer decides anything,
// and the expectation says so: 10+5+3+2 rather than max(10,5)+max(3,2).
//
// It replaces TestOrchestratorDependencyGraphDeadline, which asserted 13s for
// the same participants. That answer was correct for a tier applied
// concurrently, and nothing applies concurrently (test/weakened/4c26aef3.md).
//
// VALIDATES: I-1 -- the apply and verify deadlines cover every participant in sequence.
// PREVENTS: a reload that four plugins each take their declared budget to
// apply timing out on the fourth and rolling back work that had committed.
func TestApplyDeadlineSumsEveryParticipantBudget(t *testing.T) {
	gw := newTestGateway()
	pp := []Participant{
		{Name: "bgp", ApplyBudget: 10, VerifyBudget: 4},
		{Name: "rib", ApplyBudget: 5, VerifyBudget: 2},
		{Name: "fib-kernel", ApplyBudget: 3, VerifyBudget: 1},
		{Name: "fib-p4", ApplyBudget: 2, VerifyBudget: 1},
	}
	orch, err := NewTxCoordinator(gw, pp, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	gotApply := orch.computeApplyDeadline()
	wantApply := 20 * time.Second
	if gotApply != wantApply {
		t.Fatalf("apply deadline = %v, want %v (10+5+3+2)", gotApply, wantApply)
	}

	gotVerify := orch.computeVerifyDeadline()
	wantVerify := 8 * time.Second
	if gotVerify != wantVerify {
		t.Fatalf("verify deadline = %v, want %v (4+2+1+1)", gotVerify, wantVerify)
	}
}

// TestApplyDeadlineDefaultsWhenNoParticipantDeclaresABudget keeps the half of
// TestOrchestratorTieredDeadlineCycleFallback that survives the deletion of
// the tier computation: a deadline of zero is an instant timeout, so a
// transaction whose participants declared nothing takes the 30-second default.
//
// The other half went with its mechanism. That test made tierFn return an
// error to reach the flat-max fallback branch, and the deadline no longer
// calls tierFn at all (test/weakened/4c26aef3.md).
//
// VALIDATES: a participant set declaring no budget still gets a usable deadline.
// PREVENTS: a zero deadline timing out the apply before the first plugin answers.
func TestApplyDeadlineDefaultsWhenNoParticipantDeclaresABudget(t *testing.T) {
	gw := newTestGateway()
	pp := []Participant{
		{Name: "a"},
		{Name: "b"},
	}
	orch, err := NewTxCoordinator(gw, pp, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	if got, want := orch.computeApplyDeadline(), 30*time.Second; got != want {
		t.Fatalf("apply deadline = %v, want %v (the no-budget default)", got, want)
	}
	if got, want := orch.computeVerifyDeadline(), 30*time.Second; got != want {
		t.Fatalf("verify deadline = %v, want %v (the no-budget default)", got, want)
	}
}

// TestCommitAbortRaisesReportError verifies that a verify-phase failure
// pushes a commit-aborted entry onto the operational report bus alongside
// the stream abort event.
//
// VALIDATES: AC-21 -- config/commit-aborted raised when verify fails.
// PREVENTS: Operators losing visibility of verify failures via ze show
// errors; the stream abort event alone is engine-internal.
func TestCommitAbortRaisesReportError(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, verifyErr: "bad config"}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})
	txID := orch.TransactionID()

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, txID)
	p2.respondVerify(gw, txID)

	result := <-resultCh
	if result.State != StateAborted {
		t.Fatalf("state = %s, want %s", result.State, StateAborted)
	}

	issue := findReportError(reportCodeCommitAborted, txID)
	if issue == nil {
		t.Fatalf("report bus missing commit-aborted entry for tx %s; have %d errors", txID, len(report.Errors(0)))
	}
	if issue.Severity != report.SeverityError {
		t.Errorf("severity = %s, want error", issue.Severity)
	}
	if issue.Detail["phase"] != "verify" {
		t.Errorf("detail.phase = %v, want %q", issue.Detail["phase"], "verify")
	}
	if issue.Detail["reason"] == nil {
		t.Error("detail.reason missing; should carry the verify failure reason")
	}
}

// TestCommitRollbackRaisesReportError verifies that an apply-phase failure
// pushes a commit-rollback entry onto the report bus when the orchestrator
// publishes its rollback event.
//
// VALIDATES: AC-22 -- config/commit-rollback raised when apply fails
// mid-transaction.
// PREVENTS: Silent rollback -- operators need to see commit-rollback via
// ze show errors to understand why runtime state reverted.
func TestCommitRollbackRaisesReportError(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyErr: "iface broken"}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1, p2})
	txID := orch.TransactionID()

	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp"}},
		"interface": {{Root: "interface"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, txID)
	p2.respondVerify(gw, txID)

	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, txID)
	p2.respondApply(gw, txID)

	// Orchestrator publishes rollback on apply failure and waits for
	// rollback acks from every participant. Respond so Execute returns.
	waitForEmit(t, gw, EventRollback)
	p1.respondRollback(gw, txID)
	p2.respondRollback(gw, txID)

	result := <-resultCh
	if result.State != StateRolledBack {
		t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
	}

	issue := findReportError(reportCodeCommitRollback, txID)
	if issue == nil {
		t.Fatalf("report bus missing commit-rollback entry for tx %s; have %d errors", txID, len(report.Errors(0)))
	}
	if issue.Severity != report.SeverityError {
		t.Errorf("severity = %s, want error", issue.Severity)
	}
	if issue.Detail["phase"] != "apply" {
		t.Errorf("detail.phase = %v, want %q", issue.Detail["phase"], "apply")
	}
	if issue.Detail["reason"] == nil {
		t.Error("detail.reason missing; should carry the apply failure reason")
	}
}

// TestCommitSaveFailedRaisesReportError verifies that a ConfigWriter
// failure after a successful apply pushes a commit-save-failed entry onto
// the report bus. The transaction still reports StateCommitted because
// runtime state is live; only the persisted config file is out of sync.
//
// VALIDATES: AC-23 -- config/commit-save-failed raised when runtime
// applied successfully but the config file write failed.
// PREVENTS: Silent divergence between the running reactor and the
// persisted config file on disk; without the report entry, operators
// have no signal that ze show config is out of sync with the live state.
func TestCommitSaveFailedRaisesReportError(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	gw := newTestGateway()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(t, gw, []testParticipant{p1})
	txID := orch.TransactionID()

	orch.SetConfigWriter(func() error {
		return errConfigWriteFailed
	})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForEmit(t, gw, EventVerifyFor("bgp"))
	p1.respondVerify(gw, txID)

	waitForEmit(t, gw, EventApplyFor("bgp"))
	p1.respondApply(gw, txID)

	result := <-resultCh
	if result.State != StateCommitted {
		t.Fatalf("state = %s, want %s (runtime should be live even on save failure)", result.State, StateCommitted)
	}
	if result.Saved {
		t.Error("Saved = true, want false (writer returned an error)")
	}

	issue := findReportError(reportCodeCommitSaveFail, txID)
	if issue == nil {
		t.Fatalf("report bus missing commit-save-failed entry for tx %s; have %d errors", txID, len(report.Errors(0)))
	}
	if issue.Severity != report.SeverityError {
		t.Errorf("severity = %s, want error", issue.Severity)
	}
	if issue.Detail["phase"] != "save" {
		t.Errorf("detail.phase = %v, want %q", issue.Detail["phase"], "save")
	}
	if issue.Detail["error"] == nil {
		t.Error("detail.error missing; should carry the writer's error")
	}
}

// mixedOperationCoverage builds a two-participant transaction where both roots
// changed but only opOwners yield operations. Returns the orchestrator, the
// participants and the diffs so each test can drive the phases it cares about.
func mixedOperationCoverage(t *testing.T, gw *testGateway, opOwners ...string) (*TxCoordinator, []testParticipant, map[string][]DiffSection) {
	t.Helper()
	participants := []testParticipant{
		{name: "iface", configRoots: []string{"interface"}},
		{name: "bgp", configRoots: []string{"bgp"}},
	}
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		var ops []ConfigOperation
		for _, owner := range opOwners {
			root := "bgp"
			op := ConfigOperation{
				ID: "op-" + owner, Root: root, Owner: owner, Type: testOpAddPeer, Verb: VerbCreate,
				Target: ResourceRef{Kind: ResourcePeer, Peer: "peer1"},
			}
			if owner == "iface" {
				op.Root = "interface"
				op.Type = testOpAddAddress
				op.Target = ResourceRef{Kind: ResourceAddress, Interface: "eth0", Address: "192.0.2.1/32"}
			}
			ops = append(ops, op)
		}
		return ops, nil
	})
	diffs := map[string][]DiffSection{
		"bgp":       {{Root: "bgp", Added: `{"bgp/peer/peer1":{}}`}},
		"interface": {{Root: "interface", Changed: `{"interface/wireguard/wg0/private-key":{"old":"a","new":"b"}}`}},
	}
	return orch, participants, diffs
}

// autoAckOperations wires handlers that ack every per-operation event for owner,
// so a test can assert on which path ran without hand-driving each phase.
func autoAckOperations(gw *testGateway, owner string, seen *[]string) {
	gw.SubscribeConfigEvent(EventOperationVerifyFor(owner), func(payload []byte) {
		*seen = append(*seen, EventOperationVerifyFor(owner))
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, Plugin: owner, OperationID: ev.Operation.ID, Status: CodeOK})
		gw.mustEmit(EventOperationVerifyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationApplyFor(owner), func(payload []byte) {
		*seen = append(*seen, EventOperationApplyFor(owner))
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: owner, OperationID: ev.Operation.ID, Status: CodeOK})
		gw.mustEmit(EventOperationApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventOperationCommitFor(owner), func(payload []byte) {
		*seen = append(*seen, EventOperationCommitFor(owner))
		var ev ConfigOperationCommitEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationCommitAck{TransactionID: ev.TransactionID, Plugin: owner, Status: CodeOK})
		gw.mustEmit(EventOperationCommitOK, ack)
	})
}

// autoAckSectionApply acks the section apply event for one participant, so a
// test can assert which path each participant took without hand-driving the
// phases. Records every section apply the participant received.
func autoAckSectionApply(gw *testGateway, name string, seen *[]string) {
	gw.SubscribeConfigEvent(EventApplyFor(name), func(payload []byte) {
		*seen = append(*seen, EventApplyFor(name))
		var ev ApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: name, Status: CodeOK})
		gw.mustEmit(EventApplyOK, ack)
	})
}

// TestExecuteMixedRootTakesOperationPath is the ordering fence, and it carries
// the data-loss fence with it. A reload that touches BOTH a section that
// decomposes (bgp: a peer was added) and one that does not (interface: a
// wireguard property changed -- a root with no decomposer at all here) runs through
// the operation path, and BOTH are applied: the decomposed root through its
// per-operation callbacks, the other through one coarse node routed to the
// section apply.
//
// Before this, one participant the planner could not cover sent the whole
// transaction down the unordered section apply, so the operations that WERE
// ordered lost their order. Applying the operation path as it stood instead
// would have left that participant verified and never applied.
//
// VALIDATES: AC-1 -- a mixed transaction keeps the operation path, and the uncovered participant is applied by a coarse node.
// PREVENTS: one undecomposable participant costing the whole reload its cross-participant ordering.
func TestExecuteMixedRootTakesOperationPath(t *testing.T) {
	gw := newTestGateway()
	orch, participants, diffs := mixedOperationCoverage(t, gw, "bgp")
	var operationEvents []string
	var sectionApplies []string
	autoAckOperations(gw, "bgp", &operationEvents)
	autoAckSectionApply(gw, "iface", &sectionApplies)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}

	select {
	case result := <-resultCh:
		if result.State != StateCommitted {
			t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateCommitted)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to commit")
	}

	if !slices.Contains(operationEvents, EventOperationApplyFor("bgp")) {
		t.Fatalf("the decomposed root did not take the operation path: %v", operationEvents)
	}
	if applies := gw.findEmitted(EventApplyFor("bgp")); len(applies) != 0 {
		t.Fatalf("the decomposed root received %d section applies, want 0", len(applies))
	}
	if applies := gw.findEmitted(EventApplyFor("iface")); len(applies) != 1 {
		t.Fatalf("the uncovered participant received %d section applies, want 1", len(applies))
	}
}

// TestExecuteCoarseNodeAppliesSection reads the coarse node's payload. The
// node exists to apply exactly what the section apply applied for the same
// participant, so the diffs it carries are the diffs filterDiffs produces --
// the predicate runVerify and runApply already use.
//
// VALIDATES: AC-2, A-1 -- a coarse node emits the participant's section apply with that participant's diffs.
// PREVENTS: a root whose change is applied under a different payload than the section apply sent it.
func TestExecuteCoarseNodeAppliesSection(t *testing.T) {
	gw := newTestGateway()
	orch, participants, diffs := mixedOperationCoverage(t, gw, "bgp")
	var operationEvents []string
	var sectionApplies []string
	autoAckOperations(gw, "bgp", &operationEvents)
	autoAckSectionApply(gw, "iface", &sectionApplies)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}
	if result := <-resultCh; result.State != StateCommitted {
		t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateCommitted)
	}

	applies := gw.findEmitted(EventApplyFor("iface"))
	if len(applies) != 1 {
		t.Fatalf("the coarse node emitted %d section applies, want 1", len(applies))
	}
	var event ApplyEvent
	if err := json.Unmarshal(applies[0].Payload, &event); err != nil {
		t.Fatalf("unmarshal the coarse node's apply event: %v", err)
	}
	if event.TransactionID != orch.TransactionID() {
		t.Errorf("apply event tx = %q, want %q", event.TransactionID, orch.TransactionID())
	}
	want := diffs["interface"]
	if !reflect.DeepEqual(event.Diffs, want) {
		t.Errorf("the coarse node carried %v, want the participant's diffs %v", event.Diffs, want)
	}
}

// TestExecuteCoarseNodeEmitsNoOperationVerify holds the second half of the
// coarse node's contract. Phase 1 verified the whole candidate config for that
// participant before the planner ran, and the plugin implements no
// config-operation-verify callback to ask again with, so the node owes no
// per-operation verify and no per-operation commit.
//
// VALIDATES: A-2 -- a coarse node reaches its participant through the section events only.
// PREVENTS: a coarse node emitting an operation callback its participant answers "unknown method" to.
func TestExecuteCoarseNodeEmitsNoOperationVerify(t *testing.T) {
	gw := newTestGateway()
	orch, participants, diffs := mixedOperationCoverage(t, gw, "bgp")
	var operationEvents []string
	var sectionApplies []string
	autoAckOperations(gw, "bgp", &operationEvents)
	autoAckSectionApply(gw, "iface", &sectionApplies)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}
	if result := <-resultCh; result.State != StateCommitted {
		t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateCommitted)
	}

	if verifies := gw.findEmitted(EventVerifyFor("iface")); len(verifies) != 1 {
		t.Fatalf("the coarse node's participant received %d full-config verifies, want 1", len(verifies))
	}
	for _, eventType := range []string{
		EventOperationVerifyFor("iface"),
		EventOperationApplyFor("iface"),
		EventOperationCommitFor("iface"),
	} {
		if emitted := gw.findEmitted(eventType); len(emitted) != 0 {
			t.Errorf("the coarse node emitted %s %d times, want 0", eventType, len(emitted))
		}
	}
}

// TestParticipantsWithoutOperationsSortsTheCoarseNodes verifies that two
// participants nobody decomposes get their coarse nodes in one order, whatever
// order the transaction was handed them in.
//
// The order this function is GIVEN comes from a walk of the process manager's
// map, so a commit that applied section A then section B applied them the
// other way round on the next run, and the test that fences the placement
// failed 2 runs in 20. Nothing in the graph orders one uncovered participant
// against another, so the choice is arbitrary and the stability is not.
//
// VALIDATES: the coarse section order is the same on every run of one commit.
// PREVENTS: a reload whose applied order depends on a map walk, which no test
// can fence and no operator can reproduce.
func TestParticipantsWithoutOperationsSortsTheCoarseNodes(t *testing.T) {
	gw := newTestGateway()
	orch := newTestOrchestrator(t, gw, []testParticipant{
		{name: "zebra", configRoots: []string{"shared"}},
		{name: "alpha", configRoots: []string{"shared"}},
	})
	diffs := map[string][]DiffSection{
		"shared": {{Root: "shared", Changed: `{"shared/value":{"old":"a","new":"b"}}`}},
	}

	if uncovered := orch.participantsWithoutOperations(nil, diffs); !slices.Equal(uncovered, []string{"alpha", "zebra"}) {
		t.Fatalf("uncovered participants = %v, want them sorted", uncovered)
	}

	nodes := orch.operationNodes(nil, diffs)
	owners := make([]string, 0, len(nodes))
	for i := range nodes {
		owners = append(owners, nodes[i].Owner)
	}
	if !slices.Equal(owners, []string{"alpha", "zebra"}) {
		t.Fatalf("coarse node owners = %v, want them sorted", owners)
	}
}

// TestParticipantsWithoutOperationsEmptyAfterSynthesis states the property the
// deleted fallback tested for: after synthesis no participant with diffs is
// left without a node. The fallback branch is unreachable because coverage is
// total, and it is gone.
//
// VALIDATES: every participant with diffs owns a node in the graph the executor runs.
// PREVENTS: a participant reaching apply through no path at all.
func TestParticipantsWithoutOperationsEmptyAfterSynthesis(t *testing.T) {
	gw := newTestGateway()
	orch, _, diffs := mixedOperationCoverage(t, gw, "bgp")

	ops, err := orch.operationPlanner(context.Background(), OperationPlanRequest{TransactionID: orch.TransactionID(), Diffs: diffs})
	if err != nil {
		t.Fatalf("plan operations: %v", err)
	}
	if uncovered := orch.participantsWithoutOperations(ops, diffs); len(uncovered) != 1 {
		t.Fatalf("the planner covered %v, want the iface participant uncovered before synthesis", uncovered)
	}

	nodes := orch.operationNodes(ops, diffs)
	if uncovered := orch.participantsWithoutOperations(nodes, diffs); len(uncovered) != 0 {
		t.Fatalf("participants without a node after synthesis: %v", uncovered)
	}
	if len(nodes) != len(ops)+1 {
		t.Fatalf("synthesis produced %d nodes from %d operations, want one coarse node added", len(nodes), len(ops))
	}
	coarse := nodes[len(nodes)-1]
	if !IsSectionApply(&coarse) {
		t.Fatalf("the synthesized node is %q, want the section-apply type", coarse.Type)
	}
	if coarse.Owner != "iface" {
		t.Errorf("the coarse node's owner is %q, want the uncovered participant", coarse.Owner)
	}
	if coarse.ID == "" {
		t.Error("the coarse node has no id, so the graph cannot hold it")
	}
}

// TestOrchestratorUsesOperationPathWhenEveryParticipantYieldsOperations is the
// other half of the fence: the fallback must not swallow the ordering path
// whenever operations DO cover every participant with diffs, which is the case
// the operation graph exists for.
//
// VALIDATES: operations covering every participant with diffs still run through the operation path.
// PREVENTS: the coverage guard degrading every transaction to the unordered section apply.
func TestOrchestratorUsesOperationPathWhenEveryParticipantYieldsOperations(t *testing.T) {
	gw := newTestGateway()
	orch, participants, diffs := mixedOperationCoverage(t, gw, "bgp", "iface")
	var operationEvents []string
	autoAckOperations(gw, "bgp", &operationEvents)
	autoAckOperations(gw, "iface", &operationEvents)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}

	select {
	case result := <-resultCh:
		if result.State != StateCommitted {
			t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateCommitted)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to commit")
	}

	for _, name := range []string{"iface", "bgp"} {
		if applies := gw.findEmitted(EventApplyFor(name)); len(applies) != 0 {
			t.Fatalf("participant %s received a section apply inside the operation path", name)
		}
		if !slices.Contains(operationEvents, EventOperationApplyFor(name)) {
			t.Fatalf("participant %s received no operation apply: %v", name, operationEvents)
		}
	}
}

// TestExecuteRefusesOperationWithNoVerb drives the guard from the entry point
// the whole operation path runs behind. A planner returns one operation that
// declares no verb, and the transaction aborts: the graph orders by the verb
// and the target kind, so an operation with no verb has nothing to be ordered
// by, and reading the empty value as a modification would give a create the
// dependencies of a change in place.
//
// The abort message names the plugin, the config root and the operation id,
// and nothing else. Params carry config values, keys among them.
//
// VALIDATES: AC-6, and `ai/rules/principles.md`: no operation is ordered on a
// default.
// PREVENTS: a plugin payload written against the old vocabulary being applied
// in an arbitrary position while the transaction reports success.
func TestExecuteRefusesOperationWithNoVerb(t *testing.T) {
	gw := newTestGateway()
	participants := []testParticipant{{name: "provision", configRoots: []string{"provision"}}}
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:     "provision-claim-1",
			Root:   "provision",
			Owner:  "provision",
			Type:   ConfigOperationType("provision-claim-vip"),
			Target: ResourceRef{Kind: ResourceAddress, Address: "192.0.2.1/32"},
			Params: ConfigOperationParams{Value: "s3cret-community-string"},
		}}, nil
	})
	diffs := map[string][]DiffSection{"provision": {{Root: "provision", Added: `{"provision/vip":{}}`}}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	waitForEmit(t, gw, EventVerifyFor("provision"))
	participants[0].respondVerify(gw, orch.TransactionID())

	var result *TxResult
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to abort")
	}

	if result.State != StateAborted {
		t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateAborted)
	}
	if !errors.Is(result.Err, ErrOperationNoVerb) {
		t.Fatalf("err = %v, want %v", result.Err, ErrOperationNoVerb)
	}
	message := result.Err.Error()
	for _, want := range []string{"provision", "provision-claim-1"} {
		if !strings.Contains(message, want) {
			t.Errorf("the abort message does not name %q: %s", want, message)
		}
	}
	if strings.Contains(message, "s3cret-community-string") {
		t.Errorf("the abort message carries an operation parameter: %s", message)
	}
	if applies := gw.findEmitted(EventApplyFor("provision")); len(applies) != 0 {
		t.Errorf("the refused transaction applied %d sections, want 0", len(applies))
	}
	if applies := gw.findEmitted(EventOperationApplyFor("provision")); len(applies) != 0 {
		t.Errorf("the refused transaction applied %d operations, want 0", len(applies))
	}
}

// TestExecuteAppliesCoarseNodeAfterTheResourcesItBinds drives the coarse
// node's position from the door an operator reaches. The reload adds an
// interface, adds an address on it, and changes one root nothing decomposes.
// The uncovered root binds the address, so its section apply must arrive after
// the address exists.
//
// The order is read from the events themselves, which is what the plugins
// receive and act on, rather than from the sorted slice.
//
// VALIDATES: AC-1 through Execute, for a root with no decomposer.
// PREVENTS: the static route reaching the kernel before its address, which
// answers "network is unreachable" while the transaction reports committed.
func TestExecuteAppliesCoarseNodeAfterTheResourcesItBinds(t *testing.T) {
	gw := newTestGateway()
	participants := []testParticipant{
		{name: "iface", configRoots: []string{"interface"}},
		{name: "static", configRoots: []string{"static"}},
	}
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{
			{ID: "iface-add-zx", Root: "interface", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate,
				Target:   ResourceRef{Kind: ResourceInterface, Name: "zx"},
				Produces: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}}},
			{ID: "iface-add-address-zx", Root: "interface", Owner: "iface", Type: testOpAddAddress, Verb: VerbCreate,
				Target:   ResourceRef{Kind: ResourceAddress, Interface: "zx", Address: "10.93.0.1/24"},
				Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}},
				Consumes: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}}},
		}, nil
	})
	diffs := map[string][]DiffSection{
		"interface": {{Root: "interface", Added: `{"interface/dummy/zx":{}}`}},
		"static":    {{Root: "static", Added: `{"static/route/172.30.0.0-24":{}}`}},
	}

	var applied []string
	var operationEvents []string
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		applied = append(applied, ev.Operation.ID)
	})
	autoAckOperations(gw, "iface", &operationEvents)
	gw.SubscribeConfigEvent(EventApplyFor("static"), func(payload []byte) {
		var ev ApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		applied = append(applied, "section-apply-static")
	})
	var sectionApplies []string
	autoAckSectionApply(gw, "static", &sectionApplies)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}

	select {
	case result := <-resultCh:
		if result.State != StateCommitted {
			t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateCommitted)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to commit")
	}

	want := []string{"iface-add-zx", "iface-add-address-zx", "section-apply-static"}
	if !reflect.DeepEqual(applied, want) {
		t.Fatalf("applied in the order %v, want %v", applied, want)
	}
}

// TestExecuteRollsBackAnAppliedCoarseNode is AC-7's missing half. The
// transaction below applies the coarse node and THEN fails, so the node's
// participant has real work to undo when the rollback goes out.
//
// The reach is what is asserted, because the two node kinds are undone by two
// different mechanisms. A decomposed operation replays its inverse through
// `config-operation-rollback` (rollbackApplied, executor.go), which the coarse
// node is skipped by: its participant implements no such callback. What undoes
// the coarse node is the broadcast rollback the orchestrator publishes when
// the ordered apply fails, which the bridge turns into one `config-rollback`
// per participant.
//
// The order the events arrive in is the assertion: the static participant sees
// its section apply, and then the rollback.
//
// VALIDATES: AC-7, I-3 -- an APPLIED coarse node's participant receives the section rollback.
// PREVENTS: a failed reload leaving an uncovered root applied while every
// decomposed operation is undone around it.
func TestExecuteRollsBackAnAppliedCoarseNode(t *testing.T) {
	gw := newTestGateway()
	participants := []testParticipant{
		{name: "iface", configRoots: []string{"interface"}},
		{name: "static", configRoots: []string{"static"}},
	}
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{
			{ID: "iface-add-zx", Root: "interface", Owner: "iface", Type: testOpAddInterface, Verb: VerbCreate,
				Target:   ResourceRef{Kind: ResourceInterface, Name: "zx"},
				Produces: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}}},
			{ID: "iface-remove-address-zy", Root: "interface", Owner: "iface", Type: testOpRemoveAddress, Verb: VerbDestroy,
				Target:   ResourceRef{Kind: ResourceAddress, Interface: "zy", Address: "10.93.0.1/24"},
				Produces: []ResourceRef{{Kind: ResourceAddress, Address: "10.93.0.1/24"}}},
			{ID: "iface-start-session", Root: "interface", Owner: "iface", Type: testOpAddPeer, Verb: VerbCreate,
				Target:   ResourceRef{Kind: ResourcePeer, Peer: "edge"},
				Produces: []ResourceRef{{Kind: ResourcePeer, Peer: "edge"}}},
		}, nil
	})
	diffs := map[string][]DiffSection{
		"interface": {{Root: "interface", Added: `{"interface/dummy/zx":{}}`}},
		"static":    {{Root: "static", Added: `{"static/route/172.30.0.0-24":{}}`}},
	}

	var seen []string
	var rolledBack []string
	gw.SubscribeConfigEvent(EventOperationVerifyFor("iface"), func(payload []byte) {
		var ev ConfigOperationVerifyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		ack, _ := json.Marshal(ConfigOperationVerifyAck{TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK})
		gw.mustEmit(EventOperationVerifyOK, ack)
	})
	// The binder start is the operation that fails, and it sorts after the
	// coarse node: placeSectionNodes puts a coarse node after the addressing
	// this commit adds and before the first operation that binds it
	// (solver.go).
	gw.SubscribeConfigEvent(EventOperationApplyFor("iface"), func(payload []byte) {
		var ev ConfigOperationApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		seen = append(seen, ev.Operation.ID)
		ack := ConfigOperationApplyAck{TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operation.ID, Status: CodeOK}
		event := EventOperationApplyOK
		if ev.Operation.Target.Kind == ResourcePeer {
			ack.Status = CodeError
			ack.Error = "the session refused to start"
			event = EventOperationApplyFailed
		}
		payloadAck, _ := json.Marshal(ack)
		gw.mustEmit(event, payloadAck)
	})
	gw.SubscribeConfigEvent(EventOperationRollbackFor("iface"), func(payload []byte) {
		var ev ConfigOperationRollbackEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		for i := range ev.Operations {
			rolledBack = append(rolledBack, ev.Operations[i].ID)
			ack, _ := json.Marshal(ConfigOperationRollbackAck{TransactionID: ev.TransactionID, Plugin: "iface", OperationID: ev.Operations[i].ID, Status: CodeOK})
			gw.mustEmit(EventOperationRollbackOK, ack)
		}
	})
	gw.SubscribeConfigEvent(EventApplyFor("static"), func(payload []byte) {
		var ev ApplyEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return
		}
		seen = append(seen, "static-section-apply")
		ack, _ := json.Marshal(ApplyAck{TransactionID: ev.TransactionID, Plugin: "static", Status: CodeOK})
		gw.mustEmit(EventApplyOK, ack)
	})
	gw.SubscribeConfigEvent(EventRollback, func(payload []byte) {
		seen = append(seen, "static-section-rollback")
		for i := range participants {
			participants[i].respondRollback(gw, orch.TransactionID())
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	for i := range participants {
		waitForEmit(t, gw, EventVerifyFor(participants[i].name))
		participants[i].respondVerify(gw, orch.TransactionID())
	}

	var result *TxResult
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to roll back")
	}

	if result.State != StateRolledBack {
		t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateRolledBack)
	}
	want := []string{"iface-add-zx", "iface-remove-address-zy", "static-section-apply", "iface-start-session", "static-section-rollback"}
	if !reflect.DeepEqual(seen, want) {
		t.Fatalf("the transaction ran %v, want %v", seen, want)
	}
	if !reflect.DeepEqual(rolledBack, []string{"iface-remove-address-zy", "iface-add-zx"}) {
		t.Errorf("per-operation rollback replayed %v, want the two applied operations in reverse", rolledBack)
	}
}

// TestExecuteRefusesAParticipantCoveredForOneRootAndNotAnother drives the
// coverage guard from the door. The participant below decomposes the
// `interface` root and has a diff on `firewall` it decomposes nothing for.
//
// Coarse-node synthesis asks whether a participant owns ANY operation, so this
// participant reads as covered, gets no coarse node, and its firewall sections
// reach no phase of the transaction. The transaction would then report
// committed over a change nothing applied, which is the silently wrong answer
// `ai/rules/principles.md` bans.
//
// No first-party participant can reach it today: the two that decompose
// declare one root each (iface/register.go, bgp/plugin/register.go). The guard
// is what keeps the second root from being an unreported loss the day a
// participant declares one.
//
// VALIDATES: I-2 -- a partially decomposed participant aborts the transaction.
// PREVENTS: a root's config change being discarded while the reload reports success.
func TestExecuteRefusesAParticipantCoveredForOneRootAndNotAnother(t *testing.T) {
	gw := newTestGateway()
	participants := []testParticipant{{name: "iface", configRoots: []string{"interface", "firewall"}}}
	orch := newTestOrchestrator(t, gw, participants)
	orch.SetOperationPlanner(func(_ context.Context, _ OperationPlanRequest) ([]ConfigOperation, error) {
		return []ConfigOperation{{
			ID:       "iface-add-zx",
			Root:     "interface",
			Owner:    "iface",
			Type:     testOpAddInterface,
			Verb:     VerbCreate,
			Target:   ResourceRef{Kind: ResourceInterface, Name: "zx"},
			Produces: []ResourceRef{{Kind: ResourceInterface, Name: "zx"}},
		}}, nil
	})
	diffs := map[string][]DiffSection{
		"interface": {{Root: "interface", Added: `{"interface/dummy/zx":{}}`}},
		"firewall":  {{Root: "firewall", Added: `{"firewall/rule/drop-ssh":{}}`}},
	}
	// The operation acks make the unguarded run reach its end, so the failure
	// this test fences is the transaction COMMITTING with the firewall root
	// applied by nothing, rather than a hang on an unanswered event.
	var operationEvents []string
	autoAckOperations(gw, "iface", &operationEvents)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resultCh := make(chan *TxResult, 1)
	go func() { resultCh <- orch.Execute(ctx, diffs) }()

	waitForEmit(t, gw, EventVerifyFor("iface"))
	participants[0].respondVerify(gw, orch.TransactionID())

	var result *TxResult
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for the transaction to abort")
	}

	if result.State != StateAborted {
		t.Fatalf("state = %s (err %v), want %s", result.State, result.Err, StateAborted)
	}
	if !errors.Is(result.Err, ErrParticipantRootUncovered) {
		t.Fatalf("err = %v, want %v", result.Err, ErrParticipantRootUncovered)
	}
	message := result.Err.Error()
	for _, want := range []string{"iface", "firewall"} {
		if !strings.Contains(message, want) {
			t.Errorf("the abort message does not name %q: %s", want, message)
		}
	}
	if applies := gw.findEmitted(EventApplyFor("iface")); len(applies) != 0 {
		t.Errorf("the refused transaction applied %d sections, want 0", len(applies))
	}
	if applies := gw.findEmitted(EventOperationApplyFor("iface")); len(applies) != 0 {
		t.Errorf("the refused transaction applied %d operations, want 0", len(applies))
	}
}
