package process

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// activeWarning reports whether the bus holds the active warning (source,
// code, subject).
func activeWarning(source, code, subject string) bool {
	for _, w := range report.Warnings() {
		if w.Source == source && w.Code == code && w.Subject == subject {
			return true
		}
	}
	return false
}

// TestStoppedRecordRaisesAndClears proves a stop record is what the plugins
// health row reads: RecordStopped raises doctor-plugin-stopped with the
// restart command as its fix, ClearStopped (a start that reached running) and
// RemoveProcess (a removal by config) each clear the record and the warning.
func TestStoppedRecordRaisesAndClears(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "p1", Internal: true}})
	pm.RecordStopped("p1", PluginStop{Cause: StopPolicyExit, Policy: "ignore", ExitStatus: "exit code 3"})

	stop, down := pm.Stopped("p1")
	if !down {
		t.Fatal("Stopped(p1) = not down after RecordStopped")
	}
	if stop.Time.IsZero() {
		t.Error("stop record carries no time")
	}
	if !activeWarning(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, "p1") {
		t.Fatal("RecordStopped raised no doctor-plugin-stopped warning")
	}
	var fix string
	for _, w := range report.Warnings() {
		if w.Code == diagnostic.CodeDoctorPluginStopped {
			fix = w.Fix
			for _, want := range []string{"p1", "ignore", "exit code 3"} {
				if !strings.Contains(w.Message, want) {
					t.Errorf("warning %q does not name %q", w.Message, want)
				}
			}
		}
	}
	if fix != RestartCommand("p1") {
		t.Errorf("warning fix = %q, want %q", fix, RestartCommand("p1"))
	}

	pm.ClearStopped("p1")
	if _, down := pm.Stopped("p1"); down {
		t.Error("Stopped(p1) still down after ClearStopped")
	}
	if activeWarning(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, "p1") {
		t.Error("ClearStopped left the doctor-plugin-stopped warning")
	}

	pm.RecordStopped("p1", PluginStop{Cause: StopHandshakeFailed, Detail: "stage 2 timed out"})
	pm.RemoveProcess("p1")
	if _, down := pm.Stopped("p1"); down {
		t.Error("a removed plugin is still recorded down")
	}
	if activeWarning(reportSourcePlugin, diagnostic.CodeDoctorPluginStopped, "p1") {
		t.Error("RemoveProcess left the doctor-plugin-stopped warning of a removed plugin")
	}
}

// TestPluginExitClearsItsOwnWarnings proves a warning a plugin raised itself
// (report-issue raises under the plugin's name as source) does not outlive
// the plugin: the crash report clears the plugin's own source and raises
// doctor-plugin-crashed with the exit status and the policy, while another
// source's warning about the same plugin stays.
func TestPluginExitClearsItsOwnWarnings(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("p1", diagnostic.CodeDoctorInternalDefect, "state", "plugin-raised warning")
	report.RaiseWarning("other", diagnostic.CodeDoctorInternalDefect, "p1", "someone else's warning")

	pm := NewProcessManager([]plugin.PluginConfig{{Name: "p1", Internal: true}})
	pm.ReportCrash("p1", "exit code 2", "ignore")

	if activeWarning("p1", diagnostic.CodeDoctorInternalDefect, "state") {
		t.Error("the plugin's own warning outlived its process")
	}
	if !activeWarning("other", diagnostic.CodeDoctorInternalDefect, "p1") {
		t.Error("ReportCrash cleared a warning another source raised")
	}
	var crashed *report.Issue
	errs := report.Errors(0)
	for i := range errs {
		if errs[i].Code == diagnostic.CodeDoctorPluginCrashed && errs[i].Subject == "p1" {
			crashed = &errs[i]
		}
	}
	if crashed == nil {
		t.Fatal("no doctor-plugin-crashed event")
	}
	for _, want := range []string{"exit code 2", "ignore"} {
		if !strings.Contains(crashed.Message, want) {
			t.Errorf("crash event %q does not name %q", crashed.Message, want)
		}
	}
	if crashed.Fix != "ze show errors" {
		t.Errorf("crash event fix = %q, want ze show errors", crashed.Fix)
	}
}

// TestExitRecordFirstStatusStands proves the exit record keeps the status of
// the exit that ended the process, and that a wait with no exit recorded
// answers no status rather than an empty one.
func TestExitRecordFirstStatusStands(t *testing.T) {
	var r exitRecord
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, ok := r.wait(ctx); ok {
		t.Fatal("wait answered a status before any was recorded")
	}

	r.record("exit code 1")
	r.record("a panic in the plugin; its crash record names the cause")
	status, ok := r.wait(context.Background())
	if !ok {
		t.Fatal("wait answered no status after record")
	}
	if status != "exit code 1" {
		t.Errorf("status = %q, want the first recorded exit code 1", status)
	}
}
