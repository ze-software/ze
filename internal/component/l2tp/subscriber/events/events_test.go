// Design: plan/learned/760-subscriber-session-model.md -- subscriber events tests

package events

import (
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/pkg/ze"
)

type testBus struct {
	mu       sync.Mutex
	handlers map[string][]func(any)
}

var _ ze.EventBus = (*testBus)(nil)

func newTestBus() *testBus {
	return &testBus{handlers: make(map[string][]func(any))}
}

func (b *testBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	src := b.handlers[key]
	hs := make([]func(any), len(src))
	copy(hs, src)
	b.mu.Unlock()
	for _, h := range hs {
		h(payload)
	}
	return 0, nil
}

func (b *testBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.handlers[key] = append(b.handlers[key], handler)
	b.mu.Unlock()
	return func() {}
}

func TestSessionUpEmitSubscribe(t *testing.T) {
	bus := newTestBus()
	var received *SessionUpPayload

	SessionUp.Subscribe(bus, func(p *SessionUpPayload) {
		received = p
	})

	payload := &SessionUpPayload{
		Session: subscriber.Session{ID: "test-1", AccessType: subscriber.AccessPPPoE},
	}
	if _, err := SessionUp.Emit(bus, payload); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if received == nil {
		t.Fatal("subscriber did not receive event")
	}
	if received.Session.ID != "test-1" {
		t.Fatalf("session ID: got %q, want test-1", received.Session.ID)
	}
}

func TestSessionDownEmitSubscribe(t *testing.T) {
	bus := newTestBus()
	var received *SessionDownPayload

	SessionDown.Subscribe(bus, func(p *SessionDownPayload) {
		received = p
	})

	payload := &SessionDownPayload{
		Session: subscriber.Session{ID: "test-2", AccessType: subscriber.AccessL2TP},
		Reason:  "peer-disconnect",
	}
	if _, err := SessionDown.Emit(bus, payload); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if received == nil {
		t.Fatal("subscriber did not receive event")
	}
	if received.Reason != "peer-disconnect" {
		t.Fatalf("reason: got %q, want peer-disconnect", received.Reason)
	}
}

func TestSessionRateChangeEmitSubscribe(t *testing.T) {
	bus := newTestBus()
	var received *SessionRateChangePayload

	SessionRateChange.Subscribe(bus, func(p *SessionRateChangePayload) {
		received = p
	})

	payload := &SessionRateChangePayload{
		SessionID:    "test-3",
		DownloadRate: 100_000_000,
		UploadRate:   50_000_000,
	}
	if _, err := SessionRateChange.Emit(bus, payload); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if received == nil {
		t.Fatal("subscriber did not receive event")
	}
	if received.DownloadRate != 100_000_000 {
		t.Fatalf("download rate: got %d, want 100000000", received.DownloadRate)
	}
}
