// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/web.

package web

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorWebHistoryUnavailable       = "doctor-web-history-unavailable"
	codeDoctorWebHistoryResponse          = "doctor-web-history-response"
	codeDoctorWebEditorEvictDiscardFailed = "doctor-web-editor-evict-discard-failed"

	webShowDoctorCommand = "ze show doctor"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorWebHistoryUnavailable,
		Title:       "Browser command recall unavailable",
		Description: "A browser submission could not be recorded for recall, including an invalid submission identifier. Command execution remains available. One event per minute covers all requests without retaining commands, usernames or error values. Open /cli and reconnect to restore command recall.",
		Examples:    []string{webShowDoctorCommand},
	},
	{
		Code:        codeDoctorWebHistoryResponse,
		Title:       "Browser command recall response failed",
		Description: "Saved command recall was read but its JSON response could not be delivered to the browser. Saved recall is unchanged. One event per minute covers all requests without retaining commands, usernames or error values. Open /cli and reconnect to restore command recall.",
		Examples:    []string{webShowDoctorCommand},
	},
	{
		Code:  codeDoctorWebEditorEvictDiscardFailed,
		Title: "Idle web editor discard failed during eviction",
		Description: "An idle web configuration editor session reached eviction at capacity and its Discard call returned an error. " +
			"The session slot was still released so other users can open the editor, and persisted drafts remain unchanged. " +
			"At most one report-bus event is emitted per minute without retaining config text or secrets. " +
			"Inspect daemon health and recent events with ze show doctor.",
		Examples: []string{webShowDoctorCommand, "ze show diagnostic doctor-web-editor-evict-discard-failed"},
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
