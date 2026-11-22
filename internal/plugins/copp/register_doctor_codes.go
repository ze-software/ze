// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/copp.

package copp

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorCoppCode,
		Title:       "CoPP input chain not active",
		Description: "Control-plane policing (CoPP) for BGP is configured but the nftables input chain protecting TCP/179 may not be installed. Verify the firewall backend is running and the copp plugin started successfully.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-copp-missing"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: copp: doctor code registration refused: " + err.Error())
		}
	}
}
