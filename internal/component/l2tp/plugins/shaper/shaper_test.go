package l2tpshaper

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/traffic"
)

type mockBackend struct {
	mu      sync.Mutex
	applied map[string]traffic.InterfaceQoS
}

func (m *mockBackend) Apply(_ context.Context, desired map[string]traffic.InterfaceQoS) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	maps.Copy(m.applied, desired)
	return nil
}

func (m *mockBackend) ListQdiscs(ifaceName string) (traffic.InterfaceQoS, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applied[ifaceName], nil
}

func (m *mockBackend) Close() error { return nil }

func (m *mockBackend) getApplied(iface string) (traffic.InterfaceQoS, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.applied[iface]
	return v, ok
}

var (
	testBackendOnce    sync.Once
	testBackendInstPtr atomic.Pointer[mockBackend]
)

func setupMockBackend(t *testing.T) *mockBackend {
	t.Helper()

	mb := &mockBackend{applied: map[string]traffic.InterfaceQoS{}}
	testBackendInstPtr.Store(mb)
	testBackendOnce.Do(func() {
		if err := traffic.RegisterBackend("test-shaper", func() (traffic.Backend, error) {
			return testBackendInstPtr.Load(), nil
		}); err != nil {
			t.Fatal(err)
		}
	})
	if err := traffic.LoadBackend("test-shaper"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = traffic.CloseBackend()
	})
	return mb
}

// VALIDATES: AC-1 -- session-up with shaper config applies TC.
func TestShaperSessionUpAppliesTC(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscTBF,
		DefaultRate: 10_000_000,
	})

	s.onSessionUp(&l2tpevents.SessionUpPayload{
		TunnelID:  1,
		SessionID: 100,
		Interface: "ppp0",
	})

	qos, ok := mb.getApplied("ppp0")
	if !ok {
		t.Fatal("expected TC applied on ppp0")
	}
	if qos.Qdisc.Type != traffic.QdiscTBF {
		t.Errorf("qdisc type: got %v, want TBF", qos.Qdisc.Type)
	}
	if len(qos.Qdisc.Classes) != 1 || qos.Qdisc.Classes[0].Rate != 10_000_000 {
		t.Errorf("rate: got %v, want 10000000", qos.Qdisc.Classes)
	}

	// Check session stored.
	key := sessionKey{tunnelID: 1, sessionID: 100}
	val, ok := s.sessions.Load(key)
	if !ok {
		t.Fatal("session not stored in map")
	}
	st, ok := val.(sessionState)
	if !ok {
		t.Fatal("stored value is not sessionState")
	}
	if st.iface != "ppp0" {
		t.Errorf("interface: got %q, want %q", st.iface, "ppp0")
	}
}

// VALIDATES: AC-2 -- session-down removes from state.
func TestShaperSessionDownCleansUp(t *testing.T) {
	s := &shaperPlugin{}
	key := sessionKey{tunnelID: 1, sessionID: 100}
	s.sessions.Store(key, sessionState{iface: "ppp0", downloadRate: 10_000_000})

	s.onSessionDown(&l2tpevents.SessionDownPayload{
		TunnelID:  1,
		SessionID: 100,
	})

	if _, ok := s.sessions.Load(key); ok {
		t.Fatal("session should have been removed on session-down")
	}
}

// VALIDATES: AC-3 -- rate-change updates TC.
func TestShaperRateChange(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscTBF,
		DefaultRate: 10_000_000,
	})

	key := sessionKey{tunnelID: 1, sessionID: 100}
	s.sessions.Store(key, sessionState{
		iface:        "ppp0",
		downloadRate: 10_000_000,
		appliedAt:    time.Now(),
	})

	s.onSessionRateChange(&l2tpevents.SessionRateChangePayload{
		TunnelID:     1,
		SessionID:    100,
		DownloadRate: 50_000_000,
		UploadRate:   25_000_000,
	})

	qos, ok := mb.getApplied("ppp0")
	if !ok {
		t.Fatal("expected TC applied on ppp0")
	}
	if len(qos.Qdisc.Classes) != 1 || qos.Qdisc.Classes[0].Rate != 50_000_000 {
		t.Errorf("rate after change: got %v, want 50000000", qos.Qdisc.Classes)
	}

	// Check updated state.
	val, ok := s.sessions.Load(key)
	if !ok {
		t.Fatal("session should still be in map")
	}
	st, ok := val.(sessionState)
	if !ok {
		t.Fatal("stored value is not sessionState")
	}
	if st.downloadRate != 50_000_000 {
		t.Errorf("download rate: got %d, want 50000000", st.downloadRate)
	}
}

// VALIDATES: AC-10 -- no config means no TC applied.
func TestShaperNoConfig(t *testing.T) {
	s := &shaperPlugin{} // no config stored

	s.onSessionUp(&l2tpevents.SessionUpPayload{
		TunnelID:  1,
		SessionID: 100,
		Interface: "ppp0",
	})

	key := sessionKey{tunnelID: 1, sessionID: 100}
	if _, ok := s.sessions.Load(key); ok {
		t.Fatal("no config means session should not be stored")
	}
}

// VALIDATES: AC-8 -- TBF produces correct InterfaceQoS.
func TestBuildQoSForTBF(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscTBF,
		DefaultRate: 5_000_000,
	})

	s.onSessionUp(&l2tpevents.SessionUpPayload{
		TunnelID:  2,
		SessionID: 200,
		Interface: "ppp1",
	})

	qos, ok := mb.getApplied("ppp1")
	if !ok {
		t.Fatal("expected TC on ppp1")
	}
	if qos.Qdisc.Type != traffic.QdiscTBF {
		t.Errorf("qdisc: got %v, want TBF", qos.Qdisc.Type)
	}
}

// VALIDATES: AC-9 -- HTB produces correct InterfaceQoS with class.
func TestBuildQoSForHTB(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscHTB,
		DefaultRate: 100_000_000,
	})

	s.onSessionUp(&l2tpevents.SessionUpPayload{
		TunnelID:  3,
		SessionID: 300,
		Interface: "ppp2",
	})

	qos, ok := mb.getApplied("ppp2")
	if !ok {
		t.Fatal("expected TC on ppp2")
	}
	if qos.Qdisc.Type != traffic.QdiscHTB {
		t.Errorf("qdisc: got %v, want HTB", qos.Qdisc.Type)
	}
	if qos.Qdisc.DefaultClass != "default" {
		t.Errorf("default class: got %q, want %q", qos.Qdisc.DefaultClass, "default")
	}
	if len(qos.Qdisc.Classes) != 1 {
		t.Fatalf("class count: got %d, want 1", len(qos.Qdisc.Classes))
	}
	if qos.Qdisc.Classes[0].Ceil != 100_000_000 {
		t.Errorf("ceil: got %d, want 100000000", qos.Qdisc.Classes[0].Ceil)
	}
}
