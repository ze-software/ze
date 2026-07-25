// Design: ai/rules/feature-gate-registration.md -- ze_telemetry present build validation
//
//go:build ze_telemetry

package hub

// VALIDATES: with the ze_telemetry build tag (the default ze / ze-appliance
// feature set), the Prometheus HTTP exporter compile-out seam is installed.
// PREVENTS: a regression where ze_telemetry is set but the exporter is not wired
// through the metrics.StartExporter seam.

import (
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
)

func TestBuildTag_Telemetry_Present(t *testing.T) {
	if metrics.StartExporter == nil {
		t.Fatal("ze_telemetry build: metrics.StartExporter seam not installed")
	}
}
