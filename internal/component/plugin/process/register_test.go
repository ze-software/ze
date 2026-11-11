package process

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// pluginsRow answers the status of the "plugins" health row this package
// registers from init().
func pluginsRow(t *testing.T) health.ComponentHealth {
	t.Helper()
	for _, row := range health.Check().Components {
		if row.Name == "plugins" {
			return row
		}
	}
	t.Fatal("no plugins health row is registered")
	return health.ComponentHealth{}
}

// TestPluginsHealthRowReadsRenamedCode proves the plugins health row reads the
// code the process manager raises when it disables a plugin, under its doctor
// name. Method: raise the warning the respawn limit raises (Respawn) and read
// the row; then raise the old spelling alone and read the row again, so a probe
// still filtering on "plugin-down" fails both halves.
//
// VALIDATES: AC-6 (spec-doctor-coverage-12-plugin-infra), R-3.
// PREVENTS: the rename leaving the row green while a plugin is disabled.
func TestPluginsHealthRowReadsRenamedCode(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	if row := pluginsRow(t); row.Status != health.StatusHealthy {
		t.Fatalf("plugins row on an empty bus = %s (%q), want healthy", row.Status, row.Reason)
	}

	report.RaiseWarning(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, "p1",
		"plugin disabled (respawn limit exceeded): p1", nil)
	row := pluginsRow(t)
	if row.Status != health.StatusDown {
		t.Fatalf("plugins row with %s active = %s, want down", diagnostic.CodeDoctorPluginStopped, row.Status)
	}
	if row.Reason != "plugin disabled (respawn limit exceeded): p1" {
		t.Errorf("plugins row reason = %q, want the warning's message", row.Reason)
	}

	report.ResetForTest()
	report.RaiseWarning(reportSourcePlugin, "plugin-down", "p1", "old spelling", nil)
	if row := pluginsRow(t); row.Status != health.StatusHealthy {
		t.Errorf("plugins row reads the retired code plugin-down: status %s", row.Status)
	}
}
