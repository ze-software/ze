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
	"golang.org/x/crypto/bcrypt"

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

// TestServer_BasicAuth verifies optional Basic Auth protects every metrics
// server path, including /health.
//
// VALIDATES: Basic Auth challenge and bcrypt password verification.
// PREVENTS: unauthenticated scrape access when basic-auth is enabled.
func TestServer_BasicAuth(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	reg := metrics.NewPrometheusRegistry()
	reg.Counter("basic_auth_total", "Test counter.").Inc()

	var srv metrics.Server
	err = srv.Start(reg, metrics.TelemetryConfig{
		Enabled:   true,
		Endpoints: []metrics.Endpoint{{Host: "127.0.0.1", Port: 19404}},
		Path:      "/metrics",
		BasicAuth: metrics.BasicAuthConfig{
			Enabled:  true,
			Realm:    "ze prometheus",
			Username: "prometheus",
			Password: string(hash),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Close()) })

	resp, err := http.Get("http://127.0.0.1:19404/metrics") //nolint:noctx // test code
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, `Basic realm="ze prometheus"`, resp.Header.Get("WWW-Authenticate"))

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:19404/metrics", nil)
	require.NoError(t, err)
	req.SetBasicAuth("prometheus", "wrong")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:19404/metrics", nil)
	require.NoError(t, err)
	req.SetBasicAuth("prometheus", "secret")
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "basic_auth_total 1")

	resp, err = http.Get("http://127.0.0.1:19404/health") //nolint:noctx // test code
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestServer_BasicAuthRequiresCredentials verifies an incomplete auth config
// fails before the listener is bound.
//
// VALIDATES: basic-auth enabled requires a username and bcrypt hash.
// PREVENTS: starting an endpoint that can never authenticate a scrape.
func TestServer_BasicAuthRequiresCredentials(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	tests := []struct {
		name string
		auth metrics.BasicAuthConfig
	}{
		{
			name: "missing username",
			auth: metrics.BasicAuthConfig{Enabled: true, Password: string(hash)},
		},
		{
			name: "missing password",
			auth: metrics.BasicAuthConfig{Enabled: true, Username: "prometheus"},
		},
		{
			name: "invalid hash",
			auth: metrics.BasicAuthConfig{Enabled: true, Username: "prometheus", Password: "cleartext"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := metrics.NewPrometheusRegistry()
			var srv metrics.Server
			err := srv.Start(reg, metrics.TelemetryConfig{
				Enabled:   true,
				Endpoints: []metrics.Endpoint{{Host: "127.0.0.1", Port: 19405}},
				Path:      "/metrics",
				BasicAuth: tt.auth,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "metrics server basic-auth")
		})
	}
}

// TestExtractTelemetryConfig verifies config tree extraction for telemetry settings.
//
// VALIDATES: ExtractTelemetryConfig returns correct values from config tree.
// VALIDATES: implicit telemetry listeners default to loopback.
// PREVENTS: Telemetry config not parsed or defaults not applied.
// PREVENTS: exposing unauthenticated Prometheus metrics on all interfaces by default.
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
			addr:    "127.0.0.1",
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
			addr:    "127.0.0.1",
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
			addr:    "127.0.0.1",
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
			addr:    "127.0.0.1",
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
			addr:    "127.0.0.1",
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
			addr:    "127.0.0.1",
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

// TestExtractTelemetryConfig_NetdataPrefix verifies the Netdata-compatible OS
// metric prefix defaults to "netdata" and supports deprecated aliases.
func TestExtractTelemetryConfig_NetdataPrefix(t *testing.T) {
	// Default prefix when not specified.
	cfg := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{"enabled": "true"},
		},
	})
	assert.Equal(t, "netdata", cfg.Netdata.Prefix)

	// Deprecated root alias.
	cfg = metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{"enabled": "true", "prefix": "node"},
		},
	})
	assert.Equal(t, "node", cfg.Netdata.Prefix)
	assert.Equal(t, []string{"telemetry.prometheus.prefix"}, cfg.DeprecatedAliases)

	// New netdata container takes precedence over the deprecated alias.
	cfg = metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"prefix":  "node",
				"netdata": map[string]any{"prefix": "netdata"},
			},
		},
	})
	assert.Equal(t, "netdata", cfg.Netdata.Prefix)
	assert.Equal(t, []string{"telemetry.prometheus.prefix"}, cfg.DeprecatedAliases)
}

// TestExtractTelemetryConfig_NetdataInterval verifies the Netdata-compatible OS
// collector interval is extracted from the netdata container.
func TestExtractTelemetryConfig_NetdataInterval(t *testing.T) {
	extract := func(interval string) int {
		netdata := map[string]any{}
		if interval != "" {
			netdata["interval"] = interval
		}
		prom := map[string]any{"enabled": "true", "netdata": netdata}
		return metrics.ExtractTelemetryConfig(map[string]any{
			"telemetry": map[string]any{"prometheus": prom},
		}).Netdata.Interval
	}

	assert.Equal(t, 1, extract(""))
	assert.Equal(t, 1, extract("1"))
	assert.Equal(t, 5, extract("5"))
	assert.Equal(t, 60, extract("60"))
	assert.Equal(t, 1, extract("0"))
	assert.Equal(t, 1, extract("61"))
	assert.Equal(t, 1, extract("-1"))
	assert.Equal(t, 1, extract("abc"))

	legacy := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{"enabled": "true", "interval": "10"},
		},
	})
	assert.Equal(t, 10, legacy.Netdata.Interval)
	assert.Equal(t, []string{"telemetry.prometheus.interval"}, legacy.DeprecatedAliases)

	precedence := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled":  "true",
				"interval": "5",
				"netdata":  map[string]any{"interval": "10"},
			},
		},
	})
	assert.Equal(t, 10, precedence.Netdata.Interval)
}

// TestExtractTelemetryConfig_NetdataCollectors verifies per-collector overrides
// are parsed from the Netdata-compatible OS collector list.
func TestExtractTelemetryConfig_NetdataCollectors(t *testing.T) {
	cfg := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"netdata": map[string]any{
					"collector": map[string]any{
						"diskspace": map[string]any{
							"enabled": "false",
						},
						"cpu": map[string]any{
							"interval": "5",
						},
						"snmp6": map[string]any{
							"enabled":  "true",
							"interval": "10",
						},
					},
				},
			},
		},
	})

	assert.Len(t, cfg.Netdata.Collectors, 3)

	ds := cfg.Netdata.Collectors["diskspace"]
	assert.False(t, ds.Enabled)
	assert.Equal(t, 0, ds.Interval)

	cpu := cfg.Netdata.Collectors["cpu"]
	assert.True(t, cpu.Enabled)
	assert.Equal(t, 5, cpu.Interval)

	snmp6 := cfg.Netdata.Collectors["snmp6"]
	assert.True(t, snmp6.Enabled)
	assert.Equal(t, 10, snmp6.Interval)

	// No collector list -> empty map.
	cfg2 := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{"enabled": "true"},
		},
	})
	assert.Empty(t, cfg2.Netdata.Collectors)

	legacy := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"collector": map[string]any{
					"cpu": map[string]any{"interval": "2"},
				},
			},
		},
	})
	assert.Equal(t, 2, legacy.Netdata.Collectors["cpu"].Interval)
	assert.Equal(t, []string{"telemetry.prometheus.collector"}, legacy.DeprecatedAliases)

	precedence := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"collector": map[string]any{
					"cpu": map[string]any{"interval": "2"},
				},
				"netdata": map[string]any{
					"collector": map[string]any{
						"snmp6": map[string]any{"interval": "10"},
					},
				},
			},
		},
	})
	assert.NotContains(t, precedence.Netdata.Collectors, "cpu")
	assert.Equal(t, 10, precedence.Netdata.Collectors["snmp6"].Interval)
}

// TestExtractTelemetryConfig_BasicAuth verifies Basic Auth settings are parsed
// from telemetry.prometheus.basic-auth.
func TestExtractTelemetryConfig_BasicAuth(t *testing.T) {
	cfg := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"basic-auth": map[string]any{
					"enabled":  "true",
					"realm":    "metrics",
					"username": "prometheus",
					"password": "$2a$10$abcdefghijklmnopqrstuu2J8Y27NbaVbhpI5NoRxw4ZmW0bErjUa",
				},
			},
		},
	})

	assert.True(t, cfg.BasicAuth.Enabled)
	assert.Equal(t, "metrics", cfg.BasicAuth.Realm)
	assert.Equal(t, "prometheus", cfg.BasicAuth.Username)
	assert.NotEmpty(t, cfg.BasicAuth.Password)
}

// TestExtractTelemetryConfig_NetdataDisabled verifies the netdata container can
// disable only OS collectors without disabling the Prometheus service.
func TestExtractTelemetryConfig_NetdataDisabled(t *testing.T) {
	cfg := metrics.ExtractTelemetryConfig(map[string]any{
		"telemetry": map[string]any{
			"prometheus": map[string]any{
				"enabled": "true",
				"netdata": map[string]any{"enabled": "false"},
			},
		},
	})

	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.Netdata.Enabled)
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
