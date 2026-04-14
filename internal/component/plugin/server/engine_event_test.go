package server

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// VALIDATES: register stores a handler and dispatch invokes it once per event.
// PREVENTS: silent handler loss or duplicate delivery in the engine subscriber registry.
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

// VALIDATES: unregister removes a handler and subsequent dispatches do not fire it.
// PREVENTS: leaked handlers receiving events after the orchestrator releases them.
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

// VALIDATES: multiple handlers for the same (namespace, event) all fire.
// PREVENTS: only the first or last registered handler being called.
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

// VALIDATES: dispatching to an event with no handlers is a no-op (no panic).
// PREVENTS: nil-map dereference when emitting before any subscribers register.
func TestEngineSubscribersDispatchUnknownNoop(t *testing.T) {
	subs := newEngineEventSubscribers()
	// Dispatching to an event with no handlers must not panic.
	subs.dispatch("config", "verify-ok", `payload`)
}

// VALIDATES: a handler that unregisters itself during dispatch does not deadlock.
// PREVENTS: re-acquiring the write lock from within a handler causing self-deadlock.
// dispatch copies handlers under read lock and invokes them outside the lock,
// so re-entry into register/unregister from within a handler is safe.
func TestEngineSubscribersHandlerCanUnregisterDuringDispatch(t *testing.T) {
	subs := newEngineEventSubscribers()
	var id uint64
	id = subs.register("config", "committed", func(_ string) {
		subs.unregister("config", "committed", id)
	})
	subs.dispatch("config", "committed", `payload`)
	// Second dispatch should not call the handler (unregistered).
	subs.dispatch("config", "committed", `payload`)
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
	bID = subs.register("config", "applied", func(_ string) { bCalls++ })

	// Handler A unregisters B when invoked.
	subs.register("config", "applied", func(_ string) {
		subs.unregister("config", "applied", bID)
	})

	subs.dispatch("config", "applied", `payload`)
	// During this single dispatch, both A and B were in the local snapshot.
	// Even though A unregistered B, the snapshot still contained B, so B fired
	// exactly once. Map iteration order is randomized, so we check the count
	// rather than ordering.
	if bCalls != 1 {
		t.Errorf("first dispatch: expected B called once, got %d", bCalls)
	}

	subs.dispatch("config", "applied", `payload`)
	// On the second dispatch, B is gone from the registry; it should not fire.
	if bCalls != 1 {
		t.Errorf("second dispatch: expected B count still 1, got %d", bCalls)
	}
}

// VALIDATES: register rejects a nil handler with id 0 and stores nothing.
// PREVENTS: nil dispatch crash from a programmer error in Phase 4b wiring.
func TestEngineSubscribersRegisterNilHandler(t *testing.T) {
	subs := newEngineEventSubscribers()

	id := subs.register("config", "verify-ok", nil)
	if id != 0 {
		t.Errorf("register(nil) returned id %d, want 0", id)
	}
	// Dispatch must not panic even though we attempted to register nil.
	subs.dispatch("config", "verify-ok", `payload`)
}

// VALIDATES: unregister(0) is a no-op (does not panic, does not affect other handlers).
// PREVENTS: callers that received id 0 from a failed register accidentally
// removing valid registrations when they call the returned unsub.
func TestEngineSubscribersUnregisterZeroIsNoop(t *testing.T) {
	subs := newEngineEventSubscribers()

	var calls int
	subs.register("config", "verify-ok", func(_ string) { calls++ })

	subs.unregister("config", "verify-ok", 0)
	subs.dispatch("config", "verify-ok", `payload`)

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
	subs.register("config", "verify-ok", func(_ string) {
		panic("boom")
	})
	subs.register("config", "verify-ok", func(_ string) {
		afterCalls++
	})

	// Map iteration order is randomized; run dispatch enough times that the
	// "after" handler is observed at least once after the panicking one.
	// Each dispatch must complete without re-raising the panic.
	for range 20 {
		subs.dispatch("config", "verify-ok", `payload`)
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
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(_ string) { calls++ })

	s.dispatchEngineEvent("config", "verify-ok", `payload`)
	if calls != 1 {
		t.Fatalf("expected 1 call before unsub, got %d", calls)
	}

	unsub()
	s.dispatchEngineEvent("config", "verify-ok", `payload`)
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
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(_ string) {})
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
	s.dispatchEngineEvent("config", "verify-ok", `payload`)
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
	unsub := s.SubscribeEngineEvent(txevents.Namespace, "verify-ok", func(event string) {
		got = event
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
	defer s.SubscribeEngineEvent("typo", "nonsense", func(_ string) { calls++ })()

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

// VALIDATES: EmitEngineEvent rejects empty namespace, event type, or event payload.
// PREVENTS: deliverEvent's empty-check being bypassed by the engine path.
func TestServerEmitEngineEventRejectsEmpty(t *testing.T) {
	s := &Server{engineSubscribers: newEngineEventSubscribers()}

	cases := []struct {
		name      string
		namespace string
		eventType string
		event     string
	}{
		{"empty namespace", "", "verify-ok", `payload`},
		{"empty event type", "config", "", `payload`},
		{"empty event payload", "config", "verify-ok", ``},
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
	s.dispatchEngineEvent("config", "verify-ok", `payload`)
}

// VALIDATES: engine subscribers in different namespaces with the same event
// type name do NOT cross-fire. (config, verify-ok) and (bgp, verify-ok) are
// independent subscriptions.
// PREVENTS: cross-namespace key collision in the engineSubKey map.
func TestEngineSubscribersNamespaceIsolation(t *testing.T) {
	subs := newEngineEventSubscribers()

	var configCalls, bgpCalls int
	subs.register("config", "update", func(_ string) { configCalls++ })
	subs.register("bgp", "update", func(_ string) { bgpCalls++ })

	subs.dispatch("config", "update", `payload`)
	if configCalls != 1 || bgpCalls != 0 {
		t.Errorf("after config dispatch: configCalls=%d bgpCalls=%d, want 1 0", configCalls, bgpCalls)
	}

	subs.dispatch("bgp", "update", `payload`)
	if configCalls != 1 || bgpCalls != 1 {
		t.Errorf("after bgp dispatch: configCalls=%d bgpCalls=%d, want 1 1", configCalls, bgpCalls)
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
				id := subs.register("config", "verify-ok", func(_ string) {
					atomic.AddInt64(&dispatched, 1)
				})
				subs.unregister("config", "verify-ok", id)
			}
		}()
		go func() {
			defer wg.Done()
			for range iters {
				subs.dispatch("config", "verify-ok", `payload`)
			}
		}()
	}
	wg.Wait()
	// We do not assert on dispatched count because the race between
	// register/unregister and dispatch is non-deterministic. The point is
	// the absence of data races and panics, validated by -race.
}
