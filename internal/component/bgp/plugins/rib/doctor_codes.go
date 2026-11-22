// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/rib.

package rib

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorBGPSelectedOutput      = "doctor-bgp-selected-output"
	codeDoctorBGPNextHopUnresolvable = "doctor-bgp-next-hop-unresolvable"
	// codeDoctorBGPNextHopMetrics aliases the doctor-bgp-next-hop-metrics code
	// already registered alongside its event code in register.go so it is never
	// registered twice at init.
	codeDoctorBGPNextHopMetrics = codeRemoteMetricFailure
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
	{
		Code:  codeDoctorBGPNextHopUnresolvable,
		Title: "BGP routes excluded by unresolvable next hop",
		Description: "One or more BGP routes in Adj-RIB-In carry a NEXT_HOP that cannot be resolved in the Loc-RIB " +
			"(or would become unresolvable if installed). Per RFC 4271 Section 9.1.2, these routes are excluded from " +
			"Phase 2 best-path selection while remaining retained in Adj-RIB-In so they enter selection automatically " +
			"once a covering route resolves the next hop. Inspect excluded routes with " +
			"ze show bgp rib | match next-hop-unresolvable and check interior reachability with ze show rib.",
		Examples: []string{
			unresolvedNextHopDoctorFix,
			"ze show rib",
			"ze show diagnostic doctor-bgp-next-hop-unresolvable",
		},
		RelatedCodes: []string{codeDoctorBGPNextHopMetrics},
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
