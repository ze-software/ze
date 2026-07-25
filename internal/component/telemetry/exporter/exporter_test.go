//go:build ze_telemetry

package exporter_test

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/telemetry/exporter"
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

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
