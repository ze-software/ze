// VALIDATES: spec-fleet-3 AC-7, AC-8, AC-9, AC-10, AC-16, and operator error
// handling for show fleet audit.
// Exercises YANG/command registration (ShapeTab, columns, command-owned pipe
// filters, Tab completion), pipe validation refusals, newest-first ordering,
// device/type/inclusive since+until filtering, generic pipe composition, and
// fail-closed per-device RBAC authorization through a real ManagedServer and
// pluginserver.Dispatcher.
// PREVENTS: missing command or pipe registration, invalid filter inputs
// returning empty slices instead of operator refusals, inclusive time boundary
// off-by-one bugs, nil JSON row slices, or unauthorized callers probing denied
// devices.

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

const (
	testAuditDeviceEdge = "edge-01"
	testAuditDeviceCore = "core-01"
)

func startAuditCommandsServer(
	t *testing.T,
	clk clock.Clock,
	clients ...string,
) (*pluginserver.ManagedServer, string) {
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
		ReadConfig: func(name string) ([]byte, error) { return []byte("router-id " + name + "\n"), nil },
	}
	srv, err := pluginserver.NewManagedServer(cfg)
	require.NoError(t, err)
	require.NoError(t, srv.Start(t.Context()))
	t.Cleanup(srv.Stop)
	return srv, srv.Addrs()[0].String()
}

func buildAuditScopeTestStore() *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "edge-audit-only",
		Run: authz.Section{
			Default: authz.Deny,
			Entries: []authz.Entry{
				{Number: 10, Action: authz.Allow, Match: "^show fleet audit($| \\|.*)$", Regex: true},
				{Number: 20, Action: authz.Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("edge-op", []string{"edge-audit-only"})
	return store
}

// TestFleetAuditCommandRegistrationAndCompletion verifies that show fleet audit
// registers ShapeTab, its ordered column set, its four command-owned pipe
// filters, and Tab completion suggestions.
//
// VALIDATES: spec-fleet-3 AC-7, AC-8, AC-9, AC-10 command surface registration.
// PREVENTS: missing shape, column, or pipe filter registration.
func TestFleetAuditCommandRegistrationAndCompletion(t *testing.T) {
	shape, ok := command.ShapeForCommand(fleetAuditPath)
	require.True(t, ok)
	assert.Equal(t, command.ShapeTab, shape)

	cols := command.ColumnsForCommand(fleetAuditPath)
	require.Len(t, cols, 1)
	assert.Equal(t, command.ColumnOrder{
		"seq", "event-time", "receipt-time", "type", "device", "actor", "surface",
		"outcome", "version", "previous-version", "diagnostic-code", "summary", "metadata",
	}, cols[0])

	filters := command.PipeFiltersForCommand(fleetAuditPath)
	require.Len(t, filters, 4)
	filterArgs := make(map[string]command.PipeArgKind, len(filters))
	for _, f := range filters {
		filterArgs[f.Name] = f.Argument
		assert.NotEmpty(t, f.Description, f.Name)
	}
	assert.Equal(t, map[string]command.PipeArgKind{
		pipeOpDevice: command.ArgText,
		pipeOpType:   command.ArgText,
		pipeOpSince:  command.ArgText,
		pipeOpUntil:  command.ArgText,
	}, filterArgs)

	schema, err := configyang.DefaultLoader()
	require.NoError(t, err)
	completer := command.NewTreeCompleter(configyang.BuildCommandTree(schema))
	for _, tc := range []struct {
		line string
		want []string
	}{
		{fleetAuditPath + " | dev", []string{"device"}},
		{fleetAuditPath + " | typ", []string{"type"}},
		{fleetAuditPath + " | sin", []string{"since"}},
		{fleetAuditPath + " | unt", []string{"until"}},
	} {
		var got []string
		for _, s := range completer.Complete(tc.line) {
			got = append(got, s.Text)
		}
		assert.Equal(t, tc.want, got, tc.line)
	}
}

// TestShowFleetAuditNotServingRefusal verifies that show fleet audit returns a
// structured operator error naming ze show doctor when no managed server runs.
//
// VALIDATES: operator refusal contract when the hub is not serving.
// PREVENTS: nil-pointer panics or silent empty responses when not serving.
func TestShowFleetAuditNotServingRefusal(t *testing.T) {
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	resp, err := dispatchWithContext(t, server, ctx, fleetAuditPath)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "unable to read fleet audit log")
	assert.Contains(t, resp.Error, exampleShowDoctor)
}

// TestShowFleetAuditPipeValidationRefusals verifies that malformed timestamps,
// conflicting device/type filters, duplicate time bounds, reversed intervals,
// invalid event types, and unknown pipe operations are refused with operator
// errors.
//
// VALIDATES: spec-fleet-3 query validation contract.
// PREVENTS: invalid filters silently returning empty slices.
func TestShowFleetAuditPipeValidationRefusals(t *testing.T) {
	base := time.Date(2026, 11, 28, 12, 0, 0, 0, time.UTC)
	startAuditCommandsServer(t, newTestStepClock(base), testAuditDeviceEdge, testAuditDeviceCore)
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	refusals := []struct {
		line string
		want string
	}{
		{fleetAuditPath + " | device edge-01 | device core-01", "conflicting fleet audit device filters"},
		{fleetAuditPath + " | type config.fetch | type config.ack", "conflicting fleet audit type filters"},
		{fleetAuditPath + " | type notdotted", "invalid fleet audit event type notdotted"},
		{fleetAuditPath + " | type Bad.Case", "invalid fleet audit event type Bad.Case"},
		{fleetAuditPath + " | since not-a-timestamp", "invalid since timestamp not-a-timestamp"},
		{fleetAuditPath + " | until 2026-99-99", "invalid until timestamp 2026-99-99"},
		{fleetAuditPath + " | since 2026-11-28T00:00:00Z | since 2026-11-28T01:00:00Z", "cannot specify since more than once"},
		{fleetAuditPath + " | until 2026-11-28T02:00:00Z | until 2026-11-28T03:00:00Z", "cannot specify until more than once"},
		{fleetAuditPath + " | since 2026-11-28T05:00:00Z | until 2026-11-28T04:00:00Z", "reversed fleet audit time range"},
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

	directCases := []struct {
		name string
		ops  []command.PipeOperation
		want string
	}{
		{
			name: "unknown pipe op",
			ops:  []command.PipeOperation{{Name: "bogus", Args: []string{"x"}}},
			want: "unknown fleet audit pipe operation bogus",
		},
		{
			name: "missing device arg",
			ops:  []command.PipeOperation{{Name: pipeOpDevice}},
			want: "pipe filter device requires one argument",
		},
		{
			name: "empty type arg",
			ops:  []command.PipeOperation{{Name: pipeOpType, Args: []string{"   "}}},
			want: "pipe filter type requires a non-empty argument",
		},
		{
			name: "empty since arg",
			ops:  []command.PipeOperation{{Name: pipeOpSince, Args: []string{""}}},
			want: "pipe filter since requires a non-empty argument",
		},
		{
			name: "empty until arg",
			ops:  []command.PipeOperation{{Name: pipeOpUntil, Args: []string{" "}}},
			want: "pipe filter until requires a non-empty argument",
		},
	}
	for _, tc := range directCases {
		var args command.ValidatedArgs
		args = args.WithPipeOperations(tc.ops)
		resp, err := handleFleetAudit(ctx, args)
		require.NoError(t, err, tc.name)
		require.NotNil(t, resp, tc.name)
		require.Equal(t, plugin.StatusError, resp.Status, tc.name)
		assert.Contains(t, resp.Error, tc.want, tc.name)
	}
}

// TestShowFleetAuditQueryFiltersAndGenericPipes verifies newest-first ordering,
// empty JSON slice encoding, device/type/inclusive since+until filters (including
// non-UTC RFC 3339 offsets), and generic pipe composition against a live
// ManagedServer.
//
// VALIDATES: spec-fleet-3 AC-7, AC-8, AC-9, AC-10.
// PREVENTS: misordered sequences, broken filter predicates, off-by-one time
// bounds, nil row slices, or generic pipe incompatibilities.
func TestShowFleetAuditQueryFiltersAndGenericPipes(t *testing.T) {
	base := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	clk := newTestStepClock(base)
	srv, addr := startAuditCommandsServer(t, clk, testAuditDeviceEdge, testAuditDeviceCore)
	server := registryDispatcher(t)
	ctx := &pluginserver.CommandContext{Server: server}

	// Empty audit log answers a non-nil [] slice and count 0.
	emptyResp, err := dispatchWithContext(t, server, ctx, fleetAuditPath)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, emptyResp.Status)
	emptyRaw, err := plugin.ResponseJSON(emptyResp, nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", strings.TrimSpace(emptyRaw))

	// Connect edge-01 at t0, fetch + ack at t0+10s, connect core-01 at t0+20s.
	mcEdge := connectReportCommandClient(t, addr, testAuditDeviceEdge)
	tFetch := base.Add(10 * time.Second)
	clk.Advance(10 * time.Second)
	fetched, err := FetchConfig(t.Context(), mcEdge, "")
	require.NoError(t, err)
	require.NotEmpty(t, fetched.Version)
	require.NoError(t, SendConfigAck(t.Context(), mcEdge, fleet.ConfigAck{Version: fetched.Version, OK: true}))

	tCore := base.Add(20 * time.Second)
	clk.Advance(10 * time.Second)
	_ = connectReportCommandClient(t, addr, testAuditDeviceCore)

	require.NoError(t, srv.Audit().Flush(t.Context()))

	// AC-7: newest first (strictly descending Seq).
	allRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath)
	require.Len(t, allRows, 4)
	for i := 1; i < len(allRows); i++ {
		assert.Greater(t, allRows[i-1].Seq, allRows[i].Seq, "audit rows must be newest first")
	}
	assert.Equal(t, pluginserver.FleetAuditDeviceConnect, allRows[0].Type)
	assert.Equal(t, testAuditDeviceCore, allRows[0].Device)
	assert.Equal(t, pluginserver.FleetAuditConfigAck, allRows[1].Type)
	assert.Equal(t, fetched.Version, allRows[1].Version)
	assert.Equal(t, pluginserver.FleetAuditConfigFetch, allRows[2].Type)
	assert.Equal(t, pluginserver.FleetAuditDeviceConnect, allRows[3].Type)
	assert.Equal(t, testAuditDeviceEdge, allRows[3].Device)

	// AC-8: | device edge-01 (and repeated identical filter is idempotent).
	edgeRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | device "+testAuditDeviceEdge)
	require.Len(t, edgeRows, 3)
	for _, row := range edgeRows {
		assert.Equal(t, testAuditDeviceEdge, row.Device)
	}
	edgeRepeat := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | device "+testAuditDeviceEdge+" | device "+testAuditDeviceEdge)
	assert.Equal(t, edgeRows, edgeRepeat)

	// AC-9: | type config.fetch.
	fetchRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | type "+pluginserver.FleetAuditConfigFetch)
	require.Len(t, fetchRows, 1)
	assert.Equal(t, pluginserver.FleetAuditConfigFetch, fetchRows[0].Type)
	assert.Equal(t, testAuditDeviceEdge, fetchRows[0].Device)
	assert.Equal(t, fetched.Version, fetchRows[0].Version)

	// AC-10: inclusive | since and | until boundaries (including timezone offset).
	sinceRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | since "+tFetch.Format(time.RFC3339))
	require.Len(t, sinceRows, 3, "inclusive since must include entries at exact tFetch and later")
	assert.Equal(t, testAuditDeviceCore, sinceRows[0].Device)
	assert.Equal(t, pluginserver.FleetAuditConfigAck, sinceRows[1].Type)
	assert.Equal(t, pluginserver.FleetAuditConfigFetch, sinceRows[2].Type)

	untilRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | until "+base.Format(time.RFC3339))
	require.Len(t, untilRows, 1, "inclusive until must include entry at exact base")
	assert.Equal(t, pluginserver.FleetAuditDeviceConnect, untilRows[0].Type)
	assert.Equal(t, testAuditDeviceEdge, untilRows[0].Device)

	// Equal since and until with +02:00 offset selects the exact tCore instant.
	tCoreOffset := tCore.In(time.FixedZone("UTC+2", 2*3600)).Format(time.RFC3339)
	exactRows := decodePipeData[[]fleetAuditRow](t, server, ctx, fleetAuditPath+" | since "+tCoreOffset+" | until "+tCoreOffset)
	require.Len(t, exactRows, 1)
	assert.Equal(t, testAuditDeviceCore, exactRows[0].Device)

	// Generic pipe compatibility on ShapeTab rows.
	rawResp, err := dispatchWithContext(t, server, ctx, fleetAuditPath)
	require.NoError(t, err)
	rawJSON, err := plugin.ResponseJSON(rawResp, nil)
	require.NoError(t, err)
	cols := command.ColumnsForCommand(fleetAuditPath)

	_, countOps := command.ParsePipe(fleetAuditPath + " | count")
	countOut, pipeErr := command.ApplyPipes(rawJSON, countOps, nil, cols)
	require.Empty(t, pipeErr)
	var counted map[string]any
	require.NoError(t, json.Unmarshal([]byte(countOut), &counted))
	assert.Equal(t, float64(4), counted["count"])

	for _, tc := range []struct {
		pipe     string
		contains string
	}{
		{"json compact", `"type":"config.fetch"`},
		{"yaml", "event-time:"},
		{"table", "config.fetch"},
		{"first 1 | json compact", `"device":"core-01"`},
	} {
		_, ops := command.ParsePipe(fleetAuditPath + " | " + tc.pipe)
		out, errMsg := command.ApplyPipes(rawJSON, ops, nil, cols)
		require.Empty(t, errMsg, tc.pipe)
		assert.Contains(t, out, tc.contains, tc.pipe)
	}
}

// TestShowFleetAuditPerDeviceAuthorization proves that restricted callers see
// only rows for devices authorized under show fleet device name <device>, cannot
// distinguish denied devices from absent devices via | device, and nil contexts
// fail closed.
//
// VALIDATES: spec-fleet-3 AC-7, AC-16 per-device authorization.
// PREVENTS: cross-device audit leakage or device existence probing.
func TestShowFleetAuditPerDeviceAuthorization(t *testing.T) {
	base := time.Date(2026, 11, 28, 8, 0, 0, 0, time.UTC)
	clk := newTestStepClock(base)
	srv, addr := startAuditCommandsServer(t, clk, testAuditDeviceEdge, testAuditDeviceCore)

	_ = connectReportCommandClient(t, addr, testAuditDeviceEdge)
	_ = connectReportCommandClient(t, addr, testAuditDeviceCore)
	// Also append a hub-wide entry (empty device) to ensure scoped users cannot read it.
	require.True(t, srv.RecordAudit(pluginserver.FleetAuditEntry{
		EventTime: base.Add(time.Second),
		Type:      "template.change",
		Actor:     "admin",
		Surface:   "cli",
		Outcome:   "success",
	}))
	require.NoError(t, srv.Audit().Flush(t.Context()))

	server := registryDispatcher(t)
	server.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: buildAuditScopeTestStore()})
	edgeCtx := &pluginserver.CommandContext{Server: server, Username: "edge-op"}

	// Restricted caller sees only edge-01 rows (no core-01, no hub-wide rows).
	scopedRows := decodePipeData[[]fleetAuditRow](t, server, edgeCtx, fleetAuditPath)
	require.Len(t, scopedRows, 1)
	assert.Equal(t, testAuditDeviceEdge, scopedRows[0].Device)
	assert.Equal(t, pluginserver.FleetAuditDeviceConnect, scopedRows[0].Type)

	// Probing a denied device returns the exact same empty [] as an absent device.
	deniedRows := decodePipeData[[]fleetAuditRow](t, server, edgeCtx, fleetAuditPath+" | device "+testAuditDeviceCore)
	absentRows := decodePipeData[[]fleetAuditRow](t, server, edgeCtx, fleetAuditPath+" | device nonexistent")
	assert.NotNil(t, deniedRows)
	assert.Empty(t, deniedRows)
	assert.Equal(t, absentRows, deniedRows)

	// Filtering by type still respects per-device scope.
	connectRows := decodePipeData[[]fleetAuditRow](t, server, edgeCtx, fleetAuditPath+" | type "+pluginserver.FleetAuditDeviceConnect)
	require.Len(t, connectRows, 1)
	assert.Equal(t, testAuditDeviceEdge, connectRows[0].Device)

	// Nil CommandContext fails closed with an empty slice.
	nilResp, err := handleFleetAudit(nil, command.ValidatedArgs{})
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilResp.Status)
	nilRaw, err := plugin.ResponseJSON(nilResp, nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", strings.TrimSpace(nilRaw))
}
