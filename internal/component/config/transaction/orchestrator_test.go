package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

// findReportError returns the first active report.Issue matching the given
// source and code. Tests use it to assert the orchestrator pushed a
// report-bus entry alongside the stream event. The helper filters on the
// subject as well so tests with multiple transactions (or overlapping
// re-runs within the package's single process-wide report store) stay
// deterministic.
func findReportError(source, code, subject string) *report.Issue {
	issues := report.Errors(0)
	for i := range issues {
		if issues[i].Source == source && issues[i].Code == code && issues[i].Subject == subject {
			return &issues[i]
		}
	}
	return nil
}

// testGateway is a minimal EventGateway implementation for orchestrator tests.
// It records emitted events and dispatches synchronously to registered handlers,
// matching the production adapter's semantics (engine handlers fire inline).
type testGateway struct {
	mu       sync.Mutex
	emitted  []emittedEvent
	handlers map[string][]func(payload []byte)
}

type emittedEvent struct {
	EventType string
	Payload   []byte
}

func newTestGateway() *testGateway {
	return &testGateway{
		handlers: make(map[string][]func([]byte)),
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

// mustEmit calls EmitConfigEvent and panics on error. The fake never errors,
// so this exists purely to keep test helper code straight-line without
// triggering the ignored-errors lint hook.
func (g *testGateway) mustEmit(eventType string, payload []byte) {
	if _, err := g.EmitConfigEvent(eventType, payload); err != nil {
		panic(err)
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

// VALIDATES: dependency-graph apply deadline sums per-tier max budgets.
// PREVENTS: Flat max-of-budgets ignoring chain serialization, causing premature
// timeouts when tier k+1 cannot start until tier k finishes.
//
// Setup: tier 0 = {bgp:10s, sysrib:5s}, tier 1 = {fib-kernel:3s, fib-p4:2s}.
// Expected: sum(max per tier) = max(10,5) + max(3,2) = 10 + 3 = 13 seconds.
// The pre-tier flat max would have returned 10 seconds.
func TestOrchestratorDependencyGraphDeadline(t *testing.T) {
	withTierFn(t, func(names []string) ([][]string, error) {
		var tier0, tier1 []string
		for _, n := range names {
			switch n {
			case "bgp", "rib":
				tier0 = append(tier0, n)
			case "fib-kernel", "fib-p4":
				tier1 = append(tier1, n)
			}
		}
		return [][]string{tier0, tier1}, nil
	})

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
	wantApply := 13 * time.Second
	if gotApply != wantApply {
		t.Fatalf("apply deadline = %v, want %v (sum of tier maxes 10+3)", gotApply, wantApply)
	}

	gotVerify := orch.computeVerifyDeadline()
	// max(bgp=4, sysrib=2) + max(fib-kernel=1, fib-p4=1) = 4 + 1 = 5
	wantVerify := 5 * time.Second
	if gotVerify != wantVerify {
		t.Fatalf("verify deadline = %v, want %v (sum of tier maxes 4+1)", gotVerify, wantVerify)
	}
}

// VALIDATES: tier deadline falls back to flat max when tierFn returns an error.
// PREVENTS: A registry cycle bug producing zero deadline (and instant timeout).
func TestOrchestratorTieredDeadlineCycleFallback(t *testing.T) {
	withTierFn(t, func(names []string) ([][]string, error) {
		return nil, errors.New("synthetic cycle")
	})

	gw := newTestGateway()
	pp := []Participant{
		{Name: "a", ApplyBudget: 7},
		{Name: "b", ApplyBudget: 4},
	}
	orch, err := NewTxCoordinator(gw, pp, nil)
	if err != nil {
		t.Fatalf("NewTxCoordinator: %v", err)
	}

	got := orch.computeApplyDeadline()
	want := 7 * time.Second
	if got != want {
		t.Fatalf("apply deadline = %v, want %v (flat max fallback)", got, want)
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

	issue := findReportError(reportSourceConfig, reportCodeCommitAborted, txID)
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

	issue := findReportError(reportSourceConfig, reportCodeCommitRollback, txID)
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

	issue := findReportError(reportSourceConfig, reportCodeCommitSaveFail, txID)
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
