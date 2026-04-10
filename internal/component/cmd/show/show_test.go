package show

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
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

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "Data should be map[string]any, got %T", resp.Data)
	assert.Equal(t, 0, data["count"])

	warnings, ok := data["warnings"].([]report.Issue)
	require.True(t, ok, "warnings should be []report.Issue, got %T", data["warnings"])
	assert.Empty(t, warnings)
	assert.NotNil(t, warnings, "warnings should be empty slice, not nil, for consistent JSON encoding")
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

	data, ok := resp.Data.(map[string]any)
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

	data, ok := resp.Data.(map[string]any)
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

	data, ok := resp.Data.(map[string]any)
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

func TestHandleShowInterface(t *testing.T) {
	// List all interfaces -- requires iface backend.
	resp, err := handleShowInterface(nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.Status == "error" && resp.Data == "iface: no backend loaded" {
		t.Skip("iface backend not available in test environment")
	}
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo") // loopback always exists

	// Show specific interface -- loopback always exists.
	resp, err = handleShowInterface(nil, []string{"lo"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Contains(t, resp.Data, "lo")

	// Show nonexistent interface -- should return error response.
	resp, err = handleShowInterface(nil, []string{"nonexistent_iface99"})
	require.NoError(t, err) // Go error nil, operational error in Response
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
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

// TestHandleShowInterfaceBrief verifies show interface brief dispatches correctly.
//
// VALIDATES: show interface brief dispatches to showInterfaceBrief handler.
// PREVENTS: Brief mode not recognized, falls through to single-interface lookup.
func TestHandleShowInterfaceBrief(t *testing.T) {
	resp, err := handleShowInterface(nil, []string{"brief"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// On systems with netlink, returns "done" with interface list.
	// On systems without netlink (CI), returns "error" from ListInterfaces.
	// Either way, the brief path was taken (not the single-interface path).
	if resp.Status == "done" {
		data, ok := resp.Data.(map[string]any)
		require.True(t, ok, "brief response should be map")
		_, hasInterfaces := data["interfaces"]
		assert.True(t, hasInterfaces, "should have interfaces key")
		_, hasCount := data["count"]
		assert.True(t, hasCount, "should have count key")
	}
}
