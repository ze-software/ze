// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/radius.

package radius

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorCodeAdminEAPMismatch names the event raised when a concluding RADIUS
// Access-Accept or Access-Reject carries an encapsulated EAP packet that the
// local EAP peer rejected. Per RFC 3579 Section 2.6.3 the RADIUS packet code
// still decides the login.
const (
	doctorCodeAdminEAPMismatch = "doctor-radius-admin-eap-mismatch"
	exampleDoctorJSON          = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeAdminUnreachable,
		Title:       "RADIUS admin servers unreachable",
		Description: "No configured system/authentication/radius server answered an Access-Request probe, so operator logins via RADIUS may fall through to local or fail.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-radius-admin-unreachable"},
	},
	{
		Code:        doctorCodeAdminUnsignedResponse,
		Title:       "RADIUS admin server answers without Message-Authenticator",
		Description: "A system/authentication/radius server answered the probe Access-Request only without a Message-Authenticator. Ze discards such answers while the server's require-message-authenticator is true (BlastRADIUS, CVE-2024-3596), so operator logins via that server fail. Configure the server to sign its answers, or set require-message-authenticator false on that server row.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-radius-admin-unsigned-response"},
	},
	{
		Code:  doctorCodeAdminEAPMismatch,
		Title: "RADIUS admin concluding packet disagreed with encapsulated EAP",
		Description: "A RADIUS server sent a concluding Access-Accept or Access-Reject whose encapsulated " +
			"EAP-Message failed local EAP peer verification. Per RFC 3579 Section 2.6.3, Ze still " +
			"followed the RADIUS packet code for the operator login decision, but the disagreement " +
			"indicates a misconfigured or non-conforming EAP policy on the RADIUS server. Inspect the " +
			"RADIUS server logs and verify its EAP configuration with `ze show log recent level warn " +
			"component radius`.",
		Examples: []string{exampleDoctorJSON, "ze show errors", "ze show diagnostic doctor-radius-admin-eap-mismatch"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: radius: doctor code registration refused: " + err.Error())
		}
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: doctorCodeAdminEAPMismatch}); err != nil {
		panic("BUG: radius: EAP mismatch event code registration refused: " + err.Error())
	}
}
