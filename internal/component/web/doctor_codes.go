// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/web.

package web

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorWebHistoryUnavailable = "doctor-web-history-unavailable"
	codeDoctorWebHistoryResponse    = "doctor-web-history-response"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorWebHistoryUnavailable,
		Title:       "Browser command recall unavailable",
		Description: "A browser submission could not be recorded for recall, including an invalid submission identifier. Command execution remains available. One event per minute covers all requests without retaining commands, usernames or error values. Open /cli and reconnect to restore command recall.",
		Examples:    []string{"ze show doctor"},
	},
	{
		Code:        codeDoctorWebHistoryResponse,
		Title:       "Browser command recall response failed",
		Description: "Saved command recall was read but its JSON response could not be delivered to the browser. Saved recall is unchanged. One event per minute covers all requests without retaining commands, usernames or error values. Open /cli and reconnect to restore command recall.",
		Examples:    []string{"ze show doctor"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: web: doctor code registration refused: " + err.Error())
		}
	}
}
