//go:build ze_bgp

// Design: docs/architecture/doctor-and-health-checks.md -- EOR EventBus emit failures log at Warn under CodeDoctorBGPPluginEventsUndelivered and raise on the report bus.
// Related: events.go -- onEORReceived, reportEOREmitFailure.
// Related: register_doctor_codes.go -- CodeDoctorBGPPluginEventsUndelivered, PluginEventsUndeliveredFix.

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// failingEOREventBus implements ze.EventBus with a configurable Emit error for
// testing onEORReceived's EventBus error handling.
type failingEOREventBus struct {
	emitErr error
	calls   int
}

func (f *failingEOREventBus) Emit(_, _ string, _ any) (int, error) {
	f.calls++
	if f.emitErr != nil {
		return 0, f.emitErr
	}
	return 1, nil
}

func (f *failingEOREventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// captureWarnJSONLog redirects the package logger to a default-level (WARN)
// JSON handler and resets the operational report bus for the duration of the
// test. Tests using package globals MUST NOT run in parallel.
func captureWarnJSONLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	var buf bytes.Buffer
	captured := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	oldLogger := logger
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = oldLogger })
	return &buf
}

// TestOnEORReceivedReportsEventBusEmitFailure proves that when onEORReceived
// fails to emit an End-of-RIB notification onto eorEventBus, it logs at default
// WARN level with CodeDoctorBGPPluginEventsUndelivered, peer, family, why, and
// fix, and raises a rate-bounded error event on the operational report bus.
//
// VALIDATES: AC-3 — answer-2 Warn logging with registered doctor code and
// report-bus raise for internal/component/bgp/server/events.go onEORReceived.
// PREVENTS: End-of-RIB EventBus delivery failures disappearing at Debug level.
func TestOnEORReceivedReportsEventBusEmitFailure(t *testing.T) {
	meta := diagnostic.Lookup(CodeDoctorBGPPluginEventsUndelivered)
	require.NotNil(t, meta, "doctor code %s must be registered", CodeDoctorBGPPluginEventsUndelivered)
	require.Equal(t, CodeDoctorBGPPluginEventsUndelivered, meta.Code)

	buf := captureWarnJSONLog(t)
	srv := newTestServer(t)
	mc := pluginserver.NewMonitorClient(t.Context(), "test-eor-monitor", nil, 4)
	srv.Monitors().Add(mc)
	t.Cleanup(func() { srv.Monitors().Remove("test-eor-monitor") })

	cause := errors.New("event bus unavailable")
	bus := &failingEOREventBus{emitErr: cause}
	oldBus := eorEventBus
	eorEventBus = bus
	t.Cleanup(func() { eorEventBus = oldBus })

	peer := testPeerInfo()
	onEORReceived(srv, peer, "ipv4/unicast")
	// Second failure within the 1m window must not duplicate the report-bus entry.
	onEORReceived(srv, peer, "ipv4/unicast")

	require.Equal(t, 2, bus.calls)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2, "expected two WARN log lines, got %q", buf.String())

	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &rec))
	require.Equal(t, "WARN", rec["level"])
	require.Equal(t, CodeDoctorBGPPluginEventsUndelivered, rec["code"])
	require.Equal(t, peer.AddrStr(), rec["peer"])
	require.Equal(t, "ipv4/unicast", rec["family"])
	require.Equal(t, PluginEventsUndeliveredFix, rec["fix"])
	require.Equal(t, cause.Error(), rec["error"])
	require.NotEmpty(t, rec["why"])

	issues := report.Errors(0)
	require.Len(t, issues, 1, "RaiseErrorFixOnce must rate-bound repeated failures within the 1m window")
	require.Equal(t, reportSourceBGP, issues[0].Source)
	require.Equal(t, CodeDoctorBGPPluginEventsUndelivered, issues[0].Code)
	require.Equal(t, peer.AddrStr()+"/ipv4/unicast", issues[0].Subject)
	require.Equal(t, PluginEventsUndeliveredFix, issues[0].Fix)
	require.Contains(t, issues[0].Message, cause.Error())
}

// TestOnEORReceivedSuccessEmitsNoWarning proves that a successful EOR EventBus
// emit produces neither a WARN log nor a report-bus error entry.
func TestOnEORReceivedSuccessEmitsNoWarning(t *testing.T) {
	buf := captureWarnJSONLog(t)
	srv := newTestServer(t)
	mc := pluginserver.NewMonitorClient(t.Context(), "test-eor-monitor-ok", nil, 4)
	srv.Monitors().Add(mc)
	t.Cleanup(func() { srv.Monitors().Remove("test-eor-monitor-ok") })

	bus := &failingEOREventBus{}
	oldBus := eorEventBus
	eorEventBus = bus
	t.Cleanup(func() { eorEventBus = oldBus })

	onEORReceived(srv, testPeerInfo(), "ipv6/unicast")

	require.Equal(t, 1, bus.calls)
	require.Empty(t, buf.String())
	require.Empty(t, report.Errors(0))
}
