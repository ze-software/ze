// Design: plan/spec-cos-dynamic.md -- AccessInterface propagation test
// VALIDATES: AC-10 -- AccessInterface propagated from L2TP session-up to subscriber.Session

package l2tp

import (
	"log/slog"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/pkg/ze"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
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

func TestAccessInterfacePropagation(t *testing.T) {
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	logger := slog.Default()

	bridge := newSubscriberBridge(reg, bus, logger)
	defer bridge.stop()

	StoreSessionMetadata(1, 10, &AuthMetadata{
		FramedPool: "pool1",
	})
	defer ClearSessionMetadata(1, 10)

	var received *subevents.SessionUpPayload
	subevents.SessionUp.Subscribe(bus, func(p *subevents.SessionUpPayload) {
		received = p
	})

	if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		TunnelID:        1,
		SessionID:       10,
		Interface:       "ppp0",
		AccessInterface: "eth0.100",
	}); err != nil {
		t.Fatalf("emit session-up: %v", err)
	}

	if received == nil {
		t.Fatal("subscriber session-up event not received")
	}
	if received.Session.AccessInterface != "eth0.100" {
		t.Fatalf("AccessInterface: got %q, want %q", received.Session.AccessInterface, "eth0.100")
	}
	if received.Session.PppInterface != "ppp0" {
		t.Fatalf("PppInterface: got %q, want %q", received.Session.PppInterface, "ppp0")
	}

	sess, ok := reg.Get(l2tpSessionID(1, 10))
	if !ok {
		t.Fatal("session not found in registry")
	}
	if sess.AccessInterface != "eth0.100" {
		t.Fatalf("registry AccessInterface: got %q, want %q", sess.AccessInterface, "eth0.100")
	}
}

func TestAccessInterfacePropagationEmpty(t *testing.T) {
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	logger := slog.Default()

	bridge := newSubscriberBridge(reg, bus, logger)
	defer bridge.stop()

	var received *subevents.SessionUpPayload
	subevents.SessionUp.Subscribe(bus, func(p *subevents.SessionUpPayload) {
		received = p
	})

	if _, err := l2tpevents.SessionUp.Emit(bus, &l2tpevents.SessionUpPayload{
		TunnelID:  2,
		SessionID: 20,
		Interface: "ppp1",
	}); err != nil {
		t.Fatalf("emit session-up: %v", err)
	}

	if received == nil {
		t.Fatal("subscriber session-up event not received")
	}
	if received.Session.AccessInterface != "" {
		t.Fatalf("AccessInterface: got %q, want empty (pure LNS)", received.Session.AccessInterface)
	}
}
