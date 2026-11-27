// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/aaa.

package aaa

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCodeNoLocalFallback,
		Title:       "No local account to fall back to",
		Description: "A remote AAA backend (RADIUS or TACACS+) is configured and no system/authentication/user is. The chain asks the remote backend first and reaches the local one only when the remote fails to ANSWER, so that fallback is an account: with none declared, an unreachable server or a backend whose config will not build leaves the box with no login at all.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-aaa-no-local-fallback"},
	},
	{
		Code:        CodeDoctorAAAAllAccountsExcluded,
		Title:       "All configured accounts excluded from every login service",
		Description: "Every configured local user and remote AAA mapping references a system/authorization/login-service set that permits none of the login services registered in this build, so no configured operator can log in. Only an unshadowed stored recovery account or the appliance serial console remains reachable. Add a registered service to a login-service set (set system authorization login-service <set> service <service>) or remove the login-service reference from an administrative account.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-aaa-all-accounts-excluded"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: aaa: doctor code registration refused: " + err.Error())
		}
	}
}
