// Design: ai/rules/plugins.md -- ze_telemetry absent (compile-out) validation
//
//go:build !ze_telemetry

package hub

// VALIDATES: without the ze_telemetry build tag (e.g. ze-stripped or a hardened
// bare ze_core build), the Prometheus exporter seam is nil, the telemetry config
// schema is absent, and the binary contains no exporter symbols -- while metric
// collection stays always-on.
// PREVENTS: a regression where the exporter leaks into a hardened build via an
// always-on import or an ungated registration/schema import.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/metrics"
)

func TestBuildTag_Telemetry_Absent(t *testing.T) {
	if metrics.StartExporter != nil {
		t.Fatal("non-ze_telemetry build: metrics.StartExporter unexpectedly installed (not compiled out)")
	}
}

func TestBuildTag_Telemetry_AbsentRejectsTelemetryConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(`
telemetry {
	prometheus {
		enabled true;
	}
}
`, nil)
	if err == nil {
		t.Fatal("non-ze_telemetry build unexpectedly accepted telemetry config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("telemetry config rejection = %v, want clean unknown-field rejection", err)
	}
}

func TestBuildTag_Telemetry_AbsentBinaryDropsExporterSymbols(t *testing.T) {
	// -short guard only; this test still runs in full (make ze-standard-test
	// passes no -short). It builds and spawns the ze binary, so opt-in -short
	// runs skip it for speed. No coverage is lost in the verify/CI suite.
	if testing.Short() {
		t.Skip("builds the ze binary (slow); skipped under -short")
	}
	repoRoot := filepath.Join("..", "..", "..")
	bin := filepath.Join(t.TempDir(), "ze-core")
	build := exec.CommandContext(t.Context(), "go", "build", "-tags", "ze_core", "-o", bin, "./cmd/ze")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ze_core failed: %v\n%s", err, out)
	}

	out, err := exec.CommandContext(t.Context(), "go", "tool", "nm", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("go tool nm failed: %v\n%s", err, out)
	}
	needles := []string{
		"internal/component/telemetry/exporter.",
		"internal/component/telemetry/exporter/",
		"internal/component/telemetry/collector.",
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		for _, needle := range needles {
			if strings.Contains(line, needle) {
				t.Fatalf("ze_core binary retained telemetry exporter symbol %q matching %q", line, needle)
			}
		}
	}
}
