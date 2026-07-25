package ddosevent

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/ze"
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
	if Characterized == nil {
		t.Fatal("Characterized event handle is nil")
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
	if events.PayloadType(Namespace, "attack-characterized") == nil {
		t.Error("attack-characterized not registered in type registry")
	}
	if events.PayloadType(Namespace, "attack-ongoing") == nil {
		t.Error("attack-ongoing not registered in type registry")
	}
	if events.PayloadType(Namespace, "attack-cleared") == nil {
		t.Error("attack-cleared not registered in type registry")
	}
}

// TestGradeSeverity pins the NetHawk 1x/2x/5x boundaries (AC-13): the grade steps
// exactly at the ratio thresholds, and a zero/negative threshold degrades to the
// medium floor instead of dividing by zero.
func TestGradeSeverity(t *testing.T) {
	const threshold = 1000.0
	cases := []struct {
		name   string
		peak   float64
		thresh float64
		want   Severity
	}{
		{"just-at-1x", 1000, threshold, SeverityMedium},
		{"below-2x", 1999, threshold, SeverityMedium},
		{"at-2x", 2000, threshold, SeverityHigh},
		{"below-5x", 4999, threshold, SeverityHigh},
		{"at-5x", 5000, threshold, SeverityCritical},
		{"well-above-5x", 50000, threshold, SeverityCritical},
		{"zero-threshold", 5000, 0, SeverityMedium},
		{"negative-threshold", 5000, -1, SeverityMedium},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := GradeSeverity(tc.peak, tc.thresh); got != tc.want {
				t.Errorf("GradeSeverity(%v, %v) = %q, want %q", tc.peak, tc.thresh, got, tc.want)
			}
		})
	}
}

// TestGradeConfidence pins the composite score bounds and monotonicity (AC-7/AC-8):
// a clear attack (high ratio + specific family + distributed) reaches 100, an
// ambiguous spike stays low, the result is always [0,100], and it never decreases
// as the peak-to-threshold ratio rises.
func TestGradeConfidence(t *testing.T) {
	// Clear attack: r=10, reflection, distributed -> 25+30+25+10+10 = 100.
	if got := GradeConfidence(50000, 5000, FamilyReflection, 5.0, 2.0); got != 100 {
		t.Errorf("clear-attack confidence = %d, want 100", got)
	}
	// Ambiguous: r=1, generic-flood, low entropy -> 25+6 = 31.
	if got := GradeConfidence(5000, 5000, FamilyGenericFlood, 0.5, 2.0); got != 31 {
		t.Errorf("ambiguous confidence = %d, want 31", got)
	}
	// Bounds: extreme ratio clamps at 100; zero threshold does not divide by zero.
	if got := GradeConfidence(1e9, 1, FamilyReflection, 100, 2.0); got != 100 {
		t.Errorf("clamped confidence = %d, want 100", got)
	}
	if got := GradeConfidence(0, 0, FamilyGenericFlood, 0, 2.0); got < 0 || got > 100 {
		t.Errorf("confidence %d out of [0,100]", got)
	}
	// entropy-threshold of 0 must not award the distributed bonus unconditionally.
	if got := GradeConfidence(5000, 5000, FamilyGenericFlood, 0, 0); got != 31 {
		t.Errorf("confidence with entropyThreshold=0 = %d, want 31 (no distributed bonus)", got)
	}
	// Monotonic non-decrease as the ratio rises.
	prev := -1
	for _, pps := range []float64{5000, 10000, 20000, 30000} {
		got := GradeConfidence(pps, 5000, FamilyUDPFlood, 0, 2.0)
		if got < prev {
			t.Errorf("confidence decreased with ratio: %d < %d", got, prev)
		}
		prev = got
	}
}

// TestEmitAndSubscribeCharacterized round-trips the Stage-2 event through the bus
// with a fully-populated vector, entropy, and severity.
func TestEmitAndSubscribeCharacterized(t *testing.T) {
	bus := newTestBus()
	var received *AttackCharacterized
	unsub := Characterized.Subscribe(bus, func(e *AttackCharacterized) {
		received = e
	})
	defer unsub()

	sent := &AttackCharacterized{
		Interface: "xe0",
		Target: VectorTuple{
			DstPrefix: netip.MustParsePrefix("10.0.0.1/32"),
			Proto:     6,
			TCPFlags:  0x02,
		},
		Family:        FamilySYNFlood,
		TopSources:    []netip.Addr{netip.MustParseAddr("198.51.100.7")},
		Severity:      SeverityCritical,
		SourceEntropy: 3.5,
		Observable:    true,
	}
	if _, err := Characterized.Emit(bus, sent); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if received == nil {
		t.Fatal("subscriber did not receive characterized event")
	}
	if received.Family != FamilySYNFlood {
		t.Errorf("Family: got %q, want %q", received.Family, FamilySYNFlood)
	}
	if received.Target.TCPFlags != 0x02 {
		t.Errorf("TCPFlags: got %#x, want 0x02", received.Target.TCPFlags)
	}
	if received.Severity != SeverityCritical {
		t.Errorf("Severity: got %q, want %q", received.Severity, SeverityCritical)
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
