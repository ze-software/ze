package observe

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/ze"
)

// observeTestBus is a synchronous in-process ze.EventBus: Emit delivers to the
// matching subscribers before it returns, so a test needs no polling.
//
// Unlike the detect package's dtestBus, the unsubscribe it returns really
// removes the handler. A no-op unsubscribe would make every assertion about
// detaching a replaced store pass regardless of the code under test.
type observeTestBus struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]func(any)
}

func newObserveTestBus() *observeTestBus {
	return &observeTestBus{subs: make(map[string]map[int]func(any))}
}

func (b *observeTestBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	registered := b.subs[namespace+"\x00"+eventType]
	handlers := make([]func(any), 0, len(registered))
	for _, h := range registered {
		handlers = append(handlers, h)
	}
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *observeTestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "\x00" + eventType
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.subs[key] == nil {
		b.subs[key] = make(map[int]func(any))
	}
	b.subs[key][id] = handler
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[key], id)
	}
}

var _ ze.EventBus = (*observeTestBus)(nil)

// VALIDATES: stale-incident-timeout reaches a running worker -- the sweep ticker
// finalizes an incident that never received an AttackCleared.
// PREVENTS: the leaf reaching the store and stopping there. sweepStale had a test
// caller only, so an incident with no clear event stayed open until
// incident-ring-size evicted it, whatever the operator configured.
//
// The test drives the WORKER, not sweepStale: a test that calls the function
// proves the function and never proves that anything calls it
// (plan/journal/unwired-feature.md, 2026-09-05).
func TestDdosObserveStaleSweepTickerFinalizes(t *testing.T) {
	s := newStore(10, 10*time.Millisecond)
	s.open(&ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.9/32")},
		Family:    ddosevent.FamilyGenericFlood,
	})

	stop := startStaleSweep(s, 5*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(5 * time.Second)
	for s.activeCount() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the sweep ticker never finalized the stale incident; is it wired?")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if s.list()[0].EndTime.IsZero() {
		t.Error("the swept incident must carry an end-time")
	}
}

// VALIDATES: the sweep worker stops on its stop function and leaves no goroutine
// running, so a config apply can replace the store safely.
// PREVENTS: a leaked ticker goroutine sweeping a store the plugin has dropped
// (ai/rules/goroutine-lifecycle.md).
//
// test-asserts-nothing: the failure mode is a HANG, not a wrong value. stop()
// returns only after the worker goroutine has exited, so a worker that never
// exits blocks here and the test dies on the package timeout. There is nothing
// to compare afterwards: a stopped ticker has no observable state, and reading
// runtime.NumGoroutine() would race every other test in the package.
func TestDdosObserveStaleSweepStops(t *testing.T) {
	s := newStore(10, time.Hour)
	stop := startStaleSweep(s, time.Millisecond)
	stop() // returns only after the worker has exited
}

// VALIDATES: subscribeStore's unsubscribe detaches the store it was built for, so
// a config apply that replaces the store cannot leave a handler writing into the
// old one.
// PREVENTS: a reconfigure keeping a dead ring alive and opening every incident
// twice. runEngine's apply calls teardown before it builds the new store, and
// nothing else can prove the detach happened.
func TestDdosObserveUnsubscribeDetachesStore(t *testing.T) {
	bus := newObserveTestBus()
	s := newStore(10, time.Hour)
	unsubscribe := subscribeStore(bus, s)

	detected := &ddosevent.AttackDetected{
		Interface: "xe0",
		Target:    ddosevent.VectorTuple{DstPrefix: netip.MustParsePrefix("192.0.2.9/32")},
		Family:    ddosevent.FamilyGenericFlood,
	}
	if _, err := ddosevent.Detected.Emit(bus, detected); err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	if _, err := ddosevent.Detected.Emit(bus, detected); err != nil {
		t.Fatal(err)
	}

	if got := s.count(); got != 1 {
		t.Errorf("count = %d, want 1: the store must stop receiving after unsubscribe", got)
	}
}
