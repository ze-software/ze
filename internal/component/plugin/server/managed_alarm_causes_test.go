// Design: docs/architecture/fleet-alarms.md -- unit tests for hub alarm cause graph validation, hiding, settling, and history overlap
// Related: managed_alarms_kinds.go -- AlarmKindRegistry.ValidateCauses and CauseDepths
// Related: managed_alarms.go -- applyAlarmCauses, applyAlarmHistoryCauses, QueryOpen, and QueryHistory
// Related: managed_serve.go -- NewManagedServer alarm registry validation at startup

package server

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	testCauseDev1 = "edge-01"
	testCauseDev2 = "edge-02"

	testKindRelayNotGranted = "relay-not-granted"
	testKindCauseLeaf       = "test-cause-leaf"
	testKindCauseMidA       = "test-cause-mid-a"
	testKindCauseMidB       = "test-cause-mid-b"
	testKindCauseRoot       = "test-cause-root"
)

func findHistoryEventByID(events []AlarmHistoryEvent, id string) (AlarmHistoryEvent, bool) {
	for i := range events {
		if events[i].ID == id {
			return events[i], true
		}
	}
	return AlarmHistoryEvent{}, false
}

// VALIDATES: AC-1, AC-4, AC-6, AC-11, AC-13 -- open causes hide affected
// same-device alarms with HiddenBehind naming the nearest cause by BFS depth
// and smallest ID byte order across all row insertion orders; independent kinds
// (config-diverged, config-rejected, software-behind, relay-not-granted,
// history-full) stay shown; late-opening causes hide already-open alarms
// without clearing history; cross-device causes do not leak; and kind/severity/
// hidden filters preserve HiddenBehind and enforce cursor filter binding.
// PREVENTS: stale device alarms cluttering the operator list while offline,
// order-dependent tie-breaking, cross-device cause leakage, or filtered queries
// losing HiddenBehind because the cause row was filtered out.
func TestAlarmHiddenBehindOpenCause(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	// Build a cloned registry with relay-not-granted (no causes, O-1) and a
	// multi-level cause diamond (leaf -> mid-a, mid-b -> root).
	reg := defaultAlarmKindRegistry.Clone()
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     testKindRelayNotGranted,
		Severity: AlarmSeverityInformational,
		Scope:    AlarmScopeDevice,
		Summary:  "Device has no active relay grant",
	}))
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     testKindCauseRoot,
		Severity: AlarmSeverityCritical,
		Scope:    AlarmScopeDevice,
		Summary:  "Root cause",
	}))
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     testKindCauseMidA,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Causes:   []string{testKindCauseRoot},
		Summary:  "Mid cause A",
	}))
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     testKindCauseMidB,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Causes:   []string{testKindCauseRoot},
		Summary:  "Mid cause B",
	}))
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     testKindCauseLeaf,
		Severity: AlarmSeverityInformational,
		Scope:    AlarmScopeDevice,
		Causes:   []string{testKindCauseMidA, testKindCauseMidB},
		Summary:  "Leaf affected alarm",
	}))
	require.NoError(t, reg.ValidateCauses())

	// 1. Table-driven cases over applyAlarmCauses.
	cases := []struct {
		name string
		rows []AlarmOccurrence
		want map[string]string
	}{
		{
			name: "offline-hides-component-and-resource-only",
			rows: []AlarmOccurrence{
				{ID: "alm-001", Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-002", Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Subject: "bgp", Since: t0},
				{ID: "alm-003", Kind: AlarmKindComponentDegraded, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Subject: "ospf", Since: t0},
				{ID: "alm-004", Kind: AlarmKindResourceHigh, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Subject: "cpu", Since: t0},
				{ID: "alm-005", Kind: AlarmKindConfigDiverged, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-006", Kind: AlarmKindConfigRejected, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-007", Kind: AlarmKindSoftwareBehind, Severity: AlarmSeverityInformational, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-008", Kind: testKindRelayNotGranted, Severity: AlarmSeverityInformational, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-009", Kind: AlarmKindHistoryFull, Severity: AlarmSeverityWarning, Scope: AlarmScopeHub, Since: t0},
			},
			want: map[string]string{
				"alm-001": "",
				"alm-002": "alm-001",
				"alm-003": "alm-001",
				"alm-004": "alm-001",
				"alm-005": "",
				"alm-006": "",
				"alm-007": "",
				"alm-008": "",
				"alm-009": "",
			},
		},
		{
			name: "bfs-nearest-depth-beats-smaller-id-at-greater-depth",
			rows: []AlarmOccurrence{
				{ID: "alm-001", Kind: testKindCauseRoot, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-005", Kind: testKindCauseMidB, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-003", Kind: testKindCauseMidA, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-009", Kind: testKindCauseLeaf, Severity: AlarmSeverityInformational, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
			},
			want: map[string]string{
				"alm-001": "",
				"alm-003": "alm-001",
				"alm-005": "alm-001",
				"alm-009": "alm-003", // depth 1 beats root (depth 2); alm-003 < alm-005
			},
		},
		{
			name: "cross-device-cause-isolation",
			rows: []AlarmOccurrence{
				{ID: "alm-001", Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev2, Since: t0},
				{ID: "alm-002", Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Subject: "bgp", Since: t0},
				{ID: "alm-003", Kind: testKindCauseMidA, Severity: AlarmSeverityWarning, Scope: AlarmScopeDevice, Device: testCauseDev2, Since: t0},
				{ID: "alm-004", Kind: testKindCauseRoot, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
				{ID: "alm-005", Kind: testKindCauseLeaf, Severity: AlarmSeverityInformational, Scope: AlarmScopeDevice, Device: testCauseDev1, Since: t0},
			},
			want: map[string]string{
				"alm-001": "",
				"alm-002": "",        // edge-02 offline must not hide edge-01 component-down
				"alm-003": "",        // edge-01 root must not hide edge-02 mid-a
				"alm-004": "",        // root has no cause
				"alm-005": "alm-004", // edge-02 mid-a ignored; edge-01 root (depth 2) selected
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify identical output across original, reversed, and rotated insertion orders.
			for shift := range len(tc.rows) {
				rotated := make([]AlarmOccurrence, 0, len(tc.rows))
				rotated = append(rotated, tc.rows[shift:]...)
				rotated = append(rotated, tc.rows[:shift]...)
				applyAlarmCauses(reg, rotated)
				for _, r := range rotated {
					assert.Equal(t, tc.want[r.ID], r.HiddenBehind, "shift=%d id=%s kind=%s", shift, r.ID, r.Kind)
				}

				reversed := slices.Clone(rotated)
				slices.Reverse(reversed)
				applyAlarmCauses(reg, reversed)
				for _, r := range reversed {
					assert.Equal(t, tc.want[r.ID], r.HiddenBehind, "reversed shift=%d id=%s kind=%s", shift, r.ID, r.Kind)
				}
			}
		})
	}

	// 2. End-to-end ManagedAlarmStore: AC-4 (late-opening cause hides open
	// resource-high with no history clear), AC-11 (kind/severity/hidden filters
	// preserve HiddenBehind), and continuation cursor filter binding.
	t.Run("query-open-filters-and-late-cause", func(t *testing.T) {
		clk := newAlarmFakeClock(t0)
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 60
		cfg.ResourceCPUPercent = 90
		cfg.ResourceDurationSeconds = 60

		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config:   cfg,
			Store:    storage.CreateMemory(),
			Now:      clk.Now,
			Registry: reg,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true, testCauseDev2: true})
		store.OnDeviceConnected(testCauseDev1, t0)
		store.OnDeviceConnected(testCauseDev2, t0)

		// Open component-down, resource-high, and config-diverged while online.
		tDown := clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tDown,
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		))
		store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
			fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
		), tDown)
		tResOpen := clk.Advance(60 * time.Second)
		store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
			fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
		), tResOpen)
		store.OnConfigState(testCauseDev1, true, "local edit", false, "")

		// Verify resource-high is shown before offline opens.
		pOnline, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		resBefore, ok := findOpenAlarm(pOnline.Alarms, AlarmKindResourceHigh, testCauseDev1, "cpu")
		require.True(t, ok)
		assert.Empty(t, resBefore.HiddenBehind)
		assert.Equal(t, tResOpen, resBefore.Since)

		// edge-01 disconnects and goes offline (AC-4).
		tDisc := clk.Advance(10 * time.Second)
		store.OnDeviceDisconnected(testCauseDev1, tDisc)
		tOff := clk.Advance(60 * time.Second)
		store.Tick(tOff)

		pAll, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		require.Equal(t, 4, pAll.Total)
		assert.Equal(t, 2, pAll.Counts.Visible, "device-offline and config-diverged are shown")
		assert.Equal(t, 2, pAll.Counts.Hidden, "component-down and resource-high are hidden")

		offOcc, ok := findOpenAlarm(pAll.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
		require.True(t, ok)
		assert.Empty(t, offOcc.HiddenBehind)

		resAfter, ok := findOpenAlarm(pAll.Alarms, AlarmKindResourceHigh, testCauseDev1, "cpu")
		require.True(t, ok)
		assert.Equal(t, resBefore.ID, resAfter.ID)
		assert.Equal(t, tResOpen, resAfter.Since)
		assert.Equal(t, offOcc.ID, resAfter.HiddenBehind)

		// History for AC-4 holds NO clearing for resource-high while offline.
		hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		resHist, ok := findHistoryEventByID(hist.Events, resBefore.ID)
		require.True(t, ok)
		assert.False(t, resHist.Cleared)
		assert.True(t, resHist.Hidden)
		assert.Equal(t, offOcc.ID, resHist.HiddenBehind)

		// Filter by Kind: component-down STILL carries HiddenBehind from device-offline!
		pKind, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Kind: AlarmKindComponentDown})
		require.NoError(t, err)
		require.Equal(t, 1, pKind.Total)
		assert.Equal(t, offOcc.ID, pKind.Alarms[0].HiddenBehind)
		assert.Equal(t, 0, pKind.Counts.Visible)
		assert.Equal(t, 1, pKind.Counts.Hidden)

		// Filter by Severity: warning returns resource-high (hidden) and config-diverged (shown).
		pSev, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Severity: "warning"})
		require.NoError(t, err)
		require.Equal(t, 2, pSev.Total)
		assert.Equal(t, 1, pSev.Counts.Visible)
		assert.Equal(t, 1, pSev.Counts.Hidden)
		resSev, ok := findOpenAlarm(pSev.Alarms, AlarmKindResourceHigh, testCauseDev1, "cpu")
		require.True(t, ok)
		assert.Equal(t, offOcc.ID, resSev.HiddenBehind)
		divSev, ok := findOpenAlarm(pSev.Alarms, AlarmKindConfigDiverged, testCauseDev1, "")
		require.True(t, ok)
		assert.Empty(t, divSev.HiddenBehind)

		// Filter by Hidden=true returns only hidden alarms.
		pHidden, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Hidden: new(true), Limit: 1})
		require.NoError(t, err)
		require.Equal(t, 2, pHidden.Total)
		assert.Equal(t, 0, pHidden.Counts.Visible)
		assert.Equal(t, 2, pHidden.Counts.Hidden)
		assert.NotEmpty(t, pHidden.Alarms[0].HiddenBehind)
		require.True(t, pHidden.HasMore)

		// Changing Hidden filter on continuation cursor MUST be refused with *opererr.Error.
		_, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Hidden: new(false), Limit: 1, Cursor: pHidden.Cursor})
		require.Error(t, err)
		_, isOper := errors.AsType[*opererr.Error](err)
		assert.True(t, isOper)
		_, err = store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Limit: 1, Cursor: pHidden.Cursor})
		require.Error(t, err)
		_, isOper = errors.AsType[*opererr.Error](err)
		assert.True(t, isOper)

		// Same Hidden=true on continuation cursor succeeds.
		pHidden2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Hidden: new(true), Limit: 1, Cursor: pHidden.Cursor})
		require.NoError(t, err)
		require.Len(t, pHidden2.Alarms, 1)
		assert.NotEmpty(t, pHidden2.Alarms[0].HiddenBehind)

		// Filter by Hidden=false returns only shown alarms.
		pShown, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Hidden: new(false)})
		require.NoError(t, err)
		require.Equal(t, 2, pShown.Total)
		assert.Equal(t, 2, pShown.Counts.Visible)
		assert.Equal(t, 0, pShown.Counts.Hidden)
		for _, a := range pShown.Alarms {
			assert.Empty(t, a.HiddenBehind)
		}

		// Restricted scope sees only its authorized devices.
		pDenied, err := store.QueryOpen(allowDevicesFleetScope(testCauseDev2), AlarmListQuery{})
		require.NoError(t, err)
		assert.Equal(t, 0, pDenied.Total)
	})
}

// VALIDATES: AC-2, AC-3, AC-10, AC-13 -- clearing an open cause restores a
// non-settling alarm with its original ID and Since; an alarm that opens and
// clears while hidden leaves a history row marked Hidden with HiddenBehind;
// clearing two equally near causes in succession exposes the remaining nearest
// cause and then a more distant cause while preserving Since; and truncated
// history sets HiddenUnknown when evicted episodes could have hidden an older
// occurrence.
// PREVENTS: lost original Since timestamps on cause clear, missing hidden marks
// in history, stale HiddenBehind after cause removal, or false "not hidden"
// history claims after episode eviction.
func TestAlarmShownWhenCauseClears(t *testing.T) {
	t.Run("component-down-shown-with-original-since-on-reconnect", func(t *testing.T) {
		t0 := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 60

		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  storage.CreateMemory(),
			Now:    clk.Now,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true})
		store.OnDeviceConnected(testCauseDev1, t0)

		tDown := clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tDown,
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "peer down"},
		))
		p1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		comp1, ok := findOpenAlarm(p1.Alarms, AlarmKindComponentDown, testCauseDev1, "bgp")
		require.True(t, ok)
		assert.Empty(t, comp1.HiddenBehind)

		// Device goes offline -> component-down is hidden.
		store.OnDeviceDisconnected(testCauseDev1, clk.Advance(10*time.Second))
		store.Tick(clk.Advance(60 * time.Second))
		pOff, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		offOcc, ok := findOpenAlarm(pOff.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
		require.True(t, ok)
		compOff, ok := findOpenAlarm(pOff.Alarms, AlarmKindComponentDown, testCauseDev1, "bgp")
		require.True(t, ok)
		assert.Equal(t, offOcc.ID, compOff.HiddenBehind)
		assert.Equal(t, tDown, compOff.Since)

		// Device reconnects and still reports component down -> shown again with original ID and Since (AC-2).
		tReconn := clk.Advance(30 * time.Second)
		store.OnDeviceConnected(testCauseDev1, tReconn)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tReconn.Add(5*time.Second),
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "peer down"},
		))
		pBack, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		require.Equal(t, 1, pBack.Total)
		assert.Equal(t, comp1.ID, pBack.Alarms[0].ID)
		assert.Equal(t, tDown, pBack.Alarms[0].Since)
		assert.Empty(t, pBack.Alarms[0].HiddenBehind)
	})

	t.Run("alarm-opens-and-clears-while-hidden-marked-in-history", func(t *testing.T) {
		t0 := time.Date(2026, 11, 27, 11, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 60

		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  storage.CreateMemory(),
			Now:    clk.Now,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true})
		store.Tick(clk.Advance(60 * time.Second))

		pOff, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		offOcc, ok := findOpenAlarm(pOff.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
		require.True(t, ok)

		// Component alarm opens while device-offline is open (AC-3, AC-10).
		tOpen := clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tOpen,
			fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDegraded, Reason: "slow"},
		))

		pShown, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(false)})
		require.NoError(t, err)
		_, foundShown := findOpenAlarm(pShown.Alarms, AlarmKindComponentDegraded, testCauseDev1, "ospf")
		assert.False(t, foundShown, "alarm opened while offline must not appear in shown list")

		pHidden, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(true)})
		require.NoError(t, err)
		degOcc, ok := findOpenAlarm(pHidden.Alarms, AlarmKindComponentDegraded, testCauseDev1, "ospf")
		require.True(t, ok)
		assert.Equal(t, offOcc.ID, degOcc.HiddenBehind)

		// Condition stops while still offline.
		tClear := clk.Advance(20 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tClear,
			fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusHealthy},
		))

		pAfter, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		_, stillOpen := findOpenAlarm(pAfter.Alarms, AlarmKindComponentDegraded, testCauseDev1, "ospf")
		assert.False(t, stillOpen)

		// History retains the closed row marked hidden behind device-offline.
		hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		ev, ok := findHistoryEventByID(hist.Events, degOcc.ID)
		require.True(t, ok)
		assert.True(t, ev.Cleared)
		assert.Equal(t, tOpen, ev.Opened)
		assert.Equal(t, tClear, ev.ClearedAt)
		assert.True(t, ev.Hidden)
		assert.Equal(t, offOcc.ID, ev.HiddenBehind)
		assert.False(t, ev.HiddenUnknown)
	})

	t.Run("successive-nearest-cause-clears-preserve-since", func(t *testing.T) {
		reg := defaultAlarmKindRegistry.Clone()
		var activeRoot, activeMidA, activeMidB, activeLeaf bool

		require.NoError(t, reg.Register(AlarmKindSpec{
			Name:     testKindCauseRoot,
			Severity: AlarmSeverityCritical,
			Scope:    AlarmScopeDevice,
			Summary:  "Root cause",
			Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
				if !activeRoot {
					return nil
				}
				return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
			},
		}))
		require.NoError(t, reg.Register(AlarmKindSpec{
			Name:     testKindCauseMidA,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Causes:   []string{testKindCauseRoot},
			Summary:  "Mid cause A",
			Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
				if !activeMidA {
					return nil
				}
				return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
			},
		}))
		require.NoError(t, reg.Register(AlarmKindSpec{
			Name:     testKindCauseMidB,
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Causes:   []string{testKindCauseRoot},
			Summary:  "Mid cause B",
			Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
				if !activeMidB {
					return nil
				}
				return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
			},
		}))
		require.NoError(t, reg.Register(AlarmKindSpec{
			Name:     testKindCauseLeaf,
			Severity: AlarmSeverityInformational,
			Scope:    AlarmScopeDevice,
			Causes:   []string{testKindCauseMidA, testKindCauseMidB},
			Summary:  "Leaf alarm",
			Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
				if !activeLeaf {
					return nil
				}
				return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
			},
		}))
		require.NoError(t, reg.ValidateCauses())

		t0 := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Store:    storage.CreateMemory(),
			Now:      clk.Now,
			Registry: reg,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true})
		store.OnDeviceConnected(testCauseDev1, t0)

		// Open root, mid-a, mid-b, then leaf at tLeaf.
		activeRoot = true
		store.EvaluateExternalKinds()
		clk.Advance(5 * time.Second)
		activeMidA = true
		store.EvaluateExternalKinds()
		clk.Advance(5 * time.Second)
		activeMidB = true
		store.EvaluateExternalKinds()
		tLeaf := clk.Advance(5 * time.Second)
		activeLeaf = true
		store.EvaluateExternalKinds()

		p0, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		rootOcc, ok := findOpenAlarm(p0.Alarms, testKindCauseRoot, testCauseDev1, "")
		require.True(t, ok)
		midAOcc, ok := findOpenAlarm(p0.Alarms, testKindCauseMidA, testCauseDev1, "")
		require.True(t, ok)
		midBOcc, ok := findOpenAlarm(p0.Alarms, testKindCauseMidB, testCauseDev1, "")
		require.True(t, ok)
		leafOcc, ok := findOpenAlarm(p0.Alarms, testKindCauseLeaf, testCauseDev1, "")
		require.True(t, ok)
		require.Less(t, midAOcc.ID, midBOcc.ID)

		// Step 1: leaf hidden behind mid-a (nearest, smallest ID).
		assert.Equal(t, midAOcc.ID, leafOcc.HiddenBehind)
		assert.Equal(t, tLeaf, leafOcc.Since)

		// Step 2: clear mid-a -> leaf stays hidden behind mid-b with same Since.
		clk.Advance(10 * time.Second)
		activeMidA = false
		store.EvaluateExternalKinds()
		p1, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		leaf1, ok := findOpenAlarm(p1.Alarms, testKindCauseLeaf, testCauseDev1, "")
		require.True(t, ok)
		assert.Equal(t, leafOcc.ID, leaf1.ID)
		assert.Equal(t, midBOcc.ID, leaf1.HiddenBehind)
		assert.Equal(t, tLeaf, leaf1.Since)

		// Step 3: clear mid-b -> leaf stays hidden behind root (depth 2) with same Since.
		clk.Advance(10 * time.Second)
		activeMidB = false
		store.EvaluateExternalKinds()
		p2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		leaf2, ok := findOpenAlarm(p2.Alarms, testKindCauseLeaf, testCauseDev1, "")
		require.True(t, ok)
		assert.Equal(t, leafOcc.ID, leaf2.ID)
		assert.Equal(t, rootOcc.ID, leaf2.HiddenBehind)
		assert.Equal(t, tLeaf, leaf2.Since)

		// Step 4: clear root -> leaf is shown with original Since.
		clk.Advance(10 * time.Second)
		activeRoot = false
		store.EvaluateExternalKinds()
		p3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		require.Equal(t, 1, p3.Total)
		assert.Equal(t, leafOcc.ID, p3.Alarms[0].ID)
		assert.Empty(t, p3.Alarms[0].HiddenBehind)
		assert.Equal(t, tLeaf, p3.Alarms[0].Since)
	})

	t.Run("truncated-history-marks-hidden-unknown", func(t *testing.T) {
		t0 := time.Date(2026, 11, 27, 13, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 60
		cfg.MaxEpisodesPerDevice = 2

		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  storage.CreateMemory(),
			Now:    clk.Now,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true})

		// Open device-offline at t0+60s, component-down at t0+70s, then reconnect at t0+90s.
		store.Tick(clk.Advance(60 * time.Second))
		tDown := clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tDown,
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		))
		store.OnConfigState(testCauseDev1, true, "held-1", false, "")

		pOpen, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		offOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
		require.True(t, ok)
		bgpOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindComponentDown, testCauseDev1, "bgp")
		require.True(t, ok)
		divOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindConfigDiverged, testCauseDev1, "")
		require.True(t, ok)

		// Reconnect closes device-offline at t0+90s; bgp stays open.
		store.OnDeviceConnected(testCauseDev1, clk.Advance(20*time.Second))

		// Before eviction, history knows bgp was hidden behind offOcc.ID.
		histBefore, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		bgpHistBefore, ok := findHistoryEventByID(histBefore.Events, bgpOcc.ID)
		require.True(t, ok)
		assert.True(t, bgpHistBefore.Hidden)
		assert.Equal(t, offOcc.ID, bgpHistBefore.HiddenBehind)
		assert.False(t, bgpHistBefore.HiddenUnknown)

		// Open and close 3 unrelated config-rejected episodes to evict device-offline.
		for range 3 {
			clk.Advance(10 * time.Second)
			store.OnConfigState(testCauseDev1, true, "held-1", true, "bad")
			clk.Advance(10 * time.Second)
			store.OnConfigState(testCauseDev1, true, "held-1", false, "")
		}

		// Open a fresh component-down AFTER truncation boundary.
		clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(clk.Now(),
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
			fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDown, Reason: "new"},
		))
		pAfter, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		ospfOcc, ok := findOpenAlarm(pAfter.Alarms, AlarmKindComponentDown, testCauseDev1, "ospf")
		require.True(t, ok)

		histAfter, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		assert.True(t, histAfter.Truncated)

		// bgp opened before earliestRetained and its cause was evicted -> HiddenUnknown=true.
		bgpHistAfter, ok := findHistoryEventByID(histAfter.Events, bgpOcc.ID)
		require.True(t, ok)
		assert.False(t, bgpHistAfter.Hidden)
		assert.Empty(t, bgpHistAfter.HiddenBehind)
		assert.True(t, bgpHistAfter.HiddenUnknown)

		// config-diverged has NO declared causes -> HiddenUnknown stays false.
		divHistAfter, ok := findHistoryEventByID(histAfter.Events, divOcc.ID)
		require.True(t, ok)
		assert.False(t, divHistAfter.Hidden)
		assert.False(t, divHistAfter.HiddenUnknown)

		// ospf opened AFTER earliestRetained -> HiddenUnknown stays false.
		ospfHistAfter, ok := findHistoryEventByID(histAfter.Events, ospfOcc.ID)
		require.True(t, ok)
		assert.False(t, ospfHistAfter.Hidden)
		assert.False(t, ospfHistAfter.HiddenUnknown)
	})

	t.Run("retention-expiry-marks-hidden-unknown", func(t *testing.T) {
		t0 := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
		clk := newAlarmFakeClock(t0)
		cfg := plugin.DefaultHubAlarmsConfig()
		cfg.OfflineDelaySeconds = 60
		cfg.RetentionDays = 7
		cfg.MaxEpisodesPerDevice = 512

		store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
			Config: cfg,
			Store:  storage.CreateMemory(),
			Now:    clk.Now,
		})
		store.SyncDeclarations(map[string]bool{testCauseDev1: true})

		// Day 0: device-offline opens, bgp component-down and config-diverged open,
		// then device reconnects so device-offline closes at t0+90s.
		store.Tick(clk.Advance(60 * time.Second))
		tBGP := clk.Advance(10 * time.Second)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tBGP,
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		))
		store.OnConfigState(testCauseDev1, true, "held-old", false, "")

		pOpen, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		offOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
		require.True(t, ok)
		bgpOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindComponentDown, testCauseDev1, "bgp")
		require.True(t, ok)
		divOcc, ok := findOpenAlarm(pOpen.Alarms, AlarmKindConfigDiverged, testCauseDev1, "")
		require.True(t, ok)

		store.OnDeviceConnected(testCauseDev1, clk.Advance(20*time.Second))

		// Before retention expiry: bgp knows it was hidden behind device-offline.
		histDay0, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		bgpDay0, ok := findHistoryEventByID(histDay0.Events, bgpOcc.ID)
		require.True(t, ok)
		assert.True(t, bgpDay0.Hidden)
		assert.Equal(t, offOcc.ID, bgpDay0.HiddenBehind)
		assert.False(t, bgpDay0.HiddenUnknown)

		// Day 2 (still inside retention window on Day 8): open ospf component-down.
		tOSPF := clk.Advance(2 * 24 * time.Hour)
		store.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(tOSPF,
			fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
			fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDown, Reason: "down"},
		))
		pDay2, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
		require.NoError(t, err)
		ospfOcc, ok := findOpenAlarm(pDay2.Alarms, AlarmKindComponentDown, testCauseDev1, "ospf")
		require.True(t, ok)

		// Advance past 7-day retention so Day 0's closed device-offline is pruned by RetentionDays.
		clk.Advance(6 * 24 * time.Hour)
		histDay8, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
		require.NoError(t, err)
		assert.False(t, histDay8.Truncated, "retention-day expiry alone is not cap truncation")
		_, hasOldOffline := findHistoryEventByID(histDay8.Events, offOcc.ID)
		assert.False(t, hasOldOffline, "closed device-offline older than RetentionDays must be pruned")

		// bgp opened before RetainedSince and its cause expired -> HiddenUnknown=true.
		bgpDay8, ok := findHistoryEventByID(histDay8.Events, bgpOcc.ID)
		require.True(t, ok)
		assert.False(t, bgpDay8.Hidden)
		assert.Empty(t, bgpDay8.HiddenBehind)
		assert.True(t, bgpDay8.HiddenUnknown)

		// config-diverged has no causes -> HiddenUnknown stays false even before RetainedSince.
		divDay8, ok := findHistoryEventByID(histDay8.Events, divOcc.ID)
		require.True(t, ok)
		assert.False(t, divDay8.Hidden)
		assert.False(t, divDay8.HiddenUnknown)

		// ospf opened after RetainedSince -> HiddenUnknown stays false.
		ospfDay8, ok := findHistoryEventByID(histDay8.Events, ospfOcc.ID)
		require.True(t, ok)
		assert.False(t, ospfDay8.Hidden)
		assert.False(t, ospfDay8.HiddenUnknown)
	})
}

// VALIDATES: AC-5 -- a settling alarm (resource-high) closes at reconnect and
// is omitted from both shown and hidden open lists, stays absent one second
// short of ResourceDurationSeconds, opens a new occurrence with a new ID and
// opening time equal to the qualifying observation at the exact threshold
// duration, and resets qualification when a reading drops below threshold.
// PREVENTS: stale pre-reconnect resource pressure immediately reopening on
// reconnect, off-by-one duration boundaries, or below-threshold drops failing
// to restart settling.
func TestAlarmSettlesAfterCauseClears(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 14, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60
	cfg.ResourceCPUPercent = 90
	cfg.ResourceDurationSeconds = 120

	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  storage.CreateMemory(),
		Now:    clk.Now,
	})
	store.SyncDeclarations(map[string]bool{testCauseDev1: true})
	store.OnDeviceConnected(testCauseDev1, t0)

	// Open initial resource-high occurrence #1.
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), t0)
	tOpen1 := clk.Advance(120 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), tOpen1)

	pInit, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	res1, ok := findOpenAlarm(pInit.Alarms, AlarmKindResourceHigh, testCauseDev1, "cpu")
	require.True(t, ok)
	assert.Equal(t, tOpen1, res1.Since)
	assert.Empty(t, res1.HiddenBehind)

	// Disconnect and open device-offline -> resource-high #1 stays open, hidden behind offline.
	store.OnDeviceDisconnected(testCauseDev1, clk.Advance(30*time.Second))
	store.Tick(clk.Advance(60 * time.Second))
	pOff, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	offOcc, ok := findOpenAlarm(pOff.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
	require.True(t, ok)
	resOff, ok := findOpenAlarm(pOff.Alarms, AlarmKindResourceHigh, testCauseDev1, "cpu")
	require.True(t, ok)
	assert.Equal(t, res1.ID, resOff.ID)
	assert.Equal(t, offOcc.ID, resOff.HiddenBehind)

	// Reconnect: old resource-high #1 closes at reconnect and is absent from shown, hidden, and unfiltered lists.
	tReconn := clk.Advance(90 * time.Second)
	store.OnDeviceConnected(testCauseDev1, tReconn)

	pReconnAll, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	assert.Equal(t, 0, pReconnAll.Total)
	pReconnHidden, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(true)})
	require.NoError(t, err)
	assert.Equal(t, 0, pReconnHidden.Total)
	pReconnShown, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(false)})
	require.NoError(t, err)
	assert.Equal(t, 0, pReconnShown.Total)

	histReconn, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
	require.NoError(t, err)
	ev1, ok := findHistoryEventByID(histReconn.Events, res1.ID)
	require.True(t, ok)
	assert.True(t, ev1.Cleared)
	assert.Equal(t, tReconn, ev1.ClearedAt)
	assert.True(t, ev1.Hidden)
	assert.Equal(t, offOcc.ID, ev1.HiddenBehind)

	// Fresh sample #1 starts settling timer.
	tFreshStart := tReconn.Add(10 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 96},
	), tFreshStart)

	// One second short of configured duration (119s < 120s): still absent.
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 96},
	), tFreshStart.Add(119*time.Second))
	pShort, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	assert.Equal(t, 0, pShort.Total, "one sample short of ResourceDurationSeconds must not open resource-high")

	// Exact threshold duration (120s == 120s): opens NEW occurrence #2 with new ID and opening time.
	tOpen2 := tFreshStart.Add(120 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 96},
	), tOpen2)
	pQual, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	require.Equal(t, 1, pQual.Total)
	res2 := pQual.Alarms[0]
	assert.Equal(t, AlarmKindResourceHigh, res2.Kind)
	assert.NotEqual(t, res1.ID, res2.ID)
	assert.Equal(t, tOpen2, res2.Since)
	assert.Empty(t, res2.HiddenBehind)

	// Below-threshold drop during settling resets the interval completely.
	tReconn2 := tOpen2.Add(60 * time.Second)
	store.OnDeviceConnected(testCauseDev1, tReconn2)
	tDropStart := tReconn2.Add(10 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), tDropStart)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 89},
	), tDropStart.Add(60*time.Second))
	// High again at +120s from initial start: must restart from +120s, not open!
	tRestart := tDropStart.Add(120 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), tRestart)
	pReset, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	assert.Equal(t, 0, pReset.Total, "below-threshold reading must reset settling interval")

	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), tRestart.Add(119*time.Second))
	pResetShort, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	assert.Equal(t, 0, pResetShort.Total)

	tOpen3 := tRestart.Add(120 * time.Second)
	store.OnSampleReport(testCauseDev1, makeAlarmSampleReport(
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
	), tOpen3)
	pOpen3, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	require.Equal(t, 1, pOpen3.Total)
	assert.NotEqual(t, res2.ID, pOpen3.Alarms[0].ID)
	assert.Equal(t, tOpen3, pOpen3.Alarms[0].Since)
}

// VALIDATES: AC-7 -- ValidateCauses rejects unknown causes, 1/2/3-kind cause
// cycles, warning hiding critical, informational hiding warning, and hub-scoped
// kinds declaring causes, returning an *opererr.Error naming both kinds and the
// defect; and NewManagedServer refuses startup when the alarm registry is
// invalid.
// PREVENTS: silent misspellings, infinite hiding cycles, severity inversions,
// or hub-scoped causes surviving hub startup.
func TestAlarmKindRegistryRefusesBadCauses(t *testing.T) {
	// Positive control: built-in first-release registry is valid.
	require.NoError(t, defaultAlarmKindRegistry.ValidateCauses())

	cases := []struct {
		name        string
		setup       func(r *AlarmKindRegistry)
		wantSubstrs []string
		wantOperErr bool
	}{
		{
			name: "unknown-cause",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "bad-unknown-child",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"nonexistent-cause"},
				}))
			},
			wantSubstrs: []string{"bad-unknown-child", "nonexistent-cause", "unknown"},
			wantOperErr: true,
		},
		{
			name: "self-cycle-1-kind",
			setup: func(r *AlarmKindRegistry) {
				// Register itself refuses self-cause; also prove ValidateCauses catches
				// a 1-kind self-cycle present in the graph map directly.
				assert.Error(t, r.Register(AlarmKindSpec{
					Name:     "cycle-self",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-self"},
				}))
				r.mu.Lock()
				r.kinds["cycle-self"] = AlarmKindSpec{
					Name:     "cycle-self",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-self"},
				}
				r.mu.Unlock()
			},
			wantSubstrs: []string{"cycle-self", "cycle"},
			wantOperErr: true,
		},
		{
			name: "cycle-2-kinds",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "cycle-a",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-b"},
				}))
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "cycle-b",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-a"},
				}))
			},
			wantSubstrs: []string{"cycle-a", "cycle-b", "cycle"},
			wantOperErr: true,
		},
		{
			name: "cycle-3-kinds",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "cycle-1",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-2"},
				}))
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "cycle-2",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-3"},
				}))
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "cycle-3",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"cycle-1"},
				}))
			},
			wantSubstrs: []string{"cycle-1", "cycle-2", "cycle-3", "cycle"},
			wantOperErr: true,
		},
		{
			name: "warning-hides-critical",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "warn-cause",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
				}))
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "crit-affected",
					Severity: AlarmSeverityCritical,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"warn-cause"},
				}))
			},
			wantSubstrs: []string{"crit-affected", "warn-cause", "critical", "warning"},
			wantOperErr: true,
		},
		{
			name: "informational-hides-warning",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "info-cause",
					Severity: AlarmSeverityInformational,
					Scope:    AlarmScopeDevice,
				}))
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "warn-affected",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{"info-cause"},
				}))
			},
			wantSubstrs: []string{"warn-affected", "info-cause", "warning", "informational"},
			wantOperErr: true,
		},
		{
			name: "hub-scoped-kind-with-cause",
			setup: func(r *AlarmKindRegistry) {
				// Register itself refuses hub-scoped causes; also prove ValidateCauses
				// rejects any hub-scoped kind carrying Causes directly.
				assert.Error(t, r.Register(AlarmKindSpec{
					Name:     "hub-bad-cause",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeHub,
					Causes:   []string{AlarmKindDeviceOffline},
				}))
				r.mu.Lock()
				r.kinds["hub-bad-cause"] = AlarmKindSpec{
					Name:     "hub-bad-cause",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeHub,
					Causes:   []string{AlarmKindDeviceOffline},
				}
				r.mu.Unlock()
			},
			wantSubstrs: []string{"hub-bad-cause", AlarmKindDeviceOffline, "hub-scoped"},
			wantOperErr: true,
		},
		{
			name: "device-kind-declares-hub-scoped-cause",
			setup: func(r *AlarmKindRegistry) {
				require.NoError(t, r.Register(AlarmKindSpec{
					Name:     "dev-cites-hub-cause",
					Severity: AlarmSeverityWarning,
					Scope:    AlarmScopeDevice,
					Causes:   []string{AlarmKindHistoryFull},
				}))
			},
			wantSubstrs: []string{"dev-cites-hub-cause", AlarmKindHistoryFull, "hub-scoped"},
			wantOperErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := defaultAlarmKindRegistry.Clone()
			tc.setup(reg)
			err := reg.ValidateCauses()
			require.Error(t, err)
			if tc.wantOperErr {
				_, isOper := errors.AsType[*opererr.Error](err)
				assert.True(t, isOper, "expected *opererr.Error, got %T: %v", err, err)
			}
			msg := err.Error()
			for _, sub := range tc.wantSubstrs {
				assert.Contains(t, msg, sub)
			}
		})
	}

	// Prove NewManagedServer refuses startup when the package registry has a bad cause.
	t.Run("managed-server-refuses-startup-on-bad-registry", func(t *testing.T) {
		t.Cleanup(SnapshotAlarmKinds())
		require.NoError(t, RegisterAlarmKind(AlarmKindSpec{
			Name:     "startup-bad-kind",
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Causes:   []string{"startup-missing-cause"},
		}))

		_, err := NewManagedServer(ManagedServerConfig{
			Authority: newTestCA(t),
			Store:     storage.CreateMemory(),
			Listeners: []ManagedListener{{
				Addr:    "127.0.0.1:0",
				Clients: map[string]string{testCauseDev1: testClientSecret},
			}},
			ReadConfig: func(string) ([]byte, error) {
				return []byte(testClientConfig), nil
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "startup-bad-kind")
		assert.Contains(t, err.Error(), "startup-missing-cause")
		_, isOper := errors.AsType[*opererr.Error](err)
		assert.True(t, isOper)
	})

	t.Run("managed-server-refuses-startup-on-hub-scoped-cause", func(t *testing.T) {
		t.Cleanup(SnapshotAlarmKinds())
		require.NoError(t, RegisterAlarmKind(AlarmKindSpec{
			Name:     "startup-dev-hub-cause",
			Severity: AlarmSeverityWarning,
			Scope:    AlarmScopeDevice,
			Causes:   []string{AlarmKindHistoryFull},
		}))

		_, err := NewManagedServer(ManagedServerConfig{
			Authority: newTestCA(t),
			Store:     storage.CreateMemory(),
			Listeners: []ManagedListener{{
				Addr:    "127.0.0.1:0",
				Clients: map[string]string{testCauseDev1: testClientSecret},
			}},
			ReadConfig: func(string) ([]byte, error) {
				return []byte(testClientConfig), nil
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "startup-dev-hub-cause")
		assert.Contains(t, err.Error(), AlarmKindHistoryFull)
		_, isOper := errors.AsType[*opererr.Error](err)
		assert.True(t, isOper)
	})
}

// VALIDATES: AC-8 -- an externally registered alarm kind (such as fleet-10's
// config-resync-failed) declaring Causes: []string{AlarmKindDeviceOffline} is
// automatically validated and hidden behind device-offline in QueryOpen and
// QueryHistory without editing any central hub cause list.
// PREVENTS: central cause enumeration that breaks extensibility for later
// alarm kinds.
func TestAlarmKindRegisteredElsewhereIsHidden(t *testing.T) {
	t.Cleanup(SnapshotAlarmKinds())

	const extKind = "test-external-resync-failed"
	resyncFailed := false

	require.NoError(t, RegisterAlarmKind(AlarmKindSpec{
		Name:     extKind,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Causes:   []string{AlarmKindDeviceOffline},
		Summary:  "Configuration resync failed",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			if !resyncFailed {
				return nil
			}
			var out []AlarmObservation
			for _, dev := range ctx.Devices {
				if dev.Name == testCauseDev1 {
					out = append(out, AlarmObservation{
						Device:     dev.Name,
						Summary:    "Configuration resync failed",
						Detail:     "attempt=1",
						ObservedAt: ctx.Now,
					})
				}
			}
			return out
		},
	}))
	require.NoError(t, defaultAlarmKindRegistry.ValidateCauses())

	t0 := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60

	// Use the default package registry (no custom registry injection).
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  storage.CreateMemory(),
		Now:    clk.Now,
	})
	require.NoError(t, store.ValidateRegistry())
	store.SyncDeclarations(map[string]bool{testCauseDev1: true})
	store.OnDeviceConnected(testCauseDev1, t0)

	// Open the external kind while online -> shown with no HiddenBehind.
	clk.Advance(10 * time.Second)
	resyncFailed = true
	store.EvaluateExternalKinds()

	pOnline, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	extOnline, ok := findOpenAlarm(pOnline.Alarms, extKind, testCauseDev1, "")
	require.True(t, ok)
	assert.Empty(t, extOnline.HiddenBehind)

	// Device disconnects and goes offline -> external kind is automatically hidden behind device-offline.
	store.OnDeviceDisconnected(testCauseDev1, clk.Advance(10*time.Second))
	store.Tick(clk.Advance(60 * time.Second))

	pOff, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	offOcc, ok := findOpenAlarm(pOff.Alarms, AlarmKindDeviceOffline, testCauseDev1, "")
	require.True(t, ok)
	extOff, ok := findOpenAlarm(pOff.Alarms, extKind, testCauseDev1, "")
	require.True(t, ok)
	assert.Equal(t, extOnline.ID, extOff.ID)
	assert.Equal(t, offOcc.ID, extOff.HiddenBehind)

	// Hidden filter selects it for true and omits it for false.
	pOnlyHidden, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(true)})
	require.NoError(t, err)
	require.Equal(t, 1, pOnlyHidden.Total)
	assert.Equal(t, extKind, pOnlyHidden.Alarms[0].Kind)
	assert.Equal(t, offOcc.ID, pOnlyHidden.Alarms[0].HiddenBehind)

	pOnlyShown, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1, Hidden: new(false)})
	require.NoError(t, err)
	require.Equal(t, 1, pOnlyShown.Total)
	assert.Equal(t, AlarmKindDeviceOffline, pOnlyShown.Alarms[0].Kind)

	// Clear the external kind while still offline -> history marks it hidden behind device-offline.
	clk.Advance(15 * time.Second)
	resyncFailed = false
	store.EvaluateExternalKinds()

	hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
	require.NoError(t, err)
	ev, ok := findHistoryEventByID(hist.Events, extOnline.ID)
	require.True(t, ok)
	assert.True(t, ev.Cleared)
	assert.True(t, ev.Hidden)
	assert.Equal(t, offOcc.ID, ev.HiddenBehind)
}

// VALIDATES: half-open history interval overlap boundaries in
// applyAlarmHistoryCauses -- adjacent non-overlapping intervals [T0, T1) and
// [T1, T2) in either order do not mark Hidden, while zero-duration
// instantaneous-clear events (Opened == ClearedAt) inside [c0, c1) are marked
// Hidden and at the exclusive end c1 are not.
// PREVENTS: off-by-one boundary overlaps misattributing adjacent causes or
// dropping instantaneous-clear episodes that happened during an outage.
func TestAlarmHistoryIntervalBoundaries(t *testing.T) {
	const (
		evID    = "alm-ev"
		causeID = "alm-cause"
	)
	t0 := time.Date(2026, 11, 27, 16, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Minute)
	t2 := t0.Add(20 * time.Minute)
	tMid := t0.Add(5 * time.Minute)

	cases := []struct {
		name       string
		events     []AlarmHistoryEvent
		targetID   string
		wantHidden bool
		wantBehind string
	}{
		{
			name: "event-ends-exactly-when-cause-opens",
			events: []AlarmHistoryEvent{
				{ID: evID, Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t1},
				{ID: causeID, Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t1, Cleared: true, ClearedAt: t2},
			},
			targetID:   evID,
			wantHidden: false,
			wantBehind: "",
		},
		{
			name: "cause-ends-exactly-when-event-opens",
			events: []AlarmHistoryEvent{
				{ID: causeID, Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t1},
				{ID: evID, Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t1, Cleared: true, ClearedAt: t2},
			},
			targetID:   evID,
			wantHidden: false,
			wantBehind: "",
		},
		{
			name: "instantaneous-clear-at-cause-start",
			events: []AlarmHistoryEvent{
				{ID: causeID, Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t1},
				{ID: evID, Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t0},
			},
			targetID:   evID,
			wantHidden: true,
			wantBehind: causeID,
		},
		{
			name: "instantaneous-clear-inside-cause",
			events: []AlarmHistoryEvent{
				{ID: causeID, Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t1},
				{ID: evID, Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: tMid, Cleared: true, ClearedAt: tMid},
			},
			targetID:   evID,
			wantHidden: true,
			wantBehind: causeID,
		},
		{
			name: "instantaneous-clear-at-exact-cause-end",
			events: []AlarmHistoryEvent{
				{ID: causeID, Kind: AlarmKindDeviceOffline, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t0, Cleared: true, ClearedAt: t1},
				{ID: evID, Kind: AlarmKindComponentDown, Severity: AlarmSeverityCritical, Scope: AlarmScopeDevice, Device: testCauseDev1, Opened: t1, Cleared: true, ClearedAt: t1},
			},
			targetID:   evID,
			wantHidden: false,
			wantBehind: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := slices.Clone(tc.events)
			applyAlarmHistoryCauses(defaultAlarmKindRegistry, events, time.Time{})
			ev, ok := findHistoryEventByID(events, tc.targetID)
			require.True(t, ok)
			assert.Equal(t, tc.wantHidden, ev.Hidden)
			assert.Equal(t, tc.wantBehind, ev.HiddenBehind)
			assert.False(t, ev.HiddenUnknown)
		})
	}
}

func assertNoDerivedHiddenJSONKeys(t *testing.T, v any) {
	t.Helper()
	switch node := v.(type) {
	case map[string]any:
		for k, val := range node {
			assert.NotEqual(t, "hidden", k, "persisted alarm shard must not carry hidden")
			assert.NotEqual(t, "hidden-behind", k, "persisted alarm shard must not carry hidden-behind")
			assert.NotEqual(t, "hidden-unknown", k, "persisted alarm shard must not carry hidden-unknown")
			assertNoDerivedHiddenJSONKeys(t, val)
		}
	case []any:
		for _, elem := range node {
			assertNoDerivedHiddenJSONKeys(t, elem)
		}
	}
}

// VALIDATES: persisted device alarm shards under zefs.KeyFleetAlarmDevice omit
// all derived hidden fields (hidden, hidden-behind, hidden-unknown), and
// LoadFromStore strips any stale derived fields present in a stored shard so
// QueryOpen and QueryHistory recompute hiding purely from live/retained causes.
// PREVENTS: persisting derived hiding state to disk or leaking stale
// hidden-behind values after reload or registry changes.
func TestAlarmHistoryPersistenceStripsStaleHidden(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 17, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	mem := storage.CreateMemory()
	cfg := plugin.DefaultHubAlarmsConfig()
	cfg.OfflineDelaySeconds = 60

	// 1. Flush a store with open device-offline, open hidden component-down, and
	// closed hidden component-degraded; assert persisted JSON has zero hidden keys.
	store1 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	store1.SyncDeclarations(map[string]bool{testCauseDev1: true})
	store1.Tick(clk.Advance(60 * time.Second))
	store1.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(clk.Advance(10*time.Second),
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusDegraded, Reason: "slow"},
	))
	store1.OnHealthReport(testCauseDev1, makeAlarmHealthSnap(clk.Advance(10*time.Second),
		fleet.HealthComponent{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down"},
		fleet.HealthComponent{Name: "ospf", Status: fleet.HealthStatusHealthy},
	))
	require.NoError(t, store1.FlushFinal())

	key := zefs.FleetAlarmDevicePrefix + testCauseDev1
	raw, err := mem.ReadKey(key)
	require.NoError(t, err)
	var decoded any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assertNoDerivedHiddenJSONKeys(t, decoded)

	// 2. Inject stale derived hidden fields into a stored shard containing:
	// - an open component-down with stale hidden-behind and NO open device-offline,
	// - a closed component-degraded with stale hidden/hidden-behind/hidden-unknown and NO overlapping cause,
	// - a closed component-down overlapping a real closed device-offline, with stale hidden-behind.
	const staleHiddenID = "alm-stale"
	staleShard := persistedAlarmDeviceShard{
		Version:  alarmPersistVersion,
		Device:   testCauseDev1,
		Declared: true,
		Open: []AlarmOccurrence{
			{
				ID:           "alm-0000000000000010",
				Kind:         AlarmKindComponentDown,
				Severity:     AlarmSeverityCritical,
				Scope:        AlarmScopeDevice,
				Device:       testCauseDev1,
				Subject:      "bgp",
				Summary:      "Component bgp is down",
				Since:        t0.Add(5 * time.Minute),
				HiddenBehind: staleHiddenID,
			},
		},
		Closed: []AlarmHistoryEvent{
			{
				ID:            "alm-0000000000000001",
				Kind:          AlarmKindComponentDegraded,
				Severity:      AlarmSeverityWarning,
				Scope:         AlarmScopeDevice,
				Device:        testCauseDev1,
				Subject:       "ospf",
				Summary:       "Component ospf is degraded",
				Opened:        t0,
				Cleared:       true,
				ClearedAt:     t0.Add(1 * time.Minute),
				Hidden:        true,
				HiddenBehind:  staleHiddenID,
				HiddenUnknown: true,
			},
			{
				ID:           "alm-0000000000000002",
				Kind:         AlarmKindDeviceOffline,
				Severity:     AlarmSeverityCritical,
				Scope:        AlarmScopeDevice,
				Device:       testCauseDev1,
				Summary:      "Device has no managed connection",
				Opened:       t0.Add(2 * time.Minute),
				Cleared:      true,
				ClearedAt:    t0.Add(4 * time.Minute),
				HiddenBehind: staleHiddenID,
			},
			{
				ID:            "alm-0000000000000003",
				Kind:          AlarmKindComponentDown,
				Severity:      AlarmSeverityCritical,
				Scope:         AlarmScopeDevice,
				Device:        testCauseDev1,
				Subject:       "isis",
				Summary:       "Component isis is down",
				Opened:        t0.Add(3 * time.Minute),
				Cleared:       true,
				ClearedAt:     t0.Add(4 * time.Minute),
				Hidden:        true,
				HiddenBehind:  staleHiddenID,
				HiddenUnknown: true,
			},
		},
	}
	staleRaw, err := json.Marshal(staleShard)
	require.NoError(t, err)
	require.NoError(t, mem.WriteKey(key, staleRaw))

	clk.Set(t0.Add(10 * time.Minute))
	store2 := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Config: cfg,
		Store:  mem,
		Now:    clk.Now,
	})
	require.NoError(t, store2.LoadFromStore())

	// QueryOpen strips stale hidden-behind when no open cause exists.
	pOpen, err := store2.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	require.Equal(t, 1, pOpen.Total)
	assert.Empty(t, pOpen.Alarms[0].HiddenBehind)

	// QueryHistory strips stale values on the un-caused episode and recomputes
	// the real cause ID on the overlapping episode.
	hist, err := store2.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
	require.NoError(t, err)

	evNoCause, ok := findHistoryEventByID(hist.Events, "alm-0000000000000001")
	require.True(t, ok)
	assert.False(t, evNoCause.Hidden)
	assert.Empty(t, evNoCause.HiddenBehind)
	assert.False(t, evNoCause.HiddenUnknown)

	evRealCause, ok := findHistoryEventByID(hist.Events, "alm-0000000000000003")
	require.True(t, ok)
	assert.True(t, evRealCause.Hidden)
	assert.Equal(t, "alm-0000000000000002", evRealCause.HiddenBehind)
	assert.False(t, evRealCause.HiddenUnknown)
}

// VALIDATES: OnDeviceConnected closes only settling kinds that are
// resource-high or transitively caused by device-offline, leaving external
// settling kinds with non-offline causes open across device reconnect.
// PREVENTS: device reconnect clearing unrelated settling alarm kinds.
func TestAlarmReconnectClosesOnlyOfflineSettlingKinds(t *testing.T) {
	reg := defaultAlarmKindRegistry.Clone()
	const (
		offlineSettlingKind   = "test-offline-settling"
		unrelatedSettlingKind = "test-unrelated-settling"
	)
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     offlineSettlingKind,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Causes:   []string{AlarmKindDeviceOffline},
		Settles:  true,
		Summary:  "Offline-caused settling kind",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
		},
	}))
	require.NoError(t, reg.Register(AlarmKindSpec{
		Name:     unrelatedSettlingKind,
		Severity: AlarmSeverityWarning,
		Scope:    AlarmScopeDevice,
		Settles:  true,
		Summary:  "Non-offline settling kind",
		Evaluate: func(ctx AlarmEvalContext) []AlarmObservation {
			return []AlarmObservation{{Device: testCauseDev1, ObservedAt: ctx.Now}}
		},
	}))
	require.NoError(t, reg.ValidateCauses())

	t0 := time.Date(2026, 11, 27, 18, 0, 0, 0, time.UTC)
	clk := newAlarmFakeClock(t0)
	store := NewManagedAlarmStore(ManagedAlarmStoreConfig{
		Store:    storage.CreateMemory(),
		Now:      clk.Now,
		Registry: reg,
	})
	store.SyncDeclarations(map[string]bool{testCauseDev1: true})
	store.EvaluateExternalKinds()

	pBefore, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	offSetBefore, ok := findOpenAlarm(pBefore.Alarms, offlineSettlingKind, testCauseDev1, "")
	require.True(t, ok)
	unrelBefore, ok := findOpenAlarm(pBefore.Alarms, unrelatedSettlingKind, testCauseDev1, "")
	require.True(t, ok)

	// Reconnect via the public OnDeviceConnected path while BOTH evaluators keep
	// emitting: closeSettlingOccurrencesLocked must close offSetBefore.ID at
	// tReconn (causing Evaluate to open a brand-new ID with Since == tReconn),
	// while unrelatedSettlingKind keeps its original ID and Since with no close.
	tReconn := clk.Advance(30 * time.Second)
	store.OnDeviceConnected(testCauseDev1, tReconn)

	pAfter, err := store.QueryOpen(allowAllFleetScope(), AlarmListQuery{Device: testCauseDev1})
	require.NoError(t, err)
	offSetAfter, hasOffSet := findOpenAlarm(pAfter.Alarms, offlineSettlingKind, testCauseDev1, "")
	require.True(t, hasOffSet)
	assert.NotEqual(t, offSetBefore.ID, offSetAfter.ID, "settling kind caused by device-offline must close prior occurrence on reconnect")
	assert.Equal(t, tReconn, offSetAfter.Since)
	unrelAfter, hasUnrel := findOpenAlarm(pAfter.Alarms, unrelatedSettlingKind, testCauseDev1, "")
	require.True(t, hasUnrel, "settling kind without device-offline cause must stay open on reconnect")
	assert.Equal(t, unrelBefore.ID, unrelAfter.ID)
	assert.Equal(t, unrelBefore.Since, unrelAfter.Since)
	assert.NotEqual(t, offSetBefore.ID, unrelAfter.ID)

	hist, err := store.QueryHistory(allowAllFleetScope(), AlarmHistoryQuery{Device: testCauseDev1})
	require.NoError(t, err)
	evOffSet, ok := findHistoryEventByID(hist.Events, offSetBefore.ID)
	require.True(t, ok)
	assert.True(t, evOffSet.Cleared, "prior offline-caused settling occurrence must be closed at reconnect")
	assert.Equal(t, tReconn, evOffSet.ClearedAt)
	evUnrel, ok := findHistoryEventByID(hist.Events, unrelBefore.ID)
	require.True(t, ok)
	assert.False(t, evUnrel.Cleared, "unrelated settling kind must not be closed on reconnect")
}
