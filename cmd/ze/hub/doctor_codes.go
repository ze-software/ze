// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by cmd/ze/hub.

package hub

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorLocalAdminCredentials = "doctor-local-admin-credentials"
	codeDoctorWebNetworkCleanup     = "doctor-web-network-cleanup"
	codeDoctorWebCleanupOverdue     = "doctor-web-cleanup-overdue"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorLocalAdminCredentials,
		Title: "Local recovery credentials cannot be loaded",
		Description: "The stored local recovery account could not be read or validated, so it cannot log in. Configured operator accounts remain available. " +
			"From the console or another working login, stop Ze, restore the affected meta/auth/local credential frame from a known backup, and restart Ze.",
		Examples:     []string{"ze doctor --json", "ze show doctor", "ze data --path <database> write <credential-key> <credential-backup-file>"},
		RelatedCodes: []string{diagnostic.CodeDoctorStoreIntegrity},
	},
	{
		Code:        codeDoctorWebNetworkCleanup,
		Title:       "Web network cleanup failed",
		Description: "Web network shutdown reported a failure after joining admitted handlers. History cleanup still runs and the caller retains its strict cleanup join. One recent event per minute covers repeated shutdown callers. Inspect recent web.server errors to locate the cause.",
		Examples:    []string{"ze show log recent level err component web.server"},
	},
	{
		Code:        codeDoctorWebCleanupOverdue,
		Title:       "Web cleanup is still waiting",
		Description: "An admitted web request, an owned route resource or an accepted command-history publication did not finish within the cleanup allowance. The service keeps joining it and retains the borrowed configuration store. Inspect recent web.server errors to locate the blocked work. One recent event per minute covers concurrent shutdown callers without recording commands or user data.",
		Examples:    []string{"ze show log recent level err component web.server"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: hub: doctor code registration refused: " + err.Error())
		}
	}
}
