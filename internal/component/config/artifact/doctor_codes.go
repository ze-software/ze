// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/config/artifact.

package artifact

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorBackupWorkspaceStorage is the doctor diagnostic code "doctor-backup-workspace-storage".
	CodeDoctorBackupWorkspaceStorage = "doctor-backup-workspace-storage"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        CodeDoctorBackupWorkspaceStorage,
		Title:       "Backup workspace storage is unavailable",
		Description: "A private backup upload, export or cleanup could not finish. Live configuration was not changed. Check free space and temporary-directory permissions, close unused backup workspaces, and retry the upload. A cleanup failure may leave task-owned private files until the web service is stopped and its storage condition is repaired. Recent events retain no account, backup content or filename.",
		Examples:    []string{"ze show doctor", "ze show log recent level warn component web.server"},
	},
}

// registerDoctorCodes registers artifact's doctor diagnostic codes.
// Called by diagnostic.go's init before RegisterEventCode so registration
// order does not depend on file sorting.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: artifact: doctor code registration refused: " + err.Error())
		}
	}
}
