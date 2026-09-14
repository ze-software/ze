//go:build ze_telemetry && linux

// Design: docs/architecture/core-design.md -- the readiness check the Prometheus exporter owns
// Detail: doctor_linux.go -- checkTelemetryProcfs and its registration

package exporter

import (
	"os"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withUnreadableProcfs stands in a procfs that refuses every read for the
// life of one test.
func withUnreadableProcfs(t *testing.T) {
	t.Helper()
	previous := procfsReadFile
	procfsReadFile = func(string) ([]byte, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { procfsReadFile = previous })
}

// prometheusTree builds the smallest config that enables the exporter.
func prometheusTree(enabled bool) *config.Tree {
	tree := config.NewTree()
	prom := tree.GetOrCreateContainer("telemetry").GetOrCreateContainer("prometheus")
	if enabled {
		prom.Set("enabled", "true")
	}
	return tree
}

// TestCheckTelemetryProcfsWarnsWhenProcStatIsUnreadable drives the check over
// an enabled exporter on a procfs that refuses the read.
//
// VALIDATES: doctor-telemetry-procfs at warning severity, naming the path.
// PREVENTS: OS collector readiness being assumed from config alone.
func TestCheckTelemetryProcfsWarnsWhenProcStatIsUnreadable(t *testing.T) {
	withUnreadableProcfs(t)

	diags := checkTelemetryProcfs(diagnostic.DoctorCheckContext{Tree: prometheusTree(true)})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeTelemetryProcfs || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %s/%s, want %s at warning", diags[0].Code, diags[0].Severity, codeTelemetryProcfs)
	}
	if diags[0].Path == "" {
		t.Fatal("the diagnostic names no path")
	}
}

// TestCheckTelemetryProcfsIsSilentWhenNotNeeded is the negative half: a
// disabled exporter, no telemetry block, no config, and a readable procfs each
// name no dependency.
//
// VALIDATES: no diagnostic in each case.
// PREVENTS: a procfs warning on every box that never enabled the exporter.
func TestCheckTelemetryProcfsIsSilentWhenNotNeeded(t *testing.T) {
	withUnreadableProcfs(t)
	if diags := checkTelemetryProcfs(diagnostic.DoctorCheckContext{Tree: prometheusTree(false)}); len(diags) != 0 {
		t.Fatalf("disabled: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkTelemetryProcfs(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkTelemetryProcfs(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}

	procfsReadFile = func(string) ([]byte, error) { return []byte("cpu  1 0 1 1 0 0 0 0 0 0\n"), nil }
	if diags := checkTelemetryProcfs(diagnostic.DoctorCheckContext{Tree: prometheusTree(true)}); len(diags) != 0 {
		t.Fatalf("readable procfs: diagnostics = %d, want 0", len(diags))
	}
}

// TestTelemetryProcfsDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this package's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestTelemetryProcfsDoctorCheckRegistered(t *testing.T) {
	want := telemetryProcfsDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeTelemetryProcfs) == nil {
		t.Fatalf("diagnostic code %q is not registered", codeTelemetryProcfs)
	}
}
