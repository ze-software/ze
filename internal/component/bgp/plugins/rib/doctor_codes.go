// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/rib.

package rib

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorBGPSelectedOutput = "doctor-bgp-selected-output"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorBGPSelectedOutput,
		Title: "BGP selected routes are unavailable",
		Description: "The deciding RIB could not record or send its selected routes. It refuses an incomplete snapshot rather than replaying old route data. " +
			"The runtime doctor finding names the original failure and the configured plugin instance. " +
			"Run the restart command in that finding to rebuild the selected state; use ze show errors to inspect the retained cause.",
		Examples: []string{"ze show doctor", "ze show errors", "ze show diagnostic doctor-bgp-selected-output"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: rib: doctor code registration refused: " + err.Error())
		}
	}
}
