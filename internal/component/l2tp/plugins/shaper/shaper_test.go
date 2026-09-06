package l2tpshaper

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/traffic"
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

// TestSessionUpEnforcesUploadRate checks that the configured upload-rate
// reaches an interface. Before this the leaf was stored and reported while no
// queueing discipline or policer carried it.
func TestSessionUpEnforcesUploadRate(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{
		QdiscType:   traffic.QdiscTBF,
		DefaultRate: 10_000_000,
		UploadRate:  2_000_000,
	})

	s.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: 4, SessionID: 400, Interface: "ppp4"})

	qos, ok := mb.getApplied("ppp4")
	if !ok {
		t.Fatal("expected TC applied on ppp4")
	}
	if !qos.Ingress.Set() {
		t.Fatal("no ingress policer: the upload rate reaches no interface")
	}
	if qos.Ingress.RateBps != 2_000_000 {
		t.Fatalf("ingress rate = %d, want 2000000", qos.Ingress.RateBps)
	}
	if qos.Ingress.BurstBytes != traffic.PolicerBurstBytes(2_000_000) {
		t.Fatalf("ingress burst = %d, want %d", qos.Ingress.BurstBytes, traffic.PolicerBurstBytes(2_000_000))
	}
}

// TestSessionUpFallsBackToDefaultRateForUpload checks the leaf's documented
// default: an absent upload-rate means the download rate is used in both
// directions, so a session is never left with one direction unenforced.
func TestSessionUpFallsBackToDefaultRateForUpload(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 7_000_000})

	s.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: 5, SessionID: 500, Interface: "ppp5"})

	qos, ok := mb.getApplied("ppp5")
	if !ok {
		t.Fatal("expected TC applied on ppp5")
	}
	if qos.Ingress.RateBps != 7_000_000 {
		t.Fatalf("ingress rate = %d, want the default-rate 7000000", qos.Ingress.RateBps)
	}
}

// TestSessionUpEnforcesRadiusUploadHalf checks the RADIUS answer end to end. A
// Filter-Id of the form rate:<down>/<up> carries two rates, and the upload half
// must reach the interface rather than only the log and the show command.
func TestSessionUpEnforcesRadiusUploadHalf(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscHTB, DefaultRate: 10_000_000})

	l2tp.StoreSessionMetadata(6, 600, &l2tp.AuthMetadata{FilterID: "rate:20mbit/5mbit"})
	t.Cleanup(func() { l2tp.ClearSessionMetadata(6, 600) })

	s.onSessionUp(&l2tpevents.SessionUpPayload{TunnelID: 6, SessionID: 600, Interface: "ppp6"})

	qos, ok := mb.getApplied("ppp6")
	if !ok {
		t.Fatal("expected TC applied on ppp6")
	}
	if qos.Qdisc.Classes[0].Rate != 20_000_000 {
		t.Fatalf("download rate = %d, want 20000000", qos.Qdisc.Classes[0].Rate)
	}
	if qos.Ingress.RateBps != 5_000_000 {
		t.Fatalf("upload rate = %d, want the RADIUS Filter-Id upload half 5000000", qos.Ingress.RateBps)
	}
}

// TestRateChangeUpdatesUploadRate checks the mid-session path. A CoA that
// changes the rate must reprogram both directions: an install-once policer
// leaves the operator's authorization change accepted and unapplied.
func TestRateChangeUpdatesUploadRate(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000})

	key := sessionKey{tunnelID: 7, sessionID: 700}
	s.sessions.Store(key, sessionState{
		iface:        "ppp7",
		downloadRate: 10_000_000,
		uploadRate:   1_000_000,
		appliedAt:    time.Now(),
	})

	s.onSessionRateChange(&l2tpevents.SessionRateChangePayload{
		TunnelID:     7,
		SessionID:    700,
		DownloadRate: 50_000_000,
		UploadRate:   25_000_000,
	})

	qos, ok := mb.getApplied("ppp7")
	if !ok {
		t.Fatal("expected TC applied on ppp7")
	}
	if qos.Ingress.RateBps != 25_000_000 {
		t.Fatalf("ingress rate after change = %d, want 25000000", qos.Ingress.RateBps)
	}
	val, _ := s.sessions.Load(key)
	st, _ := val.(sessionState)
	if st.uploadRate != 25_000_000 {
		t.Fatalf("stored upload rate = %d, want 25000000", st.uploadRate)
	}
}

// TestRateChangeKeepsUploadRateWhenPayloadOmitsIt checks the download-only CoA.
// A payload carrying no upload rate must not silently drop the subscriber's
// existing upload limit to zero, which would leave that direction unenforced.
func TestRateChangeKeepsUploadRateWhenPayloadOmitsIt(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000})

	key := sessionKey{tunnelID: 8, sessionID: 800}
	s.sessions.Store(key, sessionState{
		iface:        "ppp8",
		downloadRate: 10_000_000,
		uploadRate:   3_000_000,
		appliedAt:    time.Now(),
	})

	s.onSessionRateChange(&l2tpevents.SessionRateChangePayload{
		TunnelID:     8,
		SessionID:    800,
		DownloadRate: 20_000_000,
	})

	qos, _ := mb.getApplied("ppp8")
	if qos.Ingress.RateBps != 3_000_000 {
		t.Fatalf("ingress rate = %d, want the session's existing 3000000", qos.Ingress.RateBps)
	}
}

// TestSubscriberSessionUpEnforcesUploadRate checks the PPPoE entry point. Both
// access types terminate on a pppN, so one mechanism covers both, and the
// handler must stop discarding its upload argument.
func TestSubscriberSessionUpEnforcesUploadRate(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000, UploadRate: 4_000_000})

	s.handleSubscriberSessionUp("ppp9", 30_000_000, 6_000_000)

	qos, ok := mb.getApplied("ppp9")
	if !ok {
		t.Fatal("expected TC applied on ppp9")
	}
	if qos.Qdisc.Classes[0].Rate != 30_000_000 {
		t.Fatalf("download rate = %d, want 30000000", qos.Qdisc.Classes[0].Rate)
	}
	if qos.Ingress.RateBps != 6_000_000 {
		t.Fatalf("upload rate = %d, want the argument 6000000", qos.Ingress.RateBps)
	}
}

// TestSubscriberSessionUpFallsBackToConfiguredUploadRate checks the PPPoE
// default. The session carries no per-subscriber upload rate today, so the
// configured leaf is what the interface must get.
func TestSubscriberSessionUpFallsBackToConfiguredUploadRate(t *testing.T) {
	mb := setupMockBackend(t)

	s := &shaperPlugin{}
	s.cfgPtr.Store(&shaperConfig{QdiscType: traffic.QdiscTBF, DefaultRate: 10_000_000, UploadRate: 4_000_000})

	s.handleSubscriberSessionUp("ppp10", 0, 0)

	qos, _ := mb.getApplied("ppp10")
	if qos.Qdisc.Classes[0].Rate != 10_000_000 {
		t.Fatalf("download rate = %d, want the configured default 10000000", qos.Qdisc.Classes[0].Rate)
	}
	if qos.Ingress.RateBps != 4_000_000 {
		t.Fatalf("upload rate = %d, want the configured upload-rate 4000000", qos.Ingress.RateBps)
	}
}
