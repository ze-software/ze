// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/cli.

package cli

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorCLIHistoryRead = "doctor-cli-history-read"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorCLIHistoryRead,
		Title:       "Command recall could not be read",
		Description: "Saved command recall could not be read, so it was not returned or overwritten. Missing history is not an error. One recent event per minute covers all users and modes without recording commands, usernames or storage error values. Run ze show doctor to diagnose storage readiness, then reconnect after restoring storage access.",
		Examples:    []string{"ze show doctor"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: cli: doctor code registration refused: " + err.Error())
		}
	}
}
