package l2tp

import (
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
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
	m := bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	bucket := CQMBucket{
		Start:     time.Now().Add(-100 * time.Second),
		State:     BucketStateEstablished,
		EchoCount: 10,
		MinRTT:    5 * time.Millisecond,
		MaxRTT:    50 * time.Millisecond,
		SumRTT:    200 * time.Millisecond,
	}
	lease, err := m.retainCQMSeries("user1")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	m.observeCQMBucket("user1", bucket, 100)

	m.bucketState.With("user1") // should not panic
}

func TestObserveCQMBucket_NilMetrics(t *testing.T) {
	old := l2tpMetricsPtr.Load()
	l2tpMetricsPtr.Store(nil)
	defer l2tpMetricsPtr.Store(old)

	var m *l2tpMetrics
	m.observeCQMBucket("user1", CQMBucket{EchoCount: 1, SumRTT: time.Millisecond}, 100)
}

func TestDeleteLoginMetrics(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	bindL2TPMetrics(reg)
	defer l2tpMetricsPtr.Store(nil)

	m := l2tpMetricsPtr.Load()
	lease, err := m.retainCQMSeries("user1")
	if err != nil {
		t.Fatal(err)
	}
	m.lcpEchoLossRatio.With("user1").Set(0.05)
	m.bucketState.With("user1").Set(0)

	lease.Release()
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

	ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), nil)
	reactor := newL2TPReactor(ln, nil, reactorParams{MaxTunnels: 10, MaxSessions: 10})

	poller := newL2TPStatsPoller([]*l2tpReactor{reactor}, 30*time.Second)
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

func newInertMetricsTestSubsystem(t *testing.T, cqm bool) *Subsystem {
	t.Helper()
	previousProbe, previousWorker := probeKernelModulesFn, newSubsystemKernelWorkerFn
	probeKernelModulesFn = func() error { return nil }
	newSubsystemKernelWorkerFn = func(chan<- kernelSetupFailed, chan<- kernelSetupSucceeded, *slog.Logger, kernelBackendPreparation) (*kernelWorker, error) {
		return nil, nil //nolint:nilnil // inert root without privileged netlink
	}
	t.Cleanup(func() {
		probeKernelModulesFn, newSubsystemKernelWorkerFn = previousProbe, previousWorker
	})
	return NewSubsystem(&Parameters{
		Enabled:                 true,
		ListenAddrs:             []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0")},
		ListenNames:             []string{"lab"},
		MaxTunnels:              4,
		MaxSessions:             4,
		MRU:                     1500,
		HelloInterval:           60 * time.Second,
		CQMEnabled:              cqm,
		MaxLogins:               4,
		EventRingSizePerSession: 8,
		SampleRetentionSeconds:  300,
	})
}

// TestSubsystemMetricsBindWhenRegistryArrivesAfterStart proves AC-12
// (registry-read-outruns-its-lazy-creation): on an L2TP-only daemon without
// BGP, Subsystem.Start runs before startStandaloneTelemetry creates the
// Prometheus registry, and deferring through registry.InjectPluginMetrics
// binds ze_l2tp_* gauges, attaches the CQM observer (including logins tracked
// before the registry arrived), starts the stats poller, and cleans up on Stop.
//
// VALIDATES: AC-12 end-to-end against scraped /metrics output from the
// late-registered PrometheusRegistry, plus clean Stop teardown.
// PREVENTS: Subsystem.Start reading nil from registry.GetMetricsRegistry and
// leaving ze_l2tp_* gauges and CQM series absent for the process lifetime.
func TestSubsystemMetricsBindWhenRegistryArrivesAfterStart(t *testing.T) {
	previousRegistry, previousBinding := registry.GetMetricsRegistry(), l2tpMetricsPtr.Load()
	t.Cleanup(func() {
		registry.SetMetricsRegistry(previousRegistry)
		l2tpMetricsPtr.Store(previousBinding)
	})
	registry.SetMetricsRegistry(nil)
	l2tpMetricsPtr.Store(nil)
	t.Setenv("ze.l2tp.metrics.poll-interval", "20ms")

	sub := newInertMetricsTestSubsystem(t, true)
	bus := newCQMNativeBus(t)
	ctx := t.Context()
	require.NoError(t, sub.Start(ctx, bus, nil))
	t.Cleanup(func() { require.NoError(t, sub.Stop(ctx)) })

	// Nothing is bound yet while no registry exists.
	require.Nil(t, l2tpMetricsPtr.Load())
	sub.mu.Lock()
	require.Nil(t, sub.statsPoller)
	obs := sub.observer
	sub.mu.Unlock()
	require.NotNil(t, obs)

	// Track one CQM login before the registry arrives to prove late attach
	// retains its series leases too.
	preBirth := cqmAPIBirth(t)
	at := time.Unix(2000, 0)
	cqmAPIEstablished(t, obs, obs.generation, preBirth, "pre-reg", at)

	// Telemetry creates the registry AFTER Subsystem.Start.
	reg := metrics.NewPrometheusRegistry()
	registry.SetMetricsRegistry(reg)

	require.NotNil(t, l2tpMetricsPtr.Load(), "ze_l2tp_* metrics must bind when registry arrives late")

	// The stats poller must have started and populated the core gauges.
	require.Eventually(t, func() bool {
		body := cqmAPIScrape(t, reg)
		return strings.Contains(body, "ze_l2tp_tunnels_active 0\n") &&
			strings.Contains(body, "ze_l2tp_sessions_active 0\n")
	}, 2*time.Second, 10*time.Millisecond, "late-started stats poller must populate ze_l2tp_* gauges")

	// Both pre-existing and newly admitted CQM logins publish into reg.
	postBirth := cqmAPIBirth(t)
	cqmAPIEstablished(t, obs, obs.generation, postBirth, "post-reg", at)
	require.NoError(t, obs.observeEvent(obs.generation, cqmAPIEvent(preBirth, ObserverEventEchoRTT, at.Add(BucketInterval), 11*time.Millisecond), "pre-reg"))
	require.NoError(t, obs.observeEvent(obs.generation, cqmAPIEvent(postBirth, ObserverEventEchoRTT, at.Add(BucketInterval), 19*time.Millisecond), "post-reg"))

	scraped := cqmAPIScrape(t, reg)
	require.Contains(t, scraped, `ze_l2tp_lcp_echo_rtt_seconds_count{username="pre-reg"} 1`+"\n")
	require.Contains(t, scraped, `ze_l2tp_lcp_echo_rtt_seconds_count{username="post-reg"} 1`+"\n")

	// Stop cleans up the poller and releases CQM series leases cleanly.
	require.NoError(t, sub.Stop(ctx))
	sub.mu.Lock()
	require.Nil(t, sub.statsPoller)
	sub.mu.Unlock()
	afterStop := cqmAPIScrape(t, reg)
	require.NotContains(t, afterStop, `username="pre-reg"`)
	require.NotContains(t, afterStop, `username="post-reg"`)
}

// TestSubsystemStopBeforeLateMetricsRegistryStartsNoPoller proves stopping a
// subsystem before the metrics registry arrives detaches its deferred hook so
// no stats poller goroutine leaks on a stopped subsystem.
//
// VALIDATES: AC-12 teardown safety when Stop precedes SetMetricsRegistry.
// PREVENTS: a late InjectPluginMetrics callback starting a poller goroutine on
// an already-stopped Subsystem.
func TestSubsystemStopBeforeLateMetricsRegistryStartsNoPoller(t *testing.T) {
	previousRegistry, previousBinding := registry.GetMetricsRegistry(), l2tpMetricsPtr.Load()
	t.Cleanup(func() {
		registry.SetMetricsRegistry(previousRegistry)
		l2tpMetricsPtr.Store(previousBinding)
	})
	registry.SetMetricsRegistry(nil)
	l2tpMetricsPtr.Store(nil)

	sub := newInertMetricsTestSubsystem(t, false)
	ctx := t.Context()
	require.NoError(t, sub.Start(ctx, nil, nil))
	require.NoError(t, sub.Stop(ctx))

	registry.SetMetricsRegistry(metrics.NewPrometheusRegistry())

	sub.mu.Lock()
	poller := sub.statsPoller
	sub.mu.Unlock()
	require.Nil(t, poller, "late metrics registry must not start a poller on a stopped subsystem")
}

// TestSubsystemConcurrentStopAndLateMetricsRegistry exercises concurrent
// Subsystem.Stop and registry.SetMetricsRegistry under -race to verify lock
// ordering and zero leaked pollers across interleavings.
//
// VALIDATES: AC-12 race freedom and clean poller shutdown under concurrent
// Stop and late SetMetricsRegistry.
func TestSubsystemConcurrentStopAndLateMetricsRegistry(t *testing.T) {
	previousRegistry, previousBinding := registry.GetMetricsRegistry(), l2tpMetricsPtr.Load()
	t.Cleanup(func() {
		registry.SetMetricsRegistry(previousRegistry)
		l2tpMetricsPtr.Store(previousBinding)
	})
	t.Setenv("ze.l2tp.metrics.poll-interval", "10ms")

	ctx := t.Context()
	for range 8 {
		registry.SetMetricsRegistry(nil)
		l2tpMetricsPtr.Store(nil)

		sub := newInertMetricsTestSubsystem(t, true)
		require.NoError(t, sub.Start(ctx, newCQMNativeBus(t), nil))

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			registry.SetMetricsRegistry(metrics.NewPrometheusRegistry())
		}()
		go func() {
			defer wg.Done()
			require.NoError(t, sub.Stop(ctx))
		}()
		wg.Wait()

		sub.mu.Lock()
		poller := sub.statsPoller
		sub.mu.Unlock()
		require.Nil(t, poller, "stats poller must be stopped and nil after concurrent Stop")
	}
}
