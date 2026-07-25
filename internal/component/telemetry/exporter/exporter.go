//go:build ze_telemetry

// Design: docs/architecture/core-design.md -- Prometheus HTTP exporter orchestration

package exporter

import (
	"io"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/telemetry/collector"
	"github.com/ze-software/ze/internal/core/metrics"
)

// Start parses the telemetry config from tree and, if telemetry is enabled,
// creates a PrometheusRegistry, starts the Prometheus HTTP exporter (plus the
// Netdata OS collectors) on it, and returns the registry together with a closer
// that stops both. It returns (nil, nil) when telemetry is disabled or not
// configured.
//
// Start is the package's only exported symbol. It is wired into the always-on
// metrics.StartExporter seam by the hub's register_telemetry.go (//go:build
// ze_telemetry). The caller injects the returned registry via
// registry.SetMetricsRegistry; collection is unaffected when this package is
// compiled out (metrics.StartExporter stays nil).
func Start(tree map[string]any, log *slog.Logger) (*metrics.PrometheusRegistry, io.Closer) {
	cfg := extractTelemetryConfig(tree)
	if !cfg.Enabled {
		return nil, nil
	}

	reg := metrics.NewPrometheusRegistry()
	// Phase 6: expose Go runtime (go_*) and process (process_*) metrics.
	reg.RegisterRuntimeCollectors()
	srv := &server{}
	if err := srv.start(reg, cfg); err != nil {
		log.Warn("metrics server failed to start", "error", err)
		return nil, nil
	}

	for _, path := range cfg.DeprecatedAliases {
		log.Warn("deprecated prometheus telemetry config; move setting under telemetry.prometheus.netdata", "path", path)
	}
	for _, ep := range cfg.Endpoints {
		log.Info("prometheus metrics enabled", "address", ep.Host, "port", ep.Port, "path", cfg.Path)
	}

	var mgr *collector.Manager
	if cfg.Netdata.Enabled {
		overrides := make(map[string]collector.CollectorOverride, len(cfg.Netdata.Collectors))
		for name, cc := range cfg.Netdata.Collectors {
			overrides[name] = collector.CollectorOverride{
				Enabled:  cc.Enabled,
				Interval: time.Duration(cc.Interval) * time.Second,
			}
		}
		mgr = collector.StartOSCollectors(reg, cfg.Netdata.Prefix, time.Duration(cfg.Netdata.Interval)*time.Second, overrides, log)
	}

	return reg, &exporterCloser{srv: srv, mgr: mgr}
}

// exporterCloser stops the HTTP exporter and (if started) the Netdata OS
// collectors. Returned from Start as an io.Closer so call sites can shut the
// exporter down without referencing the concrete types.
type exporterCloser struct {
	srv *server
	mgr *collector.Manager
}

func (c *exporterCloser) Close() error {
	if c.mgr != nil {
		c.mgr.Stop()
	}
	return c.srv.close()
}
