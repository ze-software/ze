// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/adj_rib_in.

package adj_rib_in

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// codeDoctorBGPAdjRIBInNLRIKeyFailed fires when a complex-family NLRI
	// (labeled unicast, VPN, EVPN, FlowSpec, etc.) cannot be keyed for
	// Adj-RIB-In installation or withdrawal.
	codeDoctorBGPAdjRIBInNLRIKeyFailed = "doctor-bgp-adj-rib-in-nlri-key-failed"

	// fixShowAdjRIBIn is the operator command to inspect stored Adj-RIB-In state.
	fixShowAdjRIBIn = "ze show bgp adj-rib-in"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorBGPAdjRIBInNLRIKeyFailed,
		Title: "Adj-RIB-In could not derive a route key for a complex NLRI",
		Description: "An announced or withdrawn NLRI for a complex address family (such as labeled unicast, VPN, EVPN, or FlowSpec) " +
			"could not be converted into a storage key, so that NLRI was not installed or removed in Adj-RIB-In while other NLRIs in " +
			"the UPDATE continued processing. ze_adjribin_nlri_key_errors_total counts every occurrence; warnings are rate-limited per " +
			"peer, family, and operation. Remedy: inspect stored received routes with `ze show bgp adj-rib-in` and check peer session " +
			"negotiation and wire encodings.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorBGPAdjRIBInNLRIKeyFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorBGPAdjRIBInNLRIKeyFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: adj-rib-in: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: adj-rib-in: doctor event code registration refused: " + err.Error())
		}
	}
}
