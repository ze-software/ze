// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/policyroute.

package policyroute

import "github.com/ze-software/ze/internal/core/diagnostic"

// codePolicyRouteNetlink names a route netlink handle this process cannot
// open. internal/core/diagnostic/codes.go declares it, so `ze show diagnostic
// doctor-policyroute-netlink` answers.
const codePolicyRouteNetlink = "doctor-policyroute-netlink"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codePolicyRouteNetlink,
		Title:       "Policy route netlink socket unavailable",
		Description: "Policy routing is configured but a route netlink handle could not be opened.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-policyroute-netlink"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: policyroute: doctor code registration refused: " + err.Error())
		}
	}
}
