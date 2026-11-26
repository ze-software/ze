// VALIDATES: hub-side managed inventory/health report receivers, per-connection
// rate limiting, snapshot deep-copy ownership, persistence across disconnect
// and restart, deletion reconciliation, and fleet health aggregation (AC-2,
// AC-4, AC-6, AC-7, AC-8, AC-9, AC-10, AC-14, AC-15, AC-16, AC-17, AC-18).
// PREVENTS: stale sessions, invalid/oversized/rate-limited reports, delayed
// record writes, operator deletions, or caller mutations from corrupting live
// or persisted device inventory and health snapshots.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type reportTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func newReportTestClock(start time.Time) *reportTestClock {
	return &reportTestClock{now: start.UTC()}
}

func (c *reportTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *reportTestClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	c.now = c.now.Add(d)
	out := c.now
	c.mu.Unlock()
	return out
}

func sampleInventoryReport(collectedAt, model, serial string, drivers []string, corr, uncorr uint64) fleet.InventoryReport {
	return fleet.InventoryReport{
		CollectedAt:     collectedAt,
		SoftwareVersion: "0.9.0-dev",
		CPU: fleet.InventoryCPU{
			Available:   true,
			LogicalCPUs: 16,
			Model:       "AMD EPYC 7313P",
		},
		NIC: fleet.InventoryNIC{
			Available: true,
			Count:     len(drivers),
			Drivers:   drivers,
		},
		Memory: fleet.InventoryMemory{
			Available:  true,
			TotalBytes: 64 << 30,
			ECC: &fleet.InventoryECC{
				CorrectableErrors:   corr,
				UncorrectableErrors: uncorr,
			},
		},
		Storage: fleet.InventoryStorage{
			Available:   true,
			DeviceCount: 2,
			TotalBytes:  1 << 40,
		},
		DMI: fleet.InventoryDMI{
			Available: true,
			Model:     model,
			Serial:    serial,
		},
	}
}

func sampleHealthReport(checkedAt string, status fleet.HealthStatus, comps ...fleet.HealthComponent) fleet.HealthReport {
	return fleet.HealthReport{
		Status:     status,
		Components: comps,
		CheckedAt:  checkedAt,
	}
}

func startReportTestServer(t *testing.T, store storage.Storage, clk *reportTestClock, clients map[string]string) (*ManagedServer, string) {
	t.Helper()
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Store:     store,
		Listeners: []ManagedListener{{Addr: "127.0.0.1:0", Clients: clients}},
		ReadConfig: func(name string) ([]byte, error) {
			if _, ok := clients[name]; ok {
				return []byte(testClientConfig), nil
			}
			return nil, ErrClientConfigNotFound
		},
	})
	require.NoError(t, err)
	if clk != nil {
		srv.now = clk.Now
	}
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	require.Len(t, addrs, 1)
	return srv, addrs[0].String()
}

func callReportRPC(t *testing.T, mc *rpc.MuxConn, method string, params any) (json.RawMessage, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return mc.CallRPC(ctx, method, params)
}

func TestInventoryReportHandler(t *testing.T) {
	hubNow := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	clk := newReportTestClock(hubNow)
	store := heldTestStore(t)
	srv, addr := startReportTestServer(t, store, clk, map[string]string{
		testClientName:  testClientSecret,
		testClient2Name: testClient2Secret,
	})

	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	inv := sampleInventoryReport(
		"2026-06-01T11:59:50Z",
		"Supermicro SYS-5019D",
		"DMI-SN-001",
		[]string{"ixgbe", "mlx5_core"},
		3,
		0,
	)

	res, err := callReportRPC(t, mc, fleet.VerbInventoryReport, inv)
	require.NoError(t, err)
	require.Equal(t, json.RawMessage(`{}`), res)

	got, err := srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.NotNil(t, got.Inventory)
	require.Equal(t, hubNow, got.Inventory.ReceivedAt)
	require.Equal(t, inv, got.Inventory.Report)
	require.Equal(t, "Supermicro SYS-5019D", got.Inventory.Report.DMI.Model)
	require.Equal(t, "DMI-SN-001", got.Inventory.Report.DMI.Serial)
	require.Equal(t, "0.9.0-dev", got.Inventory.Report.SoftwareVersion)
	require.NotNil(t, got.Inventory.Report.Memory.ECC)
	require.Equal(t, uint64(3), got.Inventory.Report.Memory.ECC.CorrectableErrors)

	// Unreported peer remains nil (scoped to authenticated session).
	other, err := srv.registryDevice(testClient2Name)
	require.NoError(t, err)
	require.Nil(t, other.Inventory)

	// Partial inventory with unavailable sections is accepted cleanly.
	mc2 := dialTestClient(t, addr, testClient2Name, testClient2Secret)
	partial := fleet.InventoryReport{
		CollectedAt:     "2026-06-01T11:59:55Z",
		SoftwareVersion: "0.9.0-dev",
		CPU:             fleet.InventoryCPU{Available: true, LogicalCPUs: 4, Model: "Intel Xeon"},
		NIC:             fleet.InventoryNIC{Available: false},
		Memory:          fleet.InventoryMemory{Available: true, TotalBytes: 16 << 30},
		Storage:         fleet.InventoryStorage{Available: false},
		DMI:             fleet.InventoryDMI{Available: false},
	}
	_, err = callReportRPC(t, mc2, fleet.VerbInventoryReport, partial)
	require.NoError(t, err)
	other, err = srv.registryDevice(testClient2Name)
	require.NoError(t, err)
	require.NotNil(t, other.Inventory)
	require.False(t, other.Inventory.Report.NIC.Available)
	require.False(t, other.Inventory.Report.DMI.Available)
	require.Nil(t, other.Inventory.Report.Memory.ECC)
}

func TestHealthReportHandler(t *testing.T) {
	hubNow := time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC)
	clk := newReportTestClock(hubNow)
	store := heldTestStore(t)
	srv, addr := startReportTestServer(t, store, clk, map[string]string{
		testClientName:  testClientSecret,
		testClient2Name: testClient2Secret,
	})

	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	health := sampleHealthReport(
		"2026-06-01T12:59:58Z",
		fleet.HealthStatusDegraded,
		fleet.HealthComponent{
			Name:      "bgp",
			Status:    fleet.HealthStatusDegraded,
			Reason:    "1 of 4 peers down",
			CheckedAt: "2026-06-01T12:59:58Z",
		},
		fleet.HealthComponent{
			Name:      "config",
			Status:    fleet.HealthStatusHealthy,
			CheckedAt: "2026-06-01T12:59:58Z",
		},
	)

	res, err := callReportRPC(t, mc, fleet.VerbHealthReport, health)
	require.NoError(t, err)
	require.Equal(t, json.RawMessage(`{}`), res)

	got, err := srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.NotNil(t, got.Health)
	require.Equal(t, hubNow, got.Health.ReceivedAt)
	require.Equal(t, fleet.HealthStatusDegraded, got.Health.Report.Status)
	require.Equal(t, health, got.Health.Report)

	// Unreported peer stays nil.
	other, err := srv.registryDevice(testClient2Name)
	require.NoError(t, err)
	require.Nil(t, other.Health)
}

func TestReportDisconnectRestart(t *testing.T) {
	t1 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t1)
	store := heldTestStore(t)

	// Pre-Fleet-4 record without inventory or health fields must load cleanly.
	require.NoError(t, store.WriteKey(
		deviceRecordKey(testClient2Name),
		[]byte(`{"name":"edge-02","ack-version":"pre-fleet-4"}`),
	))

	clients := map[string]string{
		testClientName:  testClientSecret,
		testClient2Name: testClient2Secret,
	}

	inv1 := sampleInventoryReport("2026-06-02T09:59:00Z", "Model-A", "SN-A", []string{"ixgbe"}, 1, 0)
	health1 := sampleHealthReport(
		"2026-06-02T09:59:30Z",
		fleet.HealthStatusDegraded,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDegraded, Reason: "flapping", CheckedAt: "2026-06-02T09:59:30Z"},
	)

	// Server #1 accepts reports, disconnects client, and shuts down cleanly.
	func() {
		srv1, addr1 := startReportTestServer(t, store, clk, clients)
		pre, err := srv1.registryDevice(testClient2Name)
		require.NoError(t, err)
		require.False(t, pre.Unreadable)
		require.Equal(t, "pre-fleet-4", pre.AckVersion)
		require.Nil(t, pre.Inventory)
		require.Nil(t, pre.Health)

		mc := dialTestClient(t, addr1, testClientName, testClientSecret)
		_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, inv1)
		require.NoError(t, err)
		_, err = callReportRPC(t, mc, fleet.VerbHealthReport, health1)
		require.NoError(t, err)

		clk.Advance(2 * time.Minute)
		require.NoError(t, mc.Close())
		srv1.Stop()
	}()

	// Server #2 restarts on the same store and restores both snapshots.
	t3 := clk.Advance(10 * time.Minute)
	srv2, addr2 := startReportTestServer(t, store, clk, clients)

	restored, err := srv2.registryDevice(testClientName)
	require.NoError(t, err)
	require.False(t, restored.Connected)
	require.True(t, restored.Persisted)
	require.NotNil(t, restored.Inventory)
	require.Equal(t, t1, restored.Inventory.ReceivedAt)
	require.Equal(t, inv1, restored.Inventory.Report)
	require.NotNil(t, restored.Health)
	require.Equal(t, t1, restored.Health.ReceivedAt)
	require.Equal(t, health1, restored.Health.Report)

	// Pre-Fleet-4 device remains explicitly unreported.
	pre, err := srv2.registryDevice(testClient2Name)
	require.NoError(t, err)
	require.Nil(t, pre.Inventory)
	require.Nil(t, pre.Health)

	// Reconnect and replace both snapshots with newer reports.
	mc2 := dialTestClient(t, addr2, testClientName, testClientSecret)
	inv2 := sampleInventoryReport("2026-06-02T10:11:50Z", "Model-B", "SN-B", []string{"mlx5_core"}, 0, 0)
	health2 := sampleHealthReport(
		"2026-06-02T10:11:55Z",
		fleet.HealthStatusHealthy,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: "2026-06-02T10:11:55Z"},
	)
	_, err = callReportRPC(t, mc2, fleet.VerbInventoryReport, inv2)
	require.NoError(t, err)
	_, err = callReportRPC(t, mc2, fleet.VerbHealthReport, health2)
	require.NoError(t, err)

	updated, err := srv2.registryDevice(testClientName)
	require.NoError(t, err)
	require.True(t, updated.Connected)
	require.Equal(t, t3, updated.Inventory.ReceivedAt)
	require.Equal(t, inv2, updated.Inventory.Report)
	require.Equal(t, t3, updated.Health.ReceivedAt)
	require.Equal(t, health2, updated.Health.Report)

	// Corrupt stored report fields must be rejected as unreadable on load.
	for _, bad := range []string{
		`{"name":"edge-01","inventory":{"report":{"collected-at":"2026-06-02T10:00:00Z","software-version":"0.9.0","cpu":{"available":false},"nic":{"available":false},"memory":{"available":false},"storage":{"available":false},"dmi":{"available":false}}}}`,
		`{"name":"edge-01","inventory":{"received-at":"2026-06-02T10:00:00Z","report":{"collected-at":"2026-06-02T10:00:00Z","software-version":"0.9.0","cpu":{"available":false,"logical-cpus":4},"nic":{"available":false},"memory":{"available":false},"storage":{"available":false},"dmi":{"available":false}}}}`,
		`{"name":"edge-01","health":{"received-at":"2026-06-02T10:00:00Z","report":{"status":"healthy","checked-at":"2026-06-02T10:00:00Z","components":[{"name":"bgp","status":"down","checked-at":"2026-06-02T10:00:00Z"}]}}}`,
		`{"name":"edge-01","health":{"received-at":"2026-06-02T10:00:00Z","report":{"status":"healthy","checked-at":"2026-06-02T10:00:00Z","components":[]},"extra":1}}`,
	} {
		corruptStore := heldTestStore(t)
		require.NoError(t, corruptStore.WriteKey(deviceRecordKey(testClientName), []byte(bad)))
		loaded := registryWithHistory(t, corruptStore)
		require.True(t, loaded.snapshotDevices()[0].Unreadable, "expected unreadable for %s", bad)
	}
}

func TestReportStoreSnapshotOwnership(t *testing.T) {
	store := heldTestStore(t)
	s := registryWithHistory(t, store)
	baseTime := time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC)
	clk := newReportTestClock(baseTime)
	s.now = clk.Now

	conn := &rpc.MuxConn{}
	require.NoError(t, s.admitDevice(testClientName, netip.MustParseAddr("192.0.2.10"), conn))
	s.drainDeviceRecords()

	invV1 := sampleInventoryReport("2026-06-03T07:59:00Z", "Host-1", "SN-1", []string{"ixgbe"}, 1, 0)
	healthV1 := sampleHealthReport(
		"2026-06-03T07:59:30Z",
		fleet.HealthStatusDegraded,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDegraded, Reason: "initial reason", CheckedAt: "2026-06-03T07:59:30Z"},
	)
	require.NoError(t, s.recordInventoryReport(testClientName, conn, invV1))
	require.NoError(t, s.recordHealthReport(testClientName, conn, healthV1))

	// Pause writer right after it observes invV1/healthV1 and unlocks s.mu.
	observed := make(chan struct{})
	release := make(chan struct{})
	s.mu.Lock()
	origInvPtr := s.devices[testClientName].Inventory
	s.recordIO.afterObserve = func(name string) {
		if name == testClientName {
			close(observed)
			<-release
		}
	}
	s.mu.Unlock()

	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		s.drainOneDeviceRecord()
	}()
	<-observed

	// Caller snapshot mutation while writer is paused must not touch live or stored snapshots.
	callerSnap, err := s.registryDevice(testClientName)
	require.NoError(t, err)
	callerSnap.Inventory.Report.NIC.Drivers[0] = "tampered-driver"
	callerSnap.Inventory.Report.Memory.ECC.CorrectableErrors = 999
	callerSnap.Health.Report.Components[0].Reason = "tampered-reason"

	// Replace with IDENTICAL payload at the EXACT same clock timestamp.
	// A fresh snapshot pointer must prevent the delayed write from marking it durable.
	require.NoError(t, s.recordInventoryReport(testClientName, conn, invV1))
	s.mu.Lock()
	replacedInvPtr := s.devices[testClientName].Inventory
	s.mu.Unlock()
	require.NotSame(t, origInvPtr, replacedInvPtr)

	// Resume delayed writer for invV1.
	close(release)
	<-drainDone

	// Delayed write must NOT have marked the newer identical-content report durable,
	// and caller mutations must not appear on disk or in live memory.
	afterDelayed := s.snapshotDevices()[0]
	require.False(t, afterDelayed.Persisted)
	require.Equal(t, []string{"ixgbe"}, afterDelayed.Inventory.Report.NIC.Drivers)
	require.Equal(t, uint64(1), afterDelayed.Inventory.Report.Memory.ECC.CorrectableErrors)
	require.Equal(t, "initial reason", afterDelayed.Health.Report.Components[0].Reason)

	rawDisk, err := store.ReadKey(deviceRecordKey(testClientName))
	require.NoError(t, err)
	diskRec, err := decodeDeviceRecord(testClientName, rawDisk)
	require.NoError(t, err)
	require.Equal(t, []string{"ixgbe"}, diskRec.Inventory.Report.NIC.Drivers)
	require.Equal(t, uint64(1), diskRec.Inventory.Report.Memory.ECC.CorrectableErrors)
	require.Equal(t, "initial reason", diskRec.Health.Report.Components[0].Reason)

	// Replace with DIFFERENT content and drain without pause; newer report persists.
	s.mu.Lock()
	s.recordIO.afterObserve = nil
	s.mu.Unlock()

	clk.Advance(time.Minute)
	invV2 := sampleInventoryReport("2026-06-03T08:01:00Z", "Host-2", "SN-2", []string{"mlx5_core"}, 2, 1)
	require.NoError(t, s.recordInventoryReport(testClientName, conn, invV2))
	s.drainDeviceRecords()

	persisted := s.snapshotDevices()[0]
	require.True(t, persisted.Persisted)
	require.Equal(t, invV2, persisted.Inventory.Report)

	rawDisk, err = store.ReadKey(deviceRecordKey(testClientName))
	require.NoError(t, err)
	diskRec, err = decodeDeviceRecord(testClientName, rawDisk)
	require.NoError(t, err)
	require.Equal(t, invV2, diskRec.Inventory.Report)

	// Operator deletion forgets old occurrences, while a report received AFTER deletion is retained.
	key := deviceRecordKey(testClientName)
	require.NoError(t, store.RemoveKey(key))

	afterDelete := s.snapshotDevices()[0]
	require.Nil(t, afterDelete.Inventory)
	require.Nil(t, afterDelete.Health)
	require.False(t, afterDelete.Persisted)

	clk.Advance(time.Minute)
	invV3 := sampleInventoryReport("2026-06-03T08:02:00Z", "Host-3", "SN-3", []string{"virtio_net"}, 0, 0)
	require.NoError(t, s.recordInventoryReport(testClientName, conn, invV3))
	s.drainDeviceRecords()

	restarted := registryWithHistory(t, store).snapshotDevices()[0]
	require.NotNil(t, restarted.Inventory)
	require.Equal(t, invV3, restarted.Inventory.Report)
	require.Nil(t, restarted.Health, "health occurrence from before deletion must stay forgotten")

	// Operator deletion racing with an in-flight write forgets the earlier write,
	// and a subsequent report after deletion survives.
	healthV2 := sampleHealthReport(
		"2026-06-03T08:02:30Z",
		fleet.HealthStatusHealthy,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: "2026-06-03T08:02:30Z"},
	)
	require.NoError(t, s.recordHealthReport(testClientName, conn, healthV2))

	observed2 := make(chan struct{})
	release2 := make(chan struct{})
	s.mu.Lock()
	s.recordIO.afterObserve = func(name string) {
		if name == testClientName {
			close(observed2)
			<-release2
		}
	}
	s.mu.Unlock()

	drainDone2 := make(chan struct{})
	go func() {
		defer close(drainDone2)
		s.drainOneDeviceRecord()
	}()
	<-observed2

	removeDone := make(chan error, 1)
	go func() {
		removeDone <- store.RemoveKey(key)
	}()

	close(release2)
	<-drainDone2
	require.NoError(t, <-removeDone)

	s.mu.Lock()
	s.recordIO.afterObserve = nil
	s.mu.Unlock()

	// Both prior inventory and health were forgotten by the deletion.
	afterRaceDelete := s.snapshotDevices()[0]
	require.Nil(t, afterRaceDelete.Inventory)
	require.Nil(t, afterRaceDelete.Health)
	_, err = store.ReadKey(key)
	require.ErrorIs(t, err, fs.ErrNotExist)

	// A report received after the deletion is retained and persisted.
	clk.Advance(time.Minute)
	healthV3 := sampleHealthReport(
		"2026-06-03T08:03:00Z",
		fleet.HealthStatusDown,
		fleet.HealthComponent{Name: "kernel", Status: fleet.HealthStatusDown, Reason: "netlink lost", CheckedAt: "2026-06-03T08:03:00Z"},
	)
	require.NoError(t, s.recordHealthReport(testClientName, conn, healthV3))
	s.drainDeviceRecords()

	finalRestarted := registryWithHistory(t, store).snapshotDevices()[0]
	require.Nil(t, finalRestarted.Inventory)
	require.NotNil(t, finalRestarted.Health)
	require.Equal(t, healthV3, finalRestarted.Health.Report)
}

func TestReportRefusalsPreserveLastValue(t *testing.T) {
	t0 := time.Date(2026, 6, 4, 9, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := heldTestStore(t)
	srv, addr := startReportTestServer(t, store, clk, map[string]string{
		testClientName:  testClientSecret,
		testClient2Name: testClient2Secret,
	})

	mc := dialTestClient(t, addr, testClientName, testClientSecret)
	inv1 := sampleInventoryReport("2026-06-04T08:59:00Z", "Good-1", "SN-G1", []string{"ixgbe"}, 0, 0)
	health1 := sampleHealthReport(
		"2026-06-04T08:59:30Z",
		fleet.HealthStatusHealthy,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: "2026-06-04T08:59:30Z"},
	)
	_, err := callReportRPC(t, mc, fleet.VerbInventoryReport, inv1)
	require.NoError(t, err)
	_, err = callReportRPC(t, mc, fleet.VerbHealthReport, health1)
	require.NoError(t, err)

	// Advance past rate limits for invalid-payload tests, or test invalid payloads
	// before rate-limit window; verify neither mutates stored state nor advances window.
	// 1. Rate-limited health (< 1m): at t0+30s rejected, and MUST NOT postpone t0+60s.
	clk.Advance(30 * time.Second)
	health2 := sampleHealthReport(
		"2026-06-04T09:00:30Z",
		fleet.HealthStatusDegraded,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDegraded, Reason: "warn", CheckedAt: "2026-06-04T09:00:30Z"},
	)
	_, err = callReportRPC(t, mc, fleet.VerbHealthReport, health2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")

	got, err := srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, health1, got.Health.Report)

	// Other device is unaffected while edge-01 is rate-limited.
	mc2 := dialTestClient(t, addr, testClient2Name, testClient2Secret)
	_, err = callReportRPC(t, mc2, fleet.VerbHealthReport, health2)
	require.NoError(t, err)

	// At t0+60s (only 30s after the rejected attempt), health2 MUST succeed.
	clk.Advance(30 * time.Second)
	_, err = callReportRPC(t, mc, fleet.VerbHealthReport, health2)
	require.NoError(t, err)
	got, err = srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, health2, got.Health.Report)

	// 2. Rate-limited inventory (< 24h): at t0+12h rejected, MUST NOT postpone t0+24h.
	clk.Advance(12*time.Hour - time.Minute)
	inv2 := sampleInventoryReport("2026-06-04T21:00:00Z", "Good-2", "SN-G2", []string{"mlx5_core"}, 1, 0)
	_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, inv2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")
	got, err = srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, inv1, got.Inventory.Report)

	// Advance to t0+24h: now inventory is eligible; test invalid/unknown-field/oversized
	// refusals FIRST and verify none of them consume or postpone the eligible slot.
	clk.Advance(12 * time.Hour)

	// 3a. Invalid JSON payload type (string instead of object).
	_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, "not-an-object")
	require.Error(t, err)

	// 3b. Malformed JSON bytes dispatched directly via handleRequest on a pipe.
	clientPipe, serverPipe := net.Pipe()
	t.Cleanup(func() {
		clientPipe.Close() //nolint:errcheck // test cleanup
		serverPipe.Close() //nolint:errcheck // test cleanup
	})
	pipeServerMC := rpc.NewMuxConn(rpc.NewConn(serverPipe, serverPipe))
	pipeClientMC := rpc.NewMuxConn(rpc.NewConn(clientPipe, clientPipe))
	t.Cleanup(func() {
		pipeServerMC.Close() //nolint:errcheck // test cleanup
		pipeClientMC.Close() //nolint:errcheck // test cleanup
	})
	badReqSession := &managedSession{}
	errCh := make(chan error, 1)
	go func() {
		_, callErr := pipeClientMC.CallRPC(context.Background(), fleet.VerbInventoryReport, nil)
		errCh <- callErr
	}()
	req := <-pipeServerMC.Requests()
	req.Params = json.RawMessage(`{malformed`)
	srv.handleRequest(testClientName, pipeServerMC, req, badReqSession)
	require.Error(t, <-errCh)
	require.Empty(t, badReqSession.lastReport)

	// 3c. Unknown JSON field.
	_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, map[string]any{
		"collected-at":     "2026-06-05T09:00:00Z",
		"software-version": "0.9.0",
		"unknown-field":    true,
	})
	require.Error(t, err)

	// 3d. Oversized payload (> 64 KiB).
	oversized := map[string]any{
		"collected-at":     "2026-06-05T09:00:00Z",
		"software-version": "0.9.0",
		"cpu":              map[string]any{"available": true, "model": strings.Repeat("a", fleet.ReportBodyMax+1024)},
	}
	_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, oversized)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max")

	// Previous inv1 must still be intact, and the eligible 24h slot must STILL be open!
	got, err = srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, inv1, got.Inventory.Report)

	_, err = callReportRPC(t, mc, fleet.VerbInventoryReport, inv2)
	require.NoError(t, err)
	got, err = srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, inv2, got.Inventory.Report)

	// 4. Stale / superseded session and nil connection are refused without mutating state.
	staleMC := &rpc.MuxConn{}
	require.Error(t, srv.recordInventoryReport(testClientName, staleMC, inv1))
	require.Error(t, srv.recordInventoryReport(testClientName, nil, inv1))
	require.Error(t, srv.recordHealthReport(testClientName, staleMC, health1))
	require.Error(t, srv.recordHealthReport(testClientName, nil, health1))
	got, err = srv.registryDevice(testClientName)
	require.NoError(t, err)
	require.Equal(t, inv2, got.Inventory.Report)
	require.Equal(t, health2, got.Health.Report)

	// 5. Unknown method returns "unknown method: <name>" without closing connection.
	_, err = callReportRPC(t, mc, "unknown-future-verb", struct{}{})
	require.Error(t, err)
	rpcErr, ok := errors.AsType[*rpc.RPCCallError](err)
	require.True(t, ok)
	require.Equal(t, "unknown method: unknown-future-verb", rpcErr.Message)

	// Subsequent ping succeeds on the same connection.
	_, err = callReportRPC(t, mc, fleet.VerbPing, nil)
	require.NoError(t, err)
}

func TestFleetHealthAggregation(t *testing.T) {
	// With no published managed server, ManagedFleetHealth returns ErrManagedNotServing.
	_, err := ManagedFleetHealth()
	require.ErrorIs(t, err, ErrManagedNotServing)

	t0 := time.Date(2026, 6, 5, 14, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := heldTestStore(t)

	// Seed an undeclared historical device with HealthStatusDown: must be excluded from summary.
	retiredRecord := ManagedDeviceRecord{
		Name: "retired-01",
		Health: &ManagedHealthSnapshot{
			ReceivedAt: t0,
			Report: sampleHealthReport(
				"2026-06-05T13:59:00Z",
				fleet.HealthStatusDown,
				fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "stopped", CheckedAt: "2026-06-05T13:59:00Z"},
			),
		},
	}
	retiredBytes, err := json.Marshal(retiredRecord)
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(deviceRecordKey("retired-01"), retiredBytes))

	srv, addr := startReportTestServer(t, store, clk, map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
		"edge-03": "edge03-secret-that-is-at-least-32ch",
		"edge-04": "edge04-secret-that-is-at-least-32ch",
	})

	mc1 := dialTestClient(t, addr, "edge-01", testClientSecret)
	_, err = callReportRPC(t, mc1, fleet.VerbHealthReport, sampleHealthReport(
		"2026-06-05T13:59:50Z",
		fleet.HealthStatusHealthy,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: "2026-06-05T13:59:50Z"},
	))
	require.NoError(t, err)

	mc2 := dialTestClient(t, addr, "edge-02", testClient2Secret)
	degradedReport := sampleHealthReport(
		"2026-06-05T13:59:51Z",
		fleet.HealthStatusDegraded,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDegraded, Reason: "peer idle", CheckedAt: "2026-06-05T13:59:51Z"},
	)
	_, err = callReportRPC(t, mc2, fleet.VerbHealthReport, degradedReport)
	require.NoError(t, err)

	// Disconnect edge-02: disconnection must NOT turn it down, and Connected must be false.
	srv.mu.Lock()
	edge2Done := srv.devices["edge-02"].sessionDone
	srv.mu.Unlock()
	require.NoError(t, mc2.Close())
	<-edge2Done

	mc3 := dialTestClient(t, addr, "edge-03", "edge03-secret-that-is-at-least-32ch")
	downReport := sampleHealthReport(
		"2026-06-05T13:59:52Z",
		fleet.HealthStatusDown,
		fleet.HealthComponent{Name: "kernel", Status: fleet.HealthStatusDown, Reason: "netlink error", CheckedAt: "2026-06-05T13:59:52Z"},
	)
	_, err = callReportRPC(t, mc3, fleet.VerbHealthReport, downReport)
	require.NoError(t, err)

	// edge-04 never connects or reports (Unreported).
	summary, err := ManagedFleetHealth()
	require.NoError(t, err)
	require.Equal(t, 4, summary.Total)
	require.Equal(t, 1, summary.Healthy)
	require.Equal(t, 1, summary.Degraded)
	require.Equal(t, 1, summary.Down)
	require.Equal(t, 1, summary.Unreported)
	require.Len(t, summary.Devices, 2)

	require.Equal(t, "edge-02", summary.Devices[0].Name)
	require.Equal(t, fleet.HealthStatusDegraded, summary.Devices[0].Status)
	require.False(t, summary.Devices[0].Connected)
	require.Equal(t, t0, summary.Devices[0].ReceivedAt)
	require.Equal(t, "2026-06-05T13:59:51Z", summary.Devices[0].CheckedAt)
	require.Equal(t, degradedReport.Components, summary.Devices[0].Components)

	require.Equal(t, "edge-03", summary.Devices[1].Name)
	require.Equal(t, fleet.HealthStatusDown, summary.Devices[1].Status)
	require.True(t, summary.Devices[1].Connected)
	require.Equal(t, t0, summary.Devices[1].ReceivedAt)
	require.Equal(t, "2026-06-05T13:59:52Z", summary.Devices[1].CheckedAt)
	require.Equal(t, downReport.Components, summary.Devices[1].Components)

	// Mutating returned Components must not mutate live server state.
	summary.Devices[0].Components[0].Reason = "caller-mutated"
	again, err := srv.FleetHealthSummary()
	require.NoError(t, err)
	require.Equal(t, "peer idle", again.Devices[0].Components[0].Reason)
}

func TestManagedReportReceiverRegistry(t *testing.T) {
	restore := SnapshotManagedReportReceivers()
	t.Cleanup(restore)

	// Built-in inventory and health receivers are registered by init().
	invRecv, ok := lookupManagedReportReceiver(fleet.VerbInventoryReport)
	require.True(t, ok)
	require.Equal(t, "inventory", invRecv.Name)
	require.Equal(t, 24*time.Hour, invRecv.MinInterval)

	healthRecv, ok := lookupManagedReportReceiver(fleet.VerbHealthReport)
	require.True(t, ok)
	require.Equal(t, "health", healthRecv.Name)
	require.Equal(t, time.Minute, healthRecv.MinInterval)

	noopHandle := func(*ManagedServer, string, *rpc.MuxConn, []byte) error { return nil }

	// Missing fields and non-positive intervals are refused.
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "", Method: "custom-a", MinInterval: time.Second, Handle: noopHandle,
	}))
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "custom-a", Method: "", MinInterval: time.Second, Handle: noopHandle,
	}))
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "custom-a", Method: "custom-a", MinInterval: 0, Handle: noopHandle,
	}))
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "custom-a", Method: "custom-a", MinInterval: -time.Second, Handle: noopHandle,
	}))
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "custom-a", Method: "custom-a", MinInterval: time.Second, Handle: nil,
	}))

	// Duplicate Name and duplicate Method are refused.
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "inventory", Method: "other-method", MinInterval: time.Second, Handle: noopHandle,
	}))
	require.Error(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "other-name", Method: fleet.VerbHealthReport, MinInterval: time.Second, Handle: noopHandle,
	}))

	// Valid custom registration succeeds and is discoverable.
	require.NoError(t, RegisterManagedReportReceiver(ManagedReportReceiver{
		Name: "custom-telemetry", Method: "telemetry-report", MinInterval: 5 * time.Second, Handle: noopHandle,
	}))
	got, ok := lookupManagedReportReceiver("telemetry-report")
	require.True(t, ok)
	require.Equal(t, "custom-telemetry", got.Name)

	// MustRegisterManagedReportReceiver panics with BUG prefix on error.
	require.PanicsWithValue(t, `BUG: pluginserver: duplicate report receiver method "telemetry-report"`, func() {
		MustRegisterManagedReportReceiver(ManagedReportReceiver{
			Name: "another-name", Method: "telemetry-report", MinInterval: time.Second, Handle: noopHandle,
		})
	})

	// Snapshot restore removes custom registrations while keeping built-ins.
	restore()
	_, ok = lookupManagedReportReceiver("telemetry-report")
	require.False(t, ok)
	_, ok = lookupManagedReportReceiver(fleet.VerbInventoryReport)
	require.True(t, ok)
	_, ok = lookupManagedReportReceiver(fleet.VerbHealthReport)
	require.True(t, ok)
}
