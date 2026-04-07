# Handoff: spec-config-tx-protocol Phase 4a (engine pub/sub API for stream system)

Generated 2026-04-07 after Phase 1 commit `a7d42e3a`.

## Why this handoff is "Phase 4a", not "Phase 4"

Phase 4 in the spec is "rewrite the TxCoordinator orchestrator pub/sub layer from
bus to stream." That is too large for a single handoff (the rules cap at 5 edits,
the orchestrator change touches ~15 callsites plus tests plus a new interface plus
test fakes plus reverse-tier logic plus dependency-graph deadline). Splitting:

| Sub-phase | What |
|-----------|------|
| **4a (this handoff)** | Foundation: give the engine a way to publish stream events directly and to subscribe to plugin-emitted events without being a plugin itself. New `Server` methods, no transaction code touched yet. |
| 4b (next handoff after 4a) | Define `EventGateway` interface in the transaction package, have `Server` implement it, rewrite `TxCoordinator` to use it instead of `ze.Bus`. Mechanical given 4a. |
| 4c (after 4b) | Add reverse-tier rollback ordering using `registry.TopologicalTiers`. |
| 4d (after 4c) | Add dependency-graph-aware deadline computation (sum chains, max independent). |

After 4d, Phase 7 (wire `reload.go` to `TxCoordinator`) becomes the next focus.

## RATIONALE (verify this matches what was agreed)

- **Decision: engine needs first-class pub/sub on the stream system, parallel to plugin RPC** -> EDITs 1-3
  Reason: today the stream system (`internal/component/plugin/server/dispatch.go`)
  routes events between plugin processes via the `subscribe-events` /
  `emit-event` / `deliver-event` RPCs and the `SubscriptionManager`. The
  orchestrator is engine-side code, not a plugin. It cannot register a
  `*process.Process` to subscribe to events, and it cannot use the RPC path
  to emit. It needs a parallel internal API: register Go callbacks for
  event types, dispatch in-process at the `deliverEvent` boundary.

- **Decision: parallel mechanism, not "fake plugin process"** -> EDITs 1-3
  Reason: making the orchestrator pretend to be a `*process.Process` would
  require fake processes, fake `Deliver` methods, fake cleanup paths, and
  would conflate plugin lifecycle with engine subscriptions. A separate
  `engineEventSubscribers` registry is cleaner: small, focused, no lifecycle.

- **Decision: engine handlers receive raw `[]byte` payloads, not `string`** -> EDIT 1
  Reason: the existing stream system carries `string` (the `Event` field in
  `EmitEventInput.Event` is a JSON string, line 145 of `pkg/plugin/rpc/types.go`).
  But for the orchestrator we will be marshaling/unmarshaling JSON struct payloads
  anyway. `[]byte` is the correct type for "opaque payload" and matches the
  bus interface convention. The conversion at the boundary (cast to/from `string`
  inside `deliverEvent`) is one line.
  Implementation note: since the existing `deliverEvent` uses `string event`,
  the engine subscriber dispatch should receive the same string and let callers
  cast `[]byte(eventStr)` if they need bytes. This avoids changing `deliverEvent`'s
  signature, which is called from many places.

- **Decision: engine subscribers fire AFTER plugin process subscribers** -> EDIT 2
  Reason: the existing semantics are "publish, fan out to all subscribers."
  Adding engine subscribers should not reorder existing plugin delivery.
  Engine handlers run after, in the same goroutine, with the same payload.

- **Decision: engine subscribers are NOT excluded from self-delivery** -> EDIT 1, EDIT 2
  Reason: the "exclude emitter" rule in `deliverEvent` (line 364-366) prevents
  a plugin from receiving its own emit. Engine subscribers have no such notion
  of "self" because the engine isn't a plugin. The engine can emit a config
  event and subscribe to acks from the same orchestrator instance — that is
  exactly the design.

- **Decision: subscribe returns an unsubscribe function, not a handle** -> EDIT 1
  Reason: matches the goroutine cleanup pattern. The orchestrator subscribes
  at the start of `Execute()` and unsubscribes via `defer unsub()` at the end.
  Cleaner than tracking opaque handles.

- **Decision: handler invocation is synchronous** -> EDIT 1
  Reason: the orchestrator pushes to channels in its handlers (matching the
  current bus consumer pattern). Synchronous dispatch from `deliverEvent` is
  fine because the channels are buffered. If a handler blocks, that is the
  handler author's bug, same as today's bus consumers. No goroutine spawning
  in the dispatcher.

- **Open: thread safety** -> EDIT 1
  The new subscriber registry needs a mutex. Standard `sync.RWMutex` is fine.
  Subscribe / unsubscribe acquire write lock; dispatch acquires read lock
  while iterating (or copies the handler slice under read lock and dispatches
  outside, to avoid holding the lock during user-code execution).
  Recommendation: copy under read lock, dispatch outside. See EDIT 1 sketch.

- **Open: should engine emit go through `deliverEvent` or have its own path?**
  Recommendation: have its own thin entry point `EmitEngineEvent` that calls
  the existing private `deliverEvent` with `nil` emitter. Reuses the validation
  (`plugin.IsValidEvent`) and the plugin process delivery loop. The existing
  `deliverEvent` already accepts `nil` emitter conceptually — line 365 checks
  `if p == emitter { continue }` and `nil == nil` is fine, but the loop will
  still skip the nil process. Verify this works or pass a sentinel.

  Actually, looking again at lines 363-371: if `emitter` is `nil`, the loop
  iterates over `procs` and `if p == nil { continue }` skips nothing real.
  So `nil` emitter is safe — no plugin will be skipped because no plugin
  process is `nil`. This is good. EmitEngineEvent passes `nil`.

- **NOT IN THIS HANDOFF: TxCoordinator changes** -> Phase 4b
  This handoff only adds the engine pub/sub API. The orchestrator continues
  to use the bus interface and the build remains red on `orchestrator.go`.
  Phase 4b rewrites the orchestrator to use the new API.

If any rationale bullet is wrong, STOP and fix the handoff before applying edits.

## FILES ALREADY HANDLED (do not re-read)

- `plan/spec-config-tx-protocol.md` — source of truth, status `in-progress` Phase 1/8 (will become 2/8 after this lands).
- `internal/component/plugin/events.go` — Phase 1 added `NamespaceConfig` and 12 config event types. Already committed in `a7d42e3a`. Read only the new constants if you need them.
- `internal/component/config/transaction/topics.go` — Phase 1 rewrote this to re-export the config event-type constants from the `plugin` package. Includes `ReservedPluginNames` and `ValidatePluginName` (catches plugin-name vs event-type collisions). Already committed.
- `internal/component/plugin/server/subscribe.go` — read for context, defines `Subscription` and `SubscriptionManager` for plugin-process subscriptions. The new engine mechanism is parallel, not a modification of this.
- `internal/component/plugin/server/server.go` lines 59-94 — read for context, `Server` struct definition. EDIT 3 adds one field here.
- `internal/component/plugin/server/dispatch.go` lines 315-374 — read for context, `emitEvent`, `deliverEvent`, `handleEmitEventDirect`. EDIT 2 adds engine dispatch to `deliverEvent`.

## EDITS

### EDIT 1: new file `internal/component/plugin/server/engine_event.go`

Create a new file containing the engine subscriber registry and the public methods on `Server` that the orchestrator (and future engine code) will use.

```go
// Design: docs/architecture/api/process-protocol.md -- engine-side stream pub/sub
// Related: dispatch.go -- deliverEvent fans out to engine handlers as well as plugins
// Related: subscribe.go -- parallel mechanism for plugin-process subscriptions

package server

import (
	"sync"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// EngineEventHandler is invoked when a stream event matches an engine subscription.
// The event string is the same payload that plugin subscribers receive (typically
// JSON). Handlers are called synchronously from deliverEvent. Handlers MUST NOT
// block on external I/O; push to a buffered channel and return if work is needed.
type EngineEventHandler func(event string)

// engineEventSubscribers tracks engine-side subscriptions to stream events.
// Parallel to SubscriptionManager (which tracks plugin-process subscriptions).
// Engine handlers fire from deliverEvent in addition to plugin process delivery.
type engineEventSubscribers struct {
	mu       sync.RWMutex
	nextID   uint64
	handlers map[engineSubKey]map[uint64]EngineEventHandler
}

// engineSubKey identifies an engine subscription by namespace and event type.
type engineSubKey struct {
	Namespace string
	EventType string
}

func newEngineEventSubscribers() *engineEventSubscribers {
	return &engineEventSubscribers{
		handlers: make(map[engineSubKey]map[uint64]EngineEventHandler),
	}
}

// register adds a handler and returns its unique id.
// Caller MUST hold no other locks (acquires mu).
func (e *engineEventSubscribers) register(namespace, eventType string, handler EngineEventHandler) uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextID++
	id := e.nextID
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	if _, ok := e.handlers[key]; !ok {
		e.handlers[key] = make(map[uint64]EngineEventHandler)
	}
	e.handlers[key][id] = handler
	return id
}

// unregister removes a handler by id. No-op if id is unknown.
func (e *engineEventSubscribers) unregister(namespace, eventType string, id uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	if m, ok := e.handlers[key]; ok {
		delete(m, id)
		if len(m) == 0 {
			delete(e.handlers, key)
		}
	}
}

// dispatch invokes all handlers registered for (namespace, eventType).
// Handlers are copied under read lock and invoked OUTSIDE the lock so user code
// (which may itself try to register/unregister) does not deadlock.
func (e *engineEventSubscribers) dispatch(namespace, eventType, event string) {
	e.mu.RLock()
	key := engineSubKey{Namespace: namespace, EventType: eventType}
	m, ok := e.handlers[key]
	if !ok || len(m) == 0 {
		e.mu.RUnlock()
		return
	}
	handlers := make([]EngineEventHandler, 0, len(m))
	for _, h := range m {
		handlers = append(handlers, h)
	}
	e.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// EmitEngineEvent publishes an event from the engine to the stream system.
// Both engine subscribers and plugin process subscribers receive it.
// Returns the number of plugin processes that received the event (engine
// handler count is intentionally not reported because engine subscribers
// are in-process and always receive synchronously when matching).
//
// The event must use a registered (namespace, eventType) per plugin.IsValidEvent;
// unknown pairs return an error and deliver to nobody.
func (s *Server) EmitEngineEvent(namespace, eventType, event string) (int, error) {
	// Reuse the existing deliverEvent path. Passing nil emitter means no plugin
	// process will be excluded from delivery (the existing exclusion check
	// "if p == emitter" never matches a real process when emitter is nil).
	return s.deliverEvent(nil, namespace, eventType, "", "", event)
}

// SubscribeEngineEvent registers an engine-side handler for stream events
// matching the given namespace and event type. The returned function
// unregisters the handler when called; safe to call multiple times.
//
// Handlers fire synchronously from deliverEvent. They must not block on
// external I/O. The handler receives the same event string that plugin
// process subscribers would receive.
//
// Engine subscriptions are parallel to plugin process subscriptions managed
// by SubscriptionManager. Both fire on the same deliverEvent call.
func (s *Server) SubscribeEngineEvent(namespace, eventType string, handler EngineEventHandler) func() {
	if s.engineSubscribers == nil {
		// Defensive: should never happen because NewServer initialises it.
		// Return a no-op unsubscribe so callers can defer it safely.
		return func() {}
	}
	id := s.engineSubscribers.register(namespace, eventType, handler)
	return func() {
		s.engineSubscribers.unregister(namespace, eventType, id)
	}
}

// dispatchEngineEvent is called from deliverEvent after plugin process
// delivery, to fire any engine-side handlers. Validates the namespace
// matches plugin.IsValidNamespace; unknown namespaces are silently ignored
// (matches the deliverEvent contract that already validates upstream).
func (s *Server) dispatchEngineEvent(namespace, eventType, event string) {
	if s.engineSubscribers == nil {
		return
	}
	if !plugin.IsValidNamespace(namespace) {
		return
	}
	s.engineSubscribers.dispatch(namespace, eventType, event)
}
```

### EDIT 2: `internal/component/plugin/server/dispatch.go` line 347

Update `deliverEvent` to fire engine handlers in addition to plugin process delivery. Add the call AFTER the existing for-loop over plugin processes, INSIDE the `deliverEvent` function.

OLD (lines 347-374, the entire `deliverEvent` body):

```go
func (s *Server) deliverEvent(emitter *process.Process, namespace, eventType, direction, peerAddress, event string) (int, error) {
	if namespace == "" || eventType == "" || event == "" {
		return 0, &rpc.RPCCallError{Message: "emit-event requires namespace, event-type, and event"}
	}

	// Validate event type exists in the namespace (uses canonical registry).
	if !plugin.IsValidEvent(namespace, eventType) {
		return 0, &rpc.RPCCallError{Message: "unknown event: " + namespace + "/" + eventType}
	}

	if s.subscriptions == nil {
		return 0, nil
	}

	procs := s.subscriptions.GetMatching(namespace, eventType, direction, peerAddress, "")
	delivered := 0
	for _, p := range procs {
		// Skip self-delivery to prevent loops.
		if p == emitter {
			continue
		}
		if p.Deliver(process.EventDelivery{Output: event}) {
			delivered++
		}
	}

	return delivered, nil
}
```

NEW: change two things — (1) early-return from the `subscriptions == nil` check should still dispatch engine handlers, (2) add engine dispatch after the plugin loop.

```go
func (s *Server) deliverEvent(emitter *process.Process, namespace, eventType, direction, peerAddress, event string) (int, error) {
	if namespace == "" || eventType == "" || event == "" {
		return 0, &rpc.RPCCallError{Message: "emit-event requires namespace, event-type, and event"}
	}

	// Validate event type exists in the namespace (uses canonical registry).
	if !plugin.IsValidEvent(namespace, eventType) {
		return 0, &rpc.RPCCallError{Message: "unknown event: " + namespace + "/" + eventType}
	}

	// Engine-side subscribers fire regardless of whether the plugin
	// SubscriptionManager is initialised. They are a parallel registry.
	defer s.dispatchEngineEvent(namespace, eventType, event)

	if s.subscriptions == nil {
		return 0, nil
	}

	procs := s.subscriptions.GetMatching(namespace, eventType, direction, peerAddress, "")
	delivered := 0
	for _, p := range procs {
		// Skip self-delivery to prevent loops.
		if p == emitter {
			continue
		}
		if p.Deliver(process.EventDelivery{Output: event}) {
			delivered++
		}
	}

	return delivered, nil
}
```

Why `defer`: ensures engine handlers fire even if a plugin subscriber panics or an early return path is added later. Engine handlers fire after plugin delivery in normal flow because Go executes deferred calls in LIFO at function exit, and this is the only deferred call in the function. If a future change adds another defer, reorder to ensure engine dispatch is still last.

### EDIT 3: `internal/component/plugin/server/server.go` line 70 (Server struct)

Add the engine subscribers field to the Server struct.

OLD (lines 69-70):
```
	subscriptions   *SubscriptionManager  // API-driven event subscriptions
	monitors        *MonitorManager       // CLI monitor subscriptions
```

NEW:
```
	subscriptions     *SubscriptionManager      // API-driven event subscriptions (plugin processes)
	engineSubscribers *engineEventSubscribers   // Engine-side stream subscribers (orchestrator etc.)
	monitors          *MonitorManager           // CLI monitor subscriptions
```

In `NewServer` (around line 145, find the existing initialisation of `subscriptions`), add the initialiser. Since I cannot see the exact `NewServer` body in this handoff context, the next session must read `NewServer` and add `engineSubscribers: newEngineEventSubscribers(),` adjacent to wherever `subscriptions: NewSubscriptionManager()` is initialised. If `subscriptions` is initialised separately after the struct literal, do the same for `engineSubscribers`. Either pattern is fine.

If `NewServer` does not currently initialise `subscriptions` (some servers leave it nil and set it later), set `engineSubscribers` in the same place using the same lifecycle.

### EDIT 4: new file `internal/component/plugin/server/engine_event_test.go`

Create unit tests for the new engine pub/sub mechanism. Use a minimal Server constructed via test helpers if available, or instantiate `engineEventSubscribers` directly for unit-level coverage. The orchestrator will exercise the `Server.EmitEngineEvent` / `Server.SubscribeEngineEvent` end-to-end in Phase 4b's tests, so unit tests here focus on the registry mechanics.

```go
package server

import (
	"sync"
	"testing"
)

func TestEngineSubscribersRegisterAndDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()

	var (
		mu       sync.Mutex
		received []string
	)
	id := subs.register("config", "verify-ok", func(event string) {
		mu.Lock()
		received = append(received, event)
		mu.Unlock()
	})
	if id == 0 {
		t.Fatalf("register returned id 0, expected non-zero")
	}

	subs.dispatch("config", "verify-ok", `{"plugin":"bgp"}`)
	subs.dispatch("config", "verify-ok", `{"plugin":"iface"}`)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 dispatches, got %d", len(received))
	}
	if received[0] != `{"plugin":"bgp"}` || received[1] != `{"plugin":"iface"}` {
		t.Errorf("unexpected dispatch payloads: %v", received)
	}
}

func TestEngineSubscribersUnregister(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls int
	id := subs.register("config", "apply-ok", func(_ string) { calls++ })

	subs.dispatch("config", "apply-ok", `first`)
	subs.unregister("config", "apply-ok", id)
	subs.dispatch("config", "apply-ok", `second`)

	if calls != 1 {
		t.Errorf("expected 1 call after unregister, got %d", calls)
	}
}

func TestEngineSubscribersMultipleHandlersForSameEvent(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls1, calls2 int
	subs.register("config", "rollback-ok", func(_ string) { calls1++ })
	subs.register("config", "rollback-ok", func(_ string) { calls2++ })

	subs.dispatch("config", "rollback-ok", `payload`)

	if calls1 != 1 || calls2 != 1 {
		t.Errorf("expected each handler called once, got %d and %d", calls1, calls2)
	}
}

func TestEngineSubscribersDispatchUnknownNoop(t *testing.T) {
	subs := newEngineEventSubscribers()
	// Dispatching to an event with no handlers must not panic.
	subs.dispatch("config", "verify-ok", `payload`)
}

func TestEngineSubscribersHandlerCanUnregisterDuringDispatch(t *testing.T) {
	// Sanity: a handler that unregisters itself during dispatch must not deadlock.
	// dispatch copies handlers under read lock and invokes outside the lock,
	// so re-acquiring the write lock from within a handler is safe.
	subs := newEngineEventSubscribers()
	var id uint64
	id = subs.register("config", "committed", func(_ string) {
		subs.unregister("config", "committed", id)
	})
	subs.dispatch("config", "committed", `payload`)
	// Second dispatch should not call the handler (unregistered).
	subs.dispatch("config", "committed", `payload`)
}
```

### EDIT 5: verification

```
go vet ./internal/component/plugin/server/... 2>&1
go test -race ./internal/component/plugin/server/... -run TestEngineSubscribers 2>&1
go test -race ./internal/component/plugin/server/... 2>&1
```

The first run should be clean. The second runs only the new tests; expect green. The third runs the full server package tests; expect green except for any test that already failed before this handoff (none expected from Phase 1 work).

The transaction package will still fail to build because `orchestrator.go` references the old bus topic constants — that is unchanged from the Phase 1 endpoint. Do not fix `orchestrator.go` in this handoff. Phase 4b is the next handoff and starts the actual orchestrator rewrite.

## After Phase 4a

The next handoff will be Phase 4b: rewrite `TxCoordinator` to use `Server.EmitEngineEvent` and `Server.SubscribeEngineEvent` instead of `ze.Bus`. Sketch of what 4b will need to do:

1. Define `EventGateway` interface in the transaction package with methods that match the orchestrator's needs (`EmitConfigEvent`, `SubscribeConfigEvent`, `Unsubscribe`).
2. Server implements `EventGateway` via a thin adapter (each method delegates to `EmitEngineEvent`/`SubscribeEngineEvent` with namespace = `plugin.NamespaceConfig`).
3. Change `NewTxCoordinator` signature: `bus ze.Bus` becomes `gateway EventGateway`.
4. Rewrite `subscribeAcks`, `runVerify`, `runApply`, `collectRollbackAcks`, `publishAbort`, `publishRollback`, `publishCommitted`, `publishApplied` to use `gateway.Emit*` and `gateway.Subscribe*`. The `verifyOKCh`, `applyOKCh` etc. channels stay; only the way events arrive at them changes.
5. Rewrite `orchestrator_test.go` `testBus` to be a `testGateway` that implements `EventGateway` directly. Should be smaller than the current `testBus` because it doesn't need bus prefix matching.
6. Drop the `ze.Bus` import from the transaction package. Still cannot delete `pkg/ze/bus.go` because nothing imports it now but the file remains. Phase 10 deletes it.

Phase 4b is a single handoff: ~5 edits (gateway interface, server adapter, NewTxCoordinator + helpers, test fakes, verify).

## Reference

- Source of truth: `plan/spec-config-tx-protocol.md`
- Phase 1 commit: `a7d42e3a feat(config-tx): Phase 1 stream event types and namespace registration`
- Predecessor handoff: `.claude/handoff-config-tx-stream.md` (Phase 1, completed)
- Open deferrals: `plan/deferrals.md`
- Memory: `.claude/rules/memory.md` arch-0 entry (4 components, stream system as backbone)
