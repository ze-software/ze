//go:build ze_telemetry

package exporter_test

import (
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/telemetry/exporter"
	"github.com/ze-software/ze/internal/core/metrics"
)

// TestStart_DisabledReturnsNil verifies Start is a no-op when telemetry is not
// enabled: no registry, no closer, no listener.
//
// VALIDATES: AC-1/AC-7 (disabled telemetry starts no exporter).
// PREVENTS: a metrics listener appearing without explicit enablement.
func TestStart_DisabledReturnsNil(t *testing.T) {
	reg, closer := exporter.Start(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{"enabled": "false"},
		},
	}, newTestLogger())
	assert.Nil(t, reg)
	assert.Nil(t, closer)

	// No telemetry block at all.
	reg, closer = exporter.Start(map[string]any{"bgp": map[string]any{}}, newTestLogger())
	assert.Nil(t, reg)
	assert.Nil(t, closer)
}

// TestStart_EnabledServesAndCloses verifies Start builds a registry, serves
// /metrics, and that the returned closer shuts the listener down.
//
// VALIDATES: AC-1 (exporter serves when enabled), AC-9 (closer stops it).
// PREVENTS: a leaked listener or a registry that is not wired to the handler.
func TestStart_EnabledServesAndCloses(t *testing.T) {
	reg, closer := exporter.Start(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"server": map[string]any{
					"main": map[string]any{"ip": "127.0.0.1", "port": "19279"},
				},
				"netdata": map[string]any{"enabled": "false"},
			},
		},
	}, newTestLogger())
	require.NotNil(t, reg)
	require.NotNil(t, closer)

	reg.Counter("start_test_total", "Test counter.").Add(3)

	resp, err := http.Get("http://127.0.0.1:19279/metrics") //nolint:noctx // test code
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "start_test_total 3")

	require.NoError(t, closer.Close())

	// After Close the listener is gone: the scrape must fail to connect.
	if resp, getErr := http.Get("http://127.0.0.1:19279/metrics"); getErr == nil { //nolint:noctx // test code
		_ = resp.Body.Close()
		t.Error("expected scrape to fail after Close, but the listener is still serving")
	}
}

// TestExporterStartsAndStopsPushCollectorsWithoutNetdata verifies Start invokes
// registered metrics push collectors even when netdata.enabled is false, and
// Close stops them cleanly.
//
// VALIDATES: AC-5 (registered push collectors run whenever prometheus export is
// enabled, independent of netdata.enabled, and stop on exporter Close).
// PREVENTS: Ze-native push collectors silently turning off when Netdata OS
// collectors are disabled or leaking goroutines after shutdown.
func TestExporterStartsAndStopsPushCollectorsWithoutNetdata(t *testing.T) {
	t.Cleanup(metrics.SnapshotPushCollectorsForTest())

	var started atomic.Int32
	var stopped atomic.Int32
	err := metrics.RegisterPushCollector("test-exporter-push", func(reg metrics.Registry, _ *slog.Logger) func() {
		started.Add(1)
		reg.Gauge("test_push_collector_value", "Test push collector gauge.").Set(42)
		return func() {
			stopped.Add(1)
		}
	})
	require.NoError(t, err)

	reg, closer := exporter.Start(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"server": map[string]any{
					"main": map[string]any{"ip": "127.0.0.1", "port": "19280"},
				},
				"netdata": map[string]any{"enabled": "false"},
			},
		},
	}, newTestLogger())
	require.NotNil(t, reg)
	require.NotNil(t, closer)

	assert.Equal(t, int32(1), started.Load())
	assert.Equal(t, int32(0), stopped.Load())

	resp, err := http.Get("http://127.0.0.1:19280/metrics") //nolint:noctx // test code
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "test_push_collector_value 42")

	require.NoError(t, closer.Close())
	assert.Equal(t, int32(1), stopped.Load())
}

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
