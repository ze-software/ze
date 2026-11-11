package doctor

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// TestPluginFatalStopRowsNameRecord proves the fatal-stop check names the
// plugin, the reason and the time of each fatal or boot-refusal entry, skips
// the cleanup-late entries another check owns, and carries the drop count.
func TestPluginFatalStopRowsNameRecord(t *testing.T) {
	at := time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC)
	record := server.LastStop{
		Entries: []server.LastStopEntry{
			{Reason: server.LastStopFatalPolicy, Plugin: "fatal-one", Detail: "the plugin declares that its failure stops ze", Time: at},
			{Reason: server.LastStopCleanupLate, Plugin: "late", Detail: "cleanup ran late", Time: at},
			{Reason: server.LastStopBootRefused, Plugin: "boot", Detail: "stage 1 refused", Time: at},
		},
		Dropped: 2,
	}
	rows := fatalStopRows(&record)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3 (fatal, cleanup-late, boot refusal): %+v", len(rows), rows)
	}
	late := rows[1]
	if late.Code != diagnostic.CodeDoctorPluginStopCleanupLate || late.Severity != diagnostic.SeverityWarning {
		t.Errorf("cleanup-late row %s/%s", late.Severity, late.Code)
	}
	if !strings.Contains(late.Message, "late") || !strings.Contains(late.Message, "cleanup ran late") {
		t.Errorf("cleanup-late row %q lacks owner or detail", late.Message)
	}
	rows = []diagnostic.Diagnostic{rows[0], rows[2]}
	for _, want := range []string{"fatal-one", "failure policy", "its failure stops ze", "2026-11-11T09:30:00Z"} {
		if !strings.Contains(rows[0].Message, want) {
			t.Errorf("fatal row %q does not name %q", rows[0].Message, want)
		}
	}
	if rows[0].Code != diagnostic.CodeDoctorPluginFatalStop || rows[0].Severity != diagnostic.SeverityError {
		t.Errorf("fatal row %s/%s", rows[0].Severity, rows[0].Code)
	}
	if !strings.Contains(rows[0].Fix, "grep fatal-one") || !strings.Contains(rows[0].Fix, "configuration") {
		t.Errorf("fatal row fix %q names neither the plugin's log nor the config change", rows[0].Fix)
	}
	if !strings.Contains(rows[1].Message, "stage 1 refused") || !strings.Contains(rows[1].Message, "2 later stop reasons") {
		t.Errorf("boot row %q lacks its detail or the drop count", rows[1].Message)
	}
}

// TestPluginAutoloadRowCarriesRefusal proves the auto-load row carries the
// boot refusal's own text and the fix the refusal names.
func TestPluginAutoloadRowCarriesRefusal(t *testing.T) {
	refusal := opererr.New("plugin a needs plugin b, which this build does not carry",
		"a cannot run without b", "rebuild ze with the ze_b tag")
	err := errors.Join(errors.New("missing dependency"), refusal)
	row := autoloadRow(err)
	if row.Code != diagnostic.CodeDoctorPluginAutoloadUnresolved || row.Severity != diagnostic.SeverityError {
		t.Errorf("row %s/%s", row.Severity, row.Code)
	}
	if !strings.Contains(row.Message, err.Error()) {
		t.Errorf("row %q does not carry the refusal text %q", row.Message, err.Error())
	}
	if row.Fix != "rebuild ze with the ze_b tag" {
		t.Errorf("row fix %q, want the refusal's fix", row.Fix)
	}
}
