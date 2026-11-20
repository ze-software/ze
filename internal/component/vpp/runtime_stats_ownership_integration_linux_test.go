//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- runtime counters with actual physical custody
// Related: runtime_stats_test.go -- portable state and runtime doctor questions
package vpp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
)

type runtimeStatsTestClient struct {
	peer        *processWitness
	mu          sync.Mutex
	reads       int
	disconnects int
	read        func(*api.InterfaceStats) error
}

func (c *runtimeStatsTestClient) processPeer() *processWitness { return c.peer }

func (c *runtimeStatsTestClient) GetInterfaceStats(out *api.InterfaceStats) error {
	c.mu.Lock()
	c.reads++
	read := c.read
	c.mu.Unlock()
	if read != nil {
		return read(out)
	}
	out.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 2048}}}
	return nil
}
func (*runtimeStatsTestClient) GetNodeStats(*api.NodeStats) error     { return nil }
func (*runtimeStatsTestClient) GetSystemStats(*api.SystemStats) error { return nil }
func (c *runtimeStatsTestClient) Disconnect()                         { c.mu.Lock(); defer c.mu.Unlock(); c.disconnects++ }

// publishedCounterDumpRows answers how many rows the published lease's last
// interface dump carried. A row read answers one interface, so a test that
// proves the whole dump's population reads it here.
func publishedCounterDumpRows(t *testing.T) int {
	t.Helper()
	lease, ok := GetActiveStatsProvider().(*runtimeStats)
	if !ok {
		t.Fatal("no generation-bound statistics lease is published")
	}
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return len(lease.counters.dump.Interfaces)
}

func runtimeStatsTestManager(t *testing.T) *VPPManager {
	t.Helper()
	runtimeStatsTestState(t)
	return &VPPManager{connector: &Connector{connected: true, conn: &physicalFixtureConnection{connectorConnection: new(recordingConnection), peer: newRuntimeStatsTestPeer(t)}, generation: 7},
		settings: &VPPSettings{Stats: StatsSettings{SocketPath: "/test/stats.sock", PollInterval: 30}}}
}

// Drive the manager's post-API activation entry point and read real counter
// payload during the readiness callback, with no metrics registry at all.
func TestVPPRuntimeStatsReadyWithoutMetrics(t *testing.T) {
	m := runtimeStatsTestManager(t)
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(namespace, event string, _ any) {
		if namespace != vppevents.Namespace {
			t.Fatalf("namespace=%q", namespace)
		}
		if event != vppevents.EventConnected {
			t.Fatalf("event=%q", event)
		}
		row, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if publishedCounterDumpRows(t) != 1 {
			t.Fatal("readiness preceded counter publication")
		}
		if row.Rx.Bytes != 2048 {
			t.Fatal("counter payload was lost")
		}
		events++
	}})
	lease, err := m.activateRuntimeStats(t.Context(), func(path string) (runtimeStatsClient, error) {
		if path != "/test/stats.sock" {
			t.Fatalf("path=%q", path)
		}
		return client, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("no stats owner without telemetry")
	}
	lease.Close()
	lease.Close()
	if events != 1 {
		t.Fatalf("ready events=%d", events)
	}
	if client.disconnects != 1 {
		t.Fatalf("disconnects=%d", client.disconnects)
	}
	if GetActiveStatsProvider() != nil {
		t.Fatal("closed stats remain published")
	}
}

// A failed startup read is unavailable, never published as zero counters;
// the API readiness event remains independent and doctor retains the cause.
func TestVPPRuntimeStatsFailureIsVisibleBeforeReadiness(t *testing.T) {
	m := runtimeStatsTestManager(t)
	cause := errors.New("stats segment unreadable")
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t), read: func(*api.InterfaceStats) error { return cause }}
	defer report.ClearSource(runtimeStatsReportSource)
	events := 0
	setVPPEventBus(runtimeStatsTestBus{emit: func(_, event string, _ any) {
		if event != vppevents.EventConnected {
			t.Fatalf("event=%q", event)
		}
		if GetActiveStatsProvider() != nil {
			t.Fatal("failed stats were published")
		}
		findings := checkRuntimeStats(diagnostic.DoctorCheckContext{})
		if len(findings) == 0 {
			t.Fatal("readiness preceded failure reporting")
		}
		events++
	}})
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease != nil {
		t.Fatal("failed read retained an owner")
	}
	if events != 1 {
		t.Fatalf("ready events=%d", events)
	}
	if client.disconnects != 1 {
		t.Fatalf("failed startup disconnects=%d", client.disconnects)
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); err == nil {
		t.Fatal("unavailable stats answered success")
	}
}

// A replacement using the same Connector cannot lend its new stats segment
// to a session created against an earlier API generation.
func TestVPPRuntimeStatsRejectsOtherGeneration(t *testing.T) {
	m := runtimeStatsTestManager(t)
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("stats startup failed")
	}
	defer lease.Close()
	for _, owner := range []*Connector{m.connector, {}} {
		if _, err := InterfaceCountersForGeneration(owner, 8, 17, time.Now()); err == nil {
			t.Fatal("wrong generation or owner borrowed stats")
		}
	}
	m.connector.mu.Lock()
	m.connector.generation++
	m.connector.mu.Unlock()
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); err == nil {
		t.Fatal("retired API still borrowed its stats")
	}
}

// A generation change during a read invalidates the completed snapshot too.
func TestVPPRuntimeStatsRejectsGenerationChangeDuringRead(t *testing.T) {
	m := runtimeStatsTestManager(t)
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("stats startup failed")
	}
	defer lease.Close()
	client.mu.Lock()
	// The dump carries the asked-for row, so only the post-read generation
	// check can refuse it, never a missing row.
	client.read = func(out *api.InterfaceStats) error {
		out.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 2048}}}
		m.connector.mu.Lock()
		m.connector.generation++
		m.connector.mu.Unlock()
		return nil
	}
	client.mu.Unlock()
	// A fresh bound forces the dump that changes the generation mid-read.
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); !errors.Is(err, errVppGenerationChanged) {
		t.Fatal("published counters across generation replacement")
	}
}

// The owner joins the live poller before disconnecting its underlying client.
func TestVPPRuntimeStatsCloseJoinsPoller(t *testing.T) {
	m := runtimeStatsTestManager(t)
	var reg metrics.Registry = newTestRegistry()
	metricsRegPtr.Store(&reg)
	entered := make(chan struct{})
	release := make(chan struct{})
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	calls := 0
	client.read = func(*api.InterfaceStats) error {
		calls++
		if calls == 2 {
			close(entered)
			<-release
		}
		return nil
	}
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("stats startup failed")
	}
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); lease.Close() })
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("poller never read counters")
	}
	closed := make(chan struct{})
	go func() { lease.Close(); close(closed) }()
	client.mu.Lock()
	disconnects := client.disconnects
	client.mu.Unlock()
	if disconnects != 0 {
		t.Fatal("client disconnected while poller was reading")
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("stats owner did not join poller")
	}
	if client.disconnects != 1 {
		t.Fatalf("disconnects=%d", client.disconnects)
	}
}

// A closed prior owner must neither clear a replacement publication nor use
// the replacement's client to answer an old session's read.
func TestVPPRuntimeStatsReplacementKeepsNewPublication(t *testing.T) {
	m := runtimeStatsTestManager(t)
	first := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	old, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return first, nil })
	if err != nil {
		t.Fatal(err)
	}
	if old == nil {
		t.Fatal("initial stats startup failed")
	}
	t.Cleanup(old.Close)
	m.connector.mu.Lock()
	m.connector.generation++
	m.connector.mu.Unlock()
	second := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	replacement, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return second, nil })
	if err != nil {
		t.Fatal(err)
	}
	if replacement == nil {
		t.Fatal("replacement stats startup failed")
	}
	t.Cleanup(replacement.Close)
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); err == nil {
		t.Fatal("old session read the replacement")
	}
	if second.reads != 1 {
		t.Fatal("old session reached the replacement client")
	}
	old.Close()
	if _, err := InterfaceCountersForGeneration(m.connector, 8, 17, time.Now()); err != nil {
		t.Fatal(err)
	}
	if second.disconnects != 0 {
		t.Fatal("old owner disconnected its replacement")
	}
}

// Cancellation is an activation barrier, not a connected event or a stats
// failure. Drive the same entry point with a canceled component lifetime.
func TestVPPRuntimeStatsCancellationStopsReadiness(t *testing.T) {
	m := runtimeStatsTestManager(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	setVPPEventBus(runtimeStatsTestBus{emit: func(string, string, any) { t.Fatal("canceled activation published readiness") }})
	lease, err := m.activateRuntimeStats(ctx, func(string) (runtimeStatsClient, error) {
		t.Fatal("canceled activation opened stats")
		return nil, context.Canceled
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled activation error=%v", err)
	}
	if lease != nil {
		t.Fatal("canceled activation retained stats")
	}
}

// A different statistics category succeeding cannot erase a failed interface
// read. Only a successful interface read from the pinned owner confirms it.
func TestVPPRuntimeStatsRecoveryIsCategorySpecific(t *testing.T) {
	m := runtimeStatsTestManager(t)
	t.Cleanup(func() { report.ClearSource(runtimeStatsReportSource) })
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("stats startup failed")
	}
	t.Cleanup(lease.Close)
	cause := errors.New("interface directory unavailable")
	client.mu.Lock()
	client.read = func(*api.InterfaceStats) error { return cause }
	client.mu.Unlock()
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); !errors.Is(err, cause) {
		t.Fatalf("read error=%v", err)
	}
	var nodes api.NodeStats
	if err := lease.GetNodeStats(&nodes); err != nil {
		t.Fatal(err)
	}
	if len(checkRuntimeStats(diagnostic.DoctorCheckContext{})) == 0 {
		t.Fatal("node success erased an interface failure")
	}
	client.mu.Lock()
	client.read = nil
	client.mu.Unlock()
	// The failed dump left no snapshot, so this fresh bound dumps again.
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Now()); err != nil {
		t.Fatal(err)
	}
	if findings := checkRuntimeStats(diagnostic.DoctorCheckContext{}); len(findings) != 0 {
		t.Fatalf("successful interface recovery remains failed: %+v", findings)
	}
}

// A failed connect has no acquired client. A subsequent confirmed connection
// clears only that owned connection failure and serves its actual counters.
func TestVPPRuntimeStatsConnectionRecovery(t *testing.T) {
	m := runtimeStatsTestManager(t)
	t.Cleanup(func() { report.ClearSource(runtimeStatsReportSource) })
	lease, err := m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return nil, errors.New("stats socket missing") })
	if err != nil {
		t.Fatal(err)
	}
	if lease != nil {
		t.Fatal("failed connect retained ownership")
	}
	if len(checkRuntimeStats(diagnostic.DoctorCheckContext{})) == 0 {
		t.Fatal("failed connection was not retained")
	}
	m.connector.mu.Lock()
	m.connector.generation++
	m.connector.mu.Unlock()
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err = m.activateRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	if lease == nil {
		t.Fatal("stats recovery failed")
	}
	t.Cleanup(lease.Close)
	if findings := checkRuntimeStats(diagnostic.DoctorCheckContext{}); len(findings) != 0 {
		t.Fatalf("confirmed connection recovery remains failed: %+v", findings)
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 8, 17, time.Now()); err != nil {
		t.Fatal(err)
	}
	if publishedCounterDumpRows(t) != 1 {
		t.Fatal("recovered connection returned no counter snapshot")
	}
}
