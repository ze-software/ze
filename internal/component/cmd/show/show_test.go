package show

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
)

// TestHandleShowWarningsEmpty verifies handleShowWarnings returns a non-nil
// empty list with count=0 when the report bus has no active warnings.
//
// VALIDATES: ze show warnings against an idle daemon returns
// {"warnings": [], "count": 0}, exit 0.
// PREVENTS: regression where empty bus produces null instead of [],
// or omits the count field, breaking operator expectations.
func TestHandleShowWarningsEmpty(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	resp, err := handleShowWarnings(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	assert.Equal(t, 0, data["count"])

	warnings, ok := data["warnings"].([]report.Issue)
	require.True(t, ok, "warnings should be []report.Issue, got %T", data["warnings"])
	assert.Empty(t, warnings)
	assert.NotNil(t, warnings, "warnings should be empty slice, not nil, for consistent JSON encoding")
}

// TestHandleShowAAAAccounting relocated to internal/component/aaa/cmd/show_test.go
// for plugin self-containment (ai/rules/plugins.md).

// VALIDATES: AC-13 -- show audit returns structured audit records filterable by time range and action.
// PREVENTS: Audit log existing on disk but being unavailable to operators.
func TestHandleShowAuditFilters(t *testing.T) {
	since := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	until := since.Add(time.Hour)
	var got audit.Filter
	RegisterAuditProvider(func(filter audit.Filter) []audit.Entry {
		got = filter
		return []audit.Entry{{Timestamp: since, Actor: "alice", Surface: audit.SSH, Action: audit.ActionConfigCommit, Outcome: audit.OutcomeSuccess}}
	})
	t.Cleanup(func() { RegisterAuditProvider(nil) })

	resp, err := handleShowAudit(nil, []string{"action", audit.ActionConfigCommit, "since", since.Format(time.RFC3339), "until", until.Format(time.RFC3339), "count", "5"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, audit.ActionConfigCommit, got.Action)
	assert.Equal(t, since, got.Since)
	assert.Equal(t, until, got.Until)
	assert.Equal(t, 5, got.Limit)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 1, data["count"])
	entries, ok := data["entries"].([]audit.Entry)
	require.True(t, ok)
	require.Len(t, entries, 1)
	assert.Equal(t, "alice", entries[0].Actor)
}

// TestHandleShowWarningsPopulated verifies handleShowWarnings returns the
// active warning entries seeded into the report bus.
//
// VALIDATES: ze show warnings reflects the bus snapshot, ordered most-
// recently-updated first, with the correct Source/Code/Subject/Message/
// Detail fields and Severity == warning.
// PREVENTS: regression where the handler returns wrong-shape entries
// (right count, wrong content) that a length-only assertion would miss.
func TestHandleShowWarningsPopulated(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	// Raise the older entry first so its Updated timestamp is earlier.
	report.RaiseWarning("bgp", "prefix-stale", "10.0.0.1", "stale data",
		map[string]any{"updated": "2024-01-01"})
	// Tiny pause so the second entry has a strictly later Updated timestamp,
	// guaranteeing the snapshot's most-recently-updated-first ordering is
	// observable in the test (matters on systems with sub-millisecond
	// time.Now() resolution).
	time.Sleep(2 * time.Millisecond)
	report.RaiseWarning("bgp", "prefix-threshold", "10.0.0.2/ipv4/unicast",
		"over warning", map[string]any{"family": "ipv4/unicast"})

	resp, err := handleShowWarnings(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])
	warnings, ok := data["warnings"].([]report.Issue)
	require.True(t, ok)
	require.Len(t, warnings, 2)

	// Most-recently-updated first: prefix-threshold (raised second) at index 0,
	// prefix-stale (raised first) at index 1. Verifies the ordering contract
	// in report.Warnings's godoc.
	assert.Equal(t, "prefix-threshold", warnings[0].Code)
	assert.Equal(t, "10.0.0.2/ipv4/unicast", warnings[0].Subject)
	assert.Equal(t, report.SeverityWarning, warnings[0].Severity)
	assert.Equal(t, "ipv4/unicast", warnings[0].Detail["family"])
	assert.Equal(t, "over warning", warnings[0].Message)

	assert.Equal(t, "prefix-stale", warnings[1].Code)
	assert.Equal(t, "10.0.0.1", warnings[1].Subject)
	assert.Equal(t, report.SeverityWarning, warnings[1].Severity)
	assert.Equal(t, "2024-01-01", warnings[1].Detail["updated"])
	assert.Equal(t, "stale data", warnings[1].Message)

	// Both entries share the same source.
	assert.Equal(t, "bgp", warnings[0].Source)
	assert.Equal(t, "bgp", warnings[1].Source)
}

// TestHandleShowErrorsEmpty verifies handleShowErrors returns a non-nil
// empty list with count=0 when the report bus has no error events.
//
// VALIDATES: ze show errors against an idle daemon returns
// {"errors": [], "count": 0}, exit 0.
// PREVENTS: regression where empty ring buffer produces null instead of [].
func TestHandleShowErrorsEmpty(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	resp, err := handleShowErrors(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	assert.Equal(t, 0, data["count"])

	errs, ok := data["errors"].([]report.Issue)
	require.True(t, ok, "errors should be []report.Issue, got %T", data["errors"])
	assert.Empty(t, errs)
	assert.NotNil(t, errs, "errors should be empty slice, not nil, for consistent JSON encoding")
}

// TestHandleShowErrorsPopulated verifies handleShowErrors returns the
// recent error events seeded into the report bus.
//
// VALIDATES: ze show errors reflects the ring buffer snapshot, newest first,
// with the correct Source/Code/Subject/Message/Detail fields and Severity ==
// error.
// PREVENTS: regression where the handler returns wrong-shape entries that a
// length-only assertion would miss.
func TestHandleShowErrorsPopulated(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseError("bgp", "notification-sent", "10.0.0.1",
		"first", map[string]any{"code": uint8(6), "subcode": uint8(2)})
	report.RaiseError("bgp", "notification-received", "10.0.0.2",
		"second", map[string]any{"code": uint8(2), "subcode": uint8(4)})

	resp, err := handleShowErrors(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])
	errs, ok := data["errors"].([]report.Issue)
	require.True(t, ok)
	require.Len(t, errs, 2)

	// Newest first: notification-received (raised second) at index 0,
	// notification-sent (raised first) at index 1.
	assert.Equal(t, "notification-received", errs[0].Code)
	assert.Equal(t, "10.0.0.2", errs[0].Subject)
	assert.Equal(t, report.SeverityError, errs[0].Severity)
	assert.Equal(t, uint8(2), errs[0].Detail["code"])
	assert.Equal(t, uint8(4), errs[0].Detail["subcode"])
	assert.Equal(t, "second", errs[0].Message)

	assert.Equal(t, "notification-sent", errs[1].Code)
	assert.Equal(t, "10.0.0.1", errs[1].Subject)
	assert.Equal(t, report.SeverityError, errs[1].Severity)
	assert.Equal(t, uint8(6), errs[1].Detail["code"])
	assert.Equal(t, uint8(2), errs[1].Detail["subcode"])
	assert.Equal(t, "first", errs[1].Message)

	// Both entries share the same source.
	assert.Equal(t, "bgp", errs[0].Source)
	assert.Equal(t, "bgp", errs[1].Source)
}

// TestHandleShowUptime_NilReactor verifies show uptime returns error when no daemon running.
//
// VALIDATES: show uptime with nil reactor returns StatusError.
// PREVENTS: Panic when reactor is nil.
func TestHandleShowUptime_NilReactor(t *testing.T) {
	// nil CommandContext -> Reactor() returns nil.
	resp, err := handleShowUptime(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

func TestWarningsFilterBySource(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("bgp", "prefix-threshold", "10.0.0.1", "v4 over", nil)
	report.RaiseWarning("l2tp", "echo-loss", "user1", "high loss", nil)
	report.RaiseWarning("bgp", "prefix-stale", "10.0.0.2", "stale", nil)

	resp, err := handleShowWarnings(nil, []string{"source", "bgp"})
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])

	resp2, err := handleShowWarnings(nil, []string{"source", "l2tp"})
	require.NoError(t, err)
	data2, ok := resp2.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 1, data2["count"])
}

func TestWarningsNoFilter(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("bgp", "test", "peer1", "msg", nil)
	report.RaiseWarning("l2tp", "test", "user1", "msg", nil)

	resp, err := handleShowWarnings(nil, nil)
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, 2, data["count"])
}

func TestErrorsFilterBySource(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseError("bgp", "decode-fail", "10.0.0.1", "bad update", nil)
	report.RaiseError("l2tp", "tunnel-fail", "peer1", "timeout", nil)
	report.RaiseError("bgp", "hold-timer", "10.0.0.2", "expired", nil)
	report.RaiseError("l2tp", "session-fail", "peer2", "cdn received", nil)

	resp, err := handleShowErrors(nil, []string{"source", "l2tp", "count", "1"})
	require.NoError(t, err)
	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	issues, ok2 := data["errors"].([]report.Issue)
	require.True(t, ok2, "errors should be []report.Issue")
	assert.Equal(t, 1, len(issues))
	assert.Equal(t, "l2tp", issues[0].Source)
}

// metricsQueryContext seeds the metrics registry with one counter vector and
// returns the command context `show metrics name ze_test_filter_total` builds:
// the metric name arrives as a selector, so args carry label filters only.
//
// The registry is package state, so the caller's cleanup clears it.
func metricsQueryContext(t *testing.T) *pluginserver.CommandContext {
	t.Helper()
	reg := metrics.NewPrometheusRegistry()
	series := reg.CounterVec("ze_test_filter_total", "Series a label filter selects between.", []string{"peer"})
	series.With("p1").Inc()
	series.With("p2").Inc()
	registry.SetMetricsRegistry(reg)
	t.Cleanup(func() { registry.SetMetricsRegistry(nil) })
	return &pluginserver.CommandContext{Selectors: map[string]string{"name": "ze_test_filter_total"}}
}

// TestShowMetricsQueryRefusesAMalformedLabelFilter verifies a label group that
// stops short of its value, and a token that opens no group at all, are both
// refused with an error that states the form.
//
// VALIDATES: `show metrics name <name> label <key>` reports an error rather
// than answering every series of the metric.
// PREVENTS: the silent drop the `label=value` packing carried, where a token
// with no `=` was skipped and the query answered as if no filter was asked
// for (internal/component/cmd/show/show.go, handleShowMetricsQuery).
func TestShowMetricsQueryRefusesAMalformedLabelFilter(t *testing.T) {
	ctx := metricsQueryContext(t)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "keyword alone", args: []string{"label"}, want: "label needs a key and a value"},
		{name: "key without a value", args: []string{"label", "peer"}, want: "label needs a key and a value"},
		{name: "second group short", args: []string{"label", "peer", "p1", "label", "family"}, want: "label needs a key and a value"},
		{name: "token that opens no group", args: []string{"peer"}, want: `"peer" is not the label keyword`},
		{name: "value where a keyword belongs", args: []string{"label", "peer", "p1", "p2"}, want: `"p2" is not the label keyword`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleShowMetricsQuery(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, tt.want)
			assert.Contains(t, resp.Error, "show metrics name <name> [label <key> <value> ...]")
		})
	}
}

// TestShowMetricsQueryFiltersSeriesByLabel verifies a two-token label group
// selects the series whose label holds that value.
//
// VALIDATES: `show metrics name ze_test_filter_total label peer p1` answers the
// one series carrying peer="p1".
// PREVENTS: a parse that accepts the tokens and then filters on nothing, which
// would answer both series and read as success.
func TestShowMetricsQueryFiltersSeriesByLabel(t *testing.T) {
	ctx := metricsQueryContext(t)

	tests := []struct {
		name  string
		args  []string
		count int
	}{
		{name: "no filter answers every series", args: nil, count: 2},
		{name: "one filter selects one series", args: []string{"label", "peer", "p1"}, count: 1},
		{name: "a value nothing carries selects none", args: []string{"label", "peer", "p9"}, count: 0},
		{name: "one label named twice states two conditions", args: []string{"label", "peer", "p1", "label", "peer", "p2"}, count: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleShowMetricsQuery(ctx, tt.args)
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, plugin.StatusDone, resp.Status)
			data, ok := resp.Data.(plugin.Map)
			require.True(t, ok, "response data is %T, want plugin.Map", resp.Data)
			assert.Equal(t, tt.count, data[keyCount])
		})
	}
}
