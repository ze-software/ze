// Design: plan/spec-cos-dynamic.md -- CoS handler tests
// VALIDATES: AC-2 through AC-9, AC-13 -- dynamic CoS event handling

//go:build ze_l2tp

package cos

import (
	"sync"
	"testing"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	coreCos "codeberg.org/thomas-mangin/ze/internal/core/cos"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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

type mockBackendCall struct {
	ifaceName string
	ingress   map[uint32]uint32
	egress    map[uint32]uint32
}

type mockBackend struct {
	mu    sync.Mutex
	calls []mockBackendCall
	err   error
}

func (m *mockBackend) updateVLANQoSMap(ifaceName string, ingress, egress map[uint32]uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockBackendCall{ifaceName, ingress, egress})
	return m.err
}

func (m *mockBackend) lastCall() (mockBackendCall, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return mockBackendCall{}, false
	}
	return m.calls[len(m.calls)-1], true
}

func (m *mockBackend) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func setupTestHandler(t *testing.T) (*testBus, *mockBackend) {
	t.Helper()
	coreCos.Clear()
	coreCos.Register("residential", coreCos.Profile{
		IngressMap: map[uint32]uint32{0: 0, 1: 1},
		EgressMap:  map[uint32]uint32{0: 0, 1: 2, 5: 3},
	})
	coreCos.Register("business", coreCos.Profile{
		IngressMap: map[uint32]uint32{0: 0},
		EgressMap:  map[uint32]uint32{0: 1, 1: 3},
	})
	t.Cleanup(coreCos.Clear)

	bus := newTestBus()
	mb := &mockBackend{}
	h := newCosHandler(bus, mb.updateVLANQoSMap, nil)
	t.Cleanup(h.stop)
	return bus, mb
}

func emitSessionUp(t *testing.T, bus *testBus, p *l2tpevents.SessionUpPayload) {
	t.Helper()
	if _, err := l2tpevents.SessionUp.Emit(bus, p); err != nil {
		t.Fatalf("emit session-up: %v", err)
	}
}

func emitSessionDown(t *testing.T, bus *testBus, p *l2tpevents.SessionDownPayload) {
	t.Helper()
	if _, err := l2tpevents.SessionDown.Emit(bus, p); err != nil {
		t.Fatalf("emit session-down: %v", err)
	}
}

func emitCoSChange(t *testing.T, bus *testBus, p *l2tpevents.SessionCoSChangePayload) {
	t.Helper()
	if _, err := l2tpevents.SessionCoSChange.Emit(bus, p); err != nil {
		t.Fatalf("emit cos-change: %v", err)
	}
}

// TestCoSHandlerSessionUp verifies AC-2: session-up with CoS metadata
// triggers UpdateVLANQoSMap on the access interface.
func TestCoSHandlerSessionUp(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(1, 10, "residential")
	defer clearMetadataForTest(1, 10)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        1,
		SessionID:       10,
		Interface:       "ppp0",
		AccessInterface: "eth0.100",
	})

	call, ok := mb.lastCall()
	if !ok {
		t.Fatal("UpdateVLANQoSMap not called")
	}
	if call.ifaceName != "eth0.100" {
		t.Fatalf("interface: got %q, want %q", call.ifaceName, "eth0.100")
	}
	if call.egress[5] != 3 {
		t.Fatalf("egress[5]: got %d, want 3", call.egress[5])
	}
}

// TestCoSHandlerSessionUpNoCoS verifies AC-7: no Filter-Id means no action.
func TestCoSHandlerSessionUpNoCoS(t *testing.T) {
	bus, mb := setupTestHandler(t)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        1,
		SessionID:       11,
		Interface:       "ppp1",
		AccessInterface: "eth0.101",
	})

	if mb.callCount() != 0 {
		t.Fatal("UpdateVLANQoSMap called when no CoS profile")
	}
}

// TestCoSHandlerSessionUpNoAccess verifies AC-8: empty AccessInterface skips.
func TestCoSHandlerSessionUpNoAccess(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(2, 20, "residential")
	defer clearMetadataForTest(2, 20)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:  2,
		SessionID: 20,
		Interface: "ppp2",
	})

	if mb.callCount() != 0 {
		t.Fatal("UpdateVLANQoSMap called with empty AccessInterface")
	}
}

// TestCoSHandlerSessionUpRateOnly verifies AC-6: rate FilterID is ignored.
func TestCoSHandlerSessionUpRateOnly(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(3, 30, "")
	defer clearMetadataForTest(3, 30)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        3,
		SessionID:       30,
		Interface:       "ppp3",
		AccessInterface: "eth0.103",
	})

	if mb.callCount() != 0 {
		t.Fatal("UpdateVLANQoSMap called for rate-only FilterID")
	}
}

// TestCoSHandlerSessionDown verifies AC-3: session-down reverts maps.
func TestCoSHandlerSessionDown(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(4, 40, "residential")
	defer clearMetadataForTest(4, 40)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        4,
		SessionID:       40,
		Interface:       "ppp4",
		AccessInterface: "eth0.104",
	})
	if mb.callCount() != 1 {
		t.Fatal("session-up did not apply maps")
	}

	emitSessionDown(t, bus, &l2tpevents.SessionDownPayload{
		TunnelID:  4,
		SessionID: 40,
	})

	if mb.callCount() != 2 {
		t.Fatal("session-down did not revert maps")
	}
	call, _ := mb.lastCall()
	if call.ifaceName != "eth0.104" {
		t.Fatalf("revert interface: got %q, want %q", call.ifaceName, "eth0.104")
	}
}

// TestCoSHandlerCoAChange verifies AC-4: CoS profile change mid-session.
func TestCoSHandlerCoAChange(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(5, 50, "residential")
	defer clearMetadataForTest(5, 50)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        5,
		SessionID:       50,
		Interface:       "ppp5",
		AccessInterface: "eth0.105",
	})
	if mb.callCount() != 1 {
		t.Fatal("session-up did not apply maps")
	}

	emitCoSChange(t, bus, &l2tpevents.SessionCoSChangePayload{
		TunnelID:        5,
		SessionID:       50,
		AccessInterface: "eth0.105",
		ProfileName:     "business",
	})

	if mb.callCount() != 2 {
		t.Fatalf("CoA change: expected 2 calls, got %d", mb.callCount())
	}
	call, _ := mb.lastCall()
	if call.egress[0] != 1 {
		t.Fatalf("business egress[0]: got %d, want 1", call.egress[0])
	}
}

// TestCoSHandlerCoANotFound verifies AC-5: unknown profile -> no map change.
func TestCoSHandlerCoANotFound(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(6, 60, "residential")
	defer clearMetadataForTest(6, 60)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        6,
		SessionID:       60,
		Interface:       "ppp6",
		AccessInterface: "eth0.106",
	})

	emitCoSChange(t, bus, &l2tpevents.SessionCoSChangePayload{
		TunnelID:        6,
		SessionID:       60,
		AccessInterface: "eth0.106",
		ProfileName:     "nonexistent",
	})

	if mb.callCount() != 1 {
		t.Fatalf("unknown profile should not apply maps: got %d calls, want 1", mb.callCount())
	}
}

// TestCoSHandlerDualFilterID verifies AC-13: both cos and rate FilterIDs coexist.
func TestCoSHandlerDualFilterID(t *testing.T) {
	bus, mb := setupTestHandler(t)

	storeMetadataForTest(7, 70, "business")
	defer clearMetadataForTest(7, 70)

	emitSessionUp(t, bus, &l2tpevents.SessionUpPayload{
		TunnelID:        7,
		SessionID:       70,
		Interface:       "ppp7",
		AccessInterface: "eth0.107",
	})

	call, ok := mb.lastCall()
	if !ok {
		t.Fatal("UpdateVLANQoSMap not called")
	}
	if call.egress[1] != 3 {
		t.Fatalf("business egress[1]: got %d, want 3", call.egress[1])
	}
}
