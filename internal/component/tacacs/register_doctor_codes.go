// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/tacacs.

package tacacs

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeTACACSUnreachable,
		Title:       "TACACS+ servers unreachable",
		Description: "No configured TACACS+ authentication server accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-tacacs-unreachable"},
	},
	{
		Code:        codeTACACSWeakSecret,
		Title:       "TACACS+ shared secret is short",
		Description: "A configured TACACS+ shared secret is shorter than the 16 characters recommended by RFC 8907 Section 10.5.1. Use a unique secret of at least 16 characters, preferably 32 or more, and rotate it on both ends.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-tacacs-weak-secret"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: tacacs: doctor code registration refused: " + err.Error())
		}
	}
}
