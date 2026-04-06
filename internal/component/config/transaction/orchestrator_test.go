package transaction

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// testBus is a minimal Bus implementation for orchestrator tests.
// It captures published events and delivers them to subscribers.
type testBus struct {
	mu          sync.Mutex
	published   []publishedEvent
	subscribers map[string][]ze.Consumer
}

type publishedEvent struct {
	Topic    string
	Payload  []byte
	Metadata map[string]string
}

func newTestBus() *testBus {
	return &testBus{
		subscribers: make(map[string][]ze.Consumer),
	}
}

func (b *testBus) CreateTopic(name string) (ze.Topic, error) {
	return ze.Topic{Name: name}, nil
}

func (b *testBus) Publish(topic string, payload []byte, metadata map[string]string) {
	b.mu.Lock()
	b.published = append(b.published, publishedEvent{Topic: topic, Payload: payload, Metadata: metadata})
	// Deliver to matching subscribers (prefix match).
	var matching []ze.Consumer
	for prefix, consumers := range b.subscribers {
		if len(topic) >= len(prefix) && topic[:len(prefix)] == prefix {
			matching = append(matching, consumers...)
		}
	}
	b.mu.Unlock()

	for _, c := range matching {
		_ = c.Deliver([]ze.Event{{Topic: topic, Payload: payload, Metadata: metadata}})
	}
}

func (b *testBus) Subscribe(prefix string, _ map[string]string, consumer ze.Consumer) (ze.Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[prefix] = append(b.subscribers[prefix], consumer)
	return ze.Subscription{ID: uint64(len(b.subscribers)), Prefix: prefix}, nil
}

func (b *testBus) Unsubscribe(_ ze.Subscription) {}

func (b *testBus) findPublished(topic string) []publishedEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	var result []publishedEvent
	for _, ev := range b.published {
		if ev.Topic == topic {
			result = append(result, ev)
		}
	}
	return result
}

// waitForPublish polls until at least one event is published to the given topic.
// Replaces time.Sleep for deterministic synchronization.
func waitForPublish(t *testing.T, bus *testBus, topic string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(bus.findPublished(topic)) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for publish to %q", topic)
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

func (tp *testParticipant) respondVerify(bus *testBus, txID string) {
	ack := VerifyAck{
		TransactionID:   txID,
		Plugin:          tp.name,
		ApplyBudgetSecs: tp.applyBudget,
	}
	if tp.verifyErr != "" {
		ack.Status = CodeError
		ack.Error = tp.verifyErr
		payload, _ := json.Marshal(ack)
		bus.Publish(TopicAckVerifyFailed, payload, map[string]string{"plugin": tp.name})
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		bus.Publish(TopicAckVerifyOK, payload, map[string]string{"plugin": tp.name})
	}
}

func (tp *testParticipant) respondApply(bus *testBus, txID string) {
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
		bus.Publish(TopicAckApplyFailed, payload, map[string]string{"plugin": tp.name})
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		bus.Publish(TopicAckApplyOK, payload, map[string]string{"plugin": tp.name})
	}
}

func (tp *testParticipant) respondRollback(bus *testBus, txID string) {
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
	bus.Publish(TopicAckRollbackOK, payload, map[string]string{"plugin": tp.name})
}

// newTestOrchestrator creates an orchestrator with the given participants.
func newTestOrchestrator(bus *testBus, participants []testParticipant) *TxCoordinator {
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
	return NewTxCoordinator(bus, pp, nil)
}

// VALIDATES: AC-1/AC-2 - All verify/ok triggers apply phase.
// PREVENTS: Orchestrator stuck in verify when all plugins accept.
func TestOrchestratorVerifyAllOk(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyBudget: 10}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyBudget: 5}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

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
	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())

	// Wait for apply events, then respond.
	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID())

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

	// Verify committed was published.
	committed := bus.findPublished(TopicCommitted)
	if len(committed) == 0 {
		t.Fatal("config/committed not published")
	}
}

// VALIDATES: AC-3 - Any verify/failed triggers abort.
// PREVENTS: Apply sent when a plugin rejected verify.
func TestOrchestratorVerifyFailed(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, verifyErr: "bad config"}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateAborted {
			t.Fatalf("state = %s, want %s", result.State, StateAborted)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	// Verify abort was published and no apply was sent.
	aborts := bus.findPublished(TopicVerifyAbort)
	if len(aborts) == 0 {
		t.Fatal("config/verify/abort not published")
	}
	applies := bus.findPublished(TopicApplyPrefix + "bgp")
	if len(applies) != 0 {
		t.Fatal("apply published after verify failure")
	}
}

// VALIDATES: AC-15 - Missing verify ack triggers abort after deadline.
// PREVENTS: Orchestrator hanging forever waiting for a dead plugin.
func TestOrchestratorVerifyTimeout(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, verifyBudget: 1}
	orch := newTestOrchestrator(bus, []testParticipant{p1})

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
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, applyBudget: 5}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyBudget: 30}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	// p2 provides a larger budget in verify ack.
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())

	waitForPublish(t, bus, TopicApplyFor("bgp"))
	// Verify apply deadline is from max budget (30s from p2).
	applyDeadline := orch.ApplyDeadline()
	if applyDeadline < 25*time.Second || applyDeadline > 35*time.Second {
		t.Fatalf("apply deadline = %v, want ~30s", applyDeadline)
	}

	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID())

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
	bus := newTestBus()
	writerCalled := false
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(bus, []testParticipant{p1})
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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())

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

	applied := bus.findPublished(TopicApplied)
	if len(applied) == 0 {
		t.Fatal("config/applied not published")
	}
}

// VALIDATES: AC-5 - Apply/failed triggers rollback, collects acks.
// PREVENTS: Rollback skipped when a plugin fails apply.
func TestOrchestratorRollbackOnFailure(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}, applyErr: "disk full"}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())

	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID())

	// Orchestrator publishes rollback -> respond.
	waitForPublish(t, bus, TopicRollback)
	p1.respondRollback(bus, orch.TransactionID())
	p2.respondRollback(bus, orch.TransactionID())

	select {
	case result := <-resultCh:
		if result.State != StateRolledBack {
			t.Fatalf("state = %s, want %s", result.State, StateRolledBack)
		}
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	rollbacks := bus.findPublished(TopicRollback)
	if len(rollbacks) == 0 {
		t.Fatal("config/rollback not published")
	}
}

// VALIDATES: AC-16 - Apply deadline exceeded triggers rollback.
// PREVENTS: Orchestrator hanging when plugin stops responding during apply.
func TestOrchestratorRollbackOnTimeout(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(bus, []testParticipant{p1})
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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())

	// Don't respond to apply -> timeout -> rollback published.
	// Respond to rollback.
	waitForPublish(t, bus, TopicRollback)
	p1.respondRollback(bus, orch.TransactionID())

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
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	orch := newTestOrchestrator(bus, []testParticipant{p1})
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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())

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
	applied := bus.findPublished(TopicApplied)
	if len(applied) == 0 {
		t.Fatal("config/applied not published")
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
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "iface", configRoots: []string{"interface"}}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))

	// Check that bgp got only bgp diffs.
	bgpVerify := bus.findPublished(TopicVerifyFor("bgp"))
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
	ifaceVerify := bus.findPublished(TopicVerifyFor("iface"))
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
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())
	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID())
	<-resultCh
}

// VALIDATES: AC-9 - WantsConfig plugin receives other plugin's diffs.
// PREVENTS: Read-only config interest silently ignored.
func TestWantsConfigDiffDelivery(t *testing.T) {
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "dhcp", wantsConfig: []string{"bgp"}}
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForPublish(t, bus, TopicVerifyFor("dhcp"))

	// dhcp should get bgp diffs via WantsConfig.
	dhcpVerify := bus.findPublished(TopicVerifyFor("dhcp"))
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
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())
	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID())
	<-resultCh
}

// VALIDATES: AC-6 - Broken code triggers plugin restart.
// PREVENTS: Broken plugin left in corrupt state after rollback.
func TestOrchestratorBrokenRecovery(t *testing.T) {
	bus := newTestBus()
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
	orch := NewTxCoordinator(bus, pp, restartFn)

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

	waitForPublish(t, bus, TopicVerifyFor("bgp"))
	p1.respondVerify(bus, orch.TransactionID())
	p2.respondVerify(bus, orch.TransactionID())

	waitForPublish(t, bus, TopicApplyFor("bgp"))
	p1.respondApply(bus, orch.TransactionID())
	p2.respondApply(bus, orch.TransactionID()) // apply fails

	// Rollback published -> respond with broken.
	waitForPublish(t, bus, TopicRollback)
	p1.respondRollback(bus, orch.TransactionID())
	p2.respondRollback(bus, orch.TransactionID()) // code=broken

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
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}, verifyBudget: 5, applyBudget: 10}
	orch := newTestOrchestrator(bus, []testParticipant{p1})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForPublish(t, bus, TopicVerifyFor("bgp"))

	// Send verify ack with updated apply budget.
	ack := VerifyAck{
		TransactionID:   orch.TransactionID(),
		Plugin:          "bgp",
		Status:          CodeOK,
		ApplyBudgetSecs: 20, // Updated from 10 to 20.
	}
	payload, _ := json.Marshal(ack)
	bus.Publish(TopicAckVerifyOK, payload, map[string]string{"plugin": "bgp"})

	waitForPublish(t, bus, TopicApplyFor("bgp"))

	// Send apply ack with updated budgets.
	applyAck := ApplyAck{
		TransactionID:    orch.TransactionID(),
		Plugin:           "bgp",
		Status:           CodeOK,
		VerifyBudgetSecs: 8, // Updated from 5 to 8.
		ApplyBudgetSecs:  25,
	}
	applyPayload, _ := json.Marshal(applyAck)
	bus.Publish(TopicAckApplyOK, applyPayload, map[string]string{"plugin": "bgp"})

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
	bus := newTestBus()
	p1 := testParticipant{name: "bgp", configRoots: []string{"bgp"}}
	p2 := testParticipant{name: "observer"} // No ConfigRoots, no WantsConfig.
	orch := newTestOrchestrator(bus, []testParticipant{p1, p2})

	diffs := map[string][]DiffSection{
		"bgp": {{Root: "bgp", Added: `{"peer":"1.2.3.4"}`}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resultCh := make(chan *TxResult, 1)
	go func() {
		resultCh <- orch.Execute(ctx, diffs)
	}()

	waitForPublish(t, bus, TopicVerifyFor("bgp"))

	// observer should NOT have received a verify event.
	observerVerify := bus.findPublished(TopicVerifyFor("observer"))
	if len(observerVerify) != 0 {
		t.Fatalf("observer got %d verify events, want 0", len(observerVerify))
	}

	// Only bgp needs to respond (observer is excluded from active count).
	p1.respondVerify(bus, orch.TransactionID())
	waitForPublish(t, bus, TopicApplyFor("bgp"))

	observerApply := bus.findPublished(TopicApplyFor("observer"))
	if len(observerApply) != 0 {
		t.Fatalf("observer got %d apply events, want 0", len(observerApply))
	}

	p1.respondApply(bus, orch.TransactionID())
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
