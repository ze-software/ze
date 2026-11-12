package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestAuditRecordFailedRow proves `show doctor` reports a failed audit write
// with its path, error, time and count, and a fix naming the directory, and
// that a log it cannot read is said so rather than read as clean. Method: the
// row builder over a failure; the check with no published log.
//
// VALIDATES: AC-33 (spec-doctor-coverage-12-plugin-infra).
// PREVENTS: an audit trail that stops landing on disk with nothing in doctor.
func TestAuditRecordFailedRow(t *testing.T) {
	failure := audit.WriteFailure{
		Path:     "/etc/ze/ze.audit.jsonl",
		Err:      "write audit log: no space left on device",
		Time:     time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC),
		Failures: 3,
		Surface:  audit.Web,
		Action:   audit.ActionConfigCommit,
	}
	row := auditRecordFailedRow(&failure)
	if row.Code != diagnostic.CodeDoctorAuditRecordFailed || row.Severity != diagnostic.SeverityError {
		t.Fatalf("row = %+v; want an error under doctor-audit-record-failed", row)
	}
	for _, want := range []string{"/etc/ze/ze.audit.jsonl", "no space left on device", "2026-11-11T10:00:00Z", "3 ", "web", "config-commit"} {
		if !strings.Contains(row.Message, want) {
			t.Errorf("message %q does not name %q", row.Message, want)
		}
	}
	if !strings.Contains(row.Fix, "/etc/ze") {
		t.Errorf("fix %q does not name the log's directory", row.Fix)
	}

	rows := checkAuditRecordFailed(diagnostic.DoctorCheckContext{})
	if len(rows) != 1 || rows[0].Code != diagnostic.CodeDoctorAuditRecordFailed || rows[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("check with no published log = %+v; want one warning saying it cannot tell", rows)
	}
}
