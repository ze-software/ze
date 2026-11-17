// Design: docs/architecture/doctor-and-health-checks.md -- private backup storage failures.
package web

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorBackupWorkspaceStorage}); err != nil {
		panic("BUG: backup workspace event registration refused: " + err.Error())
	}
}

// reportBackupStorageFailure keeps this class visible in the existing runtime
// doctor recent-events check without retaining uploaded content or filenames.
// One fixed source and subject bound events across every workspace and owner.
func reportBackupStorageFailure() {
	const fix = "Run ze show doctor, check free space and temporary-directory permissions, then retry /backups/"
	const message = "A private backup workspace storage operation did not finish. Live configuration was not changed."
	report.RaiseErrorFixOnce("backup-workspace", diagnostic.CodeDoctorBackupWorkspaceStorage, "private-artifact",
		message, fix, time.Minute)
	serverLogger.Warn(message, "code", diagnostic.CodeDoctorBackupWorkspaceStorage, "fix", fix)
}
