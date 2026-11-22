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
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: aaa: doctor code registration refused: " + err.Error())
		}
	}
}
