// Design: docs/architecture/doctor-and-health-checks.md -- the audit log's failed writes
// Related: register_audit.go -- registers the check at the runtime phase
// Related: ../../core/audit/write_outcome.go -- PublishedWriteFailure, the state it reads

package doctor

import (
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/audit"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The audit check's declaration values.
const (
	auditRecordCheckName  = "audit-record"
	auditRecordDependency = "audit-log"
	auditRecordOrder      = 2910
)

// auditRecordCheck reports a failed audit write. The audit log is core and
// owns no component, so its check lives here beside env-values.
var auditRecordCheck = diagnostic.DoctorCheck{
	Name:         auditRecordCheckName,
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        auditRecordOrder,
	Component:    doctorOwnComponent,
	Dependencies: []string{auditRecordDependency},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorAuditRecordFailed},
	Check:        checkAuditRecordFailed,
}

// checkAuditRecordFailed answers one error when the daemon's audit log's
// last write failed. Every surface writes through the same log, so this one
// row covers CLI, SSH, web, REST, gRPC and MCP. A log it cannot read is
// reported as a warning that says so, never as no row.
func checkAuditRecordFailed(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	failure, failed, err := audit.PublishedWriteFailure()
	if err != nil {
		var message textbuf.Buffer
		message.Str("cannot tell whether audit records reach disk: ").Err(err)
		return []diagnostic.Diagnostic{{
			Code:     diagnostic.CodeDoctorAuditRecordFailed,
			Severity: diagnostic.SeverityWarning,
			Message:  message.String(),
			Fix:      "ze show audit",
		}}
	}
	if !failed {
		return nil
	}
	return []diagnostic.Diagnostic{auditRecordFailedRow(&failure)}
}

// auditRecordFailedRow writes the failure: which file, why, when, how many
// writes failed and the last entry's surface and action. The fix names the
// directory to make writable or to free space in; the next write that lands
// persists every entry the log still holds.
func auditRecordFailedRow(failure *audit.WriteFailure) diagnostic.Diagnostic {
	dir := filepath.Dir(failure.Path)
	var message, fix textbuf.Buffer
	message.Str("the audit log ").Str(failure.Path).Str(" could not be written: ").Str(failure.Err).
		Str("; ").Int(int64(failure.Failures)).Str(" writes failed, the last at ").
		Str(failure.Time.UTC().Format(time.RFC3339)).Str(" (").Str(failure.Surface).Str(" ").
		Str(failure.Action).Str("), so those records exist in memory only")
	fix.Str("ls -ld ").Str(dir).Str(" && df -h ").Str(dir).
		Str(", then make it writable by ze with free space; the next audited action rewrites the log")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorAuditRecordFailed,
		Severity: diagnostic.SeverityError,
		Message:  message.String(),
		Fix:      fix.String(),
	}
}
