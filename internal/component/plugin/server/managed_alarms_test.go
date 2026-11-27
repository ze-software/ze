// Design: docs/architecture/fleet-alarms.md -- unit tests for hub alarm engine, registry, persistence, and queries
// Related: managed_alarms.go -- ManagedAlarmStore engine and query continuation
// Related: managed_alarms_kinds.go -- open alarm kind registry and built-in kinds
// Related: managed_alarms_version.go -- deterministic software version ordering
// Related: managed_alarms_store.go -- durable shard persistence and retention

package server

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

type alarmFakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newAlarmFakeClock(t0 time.Time) *alarmFakeClock {
	return &alarmFakeClock{now: t0.UTC()}
}

func (c *alarmFakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *alarmFakeClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}

func (c *alarmFakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t.UTC()
}

type alarmFailStore struct {
	storage.Storage
	mu        sync.Mutex
	failWrite func(key string) error
}

func newAlarmFailStore() *alarmFailStore {
	return &alarmFailStore{Storage: storage.CreateMemory()}
}

func (f *alarmFailStore) WriteKey(key string, data []byte) error {
	f.mu.Lock()
	fn := f.failWrite
	f.mu.Unlock()
	if fn != nil {
		if err := fn(key); err != nil {
			return err
		}
	}
	return f.Storage.WriteKey(key, data)
}

func (f *alarmFailStore) setFailWrite(fn func(key string) error) {
	f.mu.Lock()
	f.failWrite = fn
	f.mu.Unlock()
}

func makeAlarmHealthSnap(at time.Time, comps ...fleet.HealthComponent) ManagedHealthSnapshot {
	overall := fleet.HealthStatusHealthy
	checked := at.UTC().Format(time.RFC3339)
	for i := range comps {
		if comps[i].CheckedAt == "" {
			comps[i].CheckedAt = checked
		}
		switch comps[i].Status {
		case fleet.HealthStatusDown:
			overall = fleet.HealthStatusDown
		case fleet.HealthStatusDegraded:
			if overall != fleet.HealthStatusDown {
				overall = fleet.HealthStatusDegraded
			}
		case fleet.HealthStatusHealthy, fleet.HealthStatusUnspecified:
			// No escalation.
		default:
			// Open enum.
		}
	}
	return ManagedHealthSnapshot{
		Report: fleet.HealthReport{
			Status:     overall,
			Components: comps,
			CheckedAt:  checked,
		},
		ReceivedAt: at.UTC(),
	}
}

func makeAlarmInventorySnap(at time.Time, version string) ManagedInventorySnapshot {
	return ManagedInventorySnapshot{
		Report: fleet.InventoryReport{
			CollectedAt:     at.UTC().Format(time.RFC3339),
			SoftwareVersion: version,
		},
		ReceivedAt: at.UTC(),
	}
}

func makeAlarmSampleReport(readings ...fleet.SampleReading) fleet.SampleReport {
	return fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       readings,
	}
}

func hasActiveReportWarning(code string) bool {
	for _, w := range report.Warnings() {
		if w.Code == code {
			return true
		}
	}
	return false
}

// VALIDATES: AC-4 and AC-9 — two failing components on one device open two
// distinct occurrence IDs, repeated identical reports and queries are strictly
// idempotent, degraded-to-down transitions clear the old kind once, clears are
// recorded once, and matching component names on distinct devices remain
// independent.
// PREVENTS: duplicate history entries on polling/repeats, unstable occurrence
// IDs, or cross-device subject collisions.
func TestAlarmComponentLifecycleAndIdempotency(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: storage.CreateMemory(),
		Now:   clk.Now,
	})
	store.SyncDeclarations(map[string]bool{"r1": true, "r2": true})
	store.OnDeviceConnected("r1", t0)
	store.OnDeviceConnected("r2", t0)

	// Two components fail on r1: bgp down (critical) and ospf degraded (warning).
	t1 := clk.Advance(10 * time.Second)
	snap1 := makeAlarmHealthSnap(t1,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "session flap"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDegraded, Reason: "neighbor slow"},
	)
	store.OnHealthReport("r1", snap1)

	page1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	require.Equal(t, 2, page1.Total)
	require.Equal(t, 1, page1.Counts.Critical)
	require.Equal(t, 1, page1.Counts.Warning)
	require.Equal(t, 2, page1.Counts.Visible)
	require.Equal(t, 0, page1.Counts.Hidden)

	// Critical sorts before warning.
	bgpOcc := page1.Alarms[0]
	ospfOcc := page1.Alarms[1]
	assert.Equal(t, AlarmKindComponentDown, bgpOcc.Kind)
	assert.Equal(t, "bgp", bgpOcc.Subject)
	assert.Equal(t, t1, bgpOcc.Since)
	assert.Equal(t, AlarmKindComponentDegraded, ospfOcc.Kind)
	assert.Equal(t, "ospf", ospfOcc.Subject)
	assert.Equal(t, t1, ospfOcc.Since)
	assert.NotEqual(t, bgpOcc.ID, ospfOcc.ID)

	// Repeated identical health report at t2 preserves IDs and Since.
	t2 := clk.Advance(30 * time.Second)
	snap2 := makeAlarmHealthSnap(t2,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "session flap"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDegraded, Reason: "neighbor slow"},
	)
	store.OnHealthReport("r1", snap2)

	// Repeated reads produce zero transitions.
	for range 3 {
		p, qErr := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "r1"})
		require.NoError(t, qErr)
		require.Len(t, p.Alarms, 2)
		assert.Equal(t, bgpOcc.ID, p.Alarms[0].ID)
		assert.Equal(t, t1, p.Alarms[0].Since)
		assert.Equal(t, ospfOcc.ID, p.Alarms[1].ID)
		assert.Equal(t, t1, p.Alarms[1].Since)
	}
	hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "r1"})
	require.NoError(t, err)
	require.Equal(t, 2, hist.Total, "repeated report and reads must not create extra history rows")

	// Distinct device r2 with same component name "bgp" down stays distinct.
	t3 := clk.Advance(5 * time.Second)
	store.OnHealthReport("r2", makeAlarmHealthSnap(t3,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "peer down"},
	))
	pageAll, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindComponentDown})
	require.NoError(t, err)
	require.Equal(t, 2, pageAll.Total)
	assert.Equal(t, "r1", pageAll.Alarms[0].Device)
	assert.Equal(t, "r2", pageAll.Alarms[1].Device)
	assert.NotEqual(t, pageAll.Alarms[0].ID, pageAll.Alarms[1].ID)

	// Transition ospf on r1 from degraded to down: clears degraded, opens down.
	t4 := clk.Advance(15 * time.Second)
	store.OnHealthReport("r1", makeAlarmHealthSnap(t4,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "session flap"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDown, Reason: "process stopped"},
	))
	pR1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "r1"})
	require.NoError(t, err)
	require.Equal(t, 2, pR1.Total)
	assert.Equal(t, AlarmKindComponentDown, pR1.Alarms[0].Kind)
	assert.Equal(t, bgpOcc.ID, pR1.Alarms[0].ID)
	assert.Equal(t, AlarmKindComponentDown, pR1.Alarms[1].Kind)
	assert.Equal(t, "ospf", pR1.Alarms[1].Subject)
	assert.NotEqual(t, ospfOcc.ID, pR1.Alarms[1].ID)
	assert.Equal(t, t4, pR1.Alarms[1].Since)

	// Clear both components on r1; repeat healthy report must not duplicate clears.
	t5 := clk.Advance(20 * time.Second)
	store.OnHealthReport("r1", makeAlarmHealthSnap(t5,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusHealthy},
	))
	t6 := clk.Advance(20 * time.Second)
	store.OnHealthReport("r1", makeAlarmHealthSnap(t6,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusHealthy},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusHealthy},
	))

	pR1After, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "r1"})
	require.NoError(t, err)
	assert.Equal(t, 0, pR1After.Total)

	// r2's bgp occurrence is unaffected.
	pR2After, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "r2"})
	require.NoError(t, err)
	assert.Equal(t, 1, pR2After.Total)

	// History for r1 has exactly 3 closed episodes (bgp down, ospf degraded, ospf down), newest Opened first.
	histAfter, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "r1"})
	require.NoError(t, err)
	require.Equal(t, 3, histAfter.Total)
	assert.Equal(t, t4, histAfter.Events[0].Opened)
	assert.True(t, histAfter.Events[0].Cleared)
	assert.Equal(t, t5, histAfter.Events[0].ClearedAt)
	for _, ev := range histAfter.Events {
		assert.True(t, ev.Cleared)
		if ev.ID == ospfOcc.ID {
			assert.Equal(t, t4, ev.ClearedAt)
		}
		if ev.ID == bgpOcc.ID {
			assert.Equal(t, t5, ev.ClearedAt)
		}
	}
}

// VALIDATES: AC-10 — resource-high opens only after continuous samples above
// threshold for ResourceDuration, ignores missing figures or zero totals,
// resets on below-threshold readings, preserves open occurrences on disconnect
// without advancing pending timers, closes old resource-high on reconnect while
// keeping component-down open, and requires fresh post-reconnect samples to
// open a new occurrence ID.
// PREVENTS: retained high samples or offline gaps opening false resource alarms
// immediately on reconnect, or reconnect accidentally clearing component-down.
func TestAlarmResourceHighDisconnectReconnect(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 11, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.ResourceCPUPercent = 90
	cfg.ResourceMemoryPercent = 85
	cfg.ResourceStoragePercent = 80
	cfg.ResourceDurationSeconds = 120

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  storage.CreateMemory(),
		Now:    clk.Now,
	})
	store.SyncDeclarations(map[string]bool{"edge-1": true})
	store.OnDeviceConnected("edge-1", t0)

	// Missing CPU and zero memory total must not trigger or advance qualification.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 100},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 0},
	), t0.Add(10*time.Second))
	p, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, p.Total)

	// Start CPU qualification at t0+20s, then drop below threshold at t0+80s to reset timer.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), t0.Add(20*time.Second))
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 80},
	), t0.Add(80*time.Second))
	// High again at t0+150s must start a fresh timer (not count from t0+20s).
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), t0.Add(150*time.Second))
	p, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1"})
	require.NoError(t, err)
	assert.Equal(t, 0, p.Total, "below-threshold sample must reset qualification timer")

	// Start memory qualification at t0+200s, then disconnect at t0+250s before 120s elapses.
	// Also open CPU at t0+270s (150s + 120s) and component-down at t0+260s first.
	tCompDown := t0.Add(260 * time.Second)
	store.OnHealthReport("edge-1", makeAlarmHealthSnap(tCompDown,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
	))
	tCPUOpen := t0.Add(270 * time.Second)
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 94},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 900},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 500},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), tCPUOpen)

	p, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1"})
	require.NoError(t, err)
	require.Equal(t, 2, p.Total, "expected bgp component-down and cpu resource-high (memory just started)")
	compOcc := p.Alarms[0]
	cpuOcc1 := p.Alarms[1]
	assert.Equal(t, AlarmKindComponentDown, compOcc.Kind)
	assert.Equal(t, tCompDown, compOcc.Since)
	assert.Equal(t, AlarmKindResourceHigh, cpuOcc1.Kind)
	assert.Equal(t, "cpu", cpuOcc1.Subject)
	assert.Equal(t, tCPUOpen, cpuOcc1.Since, "resource-high must open at qualification time")

	// Disconnect at t0+300s: cpu resource-high and bgp component-down stay open;
	// pending memory qualification timer is cleared.
	tDisc := t0.Add(300 * time.Second)
	store.OnDeviceDisconnected("edge-1", tDisc)

	// Samples while disconnected are ignored.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 950},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
	), t0.Add(500*time.Second))

	pDisc, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1"})
	require.NoError(t, err)
	require.Equal(t, 2, pDisc.Total)
	assert.Equal(t, compOcc.ID, pDisc.Alarms[0].ID)
	assert.Equal(t, cpuOcc1.ID, pDisc.Alarms[1].ID)
	assert.Equal(t, tCPUOpen, pDisc.Alarms[1].Since)

	// Reconnect at t0+600s: old resource-high closes immediately at reconnect,
	// while component-down preserves its original ID and opening time.
	tReconn := t0.Add(600 * time.Second)
	store.OnDeviceConnected("edge-1", tReconn)

	pReconn, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1"})
	require.NoError(t, err)
	require.Equal(t, 1, pReconn.Total)
	assert.Equal(t, compOcc.ID, pReconn.Alarms[0].ID)
	assert.Equal(t, tCompDown, pReconn.Alarms[0].Since)

	// Stale sample predating reconnect must be ignored.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 99},
	), tReconn.Add(-5*time.Second))

	// Fresh post-reconnect sample at tReconn+10s starts a new qualification timer (does not immediately open).
	tFresh1 := tReconn.Add(10 * time.Second)
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 96},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 850},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), tFresh1)
	pFresh1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1", Kind: AlarmKindResourceHigh})
	require.NoError(t, err)
	assert.Equal(t, 0, pFresh1.Total, "first post-reconnect high sample must not immediately reopen")

	// Qualifying post-reconnect sample at tFresh1+120s opens NEW resource-high occurrences with new IDs.
	tFresh2 := tFresh1.Add(120 * time.Second)
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 96},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 850},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), tFresh2)
	pFresh2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1", Kind: AlarmKindResourceHigh})
	require.NoError(t, err)
	require.Equal(t, 2, pFresh2.Total)
	assert.Equal(t, "cpu", pFresh2.Alarms[0].Subject)
	assert.NotEqual(t, cpuOcc1.ID, pFresh2.Alarms[0].ID)
	assert.Equal(t, tFresh2, pFresh2.Alarms[0].Since)
	assert.Equal(t, "storage", pFresh2.Alarms[1].Subject)
	assert.Equal(t, tFresh2, pFresh2.Alarms[1].Since)

	// History records prior cpu resource-high closed at reconnect plus the new open ones.
	hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "edge-1"})
	require.NoError(t, err)
	var foundClosedCPU bool
	for _, ev := range hist.Events {
		if ev.ID == cpuOcc1.ID {
			foundClosedCPU = true
			assert.True(t, ev.Cleared)
			assert.Equal(t, tCPUOpen, ev.Opened)
			assert.Equal(t, tReconn, ev.ClearedAt)
		}
	}
	assert.True(t, foundClosedCPU, "prior resource-high must be recorded closed at reconnect")
}

// VALIDATES: AC-11 — relay-not-granted is NOT registered in first release, the
// open kind registry rejects malformed or duplicate registrations, and a custom
// registered kind modeling the 4 grant states (no provider, unknown, granted,
// known-absent including offline) opens only on known-absent and stays visible
// while offline.
// PREVENTS: hardcoding fleet-9 relay alarms in first release or silently
// accepting duplicate/invalid alarm kinds.
func TestAlarmOpenKindRegistrationAndRelaySeam(t *testing.T) {
	// Verify default registry has the 8 first-release kinds and NOT relay-not-granted.
	_, hasRelay := LookupAlarmKind("relay-not-granted")
	assert.False(t, hasRelay, "relay-not-granted must not be registered before fleet-9")
	assert.Len(t, RegisteredAlarmKinds(), 8)

	// Validation refusals on an isolated registry.
	reg := defaultAlarmKindRegistry.Clone()
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "", Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "bad-sev", Severity: AlarmSeverityUnspecified, Scope: AlarmScopeDevice}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "bad-scope", Severity: AlarmSeverityWarning, Scope: AlarmScopeUnspecified}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "hub-with-cause", Severity: AlarmSeverityWarning, Scope: AlarmScopeHub, Causes: []string{AlarmKindDeviceOffline}}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "self-cause", Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Causes: []string{"self-cause"}}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: "dup-cause", Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Causes: []string{AlarmKindDeviceOffline, AlarmKindDeviceOffline}}))
	assert.Error(t, reg.Register(AlarmKindSpec{Name: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice}))

	// Prove the package-level seam with proper SnapshotAlarmKinds cleanup.
	t.Cleanup(SnapshotAlarmKinds())

	type grantState uint8
	const (
		grantNoProvider grantState = iota
		grantUnknown
		grantPresent
		grantKnownAbsent
	)
	var mu sync.Mutex
	states := map[string]grantState{
		"dev-none":    grantNoProvider,
		"dev-unknown": grantUnknown,
		"dev-ok":      grantPresent,
		"dev-absent":  grantKnownAbsent,
	}

	const testKind = "test-grant-absent"
	require.NoError(t, RegisterAlarmKind(AlarmKindSpec{
		Name:     testKind,
		Severity: AlarmSeverityInformational,
		Scope:    AlarmScopeDevice,
		Summary:  "Device has no active grant",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			mu.Lock()
			defer mu.Unlock()
			var out []AlarmObservation
			for _, dev := range ctx.Devices {
				if states[dev.Name] == grantKnownAbsent {
					out = append(out, AlarmObservation{
						Device:     dev.Name,
						Summary:    "Grant explicitly absent",
						Detail:     "provider=test state=absent",
						ObservedAt: ctx.Now,
					})
				}
			}
			return out
		},
	}))
	// Duplicate registration on package registry is refused.
	assert.Error(t, RegisterAlarmKind(AlarmKindSpec{
		Name:     testKind,
		Severity: AlarmSeverityInformational,
		Scope:    AlarmScopeDevice,
	}))

	t0 := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: storage.CreateMemory(),
		Now:   clk.Now,
	})
	store.SyncDeclarations(map[string]bool{
		"dev-none": true, "dev-unknown": true, "dev-ok": true, "dev-absent": true,
	})
	store.OnDeviceConnected("dev-absent", t0)

	store.EvaluateExternalKinds()
	page, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: testKind})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	assert.Equal(t, "dev-absent", page.Alarms[0].Device)
	assert.Equal(t, AlarmSeverityInformational, page.Alarms[0].Severity)
	firstID := page.Alarms[0].ID

	// Disconnect dev-absent: known-absent grant remains visible while offline.
	clk.Advance(30 * time.Second)
	store.OnDeviceDisconnected("dev-absent", clk.Now())
	store.EvaluateExternalKinds()
	pageOffline, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: testKind})
	require.NoError(t, err)
	require.Equal(t, 1, pageOffline.Total)
	assert.Equal(t, firstID, pageOffline.Alarms[0].ID)
	assert.Equal(t, t0, pageOffline.Alarms[0].Since)

	// Grant restored: clears the custom kind.
	clk.Advance(30 * time.Second)
	mu.Lock()
	states["dev-absent"] = grantPresent
	mu.Unlock()
	store.EvaluateExternalKinds()
	pageCleared, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: testKind})
	require.NoError(t, err)
	assert.Equal(t, 0, pageCleared.Total)
}

// VALIDATES: AC-12 — hub restart preserves open occurrence IDs/timestamps and
// closed history, reconciles cleared conditions at observed restart time without
// inventing downtime timestamps, skips corrupt/oversized shards with degraded
// status and report-bus warnings, recovers cleanly from write failures, and
// erases forgotten device shards.
// PREVENTS: lost occurrence identity across restarts, fabricated downtime
// timestamps, crashes on corrupt storage, or stuck dirty state after write
// errors.
func TestAlarmRestartPersistenceAndDegradedRecovery(t *testing.T) {
	t.Cleanup(report.ResetForTest)
	report.ResetForTest()

	t0 := time.Date(2026, 11, 27, 13, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	mem := newAlarmFailStore()

	store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: mem,
		Now:   clk.Now,
	})
	store1.SyncDeclarations(map[string]bool{"r1": true, "r2": true})
	store1.OnDeviceConnected("r1", t0)
	store1.OnDeviceConnected("r2", t0)

	// Open two components on r1, close one, leave bgp open; open history-full on hub.
	t1 := clk.Advance(10 * time.Second)
	store1.OnHealthReport("r1", makeAlarmHealthSnap(t1,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "bgp down"},
		fleet.HealthComponent{Name: "ntp", Status: fleet.HealthStatusDegraded, Reason: "unsynced"},
	))
	store1.OnHistoryHealth(ManagedHistoryHealth{
		CapacityPressure:          true,
		UsedBytes:                 900,
		HubBudgetBytes:            1000,
		EffectiveRetentionSeconds: 3600,
		ConfiguredRetentionDays:   7,
	})

	t2 := clk.Advance(20 * time.Second)
	store1.OnHealthReport("r1", makeAlarmHealthSnap(t2,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "bgp down"},
		fleet.HealthComponent{Name: "ntp", Status: fleet.HealthStatusHealthy},
	))
	store1.OnHealthReport("r2", makeAlarmHealthSnap(t2,
		fleet.HealthComponent{Name: "isis", Status: fleet.HealthStatusDown, Reason: "down"},
	))

	// Simulate write failure first: store remains degraded and live alarms stay queryable.
	mem.setFailWrite(func(string) error { return errors.New("disk full") })
	err := store1.FlushOnce()
	require.Error(t, err)
	hFail := store1.Health()
	assert.True(t, hFail.Degraded)
	assert.True(t, hFail.WriteFailed)
	assert.Contains(t, hFail.LastWriteError, "disk full")
	assert.True(t, hasActiveReportWarning(CodeDoctorManagedAlarmWriteFailed))

	pDegraded, err := store1.QueryOpen(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	assert.True(t, pDegraded.Degraded)
	assert.Equal(t, 3, pDegraded.Total)

	// Capture original IDs before restart.
	var bgpID, isisID, histID string
	for _, a := range pDegraded.Alarms {
		switch a.Kind {
		case AlarmKindComponentDown:
			if a.Device == "r1" {
				bgpID = a.ID
			} else {
				isisID = a.ID
			}
		case AlarmKindHistoryFull:
			histID = a.ID
		}
	}
	require.NotEmpty(t, bgpID)
	require.NotEmpty(t, isisID)
	require.NotEmpty(t, histID)

	// Clear injected write failure: retry flush succeeds and clears degraded warning.
	mem.setFailWrite(nil)
	require.NoError(t, store1.FlushFinal())
	hOK := store1.Health()
	assert.False(t, hOK.Degraded)
	assert.False(t, hOK.WriteFailed)
	assert.False(t, hasActiveReportWarning(CodeDoctorManagedAlarmWriteFailed))

	// Inject one corrupt shard and one oversized shard before restart.
	require.NoError(t, mem.WriteKey(alarmDeviceKey("corrupt-dev"), []byte("{not-json")))
	oversized := make([]byte, zefs.FleetAlarmDeviceValueMax+16)
	for i := range oversized {
		oversized[i] = 'a'
	}
	require.NoError(t, mem.WriteKey(alarmDeviceKey("oversized-dev"), oversized))

	// Simulate hub restart at t3 (5 minutes later).
	t3 := clk.Advance(5 * time.Minute)
	store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: mem,
		Now:   clk.Now,
	})
	require.NoError(t, store2.LoadFromStore())

	hLoaded := store2.Health()
	assert.True(t, hLoaded.Degraded)
	assert.Equal(t, 2, hLoaded.CorruptShards)
	assert.True(t, hasActiveReportWarning(CodeDoctorManagedAlarmCorrupt))

	// ReconcileStartup: r1 bgp still down (keeps ID and t1), r2 isis now healthy
	// (records observed clear at t3, NOT during downtime), history-full still true.
	r1Health := makeAlarmHealthSnap(t3, fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "bgp down"})
	r2Health := makeAlarmHealthSnap(t3, fleet.HealthComponent{Name: "isis", Status: fleet.HealthStatusHealthy})
	store2.ReconcileStartup(
		[]ManagedDevice{
			{Name: "r1", Health: &r1Health, Declared: true},
			{Name: "r2", Health: &r2Health, Declared: true},
		},
		nil,
		&ManagedHistoryHealth{CapacityPressure: true, UsedBytes: 910, HubBudgetBytes: 1000, EffectiveRetentionSeconds: 3600, ConfiguredRetentionDays: 7},
	)

	pAfter, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	require.Equal(t, 2, pAfter.Total)
	assert.Equal(t, bgpID, pAfter.Alarms[0].ID)
	assert.Equal(t, t1, pAfter.Alarms[0].Since, "persisted open timestamp must survive restart")
	assert.Equal(t, histID, pAfter.Alarms[1].ID)
	assert.Equal(t, t1, pAfter.Alarms[1].Since)

	// r1 closed ntp episode survived restart.
	histR1, err := store2.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "r1"})
	require.NoError(t, err)
	require.Equal(t, 2, histR1.Total)

	// r2 isis clear is recorded at t3 (observed restart time), never invented earlier.
	histR2, err := store2.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "r2"})
	require.NoError(t, err)
	require.Equal(t, 1, histR2.Total)
	assert.Equal(t, isisID, histR2.Events[0].ID)
	assert.True(t, histR2.Events[0].Cleared)
	assert.Equal(t, t2, histR2.Events[0].Opened)
	assert.Equal(t, t3, histR2.Events[0].ClearedAt)

	// New alarm after restart gets a strictly larger ID (never collides).
	store2.OnConfigState("r2", true, "local edit", false, "")
	pNew, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "r2"})
	require.NoError(t, err)
	require.Equal(t, 1, pNew.Total)
	assert.Greater(t, pNew.Alarms[0].ID, isisID)

	// OnDeviceForgotten removes in-memory state and deletes the persisted shard.
	store2.OnDeviceForgotten("r2")
	require.NoError(t, store2.FlushFinal())
	_, readErr := mem.ReadKey(alarmDeviceKey("r2"))
	assert.Error(t, readErr)
}

// VALIDATES: AC-13 — 2,050 disconnecting devices traverse completely across
// bounded 1,024-row pages via continuation cursor without skipping or
// displacement when new alarms open mid-traversal; counts and totals cover the
// full authorized snapshot; restricted callers see only permitted devices and
// no hub-scoped alarms; narrowed permissions, changed filters, expired
// snapshots, and unknown severity/kind filters are refused with *opererr.Error.
// PREVENTS: silent truncation at 1,024 rows, row displacement under churn,
// cross-tenant leakage, or stale cursor bypass on permission revocation.
func TestAlarmScalePaginationScopeAndCursorValidation(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 14, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  storage.CreateMemory(),
		Now:    clk.Now,
	})

	const totalDevices = 2050
	declared := make(map[string]bool, totalDevices)
	var tb textbuf.Buffer
	for i := range totalDevices {
		name := tb.Reset().Str("dev-").PadLeft(formatSmallInt(i), 4).String()
		name = strings.ReplaceAll(name, " ", "0")
		declared[name] = true
	}
	store.SyncDeclarations(declared)

	// Trigger offline on all 2,050 devices simultaneously plus hub-scoped history-full.
	tOffline := clk.Advance(60 * time.Second)
	store.Tick(tOffline)
	store.OnHistoryHealth(ManagedHistoryHealth{
		CapacityPressure:          true,
		UsedBytes:                 500,
		HubBudgetBytes:            512,
		EffectiveRetentionSeconds: 1800,
		ConfiguredRetentionDays:   7,
	})

	// Page 1 (unrestricted): bounded to 1,024 rows; Total=2051, Critical=2050, Warning=1.
	page1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{})
	require.NoError(t, err)
	assert.Len(t, page1.Alarms, AlarmPageMax)
	assert.Equal(t, totalDevices+1, page1.Total)
	assert.Equal(t, totalDevices, page1.Counts.Critical)
	assert.Equal(t, 1, page1.Counts.Warning)
	assert.Equal(t, totalDevices+1, page1.Counts.Visible)
	assert.True(t, page1.HasMore)
	assert.NotEmpty(t, page1.Cursor)

	// Open 20 brand-new critical component-down alarms BEFORE page 2: active snapshot must stay stable.
	clk.Advance(5 * time.Second)
	for i := range 20 {
		name := tb.Reset().Str("dev-").PadLeft(formatSmallInt(i), 4).String()
		name = strings.ReplaceAll(name, " ", "0")
		store.OnHealthReport(name, makeAlarmHealthSnap(clk.Now(),
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "flap"},
		))
	}

	// Traverse remaining pages of the original snapshot.
	seenIDs := make(map[string]struct{}, totalDevices+1)
	for _, a := range page1.Alarms {
		seenIDs[a.ID] = struct{}{}
	}
	cursor := page1.Cursor
	pages := 1
	for cursor != "" {
		p, pErr := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Cursor: cursor})
		require.NoError(t, pErr)
		assert.Equal(t, page1.SnapshotID, p.SnapshotID)
		assert.Equal(t, page1.Total, p.Total)
		assert.Equal(t, page1.Counts, p.Counts)
		assert.LessOrEqual(t, len(p.Alarms), AlarmPageMax)
		for _, a := range p.Alarms {
			_, dup := seenIDs[a.ID]
			assert.False(t, dup, "continuation must not repeat occurrence %s", a.ID)
			seenIDs[a.ID] = struct{}{}
		}
		pages++
		cursor = p.Cursor
	}
	assert.Equal(t, 3, pages)
	assert.Len(t, seenIDs, totalDevices+1, "full traversal must visit every occurrence in snapshot")

	// Restricted scope: sees only 5 allowed devices and NO hub-scoped history-full.
	restricted := allowDevicesFleetScope("dev-0000", "dev-0001", "dev-0002", "dev-0003", "dev-0004")
	pRestr, err := store.QueryOpen(restricted, AlarmListQuery{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, pRestr.Alarms, 3)
	// Each of dev-0000..dev-0004 has device-offline + component-down = 10 total, 0 hub warnings.
	assert.Equal(t, 10, pRestr.Total)
	assert.Equal(t, 10, pRestr.Counts.Critical)
	assert.Equal(t, 0, pRestr.Counts.Warning, "restricted caller must not see hub-scoped history-full")
	assert.True(t, pRestr.HasMore)

	// Narrowed permissions between page 1 and page 2 MUST be refused with *opererr.Error.
	narrowed := allowDevicesFleetScope("dev-0000", "dev-0001")
	_, err = store.QueryOpen(narrowed, AlarmListQuery{Limit: 3, Cursor: pRestr.Cursor})
	require.Error(t, err)
	_, isOper := errors.AsType[*opererr.Error](err)
	assert.True(t, isOper, "narrowed-scope continuation refusal must be *opererr.Error")

	// Changed filter between pages MUST be refused with *opererr.Error.
	_, err = store.QueryOpen(restricted, AlarmListQuery{Limit: 3, Severity: "critical", Cursor: pRestr.Cursor})
	require.Error(t, err)
	_, isOper = errors.AsType[*opererr.Error](err)
	assert.True(t, isOper)

	// Unknown severity or unknown kind MUST be refused with *opererr.Error.
	_, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Severity: "fatal"})
	require.Error(t, err)
	_, isOper = errors.AsType[*opererr.Error](err)
	assert.True(t, isOper)

	_, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: "no-such-kind"})
	require.Error(t, err)
	_, isOper = errors.AsType[*opererr.Error](err)
	assert.True(t, isOper)

	// Unauthorized device history query MUST be refused with *opererr.Error.
	_, err = store.QueryHistory(restricted, AlarmHistoryQuery{Device: "dev-0999"})
	require.Error(t, err)
	_, isOper = errors.AsType[*opererr.Error](err)
	assert.True(t, isOper)

	// Expired snapshot (advance > 5m) MUST be refused with *opererr.Error.
	clk.Advance(alarmSnapshotTTL + time.Second)
	_, err = store.QueryOpen(restricted, AlarmListQuery{Limit: 3, Cursor: pRestr.Cursor})
	require.Error(t, err)
	_, isOper = errors.AsType[*opererr.Error](err)
	assert.True(t, isOper)
}

func formatSmallInt(n int) string {
	var tb textbuf.Buffer
	return tb.Int(int64(n)).String()
}

// VALIDATES: AC-14 — declaring a device starts its offline grace interval,
// unchanged reloads and hub restarts mid-grace preserve the original deadline,
// connecting before expiry cancels opening, removing and re-adding the same
// name starts a fresh interval even with prior FirstConnected, and legacy
// declared devices without persisted declaration timing start at first
// observation.
// PREVENTS: grace timers resetting on reload/restart, re-added devices opening
// immediately from stale disconnect/FirstConnected state, or legacy devices
// never timing out.
func TestAlarmDeclarationGraceReloadRestartAndReadd(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	mem := storage.CreateMemory()
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 300

	store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	store1.SyncDeclarations(map[string]bool{"never-conn": true, "early-conn": true})

	// Unchanged reload 60s later MUST preserve original t0 + 300s deadline.
	clk.Advance(60 * time.Second)
	store1.SyncDeclarations(map[string]bool{"never-conn": true, "early-conn": true})

	// early-conn connects before expiry -> cancels its pending offline opening.
	store1.OnDeviceConnected("early-conn", clk.Now())
	require.NoError(t, store1.FlushFinal())

	// Hub restarts 120s into grace period (t0 + 180s).
	clk.Advance(120 * time.Second)
	store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	require.NoError(t, store2.LoadFromStore())
	store2.ReconcileStartup([]ManagedDevice{
		{Name: "never-conn", Declared: true, Connected: false},
		{Name: "early-conn", FirstConnected: t0.Add(60 * time.Second), ConnectedSince: t0.Add(60 * time.Second), Declared: true, Connected: true},
		// legacy-dev has no alarm shard yet; grace starts at restart observation time (t0 + 180s).
		{Name: "legacy-dev", Declared: true, Connected: false},
	}, nil, nil)

	deadline, ok := store2.NextDeadline()
	require.True(t, ok)
	assert.Equal(t, t0.Add(300*time.Second), deadline, "restart mid-grace must keep original deadline")

	// Just before original expiry (t0 + 299s): still not open.
	store2.Tick(t0.Add(299 * time.Second))
	pBefore, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	assert.Equal(t, 0, pBefore.Total)

	// At exact original expiry (t0 + 300s): never-conn opens; early-conn and legacy-dev do NOT.
	store2.Tick(t0.Add(300 * time.Second))
	pAt, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	require.Equal(t, 1, pAt.Total)
	assert.Equal(t, "never-conn", pAt.Alarms[0].Device)
	assert.Equal(t, t0.Add(300*time.Second), pAt.Alarms[0].Since)

	// At t0 + 480s (legacy-dev's observation time + 300s): legacy-dev opens.
	store2.Tick(t0.Add(480 * time.Second))
	pLegacy, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	require.Equal(t, 2, pLegacy.Total)

	// Remove early-conn after disconnecting, then re-add it: MUST start a fresh grace interval.
	clk.Set(t0.Add(500 * time.Second))
	store2.OnDeviceDisconnected("early-conn", clk.Now())
	clk.Set(t0.Add(600 * time.Second))
	store2.SyncDeclarations(map[string]bool{"never-conn": true, "legacy-dev": true})

	tReadd := t0.Add(700 * time.Second)
	clk.Set(tReadd)
	store2.SyncDeclarations(map[string]bool{"never-conn": true, "legacy-dev": true, "early-conn": true})

	// At t0 + 850s (350s past old disconnect, only 150s past re-add): early-conn must NOT be offline yet.
	store2.Tick(t0.Add(850 * time.Second))
	pReaddMid, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "early-conn", Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	assert.Equal(t, 0, pReaddMid.Total)

	// At tReadd + 300s: early-conn opens device-offline.
	store2.Tick(tReadd.Add(300 * time.Second))
	pReaddEnd, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "early-conn", Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	require.Equal(t, 1, pReaddEnd.Total)
	assert.Equal(t, tReadd.Add(300*time.Second), pReaddEnd.Alarms[0].Since)
}

// VALIDATES: config-diverged vs config-rejected independence, software-behind
// deterministic version ordering and skipping unorderable versions,
// history-full carrying cap/used/retention details, retention horizon &
// per-device/hub episode cap eviction preserving active occurrences, and
// RunLoop background deadline/flush lifecycle.
// PREVENTS: conflating config divergence with rejection, false alarms on dev
// software strings, evicting active occurrences under history pressure, or
// leaked worker timers.
func TestAlarmConfigSoftwareHistoryAndRetention(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 16, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.SoftwareBehindVersions = 1
	cfg.RetentionDays = 7
	cfg.MaxEpisodesPerDevice = 3
	cfg.MaxEpisodesHub = 5

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config:        cfg,
		Store:         storage.CreateMemory(),
		Now:           clk.Now,
		FlushInterval: 20 * time.Millisecond,
	})
	store.SyncDeclarations(map[string]bool{
		"d1": true, "d2": true, "d3": true, "d-dev": true, "d-unk": true,
	})

	// 1. config-diverged and config-rejected act independently.
	store.OnConfigState("d1", true, "local edit hash abc", false, "")
	pCfg1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "d1"})
	require.NoError(t, err)
	require.Equal(t, 1, pCfg1.Total)
	assert.Equal(t, AlarmKindConfigDiverged, pCfg1.Alarms[0].Kind)

	store.OnConfigState("d1", true, "local edit hash abc", true, "syntax error line 4")
	pCfg2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "d1"})
	require.NoError(t, err)
	require.Equal(t, 2, pCfg2.Total)

	store.OnConfigState("d1", false, "", true, "syntax error line 4")
	pCfg3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "d1"})
	require.NoError(t, err)
	require.Equal(t, 1, pCfg3.Total)
	assert.Equal(t, AlarmKindConfigRejected, pCfg3.Alarms[0].Kind)
	assert.Equal(t, "syntax error line 4", pCfg3.Alarms[0].Detail)

	store.OnConfigState("d1", false, "", false, "")

	// 2. software-behind: 3 ordered versions [1.0.0, 1.1.0-rc1, 2.0.0] + unorderable "dev" & "unknown".
	store.OnInventoryReport("d1", makeAlarmInventorySnap(t0, "v1.0.0"))
	store.OnInventoryReport("d2", makeAlarmInventorySnap(t0, "1.1.0-rc1"))
	store.OnInventoryReport("d3", makeAlarmInventorySnap(t0, "2.0.0"))
	store.OnInventoryReport("d-dev", makeAlarmInventorySnap(t0, "dev"))
	store.OnInventoryReport("d-unk", makeAlarmInventorySnap(t0, "unknown"))

	pSW1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindSoftwareBehind})
	require.NoError(t, err)
	require.Equal(t, 2, pSW1.Total, "d1 (2 behind) and d2 (1 behind) should open; dev/unknown skipped")
	for _, a := range pSW1.Alarms {
		assert.Contains(t, a.Summary, "2.0.0")
		assert.Contains(t, a.Detail, "newest=2.0.0")
	}

	// Raise threshold to 2 versions behind: only d1 stays open, d2 clears.
	cfg.SoftwareBehindVersions = 2
	store.UpdateConfig(cfg)
	pSW2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindSoftwareBehind})
	require.NoError(t, err)
	require.Equal(t, 1, pSW2.Total)
	assert.Equal(t, "d1", pSW2.Alarms[0].Device)

	// Upgrade d1 to 2.0.0: software-behind clears.
	store.OnInventoryReport("d1", makeAlarmInventorySnap(clk.Advance(5*time.Second), "2.0.0"))
	pSW3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindSoftwareBehind})
	require.NoError(t, err)
	assert.Equal(t, 0, pSW3.Total)

	// 3. history-full opens with cap/used/retention details and clears when pressure ends.
	store.OnHistoryHealth(ManagedHistoryHealth{
		CapacityPressure:          true,
		UsedBytes:                 4096,
		HubBudgetBytes:            8192,
		DeviceBudgetBytes:         2048,
		EffectiveRetentionSeconds: 7200,
		ConfiguredRetentionDays:   7,
	})
	pHist, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindHistoryFull})
	require.NoError(t, err)
	require.Equal(t, 1, pHist.Total)
	assert.Equal(t, "", pHist.Alarms[0].Device)
	assert.Equal(t, AlarmScopeHub, pHist.Alarms[0].Scope)
	assert.Contains(t, pHist.Alarms[0].Summary, "4096")
	assert.Contains(t, pHist.Alarms[0].Summary, "8192")
	assert.Contains(t, pHist.Alarms[0].Detail, "effective-retention-seconds=7200")

	store.OnHistoryHealth(ManagedHistoryHealth{CapacityPressure: false})
	pHistCleared, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindHistoryFull})
	require.NoError(t, err)
	assert.Equal(t, 0, pHistCleared.Total)

	// 4. Per-device episode cap (MaxEpisodesPerDevice = 3) & hub cap (MaxEpisodesHub = 5)
	// evict oldest ClearedAt first while preserving active occurrences.
	store.OnHealthReport("d2", makeAlarmHealthSnap(clk.Advance(time.Second),
		fleet.HealthComponent{Name: "active-comp", Status: fleet.HealthStatusDown, Reason: "stays open"},
	))
	for range 5 {
		tOpen := clk.Advance(10 * time.Second)
		store.OnHealthReport("d2", makeAlarmHealthSnap(tOpen,
			fleet.HealthComponent{Name: "active-comp", Status: fleet.HealthStatusDown, Reason: "stays open"},
			fleet.HealthComponent{Name: "flap", Status: fleet.HealthStatusDown, Reason: "flap"},
		))
		tClose := clk.Advance(10 * time.Second)
		store.OnHealthReport("d2", makeAlarmHealthSnap(tClose,
			fleet.HealthComponent{Name: "active-comp", Status: fleet.HealthStatusDown, Reason: "stays open"},
			fleet.HealthComponent{Name: "flap", Status: fleet.HealthStatusHealthy},
		))
	}

	hD2, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "d2"})
	require.NoError(t, err)
	assert.True(t, hD2.Truncated)
	assert.False(t, hD2.RetainedSince.IsZero())
	// 1 active open + 3 retained closed = 4 events.
	assert.Equal(t, 4, hD2.Total)
	var activeStillOpen bool
	for _, ev := range hD2.Events {
		if ev.Subject == "active-comp" && !ev.Cleared {
			activeStillOpen = true
		}
	}
	assert.True(t, activeStillOpen, "eviction must never drop active open occurrences")

	// Trigger additional closed episodes on d3 to exceed MaxEpisodesHub=5.
	for range 3 {
		store.OnConfigState("d3", true, "edit", false, "")
		clk.Advance(10 * time.Second)
		store.OnConfigState("d3", false, "", false, "")
		clk.Advance(10 * time.Second)
	}
	healthAfterCap := store.Health()
	assert.LessOrEqual(t, healthAfterCap.RetainedClosed, 5, "hub-wide closed episodes must stay <= MaxEpisodesHub")
	assert.True(t, healthAfterCap.Truncated)

	// 5. Horizon retention (7 days): advancing 8 days prunes expired closed episodes
	// while keeping active occurrences intact.
	clk.Advance(8 * 24 * time.Hour)
	store.Tick(clk.Now())
	hD2Aged, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: "d2"})
	require.NoError(t, err)
	// Only active open occurrence(s) remain; closed episodes aged past 7 days are pruned.
	for _, ev := range hD2Aged.Events {
		assert.False(t, ev.Cleared, "closed episodes older than 7 days must be pruned")
	}

	// 6. RunLoop clean shutdown smoke test.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.RunLoop(ctx)
	}()
	cancel()
	<-done
	require.NoError(t, store.FlushFinal())
}

// VALIDATES: AC-14 — removing a device declaration, persisting state, and
// restarting the hub with the same device re-added starts a fresh declaration
// generation and resets declaredAt to the restart observation time so
// device-offline does not open immediately from an earlier declaration.
// PREVENTS: stale declaredAt timestamps from a removed declaration firing
// immediate false device-offline alarms on restart after re-add.
func TestAlarmRestartAfterRemoveAndReaddResetsGrace(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	mem := storage.CreateMemory()
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 300

	store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	store1.SyncDeclarations(map[string]bool{"edge-01": true})

	// Remove edge-01 and flush the undeclared shard before stopping.
	clk.Advance(100 * time.Second)
	store1.SyncDeclarations(map[string]bool{})
	require.NoError(t, store1.FlushFinal())

	// Restart 600s later (well past t0 + 300s) with edge-01 re-added in config.
	tRestart := clk.Advance(600 * time.Second)
	store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	require.NoError(t, store2.LoadFromStore())
	store2.ReconcileStartup([]ManagedDevice{
		{Name: "edge-01", Declared: true, Connected: false},
	}, nil, nil)

	store2.mu.Lock()
	dev := store2.devices["edge-01"]
	require.NotNil(t, dev)
	assert.True(t, dev.declared)
	assert.Equal(t, tRestart, dev.declaredAt, "re-added device across restart must reset declaredAt to restart time")
	assert.Equal(t, uint64(2), dev.declarationGeneration, "re-added device must increment declarationGeneration")
	assert.False(t, dev.connectedInDeclaration)
	assert.True(t, dev.lastDisconnectedAt.IsZero())
	store2.mu.Unlock()

	// Must NOT open device-offline at restart or mid-grace.
	store2.Tick(tRestart)
	p0, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	assert.Equal(t, 0, p0.Total)

	store2.Tick(tRestart.Add(299 * time.Second))
	pMid, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	assert.Equal(t, 0, pMid.Total)

	// Opens at tRestart + offlineDelay.
	store2.Tick(tRestart.Add(300 * time.Second))
	pEnd, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
	require.NoError(t, err)
	require.Equal(t, 1, pEnd.Total)
	assert.Equal(t, tRestart.Add(300*time.Second), pEnd.Alarms[0].Since)
}

// VALIDATES: AC-12 and AC-14 — an unclean hub crash while a device is
// connected (both first-connection crash and reconnect-after-earlier-disconnect
// crash) seeds a fresh offline-delay grace window from restart time instead of
// immediately flapping device-offline from an old declaredAt or stale earlier
// lastDisconnectedAt.
// PREVENTS: hub crash-restarts flooding false immediate device-offline alarms
// for every previously connected device.
func TestAlarmUncleanHubCrashWhileConnectedGrace(t *testing.T) {
	t.Run("first-connect-crash", func(t *testing.T) {
		t0 := time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		mem := storage.CreateMemory()
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 120

		store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  mem,
			Now:    clk.Now,
		})
		store1.SyncDeclarations(map[string]bool{"edge-01": true})
		clk.Advance(10 * time.Second)
		store1.OnDeviceConnected("edge-01", clk.Now())
		// Persist connected state WITHOUT calling OnDeviceDisconnected (simulates crash).
		require.NoError(t, store1.FlushFinal())

		// Restart long after t0 + 120s.
		tRestart := clk.Advance(1000 * time.Second)
		store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  mem,
			Now:    clk.Now,
		})
		require.NoError(t, store2.LoadFromStore())
		store2.ReconcileStartup([]ManagedDevice{
			{Name: "edge-01", Declared: true, Connected: false},
		}, nil, nil)

		store2.Tick(tRestart)
		p0, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
		require.NoError(t, err)
		assert.Equal(t, 0, p0.Total, "unclean crash must not trigger immediate device-offline from old declaredAt")

		store2.Tick(tRestart.Add(119 * time.Second))
		pBefore, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
		require.NoError(t, err)
		assert.Equal(t, 0, pBefore.Total)

		store2.Tick(tRestart.Add(120 * time.Second))
		pAfter, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
		require.NoError(t, err)
		require.Equal(t, 1, pAfter.Total)
		assert.Equal(t, tRestart.Add(120*time.Second), pAfter.Alarms[0].Since)
	})

	t.Run("disconnect-reconnect-crash", func(t *testing.T) {
		t0 := time.Date(2026, 11, 27, 19, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		mem := storage.CreateMemory()
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 120

		store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  mem,
			Now:    clk.Now,
		})
		store1.SyncDeclarations(map[string]bool{"edge-01": true})
		store1.OnDeviceConnected("edge-01", t0)

		// Disconnect at T1, persist, then reconnect at T2 (clears lastDisconnectedAt).
		t1 := clk.Advance(60 * time.Second)
		store1.OnDeviceDisconnected("edge-01", t1)
		require.NoError(t, store1.FlushFinal())

		t2 := clk.Advance(30 * time.Second)
		store1.OnDeviceConnected("edge-01", t2)
		// Persist reconnected state WITHOUT calling OnDeviceDisconnected (simulates crash at T3).
		require.NoError(t, store1.FlushFinal())

		tRestart := clk.Advance(900 * time.Second)
		store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  mem,
			Now:    clk.Now,
		})
		require.NoError(t, store2.LoadFromStore())
		store2.ReconcileStartup([]ManagedDevice{
			{Name: "edge-01", Declared: true, Connected: false},
		}, nil, nil)

		store2.Tick(tRestart)
		p0, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
		require.NoError(t, err)
		assert.Equal(t, 0, p0.Total, "crash after reconnect must not flap device-offline from stale T1")

		store2.Tick(tRestart.Add(120 * time.Second))
		pAfter, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01", Kind: AlarmKindDeviceOffline})
		require.NoError(t, err)
		require.Equal(t, 1, pAfter.Total)
		assert.Equal(t, tRestart.Add(120*time.Second), pAfter.Alarms[0].Since)
	})
}

// VALIDATES: undeclaring a device via SyncDeclarations or ReconcileStartup
// closes session- and telemetry-driven alarms (device-offline, software-behind,
// component-down, component-degraded, resource-high, config-rejected) while
// preserving config-diverged when a held configuration copy still exists.
// PREVENTS: undeclared devices leaving zombie component/resource/rejected
// alarms open forever.
func TestAlarmUndeclareClearsSessionTelemetryPreservesConfigDiverged(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 20, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	mem := storage.CreateMemory()
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60
	cfg.ResourceCPUPercent = 90
	cfg.ResourceDurationSeconds = 60
	cfg.SoftwareBehindVersions = 1

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	store.SyncDeclarations(map[string]bool{"edge-01": true, "edge-02": true})
	store.OnDeviceConnected("edge-01", t0)
	store.OnDeviceConnected("edge-02", t0)

	// Open component-down, component-degraded, resource-high, software-behind,
	// config-rejected, and config-diverged on edge-01.
	store.OnHealthReport("edge-01", makeAlarmHealthSnap(t0,
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDegraded, Reason: "slow"},
	))
	store.OnInventoryReport("edge-01", makeAlarmInventorySnap(t0, "1.0.0"))
	store.OnInventoryReport("edge-02", makeAlarmInventorySnap(t0, "2.0.0"))
	store.OnSampleReport("edge-01", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), t0)
	store.OnSampleReport("edge-01", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), clk.Advance(60*time.Second))
	store.OnConfigState("edge-01", true, "held-version=abcdef0123456789", true, "rejected config")

	// Disconnect and open device-offline too.
	store.OnDeviceDisconnected("edge-01", clk.Now())
	store.Tick(clk.Advance(60 * time.Second))

	pBefore, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	require.Equal(t, 7, pBefore.Total)
	divBefore, ok := findOpenAlarm(pBefore.Alarms, AlarmKindConfigDiverged, "edge-01", "")
	require.True(t, ok)

	// Undeclare edge-01: all 6 session/telemetry alarms close; config-diverged remains.
	clk.Advance(10 * time.Second)
	store.SyncDeclarations(map[string]bool{"edge-02": true})

	pAfter, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	require.Equal(t, 1, pAfter.Total)
	assert.Equal(t, AlarmKindConfigDiverged, pAfter.Alarms[0].Kind)
	assert.Equal(t, divBefore.ID, pAfter.Alarms[0].ID)
	assert.Equal(t, divBefore.Since, pAfter.Alarms[0].Since)

	// Verify startup reconciliation with an undeclared device holding stale
	// persisted health/rejection and a held copy keeps ONLY config-diverged.
	require.NoError(t, store.FlushFinal())
	clk.Advance(30 * time.Second)
	store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	require.NoError(t, store2.LoadFromStore())
	staleHealth := makeAlarmHealthSnap(t0, fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"})
	store2.ReconcileStartup([]ManagedDevice{
		{
			Name:      "edge-01",
			Health:    &staleHealth,
			AckResult: ManagedAckRefused,
			AckError:  "rejected config",
			Declared:  false,
		},
		{Name: "edge-02", Declared: true},
	}, map[string]string{"edge-01": "held-version=abcdef0123456789"}, nil)

	pRestart, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-01"})
	require.NoError(t, err)
	require.Equal(t, 1, pRestart.Total)
	assert.Equal(t, AlarmKindConfigDiverged, pRestart.Alarms[0].Kind)
	assert.Equal(t, divBefore.ID, pRestart.Alarms[0].ID)
	assert.Equal(t, "held-version=abcdef0123456789", pRestart.Alarms[0].Detail)
}

// VALIDATES: AC-10 — missing resource figures or zero capacity totals reset
// pending qualification timers so gaps never advance qualification while
// preserving already-open resource-high occurrences, and exact boundary
// readings (pct == threshold) qualify with "at or above" wording while
// pct < threshold resets.
// PREVENTS: intermittent missing-figure reports advancing resource-high
// qualification timers or boundary wording disagreeing with >= comparison.
func TestAlarmResourceHighMissingFiguresAndExactThreshold(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 21, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.ResourceCPUPercent = 90
	cfg.ResourceMemoryPercent = 80
	cfg.ResourceStoragePercent = 100
	cfg.ResourceDurationSeconds = 120

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  storage.CreateMemory(),
		Now:    clk.Now,
	})
	store.SyncDeclarations(map[string]bool{"edge-1": true})
	store.OnDeviceConnected("edge-1", t0)

	// Start CPU qualification at exact boundary (90% == 90%) and storage at 100% == 100%.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 90},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 800},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), t0)

	// At t0+60s: omit CPU and pass memory_total=0; keep storage at 100%.
	// CPU and memory pending qualification timers MUST reset.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 900},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 0},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), t0.Add(60*time.Second))

	// At t0+120s: CPU=90%, memory=80%, storage=100%.
	// Only storage had continuous readings for 120s; CPU and memory restarted at t0+120s.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 90},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 800},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageUsed, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureStorageTotal, Value: 1000},
	), t0.Add(120*time.Second))

	p1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1", Kind: AlarmKindResourceHigh})
	require.NoError(t, err)
	require.Equal(t, 1, p1.Total, "missing-figure gaps must reset CPU and memory pending timers")
	assert.Equal(t, "storage", p1.Alarms[0].Subject)
	assert.Contains(t, p1.Alarms[0].Summary, "at or above 100% threshold")
	storageID := p1.Alarms[0].ID

	// At t0+180s: omit storage while keeping CPU and memory at exact threshold;
	// already-open storage occurrence MUST be preserved!
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 90},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 800},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
	), t0.Add(180*time.Second))

	p2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1", Kind: AlarmKindResourceHigh})
	require.NoError(t, err)
	require.Equal(t, 1, p2.Total)
	assert.Equal(t, storageID, p2.Alarms[0].ID, "missing figure must preserve already-open resource-high")

	// At t0+240s: CPU (90% == 90%) opens; memory drops just below threshold (79.9% < 80%) and resets.
	store.OnSampleReport("edge-1", makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 90},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 799},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
	), t0.Add(240*time.Second))

	p3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: "edge-1", Kind: AlarmKindResourceHigh})
	require.NoError(t, err)
	require.Equal(t, 2, p3.Total)
	cpuOcc, hasCPU := findOpenAlarm(p3.Alarms, AlarmKindResourceHigh, "edge-1", "cpu")
	require.True(t, hasCPU)
	assert.Contains(t, cpuOcc.Summary, "at or above 90% threshold")
	_, hasMem := findOpenAlarm(p3.Alarms, AlarmKindResourceHigh, "edge-1", "memory")
	assert.False(t, hasMem, "below-threshold reading (79.9% < 80%) must reset qualification")
}

// VALIDATES: AC-13 hub-scoped alarm authorization — a device-restricted caller
// is denied hub-scoped alarms (history-full) even when the fleet is empty
// (len(s.devices) == 0), whereas an unrestricted fleet alarm operator
// authorized for "show fleet alarm" and "show fleet device name *" (without
// "show fleet sample storage") is allowed hub-scoped alarms.
// PREVENTS: allowsHubScopeLocked failing open on an empty fleet or falsely
// denying unrestricted alarm operators who lack sample-storage permissions.
func TestAlarmHubScopeEmptyFleetAndUnrestrictedAlarmOperator(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 22, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)

	authStore := authz.NewStore()
	authStore.AddProfile(authz.Profile{
		Name: "edge-restricted-alarms",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet alarm($| \\|.*)$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	authStore.AddProfile(authz.Profile{
		Name: "unrestricted-alarms-no-storage",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet alarm($| \\|.*)$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet device name .*$", Regex: true},
			},
		},
	})
	authStore.AddProfile(authz.Profile{
		Name: "deny-one-device-alarms",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Deny, Match: "^show fleet device name core-01$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet alarm($| \\|.*)$", Regex: true},
				{Number: 30, Action: authz.Allow, Match: "^show fleet device name .*$", Regex: true},
			},
		},
	})
	authStore.AssignProfiles("edge-op", []string{"edge-restricted-alarms"})
	authStore.AssignProfiles("alarm-admin", []string{"unrestricted-alarms-no-storage"})
	authStore.AssignProfiles("almost-admin", []string{"deny-one-device-alarms"})

	scopeAuth := authz.StoreAuthorizer{Store: authStore}
	edgeScope := FleetDeviceScope{mode: fleetScopeEvaluate, username: "edge-op", scopeAuth: scopeAuth}
	adminScope := FleetDeviceScope{mode: fleetScopeEvaluate, username: "alarm-admin", scopeAuth: scopeAuth}
	almostScope := FleetDeviceScope{mode: fleetScopeEvaluate, username: "almost-admin", scopeAuth: scopeAuth}

	// Empty fleet (len(s.devices) == 0) with open hub-scoped history-full.
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: storage.CreateMemory(),
		Now:   clk.Now,
	})
	store.OnHistoryHealth(ManagedHistoryHealth{
		CapacityPressure:          true,
		UsedBytes:                 900,
		HubBudgetBytes:            1000,
		EffectiveRetentionSeconds: 1800,
		ConfiguredRetentionDays:   7,
	})

	pEdgeEmpty, err := store.QueryOpen(edgeScope, AlarmListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, pEdgeEmpty.Total, "device-restricted caller must not see history-full on an empty fleet")

	pAdminEmpty, err := store.QueryOpen(adminScope, AlarmListQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, pAdminEmpty.Total, "unrestricted alarm operator without sample-storage permission must see history-full")
	assert.Equal(t, AlarmKindHistoryFull, pAdminEmpty.Alarms[0].Kind)

	// Add core-01: almost-admin (explicitly denied core-01) must now be denied history-full.
	store.SyncDeclarations(map[string]bool{"core-01": true})
	pAlmost, err := store.QueryOpen(almostScope, AlarmListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, pAlmost.Total, "caller denied any known device must not see hub-scoped history-full")

	pAdminWithDev, err := store.QueryOpen(adminScope, AlarmListQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, pAdminWithDev.Total)
	assert.Equal(t, AlarmKindHistoryFull, pAdminWithDev.Alarms[0].Kind)
}

// VALIDATES: AC-11 — externally registered alarm kinds with an Evaluate
// callback re-evaluate reactively on lifecycle events (SyncDeclarations,
// OnDeviceDisconnected) and Tick after startup without manually calling
// EvaluateExternalKinds.
// PREVENTS: external kinds (such as fleet-9 relay-not-granted) becoming stale
// after startup because EvaluateExternalKinds is never triggered again.
func TestAlarmExternalKindReactiveOnEventsAndTick(t *testing.T) {
	t.Cleanup(SnapshotAlarmKinds())

	var mu sync.Mutex
	alertOnTick := false

	const reactiveKind = "test-reactive-external"
	require.NoError(t, RegisterAlarmKind(AlarmKindSpec{
		Name:     reactiveKind,
		Severity: AlarmSeverityInformational,
		Scope:    AlarmScopeDevice,
		Summary:  "Reactive external condition",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			mu.Lock()
			wantTick := alertOnTick
			mu.Unlock()
			var out []AlarmObservation
			for _, dev := range ctx.Devices {
				if !dev.Declared {
					continue
				}
				if !dev.Connected && !dev.LastDisconnectedAt.IsZero() {
					out = append(out, AlarmObservation{
						Device:     dev.Name,
						Subject:    "disconnect",
						Summary:    "External noticed disconnect",
						ObservedAt: ctx.Now,
					})
				}
				if wantTick {
					out = append(out, AlarmObservation{
						Device:     dev.Name,
						Subject:    "tick",
						Summary:    "External noticed tick condition",
						ObservedAt: ctx.Now,
					})
				}
			}
			return out
		},
	}))

	t0 := time.Date(2026, 11, 27, 23, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store: storage.CreateMemory(),
		Now:   clk.Now,
	})
	store.SyncDeclarations(map[string]bool{"edge-01": true})
	store.OnDeviceConnected("edge-01", t0)

	// No external alarm open while connected and alertOnTick is false.
	p0, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: reactiveKind})
	require.NoError(t, err)
	assert.Equal(t, 0, p0.Total)

	// OnDeviceDisconnected triggers external evaluation without calling EvaluateExternalKinds.
	tDisc := clk.Advance(15 * time.Second)
	store.OnDeviceDisconnected("edge-01", tDisc)
	p1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: reactiveKind})
	require.NoError(t, err)
	require.Equal(t, 1, p1.Total)
	assert.Equal(t, "disconnect", p1.Alarms[0].Subject)
	assert.Equal(t, tDisc, p1.Alarms[0].Since)

	// Tick triggers external evaluation without calling EvaluateExternalKinds.
	mu.Lock()
	alertOnTick = true
	mu.Unlock()
	tTick := clk.Advance(15 * time.Second)
	store.Tick(tTick)
	p2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: reactiveKind})
	require.NoError(t, err)
	require.Equal(t, 2, p2.Total)
	tickOcc, hasTick := findOpenAlarm(p2.Alarms, reactiveKind, "edge-01", "tick")
	require.True(t, hasTick)
	assert.Equal(t, tTick, tickOcc.Since)

	// Clearing alertOnTick clears on the next Tick automatically.
	mu.Lock()
	alertOnTick = false
	mu.Unlock()
	store.Tick(clk.Advance(15 * time.Second))
	p3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: reactiveKind})
	require.NoError(t, err)
	require.Equal(t, 1, p3.Total)
	assert.Equal(t, "disconnect", p3.Alarms[0].Subject)
}
