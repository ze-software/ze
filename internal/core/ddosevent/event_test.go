package ddosevent

import (
	"net/netip"
	"sync"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

type testBus struct {
	mu   sync.Mutex
	subs map[string][]func(any)
}

func newTestBus() *testBus {
	return &testBus{subs: make(map[string][]func(any))}
}

func (b *testBus) Emit(namespace, eventType string, payload any) (int, error) {
	key := namespace + "/" + eventType
	b.mu.Lock()
	handlers := make([]func(any), len(b.subs[key]))
	copy(handlers, b.subs[key])
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return 0, nil
}

func (b *testBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.subs[key] = append(b.subs[key], handler)
	b.mu.Unlock()
	return func() {}
}

var _ ze.EventBus = (*testBus)(nil)

func TestEventHandlesRegistered(t *testing.T) {
	if Detected == nil {
		t.Fatal("Detected event handle is nil")
	}
	if Ongoing == nil {
		t.Fatal("Ongoing event handle is nil")
	}
	if Cleared == nil {
		t.Fatal("Cleared event handle is nil")
	}
}

func TestEventNamespace(t *testing.T) {
	if Detected.Namespace() != Namespace {
		t.Errorf("Detected namespace: got %q, want %q", Detected.Namespace(), Namespace)
	}
	if Ongoing.Namespace() != Namespace {
		t.Errorf("Ongoing namespace: got %q, want %q", Ongoing.Namespace(), Namespace)
	}
	if Cleared.Namespace() != Namespace {
		t.Errorf("Cleared namespace: got %q, want %q", Cleared.Namespace(), Namespace)
	}
}

func TestEventPayloadTypes(t *testing.T) {
	if events.PayloadType(Namespace, "attack-detected") == nil {
		t.Error("attack-detected not registered in type registry")
	}
	if events.PayloadType(Namespace, "attack-ongoing") == nil {
		t.Error("attack-ongoing not registered in type registry")
	}
	if events.PayloadType(Namespace, "attack-cleared") == nil {
		t.Error("attack-cleared not registered in type registry")
	}
}

func TestEmitAndSubscribeDetected(t *testing.T) {
	bus := newTestBus()
	var received *AttackDetected
	unsub := Detected.Subscribe(bus, func(e *AttackDetected) {
		received = e
	})
	defer unsub()

	sent := &AttackDetected{
		Interface: "xe0",
		Target: VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     17,
			DstPort:   53,
		},
		Family:     FamilyUDPFlood,
		PeakRxPps:  500000,
		PeakRxBps:  32000000,
		Observable: true,
	}
	if _, err := Detected.Emit(bus, sent); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if received == nil {
		t.Fatal("subscriber did not receive event")
	}
	if received.Interface != "xe0" {
		t.Errorf("Interface: got %q, want xe0", received.Interface)
	}
	if received.Family != FamilyUDPFlood {
		t.Errorf("Family: got %q, want %q", received.Family, FamilyUDPFlood)
	}
	if received.PeakRxPps != 500000 {
		t.Errorf("PeakRxPps: got %f, want 500000", received.PeakRxPps)
	}
}

func TestEmitNoSubscriber(t *testing.T) {
	// VALIDATES: AC-7 -- publishing with zero subscribers is safe
	bus := newTestBus()
	sent := &AttackDetected{
		Interface:  "xe0",
		Observable: true,
	}
	if _, err := Detected.Emit(bus, sent); err != nil {
		t.Fatalf("Emit with no subscriber: %v", err)
	}
}
