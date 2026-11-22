// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ntp.

package ntp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	exampleDoctorJSON = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeClockNoSync,
		Title:       "No clock synchronization configured",
		Description: "Ze NTP is disabled on a platform where clock synchronization must be provided by Ze or explicitly verified externally.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-clock-no-sync"},
	},
	{
		Code:        codeNTPClockPrivilege,
		Title:       "NTP clock adjustment privilege missing",
		Description: "NTP is configured but the process lacks CAP_SYS_TIME. Clock adjustment will fail unless running as root or with the capability granted.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ntp-clock-privilege"},
	},
	{
		Code:        codeNTPServerUnreachable,
		Title:       "NTP servers unreachable",
		Description: "None of the configured NTP servers responded to a reachability probe. Clock synchronization may fail.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-ntp-server-unreachable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: ntp: doctor code registration refused: " + err.Error())
		}
	}
}
