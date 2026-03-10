package metrics_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

// TestServer_StartAndScrape verifies the metrics HTTP server starts and serves metrics.
//
// VALIDATES: Server serves Prometheus metrics over HTTP.
// PREVENTS: Server not starting or metrics not being scraped.
func TestServer_StartAndScrape(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	c := reg.Counter("http_test_total", "Test counter for HTTP scrape.")
	c.Add(7)

	var srv metrics.Server
	err := srv.Start(reg, "127.0.0.1", 19274, "/metrics")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	resp, err := http.Get("http://127.0.0.1:19274/metrics") //nolint:noctx // test code, no context needed
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "http_test_total 7")
}

// TestServer_CloseIdempotent verifies Close is safe to call multiple times.
//
// VALIDATES: Server.Close is idempotent.
// PREVENTS: Panic on double close.
func TestServer_CloseIdempotent(t *testing.T) {
	var srv metrics.Server
	// Close without Start should not panic.
	require.NoError(t, srv.Close())
	require.NoError(t, srv.Close())
}

// TestServer_CloseAfterStartIdempotent verifies double Close after Start is safe.
//
// VALIDATES: Server.Close is idempotent after a successful Start.
// PREVENTS: Panic or error on double close of a started server.
func TestServer_CloseAfterStartIdempotent(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	var srv metrics.Server
	require.NoError(t, srv.Start(reg, "127.0.0.1", 19277, "/metrics"))
	require.NoError(t, srv.Close())
	require.NoError(t, srv.Close())
}

// TestServer_InvalidAddress verifies Start returns error for invalid address.
//
// VALIDATES: Server.Start returns error for bad listen address.
// PREVENTS: Silent failure on invalid address.
func TestServer_InvalidAddress(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	var srv metrics.Server
	err := srv.Start(reg, "999.999.999.999", 19275, "/metrics")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metrics server listen")
}

// TestServer_CustomPath verifies metrics are served at a custom path.
//
// VALIDATES: Server serves metrics at user-specified path.
// PREVENTS: Custom path ignored, metrics only at /metrics.
func TestServer_CustomPath(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("custom_path_total", "Test counter.").Inc()

	var srv metrics.Server
	err := srv.Start(reg, "127.0.0.1", 19276, "/custom")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	resp, err := http.Get("http://127.0.0.1:19276/custom") //nolint:noctx // test code
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "custom_path_total 1")
}

// TestExtractTelemetryConfig verifies config tree extraction for telemetry settings.
//
// VALIDATES: ExtractTelemetryConfig returns correct values from config tree.
// PREVENTS: Telemetry config not parsed or defaults not applied.
func TestExtractTelemetryConfig(t *testing.T) {
	tests := []struct {
		name    string
		tree    map[string]any
		addr    string
		port    int
		path    string
		enabled bool
	}{
		{
			name:    "nil tree",
			tree:    nil,
			enabled: false,
		},
		{
			name:    "no telemetry key",
			tree:    map[string]any{"bgp": map[string]any{}},
			enabled: false,
		},
		{
			name: "prometheus disabled",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "false",
					},
				},
			},
			enabled: false,
		},
		{
			name: "enabled with defaults",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "custom values",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"address": "127.0.0.1",
						"port":    "9999",
						"path":    "/custom",
					},
				},
			},
			addr:    "127.0.0.1",
			port:    9999,
			path:    "/custom",
			enabled: true,
		},
		{
			name: "implicit enable via port",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"port": "9273",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "implicit enable via address",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"address": "127.0.0.1",
					},
				},
			},
			addr:    "127.0.0.1",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "implicit enable via path",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"path": "/prom",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/prom",
			enabled: true,
		},
		{
			name: "empty prometheus block not enabled",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{},
				},
			},
			enabled: false,
		},
		{
			name: "boundary port 65535",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"port":    "65535",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    65535,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "boundary port 1",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"port":    "1",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    1,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "port 0 falls back to default",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"port":    "0",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "port 65536 falls back to default",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"port":    "65536",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "negative port falls back to default",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "true",
						"port":    "-1",
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "explicit disable overrides implicit port",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "false",
						"port":    "9999",
					},
				},
			},
			enabled: false,
		},
		{
			name: "explicit disable overrides implicit address",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "false",
						"address": "127.0.0.1",
					},
				},
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, port, path, enabled := metrics.ExtractTelemetryConfig(tt.tree)
			assert.Equal(t, tt.enabled, enabled)
			if enabled {
				assert.Equal(t, tt.addr, addr)
				assert.Equal(t, tt.port, port)
				assert.Equal(t, tt.path, path)
			}
		})
	}
}
