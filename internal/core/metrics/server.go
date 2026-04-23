// Design: docs/architecture/core-design.md — Prometheus HTTP server
// Overview: metrics.go — metric collection interfaces
// Related: prometheus.go — Prometheus backend providing Handler()

package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Endpoint is one parsed "address:port" listen entry for the metrics server.
type Endpoint struct {
	Host string
	Port int
}

// JoinHostPort returns the endpoint as a Go net.JoinHostPort string.
func (e Endpoint) JoinHostPort() string {
	return net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
}

// TelemetryConfig holds the parsed telemetry.prometheus block.
// Endpoints is guaranteed non-empty when Enabled is true.
// Entries are returned in YANG list key order (sorted alphabetically when
// the configuration tree does not preserve order, e.g. after ToMap()).
type TelemetryConfig struct {
	Enabled   bool
	Endpoints []Endpoint
	Path      string
	Prefix    string
}

// Server serves Prometheus metrics over HTTP on one or more listeners.
// Start binds every entry in the supplied endpoint slice; Shutdown / Close
// closes all of them because they are registered with the same *http.Server.
type Server struct {
	httpServer *http.Server
}

// Start binds every endpoint in cfg and begins serving Prometheus metrics
// from the given registry at cfg.Path.
//
// Bind is all-or-nothing: if ANY listener fails to bind, the already-bound
// listeners are closed and Start returns the bind error without entering
// the serve loop.
func (s *Server) Start(registry *PrometheusRegistry, cfg TelemetryConfig) error {
	if len(cfg.Endpoints) == 0 {
		return errors.New("metrics server: at least one endpoint is required")
	}
	path := cfg.Path
	if path == "" {
		path = "/metrics"
	}

	mux := http.NewServeMux()
	mux.Handle(path, registry.Handler())

	s.httpServer = &http.Server{
		// Addr is informational; multi-listener serving uses Serve(ln).
		Addr:              cfg.Endpoints[0].JoinHostPort(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var lc net.ListenConfig
	listeners := make([]net.Listener, 0, len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		ln, err := lc.Listen(context.Background(), "tcp", ep.JoinHostPort())
		if err != nil {
			for _, prev := range listeners {
				if closeErr := prev.Close(); closeErr != nil {
					// Best-effort rollback; carry on closing the rest.
					_ = closeErr
				}
			}
			s.httpServer = nil
			return fmt.Errorf("metrics server listen %s: %w", ep.JoinHostPort(), err)
		}
		listeners = append(listeners, ln)
	}

	// Lifecycle goroutine per listener. Every listener is registered with
	// the same *http.Server so Close() closes all of them.
	var wg sync.WaitGroup
	for _, ln := range listeners {
		wg.Add(1)
		go func(ln net.Listener) {
			defer wg.Done()
			if serveErr := s.httpServer.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				// Serve failures on metrics are not fatal; swallow and let
				// the other listeners keep running. Close() ultimately
				// stops the server.
				_ = serveErr
			}
		}(ln)
	}

	return nil
}

// Close shuts down the HTTP server. Safe to call without Start and idempotent
// (http.Server.Close is idempotent in the Go stdlib).
func (s *Server) Close() error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Close()
}

// ExtractTelemetryConfig extracts Prometheus telemetry settings from a config
// tree. The service must be explicitly enabled via the enabled leaf (default
// false). Every YANG `list server {}` entry is returned as an Endpoint in
// alphabetical key order; when the list is empty, one default entry using
// the YANG refine defaults is synthesized so the binder always sees at least
// one endpoint.
func ExtractTelemetryConfig(tree map[string]any) TelemetryConfig {
	var zero TelemetryConfig
	if tree == nil {
		return zero
	}
	telemetry, ok := tree["telemetry"].(map[string]any)
	if !ok {
		return zero
	}
	prom, ok := telemetry["prometheus"].(map[string]any)
	if !ok {
		return zero
	}

	// Service must be explicitly enabled (default false).
	enabledStr, _ := prom["enabled"].(string)
	if enabledStr != "true" {
		return zero
	}

	cfg := TelemetryConfig{Enabled: true}

	// Extract path (default: /metrics).
	cfg.Path, _ = prom["path"].(string)
	if cfg.Path == "" {
		cfg.Path = "/metrics"
	}

	// Extract prefix (default: netdata).
	cfg.Prefix, _ = prom["prefix"].(string)
	if cfg.Prefix == "" {
		cfg.Prefix = "netdata"
	}

	// Read every server list entry in alphabetical key order. ToMap() loses
	// the original insertion order, so alphabetical is the best deterministic
	// substitute; users who care about order should name entries accordingly
	// (primary, secondary, ...).
	if serverMap, ok := prom["server"].(map[string]any); ok && len(serverMap) > 0 {
		keys := make([]string, 0, len(serverMap))
		for k := range serverMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			srv, ok := serverMap[key].(map[string]any)
			if !ok {
				continue
			}
			ep := Endpoint{Host: "0.0.0.0", Port: 9273}
			if v, ok := srv["ip"].(string); ok && v != "" {
				ep.Host = v
			}
			if portStr, ok := srv["port"].(string); ok {
				if n, err := strconv.Atoi(portStr); err == nil && n >= 1 && n <= 65535 {
					ep.Port = n
				}
			}
			cfg.Endpoints = append(cfg.Endpoints, ep)
		}
	}

	// Synthesize a default entry when no list entries are present.
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []Endpoint{{Host: "0.0.0.0", Port: 9273}}
	}

	return cfg
}
