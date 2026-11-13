// Design: docs/architecture/isis/isis-3-l2-transport.md -- a raw open belongs to its enabling configuration.
// Related: transport.go -- HandleLinkUp, DisableInterface and Close own circuit admission.
package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestISISTransportOpenRetainsEnablementOwner proves that disabling or replacing
// an interface, or closing the transport, revokes an already admitted raw open.
// Method: hold the backend at a channel barrier across the transition, then
// inspect actual circuits, handle closes and callbacks. The replacement case
// also opens a fresh circuit to prove the new configuration remains usable.
func TestISISTransportOpenRetainsEnablementOwner(t *testing.T) {
	for _, transition := range []string{"disable", "replace at same level", "close"} {
		t.Run(transition, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			backend := newHeldOpenBackend()
			tr := New(backend)
			var down atomic.Int32
			tr.OnCircuitDown(func(int, string, CircuitID) { down.Add(1) })
			tr.EnableInterface("owner-a", Level1)
			opened := make(chan error, 1)
			openFinished := make(chan struct{})
			go func() {
				defer close(openFinished)
				opened <- tr.HandleLinkUp("owner-a")
			}()
			t.Cleanup(func() {
				backend.release()
				select {
				case <-openFinished:
				case <-time.After(30 * time.Second):
					t.Error("raw open did not stop during cleanup")
					return
				}
				tr.Close()
			})
			awaitOpenOwnershipSignal(t, ctx, backend.started)

			var closed chan struct{}
			if transition == "close" {
				closed = make(chan struct{})
				go func() {
					tr.Close()
					close(closed)
				}()
				awaitOpenOwnershipClosed(t, ctx, tr)
			} else {
				tr.DisableInterface("owner-a")
				if transition == "replace at same level" {
					tr.EnableInterface("owner-a", Level1)
				}
			}
			backend.release()
			select {
			case err := <-opened:
				if err != nil {
					t.Fatalf("retired raw open: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("raw open did not leave its backend barrier")
			}
			if closed != nil {
				awaitOpenOwnershipSignal(t, ctx, closed)
			}
			if count := tr.OpenCircuitCount(); count != 0 {
				t.Errorf("retired raw open published %d circuits", count)
			}
			first := backend.handle(0)
			if closes := first.closes.Load(); closes != 1 {
				t.Errorf("retired handle closed %d times, want once", closes)
			}
			if receives := first.receives.Load(); receives != 0 {
				t.Errorf("retired handle started %d receive loops", receives)
			}
			if callbacks := down.Load(); callbacks != 0 {
				t.Errorf("unpublished handle emitted %d circuit-down callbacks", callbacks)
			}

			if transition == "replace at same level" {
				if err := tr.HandleLinkUp("owner-a"); err != nil {
					t.Fatalf("replacement raw open: %v", err)
				}
				if count := backend.count(); count != 2 {
					t.Fatalf("replacement made %d raw opens, want the retired and fresh handles", count)
				}
				if count := tr.OpenCircuitCount(); count != 1 {
					t.Errorf("replacement published %d circuits, want one", count)
				}
				if closes := backend.handle(1).closes.Load(); closes != 0 {
					t.Errorf("replacement handle closed %d times before shutdown", closes)
				}
				tr.Close()
				if closes := backend.handle(1).closes.Load(); closes != 1 {
					t.Errorf("replacement handle closed %d times after shutdown, want once", closes)
				}
				if callbacks := down.Load(); callbacks != 1 {
					t.Errorf("replacement shutdown emitted %d callbacks, want one", callbacks)
				}
			}
			if transition == "close" {
				tr.EnableInterface("owner-a", Level1)
				if err := tr.HandleLinkUp("owner-a"); err != nil {
					t.Fatalf("link event after shutdown: %v", err)
				}
				if count := backend.count(); count != 1 {
					t.Errorf("closed transport admitted %d raw opens, want only the retired one", count)
				}
			}
		})
	}
}

// heldOpenBackend admits one raw open at a barrier. Its owner MUST release the
// barrier and join HandleLinkUp before completing the test's transport cleanup.
type heldOpenBackend struct {
	mu      sync.Mutex
	handles []*heldOpenCircuit
	started chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func newHeldOpenBackend() *heldOpenBackend {
	return &heldOpenBackend{started: make(chan struct{}), resume: make(chan struct{})}
}

func (b *heldOpenBackend) OpenCircuit(name string) (CircuitHandle, error) {
	b.mu.Lock()
	first := len(b.handles) == 0
	c := &heldOpenCircuit{fakeCircuit: newFakeCircuit(name, len(b.handles)+1, 1500)}
	b.handles = append(b.handles, c)
	b.mu.Unlock()
	if first {
		close(b.started)
		<-b.resume
	}
	return c, nil
}

// release MUST run before joining the admitted HandleLinkUp call.
func (b *heldOpenBackend) release() { b.once.Do(func() { close(b.resume) }) }

func (b *heldOpenBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.handles)
}

func (b *heldOpenBackend) handle(index int) *heldOpenCircuit {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.handles[index]
}

type heldOpenCircuit struct {
	*fakeCircuit
	closes   atomic.Int32
	receives atomic.Int32
}

func (c *heldOpenCircuit) Close() error {
	c.closes.Add(1)
	return c.fakeCircuit.Close()
}

func (c *heldOpenCircuit) Recv() <-chan RawFrame {
	c.receives.Add(1)
	return c.fakeCircuit.Recv()
}

func awaitOpenOwnershipSignal(t *testing.T, ctx context.Context, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-ctx.Done():
		t.Fatal("raw-open ownership barrier did not advance")
	}
}

// awaitOpenOwnershipClosed observes Close's real admission revocation before
// releasing the held I/O. The ticker only paces the state check.
func awaitOpenOwnershipClosed(t *testing.T, ctx context.Context, tr *Transport) {
	t.Helper()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		tr.mu.Lock()
		closed := tr.enabled == nil
		tr.mu.Unlock()
		if closed {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("transport did not revoke admission during Close")
		case <-tick.C:
		}
	}
}

// TestISISTransportSubscriptionRetainsEnablementOwner proves that an interface
// subscription admitted before disable or Close cannot publish afterward.
// Method: hold the resolver subscription at a channel barrier and count its
// cancellation and published owner. After disable, a fresh subscription remains
// usable and is canceled exactly once by ordinary shutdown.
func TestISISTransportSubscriptionRetainsEnablementOwner(t *testing.T) {
	for _, transition := range []string{"disable", "close"} {
		t.Run(transition, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			tr := New(newFakeBackend())
			first := newHeldIfaceSubscription()
			second := newHeldIfaceSubscription()
			started, resume := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			var release sync.Once
			tr.subscribe = func(string) (<-chan iface.LinkEvent, func()) {
				if calls.Add(1) == 1 {
					close(started)
					<-resume
					return first.events, first.cancel
				}
				return second.events, second.cancel
			}
			tr.EnableInterface("owner-a", Level1)
			wired := make(chan struct{})
			go func() {
				defer close(wired)
				tr.subscribeIfaceEventsWithRescan(newFakeBus(), 0)
			}()
			t.Cleanup(func() {
				release.Do(func() { close(resume) })
				// On a failing implementation, a subscription may have escaped
				// the shutdown census. Release its worker before joining Close.
				first.finish()
				second.finish()
				select {
				case <-wired:
				case <-time.After(30 * time.Second):
					t.Error("subscription setup did not stop during cleanup")
					return
				}
				tr.Close()
			})
			awaitOpenOwnershipSignal(t, ctx, started)
			var closed chan struct{}
			if transition == "close" {
				closed = make(chan struct{})
				go func() {
					tr.Close()
					close(closed)
				}()
				awaitOpenOwnershipClosed(t, ctx, tr)
			} else {
				tr.DisableInterface("owner-a")
			}
			release.Do(func() { close(resume) })
			awaitOpenOwnershipSignal(t, ctx, wired)
			if cancels := first.cancels.Load(); cancels != 1 {
				t.Errorf("retired subscription canceled %d times, want once", cancels)
			}
			tr.mu.Lock()
			published := len(tr.ifaceSubs)
			tr.mu.Unlock()
			if published != 0 {
				t.Errorf("retired subscription published %d owners", published)
			}
			if transition == "disable" {
				tr.EnableInterface("owner-a", Level1)
				tr.mu.Lock()
				published = len(tr.ifaceSubs)
				tr.mu.Unlock()
				if published != 1 {
					t.Errorf("fresh configuration published %d subscriptions, want one", published)
				}
				if cancels := second.cancels.Load(); cancels != 0 {
					t.Errorf("fresh subscription canceled %d times before shutdown", cancels)
				}
				tr.Close()
				if cancels := second.cancels.Load(); cancels != 1 {
					t.Errorf("fresh subscription canceled %d times after shutdown, want once", cancels)
				}
			}
			if closed != nil {
				// The assertions above decide the behavior before cleanup also
				// releases a stale subscription's worker on the old code.
				first.finish()
				awaitOpenOwnershipSignal(t, ctx, closed)
			}
		})
	}
}

type heldIfaceSubscription struct {
	events  chan iface.LinkEvent
	cancels atomic.Int32
	once    sync.Once
}

func newHeldIfaceSubscription() *heldIfaceSubscription {
	return &heldIfaceSubscription{events: make(chan iface.LinkEvent)}
}

func (s *heldIfaceSubscription) cancel() {
	s.cancels.Add(1)
	s.finish()
}

func (s *heldIfaceSubscription) finish() {
	s.once.Do(func() { close(s.events) })
}

// TestISISTransportOldMonitorCleanupKeepsReplacement proves that a prior
// monitoring owner's cleanup cannot cancel a replacement's subscriptions.
// Method: replace the monitor, run the old cleanup, then enable another interface
// and check both live subscriptions survive until the new owner's shutdown.
func TestISISTransportOldMonitorCleanupKeepsReplacement(t *testing.T) {
	tr := New(newFakeBackend())
	var subscriptions []*heldIfaceSubscription
	tr.subscribe = func(string) (<-chan iface.LinkEvent, func()) {
		s := newHeldIfaceSubscription()
		subscriptions = append(subscriptions, s)
		return s.events, s.cancel
	}
	t.Cleanup(tr.Close)
	tr.EnableInterface("owner-a", Level1)
	oldStop := tr.subscribeIfaceEventsWithRescan(newFakeBus(), 0)
	newStop := tr.subscribeIfaceEventsWithRescan(newFakeBus(), 0)
	oldStop()
	tr.EnableInterface("owner-b", Level1)
	tr.mu.Lock()
	published := len(tr.ifaceSubs)
	tr.mu.Unlock()
	if published != 2 {
		t.Errorf("replacement monitor retained %d subscriptions after old cleanup, want two", published)
	}
	if len(subscriptions) != 3 {
		t.Fatalf("replacement made %d subscriptions, want one retired and two current", len(subscriptions))
	}
	if cancels := subscriptions[0].cancels.Load(); cancels != 1 {
		t.Errorf("retired monitor subscription canceled %d times, want once", cancels)
	}
	for _, current := range subscriptions[1:] {
		if cancels := current.cancels.Load(); cancels != 0 {
			t.Errorf("current subscription canceled %d times before shutdown", cancels)
		}
	}
	newStop()
	tr.Close()
	for _, s := range subscriptions {
		if cancels := s.cancels.Load(); cancels != 1 {
			t.Errorf("subscription canceled %d times after shutdown, want once", cancels)
		}
	}
}
