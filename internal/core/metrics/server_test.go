package metrics_test

import (
	"context"
	"fmt"
	"io"
	"net"
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
	err := srv.Start(reg, metrics.TelemetryConfig{
		Enabled:   true,
		Endpoints: []metrics.Endpoint{{Host: "127.0.0.1", Port: 19274}},
		Path:      "/metrics",
	})
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
	require.NoError(t, srv.Start(reg, metrics.TelemetryConfig{
		Enabled:   true,
		Endpoints: []metrics.Endpoint{{Host: "127.0.0.1", Port: 19277}},
		Path:      "/metrics",
	}))
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
	err := srv.Start(reg, metrics.TelemetryConfig{
		Enabled:   true,
		Endpoints: []metrics.Endpoint{{Host: "999.999.999.999", Port: 19275}},
		Path:      "/metrics",
	})
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
	err := srv.Start(reg, metrics.TelemetryConfig{
		Enabled:   true,
		Endpoints: []metrics.Endpoint{{Host: "127.0.0.1", Port: 19276}},
		Path:      "/custom",
	})
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
						"server": map[string]any{
							"main": map[string]any{
								"ip":   "127.0.0.1",
								"port": "9999",
							},
						},
						"path": "/custom",
					},
				},
			},
			addr:    "127.0.0.1",
			port:    9999,
			path:    "/custom",
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
						"server": map[string]any{
							"main": map[string]any{
								"port": "65535",
							},
						},
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
						"server": map[string]any{
							"main": map[string]any{
								"port": "1",
							},
						},
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
						"server": map[string]any{
							"main": map[string]any{
								"port": "0",
							},
						},
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
						"server": map[string]any{
							"main": map[string]any{
								"port": "65536",
							},
						},
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
						"server": map[string]any{
							"main": map[string]any{
								"port": "-1",
							},
						},
					},
				},
			},
			addr:    "0.0.0.0",
			port:    9273,
			path:    "/metrics",
			enabled: true,
		},
		{
			name: "explicit disable with server config",
			tree: map[string]any{
				"telemetry": map[string]any{
					"prometheus": map[string]any{
						"enabled": "false",
						"server": map[string]any{
							"main": map[string]any{
								"ip":   "127.0.0.1",
								"port": "9999",
							},
						},
					},
				},
			},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := metrics.ExtractTelemetryConfig(tt.tree)
			assert.Equal(t, tt.enabled, cfg.Enabled)
			if cfg.Enabled {
				require.NotEmpty(t, cfg.Endpoints, "enabled config must yield at least one endpoint")
				assert.Equal(t, tt.addr, cfg.Endpoints[0].Host)
				assert.Equal(t, tt.port, cfg.Endpoints[0].Port)
				assert.Equal(t, tt.path, cfg.Path)
			}
		})
	}
}

// TestExtractTelemetryConfig_MultipleServers verifies every YANG list entry
// becomes an Endpoint in the returned slice, sorted alphabetically by key.
//
// VALIDATES: AC-4 (telemetry config with two server entries yields two
// endpoints). The original "first entry only" behavior (the `break` on
// line 97 of the old server.go) is gone.
// PREVENTS: Silent drop of extra telemetry listeners.
func TestExtractTelemetryConfig_MultipleServers(t *testing.T) {
	tree := map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"server": map[string]any{
					// Alphabetical order: "a" first, "b" second.
					"a": map[string]any{"ip": "0.0.0.0", "port": "9273"},
					"b": map[string]any{"ip": "127.0.0.1", "port": "19273"},
				},
				"path": "/metrics",
			},
		},
	}

	cfg := metrics.ExtractTelemetryConfig(tree)
	require.True(t, cfg.Enabled)
	require.Len(t, cfg.Endpoints, 2)
	assert.Equal(t, "0.0.0.0", cfg.Endpoints[0].Host)
	assert.Equal(t, 9273, cfg.Endpoints[0].Port)
	assert.Equal(t, "127.0.0.1", cfg.Endpoints[1].Host)
	assert.Equal(t, 19273, cfg.Endpoints[1].Port)
}

// TestServer_MultiListener verifies Server.Start binds every endpoint and
// both listeners serve the same metrics handler.
//
// VALIDATES: AC-4 (multi-listener binding end-to-end).
// VALIDATES: AC-14 (Close shuts every listener down).
// PREVENTS: Regression where only the first telemetry listener is bound.
func TestServer_MultiListener(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	reg.Counter("multi_listener_total", "Test counter.").Add(42)

	var srv metrics.Server
	err := srv.Start(reg, metrics.TelemetryConfig{
		Enabled: true,
		Endpoints: []metrics.Endpoint{
			{Host: "127.0.0.1", Port: 19401},
			{Host: "127.0.0.1", Port: 19402},
		},
		Path: "/metrics",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, srv.Close())
	})

	for _, addr := range []string{"127.0.0.1:19401", "127.0.0.1:19402"} {
		resp, getErr := http.Get("http://" + addr + "/metrics") //nolint:noctx // test code, no context needed
		require.NoError(t, getErr, "GET %s", addr)
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, readErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "listener %s", addr)
		assert.Contains(t, string(body), "multi_listener_total 42", "listener %s", addr)
	}
}

// TestServer_BindFailureRollsBack verifies that when the second endpoint
// fails to bind, the first listener is closed and Start returns the error.
//
// VALIDATES: AC-15 (fail-fast on partial bind).
func TestServer_BindFailureRollsBack(t *testing.T) {
	// Squat on a port so the second bind fails.
	squatter, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		if closeErr := squatter.Close(); closeErr != nil {
			t.Logf("close squatter: %v", closeErr)
		}
	})
	_, squattedPortStr, err := net.SplitHostPort(squatter.Addr().String())
	require.NoError(t, err)
	var squattedPort int
	_, err = fmt.Sscanf(squattedPortStr, "%d", &squattedPort)
	require.NoError(t, err)

	reg := metrics.NewPrometheusRegistry()
	var srv metrics.Server
	err = srv.Start(reg, metrics.TelemetryConfig{
		Enabled: true,
		Endpoints: []metrics.Endpoint{
			{Host: "127.0.0.1", Port: 19403},
			{Host: "127.0.0.1", Port: squattedPort},
		},
		Path: "/metrics",
	})
	require.Error(t, err, "Start must fail when any bind fails")
	assert.Contains(t, err.Error(), "metrics server listen")

	// The first port must be free again (partial rollback).
	probe, probeErr := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:19403")
	if probeErr != nil {
		t.Errorf("first port should be free after bind failure rollback: %v", probeErr)
	} else {
		if closeErr := probe.Close(); closeErr != nil {
			t.Logf("close probe: %v", closeErr)
		}
	}
}
