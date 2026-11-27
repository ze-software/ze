// Design: docs/architecture/diagnostics/production-diagnostics.md -- correlation pipes, reload history, watchdog, and lock-free wedge survival tests
// Related: op_pipe.go -- | operation <id> pipe filter
// Related: reload_history.go -- show system reload history handler
// Related: watchdog.go -- show system watchdog handler
// Related: profile.go -- profileHandlerTimeout

package show

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeaudit "github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/opid"
	"github.com/ze-software/ze/internal/core/report"
)

// TestOperationPipeFilterRegistrations verifies every show command supporting
// | operation <id> declares it once with command.ArgText.
//
// VALIDATES: AC-2 -- show event recent, show errors, show audit, and show
// system reload history register the command-owned operation pipe filter.
// PREVENTS: a missing pipe declaration breaking CLI/MCP | operation filtering.
func TestOperationPipeFilterRegistrations(t *testing.T) {
	paths := []string{
		"show event recent",
		"show errors",
		"show audit",
		"show system reload history",
	}
	for _, path := range paths {
		filters := command.PipeFiltersForCommand(path)
		found := false
		for _, f := range filters {
			if f.Name == pipeOpOperation && f.Argument == command.ArgText {
				found = true
				break
			}
		}
		assert.True(t, found, "command.PipeFiltersForCommand(%q) = %+v, want operation (ArgText)", path, filters)
	}
}

// TestShowEventRecentOperationPipe verifies | operation <id> narrows event
// rows, includes operation-id when present, and refuses malformed pipe usage.
//
// VALIDATES: AC-2 -- show event recent | operation <id> returns only matching
// events and refuses duplicate/blank | operation with structured opererr.
// PREVENTS: cross-operation event noise or silent acceptance of invalid pipes.
func TestShowEventRecentOperationPipe(t *testing.T) {
	srv, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: srv}

	_, err = srv.EmitEngineEventContext(opid.With(context.Background(), "op-ev-1"), "config", "verify-ok", `{"tx-id":"1"}`)
	require.NoError(t, err)
	_, err = srv.EmitEngineEventContext(opid.With(context.Background(), "op-ev-2"), "config", "apply-ok", `{"tx-id":"2"}`)
	require.NoError(t, err)
	_, err = srv.EmitEngineEventContext(opid.With(context.Background(), "op-ev-1"), "config", "verify-failed", `{"tx-id":"1"}`)
	require.NoError(t, err)
	srv.EventRing().AppendWithMeta("bgp", "peer-up", "bgp", "192.0.2.1", "out")

	args := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpOperation, Args: []string{"op-ev-1"}},
	})
	resp, err := handleShowEventRecent(ctx, args)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	events, ok := data["events"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, events, 2)
	assert.Equal(t, 2, data[keyCount])
	assert.Equal(t, "verify-failed", events[0]["event-type"])
	assert.Equal(t, "op-ev-1", events[0][keyOperationID])
	assert.Equal(t, "verify-ok", events[1]["event-type"])
	assert.Equal(t, "op-ev-1", events[1][keyOperationID])

	// Unfiltered response keeps all events and omits operation-id when empty.
	allResp, err := handleShowEventRecent(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, allResp.Status)
	allData, ok := allResp.Data.(plugin.Map)
	require.True(t, ok)
	allEvents, ok := allData["events"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, allEvents, 4)
	_, hasOpOnUntagged := allEvents[0][keyOperationID]
	assert.False(t, hasOpOnUntagged, "untagged event must omit operation-id")

	assertOperationPipeRefusals(t, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowEventRecent(ctx, commandtest.Args().WithPipeOperations(ops))
	})
}

// TestShowErrorsOperationPipe verifies | operation <id> filters report-bus
// errors before count truncation and refuses malformed pipe usage.
//
// VALIDATES: AC-2 -- show errors | operation <id> narrows to matching issues
// and applies count after filtering.
// PREVENTS: count truncation hiding correlated errors behind newer unrelated
// faults.
func TestShowErrorsOperationPipe(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	ctxA := opid.With(context.Background(), "op-err-a")
	ctxB := opid.With(context.Background(), "op-err-b")
	report.RaiseErrorContext(ctxA, "config", "verify-failed", "peer-1", "first correlated error", "ze show errors")
	report.RaiseErrorContext(ctxA, "config", "apply-failed", "peer-2", "second correlated error", "ze show errors")
	report.RaiseErrorContext(ctxB, "bgp", "session-reset", "peer-3", "other operation error", "ze show errors")
	report.RaiseError("bgp", "untagged-err", "peer-4", "untagged error")

	args := commandtest.Args(argCount, "1").WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpOperation, Args: []string{"op-err-a"}},
	})
	resp, err := handleShowErrors(nil, args)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	issues, ok := data["errors"].([]report.Issue)
	require.True(t, ok)
	require.Len(t, issues, 1)
	assert.Equal(t, 1, data[keyCount])
	assert.Equal(t, "op-err-a", issues[0].OpID)
	assert.Equal(t, "peer-2", issues[0].Subject)

	assertOperationPipeRefusals(t, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowErrors(nil, commandtest.Args().WithPipeOperations(ops))
	})
}

// TestShowAuditOperationPipe verifies | operation <id> propagates OpID into
// audit.Filter and refuses duplicate/blank | operation pipes.
//
// VALIDATES: AC-2 -- show audit | operation <id> filters audit entries by
// operation-id.
// PREVENTS: show audit ignoring | operation <id> or dropping Filter.OpID.
func TestShowAuditOperationPipe(t *testing.T) {
	auditLog, err := zeaudit.NewMemory(zeaudit.MinMaxEntries)
	require.NoError(t, err)
	require.NoError(t, auditLog.Record(zeaudit.Entry{
		OpID:    "op-aud-1",
		Actor:   "admin",
		Surface: zeaudit.CLI,
		Action:  zeaudit.ActionConfigCommit,
		Outcome: zeaudit.OutcomeSuccess,
	}))
	require.NoError(t, auditLog.Record(zeaudit.Entry{
		OpID:    "op-aud-2",
		Actor:   "admin",
		Surface: zeaudit.SSH,
		Action:  zeaudit.ActionDaemonReload,
		Outcome: zeaudit.OutcomeError,
	}))

	var gotFilter zeaudit.Filter
	RegisterAuditProvider(func(f zeaudit.Filter) []zeaudit.Entry {
		gotFilter = f
		return auditLog.Query(f)
	})
	t.Cleanup(func() { RegisterAuditProvider(nil) })

	args := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpOperation, Args: []string{"op-aud-2"}},
	})
	resp, err := handleShowAudit(nil, args)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, "op-aud-2", gotFilter.OpID)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	entries, ok := data["entries"].([]zeaudit.Entry)
	require.True(t, ok)
	require.Len(t, entries, 1)
	assert.Equal(t, 1, data[keyCount])
	assert.Equal(t, "op-aud-2", entries[0].OpID)
	assert.Equal(t, zeaudit.ActionDaemonReload, entries[0].Action)

	assertOperationPipeRefusals(t, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowAudit(nil, commandtest.Args().WithPipeOperations(ops))
	})
}

// TestShowSystemReloadHistoryAndStatus verifies newest-first ordering, count
// bounds, | operation <id> filtering, nil-server degradation, and
// last-operation-id on show system reload status.
//
// VALIDATES: AC-3 -- show system reload history returns newest-first bounded
// reload records with count and | operation filtering, and show system reload
// status exposes last-operation-id when present.
// PREVENTS: out-of-range counts being accepted or reload history returning
// stale/reversed ordering.
func TestShowSystemReloadHistoryAndStatus(t *testing.T) {
	// Nil context returns empty history cleanly without panicking.
	nilResp, err := handleShowSystemReloadHistory(nil, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, nilResp.Status)
	nilData, ok := nilResp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 0, nilData[keyCount])

	srv := &pluginserver.Server{}
	ctx := &pluginserver.CommandContext{Server: srv}
	now := time.Now().UTC()

	srv.MarkReloadProcessedWithRecord(pluginserver.ReloadRecord{
		OpID:         "op-rel-1",
		Trigger:      "sighup",
		Outcome:      pluginserver.ReloadOutcomeApplied,
		StartedAt:    now.Add(-3 * time.Second),
		FinishedAt:   now.Add(-2 * time.Second),
		DurationMS:   12,
		ChangedRoots: []string{"bgp"},
		TxState:      "committed",
	})
	srv.MarkReloadProcessedWithRecord(pluginserver.ReloadRecord{
		OpID:         "op-rel-2",
		Trigger:      "commit",
		Outcome:      pluginserver.ReloadOutcomeFailed,
		StartedAt:    now.Add(-2 * time.Second),
		FinishedAt:   now.Add(-1 * time.Second),
		DurationMS:   25,
		ChangedRoots: []string{"interface"},
		TxState:      "aborted",
		Error:        "verify refused",
	})
	srv.MarkReloadProcessedWithRecord(pluginserver.ReloadRecord{
		OpID:         "op-rel-3",
		Trigger:      "request-reload",
		Outcome:      pluginserver.ReloadOutcomeApplied,
		StartedAt:    now.Add(-500 * time.Millisecond),
		FinishedAt:   now,
		DurationMS:   8,
		ChangedRoots: []string{"logging"},
		TxState:      "no-op",
	})

	// Newest-first full list.
	resp, err := handleShowSystemReloadHistory(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	entries, ok := data["entries"].([]pluginserver.ReloadRecord)
	require.True(t, ok)
	require.Len(t, entries, 3)
	assert.Equal(t, 3, data[keyCount])
	assert.Equal(t, uint64(3), entries[0].Generation)
	assert.Equal(t, "op-rel-3", entries[0].OpID)
	assert.Equal(t, uint64(2), entries[1].Generation)
	assert.Equal(t, "op-rel-2", entries[1].OpID)
	assert.Equal(t, uint64(1), entries[2].Generation)
	assert.Equal(t, "op-rel-1", entries[2].OpID)

	// count 2 narrows to the two newest entries.
	countResp, err := handleShowSystemReloadHistory(ctx, commandtest.Args(argCount, "2"))
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, countResp.Status)
	countData, ok := countResp.Data.(plugin.Map)
	require.True(t, ok)
	countEntries, ok := countData["entries"].([]pluginserver.ReloadRecord)
	require.True(t, ok)
	require.Len(t, countEntries, 2)
	assert.Equal(t, uint64(3), countEntries[0].Generation)
	assert.Equal(t, uint64(2), countEntries[1].Generation)

	// | operation op-rel-2 narrows to the failed record.
	opArgs := commandtest.Args().WithPipeOperations([]command.PipeOperation{
		{Name: pipeOpOperation, Args: []string{"op-rel-2"}},
	})
	opResp, err := handleShowSystemReloadHistory(ctx, opArgs)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, opResp.Status)
	opData, ok := opResp.Data.(plugin.Map)
	require.True(t, ok)
	opEntries, ok := opData["entries"].([]pluginserver.ReloadRecord)
	require.True(t, ok)
	require.Len(t, opEntries, 1)
	assert.Equal(t, "op-rel-2", opEntries[0].OpID)
	assert.Equal(t, "verify refused", opEntries[0].Error)

	// Invalid count bounds and unknown options return structured StatusError.
	for _, badArgs := range [][]string{
		{argCount},
		{argCount, "0"},
		{argCount, "-1"},
		{argCount, "33"},
		{argCount, "abc"},
		{"bogus"},
	} {
		badResp, badErr := handleShowSystemReloadHistory(ctx, commandtest.Args(badArgs...))
		require.NoError(t, badErr)
		assert.Equal(t, plugin.StatusError, badResp.Status, "args %v should be refused", badArgs)
	}

	assertOperationPipeRefusals(t, func(ops []command.PipeOperation) (*plugin.Response, error) {
		return handleShowSystemReloadHistory(ctx, commandtest.Args().WithPipeOperations(ops))
	})

	// show system reload status reports generation 3 and last-operation-id.
	statusResp, err := handleShowReloadStatus(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, statusResp.Status)
	statusData, ok := statusResp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, uint64(3), statusData["generation"])
	assert.Equal(t, pluginserver.ReloadOutcomeApplied, statusData["last-outcome"])
	assert.Equal(t, "op-rel-3", statusData["last-operation-id"])
}

// TestShowSystemWatchdogAndTriageSummary verifies show system watchdog's JSON
// contract and show system triage's runtime watchdog/hung-handler summary keys.
//
// VALIDATES: AC-6 -- show system watchdog emits kebab-case status, probes,
// hung-handlers, and recent-stalls as non-nil JSON arrays, and show system
// triage surfaces watchdog-running, stall-count, and hung-handler-count.
// PREVENTS: null JSON arrays or missing triage summary fields.
func TestShowSystemWatchdogAndTriageSummary(t *testing.T) {
	srv, err := pluginserver.NewServer(nil, nil)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: srv}

	resp, err := handleShowSystemWatchdog(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	raw, err := json.Marshal(resp.Data)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	for _, key := range []string{
		"running",
		"interval-ms",
		"stall-threshold-ms",
		"ticks",
		"stall-count",
		"probes",
		"hung-handlers",
		"recent-stalls",
	} {
		_, exists := decoded[key]
		assert.True(t, exists, "show system watchdog missing key %q", key)
	}
	_, probesIsSlice := decoded["probes"].([]any)
	_, hungIsSlice := decoded["hung-handlers"].([]any)
	_, stallsIsSlice := decoded["recent-stalls"].([]any)
	assert.True(t, probesIsSlice, "probes must serialize as JSON array")
	assert.True(t, hungIsSlice, "hung-handlers must serialize as JSON array")
	assert.True(t, stallsIsSlice, "recent-stalls must serialize as JSON array")

	srv.MarkReloadProcessedWithRecord(pluginserver.ReloadRecord{
		OpID:    "op-triage-7",
		Trigger: "commit",
		Outcome: pluginserver.ReloadOutcomeApplied,
	})

	triageResp, err := handleShowSystemTriage(ctx, commandtest.Args())
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, triageResp.Status)
	triageData, ok := triageResp.Data.(plugin.Map)
	require.True(t, ok)
	reloadSec := requireTriageMap(t, triageData, "reload")
	assert.Equal(t, "op-triage-7", reloadSec["last-operation-id"])
	runtimeSec := requireTriageMap(t, triageData, "runtime")
	_, hasWatchdogRunning := runtimeSec["watchdog-running"].(bool)
	_, hasStallCount := runtimeSec["stall-count"].(uint64)
	_, hasHungCount := runtimeSec["hung-handler-count"].(int)
	assert.True(t, hasWatchdogRunning, "triage runtime missing bool watchdog-running")
	assert.True(t, hasStallCount, "triage runtime missing uint64 stall-count")
	assert.True(t, hasHungCount, "triage runtime missing int hung-handler-count")
}

// TestProfileHandlerTimeout verifies show system profile declares an explicit
// timeout budget for CPU profiles while leaving instant snapshots on the
// dispatcher's default timeout.
//
// VALIDATES: AC-5 -- heap uses dispatcher default (0), default cpu requests
// 25s, cpu duration 60s requests 75s, and oversized durations clamp to 75s.
// PREVENTS: show system profile type cpu duration 60s being prematurely
// aborted by the 60s default dispatcher timeout.
func TestProfileHandlerTimeout(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{name: "default heap", args: nil, want: 0},
		{name: "explicit heap", args: []string{"type", "heap"}, want: 0},
		{name: "cpu default duration", args: []string{"type", "cpu"}, want: 25 * time.Second},
		{name: "bare cpu", args: []string{"cpu"}, want: 25 * time.Second},
		{name: "cpu duration 60s", args: []string{"type", "cpu", "duration", "60s"}, want: 75 * time.Second},
		{name: "cpu duration clamped", args: []string{"type", "cpu", "duration", "120s"}, want: 75 * time.Second},
		{name: "explicit duration", args: []string{"duration", "30s"}, want: 45 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := profileHandlerTimeout(commandtest.Args(tc.args...))
			assert.Equal(t, tc.want, got)
		})
	}
}

type blockingReloadReactor struct {
	plugin.ReactorLifecycle
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (r *blockingReloadReactor) ReloadRunning(_ map[string]any) map[string]any {
	r.once.Do(func() { close(r.entered) })
	<-r.release
	return map[string]any{}
}

func (*blockingReloadReactor) Stats() plugin.ReactorStats {
	return plugin.ReactorStats{StartTime: time.Now().UTC(), Uptime: time.Second}
}

// TestDiagnosticsSurviveHeldTxLock holds Server.txLock via an in-flight
// ReloadConfig blocked inside the reactor and verifies every core diagnostic
// handler completes immediately without blocking on txLock.
//
// VALIDATES: AC-4 -- core diagnostic handlers never acquire Server.txLock and
// remain reachable when a config reload transaction is wedged.
// PREVENTS: a future handler acquiring txLock and blinding live diagnosis
// during a wedged reload.
func TestDiagnosticsSurviveHeldTxLock(t *testing.T) {
	reactor := &blockingReloadReactor{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	srv, err := pluginserver.NewServer(nil, reactor)
	require.NoError(t, err)

	reloadDone := make(chan error, 1)
	go func() {
		reloadDone <- srv.ReloadConfig(context.Background(), map[string]any{})
	}()
	t.Cleanup(func() {
		close(reactor.release)
		select {
		case <-reloadDone:
		case <-time.After(5 * time.Second):
			t.Errorf("blocked ReloadConfig did not unwind on cleanup")
		}
	})

	select {
	case <-reactor.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("ReloadConfig did not enter blockingReloadReactor.ReloadRunning")
	}
	require.NotNil(t, srv.ConfigTransactionDone(), "precondition: Server.txLock must be held")

	ctx := &pluginserver.CommandContext{Server: srv}
	checks := []struct {
		name    string
		handler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)
		args    command.ValidatedArgs
	}{
		{name: "show system triage", handler: handleShowSystemTriage, args: commandtest.Args()},
		{name: "show system goroutines", handler: handleShowSystemGoroutines, args: commandtest.Args()},
		{name: "show system profile", handler: handleShowSystemProfile, args: commandtest.Args("type", "heap")},
		{name: "show system trace", handler: handleShowSystemTrace, args: commandtest.Args()},
		{name: "show runtime memory", handler: handleShowSystemMemory, args: commandtest.Args()},
		{name: "show event recent", handler: handleShowEventRecent, args: commandtest.Args()},
		{name: "show errors", handler: handleShowErrors, args: commandtest.Args()},
		{name: "show warnings", handler: handleShowWarnings, args: commandtest.Args()},
		{name: "show system reload status", handler: handleShowReloadStatus, args: commandtest.Args()},
		{name: "show system reload history", handler: handleShowSystemReloadHistory, args: commandtest.Args()},
		{name: "show system watchdog", handler: handleShowSystemWatchdog, args: commandtest.Args()},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			type outcome struct {
				resp *plugin.Response
				err  error
			}
			ch := make(chan outcome, 1)
			go func() {
				resp, callErr := tc.handler(ctx, tc.args)
				ch <- outcome{resp: resp, err: callErr}
			}()
			select {
			case got := <-ch:
				require.NoError(t, got.err)
				require.NotNil(t, got.resp)
				assert.Equal(t, plugin.StatusDone, got.resp.Status, "error: %s", got.resp.Error)
			case <-time.After(2 * time.Second):
				t.Fatalf("%s blocked while Server.txLock was held", tc.name)
			}
		})
	}

	assert.NotNil(t, srv.ConfigTransactionDone(), "Server.txLock must still be held after all diagnostic checks")
}

func assertOperationPipeRefusals(t *testing.T, invoke func([]command.PipeOperation) (*plugin.Response, error)) {
	t.Helper()
	cases := []struct {
		name    string
		ops     []command.PipeOperation
		wantSub string
	}{
		{
			name: "duplicate",
			ops: []command.PipeOperation{
				{Name: pipeOpOperation, Args: []string{"op-1"}},
				{Name: pipeOpOperation, Args: []string{"op-2"}},
			},
			wantSub: "duplicate | operation",
		},
		{
			name: "blank",
			ops: []command.PipeOperation{
				{Name: pipeOpOperation, Args: []string{"   "}},
			},
			wantSub: "| operation requires an operation ID",
		},
		{
			name: "missing",
			ops: []command.PipeOperation{
				{Name: pipeOpOperation},
			},
			wantSub: "| operation requires an operation ID",
		},
		{
			name: "extra-args",
			ops: []command.PipeOperation{
				{Name: pipeOpOperation, Args: []string{"op-1", "op-2"}},
			},
			wantSub: "| operation takes exactly one operation ID",
		},
	}
	for _, tc := range cases {
		t.Run("refuse-"+tc.name, func(t *testing.T) {
			resp, err := invoke(tc.ops)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, tc.wantSub)
		})
	}
}
