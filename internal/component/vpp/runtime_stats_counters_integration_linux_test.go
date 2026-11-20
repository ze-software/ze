//go:build integration && linux

// Design: docs/research/vpp-deployment-reference.md -- runtime statistics ownership
// Related: runtime_stats_counters.go -- the shared per-interface counter snapshot
// Related: runtime_stats_ownership_integration_linux_test.go -- the test client and manager
package vpp

import (
	"errors"
	"io"
	"testing"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/clock"
)

// stepClock answers a Now the test moves by hand, so snapshot age is decided
// by the test rather than by scheduling.
type stepClock struct {
	clock.RealClock
	now time.Time
}

func (c *stepClock) Now() time.Time { return c.now }

// counterTestLease publishes a generation-7 lease with no telemetry poller and
// returns it with its client. The initial readiness read has already happened.
func counterTestLease(t *testing.T) (*VPPManager, *runtimeStats, *runtimeStatsTestClient) {
	t.Helper()
	m := runtimeStatsTestManager(t)
	client := &runtimeStatsTestClient{peer: newRuntimeStatsTestPeer(t)}
	lease, err := m.connectRuntimeStats(t.Context(), func(string) (runtimeStatsClient, error) { return client, nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(lease.Close)
	return m, lease, client
}

func (c *runtimeStatsTestClient) setRead(read func(*api.InterfaceStats) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.read = read
}

func (c *runtimeStatsTestClient) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

// Goal: many sessions read within one interval cost one VPP dump, not one each.
// Method: a dump of 512 interfaces, then one lookup per interface through the
// exported entry the PPPoE session calls, all under the same freshness bound.
// MUTATION: taking a dump on every lookup, or scanning instead of indexing
// with a wrong match, turns this red.
func TestVPPInterfaceCountersShareOneDumpAcrossSessions(t *testing.T) {
	m, _, client := counterTestLease(t)
	const sessions = 512
	client.setRead(func(out *api.InterfaceStats) error {
		out.Interfaces = make([]api.InterfaceCounters, sessions)
		for i := range out.Interfaces {
			index := uint32(1000 + i)
			out.Interfaces[i] = api.InterfaceCounters{InterfaceIndex: index,
				Rx: api.InterfaceCounterCombined{Bytes: uint64(index) * 10}, Tx: api.InterfaceCounterCombined{Bytes: uint64(index) * 20}}
		}
		return nil
	})
	before := client.readCount()
	notBefore := time.Now()
	for i := range sessions {
		index := uint32(1000 + i)
		counters, err := InterfaceCountersForGeneration(m.connector, 7, index, notBefore)
		if err != nil {
			t.Fatalf("index %d: %v", index, err)
		}
		if counters.InterfaceIndex != index {
			t.Fatalf("index %d answered row %d", index, counters.InterfaceIndex)
		}
		if counters.Rx.Bytes != uint64(index)*10 {
			t.Fatalf("index %d rx bytes %d", index, counters.Rx.Bytes)
		}
		if counters.Tx.Bytes != uint64(index)*20 {
			t.Fatalf("index %d tx bytes %d", index, counters.Tx.Bytes)
		}
	}
	if dumps := client.readCount() - before; dumps != 1 {
		t.Fatalf("%d sessions took %d dumps within one interval, want 1", sessions, dumps)
	}
	// A fresh request after that dump started takes the next one.
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 1000, time.Now()); err != nil {
		t.Fatal(err)
	}
	if dumps := client.readCount() - before; dumps != 2 {
		t.Fatalf("a fresh read after the interval dump took %d dumps in total, want 2", dumps)
	}
}

// Goal: a snapshot never answers for another owner or generation.
// Method: cache a snapshot, then ask with a wrong generation and a wrong owner,
// then retire the generation on the live connector and ask with the old one.
// MUTATION: serving the cached rows before checking the live generation
// answers the retired request.
func TestVPPInterfaceCountersRefuseOtherGeneration(t *testing.T) {
	m, _, client := counterTestLease(t)
	notBefore := time.Now()
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, notBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 8, 17, notBefore); !errors.Is(err, errVppGenerationChanged) {
		t.Fatalf("another generation borrowed the snapshot: %v", err)
	}
	if _, err := InterfaceCountersForGeneration(&Connector{}, 7, 17, notBefore); err == nil {
		t.Fatal("another API owner borrowed the snapshot")
	}
	reads := client.readCount()
	m.connector.mu.Lock()
	m.connector.generation++
	m.connector.mu.Unlock()
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, notBefore); !errors.Is(err, errVppGenerationChanged) {
		t.Fatalf("retired generation answered from its cached snapshot: %v", err)
	}
	if client.readCount() != reads {
		t.Fatal("retired generation took a dump")
	}
}

// Goal: a missing row and a duplicate row are errors, never zero counters.
// Method: one dump carrying index 17 once and index 18 twice; the third
// lookup asks for an index the dump never carried.
// MUTATION: letting the last duplicate win, or returning a zero row for an
// absent index, turns this red.
func TestVPPInterfaceCountersRefuseMissingAndDuplicateRows(t *testing.T) {
	m, _, client := counterTestLease(t)
	client.setRead(func(out *api.InterfaceStats) error {
		out.Interfaces = []api.InterfaceCounters{
			{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 7}},
			{InterfaceIndex: 18, Rx: api.InterfaceCounterCombined{Bytes: 1}},
			{InterfaceIndex: 18, Rx: api.InterfaceCounterCombined{Bytes: 2}},
		}
		return nil
	})
	notBefore := time.Now()
	counters, err := InterfaceCountersForGeneration(m.connector, 7, 17, notBefore)
	if err != nil {
		t.Fatal(err)
	}
	if counters.Rx.Bytes != 7 {
		t.Fatalf("index 17 rx bytes %d", counters.Rx.Bytes)
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 18, notBefore); err == nil {
		t.Fatal("a duplicated index answered one of its rows")
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 19, notBefore); err == nil {
		t.Fatal("an absent index answered")
	}
}

// Goal: an expired snapshot never answers, and a failed refresh is reported
// rather than masked by the older snapshot.
// Method: a hand-moved clock. A snapshot taken at t0 is older than a bound of
// t1; the refresh fails, which must be the answer. The failure discards the
// t0 snapshot, so even a request t0 would satisfy takes a new dump.
// MUTATION: keeping the old snapshot after a failed dump answers 100 here.
func TestVPPInterfaceCountersRefuseExpiredSnapshot(t *testing.T) {
	m, lease, client := counterTestLease(t)
	t0 := time.Unix(1_700_000_000, 0)
	moved := &stepClock{now: t0}
	lease.clock = moved
	client.setRead(func(out *api.InterfaceStats) error {
		out.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 100}}}
		return nil
	})
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, t0); err != nil {
		t.Fatal(err)
	}
	t1 := t0.Add(time.Minute)
	moved.now = t1
	cause := errors.New("stats segment unreadable")
	client.setRead(func(*api.InterfaceStats) error { return cause })
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, t1); !errors.Is(err, cause) {
		t.Fatalf("expired snapshot answered instead of its failed refresh: %v", err)
	}
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, t0); !errors.Is(err, cause) {
		t.Fatalf("snapshot from before a failed refresh still answered: %v", err)
	}
	client.setRead(func(out *api.InterfaceStats) error {
		out.Interfaces = []api.InterfaceCounters{{InterfaceIndex: 17, Rx: api.InterfaceCounterCombined{Bytes: 200}}}
		return nil
	})
	counters, err := InterfaceCountersForGeneration(m.connector, 7, 17, t1)
	if err != nil {
		t.Fatal(err)
	}
	if counters.Rx.Bytes != 200 {
		t.Fatalf("recovered read answered rx bytes %d, want the new dump's 200", counters.Rx.Bytes)
	}
}

// Goal: no request can accept a snapshot of unbounded age, and a closed
// owner answers nothing.
// Method: a zero bound against a live lease, then any bound after Close.
func TestVPPInterfaceCountersRefuseUnboundedAndClosed(t *testing.T) {
	m, lease, client := counterTestLease(t)
	reads := client.readCount()
	if _, err := InterfaceCountersForGeneration(m.connector, 7, 17, time.Time{}); err == nil {
		t.Fatal("a request with no freshness bound answered")
	}
	if client.readCount() != reads {
		t.Fatal("a request with no freshness bound took a dump")
	}
	if _, err := lease.interfaceCounters(17, time.Now()); err != nil {
		t.Fatal(err)
	}
	lease.Close()
	if _, err := lease.interfaceCounters(17, time.Now()); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed owner answered: %v", err)
	}
}
