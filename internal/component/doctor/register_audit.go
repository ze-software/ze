// Design: docs/architecture/doctor-and-health-checks.md -- the audit log's failed writes
// Related: checks_audit.go -- the check this file registers

package doctor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The audit log is core, so the check that reports its failed writes is
// doctor's own, registered apart from register.go beside its problem class.
func init() {
	if err := diagnostic.RegisterDoctorCheck(auditRecordCheck); err != nil {
		panic("BUG: audit-record doctor check registration refused: " + err.Error())
	}
}
