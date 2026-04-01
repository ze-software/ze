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
// The service must be explicitly enabled via the enabled leaf (default false).
// Listener settings come from the server list; defaults used when list is empty.
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

	// Service must be explicitly enabled (default false).
	enabledStr, _ := prom["enabled"].(string)
	if enabledStr != "true" {
		return "", 0, "", false
	}

	// Defaults from YANG refine.
	address = "0.0.0.0"
	port = 9273

	// Read first server list entry if present.
	if serverMap, ok := prom["server"].(map[string]any); ok {
		for _, entry := range serverMap {
			if srv, ok := entry.(map[string]any); ok {
				if v, ok := srv["ip"].(string); ok && v != "" {
					address = v
				}
				if portStr, ok := srv["port"].(string); ok {
					if n, err := strconv.Atoi(portStr); err == nil && n >= 1 && n <= 65535 {
						port = n
					}
				}
				break // Use first entry only.
			}
		}
	}

	// Extract path (default: /metrics)
	path, _ = prom["path"].(string)
	if path == "" {
		path = "/metrics"
	}

	return address, port, path, true
}
