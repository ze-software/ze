// Design: ai/rules/feature-gate-registration.md -- ze_telemetry compile-out seam
//
// Wires the gated Prometheus HTTP exporter into the always-on
// metrics.StartExporter seam. Compiled only under //go:build ze_telemetry;
// absent the tag this init() does not run, the seam stays nil, the exporter
// package is dropped from the binary, and the daemon serves no /metrics listener
// (metric collection stays always-on). ze / ze-appliance always link the hub,
// so both the standalone hub path and the bgp reactor path reach the wired seam.

//go:build ze_telemetry

package hub

import (
	"github.com/ze-software/ze/internal/component/telemetry/exporter"
	"github.com/ze-software/ze/internal/core/metrics"
)

func init() {
	metrics.StartExporter = exporter.Start
}
