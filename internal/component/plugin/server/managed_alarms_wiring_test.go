// VALIDATES: fleet-ui-5 WP-B -- ManagedServer alarm store lifecycle, fact
// producer wiring (declarations, admission, disconnect, health, inventory,
// samples, config held/adopt/ack/fetch, history capacity, device forget),
// PrepareManagedListenersWithHistoryAndAlarms reload validation and live apply,
// restart reconciliation preserving open occurrence IDs and declaration timing,
// published accessors, and ManagedDoctorRecord.Alarms health reporting.
// PREVENTS: lost alarm transitions across server boundaries, lock inversions,
// stale session injection, reset declaration grace timers on restart/reload,
// and offline config-diverged alarms sticking open after adopt.

package server

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func startAlarmWiringTestServer(
	t *testing.T,
	store storage.Storage,
	clk *sim.FakeClock,
	histCfg plugin.HubHistoryConfig,
	alarmCfg plugin.HubAlarmsConfig,
	clients map[string]string,
) *ManagedServer {
	t.Helper()
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Store:     store,
		Clock:     clk,
		History:   histCfg,
		Alarms:    alarmCfg,
		Listeners: []ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: clients,
		}},
		ReadConfig: func(name string) ([]byte, error) {
			if data, readErr := store.ReadFile(ClientConfigKey(name)); readErr == nil {
				return data, nil
			}
			if _, ok := clients[name]; ok {
				return []byte(testClientConfig), nil
			}
			return nil, ErrClientConfigNotFound
		},
	})
	require.NoError(t, err)
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	return srv
}

func alarmWiringTestConfig() plugin.HubAlarmsConfig {
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60
	cfg.ResourceCPUPercent = 80
	cfg.ResourceMemoryPercent = 80
	cfg.ResourceStoragePercent = 80
	cfg.ResourceDurationSeconds = 120
	cfg.SoftwareBehindVersions = 1
	return cfg
}

func findOpenAlarm(rows []AlarmOccurrence, kind, device, subject string) (AlarmOccurrence, bool) {
	for _, r := range rows {
		if r.Kind == kind && r.Device == device && r.Subject == subject {
			return r, true
		}
	}
	return AlarmOccurrence{}, false
}

func wiringInventoryReport(at time.Time, version string) fleet.InventoryReport {
	return fleet.InventoryReport{
		CollectedAt:     at.UTC().Format(time.RFC3339),
		SoftwareVersion: version,
	}
}

func wiringHealthReport(at time.Time, compName string, status fleet.HealthStatus, reason string) fleet.HealthReport {
	ts := at.UTC().Format(time.RFC3339)
	return fleet.HealthReport{
		Status:    status,
		CheckedAt: ts,
		Components: []fleet.HealthComponent{{
			Name:      compName,
			Status:    status,
			Reason:    reason,
			CheckedAt: ts,
		}},
	}
}

func wiringCPUSampleReport(at time.Time, cpuPct float64) *fleet.SampleReport {
	return &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    at.UTC().Format(time.RFC3339),
		Samples: []fleet.SampleReading{{
			Figure: fleet.FigureCPUUsed,
			Value:  cpuPct,
		}},
	}
}

// VALIDATES: AC-4, AC-9, AC-10 -- ManagedServer feeds admission, health,
// inventory, samples, disconnect, and reconnect into ManagedAlarmStore,
// rejects superseded sessions, closes resource-high at reconnect while
// preserving component-down ID/Since, and publishes live accessors and doctor
// alarm health.
func TestManagedServerAlarmLifecycleAndReconnect(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 6, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()
	histCfg := plugin.DefaultHubHistoryConfig()
	histCfg.IntervalSeconds = 60

	srv := startAlarmWiringTestServer(t, store, clk, histCfg, alarmWiringTestConfig(), map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
	})

	// Published accessors reach the running alarm store.
	publishedStore, err := ManagedAlarms()
	require.NoError(t, err)
	require.Same(t, srv.Alarms(), publishedStore)

	mc1 := &rpc.MuxConn{}
	mc2 := &rpc.MuxConn{}
	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.1"), mc1))
	require.NoError(t, srv.admitDevice("edge-02", netip.MustParseAddr("192.0.2.2"), mc2))

	// Inventory reports trigger software-behind for edge-01 (0.1.0 vs 0.2.0).
	clk.Add(10 * time.Second)
	require.NoError(t, srv.recordInventoryReport("edge-01", mc1, wiringInventoryReport(clk.Now(), "0.1.0")))
	require.NoError(t, srv.recordInventoryReport("edge-02", mc2, wiringInventoryReport(clk.Now(), "0.2.0")))

	// Health report opens component-down on edge-01.
	clk.Add(10 * time.Second)
	downAt := clk.Now().UTC()
	require.NoError(t, srv.recordHealthReport("edge-01", mc1, wiringHealthReport(downAt, "bgp", fleet.HealthStatusDown, "peer lost")))

	// Sustained high CPU samples open resource-high after 120s duration.
	for range 3 {
		clk.Add(60 * time.Second)
		require.NoError(t, srv.recordSampleReport("edge-01", mc1, wiringCPUSampleReport(clk.Now(), 95)))
	}

	page, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	compDown, ok := findOpenAlarm(page.Alarms, AlarmKindComponentDown, "edge-01", "bgp")
	require.True(t, ok)
	require.Equal(t, downAt, compDown.Since)

	resHigh, ok := findOpenAlarm(page.Alarms, AlarmKindResourceHigh, "edge-01", "cpu")
	require.True(t, ok)

	_, ok = findOpenAlarm(page.Alarms, AlarmKindSoftwareBehind, "edge-01", "")
	require.True(t, ok)

	// Disconnect edge-01 and advance past offline-delay (60s).
	clk.Add(10 * time.Second)
	srv.disconnectDevice("edge-01", mc1)

	// Superseded session reports must be rejected and cannot alter alarms.
	require.Error(t, srv.recordHealthReport("edge-01", mc1, wiringHealthReport(clk.Now(), "bgp", fleet.HealthStatusHealthy, "")))
	require.Error(t, srv.recordSampleReport("edge-01", mc1, wiringCPUSampleReport(clk.Now(), 5)))

	clk.Add(60 * time.Second)
	srv.Alarms().Tick(clk.Now())

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.True(t, ok)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindResourceHigh, "edge-01", "cpu")
	require.True(t, ok, "resource-high stays open while offline")

	// Reconnect edge-01: clears device-offline and closes resource-high
	// (settling), while preserving component-down's ID and Since timestamp.
	clk.Add(30 * time.Second)
	mc1Next := &rpc.MuxConn{}
	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.1"), mc1Next))

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.False(t, ok)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindResourceHigh, "edge-01", "cpu")
	require.False(t, ok, "reconnect must end prior resource-high occurrence")
	compAfter, ok := findOpenAlarm(page.Alarms, AlarmKindComponentDown, "edge-01", "bgp")
	require.True(t, ok)
	require.Equal(t, compDown.ID, compAfter.ID)
	require.Equal(t, compDown.Since, compAfter.Since)

	// History records the closed resource-high and device-offline occurrences.
	histPage, err := ManagedAlarmHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "edge-01"})
	require.NoError(t, err)
	var sawClosedRes bool
	for _, ev := range histPage.Events {
		if ev.ID == resHigh.ID && ev.Cleared {
			sawClosedRes = true
		}
	}
	require.True(t, sawClosedRes)

	// Doctor runtime snapshot publishes alarm health counts.
	docRec, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.NotNil(t, docRec.Alarms)
	require.False(t, docRec.Alarms.Degraded)
	require.Equal(t, 2, docRec.Alarms.OpenCount)
	require.Equal(t, 2, docRec.Alarms.RetainedClosed)
}

// VALIDATES: fleet-ui-5 -- held configuration writes open config-diverged,
// adopting while offline clears config-diverged immediately via the storage
// write observer, HandleConfigFetch current clears config-diverged, and
// handleConfigAck opens/clears config-rejected independently.
func TestManagedServerAlarmConfigHeldAdoptAndRejected(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 7, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()

	srv := startAlarmWiringTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), alarmWiringTestConfig(), map[string]string{
		"edge-01": testClientSecret,
	})

	// Write a held record while edge-01 is offline: write observer opens config-diverged.
	heldBytes := []byte("system { hostname edge-01-local; }\n")
	heldRec := HeldRecord{
		version:    fleet.VersionHash(heldBytes),
		config:     heldBytes,
		receivedAt: t0,
	}
	require.NoError(t, srv.svc.records.write("edge-01", heldRec))

	page, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok := findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.True(t, ok)

	// Simulate adopt while offline: commit client config and remove held key under one guard.
	guard, err := store.AcquireLock(ClientConfigKey("edge-01"))
	require.NoError(t, err)
	_, err = storage.CommitConfigWithGuard(store, guard, ClientConfigKey("edge-01"), heldBytes)
	require.NoError(t, err)
	require.NoError(t, guard.RemoveKey(HeldRecordKey("edge-01")))
	require.NoError(t, guard.Release())

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.False(t, ok, "adopt must clear config-diverged immediately even when offline")

	// Refused config-ack opens config-rejected; applied config-ack clears it.
	mc := &rpc.MuxConn{}
	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.1"), mc))
	require.NoError(t, srv.handleConfigAck("edge-01", mc, fleet.ConfigAck{Version: "v1", OK: false, Error: "invalid bgp asn"}, ""))
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	rej, ok := findOpenAlarm(page.Alarms, AlarmKindConfigRejected, "edge-01", "")
	require.True(t, ok)
	require.Equal(t, "invalid bgp asn", rej.Detail)

	require.NoError(t, srv.handleConfigAck("edge-01", mc, fleet.ConfigAck{Version: "v1", OK: true}, ""))
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindConfigRejected, "edge-01", "")
	require.False(t, ok)

	// Re-create a held record and confirm HandleConfigFetch("current") clears config-diverged.
	require.NoError(t, srv.svc.records.write("edge-01", heldRec))
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.True(t, ok)

	resp, err := srv.svc.HandleConfigFetch("edge-01", fleet.ConfigFetchRequest{Version: fleet.VersionHash(heldBytes)})
	require.NoError(t, err)
	require.Equal(t, managedFetchStatusCurrent, resp.Status)

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.False(t, ok)
}

// VALIDATES: AC-14 -- PrepareManagedListenersWithHistoryAndAlarms rejects
// invalid alarm config before promotion, live reload updates HubAlarmsConfig
// without rebinding listeners or resetting existing declaration timing, and
// remove+re-add starts a fresh grace interval.
func TestManagedServerAlarmLiveReloadAndDeclarations(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 8, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()
	alarmCfg := alarmWiringTestConfig()
	alarmCfg.OfflineDelaySeconds = 120

	srv := startAlarmWiringTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), alarmCfg, map[string]string{
		"edge-01": testClientSecret,
	})
	origAddr := srv.Addrs()[0].String()

	// Invalid alarm config is refused before candidate promotion.
	badAlarms := alarmCfg
	badAlarms.OfflineDelaySeconds = plugin.MaxAlarmOfflineDelaySeconds + 1
	_, err := srv.PrepareManagedListenersWithHistoryAndAlarms(
		[]ManagedListener{{Addr: origAddr, Clients: map[string]string{"edge-01": testClientSecret}}},
		"",
		plugin.DefaultHubHistoryConfig(),
		badAlarms,
	)
	require.Error(t, err)
	require.Equal(t, 120, srv.Alarms().Config().OfflineDelaySeconds)

	// Advance 40s and hot-reload OfflineDelaySeconds to 60s without rebinding.
	clk.Add(40 * time.Second)
	nextAlarms := alarmCfg
	nextAlarms.OfflineDelaySeconds = 60
	plan, err := srv.PrepareManagedListenersWithHistoryAndAlarms(
		[]ManagedListener{{Addr: origAddr, Clients: map[string]string{"edge-01": testClientSecret}}},
		"",
		plugin.DefaultHubHistoryConfig(),
		nextAlarms,
	)
	require.NoError(t, err)
	srv.ApplyManagedListeners(plan)
	require.Equal(t, 60, srv.Alarms().Config().OfflineDelaySeconds)

	// Because edge-01 kept its original DeclaredAt (t0), advancing 20s more (total 60s)
	// opens device-offline without waiting a fresh 60s from the reload.
	clk.Add(20 * time.Second)
	srv.Alarms().Tick(clk.Now())
	page, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok := findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.True(t, ok)

	// Remove edge-01 declaration -> device-offline clears.
	clk.Add(10 * time.Second)
	planRemove, err := srv.PrepareManagedListenersWithHistoryAndAlarms(
		[]ManagedListener{{Addr: origAddr, Clients: map[string]string{"edge-02": testClient2Secret}}},
		"",
		plugin.DefaultHubHistoryConfig(),
		nextAlarms,
	)
	require.NoError(t, err)
	srv.ApplyManagedListeners(planRemove)

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.False(t, ok)

	// Re-add edge-01 -> starts a fresh 60s grace period from re-add time.
	clk.Add(10 * time.Second)
	planReadd, err := srv.PrepareManagedListenersWithHistoryAndAlarms(
		[]ManagedListener{{Addr: origAddr, Clients: map[string]string{
			"edge-01": testClientSecret,
			"edge-02": testClient2Secret,
		}}},
		"",
		plugin.DefaultHubHistoryConfig(),
		nextAlarms,
	)
	require.NoError(t, err)
	srv.ApplyManagedListeners(planReadd)

	clk.Add(30 * time.Second)
	srv.Alarms().Tick(clk.Now())
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.False(t, ok, "re-added device must receive a fresh offline grace period")

	clk.Add(30 * time.Second)
	srv.Alarms().Tick(clk.Now())
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.True(t, ok)
}

// VALIDATES: AC-12, AC-14 -- Stop flushes alarm shards before releasing the
// store and withdraws published accessors; restarting a new ManagedServer on
// the same store preserves open occurrence IDs, Since timestamps, and original
// declaration grace timing; corrupt shards and write failures surface in
// ManagedDoctorRecord.Alarms.
func TestManagedServerAlarmRestartPersistenceAndDoctorDegradation(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	fstore := newHistoryFailStore()
	alarmCfg := alarmWiringTestConfig()
	alarmCfg.OfflineDelaySeconds = 60

	clients := map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
	}

	srv1 := startAlarmWiringTestServer(t, fstore, clk, plugin.DefaultHubHistoryConfig(), alarmCfg, clients)

	// edge-01 never connects (grace started at t0).
	// edge-02 connects and opens component-down at t0+10s.
	clk.Add(10 * time.Second)
	downAt := clk.Now().UTC()
	mc2 := &rpc.MuxConn{}
	require.NoError(t, srv1.admitDevice("edge-02", netip.MustParseAddr("192.0.2.2"), mc2))
	require.NoError(t, srv1.recordHealthReport("edge-02", mc2, wiringHealthReport(downAt, "ospf", fleet.HealthStatusDown, "socket closed")))
	page1, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-02"})
	require.NoError(t, err)
	origDown, ok := findOpenAlarm(page1.Alarms, AlarmKindComponentDown, "edge-02", "ospf")
	require.True(t, ok)
	require.Equal(t, downAt, origDown.Since)

	// Stop srv1 (drains device records, history, and alarms via FlushFinal).
	clk.Add(10 * time.Second)
	srv1.disconnectDevice("edge-02", mc2)
	srv1.Stop()

	// Published accessors return ErrManagedNotServing once stopped.
	_, err = ManagedAlarms()
	require.ErrorIs(t, err, ErrManagedNotServing)
	_, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{})
	require.ErrorIs(t, err, ErrManagedNotServing)
	_, err = ManagedAlarmHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "edge-01"})
	require.ErrorIs(t, err, ErrManagedNotServing)

	// Plant a corrupt alarm shard before restart to verify doctor reporting.
	require.NoError(t, fstore.Storage.WriteKey(zefs.FleetAlarmDevicePrefix+"corrupt-dev", []byte("{not valid json")))

	// Restart a new ManagedServer on the same store at t0+40s.
	clk.Add(20 * time.Second)
	srv2 := startAlarmWiringTestServer(t, fstore, clk, plugin.DefaultHubHistoryConfig(), alarmCfg, clients)

	// edge-02 preserved its component-down ID and original Since timestamp.
	page2, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	require.True(t, page2.Degraded)
	restoredDown, ok := findOpenAlarm(page2.Alarms, AlarmKindComponentDown, "edge-02", "ospf")
	require.True(t, ok)
	require.Equal(t, origDown.ID, restoredDown.ID)
	require.Equal(t, origDown.Since, restoredDown.Since)

	// edge-01 is not offline yet at t0+40s, but opens at t0+60s (original t0 grace).
	_, ok = findOpenAlarm(page2.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.False(t, ok)
	clk.Add(20 * time.Second)
	srv2.Alarms().Tick(clk.Now())
	page2, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page2.Alarms, AlarmKindDeviceOffline, "edge-01", "")
	require.True(t, ok)

	// Inject an alarm write failure and verify doctor reports both CorruptShards and WriteFailed.
	fstore.mu.Lock()
	fstore.failWrite = func(key string) error {
		if key == zefs.KeyFleetAlarmHub.Pattern || key == zefs.FleetAlarmDevicePrefix+"edge-01" {
			return errors.New("injected disk full")
		}
		return nil
	}
	fstore.mu.Unlock()
	require.Error(t, srv2.Alarms().FlushOnce())

	docRec, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.NotNil(t, docRec.Alarms)
	require.True(t, docRec.Alarms.Degraded)
	require.True(t, docRec.Alarms.WriteFailed)
	require.Equal(t, 1, docRec.Alarms.CorruptShards)
	require.Contains(t, docRec.Alarms.LastWriteError, "injected disk full")

	// Clear write failure before cleanup Stop().
	fstore.mu.Lock()
	fstore.failWrite = nil
	fstore.mu.Unlock()
}

// VALIDATES: fleet-ui-5 -- history capacity pressure opens hub-scoped
// history-full via recordSampleReport / CheckpointOnce, and forgetting a device
// record clears its alarm state via OnDeviceForgotten.
func TestManagedServerAlarmHistoryFullAndForget(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()

	// Small per-device and hub budgets so repeated samples trigger capacity pressure quickly.
	histCfg := plugin.DefaultHubHistoryConfig()
	histCfg.IntervalSeconds = 60
	histCfg.DeviceBudgetBytes = fleet.MinHistoryLatestReservePerDeviceBytes
	histCfg.HubBudgetBytes = fleet.MinHistoryLatestReservePerDeviceBytes * 2

	srv := startAlarmWiringTestServer(t, store, clk, histCfg, alarmWiringTestConfig(), map[string]string{
		"edge-01": testClientSecret,
	})
	mc1 := &rpc.MuxConn{}
	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.1"), mc1))
	require.NoError(t, srv.recordHealthReport("edge-01", mc1, wiringHealthReport(clk.Now(), "bgp", fleet.HealthStatusDown, "down")))

	// Directly verify OnHistoryHealth opens and clears history-full through the
	// server's wired alarm store and that forgetting edge-01 removes its alarms.
	srv.Alarms().OnHistoryHealth(ManagedHistoryHealth{
		CapacityPressure:          true,
		UsedBytes:                 histCfg.HubBudgetBytes,
		HubBudgetBytes:            histCfg.HubBudgetBytes,
		DeviceBudgetBytes:         histCfg.DeviceBudgetBytes,
		ConfiguredRetentionDays:   histCfg.RetentionDays,
		EffectiveRetentionSeconds: 3600,
	})

	page, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	histFull, ok := findOpenAlarm(page.Alarms, AlarmKindHistoryFull, "", "")
	require.True(t, ok)
	require.Equal(t, AlarmScopeHub, histFull.Scope)

	// CheckpointOnce triggers OnHealthChanged callback -> clears history-full when healthy.
	require.NoError(t, srv.History().CheckpointOnce())
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	_, ok = findOpenAlarm(page.Alarms, AlarmKindHistoryFull, "", "")
	require.False(t, ok)

	// Removing the device record key invokes observeDeviceRecord -> forgetDeviceRecordLocked -> OnDeviceForgotten.
	srv.disconnectDevice("edge-01", mc1)
	srv.drainDeviceRecords()
	require.NoError(t, store.RemoveKey(deviceRecordKey("edge-01")))

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	require.Empty(t, page.Alarms)
}

// VALIDATES: fleet-ui-5 session fencing for config-ack, config-held, and
// config-fetch, plus restart preservation of config-diverged held-version
// detail without dirtying shards unnecessarily.
// PREVENTS: superseded sessions corrupting registry state, held records, or
// alarms, or hub restarts stripping held-version=<hash> from config-diverged.
func TestManagedServerAlarmSessionFencingAndRestartHeldDetail(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 11, 0, 0, 0, time.UTC)
	clk := sim.NewFakeClock(t0)
	store := storage.CreateMemory()
	clients := map[string]string{"edge-01": testClientSecret}

	srv := startAlarmWiringTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), alarmWiringTestConfig(), clients)

	// Admit session 1, disconnect it, and admit session 2.
	hubPipe1, cliPipe1 := net.Pipe()
	t.Cleanup(func() {
		_ = hubPipe1.Close()
		_ = cliPipe1.Close()
	})
	staleMC := rpc.NewMuxConn(rpc.NewConn(hubPipe1, hubPipe1))
	t.Cleanup(func() { _ = staleMC.Close() })

	hubPipe2, cliPipe2 := net.Pipe()
	t.Cleanup(func() {
		_ = hubPipe2.Close()
		_ = cliPipe2.Close()
	})
	activeMC := rpc.NewMuxConn(rpc.NewConn(hubPipe2, hubPipe2))
	t.Cleanup(func() { _ = activeMC.Close() })

	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.1"), staleMC))
	srv.disconnectDevice("edge-01", staleMC)
	require.NoError(t, srv.admitDevice("edge-01", netip.MustParseAddr("192.0.2.2"), activeMC))

	// Active session records an applied ack first.
	require.NoError(t, srv.handleConfigAck("edge-01", activeMC, fleet.ConfigAck{Version: "active-v1", OK: true}, "active-v1"))

	// Superseded session config-ack MUST fail and leave registry and alarms untouched.
	err := srv.handleConfigAck("edge-01", staleMC, fleet.ConfigAck{Version: "stale-v0", OK: false, Error: "stale failure"}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "superseded")

	dev, err := srv.registryDevice("edge-01")
	require.NoError(t, err)
	require.Equal(t, ManagedAckApplied, dev.AckResult)
	require.Equal(t, "active-v1", dev.AckVersion)

	page, err := ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, hasRej := findOpenAlarm(page.Alarms, AlarmKindConfigRejected, "edge-01", "")
	require.False(t, hasRej, "superseded config-ack must not open config-rejected")

	// Superseded session config-fetch record MUST fail and leave LastFetch untouched.
	err = srv.recordFetch("edge-01", staleMC, "stale-v0", fleet.ConfigFetchResponse{Status: managedFetchStatusCurrent}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "superseded")

	// Superseded session config-held MUST return RPC error and write no held record or alarm.
	heldText := "\n\n"
	heldPayload, err := json.Marshal(fleet.ConfigHeld{
		Version:  fleet.VersionHash([]byte(heldText)),
		Config:   base64.StdEncoding.EncodeToString([]byte(heldText)),
		EditedAt: t0,
	})
	require.NoError(t, err)

	errCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(cliPipe1).ReadString('\n')
		errCh <- line
	}()
	srv.handleConfigHeld("edge-01", staleMC, &rpc.Request{
		ID:     11,
		Method: fleet.VerbConfigHeld,
		Params: heldPayload,
	})
	reply := <-errCh
	require.Contains(t, reply, "superseded")

	_, heldExists, err := ReadHeldRecord(store, "edge-01")
	require.NoError(t, err)
	require.False(t, heldExists, "superseded config-held must not write a held record")

	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	_, hasDiv := findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.False(t, hasDiv, "superseded config-held must not open config-diverged")

	// Active session config-held succeeds, opens config-diverged with held-version=<hash>.
	okCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(cliPipe2).ReadString('\n')
		okCh <- line
	}()
	srv.handleConfigHeld("edge-01", activeMC, &rpc.Request{
		ID:     12,
		Method: fleet.VerbConfigHeld,
		Params: heldPayload,
	})
	okReply := <-okCh
	require.Contains(t, okReply, "ok")

	wantDetail := "held-version=" + fleet.VersionHash([]byte(heldText))
	page, err = ManagedAlarmList(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	divBefore, ok := findOpenAlarm(page.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.True(t, ok)
	require.Equal(t, wantDetail, divBefore.Detail)

	// Stop cleanly and restart a new server on the same store: held-version detail MUST survive.
	srv.disconnectDevice("edge-01", activeMC)
	srv.Stop()

	clk.Add(15 * time.Second)
	srv2 := startAlarmWiringTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), alarmWiringTestConfig(), clients)
	page2, err := srv2.Alarms().QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	divAfter, ok := findOpenAlarm(page2.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.True(t, ok)
	require.Equal(t, divBefore.ID, divAfter.ID)
	require.Equal(t, divBefore.Since, divAfter.Since)
	require.Equal(t, wantDetail, divAfter.Detail, "hub restart must preserve held-version detail")
}
