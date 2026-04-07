# Handoff: spec-config-tx-protocol Phase 4b-i (gateway interface + orchestrator rewrite)

Generated 2026-04-07 after Phase 4a commit.

## Why this handoff is "Phase 4b-i", not "Phase 4b"

Phase 4b in the spec is "rewrite the TxCoordinator orchestrator pub/sub layer."
That requires rewriting BOTH `orchestrator.go` (~546 lines) AND
`orchestrator_test.go` (~808 lines). Both reference the bus interface and the
old `Topic*` constants that Phase 1 removed; both need to change together.

Splitting again:

| Sub-phase | What | Build state after |
|-----------|------|-------------------|
| **4b-i (this handoff)** | Define `EventGateway` interface in transaction package, add `ConfigEventGateway` adapter in plugin/server package, rewrite `orchestrator.go` to use the gateway. Tests are NOT touched. | Still red. The transaction package now has a working orchestrator.go but `orchestrator_test.go` references gone types and old constants. |
| 4b-ii (next handoff) | Rewrite `orchestrator_test.go` to use a `testGateway` instead of `testBus`, update all `TopicX` references to event-type references, update test helpers. | **Green.** Build compiles, tests pass. This is the moment the bus debt from Phase 1 is fully repaid. |

After 4b-ii: Phase 4c (reverse-tier rollback) and 4d (dependency-graph deadline) can land.

## RATIONALE (verify this matches what was agreed)

- **Decision: orchestrator depends on a tiny `EventGateway` interface defined in the transaction package** -> EDIT 1
  Reason: keeps the transaction package free of imports from `plugin/server`.
  Server satisfies the interface implicitly (Go duck typing) via a wrapper.
  The interface is the orchestrator's view of the stream system. It is small
  on purpose: emit one type of event, subscribe to one type of event,
  unsubscribe. No bus-style topic prefixes, no metadata maps.

- **Decision: `ConfigEventGateway` adapter in plugin/server hides the namespace parameter** -> EDIT 2
  Reason: the orchestrator only ever publishes/subscribes in the `config`
  namespace. Hardcoding it inside the adapter means the orchestrator code
  reads `gateway.EmitConfigEvent(eventType, payload)` instead of
  `gateway.EmitEngineEvent("config", eventType, payload)` -- one less moving
  part to get wrong, no risk of typos in the namespace string.

- **Decision: gateway methods take/return `[]byte`, not `string`** -> EDITs 1, 2
  Reason: payloads are JSON. `json.Marshal` returns `[]byte`. `json.Unmarshal`
  takes `[]byte`. Converting to string at the orchestrator boundary just to
  convert back inside the adapter is wasted work. The adapter does the
  string conversion when calling Server.EmitEngineEvent (which still takes
  string for compatibility with the existing deliverEvent path).

- **Decision: `Subscribe` returns an unsubscribe function, NOT a handle** -> EDIT 1
  Reason: matches Phase 4a's `Server.SubscribeEngineEvent` shape. Cleaner
  than tracking opaque IDs. The orchestrator stores the funcs in a slice
  and calls them from `unsubscribeAcks`.

- **Decision: nil-handler safety preserved through the gateway** -> EDIT 1, EDIT 2
  Reason: Phase 4a's `register` returns 0 for a nil handler, and
  `unregister(0)` is a no-op. The gateway's `SubscribeConfigEvent` must
  preserve this: if a caller passes nil, the returned unsubscribe should
  be a no-op so `defer unsub()` is safe. The adapter implementation in
  EDIT 2 handles this.

- **Decision: orchestrator.go is rewritten via targeted Edits, not a Write** -> EDIT 3
  Reason: the changes are concentrated in ~10 methods. The rest of the file
  (state constants, Participant struct, TxResult, Execute, filter helpers,
  budget computation, etc.) is unchanged. Targeted edits are smaller and
  safer than a full file rewrite, and they make the diff legible at review
  time.

- **Decision: orchestrator_test.go is NOT touched in this handoff** -> "After 4b-i"
  Reason: rewriting both files in one handoff exceeds the 5-edit budget and
  risks getting one wrong. Splitting means an intermediate red state where
  the test file references gone identifiers, but that is acceptable because
  no commit happens between 4b-i and 4b-ii (or 4b-i is committed alone with
  a build still failing on test compilation -- the user decides per-commit
  whether to land 4b-i alone or wait for 4b-ii).

- **Open: how to commit between 4b-i and 4b-ii?**
  Three options. The user decides at commit time:
  1. Commit 4b-i alone. Build still red on test compilation. Low cost since
     it has been red since Phase 1.
  2. Wait for 4b-ii. Commit both together as a single "Phase 4b" commit.
     Cleaner history, larger commit.
  3. Commit 4b-i with 4b-ii implemented but uncommitted. Two commits, build
     stays red between them only briefly.
  Recommendation: option 2. Less commit churn, the build goes from red to
  green in one step.

If any rationale bullet is wrong, STOP and fix the handoff before applying edits.

## FILES ALREADY HANDLED (do not re-read unless executing)

- `plan/spec-config-tx-protocol.md` -- source of truth, status `in-progress` Phase 1/8 (will become 2/8 or 4b-i/8 after this lands).
- `internal/component/plugin/events.go` -- Phase 1 added `NamespaceConfig` and 12 config event types. Already committed.
- `internal/component/config/transaction/topics.go` -- Phase 1 rewrote this with the new event-type constants (EventVerify, EventApply, EventRollback, EventVerifyAbort, EventCommitted, EventApplied, EventRolledBack, EventVerifyOK, EventVerifyFailed, EventApplyOK, EventApplyFailed, EventRollbackOK, plus EventVerifyFor/EventApplyFor helpers and ValidatePluginName). Already committed.
- `internal/component/plugin/server/engine_event.go` -- Phase 4a added `Server.EmitEngineEvent(namespace, eventType, event string) (int, error)` and `Server.SubscribeEngineEvent(namespace, eventType string, handler EngineEventHandler) func()`. Already committed.
- `internal/component/plugin/server/dispatch.go` -- Phase 4a added `defer s.dispatchEngineEvent(...)` to deliverEvent. Already committed.
- `internal/component/plugin/server/server.go` -- Phase 4a added `engineSubscribers` field. Already committed.
- `internal/component/config/transaction/types.go` -- defines VerifyEvent, ApplyEvent, RollbackEvent, AbortEvent, CommittedEvent, AppliedEvent, VerifyAck, ApplyAck, RollbackAck, DiffSection. NOT touched in this handoff. Read for reference if needed.
- `internal/component/config/transaction/orchestrator.go` -- the file being rewritten in EDIT 3. Read it once at the start to understand the current structure; EDIT 3 provides targeted Edit instructions.

## EDITS

### EDIT 1: new file `internal/component/config/transaction/gateway.go`

```go
// Design: docs/architecture/config/transaction-protocol.md -- orchestrator's view of the stream system
// Related: orchestrator.go -- the consumer of this interface
// Related: topics.go -- event type constants the orchestrator passes to gateway methods

package transaction

// EventGateway is the orchestrator's view of the stream event system.
//
// The orchestrator publishes config namespace events (verify-<plugin>,
// apply-<plugin>, rollback, committed, applied, rolled-back, verify-abort)
// and subscribes to plugin-emitted ack events (verify-ok, verify-failed,
// apply-ok, apply-failed, rollback-ok). All events are in the config
// namespace; the gateway hides the namespace parameter from the
// orchestrator.
//
// The Server in internal/component/plugin/server provides a
// ConfigEventGateway adapter that satisfies this interface by delegating
// to Server.EmitEngineEvent / Server.SubscribeEngineEvent in the config
// namespace.
type EventGateway interface {
	// EmitConfigEvent publishes a stream event in the config namespace.
	// Returns the number of plugin processes that received it. Engine
	// subscribers (other orchestrators, observers) also receive the event
	// but are not counted in the return value.
	//
	// eventType MUST be a registered event type in the config namespace
	// (see plugin.IsValidEvent). Per-plugin event types
	// (verify-<plugin>, apply-<plugin>) must be registered before they
	// can be emitted.
	EmitConfigEvent(eventType string, payload []byte) (int, error)

	// SubscribeConfigEvent registers a handler for a config namespace
	// event type. The handler fires synchronously when a matching event
	// is published; it must not block on external I/O (push to a buffered
	// channel and return).
	//
	// Returns an unsubscribe function. Calling it removes the handler.
	// Safe to call multiple times -- subsequent calls are no-ops.
	// If handler is nil, the returned function is a no-op (no
	// registration is performed).
	SubscribeConfigEvent(eventType string, handler func(payload []byte)) func()
}
```

### EDIT 2: new file `internal/component/plugin/server/engine_event_gateway.go`

```go
// Design: docs/architecture/config/transaction-protocol.md -- engine-side stream pub/sub adapter
// Related: engine_event.go -- the underlying Server methods
// Related: ../../config/transaction/gateway.go -- the EventGateway interface this adapter satisfies

package server

import (
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// ConfigEventGateway adapts Server to the
// internal/component/config/transaction.EventGateway interface used by the
// config transaction orchestrator.
//
// The adapter hides the namespace parameter (always plugin.NamespaceConfig)
// and converts between the orchestrator's []byte payloads and Server's
// string event payloads.
type ConfigEventGateway struct {
	server *Server
}

// NewConfigEventGateway creates a new adapter wrapping the given Server.
// The Server must outlive the gateway; the gateway holds a reference but
// does not manage Server lifecycle.
func NewConfigEventGateway(s *Server) *ConfigEventGateway {
	return &ConfigEventGateway{server: s}
}

// EmitConfigEvent publishes a stream event in the config namespace.
// Returns the number of plugin processes that received the event.
func (g *ConfigEventGateway) EmitConfigEvent(eventType string, payload []byte) (int, error) {
	return g.server.EmitEngineEvent(plugin.NamespaceConfig, eventType, string(payload))
}

// SubscribeConfigEvent registers a handler for a config namespace event type.
// The handler is invoked synchronously from deliverEvent. Returns an
// unsubscribe function; nil handler returns a no-op unsubscribe.
func (g *ConfigEventGateway) SubscribeConfigEvent(eventType string, handler func(payload []byte)) func() {
	if handler == nil {
		return func() {}
	}
	return g.server.SubscribeEngineEvent(plugin.NamespaceConfig, eventType, func(event string) {
		handler([]byte(event))
	})
}
```

### EDIT 3: rewrite `internal/component/config/transaction/orchestrator.go` -- targeted changes

The orchestrator's logic is unchanged. Only the pub/sub layer changes. Apply
the following edits in order using the Edit tool. Read the current file once
at the start to verify the OLD snippets match.

#### EDIT 3.1: imports

OLD:
```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)
```

NEW:
```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)
```

#### EDIT 3.2: TxCoordinator struct fields

Find the `TxCoordinator struct` definition (around line 61). Two fields change:

OLD:
```go
type TxCoordinator struct {
	bus          ze.Bus
	participants []Participant
```

NEW:
```go
type TxCoordinator struct {
	gateway      EventGateway
	participants []Participant
```

And further down in the same struct, the subscription handle field:

OLD:
```go
	// Stored subscription handles for cleanup.
	subs []ze.Subscription
```

NEW:
```go
	// Stored unsubscribe functions for cleanup.
	unsubs []func()
```

#### EDIT 3.3: NewTxCoordinator constructor signature and body

OLD:
```go
// NewTxCoordinator creates a transaction coordinator.
// restartFn may be nil if broken-plugin recovery is not needed.
func NewTxCoordinator(bus ze.Bus, participants []Participant, restartFn RestartFunc) *TxCoordinator {
	return &TxCoordinator{
		bus:            bus,
		participants:   participants,
```

NEW:
```go
// NewTxCoordinator creates a transaction coordinator.
// gateway is the orchestrator's view of the stream event system; the Server
// in internal/component/plugin/server provides a ConfigEventGateway adapter
// that satisfies it. restartFn may be nil if broken-plugin recovery is not
// needed.
func NewTxCoordinator(gateway EventGateway, participants []Participant, restartFn RestartFunc) *TxCoordinator {
	return &TxCoordinator{
		gateway:        gateway,
		participants:   participants,
```

#### EDIT 3.4: subscribeAcks function -- complete rewrite

Find the `subscribeAcks` function (around line 175). It currently subscribes
to 5 bus topics and stores the subscriptions in `o.subs`. Replace the
entire function body with stream-system subscriptions.

OLD:
```go
// subscribeAcks registers bus consumers for all ack topics.
func (o *TxCoordinator) subscribeAcks() {
	subscribe := func(topic string, handler func([]ze.Event) error) {
		sub, err := o.bus.Subscribe(topic, nil, consumerFunc(handler))
		if err != nil {
			logger().Error("subscribe failed", "topic", topic, "error", err)
			return
		}
		o.subs = append(o.subs, sub)
	}

	subscribe(TopicAckVerifyOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack VerifyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.verifyOKCh <- ack
		}
		return nil
	})

	subscribe(TopicAckVerifyFailed, func(events []ze.Event) error {
		for _, ev := range events {
			var ack VerifyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.verifyFailedCh <- ack
		}
		return nil
	})

	subscribe(TopicAckApplyOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack ApplyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.applyOKCh <- ack
		}
		return nil
	})

	subscribe(TopicAckApplyFailed, func(events []ze.Event) error {
		for _, ev := range events {
			var ack ApplyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.applyFailedCh <- ack
		}
		return nil
	})

	subscribe(TopicAckRollbackOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack RollbackAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.rollbackOKCh <- ack
		}
		return nil
	})
}
```

NEW:
```go
// subscribeAcks registers engine handlers for all config ack event types.
// Handlers parse the payload, filter by transaction ID, and push the ack
// onto the appropriate channel for the orchestrator's main loop.
func (o *TxCoordinator) subscribeAcks() {
	subscribeVerifyAck := func(eventType string, ch chan<- VerifyAck) {
		unsub := o.gateway.SubscribeConfigEvent(eventType, func(payload []byte) {
			var ack VerifyAck
			if err := json.Unmarshal(payload, &ack); err != nil {
				return
			}
			if ack.TransactionID != o.txID {
				return
			}
			ch <- ack
		})
		o.unsubs = append(o.unsubs, unsub)
	}

	subscribeApplyAck := func(eventType string, ch chan<- ApplyAck) {
		unsub := o.gateway.SubscribeConfigEvent(eventType, func(payload []byte) {
			var ack ApplyAck
			if err := json.Unmarshal(payload, &ack); err != nil {
				return
			}
			if ack.TransactionID != o.txID {
				return
			}
			ch <- ack
		})
		o.unsubs = append(o.unsubs, unsub)
	}

	subscribeVerifyAck(EventVerifyOK, o.verifyOKCh)
	subscribeVerifyAck(EventVerifyFailed, o.verifyFailedCh)
	subscribeApplyAck(EventApplyOK, o.applyOKCh)
	subscribeApplyAck(EventApplyFailed, o.applyFailedCh)

	o.unsubs = append(o.unsubs, o.gateway.SubscribeConfigEvent(EventRollbackOK, func(payload []byte) {
		var ack RollbackAck
		if err := json.Unmarshal(payload, &ack); err != nil {
			return
		}
		if ack.TransactionID != o.txID {
			return
		}
		o.rollbackOKCh <- ack
	}))
}
```

#### EDIT 3.5: runVerify publish call

In `runVerify` (around line 266), find the inner publish call:

OLD:
```go
		o.bus.Publish(TopicVerifyFor(p.Name), payload, map[string]string{"plugin": p.Name})
```

NEW:
```go
		_, _ = o.gateway.EmitConfigEvent(EventVerifyFor(p.Name), payload)
```

#### EDIT 3.6: runApply publish call

In `runApply` (around line 320), find the inner publish call:

OLD:
```go
		o.bus.Publish(TopicApplyFor(p.Name), payload, map[string]string{"plugin": p.Name})
```

NEW:
```go
		_, _ = o.gateway.EmitConfigEvent(EventApplyFor(p.Name), payload)
```

#### EDIT 3.7: publishAbort

OLD:
```go
func (o *TxCoordinator) publishAbort(reason string) {
	ev := AbortEvent{TransactionID: o.txID, Reason: reason}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicVerifyAbort, payload, nil)
}
```

NEW:
```go
func (o *TxCoordinator) publishAbort(reason string) {
	ev := AbortEvent{TransactionID: o.txID, Reason: reason}
	payload, _ := json.Marshal(ev)
	_, _ = o.gateway.EmitConfigEvent(EventVerifyAbort, payload)
}
```

#### EDIT 3.8: publishRollback

OLD:
```go
func (o *TxCoordinator) publishRollback(reason string) {
	ev := RollbackEvent{TransactionID: o.txID, Reason: reason}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicRollback, payload, nil)
}
```

NEW:
```go
func (o *TxCoordinator) publishRollback(reason string) {
	ev := RollbackEvent{TransactionID: o.txID, Reason: reason}
	payload, _ := json.Marshal(ev)
	_, _ = o.gateway.EmitConfigEvent(EventRollback, payload)
}
```

#### EDIT 3.9: publishCommitted

OLD:
```go
func (o *TxCoordinator) publishCommitted() {
	ev := CommittedEvent{TransactionID: o.txID}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicCommitted, payload, nil)
}
```

NEW:
```go
func (o *TxCoordinator) publishCommitted() {
	ev := CommittedEvent{TransactionID: o.txID}
	payload, _ := json.Marshal(ev)
	_, _ = o.gateway.EmitConfigEvent(EventCommitted, payload)
}
```

#### EDIT 3.10: publishApplied

OLD:
```go
func (o *TxCoordinator) publishApplied(saved bool) {
	ev := AppliedEvent{TransactionID: o.txID, Saved: saved}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicApplied, payload, nil)
}
```

NEW:
```go
func (o *TxCoordinator) publishApplied(saved bool) {
	ev := AppliedEvent{TransactionID: o.txID, Saved: saved}
	payload, _ := json.Marshal(ev)
	_, _ = o.gateway.EmitConfigEvent(EventApplied, payload)
}
```

#### EDIT 3.11: unsubscribeAcks

OLD:
```go
// unsubscribeAcks removes all bus subscriptions created by subscribeAcks.
func (o *TxCoordinator) unsubscribeAcks() {
	for _, sub := range o.subs {
		o.bus.Unsubscribe(sub)
	}
	o.subs = nil
}
```

NEW:
```go
// unsubscribeAcks calls each unsubscribe function recorded by subscribeAcks.
func (o *TxCoordinator) unsubscribeAcks() {
	for _, unsub := range o.unsubs {
		unsub()
	}
	o.unsubs = nil
}
```

#### EDIT 3.12: delete consumerFunc adapter

The `consumerFunc` type and its `Deliver` method are no longer used (the
gateway uses plain Go callbacks). Delete them from the bottom of the file.

OLD:
```go
// consumerFunc adapts a function to the ze.Consumer interface.
type consumerFunc func([]ze.Event) error

func (f consumerFunc) Deliver(events []ze.Event) error { return f(events) }
```

NEW:
(deleted -- nothing replaces it)

### EDIT 4: verification (with caveats)

```
go vet ./internal/component/config/transaction/... 2>&1
```

Expected result: **build still fails**, but on different errors than before
this handoff. The orchestrator.go errors should be GONE; the remaining
errors will all be in `orchestrator_test.go` referencing identifiers like
`testBus`, `TopicVerifyFor`, `TopicAckVerifyOK`, etc. that the test file
uses but no longer exist.

If `orchestrator.go` itself produces vet errors, fix them before proceeding.
The most likely causes:
- A missed `bus.` reference (search for it: `grep "o\.bus" internal/component/config/transaction/orchestrator.go` should return zero matches after EDIT 3 is complete)
- A missed `Topic*` reference (search: `grep "Topic" internal/component/config/transaction/orchestrator.go` should return zero matches)
- A missed `ze.` reference (search: `grep "ze\." internal/component/config/transaction/orchestrator.go` should return zero matches except in comments)

If all three greps return zero matches, the orchestrator.go side is done and
the remaining errors are entirely in the test file. That signals Phase 4b-i
is complete and Phase 4b-ii (test rewrite) is the next handoff.

Also verify `gofmt -l` is clean on the touched files:
```
gofmt -l internal/component/config/transaction/orchestrator.go internal/component/config/transaction/gateway.go internal/component/plugin/server/engine_event_gateway.go 2>&1
```

Should produce no output.

## After Phase 4b-i

Phase 4b-ii rewrites `orchestrator_test.go`. The plan:

1. Replace the `testBus` type with `testGateway` that satisfies
   `transaction.EventGateway`. The new type has emit/subscribe methods
   instead of Publish/Subscribe and stores `published []emittedEvent`
   keyed by event type instead of topic.
2. Update `newTestOrchestrator` to construct via `NewTxCoordinator(gw, ...)`
   where `gw *testGateway`.
3. Update `testParticipant.respondVerify/respondApply/respondRollback` to
   call `gw.dispatchEvent(eventType, payload)` (a test helper that fires
   subscribed handlers, simulating an emit-event from the plugin side).
4. Update `waitForPublish(t, bus, topic)` to `waitForEmit(t, gw, eventType)`.
5. Replace every `TopicVerifyFor("X")` with `EventVerifyFor("X")`, every
   `TopicApplyFor("X")` with `EventApplyFor("X")`, every `TopicVerifyAbort`
   with `EventVerifyAbort`, etc.

Phase 4b-ii is one big mechanical edit. I will write the handoff for it
after 4b-i lands (or after the orchestrator.go changes are confirmed
working in vet output).

After Phase 4b-ii: the build is GREEN. Phase 4c (reverse-tier rollback) and
Phase 4d (dependency-graph deadline) follow.

## Reference

- Source of truth: `plan/spec-config-tx-protocol.md`
- Phase 1 commit: `a7d42e3a feat(config-tx): Phase 1 stream event types and namespace registration`
- Phase 4a commit: just landed, contains engine pub/sub API
- Predecessor handoffs:
  - `.claude/handoff-config-tx-stream.md` (Phase 1, completed)
  - `.claude/handoff-config-tx-stream-phase4a.md` (Phase 4a, completed)
- Open deferrals: `plan/deferrals.md`
- Memory: `.claude/rules/memory.md` arch-0 entry (4 components, stream system as backbone)
