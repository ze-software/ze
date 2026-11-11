package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestPluginSetupFailedReportsRecordedOutcome proves the setup check reads
// the outcome each plugin recorded: a hard failure is an error and a soft one
// a warning, each naming the plugin and the recorded reason, with the remedy
// the reason names as its fix; succeeded and unknown records give no row.
// Method: record into a registry snapshot restored on cleanup, then run the
// registered check itself.
func TestPluginSetupFailedReportsRecordedOutcome(t *testing.T) {
	saved := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(saved) })

	registry.RecordSetup("hard-plugin", registry.SetupFailedHard, "the device is absent: open /dev/x: no such file; load the x module with modprobe x")
	registry.RecordSetup("soft-plugin", registry.SetupFailedSoft, "the executable is not locked in memory")
	registry.RecordSetup("fine-plugin", registry.SetupSucceeded, "")

	rows := checkPluginSetup(diagnostic.DoctorCheckContext{})
	byPlugin := map[string]diagnostic.Diagnostic{}
	for _, row := range rows {
		for _, name := range []string{"hard-plugin", "soft-plugin", "fine-plugin"} {
			if strings.Contains(row.Message, "plugin "+name+" ") {
				byPlugin[name] = row
			}
		}
	}

	hard, ok := byPlugin["hard-plugin"]
	if !ok {
		t.Fatal("no row for the hard setup failure")
	}
	if hard.Code != diagnostic.CodeDoctorPluginSetupFailed || hard.Severity != diagnostic.SeverityError {
		t.Errorf("hard row %s/%s, want error %s", hard.Severity, hard.Code, diagnostic.CodeDoctorPluginSetupFailed)
	}
	if !strings.Contains(hard.Message, "open /dev/x: no such file") {
		t.Errorf("hard row %q does not carry the recorded reason", hard.Message)
	}
	if hard.Fix != "load the x module with modprobe x" {
		t.Errorf("hard row fix %q, want the remedy the reason names", hard.Fix)
	}

	soft, ok := byPlugin["soft-plugin"]
	if !ok {
		t.Fatal("no row for the soft setup failure")
	}
	if soft.Severity != diagnostic.SeverityWarning {
		t.Errorf("soft row severity %s, want warning", soft.Severity)
	}
	if !strings.Contains(soft.Message, "not locked in memory") {
		t.Errorf("soft row %q does not carry the recorded reason", soft.Message)
	}
	if soft.Fix != pluginSetupFixDefault {
		t.Errorf("soft row fix %q, want %q for a reason with no remedy", soft.Fix, pluginSetupFixDefault)
	}

	if _, ok := byPlugin["fine-plugin"]; ok {
		t.Error("a plugin recorded succeeded was reported")
	}
}
