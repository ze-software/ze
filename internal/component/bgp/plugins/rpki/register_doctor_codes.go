// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/rpki.

package rpki

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeRPKIUnreachable,
		Title:       "RPKI cache servers unreachable",
		Description: "None of the configured RPKI cache servers accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-rpki-unreachable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: rpki: doctor code registration refused: " + err.Error())
		}
	}
}
