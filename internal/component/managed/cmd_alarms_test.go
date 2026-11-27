// VALIDATES: fleet-ui-5 AC-4, AC-8, AC-9, AC-13 and local device visibility
// scoping for show fleet alarm and show fleet alarm history device.
// Exercises command registration (shapes, columns, pipe filters, RPC handlers,
// Tab completion), pipe parsing and validation refusals, pagination cursors,
// generic pipe composition via command.ApplyPipes and
// command.ProcessPipesDefaultFormatChecked, and RBAC fail-closed device
// scoping through a real pluginserver.Dispatcher and ManagedServer.
// PREVENTS: missing command or pipe registration, parent pipe filter leakage
// onto show fleet alarm history device, invalid filter/limit/cursor inputs
// returning empty answers instead of operator refusals, ShapeTab envelope
// incompatibilities with generic pipes, or unauthorized devices leaking
// existence or counts.

package managed

import (
	"encoding/json"
	"strings"
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
)

func startAlarmCommandsServer(
	t *testing.T,
	clk clock.Clock,
	clients ...string,
) string {
	t.Helper()
	root := testRoot(t)
	installCA(t, root)
	secrets := make(map[string]string, len(clients))
	for _, name := range clients {
		secrets[name] = testHubToken
	}
	cfg := pluginserver.ManagedServerConfig{
		Authority: root,
		Clock:     clk,
		Listeners: []pluginserver.ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: secrets,
		}},
		ReadConfig: func(name string) ([]byte, error) { return []byte(name), nil },
	}
	srv, err := pluginserver.NewManagedServer(cfg)
	require.NoError(t, err)
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(srv.Stop)
	return srv.Addrs()[0].String()
}

func buildAlarmScopeTestStore() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "west-alarm-only",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet alarm($| \\|.*)$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet alarm history device edge-.*$", Regex: true},
				{Number: 30, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("west-op", []string{"west-alarm-only"})
	return store
}

// TestFleetAlarmCommandRegistrationAndCompletion proves that both alarm
// commands declare ShapeTab, envelope and row ColumnOrders, isolated
// command-owned PipeFilters, and Tab completion for both pipe filters and
// managed device names.
//
// VALIDATES: fleet-ui-5 AC-8 command registration and completion surface.
// PREVENTS: missing shape/column/filter registrations or show fleet alarm
// history device inheriting severity/kind/device filters from show fleet alarm.
func TestFleetAlarmCommandRegistrationAndCompletion(t *testing.T) {
	openShape, openHasShape := command.ShapeForCommand(fleetAlarmPath)
	require.True(t, openHasShape)
	assert.Equal(t, command.ShapeTab, openShape)

	histShape, histHasShape := command.ShapeForCommand(fleetAlarmHistoryPath)
	require.True(t, histHasShape)
	assert.Equal(t, command.ShapeTab, histShape)

	openCols := command.ColumnsForCommand(fleetAlarmPath)
	require.Len(t, openCols, 2)
	assert.Equal(t, command.ColumnOrder{
		"total", "counts", "has-more", "cursor", "snapshot-id",
		"degraded", "degraded-reason", "alarms",
	}, openCols[0])
	assert.Equal(t, command.ColumnOrder{
		"id", "kind", "severity", "scope", "device",
		"subject", "summary", "detail", "since", "hidden-behind",
	}, openCols[1])

	histCols := command.ColumnsForCommand(fleetAlarmHistoryPath)
	require.Len(t, histCols, 2)
	assert.Equal(t, command.ColumnOrder{
		"device", "total", "has-more", "cursor", "snapshot-id",
		"retained-since", "truncated", "degraded", "degraded-reason", "events",
	}, histCols[0])
	assert.Equal(t, command.ColumnOrder{
		"id", "kind", "severity", "scope", "device",
		"subject", "summary", "detail", "opened", "cleared", "cleared-at",
	}, histCols[1])

	openFilters := command.PipeFiltersForCommand(fleetAlarmPath)
	require.Len(t, openFilters, 5)
	openFilterArgs := make(map[string]command.PipeArgKind, len(openFilters))
	for _, f := range openFilters {
		openFilterArgs[f.Name] = f.Argument
		assert.NotEmpty(t, f.Description, f.Name)
	}
	assert.Equal(t, map[string]command.PipeArgKind{
		pipeOpSeverity: command.ArgText,
		pipeOpKind:     command.ArgText,
		pipeOpDevice:   command.ArgText,
		pipeOpCursor:   command.ArgText,
		pipeOpLimit:    command.ArgCount,
	}, openFilterArgs)

	histFilters := command.PipeFiltersForCommand(fleetAlarmHistoryPath)
	require.Len(t, histFilters, 2)
	histFilterArgs := make(map[string]command.PipeArgKind, len(histFilters))
	for _, f := range histFilters {
		histFilterArgs[f.Name] = f.Argument
		assert.NotEmpty(t, f.Description, f.Name)
	}
	assert.Equal(t, map[string]command.PipeArgKind{
		pipeOpCursor: command.ArgText,
		pipeOpLimit:  command.ArgCount,
	}, histFilterArgs)

	// Verify Tab completion exposes each command's declared pipe filters
	// without leaking parent filters onto the history command.
	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	completer := command.NewTreeCompleter(configyang.BuildCommandTree(schema))
	for _, tc := range []struct {
		line string
		want []string
	}{
		{fleetAlarmPath + " | sev", []string{"severity"}},
		{fleetAlarmPath + " | kin", []string{"kind"}},
		{fleetAlarmPath + " | dev", []string{"device"}},
		{fleetAlarmPath + " | cur", []string{"cursor"}},
		{fleetAlarmPath + " | lim", []string{"limit"}},
		{fleetAlarmHistoryPath + " edge-01 | cur", []string{"cursor"}},
		{fleetAlarmHistoryPath + " edge-01 | lim", []string{"limit"}},
		{fleetAlarmHistoryPath + " edge-01 | sev", nil},
	} {
		var got []string
		for _, s := range completer.Complete(tc.line) {
			got = append(got, s.Text)
		}
		assert.Equal(t, tc.want, got, tc.line)
	}

	// Verify the history command's device-name completer returns declared devices.
	startAlarmCommandsServer(t, newTestStepClock(time.Now().UTC()), "edge-01", "edge-02")
	completions := completeFleetDevices(nil, nil)
	var deviceNames []string
	for _, c := range completions {
		deviceNames = append(deviceNames, c.Value)
	}
	assert.Equal(t, []string{"edge-01", "edge-02"}, deviceNames)
}

// TestShowFleetAlarmNotServingRefusal verifies that both alarm commands return
// operator errors naming ze show doctor when no managed server is published.
//
// VALIDATES: operator error contract when the hub is not serving.
// PREVENTS: nil-pointer panics or silent empty responses when not serving.
func TestShowFleetAlarmNotServingRefusal(t *testing.T) {
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	for _, line := range []string{
		fleetAlarmPath,
		fleetAlarmHistoryPath + " edge-01",
	} {
		resp, err := dispatchWithContext(t, server, ctx, line)
		require.NoError(t, err, line)
		require.NotNil(t, resp, line)
		require.Equal(t, plugin.StatusError, resp.Status, line)
		assert.Contains(t, resp.Error, "unable to read fleet alarms", line)
		assert.Contains(t, resp.Error, exampleShowDoctor, line)
	}
}

// TestShowFleetAlarmPipeValidationRefusals verifies that invalid severities,
// unknown kinds, conflicting filters, out-of-range or duplicate limits,
// duplicate or malformed cursors, and unknown direct pipe operations are
// rejected with clear operator errors.
//
// VALIDATES: fleet-ui-5 AC-8 and AC-13 query validation.
// PREVENTS: invalid filters silently returning empty pages or panicking.
func TestShowFleetAlarmPipeValidationRefusals(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	startAlarmCommandsServer(t, newTestStepClock(base), "edge-01", "edge-02")
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	refusals := []struct {
		line string
		want string
	}{
		{fleetAlarmPath + " | severity bogus", "unknown alarm severity bogus"},
		{fleetAlarmPath + " | severity critical | severity warning", "conflicting alarm severity filters"},
		{fleetAlarmPath + " | kind not-a-kind", "unknown alarm kind not-a-kind"},
		{fleetAlarmPath + " | kind device-offline | kind component-down", "conflicting alarm kind filters"},
		{fleetAlarmPath + " | device edge-01 | device edge-02", "conflicting alarm device filters"},
		{fleetAlarmPath + " | limit 0", "positive number"},
		{fleetAlarmPath + " | limit 1025", "invalid alarm page limit 1025"},
		{fleetAlarmPath + " | limit 10 | limit 20", "cannot specify limit more than once"},
		{fleetAlarmPath + " | cursor aaa | cursor bbb", "cannot specify cursor more than once"},
		{fleetAlarmPath + " | cursor not-valid-base64!!!", "invalid alarm continuation cursor"},
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

	// Direct handler checks for invalid arguments that generic pipe partitioning
	// might reject earlier.
	directCases := []struct {
		name string
		ops  []command.PipeOperation
		want string
	}{
		{
			name: "unknown pipe op",
			ops:  []command.PipeOperation{{Name: "bogus", Args: []string{"x"}}},
			want: "unknown fleet alarm pipe operation bogus",
		},
		{
			name: "negative limit",
			ops:  []command.PipeOperation{{Name: pipeOpLimit, Args: []string{"-1"}}},
			want: "invalid alarm page limit -1",
		},
		{
			name: "non-numeric limit",
			ops:  []command.PipeOperation{{Name: pipeOpLimit, Args: []string{"abc"}}},
			want: "invalid alarm page limit abc",
		},
		{
			name: "empty cursor",
			ops:  []command.PipeOperation{{Name: pipeOpCursor, Args: []string{"   "}}},
			want: "pipe filter cursor requires a non-empty argument",
		},
		{
			name: "missing cursor arg",
			ops:  []command.PipeOperation{{Name: pipeOpCursor}},
			want: "pipe filter cursor requires one argument",
		},
	}
	for _, tc := range directCases {
		var args command.ValidatedArgs
		args = args.WithPipeOperations(tc.ops)
		resp, err := handleFleetAlarm(ctx, args)
		require.NoError(t, err, tc.name)
		require.NotNil(t, resp, tc.name)
		require.Equal(t, plugin.StatusError, resp.Status, tc.name)
		assert.Contains(t, resp.Error, tc.want, tc.name)
	}
}

// TestShowFleetAlarmOpenFiltersPaginationAndGenericPipes proves that
// show fleet alarm returns live occurrences with totals/counts, filters by
// severity/kind/device, paginates stably across cursors, and composes cleanly
// with every generic pipe operator over its ShapeTab envelope.
//
// VALIDATES: fleet-ui-5 AC-4, AC-8, AC-9, AC-13.
// PREVENTS: broken filter wiring, broken cursor continuation, nil row slices,
// or ShapeTab envelope rejections in command.ApplyPipes.
func TestShowFleetAlarmOpenFiltersPaginationAndGenericPipes(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	clk := newTestStepClock(base)
	addr := startAlarmCommandsServer(t, clk, "edge-01", "edge-02")
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	// Empty state first: ensure alarms slice is non-nil [] and generic pipes work on empty envelope.
	emptyPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	assert.Equal(t, 0, emptyPage.Total)
	assert.NotNil(t, emptyPage.Alarms)
	assert.Empty(t, emptyPage.Alarms)
	emptyCount := decodePipeData[map[string]any](t, server, ctx, fleetAlarmPath+" | count")
	assert.Equal(t, float64(0), emptyCount["count"])

	mcEdge1 := connectReportCommandClient(t, addr, "edge-01")
	mcEdge2 := connectReportCommandClient(t, addr, "edge-02")

	sendTestHealth(t, mcEdge1, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: base.Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "session down", CheckedAt: base.Format(time.RFC3339)},
			{Name: "ntp", Status: fleet.HealthStatusDegraded, Reason: "drift high", CheckedAt: base.Format(time.RFC3339)},
		},
	})
	clk.Advance(time.Second)
	sendTestHealth(t, mcEdge2, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: clk.Now().Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "isis", Status: fleet.HealthStatusDown, Reason: "adjacency lost", CheckedAt: clk.Now().Format(time.RFC3339)},
		},
	})

	// Unfiltered listing: 3 open alarms (2 critical, 1 warning).
	fullPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath)
	require.Equal(t, 3, fullPage.Total)
	require.Len(t, fullPage.Alarms, 3)
	assert.Equal(t, 3, fullPage.Counts.Total)
	assert.Equal(t, 2, fullPage.Counts.Critical)
	assert.Equal(t, 1, fullPage.Counts.Warning)
	assert.Equal(t, 3, fullPage.Counts.Visible)
	assert.False(t, fullPage.HasMore)
	assert.NotEmpty(t, fullPage.SnapshotID)

	// Command-owned filters: severity, kind, device.
	critPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | severity critical")
	require.Equal(t, 2, critPage.Total)
	require.Len(t, critPage.Alarms, 2)
	for _, a := range critPage.Alarms {
		assert.Equal(t, pluginserver.AlarmSeverityCritical, a.Severity)
	}

	degradedPage := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | kind component-degraded")
	require.Equal(t, 1, degradedPage.Total)
	require.Len(t, degradedPage.Alarms, 1)
	assert.Equal(t, "ntp", degradedPage.Alarms[0].Subject)

	edge1Page := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | device edge-01")
	require.Equal(t, 2, edge1Page.Total)
	require.Len(t, edge1Page.Alarms, 2)
	for _, a := range edge1Page.Alarms {
		assert.Equal(t, "edge-01", a.Device)
	}

	// Cursor pagination across pages of the same snapshot.
	page1 := decodePipeData[pluginserver.AlarmListPage](t, server, ctx, fleetAlarmPath+" | limit 2")
	require.Equal(t, 3, page1.Total)
	require.Len(t, page1.Alarms, 2)
	require.True(t, page1.HasMore)
	require.NotEmpty(t, page1.Cursor)

	page2 := decodePipeData[pluginserver.AlarmListPage](
		t, server, ctx, fleetAlarmPath+" | limit 2 | cursor "+page1.Cursor,
	)
	require.Equal(t, 3, page2.Total)
	require.Len(t, page2.Alarms, 1)
	assert.False(t, page2.HasMore)
	assert.Equal(t, page1.SnapshotID, page2.SnapshotID)
	assert.Equal(t, page1.Counts, page2.Counts)

	seenIDs := map[string]bool{
		page1.Alarms[0].ID: true,
		page1.Alarms[1].ID: true,
		page2.Alarms[0].ID: true,
	}
	assert.Len(t, seenIDs, 3)

	// Generic pipe composition directly through command.ApplyPipes and
	// command.ProcessPipesDefaultFormatChecked on the raw JSON answer.
	rawResp, err := dispatchWithContext(t, server, ctx, fleetAlarmPath)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, rawResp.Status)
	rawJSON, err := plugin.ResponseJSON(rawResp, nil)
	require.NoError(t, err)
	var rawPage pluginserver.AlarmListPage
	require.NoError(t, json.Unmarshal([]byte(rawJSON), &rawPage))
	cols := command.ColumnsForCommand(fleetAlarmPath)

	// | count
	_, countOps := command.ParsePipe(fleetAlarmPath + " | count")
	countOut, pipeErr := command.ApplyPipes(rawJSON, countOps, nil, cols)
	require.Empty(t, pipeErr)
	var counted map[string]any
	require.NoError(t, json.Unmarshal([]byte(countOut), &counted))
	assert.Equal(t, float64(3), counted["count"])

	// | match bgp (narrows alarms while preserving envelope metadata)
	_, matchOps := command.ParsePipe(fleetAlarmPath + " | match bgp")
	matchOut, pipeErr := command.ApplyPipes(rawJSON, matchOps, nil, cols)
	require.Empty(t, pipeErr)
	var matched pluginserver.AlarmListPage
	require.NoError(t, json.Unmarshal([]byte(matchOut), &matched))
	assert.Equal(t, 3, matched.Total)
	assert.Equal(t, rawPage.SnapshotID, matched.SnapshotID)
	require.Len(t, matched.Alarms, 1)
	assert.Equal(t, "bgp", matched.Alarms[0].Subject)

	// | first 1 and | last 1
	_, firstOps := command.ParsePipe(fleetAlarmPath + " | first 1")
	firstOut, pipeErr := command.ApplyPipes(rawJSON, firstOps, nil, cols)
	require.Empty(t, pipeErr)
	var firstPage pluginserver.AlarmListPage
	require.NoError(t, json.Unmarshal([]byte(firstOut), &firstPage))
	assert.Equal(t, 3, firstPage.Total)
	require.Len(t, firstPage.Alarms, 1)
	assert.Equal(t, fullPage.Alarms[0].ID, firstPage.Alarms[0].ID)

	_, lastOps := command.ParsePipe(fleetAlarmPath + " | last 1")
	lastOut, pipeErr := command.ApplyPipes(rawJSON, lastOps, nil, cols)
	require.Empty(t, pipeErr)
	var lastPage pluginserver.AlarmListPage
	require.NoError(t, json.Unmarshal([]byte(lastOut), &lastPage))
	require.Len(t, lastPage.Alarms, 1)
	assert.Equal(t, fullPage.Alarms[2].ID, lastPage.Alarms[0].ID)

	// | display and | fill on ShapeTab
	_, displayFormat, refusal := command.ProcessPipesDefaultFormatChecked(fleetAlarmPath+" | display id kind severity | json compact", "")
	require.Empty(t, refusal)
	displayOut := displayFormat(rawJSON)
	assert.NotContains(t, displayOut, "\"error\"")
	assert.Contains(t, displayOut, "\"kind\":\"component-down\"")

	_, fillFormat, refusal := command.ProcessPipesDefaultFormatChecked(fleetAlarmPath+" | display id kind | fill | table", "")
	require.Empty(t, refusal)
	fillOut := fillFormat(rawJSON)
	assert.Contains(t, fillOut, "component-down")

	// | json compact, | yaml, | table, | text, | ndjson, | raw
	for _, tc := range []struct {
		pipe     string
		contains string
	}{
		{"json compact", `"snapshot-id":"`},
		{"yaml", "alarms:"},
		{"table", "component-down"},
		{"text", "component-down"},
		{"ndjson", `"snapshot-id":`},
		{"raw", `"alarms":`},
	} {
		_, ops := command.ParsePipe(fleetAlarmPath + " | " + tc.pipe)
		out, errMsg := command.ApplyPipes(rawJSON, ops, nil, cols)
		require.Empty(t, errMsg, tc.pipe)
		assert.Contains(t, out, tc.contains, tc.pipe)
		if tc.pipe == "json compact" {
			assert.NotContains(t, strings.TrimSpace(out), "\n", "json compact must be single-line")
		}
	}
}

// TestShowFleetAlarmHistoryValidationScopingAndGenericPipes proves that
// show fleet alarm history device validates limits and cursors, paginates
// open and closed events, enforces fail-closed RBAC device scoping without
// leaking device existence, and works with generic pipes.
//
// VALIDATES: fleet-ui-5 AC-4, AC-8, AC-13 and RBAC device visibility.
// PREVENTS: unauthorized callers distinguishing existing vs absent devices,
// nil contexts leaking alarms, or broken history pipe compatibility.
func TestShowFleetAlarmHistoryValidationScopingAndGenericPipes(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	clk := newTestStepClock(base)
	addr := startAlarmCommandsServer(t, clk, "edge-01", "edge-02", "core-01")

	mcEdge1 := connectReportCommandClient(t, addr, "edge-01")
	mcCore1 := connectReportCommandClient(t, addr, "core-01")

	// Open and clear component-down on edge-01, then open component-degraded.
	sendTestHealth(t, mcEdge1, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: base.Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusDown, Reason: "peer down", CheckedAt: base.Format(time.RFC3339)},
		},
	})
	clk.Advance(61 * time.Second)
	sendTestHealth(t, mcEdge1, fleet.HealthReport{
		Status:    fleet.HealthStatusHealthy,
		CheckedAt: clk.Now().Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "bgp", Status: fleet.HealthStatusHealthy, CheckedAt: clk.Now().Format(time.RFC3339)},
		},
	})
	clk.Advance(61 * time.Second)
	sendTestHealth(t, mcEdge1, fleet.HealthReport{
		Status:    fleet.HealthStatusDegraded,
		CheckedAt: clk.Now().Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "ntp", Status: fleet.HealthStatusDegraded, Reason: "unsynced", CheckedAt: clk.Now().Format(time.RFC3339)},
		},
	})

	// Open component-down on core-01 (unauthorized for west-op).
	sendTestHealth(t, mcCore1, fleet.HealthReport{
		Status:    fleet.HealthStatusDown,
		CheckedAt: clk.Now().Format(time.RFC3339),
		Components: []fleet.HealthComponent{
			{Name: "ospf", Status: fleet.HealthStatusDown, Reason: "down", CheckedAt: clk.Now().Format(time.RFC3339)},
		},
	})

	server := registryDispatcher(t)
	adminCtx := &pluginserver.CommandContext{Server: server}

	// 1. Unrestricted history read of edge-01 returns both open and closed events.
	hist := decodePipeData[pluginserver.AlarmHistoryPage](t, server, adminCtx, fleetAlarmHistoryPath+" edge-01")
	assert.Equal(t, "edge-01", hist.Device)
	require.Equal(t, 2, hist.Total)
	require.Len(t, hist.Events, 2)
	assert.False(t, hist.HasMore)
	assert.False(t, hist.RetainedSince.IsZero())
	assert.NotEmpty(t, hist.SnapshotID)
	assert.False(t, hist.Events[0].Cleared, "newest event (ntp degraded) is open")
	assert.Equal(t, "ntp", hist.Events[0].Subject)
	assert.True(t, hist.Events[1].Cleared, "older event (bgp down) is cleared")
	assert.Equal(t, "bgp", hist.Events[1].Subject)
	assert.False(t, hist.Events[1].ClearedAt.IsZero())

	// Known device with zero alarm events returns StatusDone with non-nil [].
	edge2Hist := decodePipeData[pluginserver.AlarmHistoryPage](t, server, adminCtx, fleetAlarmHistoryPath+" edge-02")
	assert.Equal(t, "edge-02", edge2Hist.Device)
	assert.Equal(t, 0, edge2Hist.Total)
	assert.NotNil(t, edge2Hist.Events)
	assert.Empty(t, edge2Hist.Events)

	// 2. Cursor pagination on history.
	hPage1 := decodePipeData[pluginserver.AlarmHistoryPage](t, server, adminCtx, fleetAlarmHistoryPath+" edge-01 | limit 1")
	require.Equal(t, 2, hPage1.Total)
	require.Len(t, hPage1.Events, 1)
	require.True(t, hPage1.HasMore)
	require.NotEmpty(t, hPage1.Cursor)

	hPage2 := decodePipeData[pluginserver.AlarmHistoryPage](
		t, server, adminCtx, fleetAlarmHistoryPath+" edge-01 | limit 1 | cursor "+hPage1.Cursor,
	)
	require.Equal(t, 2, hPage2.Total)
	require.Len(t, hPage2.Events, 1)
	assert.False(t, hPage2.HasMore)
	assert.Equal(t, hPage1.SnapshotID, hPage2.SnapshotID)
	assert.NotEqual(t, hPage1.Events[0].ID, hPage2.Events[0].ID)

	// 3. History validation refusals.
	for _, tc := range []struct {
		line string
		want string
	}{
		{fleetAlarmHistoryPath + " nonexistent", "unknown device"},
		{fleetAlarmHistoryPath + " edge-01 | limit 0", "positive number"},
		{fleetAlarmHistoryPath + " edge-01 | limit 513", "invalid alarm page limit 513"},
		{fleetAlarmHistoryPath + " edge-01 | limit 10 | limit 20", "cannot specify limit more than once"},
		{fleetAlarmHistoryPath + " edge-01 | cursor aaa | cursor bbb", "cannot specify cursor more than once"},
		{fleetAlarmHistoryPath + " edge-01 | cursor bad!!!", "invalid alarm continuation cursor"},
	} {
		resp, err := dispatchWithContext(t, server, adminCtx, tc.line)
		if err != nil {
			assert.Contains(t, err.Error(), tc.want, tc.line)
			continue
		}
		require.NotNil(t, resp, tc.line)
		require.Equal(t, plugin.StatusError, resp.Status, tc.line)
		assert.Contains(t, resp.Error, tc.want, tc.line)
	}

	// Direct handler checks for missing device name and unknown pipe op.
	missingNameResp, err := handleFleetAlarmHistory(adminCtx, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, missingNameResp.Status)
	assert.Contains(t, missingNameResp.Error, "a device name is required")

	validatedEdge1, vErr := command.ValidateModelArgs(fleetAlarmHistoryPath, []string{"edge-01"}, nil)
	require.NoError(t, vErr)
	badOpArgs := validatedEdge1.WithPipeOperations([]command.PipeOperation{{Name: "severity", Args: []string{"critical"}}})
	badOpResp, err := handleFleetAlarmHistory(adminCtx, badOpArgs)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, badOpResp.Status)
	assert.Contains(t, badOpResp.Error, "unknown fleet alarm history pipe operation severity")

	// 4. Generic pipe compatibility (json, count, table, match, first, yaml, raw).
	rawResp, err := dispatchWithContext(t, server, adminCtx, fleetAlarmHistoryPath+" edge-01")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, rawResp.Status)
	rawJSON, err := plugin.ResponseJSON(rawResp, nil)
	require.NoError(t, err)
	histCols := command.ColumnsForCommand(fleetAlarmHistoryPath)

	_, hCountOps := command.ParsePipe(fleetAlarmHistoryPath + " edge-01 | count")
	hCountOut, pipeErr := command.ApplyPipes(rawJSON, hCountOps, nil, histCols)
	require.Empty(t, pipeErr)
	var hCounted map[string]any
	require.NoError(t, json.Unmarshal([]byte(hCountOut), &hCounted))
	assert.Equal(t, float64(2), hCounted["count"])

	_, hMatchOps := command.ParsePipe(fleetAlarmHistoryPath + " edge-01 | match bgp")
	hMatchOut, pipeErr := command.ApplyPipes(rawJSON, hMatchOps, nil, histCols)
	require.Empty(t, pipeErr)
	var hMatched pluginserver.AlarmHistoryPage
	require.NoError(t, json.Unmarshal([]byte(hMatchOut), &hMatched))
	assert.Equal(t, "edge-01", hMatched.Device)
	require.Len(t, hMatched.Events, 1)
	assert.Equal(t, "bgp", hMatched.Events[0].Subject)

	for _, tc := range []struct {
		pipe     string
		contains string
	}{
		{"json compact", `"device":"edge-01"`},
		{"yaml", "events:"},
		{"table", "component-down"},
		{"text", "component-down"},
		{"raw", `"events":`},
	} {
		_, ops := command.ParsePipe(fleetAlarmHistoryPath + " edge-01 | " + tc.pipe)
		out, errMsg := command.ApplyPipes(rawJSON, ops, nil, histCols)
		require.Empty(t, errMsg, tc.pipe)
		assert.Contains(t, out, tc.contains, tc.pipe)
	}

	// 5. RBAC device scoping: west-op sees edge-* only, never core-01.
	server.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: buildAlarmScopeTestStore()})
	westCtx := &pluginserver.CommandContext{Server: server, Username: "west-op"}

	westOpen := decodePipeData[pluginserver.AlarmListPage](t, server, westCtx, fleetAlarmPath)
	assert.Equal(t, 1, westOpen.Total, "west-op must see only edge-01 ntp alarm, not core-01")
	require.Len(t, westOpen.Alarms, 1)
	assert.Equal(t, "edge-01", westOpen.Alarms[0].Device)
	assert.Equal(t, 0, westOpen.Counts.Critical)
	assert.Equal(t, 1, westOpen.Counts.Warning)

	deniedOpen := decodePipeData[pluginserver.AlarmListPage](t, server, westCtx, fleetAlarmPath+" | device core-01")
	absentOpen := decodePipeData[pluginserver.AlarmListPage](t, server, westCtx, fleetAlarmPath+" | device nonexistent")
	assert.Equal(t, 0, deniedOpen.Total)
	assert.Empty(t, deniedOpen.Alarms)
	assert.Equal(t, absentOpen.Total, deniedOpen.Total)
	assert.Equal(t, absentOpen.Counts, deniedOpen.Counts)

	// Direct handler call verifies unauthorized device returns the exact same
	// unknown-device refusal shape as a non-existent device.
	validatedCore1, vErr := command.ValidateModelArgs(fleetAlarmHistoryPath, []string{"core-01"}, nil)
	require.NoError(t, vErr)
	validatedAbsent, vErr := command.ValidateModelArgs(fleetAlarmHistoryPath, []string{"nonexistent"}, nil)
	require.NoError(t, vErr)

	deniedHistResp, err := handleFleetAlarmHistory(westCtx, validatedCore1)
	require.NoError(t, err)
	absentHistResp, err := handleFleetAlarmHistory(westCtx, validatedAbsent)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, deniedHistResp.Status)
	require.Equal(t, plugin.StatusError, absentHistResp.Status)
	assert.Contains(t, deniedHistResp.Error, "unknown device")
	assert.Contains(t, absentHistResp.Error, "unknown device")
	assert.Equal(
		t,
		strings.ReplaceAll(deniedHistResp.Error, "core-01", "<dev>"),
		strings.ReplaceAll(absentHistResp.Error, "nonexistent", "<dev>"),
		"unauthorized device refusal must match unknown device refusal identically",
	)

	// Authorized device history succeeds for west-op.
	westEdge1Hist := decodePipeData[pluginserver.AlarmHistoryPage](t, server, westCtx, fleetAlarmHistoryPath+" edge-01")
	assert.Equal(t, 2, westEdge1Hist.Total)

	// 6. Nil CommandContext fails closed for both alarm handlers.
	nilOpenResp, err := handleFleetAlarm(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilOpenResp.Status)
	nilOpenRaw, err := plugin.ResponseJSON(nilOpenResp, nil)
	require.NoError(t, err)
	var nilOpen pluginserver.AlarmListPage
	require.NoError(t, json.Unmarshal([]byte(nilOpenRaw), &nilOpen))
	assert.Equal(t, 0, nilOpen.Total)
	assert.Empty(t, nilOpen.Alarms)

	nilHistResp, err := handleFleetAlarmHistory(nil, validatedEdge1)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusError, nilHistResp.Status)
	assert.Contains(t, nilHistResp.Error, "unknown device")
}
