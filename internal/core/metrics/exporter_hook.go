// Design: docs/architecture/core-design.md — gated Prometheus HTTP exporter seam
// Overview: metrics.go — metric collection interfaces (always-on)
// Related: internal/component/telemetry/exporter — the gated exporter (ze_telemetry)

package metrics

import (
	"io"
	"log/slog"
)

// StartExporter is the seam between the always-on metric COLLECTION API and the
// optional Prometheus HTTP EXPORTER. When set (by the gated telemetry exporter
// package, wired via cmd/ze/hub/register_telemetry.go under //go:build
// ze_telemetry), it parses the telemetry config from tree and, if telemetry is
// enabled, creates a PrometheusRegistry, starts the Prometheus HTTP exporter
// (plus the Netdata OS collectors) on it, and returns the registry together with
// a closer that stops both. It returns (nil, nil) when telemetry is disabled or
// not configured.
//
// It is nil in a build without ze_telemetry. The metric collection API
// (Registry, PrometheusRegistry, the NopRegistry dummy, Gauge, Counter, ...)
// stays always-on, so every component that records metrics keeps working; only
// the HTTP exposure is compiled out. A no-telemetry build serves no /metrics or
// /health exporter listener and links zero exporter Server symbols.
//
// Call sites (the cmd/ze/hub standalone path and the bgp reactor path in
// internal/component/bgp/config) guard on `if StartExporter != nil` so neither
// imports the gated exporter package; the registry they inject via
// registry.SetMetricsRegistry is created here, not by the caller.
var StartExporter func(tree map[string]any, log *slog.Logger) (*PrometheusRegistry, io.Closer)
