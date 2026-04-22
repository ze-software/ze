package l2tp

import (
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

func TestBindL2TPMetrics_Registration(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	m := l2tpMetricsPtr.Load()
	if m == nil {
		t.Fatal("expected metrics to be bound")
	}
	defer l2tpMetricsPtr.Store(nil)

	if m.sessionsActive == nil {
		t.Error("sessionsActive not registered")
	}
	if m.tunnelsActive == nil {
		t.Error("tunnelsActive not registered")
	}
	if m.sessionState == nil {
		t.Error("sessionState not registered")
	}
	if m.lcpEchoRTTSecs == nil {
		t.Error("lcpEchoRTTSecs not registered")
	}
}

func TestBindL2TPMetrics_NilRegistry(t *testing.T) {
	old := l2tpMetricsPtr.Load()
	l2tpMetricsPtr.Store(nil)
	defer l2tpMetricsPtr.Store(old)

	bindL2TPMetrics(nil)
	if l2tpMetricsPtr.Load() != nil {
		t.Error("nil registry should not bind metrics")
	}
}

func TestObserveCQMBucket(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	bucket := CQMBucket{
		Start:     time.Now().Add(-100 * time.Second),
		State:     BucketStateEstablished,
		EchoCount: 10,
		MinRTT:    5 * time.Millisecond,
		MaxRTT:    50 * time.Millisecond,
		SumRTT:    200 * time.Millisecond,
	}
	observeCQMBucket("user1", bucket, 100)

	m := l2tpMetricsPtr.Load()
	m.bucketState.With("user1") // should not panic
}

func TestObserveCQMBucket_NilMetrics(t *testing.T) {
	old := l2tpMetricsPtr.Load()
	l2tpMetricsPtr.Store(nil)
	defer l2tpMetricsPtr.Store(old)

	observeCQMBucket("user1", CQMBucket{EchoCount: 1, SumRTT: time.Millisecond}, 100)
}

func TestDeleteLoginMetrics(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	m := l2tpMetricsPtr.Load()
	m.lcpEchoLossRatio.With("user1").Set(0.05)
	m.bucketState.With("user1").Set(0)

	deleteLoginMetrics("user1")
	// Verify deletion succeeded (no-op on missing is fine).
	ok := m.lcpEchoLossRatio.Delete("user1")
	if ok {
		t.Error("expected series already deleted")
	}
}

func TestStatsPoller_CounterDeltas(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	callCount := 0
	oldFn := getStatsFn
	getStatsFn = func(name string) (*iface.InterfaceStats, error) {
		callCount++
		return &iface.InterfaceStats{
			RxBytes:   uint64(callCount) * 1000,
			TxBytes:   uint64(callCount) * 2000,
			RxPackets: uint64(callCount) * 10,
			TxPackets: uint64(callCount) * 20,
		}, nil
	}
	defer func() { getStatsFn = oldFn }()

	ln := NewUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), nil)
	reactor := NewL2TPReactor(ln, nil, ReactorParams{MaxTunnels: 10, MaxSessions: 10})

	poller := newL2TPStatsPoller([]*L2TPReactor{reactor}, 30*time.Second)
	poller.poll()

	if callCount != 0 {
		t.Errorf("expected 0 calls with no sessions, got %d", callCount)
	}
}

func TestStatsPoller_SessionDisappears(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	poller := &l2tpStatsPoller{
		prev:     make(map[string]ifaceSnapshot),
		prevKeys: make(map[string][5]string),
	}
	poller.prevKeys["ppp0"] = [5]string{"1", "ppp0", "user", "10.0.0.1", ""}
	poller.prev["ppp0"] = ifaceSnapshot{rxBytes: 100}

	// Poll with no reactors => no sessions seen => ppp0 should be cleaned up.
	poller.reactors = nil
	poller.poll()

	if _, ok := poller.prevKeys["ppp0"]; ok {
		t.Error("expected ppp0 prevKeys to be cleaned up")
	}
	if _, ok := poller.prev["ppp0"]; ok {
		t.Error("expected ppp0 prev to be cleaned up")
	}
}

func TestDeleteSessionSeries(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	m := l2tpMetricsPtr.Load()
	labels := [5]string{"1", "ppp0", "user", "10.0.0.1", ""}
	m.sessionState.With(labels[0], labels[1], labels[2], labels[3], labels[4]).Set(5)
	m.sessionUptimeSecs.With(labels[0], labels[1], labels[2], labels[3], labels[4]).Set(100)

	deleteSessionSeries(m, labels)

	ok := m.sessionState.Delete(labels[0], labels[1], labels[2], labels[3], labels[4])
	if ok {
		t.Error("expected session_state series already deleted")
	}
}

func TestFormatSID(t *testing.T) {
	tests := []struct {
		sid  uint16
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{65535, "65535"},
	}
	for _, tc := range tests {
		got := formatSID(tc.sid)
		if got != tc.want {
			t.Errorf("formatSID(%d) = %q, want %q", tc.sid, got, tc.want)
		}
	}
}

func TestParsePollInterval_Default(t *testing.T) {
	d := parsePollInterval()
	if d != 30*time.Second {
		t.Errorf("expected 30s default, got %v", d)
	}
}
