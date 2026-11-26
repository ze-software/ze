// VALIDATES: fleet-8 AC-1, AC-2, AC-4, AC-6, AC-10, AC-11, AC-14 and local
// device visibility scoping across fleet sample and legacy fleet commands.
// Exercises show fleet sample latest, show fleet sample series, show fleet
// sample storage, command-specific pipe filters, generic pipe composition
// (first, json, count), pipeline ordering refusals, absent-vs-zero semantics,
// and RBAC device scoping through a real pluginserver.Dispatcher and
// ManagedServer.
// PREVENTS: broken YANG/RPC registration, command-specific pipes swallowing
// generic pipes, device/label/rank allowed after reduce, unauthorized devices
// leaking into totals/rankings/storage/device/health answers, or absent figures
// masquerading as zero.

package managed

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type testStepClock struct {
	clock.RealClock
	mu  sync.Mutex
	now time.Time
}

func newTestStepClock(start time.Time) *testStepClock {
	return &testStepClock{now: start.UTC()}
}

func (c *testStepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testStepClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}

func startHistoryCommandsServer(
	t *testing.T,
	clk clock.Clock,
	clients map[string]map[string]string,
) string {
	t.Helper()
	root := testRoot(t)
	installCA(t, root)
	secrets := make(map[string]string, len(clients))
	for name := range clients {
		secrets[name] = testHubToken
	}
	cfg := pluginserver.ManagedServerConfig{
		Authority: root,
		Clock:     clk,
		Listeners: []pluginserver.ManagedListener{{
			Addr:         "127.0.0.1:0",
			Clients:      secrets,
			ClientLabels: clients,
		}},
		ReadConfig: func(name string) ([]byte, error) { return []byte(name), nil },
		History: plugin.HubHistoryConfig{
			IntervalSeconds:   60,
			RetentionDays:     7,
			DeviceBudgetBytes: fleet.DefaultHistoryPerDeviceBytes,
			HubBudgetBytes:    fleet.DefaultHistoryHubBytes,
		},
	}
	srv, err := pluginserver.NewManagedServer(cfg)
	require.NoError(t, err)
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(srv.Stop)
	return srv.Addrs()[0].String()
}

func sendTestSample(t *testing.T, mc *rpc.MuxConn, now time.Time, readings ...fleet.SampleReading) {
	t.Helper()
	payload, err := fleet.EncodeSampleReport(fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    now.UTC().Format(time.RFC3339),
		Samples:       readings,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err = mc.CallRPC(ctx, fleet.VerbSampleReport, json.RawMessage(payload))
	require.NoError(t, err)
}

func dispatchWithContext(
	t *testing.T,
	server *pluginserver.Server,
	ctx *pluginserver.CommandContext,
	line string,
) (*plugin.Response, error) {
	t.Helper()
	return server.Dispatcher().Dispatch(ctx, line)
}

func decodePipeData[T any](t *testing.T, server *pluginserver.Server, ctx *pluginserver.CommandContext, line string) T {
	t.Helper()
	resp, err := dispatchWithContext(t, server, ctx, line)
	require.NoError(t, err, line)
	require.NotNil(t, resp, line)
	require.Equal(t, plugin.StatusDone, resp.Status, "%s: %s", line, resp.Error)
	raw, err := plugin.ResponseJSON(resp, nil)
	require.NoError(t, err, line)

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if unmarshalErr := json.Unmarshal([]byte(raw), &envelope); unmarshalErr == nil && len(envelope.Data) > 0 {
		var out T
		require.NoError(t, json.Unmarshal(envelope.Data, &out), "%s returned %s", line, raw)
		return out
	}

	var out T
	require.NoError(t, json.Unmarshal([]byte(raw), &out), "%s returned %s", line, raw)
	return out
}

func buildFleetScopeTestStore() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "west-only",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet sample (latest|series|storage).*$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet (devices|health).*$", Regex: true},
				{Number: 30, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
				{Number: 40, Action: authz.Allow, Match: "^show fleet device (inventory|health) name edge-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("west-op", []string{"west-only"})
	return store
}

// TestShowFleetSampleCommandsAndPipeComposition proves that show fleet sample
// latest, series, and storage dispatch over a real pluginserver.Dispatcher,
// compose command-specific pipes with generic pipes (first, json, count),
// distinguish absent figures from measured zero and no-reports, and preserve
// series gaps and reduced points.
//
// VALIDATES: AC-1, AC-2, AC-4, AC-6, AC-10, AC-11, AC-14.
// PREVENTS: broken command registration, broken pipe partitioning, or false
// zeros for absent figures.
func TestShowFleetSampleCommandsAndPipeComposition(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute)
	clk := newTestStepClock(base)
	addr := startHistoryCommandsServer(t, clk, map[string]map[string]string{
		"edge-01": {"region": "west", "role": "edge"},
		"edge-02": {"region": "west", "role": "core"},
		"edge-03": {"region": "west", "role": "edge"},
		"core-01": {"region": "east", "role": "core"},
	})
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	mcEdge1 := connectReportCommandClient(t, addr, "edge-01")
	mcEdge2 := connectReportCommandClient(t, addr, "edge-02")
	mcCore1 := connectReportCommandClient(t, addr, "core-01")

	// Tick 0 at base:
	// - edge-01 reports cpu=80, mem=400/1000, routes-held=0 (measured zero), no subscriber-sessions (absent).
	// - edge-02 reports cpu=40, mem=600/1000, routes-held=500, subscriber-sessions=25.
	// - core-01 reports cpu=95, mem=800/1000, routes-held=1000.
	// - edge-03 never reports (no-reports).
	sendTestSample(t, mcEdge1, base,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 80},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 400},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 0},
	)
	sendTestSample(t, mcEdge2, base,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 40},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 600},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 500},
		fleet.SampleReading{Figure: fleet.FigureSubscriberSessions, Value: 25},
	)
	sendTestSample(t, mcCore1, base,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 800},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 1000},
	)

	// Skip minute +1 (gap for all devices), then send tick at minute +2.
	t2 := clk.Advance(2 * time.Minute)
	sendTestSample(t, mcEdge1, t2,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 60},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 400},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 0},
	)
	sendTestSample(t, mcEdge2, t2,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 20},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 600},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 500},
		fleet.SampleReading{Figure: fleet.FigureSubscriberSessions, Value: 25},
	)
	sendTestSample(t, mcCore1, t2,
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 95},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 800},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1000},
		fleet.SampleReading{Figure: fleet.FigureRoutesHeld, Value: 1000},
	)

	// 1. Bare latest: verify measured zero vs absent figure vs no-reports.
	allLatest := decodePipeData[[]pluginserver.HistoryLatestRow](t, server, ctx, fleetSampleLatestPath)
	require.Len(t, allLatest, 4)
	byDevice := make(map[string]pluginserver.HistoryLatestRow, len(allLatest))
	for _, row := range allLatest {
		byDevice[row.Device] = row
	}
	edge1 := byDevice["edge-01"]
	require.True(t, edge1.HasReport)
	assert.Equal(t, "ok", edge1.Status)
	routesVal, hasRoutes := edge1.Figures[fleet.FigureRoutesHeld]
	assert.True(t, hasRoutes, "measured zero routes-held must be present")
	assert.Equal(t, 0.0, routesVal)
	_, hasSubs := edge1.Figures[fleet.FigureSubscriberSessions]
	assert.False(t, hasSubs, "absent optional figure must not appear as zero")

	edge3 := byDevice["edge-03"]
	assert.False(t, edge3.HasReport)
	assert.Equal(t, "no-reports", edge3.Status)
	assert.Empty(t, edge3.Figures)

	// 2. Command-specific rank + generic first 2 + generic json composition.
	top2 := decodePipeData[[]pluginserver.HistoryLatestRow](
		t, server, ctx,
		fleetSampleLatestPath+" | rank cpu-used | first 2 | json",
	)
	require.Len(t, top2, 2)
	assert.Equal(t, "core-01", top2[0].Device)
	assert.Equal(t, 95.0, top2[0].Figures[fleet.FigureCPUUsed])
	assert.Equal(t, "edge-01", top2[1].Device)
	assert.Equal(t, 60.0, top2[1].Figures[fleet.FigureCPUUsed])

	// 3. Label intersection + reduce on latest.
	westReduced := decodePipeData[[]pluginserver.HistoryLatestReducedRow](
		t, server, ctx,
		fleetSampleLatestPath+" | label region=west | reduce",
	)
	reducedByFig := make(map[string]pluginserver.HistoryLatestReducedRow, len(westReduced))
	for _, row := range westReduced {
		reducedByFig[row.Figure] = row
	}
	cpuRow := reducedByFig[fleet.FigureCPUUsed]
	require.NotNil(t, cpuRow.Value)
	assert.InDelta(t, 40.0, *cpuRow.Value, 1e-9, "mean of 60 and 20")
	assert.Equal(t, 2, cpuRow.Contributors)
	assert.Equal(t, 1, cpuRow.Missing, "edge-03 has no reports")

	memRow := reducedByFig[fleet.FigureMemoryUsed]
	require.NotNil(t, memRow.Value)
	assert.InDelta(t, 1000.0, *memRow.Value, 1e-9)
	require.NotNil(t, memRow.Ratio)
	assert.InDelta(t, 0.5, *memRow.Ratio, 1e-9, "paired ratio (400+600)/(1000+1000)")

	subsRow := reducedByFig[fleet.FigureSubscriberSessions]
	require.NotNil(t, subsRow.Value)
	assert.InDelta(t, 25.0, *subsRow.Value, 1e-9)
	assert.Equal(t, 1, subsRow.Contributors)
	assert.Equal(t, 2, subsRow.Missing, "edge-01 absent and edge-03 no-reports")

	// figure memory-used | reduce and reduce | figure memory-used both preserve paired ratio.
	memBefore := decodePipeData[[]pluginserver.HistoryLatestReducedRow](
		t, server, ctx,
		fleetSampleLatestPath+" | label region=west | figure memory-used | reduce",
	)
	memAfter := decodePipeData[[]pluginserver.HistoryLatestReducedRow](
		t, server, ctx,
		fleetSampleLatestPath+" | label region=west | reduce | figure memory-used",
	)
	require.Len(t, memBefore, 1)
	require.Equal(t, memBefore, memAfter)
	require.NotNil(t, memBefore[0].Ratio)
	assert.InDelta(t, 0.5, *memBefore[0].Ratio, 1e-9)

	// Repeated label filters intersect.
	westEdge := decodePipeData[[]pluginserver.HistoryLatestRow](
		t, server, ctx,
		fleetSampleLatestPath+" | label region=west | label role=edge",
	)
	require.Len(t, westEdge, 2)
	assert.Equal(t, "edge-01", westEdge[0].Device)
	assert.Equal(t, "edge-03", westEdge[1].Device)

	// Generic count composes on latest.
	countDoc := decodePipeData[map[string]any](
		t, server, ctx,
		fleetSampleLatestPath+" | label region=west | count",
	)
	assert.Equal(t, float64(3), countDoc["count"])

	// 4. Series query with gap preservation and reduce + points.
	// Advance clock to minute +3 so all 4 buckets [base, base+4m) lie inside
	// the retained horizon (with minute +1 and minute +3 as explicit gaps).
	clk.Advance(time.Minute)
	fromStr := base.Format(time.RFC3339)
	untilStr := base.Add(4 * time.Minute).Format(time.RFC3339)

	edge1Series := decodePipeData[[]pluginserver.HistorySeriesRow](
		t, server, ctx,
		fleetSampleSeriesPath+" | device edge-01 | figure cpu-used | from "+fromStr+" | until "+untilStr,
	)
	require.Len(t, edge1Series, 4)
	assert.False(t, edge1Series[0].Gap)
	require.NotNil(t, edge1Series[0].Value)
	assert.InDelta(t, 80.0, *edge1Series[0].Value, 1e-9)
	assert.True(t, edge1Series[1].Gap, "minute +1 had no report and must be an explicit gap")
	assert.Nil(t, edge1Series[1].Value)
	assert.False(t, edge1Series[2].Gap)
	require.NotNil(t, edge1Series[2].Value)
	assert.InDelta(t, 60.0, *edge1Series[2].Value, 1e-9)
	assert.True(t, edge1Series[3].Gap)

	westSeriesPoints := decodePipeData[[]pluginserver.HistorySeriesRow](
		t, server, ctx,
		fleetSampleSeriesPath+" | label region=west | figure cpu-used | from "+fromStr+" | until "+untilStr+" | reduce | points 2",
	)
	require.Len(t, westSeriesPoints, 2)
	assert.False(t, westSeriesPoints[0].Gap)
	require.NotNil(t, westSeriesPoints[0].Value)
	assert.InDelta(t, 60.0, *westSeriesPoints[0].Value, 1e-9, "mean of 80 and 40")
	assert.False(t, westSeriesPoints[1].Gap)
	require.NotNil(t, westSeriesPoints[1].Value)
	assert.InDelta(t, 40.0, *westSeriesPoints[1].Value, 1e-9, "mean of 60 and 20")

	// 5. Storage query.
	storage := decodePipeData[pluginserver.HistoryStorageStatus](t, server, ctx, fleetSampleStoragePath)
	assert.Equal(t, 60, storage.IntervalSeconds)
	assert.Equal(t, 7, storage.ConfiguredRetentionDays)
	assert.Greater(t, storage.UsedBytes, int64(0))
	assert.Equal(t, 3, storage.DeviceCount)
	require.Len(t, storage.Devices, 3)
}

// TestShowFleetSamplePipelineRefusalsAndCompletion checks every disallowed pipe
// ordering, malformed argument, out-of-range points, not-serving refusal, and
// Tab completion for the three fleet sample commands.
//
// VALIDATES: spec L285-L292 pipe refusals and completion surface.
// PREVENTS: invalid queries returning misleading empty answers or silent
// mis-ordered execution.
func TestShowFleetSamplePipelineRefusalsAndCompletion(t *testing.T) {
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	// Not serving: all three sample commands refuse with operator errors.
	for _, cmd := range []string{fleetSampleLatestPath, fleetSampleSeriesPath, fleetSampleStoragePath} {
		resp, err := dispatchWithContext(t, server, ctx, cmd)
		require.NoError(t, err, cmd)
		require.Equal(t, plugin.StatusError, resp.Status, cmd)
		assert.Contains(t, resp.Error, "unable to read fleet metric history", cmd)
		assert.Contains(t, resp.Error, "ze show doctor", cmd)
	}

	// Start serving and test pipeline refusals.
	base := time.Now().UTC().Truncate(time.Minute)
	startHistoryCommandsServer(t, newTestStepClock(base), map[string]map[string]string{
		"edge-01": {"region": "west"},
	})

	refusals := []struct {
		line string
		want string
	}{
		{fleetSampleLatestPath + " | reduce | device edge-01", "cannot filter by device after reduce"},
		{fleetSampleLatestPath + " | reduce | label region=west", "cannot filter by label after reduce"},
		{fleetSampleLatestPath + " | reduce | rank cpu-used", "cannot rank after reduce"},
		{fleetSampleLatestPath + " | reduce | reduce", "cannot apply reduce more than once"},
		{fleetSampleLatestPath + " | figure bogus-metric", "unknown fleet sample figure bogus-metric"},
		{fleetSampleLatestPath + " | rank bogus-metric", "unknown fleet sample figure bogus-metric"},
		{fleetSampleLatestPath + " | label missing-equals", "key=value"},
		{fleetSampleSeriesPath + " | reduce | device edge-01", "cannot filter by device after reduce"},
		{fleetSampleSeriesPath + " | reduce | label region=west", "cannot filter by label after reduce"},
		{fleetSampleSeriesPath + " | figure bogus-metric", "unknown fleet sample figure bogus-metric"},
		{fleetSampleSeriesPath + " | label =west", "key=value"},
		{fleetSampleSeriesPath + " | from 2026-11-25T00:00:00Z | until 2026-11-24T00:00:00Z", "empty or reversed time range"},
		{fleetSampleSeriesPath + " | from 2026-11-24T00:00:00Z | until 2026-11-24T00:00:00Z", "empty or reversed time range"},
		{fleetSampleSeriesPath + " | from not-a-time", "invalid from timestamp"},
		{fleetSampleSeriesPath + " | from 2026-11-24T02:00:00+02:00", "must be in UTC"},
		{fleetSampleSeriesPath + " | points 2113", "invalid points count 2113"},
		{fleetSampleSeriesPath + " | points 0", "positive number"},
		{fleetSampleSeriesPath + " | points 10 | reduce", "cannot apply reduce after points"},
		{fleetSampleSeriesPath + " | points 10 | from 2026-11-24T00:00:00Z", "cannot set from after points"},
	}
	for _, tc := range refusals {
		resp, err := dispatchWithContext(t, server, ctx, tc.line)
		if err != nil {
			assert.Contains(t, err.Error(), tc.want, tc.line)
			continue
		}
		require.NotNil(t, resp, tc.line)
		require.Equal(t, plugin.StatusError, resp.Status, tc.line)
		assert.Contains(t, resp.Error, tc.want, tc.line)
	}

	// Verify Tab completion exposes declared command-specific pipe filters.
	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	completer := command.NewTreeCompleter(configyang.BuildCommandTree(schema))
	for _, tc := range []struct {
		line string
		want []string
	}{
		{fleetSampleLatestPath + " | ran", []string{"rank"}},
		{fleetSampleLatestPath + " | red", []string{"reduce"}},
		{fleetSampleSeriesPath + " | poi", []string{"points"}},
		{fleetSampleSeriesPath + " | unt", []string{"until"}},
	} {
		var got []string
		for _, s := range completer.Complete(tc.line) {
			got = append(got, s.Text)
		}
		assert.Equal(t, tc.want, got, tc.line)
	}
}

// TestFleetCommandsEnforceDeviceScope proves that local RBAC device visibility
// scopes show fleet sample latest, series, and storage as well as show fleet
// devices, device name, device inventory, device health, and fleet health, and
// makes denied devices indistinguishable from absent devices.
//
// VALIDATES: AC-10 and fleet-8 local device-scope projection across all fleet
// commands.
// PREVENTS: unauthorized devices leaking into aggregates, rankings, storage
// usage lists, or legacy fleet registry/report views.
func TestFleetCommandsEnforceDeviceScope(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute)
	clk := newTestStepClock(base)
	addr := startHistoryCommandsServer(t, clk, map[string]map[string]string{
		"edge-01": {"region": "west"},
		"edge-02": {"region": "west"},
		"core-01": {"region": "east"},
	})

	mcEdge1 := connectReportCommandClient(t, addr, "edge-01")
	mcEdge2 := connectReportCommandClient(t, addr, "edge-02")
	mcCore1 := connectReportCommandClient(t, addr, "core-01")

	sendTestSample(t, mcEdge1, base, fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 40})
	sendTestSample(t, mcEdge2, base, fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 20})
	sendTestSample(t, mcCore1, base, fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 98})

	sendTestHealth(t, mcEdge1, fleet.HealthReport{
		Status:    fleet.HealthStatusHealthy,
		CheckedAt: base.Format(time.RFC3339),
	})
	sendTestHealth(t, mcCore1, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: base.Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "down", CheckedAt: base.Format(time.RFC3339)},
		},
	})

	server := registryDispatcher(t)
	server.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: buildFleetScopeTestStore()})
	westCtx := &pluginserver.CommandContext{Server: server, Username: "west-op"}

	// 1. Latest excludes core-01; denied device yields same empty answer as absent.
	latest := decodePipeData[[]pluginserver.HistoryLatestRow](t, server, westCtx, fleetSampleLatestPath)
	require.Len(t, latest, 2)
	assert.Equal(t, "edge-01", latest[0].Device)
	assert.Equal(t, "edge-02", latest[1].Device)

	deniedLatest := decodePipeData[[]pluginserver.HistoryLatestRow](t, server, westCtx, fleetSampleLatestPath+" | device core-01")
	absentLatest := decodePipeData[[]pluginserver.HistoryLatestRow](t, server, westCtx, fleetSampleLatestPath+" | device nonexistent")
	assert.Empty(t, deniedLatest)
	assert.Equal(t, absentLatest, deniedLatest)

	// Reduce averages only edge-01 (40) and edge-02 (20) -> 30, never core-01 (98).
	reduced := decodePipeData[[]pluginserver.HistoryLatestReducedRow](t, server, westCtx, fleetSampleLatestPath+" | figure cpu-used | reduce")
	require.Len(t, reduced, 1)
	require.NotNil(t, reduced[0].Value)
	assert.InDelta(t, 30.0, *reduced[0].Value, 1e-9)
	assert.Equal(t, 2, reduced[0].Contributors)

	// Rank omits core-01 even though core-01 has highest CPU.
	ranked := decodePipeData[[]pluginserver.HistoryLatestRow](t, server, westCtx, fleetSampleLatestPath+" | rank cpu-used")
	require.Len(t, ranked, 2)
	assert.Equal(t, "edge-01", ranked[0].Device)
	assert.Equal(t, "edge-02", ranked[1].Device)

	// 2. Series: denied device matches absent device (empty).
	fromStr := base.Format(time.RFC3339)
	untilStr := base.Add(2 * time.Minute).Format(time.RFC3339)
	deniedSeries := decodePipeData[[]pluginserver.HistorySeriesRow](
		t, server, westCtx,
		fleetSampleSeriesPath+" | device core-01 | figure cpu-used | from "+fromStr+" | until "+untilStr,
	)
	absentSeries := decodePipeData[[]pluginserver.HistorySeriesRow](
		t, server, westCtx,
		fleetSampleSeriesPath+" | device nonexistent | figure cpu-used | from "+fromStr+" | until "+untilStr,
	)
	assert.Empty(t, deniedSeries)
	assert.Equal(t, absentSeries, deniedSeries)

	// 3. Storage filters per-device slice to authorized devices only.
	storage := decodePipeData[pluginserver.HistoryStorageStatus](t, server, westCtx, fleetSampleStoragePath)
	assert.Equal(t, 3, storage.DeviceCount, "hub-wide device count reflects hub capacity")
	require.Len(t, storage.Devices, 2, "per-device usage list must omit unauthorized core-01")
	for _, dev := range storage.Devices {
		assert.NotEqual(t, "core-01", dev.Device)
	}

	// 4. Legacy fleet commands honor the same local device scope.
	devices := decodePipeData[[]fleetDeviceRow](t, server, westCtx, fleetDevicesPath)
	require.Len(t, devices, 2)
	assert.Equal(t, "edge-01", devices[0].Name)
	assert.Equal(t, "edge-02", devices[1].Name)

	// Denied device detail/inventory/health produce the same unknown-device refusal as absent.
	for _, path := range []string{fleetDevicePath, fleetDeviceInventoryPath, fleetDeviceHealthPath} {
		// Dispatch directly to handler with westCtx (bypassing command-level
		// authz refusal for core-01) so we verify the handler's own scope check
		// produces the same unknown-device refusal as an absent device.
		validatedDenied, vErr := command.ValidateModelArgs(path, []string{"core-01"}, nil)
		require.NoError(t, vErr, path)
		validatedAbsent, vErr := command.ValidateModelArgs(path, []string{"nonexistent"}, nil)
		require.NoError(t, vErr, path)

		var handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
		switch path {
		case fleetDevicePath:
			handler = handleFleetDevice
		case fleetDeviceInventoryPath:
			handler = handleFleetDeviceInventory
		case fleetDeviceHealthPath:
			handler = handleFleetDeviceHealth
		}
		deniedResp, hErr := handler(westCtx, validatedDenied)
		require.NoError(t, hErr, path)
		absentResp, hErr := handler(westCtx, validatedAbsent)
		require.NoError(t, hErr, path)
		require.Equal(t, plugin.StatusError, deniedResp.Status, path)
		require.Equal(t, plugin.StatusError, absentResp.Status, path)
		assert.Contains(t, deniedResp.Error, "unknown device", path)
		assert.Contains(t, absentResp.Error, "unknown device", path)
	}

	// show fleet health counts only edge-01 (healthy) and edge-02 (unreported); omits core-01 (down).
	health := decodePipeData[fleetHealthSummaryDoc](t, server, westCtx, fleetHealthPath)
	assert.Equal(t, 2, health.Total)
	assert.Equal(t, 1, health.Healthy)
	assert.Equal(t, 0, health.Degraded)
	assert.Equal(t, 0, health.Down)
	assert.Equal(t, 1, health.Unreported)
	assert.Empty(t, health.Devices)

	// 5. A nil CommandContext MUST fail closed across every fleet handler.
	nilDevicesResp, err := handleFleetDevices(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilDevicesResp.Status)
	nilDevicesRaw, err := plugin.ResponseJSON(nilDevicesResp, nil)
	require.NoError(t, err)
	var nilDevices []fleetDeviceRow
	require.NoError(t, json.Unmarshal([]byte(nilDevicesRaw), &nilDevices))
	assert.Empty(t, nilDevices)

	for _, tc := range []struct {
		path    string
		handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
	}{
		{fleetDevicePath, handleFleetDevice},
		{fleetDeviceInventoryPath, handleFleetDeviceInventory},
		{fleetDeviceHealthPath, handleFleetDeviceHealth},
	} {
		validated, vErr := command.ValidateModelArgs(tc.path, []string{"edge-01"}, nil)
		require.NoError(t, vErr, tc.path)
		resp, hErr := tc.handler(nil, validated)
		require.NoError(t, hErr, tc.path)
		require.Equal(t, plugin.StatusError, resp.Status, tc.path)
		assert.Contains(t, resp.Error, "unknown device", tc.path)
	}

	nilHealthResp, err := handleFleetHealth(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilHealthResp.Status)
	nilHealthRaw, err := plugin.ResponseJSON(nilHealthResp, nil)
	require.NoError(t, err)
	var nilHealth fleetHealthSummaryDoc
	require.NoError(t, json.Unmarshal([]byte(nilHealthRaw), &nilHealth))
	assert.Equal(t, 0, nilHealth.Total)
	assert.Empty(t, nilHealth.Devices)

	nilLatestResp, err := handleFleetSampleLatest(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilLatestResp.Status)
	nilLatestRaw, err := plugin.ResponseJSON(nilLatestResp, nil)
	require.NoError(t, err)
	var nilLatest []pluginserver.HistoryLatestRow
	require.NoError(t, json.Unmarshal([]byte(nilLatestRaw), &nilLatest))
	assert.Empty(t, nilLatest)

	nilSeriesResp, err := handleFleetSampleSeries(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilSeriesResp.Status)
	nilSeriesRaw, err := plugin.ResponseJSON(nilSeriesResp, nil)
	require.NoError(t, err)
	var nilSeries []pluginserver.HistorySeriesRow
	require.NoError(t, json.Unmarshal([]byte(nilSeriesRaw), &nilSeries))
	assert.Empty(t, nilSeries)

	nilStorageResp, err := handleFleetSampleStorage(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilStorageResp.Status)
	nilStorageRaw, err := plugin.ResponseJSON(nilStorageResp, nil)
	require.NoError(t, err)
	var nilStorage pluginserver.HistoryStorageStatus
	require.NoError(t, json.Unmarshal([]byte(nilStorageRaw), &nilStorage))
	assert.Empty(t, nilStorage.Devices)
}
