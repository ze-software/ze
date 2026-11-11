package doctor

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// TestReportBusFullRow proves `ze doctor` names a report bus that dropped
// entries, with the cap, the count lost and the export that raises the cap.
// Method: a bus below its caps answers no row; one warning past the default
// warning cap and one error past the default error ring answer one row each,
// and the check is registered at the runtime phase declaring its code.
//
// VALIDATES: AC-7 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: the bus dropping what show warnings and the health rows read,
// with doctor clean.
func TestReportBusFullRow(t *testing.T) {
	if report.CodeReportBusFull != diagnostic.CodeDoctorReportBusFull {
		t.Fatalf("report.CodeReportBusFull %q differs from diagnostic.CodeDoctorReportBusFull %q",
			report.CodeReportBusFull, diagnostic.CodeDoctorReportBusFull)
	}

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	if rows := checkReportBusFull(diagnostic.DoctorCheckContext{}); len(rows) != 0 {
		t.Fatalf("a bus below its caps answered %d rows, want none: %+v", len(rows), rows)
	}

	loss := report.Losses()
	for i := range loss.WarningCap + 1 {
		report.RaiseWarning("test", "flood", "s"+strconv.Itoa(i), "m", nil)
	}
	for range loss.ErrorCap + 1 {
		report.RaiseError("test", "flood", "s", "m", nil)
	}

	rows := checkReportBusFull(diagnostic.DoctorCheckContext{})
	if len(rows) != 2 {
		t.Fatalf("a bus past both caps answered %d rows, want 2: %+v", len(rows), rows)
	}
	wantParts := [][]string{
		{"1 ", strconv.Itoa(loss.WarningCap), "ze.report.warnings.max"},
		{"1 ", strconv.Itoa(loss.ErrorCap), "ze.report.errors.max"},
	}
	for i, row := range rows {
		if row.Code != diagnostic.CodeDoctorReportBusFull {
			t.Errorf("row %d code = %q, want %q", i, row.Code, diagnostic.CodeDoctorReportBusFull)
		}
		if row.Severity != diagnostic.SeverityWarning {
			t.Errorf("row %d severity = %q, want warning", i, row.Severity)
		}
		for _, want := range wantParts[i] {
			if !strings.Contains(row.Message, want) {
				t.Errorf("row %d message %q does not name %q", i, row.Message, want)
			}
		}
		if row.Fix == "" {
			t.Errorf("row %d has no fix", i)
		}
	}
	if !strings.Contains(rows[0].Fix, "ZE_REPORT_WARNINGS_MAX=") {
		t.Errorf("warning row fix %q does not export the warning cap", rows[0].Fix)
	}
	if !strings.Contains(rows[1].Fix, "ZE_REPORT_ERRORS_MAX=") {
		t.Errorf("error row fix %q does not export the error cap", rows[1].Fix)
	}

	found := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name == reportBusCheckName {
			found = slices.Contains(check.Codes, diagnostic.CodeDoctorReportBusFull)
		}
	}
	if !found {
		t.Errorf("no runtime check %q declares %s", reportBusCheckName, diagnostic.CodeDoctorReportBusFull)
	}
}
