// Design: docs/architecture/doctor-and-health-checks.md -- private backup storage failures.
package artifact

import (
	"github.com/ze-software/ze/internal/core/slogutil"
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

var artifactLog = slogutil.Logger("config.artifact")

func init() {
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: diagnostic.CodeDoctorBackupWorkspaceStorage}); err != nil {
		panic("BUG: backup workspace event registration refused: " + err.Error())
	}
}

// ReportStorageFailure keeps this class visible in the existing runtime
// doctor recent-events check without retaining uploaded content or filenames.
// One fixed source and subject bound events across every workspace and owner.
func ReportStorageFailure() {
	const fix = "Run ze show doctor, check free space and temporary-directory permissions, then retry /backups/"
	const message = "A private backup workspace storage operation did not finish. Live configuration was not changed."
	report.RaiseErrorFixOnce("backup-workspace", diagnostic.CodeDoctorBackupWorkspaceStorage, "private-artifact",
		message, fix, time.Minute)
	artifactLog.Warn(message, "code", diagnostic.CodeDoctorBackupWorkspaceStorage, "fix", fix)
}
