package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestPluginStoppedEveryDownPath proves the plugin-stopped check reports
// every down path with the plugin, the cause and the restart command, and
// that each path's record raises doctor-plugin-stopped on the bus (what the
// plugins health row reads). Method: one ProcessManager record per cause,
// read back into the snapshot shape the check reads; then the rows a removed,
// a running and an unconfigured plugin must not produce.
func TestPluginStoppedEveryDownPath(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	at := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	paths := []struct {
		name string
		stop process.PluginStop
		want []string
	}{
		{"limit", process.PluginStop{Cause: process.StopRespawnLimit, Detail: "it was started 5 times within 1m0s"}, []string{"respawn limit", "5 times"}},
		{"ignore", process.PluginStop{Cause: process.StopPolicyExit, Policy: "ignore", ExitStatus: process.ExitedWith("exit code 1")}, []string{"failure policy ignore", "exit code 1"}},
		{"unspecified", process.PluginStop{Cause: process.StopPolicyExit, Policy: "unspecified", ExitStatus: process.ExitedWith("signal: killed")}, []string{"failure policy unspecified", "signal: killed"}},
		{"restart", process.PluginStop{Cause: process.StopRestartFailed, Detail: "restart handshake restart: stage 1 timed out"}, []string{"could not be started again", "stage 1 timed out"}},
		{"handshake", process.PluginStop{Cause: process.StopHandshakeFailed, Detail: "stage 2 (configure): refused"}, []string{"startup handshake failed", "stage 2"}},
	}

	configs := make([]plugin.PluginConfig, 0, len(paths))
	for _, p := range paths {
		configs = append(configs, plugin.PluginConfig{Name: p.name, Internal: true})
	}
	pm := process.NewProcessManager(configs)

	snapshot := server.DoctorSnapshot{}
	for _, p := range paths {
		p.stop.Time = at
		pm.RecordStopped(p.name, p.stop)
		stop, down := pm.Stopped(p.name)
		snapshot.Plugins = append(snapshot.Plugins, server.PluginDoctorState{Name: p.name, Configured: true, Down: down, Stop: stop})
	}
	// Removed by config: its record was cleared, so it is not down.
	snapshot.Plugins = append(snapshot.Plugins,
		server.PluginDoctorState{Name: "removed", Configured: true},
		server.PluginDoctorState{Name: "running", Configured: true, Running: true, Down: true, Stop: process.PluginStop{Cause: process.StopPolicyExit}},
		server.PluginDoctorState{Name: "unconfigured", Down: true, Stop: process.PluginStop{Cause: process.StopPolicyExit}},
	)

	rows := pluginStoppedRows(&snapshot)
	if len(rows) != len(paths) {
		t.Fatalf("rows = %d, want one per down path (%d): %+v", len(rows), len(paths), rows)
	}
	for i, p := range paths {
		row := rows[i]
		if row.Code != diagnostic.CodeDoctorPluginStopped || row.Severity != diagnostic.SeverityError {
			t.Errorf("%s: row %s/%s, want error %s", p.name, row.Severity, row.Code, diagnostic.CodeDoctorPluginStopped)
		}
		for _, want := range append([]string{"plugin " + p.name + " ", "2026-11-11T12:00:00Z"}, p.want...) {
			if !strings.Contains(row.Message, want) {
				t.Errorf("%s: message %q does not name %q", p.name, row.Message, want)
			}
		}
		if row.Fix != "ze request plugin restart "+p.name {
			t.Errorf("%s: fix %q, want the restart command", p.name, row.Fix)
		}
		found := false
		for _, w := range report.Warnings() {
			if w.Code == diagnostic.CodeDoctorPluginStopped && w.Subject == p.name {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no doctor-plugin-stopped warning on the bus, so the plugins row would read healthy", p.name)
		}
	}
}

// TestPluginStoppedRowWhileCleanupOwed reads the row of a plugin whose failed
// restart left a stopped process owing its cleanup.
//
// Goal: while the restart command refuses the plugin, its row says only what
// is known (the stopped process's cleanup had not finished within the bound,
// whether it waits on a call or is still running), says ze refuses to start
// the plugin until it has, keeps the restart command alone as the fix, and
// names restarting the ze daemon as the last resort, for what it is.
// Method: the same down plugin with and without CleanupOwed in the snapshot.
// PREVENTS: a row claiming a call has not returned when the cleanup may only
// be slow, a fix line carrying prose after the command, and an operator left
// with no way out, or told `ze signal restart` brings ze back on its own.
func TestPluginStoppedRowWhileCleanupOwed(t *testing.T) {
	stop := process.PluginStop{Cause: process.StopRestartFailed, Detail: "the stopped process has not finished its cleanup"}
	owed := pluginStoppedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
		{Name: "stuck", Configured: true, Down: true, Stop: stop, CleanupOwed: true},
	}})
	if len(owed) != 1 {
		t.Fatalf("rows = %d, want one: %+v", len(owed), owed)
	}
	for _, want := range []string{
		"refuses to start the plugin",
		"had not finished within the restart's bound",
		"or still be running",
		daemonRestartLastResort,
	} {
		if !strings.Contains(owed[0].Message, want) {
			t.Errorf("message %q does not say %q", owed[0].Message, want)
		}
	}
	if strings.Contains(owed[0].Message, "has not returned") {
		t.Errorf("message %q claims a call has not returned, which ze does not know", owed[0].Message)
	}
	if owed[0].Fix != process.RestartCommand("stuck") {
		t.Errorf("fix %q, want the restart command alone, %q", owed[0].Fix, process.RestartCommand("stuck"))
	}

	settled := pluginStoppedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
		{Name: "stuck", Configured: true, Down: true, Stop: stop},
	}})
	if len(settled) != 1 || settled[0].Fix != process.RestartCommand("stuck") {
		t.Fatalf("once the cleanup ran the fix is not the restart command alone: %+v", settled)
	}
	if strings.Contains(settled[0].Message, "refuses to start the plugin") {
		t.Errorf("once the cleanup ran the row still says ze refuses the plugin: %q", settled[0].Message)
	}
}
