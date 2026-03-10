// Design: docs/architecture/core-design.md — Prometheus HTTP server
// Overview: metrics.go — metric collection interfaces
// Related: prometheus.go — Prometheus backend providing Handler()

package metrics

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Server serves Prometheus metrics over HTTP.
type Server struct {
	httpServer *http.Server
}

// Start begins serving metrics from the given PrometheusRegistry.
// Listens on address:port and serves the metrics handler at the given path.
func (s *Server) Start(registry *PrometheusRegistry, address string, port int, path string) error {
	mux := http.NewServeMux()
	mux.Handle(path, registry.Handler())

	listenAddr := net.JoinHostPort(address, strconv.Itoa(port))
	s.httpServer = &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("metrics server listen: %w", err)
	}

	go func() {
		if serveErr := s.httpServer.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			_ = serveErr
		}
	}()

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

// ExtractTelemetryConfig extracts Prometheus telemetry settings from a config tree.
// Returns address, port, path, and whether telemetry is enabled.
// Implicitly enabled if any of address/port/path is explicitly set.
func ExtractTelemetryConfig(tree map[string]any) (address string, port int, path string, enabled bool) {
	if tree == nil {
		return "", 0, "", false
	}
	telemetry, ok := tree["telemetry"].(map[string]any)
	if !ok {
		return "", 0, "", false
	}
	prom, ok := telemetry["prometheus"].(map[string]any)
	if !ok {
		return "", 0, "", false
	}

	// Explicit enable/disable takes precedence over implicit.
	enabledStr, _ := prom["enabled"].(string)
	if enabledStr == "false" {
		return "", 0, "", false
	}
	explicitlyEnabled := enabledStr == "true"

	// Implicit enable: any of address/port/path explicitly set
	_, hasAddress := prom["address"]
	_, hasPort := prom["port"]
	_, hasPath := prom["path"]
	implicitlyEnabled := hasAddress || hasPort || hasPath

	if !explicitlyEnabled && !implicitlyEnabled {
		return "", 0, "", false
	}

	// Extract address (default: 0.0.0.0)
	address, _ = prom["address"].(string)
	if address == "" {
		address = "0.0.0.0"
	}

	// Extract port (default: 9273, valid range: 1-65535)
	port = 9273
	if portStr, ok := prom["port"].(string); ok {
		if n, err := strconv.Atoi(portStr); err == nil && n >= 1 && n <= 65535 {
			port = n
		}
	}

	// Extract path (default: /metrics)
	path, _ = prom["path"].(string)
	if path == "" {
		path = "/metrics"
	}

	return address, port, path, true
}
