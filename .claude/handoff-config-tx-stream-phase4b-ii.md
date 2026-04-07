# Handoff: spec-config-tx-protocol Phase 4b-ii (orchestrator_test.go rewrite)

Generated 2026-04-07 after Phase 4b-i commit.

## Goal of this phase

Bring the build from RED to GREEN by rewriting `orchestrator_test.go` to use the
new `EventGateway` interface, the new event-type constants, and the new
constructor signature. After this phase commits, `make ze-verify` passes for
the first time since Phase 1 introduced the bus removal.

## RATIONALE (verify this matches what was agreed)

- **Decision: testGateway implements transaction.EventGateway directly** -> EDIT 1
  Reason: the orchestrator depends on the interface, not on Server. The test
  fake satisfies the same interface and the orchestrator cannot tell the
  difference. No need for the engine_event subscriber registry indirection
  in tests; the testGateway maintains its own handler map and dispatches
  synchronously when EmitConfigEvent is called.

- **Decision: testParticipant respond methods call EmitConfigEvent on the gateway** -> EDIT 1
  Reason: in production, plugins emit ack events via the stream system, the
  Server forwards them through engineSubscribers, the orchestrator's
  registered handler receives the payload. In tests, the testGateway
  short-circuits this entire path: respondVerify calls
  gw.EmitConfigEvent(EventVerifyOK, payload), the gateway's dispatch fires
  the orchestrator's registered handler, the handler pushes onto the
  orchestrator's channel. Same effect, simpler fake.

- **Decision: NewTxCoordinator now returns (*TxCoordinator, error)** -> EDIT 1, EDIT 2
  Reason: Phase 4b-i added gateway nil-check and per-participant
  ValidatePluginName check. The tests must check the error. Use a small
  helper inside newTestOrchestrator that calls t.Fatalf on error so each
  individual test stays terse.

- **Decision: rename map applied via Edit tool, not sed** -> EDIT 2
  Reason: precise control, reviewable diff, no risk of false matches in
  comments or strings. The Edit tool can be used with replace_all=true for
  each rename.

- **Decision: TestBudgetUpdatesStored and TestOrchestratorBrokenRecovery
  also need updates beyond the rename map** -> EDIT 2
  Reason: TestBudgetUpdatesStored builds VerifyAck/ApplyAck payloads
  manually and calls bus.Publish on TopicAckVerifyOK directly (lines 720,
  734). TestOrchestratorBrokenRecovery calls NewTxCoordinator directly
  rather than via newTestOrchestrator (line 649). Both need explicit
  attention beyond the rename pattern.

- **Decision: handoff doc included in the commit** -> commit time
  Reason: matches the precedent set by Phase 1, Phase 4a, and Phase 4b-i
  commits. The handoff is the historical record of the work.

If any rationale bullet is wrong, STOP and fix the handoff before applying edits.

## FILES ALREADY HANDLED (do not re-read unless executing)

- `plan/spec-config-tx-protocol.md` -- source of truth, status `in-progress` Phase 1/8 (will become 2/8 after this lands).
- `internal/component/config/transaction/gateway.go` -- Phase 4b-i defined the EventGateway interface. Read it once for the method signatures the testGateway must implement.
- `internal/component/config/transaction/orchestrator.go` -- Phase 4b-i rewrote it to use EventGateway. Constructor is now `NewTxCoordinator(gateway EventGateway, participants []Participant, restartFn RestartFunc) (*TxCoordinator, error)`. Read once for the new signature; do not modify.
- `internal/component/config/transaction/topics.go` -- Phase 1 defined the new event-type constants. Read once for the rename map below.
- `internal/component/config/transaction/types.go` -- defines VerifyEvent, ApplyEvent, RollbackEvent, AbortEvent, CommittedEvent, AppliedEvent, VerifyAck, ApplyAck, RollbackAck, DiffSection. Unchanged. Tests still use these directly.
- `internal/component/plugin/server/engine_event_gateway.go` -- Phase 4b-i added the production adapter. The test fake is independent (a separate type that satisfies the same interface), but you can read this file to confirm the interface shape.
- `internal/component/config/transaction/orchestrator_test.go` -- the file being rewritten. Read once at the start; the handoff describes the transformations precisely.

## EDITS

### EDIT 1: rewrite the test helper section in `internal/component/config/transaction/orchestrator_test.go`

Replace lines 1 through 169 (everything from the package declaration up to and including `newTestOrchestrator`). Use the Edit tool with the OLD/NEW snippets below.

OLD (lines 1-169):
```go
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
```

NEW (lines 1-169):
```go
package transaction

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// testGateway is a minimal EventGateway implementation for orchestrator tests.
// It records emitted events and dispatches synchronously to registered handlers.
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
		_, _ = gw.EmitConfigEvent(EventVerifyFailed, payload)
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		_, _ = gw.EmitConfigEvent(EventVerifyOK, payload)
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
		_, _ = gw.EmitConfigEvent(EventApplyFailed, payload)
	} else {
		ack.Status = CodeOK
		payload, _ := json.Marshal(ack)
		_, _ = gw.EmitConfigEvent(EventApplyOK, payload)
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
	_, _ = gw.EmitConfigEvent(EventRollbackOK, payload)
}

// newTestOrchestrator creates an orchestrator with the given participants.
// Fails the test if NewTxCoordinator returns an error (which only happens
// for nil gateway or reserved participant names; tests should not exercise
// those paths through this helper -- use NewTxCoordinator directly instead).
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
```

### EDIT 2: rename references in test bodies (lines 170-end)

Apply the following renames to the rest of the file (test functions). The Edit tool with `replace_all: true` works for each rename. Apply them one at a time and verify with `go vet` between renames if you want extra safety; alternatively apply them all at once.

#### 2a: variable name renames (use `replace_all: true`)

| OLD | NEW |
|-----|-----|
| `bus := newTestBus()` | `gw := newTestGateway()` |
| `bus *testBus` | `gw *testGateway` |
| `respondVerify(bus,` | `respondVerify(gw,` |
| `respondApply(bus,` | `respondApply(gw,` |
| `respondRollback(bus,` | `respondRollback(gw,` |
| `newTestOrchestrator(bus,` | `newTestOrchestrator(t, gw,` |
| `waitForPublish(t, bus,` | `waitForEmit(t, gw,` |
| `bus.findPublished(` | `gw.findEmitted(` |

The `newTestOrchestrator` rename adds a `t` parameter (the helper now needs `*testing.T` to call `t.Fatalf` on constructor error). Every call site must be updated.

#### 2b: event type renames (use `replace_all: true`)

| OLD | NEW |
|-----|-----|
| `TopicVerifyFor(` | `EventVerifyFor(` |
| `TopicApplyFor(` | `EventApplyFor(` |
| `TopicVerifyAbort` | `EventVerifyAbort` |
| `TopicRollback` | `EventRollback` |
| `TopicCommitted` | `EventCommitted` |
| `TopicApplied` | `EventApplied` |
| `TopicAckVerifyOK` | `EventVerifyOK` |
| `TopicAckVerifyFailed` | `EventVerifyFailed` |
| `TopicAckApplyOK` | `EventApplyOK` |
| `TopicAckApplyFailed` | `EventApplyFailed` |
| `TopicAckRollbackOK` | `EventRollbackOK` |

#### 2c: TopicApplyPrefix special case

There is one reference to `TopicApplyPrefix + "bgp"` (currently around line 261 in `TestOrchestratorVerifyFailed`). The `TopicApplyPrefix` constant no longer exists. Replace:

OLD:
```go
applies := bus.findPublished(TopicApplyPrefix + "bgp")
```

NEW (after the bus -> gw and findPublished -> findEmitted renames in 2a):
```go
applies := gw.findEmitted(EventApplyFor("bgp"))
```

Note: if you apply 2a before 2c, the line will already read `applies := gw.findEmitted(TopicApplyPrefix + "bgp")` and you need to replace `TopicApplyPrefix + "bgp"` with `EventApplyFor("bgp")`. Either way, the result is the same.

#### 2d: TestBudgetUpdatesStored manual ack publishing

`TestBudgetUpdatesStored` manually constructs ack payloads and calls `bus.Publish` directly (around lines 720, 734) instead of using the participant respond helpers. After 2a and 2b, the code reads:

```go
gw.Publish(EventVerifyOK, payload, map[string]string{"plugin": "bgp"})
```

This won't compile because the testGateway doesn't have a `Publish` method (it has `EmitConfigEvent`). Find both lines and rewrite:

OLD (line 720 area):
```go
payload, _ := json.Marshal(ack)
bus.Publish(TopicAckVerifyOK, payload, map[string]string{"plugin": "bgp"})
```

NEW:
```go
payload, _ := json.Marshal(ack)
_, _ = gw.EmitConfigEvent(EventVerifyOK, payload)
```

OLD (line 734 area):
```go
applyPayload, _ := json.Marshal(applyAck)
bus.Publish(TopicAckApplyOK, applyPayload, map[string]string{"plugin": "bgp"})
```

NEW:
```go
applyPayload, _ := json.Marshal(applyAck)
_, _ = gw.EmitConfigEvent(EventApplyOK, applyPayload)
```

#### 2e: TestOrchestratorBrokenRecovery direct constructor call

`TestOrchestratorBrokenRecovery` calls `NewTxCoordinator` directly (around line 649) instead of using `newTestOrchestrator`. The constructor signature changed. Find:

OLD:
```go
orch := NewTxCoordinator(bus, pp, restartFn)
```

NEW (after rename 2a applies bus -> gw):
```go
orch, err := NewTxCoordinator(gw, pp, restartFn)
if err != nil {
	t.Fatalf("NewTxCoordinator: %v", err)
}
```

### EDIT 3: verification

```
go vet ./internal/component/config/transaction/... 2>&1
go test -race ./internal/component/config/transaction/ 2>&1
go vet ./... 2>&1 | grep -v "^$"
make ze-verify-changed 2>&1
```

Expected results:
1. `go vet ./internal/component/config/transaction/...` -> clean (the transaction package is now fully on the stream system).
2. `go test -race ./internal/component/config/transaction/` -> all existing tests pass. Some may need timing adjustments because the testGateway dispatches synchronously (no goroutine hop) -- if a test was relying on the bus's prefix-matching delay, it may now race. Watch for any test that pushes acks immediately after the orchestrator subscribes.
3. `go vet ./...` -> should be clean across the whole tree, assuming no other consumer of `pkg/ze.Bus` exists. There shouldn't be any after Phase 4b-i, but verify.
4. `make ze-verify-changed` -> the first GREEN result since Phase 1.

If `go test` hangs on a particular test, it is most likely the synchronous-dispatch issue: the test calls `respondVerify` before the orchestrator's subscribeAcks has run. The fix is to call `waitForEmit(t, gw, EventVerifyFor("bgp"))` BEFORE calling `respondVerify`, so the orchestrator has emitted the verify event (which means subscribeAcks has been called and the ack handlers are registered).

The existing test code follows this pattern already (every test calls `waitForPublish` before `respondVerify`). After the rename to `waitForEmit`, the pattern continues to work. If a test breaks, it is likely a test that did not previously need the wait.

## After Phase 4b-ii

The build is GREEN. Phase 4c (reverse-tier rollback) and Phase 4d
(dependency-graph deadline) become writeable. Phase 7 (wire reload.go to
TxCoordinator) is still pending and depends on 4b-ii being green.

The `pkg/ze/bus.go` interface still exists but has no remaining callers.
Phase 10 deletes it.

## Reference

- Source of truth: `plan/spec-config-tx-protocol.md`
- Phase 1 commit: `a7d42e3a feat(config-tx): Phase 1 stream event types`
- Phase 4a commit: engine pub/sub API
- Phase 4b-i commit: just landed (assumed), gateway interface and orchestrator rewrite
- Predecessor handoffs:
  - `.claude/handoff-config-tx-stream.md` (Phase 1, completed)
  - `.claude/handoff-config-tx-stream-phase4a.md` (Phase 4a, completed)
  - `.claude/handoff-config-tx-stream-phase4b-i.md` (Phase 4b-i, completed)
- Memory: `.claude/rules/memory.md` arch-0 entry (4 components, stream system as backbone)
