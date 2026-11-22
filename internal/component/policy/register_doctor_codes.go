// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/policy.

package policy

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPolicyIGPImportHidesNextHop = "doctor-policy-igp-import-hides-nexthop"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorPolicyIGPImportHidesNextHop,
		Title:       "IGP import filter hides the route a next hop resolves through",
		Description: "An IS-IS or OSPF import chain (or the central `rib filter import` chain) hid a prefix the IGP learned, and that prefix is the most specific cover of the next hop of a route this router selected. The IGP still floods the prefix, so routers downstream keep sending its traffic here, while this router resolves the next hop through a shorter prefix or not at all: transit traffic is blackholed or loops. The check reads the running daemon's Loc-RIB, so only `show doctor` inside the daemon reports it; `ze doctor` before a start has no RIB to read. Run `show rib candidates prefix <prefix>` to see the hidden path and the chain that hid it, then let the chain accept that prefix, or stop originating it in the IGP instead of filtering it locally.",
		Examples:    []string{"ze show doctor", "ze show rib candidates prefix 10.1.0.0/24", "ze show diagnostic doctor-policy-igp-import-hides-nexthop"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: policy: doctor code registration refused: " + err.Error())
		}
	}
}
