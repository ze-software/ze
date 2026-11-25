package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
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
	for _, membership := range []privilege.ServiceMembership{privilege.MembershipZeService, privilege.MembershipOther} {
		t.Run(membership.String(), func(t *testing.T) {
			t.Cleanup(privilege.SetServiceMembershipForTest(membership, nil))
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
				process.DaemonRestartLastResort(),
			} {
				if !strings.Contains(owed[0].Message, want) {
					t.Errorf("message %q does not say %q", owed[0].Message, want)
				}
			}
			if membership != privilege.MembershipZeService && strings.Contains(owed[0].Message, "systemctl") {
				t.Errorf("message %q offers systemctl outside ze.service", owed[0].Message)
			}
			if strings.Contains(owed[0].Message, "has not returned") {
				t.Errorf("message %q claims a call has not returned, which ze does not know", owed[0].Message)
			}
			if owed[0].Fix != process.RestartCommand("stuck") {
				t.Errorf("fix %q, want the restart command alone, %q", owed[0].Fix, process.RestartCommand("stuck"))
			}
		})
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

// TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed verifies that the
// doctor-plugin-stopped report-bus warning and ze show doctor's plugin-stopped
// row render the exact same message and command-only fix from the shared
// process.StoppedMessage/StoppedFix declaration while cleanup is owed, and
// that clearing CleanupOwed refreshes the bus warning to match the settled row
// while keeping the warning active.
//
// VALIDATES: spec-plugin-restart-bounded-and-configuration-true AC-6 --
// PluginStop.CleanupOwed, process.StoppedMessage, process.StoppedFix,
// ProcessManager.SetStoppedCleanupOwed, and ProcessManager.ClearStoppedCleanupOwed.
// PREVENTS: ze show warnings and daemon logs omitting the cleanup-owed refusal
// and daemon-restart last resort that ze show doctor displays, or leaving the
// bus warning stale once the owed cleanup completes.
func TestStoppedBusWarningMatchesDoctorRowWhileCleanupOwed(t *testing.T) {
	at := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	const refusalPhrase = "refuses to start the plugin"
	findStoppedWarning := func(t *testing.T, name string) report.Issue {
		t.Helper()
		for _, w := range report.Warnings() {
			if w.Code == diagnostic.CodeDoctorPluginStopped && w.Subject == name {
				return w
			}
		}
		t.Fatalf("no active %s warning on the report bus for %s", diagnostic.CodeDoctorPluginStopped, name)
		return report.Issue{}
	}

	for _, membership := range []privilege.ServiceMembership{privilege.MembershipZeService, privilege.MembershipOther} {
		t.Run(membership.String(), func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			t.Cleanup(privilege.SetServiceMembershipForTest(membership, nil))

			const name = "stuck-shared"
			pm := process.NewProcessManager([]plugin.PluginConfig{{Name: name, Internal: true}})
			pm.RecordStopped(name, process.PluginStop{
				Cause:       process.StopRestartFailed,
				Detail:      "a stopped process of the plugin has not finished its cleanup",
				Time:        at,
				CleanupOwed: true,
			})

			stop, down := pm.Stopped(name)
			if !down || !stop.CleanupOwed {
				t.Fatalf("Stopped(%s) = down:%v CleanupOwed:%v, want true/true", name, down, stop.CleanupOwed)
			}
			rows := pluginStoppedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
				{Name: name, Configured: true, Down: down, Stop: stop, CleanupOwed: stop.CleanupOwed},
			}})
			if len(rows) != 1 {
				t.Fatalf("rows = %d, want 1: %+v", len(rows), rows)
			}
			busWarn := findStoppedWarning(t, name)
			if busWarn.Message != rows[0].Message {
				t.Errorf("bus warning message != doctor row message:\n bus: %q\n row: %q", busWarn.Message, rows[0].Message)
			}
			for _, want := range []string{
				refusalPhrase,
				"had not finished within the restart's bound",
				"or still be running",
				privilege.RestartRemedy(),
				process.DaemonRestartLastResort(),
			} {
				if !strings.Contains(busWarn.Message, want) {
					t.Errorf("bus warning %q missing %q", busWarn.Message, want)
				}
			}
			if busWarn.Fix != rows[0].Fix || busWarn.Fix != process.RestartCommand(name) {
				t.Errorf("fixes diverged: bus=%q row=%q want=%q", busWarn.Fix, rows[0].Fix, process.RestartCommand(name))
			}

			// Clearing CleanupOwed refreshes the bus warning without clearing
			// it (the plugin is still down) and keeps bus and doctor identical.
			pm.ClearStoppedCleanupOwed(name)
			settledStop, stillDown := pm.Stopped(name)
			if !stillDown || settledStop.CleanupOwed {
				t.Fatalf("after ClearStoppedCleanupOwed: down=%v CleanupOwed=%v, want true/false", stillDown, settledStop.CleanupOwed)
			}
			settledRows := pluginStoppedRows(&server.DoctorSnapshot{Plugins: []server.PluginDoctorState{
				{Name: name, Configured: true, Down: stillDown, Stop: settledStop},
			}})
			if len(settledRows) != 1 {
				t.Fatalf("settled rows = %d, want 1: %+v", len(settledRows), settledRows)
			}
			refreshedWarn := findStoppedWarning(t, name)
			if refreshedWarn.Message != settledRows[0].Message {
				t.Errorf("refreshed bus warning != settled doctor row:\n bus: %q\n row: %q", refreshedWarn.Message, settledRows[0].Message)
			}
			if strings.Contains(refreshedWarn.Message, refusalPhrase) {
				t.Errorf("refreshed bus warning still carries cleanup-owed refusal: %q", refreshedWarn.Message)
			}
			if strings.Contains(refreshedWarn.Message, process.DaemonRestartLastResort()) {
				t.Errorf("refreshed bus warning still carries daemon-restart last resort: %q", refreshedWarn.Message)
			}
			if refreshedWarn.Fix != settledRows[0].Fix || refreshedWarn.Fix != process.RestartCommand(name) {
				t.Errorf("refreshed fixes diverged: bus=%q row=%q want=%q", refreshedWarn.Fix, settledRows[0].Fix, process.RestartCommand(name))
			}

			// SetStoppedCleanupOwed upgrades an existing stop record in place
			// (e.g. a preserved StopRespawnLimit hit by a cleanup-owed refusal).
			pm.SetStoppedCleanupOwed(name)
			upgradedStop, upgradedDown := pm.Stopped(name)
			if !upgradedDown || !upgradedStop.CleanupOwed {
				t.Fatalf("after SetStoppedCleanupOwed: down=%v CleanupOwed=%v, want true/true", upgradedDown, upgradedStop.CleanupOwed)
			}
			upgradedWarn := findStoppedWarning(t, name)
			if upgradedWarn.Message != busWarn.Message {
				t.Errorf("upgraded bus warning != initial cleanup-owed message:\n got:  %q\n want: %q", upgradedWarn.Message, busWarn.Message)
			}
		})
	}
}
