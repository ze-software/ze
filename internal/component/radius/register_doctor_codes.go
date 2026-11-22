// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/radius.

package radius

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeAdminUnreachable,
		Title:       "RADIUS admin servers unreachable",
		Description: "No configured system/authentication/radius server answered an Access-Request probe, so operator logins via RADIUS may fall through to local or fail.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-radius-admin-unreachable"},
	},
	{
		Code:        doctorCodeAdminUnsignedResponse,
		Title:       "RADIUS admin server answers without Message-Authenticator",
		Description: "A system/authentication/radius server answered the probe Access-Request only without a Message-Authenticator. Ze discards such answers while the server's require-message-authenticator is true (BlastRADIUS, CVE-2024-3596), so operator logins via that server fail. Configure the server to sign its answers, or set require-message-authenticator false on that server row.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-radius-admin-unsigned-response"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: radius: doctor code registration refused: " + err.Error())
		}
	}
}
