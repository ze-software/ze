package server

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

func nsID(s string) events.NamespaceID { return events.LookupNamespaceID(s) }
func etID(s string) events.EventTypeID { return events.LookupEventTypeID(s) }

// mustString type-asserts to string for tests that passed string payloads.
// Uses t.Helper so the failure points at the caller, not this helper.
func mustString(t *testing.T, p any) string {
	t.Helper()
	s, ok := p.(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", p)
	}
	return s
}

// VALIDATES: register stores a handler and dispatch invokes it once per event.
// PREVENTS: silent handler loss or duplicate delivery in the engine subscriber registry.
func TestEngineSubscribersRegisterAndDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()

	var (
		mu       sync.Mutex
		received []string
	)
	id := subs.register(nsID("config"), etID("verify-ok"), func(p any) {
		mu.Lock()
		received = append(received, mustString(t, p))
		mu.Unlock()
	})
	if id == 0 {
		t.Fatalf("register returned id 0, expected non-zero")
	}

	subs.dispatch(nsID("config"), etID("verify-ok"), `{"plugin":"bgp"}`)
	subs.dispatch(nsID("config"), etID("verify-ok"), `{"plugin":"iface"}`)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 dispatches, got %d", len(received))
	}
	if received[0] != `{"plugin":"bgp"}` || received[1] != `{"plugin":"iface"}` {
		t.Errorf("unexpected dispatch payloads: %v", received)
	}
}

// VALIDATES: unregister removes a handler and subsequent dispatches do not fire it.
// PREVENTS: leaked handlers receiving events after the orchestrator releases them.
func TestEngineSubscribersUnregister(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls int
	id := subs.register(nsID("config"), etID("apply-ok"), func(_ any) { calls++ })

	subs.dispatch(nsID("config"), etID("apply-ok"), `first`)
	subs.unregister(nsID("config"), etID("apply-ok"), id)
	subs.dispatch(nsID("config"), etID("apply-ok"), `second`)

	if calls != 1 {
		t.Errorf("expected 1 call after unregister, got %d", calls)
	}
}

// VALIDATES: multiple handlers for the same (namespace, event) all fire.
// PREVENTS: only the first or last registered handler being called.
func TestEngineSubscribersMultipleHandlersForSameEvent(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls1, calls2 int
	subs.register(nsID("config"), etID("rollback-ok"), func(_ any) { calls1++ })
	subs.register(nsID("config"), etID("rollback-ok"), func(_ any) { calls2++ })

	subs.dispatch(nsID("config"), etID("rollback-ok"), `payload`)

	if calls1 != 1 || calls2 != 1 {
		t.Errorf("expected each handler called once, got %d and %d", calls1, calls2)
	}
}

// VALIDATES: dispatching to an event with no handlers is a no-op (no panic).
// PREVENTS: nil-map dereference when emitting before any subscribers register.
func TestEngineSubscribersDispatchUnknownNoop(t *testing.T) {
	subs := newEngineEventSubscribers()
	// Dispatching to an event with no handlers must not panic.
	subs.dispatch(nsID("config"), etID("verify-ok"), `payload`)
}

// VALIDATES: a handler that unregisters itself during dispatch does not deadlock.
// PREVENTS: re-acquiring the write lock from within a handler causing self-deadlock.
// dispatch copies handlers under read lock and invokes them outside the lock,
// so re-entry into register/unregister from within a handler is safe.
func TestEngineSubscribersHandlerCanUnregisterDuringDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()
	var id uint64
	id = subs.register(nsID("config"), etID("committed"), func(_ any) {
		subs.unregister(nsID("config"), etID("committed"), id)
	})
	subs.dispatch(nsID("config"), etID("committed"), `payload`)
	// Second dispatch should not call the handler (unregistered).
	subs.dispatch(nsID("config"), etID("committed"), `payload`)
}

// VALIDATES: when handler A unregisters peer handler B during dispatch, B is
// still called once this dispatch (it is in the local snapshot taken before
// the lock was released). Locks in the documented contract: unregister takes
// effect on the NEXT dispatch, not the current one.
// PREVENTS: a future implementation change that makes unregister immediate
// (skipping not-yet-invoked handlers in the current dispatch) without
// updating the documented contract.
func TestEngineSubscribersMidDispatchUnregisterTakesEffectNextDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()
	var bID uint64
	var bCalls int

	// Handler B records each invocation.
	bID = subs.register(nsID("config"), etID("applied"), func(_ any) { bCalls++ })

	// Handler A unregisters B when invoked.
	subs.register(nsID("config"), etID("applied"), func(_ any) {
		subs.unregister(nsID("config"), etID("applied"), bID)
	})

	subs.dispatch(nsID("config"), etID("applied"), `payload`)
	// During this single dispatch, both A and B were in the local snapshot.
	// Even though A unregistered B, the snapshot still contained B, so B fired
	// exactly once. Map iteration order is randomized, so we check the count
	// rather than ordering.
	if bCalls != 1 {
		t.Errorf("first dispatch: expected B called once, got %d", bCalls)
	}

	subs.dispatch(nsID("config"), etID("applied"), `payload`)
	// On the second dispatch, B is gone from the registry; it should not fire.
	if bCalls != 1 {
		t.Errorf("second dispatch: expected B count still 1, got %d", bCalls)
	}
}

// VALIDATES: register rejects a nil handler with id 0 and stores nothing.
// PREVENTS: nil dispatch crash from a programmer error in Phase 4b wiring.
func TestEngineSubscribersRegisterNilHandler(t *testing.T) {
	subs := newEngineEventSubscribers()

	id := subs.register(nsID("config"), etID("verify-ok"), nil)
	if id != 0 {
		t.Errorf("register(nil) returned id %d, want 0", id)
	}
	// Dispatch must not panic even though we attempted to register nil.
	subs.dispatch(nsID("config"), etID("verify-ok"), `payload`)
}

// VALIDATES: unregister(0) is a no-op (does not panic, does not affect other handlers).
// PREVENTS: callers that received id 0 from a failed register accidentally
// removing valid registrations when they call the returned unsub.
func TestEngineSubscribersUnregisterZeroIsNoop(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls int
	subs.register(nsID("config"), etID("verify-ok"), func(_ any) { calls++ })

	subs.unregister(nsID("config"), etID("verify-ok"), 0)
	subs.dispatch(nsID("config"), etID("verify-ok"), `payload`)

	if calls != 1 {
		t.Errorf("expected handler still active after unregister(0), got %d calls", calls)
	}
}

// VALIDATES: a panicking handler is recovered and the next handler still fires.
// The panic does not propagate to the dispatch caller.
// PREVENTS: a single buggy engine subscriber crashing the emitter
// (orchestrator, plugin, bridge) and breaking unrelated event delivery.
func TestEngineSubscribersHandlerPanicRecovered(t *testing.T) {
	subs := newEngineEventSubscribers()

	var afterCalls int
	subs.register(nsID("config"), etID("verify-ok"), func(_ any) {
		panic("boom")
	})
	subs.register(nsID("config"), etID("verify-ok"), func(_ any) {
		afterCalls++
	})

	// Map iteration order is randomized; run dispatch enough times that the
	// "after" handler is observed at least once after the panicking one.
	// Each dispatch must complete without re-raising the panic.
	for range 20 {
		subs.dispatch(nsID("config"), etID("verify-ok"), `payload`)
	}
	if afterCalls != 20 {
		t.Errorf("expected 20 calls to non-panicking handler, got %d", afterCalls)
	}
}

// VALIDATES: SubscribeEngineEvent returns a working unsubscribe function.
// PREVENTS: orchestrator's defer-unsub pattern silently leaking subscriptions.
func TestServerSubscribeEngineEventUnsub(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	var calls int
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(_ any) { calls++ })

	s.dispatchEngineEvent(nsID("config"), etID("verify-ok"), `payload`)
	if calls != 1 {
		t.Fatalf("expected 1 call before unsub, got %d", calls)
	}

	unsub()
	s.dispatchEngineEvent(nsID("config"), etID("verify-ok"), `payload`)
	if calls != 1 {
		t.Errorf("expected 1 call after unsub, got %d", calls)
	}
}

// VALIDATES: SubscribeEngineEvent on a Server with nil engineSubscribers
// returns a no-op unsubscribe function (the documented defensive path).
// PREVENTS: orchestrator deferring nil and crashing on shutdown if the
// Server was constructed without the registry.
func TestServerSubscribeEngineEventNilRegistry(t *testing.T) {
	s := &Server{} // engineSubscribers is nil
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(_ any) {})
	if unsub == nil {
		t.Fatal("SubscribeEngineEvent returned nil unsub function")
	}
	// Calling unsub must not panic on nil registry.
	unsub()
}

// VALIDATES: SubscribeEngineEvent rejects a nil handler with a no-op unsub
// instead of registering it (which would later crash dispatch).
// PREVENTS: nil handler stored in the registry causing dispatch panic.
func TestServerSubscribeEngineEventNilHandler(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", nil)
	if unsub == nil {
		t.Fatal("SubscribeEngineEvent returned nil unsub function")
	}
	unsub()

	// Verify nothing was actually registered: dispatch must be a no-op.
	s.dispatchEngineEvent(nsID("config"), etID("verify-ok"), `payload`)
	// No assertion needed beyond not panicking.
}

// VALIDATES: EmitEngineEvent end-to-end fires engine subscribers via the
// shared deliverEvent path, even when subscriptions (plugin process manager)
// is nil. Subscribers receive the same event payload that was emitted.
// PREVENTS: a future change to deliverEvent breaking the engine fan-out
// without a Server-level test catching it.
func TestServerEmitEngineEventEndToEnd(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	var got string
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(p any) {
		got = mustString(t, p)
	})
	defer unsub()

	delivered, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", `{"plugin":"bgp","code":"ok"}`)
	if err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	// No plugin processes; delivered count is 0. Engine handler still fires.
	if delivered != 0 {
		t.Errorf("delivered = %d, want 0 (no plugin processes)", delivered)
	}
	if got != `{"plugin":"bgp","code":"ok"}` {
		t.Errorf("handler received %q, want %q", got, `{"plugin":"bgp","code":"ok"}`)
	}
}

// VALIDATES: EmitEngineEvent rejects unknown (namespace, eventType) via the
// shared deliverEvent validation, and engine handlers do NOT fire on rejected
// emits (because the defer is registered after validation).
// PREVENTS: orchestrator typo silently no-op'ing AND engine handlers being
// triggered by an invalid event.
func TestServerEmitEngineEventRejectsUnknown(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	var calls int
	defer s.SubscribeEngineEvent("typo", "nonsense", func(_ any) { calls++ })()

	_, err := s.EmitEngineEvent("typo", "nonsense", `{}`)
	if err == nil {
		t.Fatal("expected error for unknown namespace/event, got nil")
	}
	var rpcErr *rpc.RPCCallError
	if !errors.As(err, &rpcErr) {
		t.Errorf("expected *rpc.RPCCallError, got %T", err)
	}
	if calls != 0 {
		t.Errorf("handler fired on rejected emit, calls = %d", calls)
	}
}

// VALIDATES: EmitEngineEvent rejects empty namespace and event type. Empty
// and nil payloads are valid under the typed-payload design (signal events
// carry nil; empty string payload is a non-nil typed value).
// PREVENTS: deliverEvent's empty-check regressing into payload rejection.
func TestServerEmitEngineEventRejectsEmpty(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	cases := []struct {
		name      string
		namespace string
		eventType string
		event     any
	}{
		{"empty namespace", "", "verify-ok", `payload`},
		{"empty event type", "config", "", `payload`},
	}
	for _, tc := range cases {
		_, err := s.EmitEngineEvent(tc.namespace, tc.eventType, tc.event)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

// VALIDATES: dispatchEngineEvent on a Server with nil engineSubscribers is
// a safe no-op (the defensive guard for use cases that construct a Server
// without the registry).
// PREVENTS: nil pointer dereference if a future test or harness builds a
// minimal Server and triggers deliverEvent.
func TestServerDispatchEngineEventNilRegistry(t *testing.T) {
	s := &Server{} // engineSubscribers is nil
	s.dispatchEngineEvent(nsID("config"), etID("verify-ok"), `payload`)
}

// VALIDATES: engine subscribers in different namespaces with the same event
// type name do NOT cross-fire. (config, verify-ok) and (bgp, verify-ok) are
// independent subscriptions.
// PREVENTS: cross-namespace key collision in the engineSubKey map.
func TestEngineSubscribersNamespaceIsolation(t *testing.T) {
	subs := newEngineEventSubscribers()

	var configCalls, bgpCalls int
	subs.register(nsID("config"), etID("update"), func(_ any) { configCalls++ })
	subs.register(nsID("bgp"), etID("update"), func(_ any) { bgpCalls++ })

	subs.dispatch(nsID("config"), etID("update"), `payload`)
	if configCalls != 1 || bgpCalls != 0 {
		t.Errorf("after config dispatch: configCalls=%d bgpCalls=%d, want 1 0", configCalls, bgpCalls)
	}

	subs.dispatch(nsID("bgp"), etID("update"), `payload`)
	if configCalls != 1 || bgpCalls != 1 {
		t.Errorf("after bgp dispatch: configCalls=%d bgpCalls=%d, want 1 1", configCalls, bgpCalls)
	}
}

// VALIDATES: AC-2 — in-process subscriber receives the publisher's pointer
// untouched. No JSON marshal happens for an engine-only delivery.
// PREVENTS: a future change accidentally serializing the engine path.
func TestServerEmitTypedPayloadInProcess(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	type myPayload struct{ N int }
	emitted := &myPayload{N: 42}

	var got *myPayload
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(p any) {
		v, ok := p.(*myPayload)
		if !ok {
			t.Errorf("expected *myPayload, got %T", p)
			return
		}
		got = v
	})
	defer unsub()

	if _, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", emitted); err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	if got != emitted {
		t.Errorf("subscriber received different pointer (got=%p emitted=%p)", got, emitted)
	}
}

// VALIDATES: AC-3 — when no plugin-process subscribers exist, deliverEvent
// does not invoke json.Marshal even for a non-string payload.
// PREVENTS: regressing to eager marshal on every Emit.
func TestServerEmitSkipsMarshalWhenNoExternalSubs(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	// marshalCounter wraps a payload with a json.Marshaler that increments
	// on every Marshal call. Because there are no plugin-process subscribers,
	// the bus must not call it.
	var marshalCalls int
	payload := &marshalCounter{calls: &marshalCalls}

	var fired bool
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(_ any) {
		fired = true
	})
	defer unsub()

	if _, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", payload); err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	if !fired {
		t.Error("engine subscriber did not fire")
	}
	if marshalCalls != 0 {
		t.Errorf("MarshalJSON called %d times, want 0 (no external subs)", marshalCalls)
	}
}

// marshalCounter satisfies json.Marshaler and counts calls so tests can
// verify whether the bus invoked json.Marshal.
type marshalCounter struct {
	calls *int
}

func (m *marshalCounter) MarshalJSON() ([]byte, error) {
	*m.calls++
	return []byte(`"counter"`), nil
}

// VALIDATES: AC-6 — nil payload is a valid signal-only event. Handlers
// receive nil; emit succeeds.
// PREVENTS: a nil payload being rejected as "empty event".
func TestServerEmitNilPayload(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	var fired bool
	var received any = "sentinel"
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(p any) {
		fired = true
		received = p
	})
	defer unsub()

	if _, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", nil); err != nil {
		t.Fatalf("EmitEngineEvent(nil): %v", err)
	}
	if !fired {
		t.Error("engine subscriber did not fire on nil payload")
	}
	if received != nil {
		t.Errorf("subscriber received %v, want nil", received)
	}
}

// VALIDATES: empty-string payload flows through the bus to engine
// subscribers under the new typed contract. The old deliverEvent guard
// "event == ”" no longer exists; callers that need to forbid empty
// payloads (e.g. ConfigEventGateway) gate at their own layer. This test
// pins the new behavior so the guard cannot silently reappear.
// PREVENTS: an accidental re-introduction of the empty-string rejection
// breaking every replay-request style signal that used to emit "".
func TestServerEmitEmptyStringPayload(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	var got any = "sentinel"
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(p any) {
		got = p
	})
	defer unsub()

	delivered, err := s.EmitEngineEvent(txevents.Namespace, "verify-ok", "")
	if err != nil {
		t.Fatalf("EmitEngineEvent empty-string: %v", err)
	}
	if delivered != 0 {
		t.Errorf("delivered = %d, want 0 (no plugin processes)", delivered)
	}
	s2, ok := got.(string)
	if !ok {
		t.Fatalf("subscriber received %T, want string", got)
	}
	if s2 != "" {
		t.Errorf("subscriber received %q, want empty string", s2)
	}
}

// VALIDATES: payloadToJSON returns "null" for nil, passes string and
// json.RawMessage through, and marshals other types once.
// PREVENTS: regressions where the lazy-marshal helper allocates twice
// or mishandles the pre-marshaled paths.
func TestPayloadToJSON(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "null"},
		{"string", `{"k":"v"}`, `{"k":"v"}`},
		{"raw-message", json.RawMessage(`{"k":1}`), `{"k":1}`},
		{"struct", struct {
			A int `json:"a"`
		}{A: 7}, `{"a":7}`},
	}
	for _, tc := range cases {
		got, err := payloadToJSON(txevents.Namespace, "verify-ok", tc.in)
		if err != nil {
			t.Errorf("%s: payloadToJSON err = %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: payloadToJSON = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// VALIDATES: concurrent register/unregister/dispatch do not race.
// PREVENTS: data races introduced by future locking changes.
// Run under -race to be meaningful.
func TestEngineSubscribersConcurrentRegisterDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()

	const goroutines = 8
	const iters = 200
	var wg sync.WaitGroup
	var dispatched int64

	for range goroutines {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range iters {
				id := subs.register(nsID("config"), etID("verify-ok"), func(_ any) {
					atomic.AddInt64(&dispatched, 1)
				})
				subs.unregister(nsID("config"), etID("verify-ok"), id)
			}
		}()
		go func() {
			defer wg.Done()
			for range iters {
				subs.dispatch(nsID("config"), etID("verify-ok"), `payload`)
			}
		}()
	}
	wg.Wait()
	// We do not assert on dispatched count because the race between
	// register/unregister and dispatch is non-deterministic. The point is
	// the absence of data races and panics, validated by -race.
}
