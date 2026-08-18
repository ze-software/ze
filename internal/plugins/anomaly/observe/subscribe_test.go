package observe

import (
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/pkg/ze"
)

// observeTestBus is a synchronous in-process ze.EventBus: Emit delivers to the
// matching subscribers before it returns, so a test needs no polling.
//
// Unlike the detect package's chainTestBus, the unsubscribe it returns really
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

// VALIDATES: AC-4 -- a Detected event published on the bus reaches store.open
// through the subscription, not through a direct method call.
// PREVENTS: a store that works in isolation while the plugin never subscribes it,
// which is the failure a store-only test cannot see.
func TestObserveSubscribeOpensIncident(t *testing.T) {
	bus := newObserveTestBus()
	s := newStore(10, time.Hour)
	unsubscribe := subscribeStore(bus, s)
	defer unsubscribe()

	entity := netip.MustParsePrefix("10.0.0.9/32")
	confirmed := time.Now().Add(-time.Minute)
	if _, err := anomalyevent.Detected.Emit(bus, &anomalyevent.AnomalyDetected{
		Entity: entity,
		Cohort: "10.0.0.0/24",
		Score:  8,
		At:     confirmed,
	}); err != nil {
		t.Fatal(err)
	}

	list := s.list()
	if len(list) != 1 {
		t.Fatalf("got %d incidents, want 1 opened by the Detected subscription", len(list))
	}
	if list[0].Entity != entity {
		t.Errorf("entity = %s, want %s", list[0].Entity, entity)
	}
	if !list[0].StartTime.Equal(confirmed) {
		t.Errorf("start-time = %s, want the event's At %s", list[0].StartTime, confirmed)
	}
	if !list[0].Active {
		t.Error("the opened incident must be active")
	}
}

// VALIDATES: AC-5 -- a Cleared event for the same entity reaches store.finalize
// through the subscription.
// PREVENTS: incidents leaking as permanently active because only the open half of
// the lifecycle is wired.
func TestObserveSubscribeFinalizesIncident(t *testing.T) {
	bus := newObserveTestBus()
	s := newStore(10, time.Hour)
	unsubscribe := subscribeStore(bus, s)
	defer unsubscribe()

	entity := netip.MustParsePrefix("10.0.0.9/32")
	if _, err := anomalyevent.Detected.Emit(bus, &anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := anomalyevent.Cleared.Emit(bus, &anomalyevent.AnomalyCleared{Entity: entity}); err != nil {
		t.Fatal(err)
	}

	if got := s.activeCount(); got != 0 {
		t.Errorf("activeCount = %d, want 0 after the Cleared subscription finalized it", got)
	}
	inc := s.list()[0]
	if inc.Active {
		t.Error("the incident must be finalized by the Cleared event")
	}
	if inc.EndTime.IsZero() {
		t.Error("the Cleared event must set end-time")
	}
}

// VALIDATES: an Ongoing event neither opens a second incident nor finalizes the
// open one; it is subscribed so the store sees the whole contract, and it changes
// nothing.
// PREVENTS: one incident being reported once per tick while it lasts, which would
// fill the ring with duplicates of the live incident.
func TestObserveSubscribeIgnoresOngoing(t *testing.T) {
	bus := newObserveTestBus()
	s := newStore(10, time.Hour)
	unsubscribe := subscribeStore(bus, s)
	defer unsubscribe()

	entity := netip.MustParsePrefix("10.0.0.9/32")
	if _, err := anomalyevent.Detected.Emit(bus, &anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := anomalyevent.Ongoing.Emit(bus, &anomalyevent.AnomalyOngoing{Entity: entity, Score: 9}); err != nil {
			t.Fatal(err)
		}
	}

	if got := s.count(); got != 1 {
		t.Errorf("count = %d, want 1: Ongoing must not open a new incident", got)
	}
	if got := s.activeCount(); got != 1 {
		t.Errorf("activeCount = %d, want 1: Ongoing must not finalize the incident", got)
	}
}

// VALIDATES: the unsubscribe returned by subscribeStore detaches every handler, so
// a store replaced by a reconfigure stops receiving events.
// PREVENTS: a reconfigure leaving the old store subscribed, which would keep a dead
// ring alive and double every incident's bookkeeping.
func TestObserveUnsubscribeDetachesStore(t *testing.T) {
	bus := newObserveTestBus()
	s := newStore(10, time.Hour)
	unsubscribe := subscribeStore(bus, s)

	entity := netip.MustParsePrefix("10.0.0.9/32")
	if _, err := anomalyevent.Detected.Emit(bus, &anomalyevent.AnomalyDetected{Entity: entity, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	if _, err := anomalyevent.Detected.Emit(bus, &anomalyevent.AnomalyDetected{
		Entity: netip.MustParsePrefix("10.0.0.10/32"),
		At:     time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	if got := s.count(); got != 1 {
		t.Errorf("count = %d, want 1: the store must stop receiving after unsubscribe", got)
	}
}

// VALIDATES: AC-3 wiring -- the sweep worker calls store.sweepStale on its ticker,
// so stale-incident-timeout is a live control and not a dead config leaf.
// PREVENTS: the defect this child fixes in the ddos template it copies, where
// sweepStale exists and nothing in production ever calls it.
func TestObserveStaleSweepTickerFinalizes(t *testing.T) {
	s := newStore(10, 10*time.Millisecond)
	s.open(&anomalyevent.AnomalyDetected{Entity: netip.MustParsePrefix("10.0.0.9/32"), At: time.Now()})

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
// running, so a reconfigure can replace the store safely.
// PREVENTS: a leaked ticker goroutine sweeping a store the plugin has dropped
// (ai/rules/goroutine-lifecycle.md).
//
// test-asserts-nothing: the failure mode is a HANG, not a wrong value. stop()
// returns only after the worker goroutine has exited, so a worker that never
// exits blocks here and the test dies on the package timeout. There is nothing
// to compare afterwards: a stopped ticker has no observable state, and reading
// runtime.NumGoroutine() would race every other test in the package.
func TestObserveStaleSweepStops(t *testing.T) {
	s := newStore(10, time.Hour)
	stop := startStaleSweep(s, time.Millisecond)
	stop() // returns only after the worker has exited
}
