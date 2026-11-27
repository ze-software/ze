// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/flowexport.

package flowexport

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorFlowexportConntrackUnavailable        = "doctor-flowexport-conntrack-unavailable"
	codeDoctorFlowexportConntrackDestroyUnavailable = "doctor-flowexport-conntrack-destroy-unavailable"
	codeDoctorFlowexportSamplingTeardownFailed      = "doctor-flowexport-sampling-teardown-failed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowWarnings   = "ze show warnings"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorFlowexportConntrackUnavailable,
		Title:       "flow-export conntrack table unavailable",
		Description: "flow-export conntrack export is enabled but nf_conntrack is not loaded (nothing -- no firewall/NAT rule, no modprobe -- pulled it in), so the ctnetlink dump reads an empty table. The recent-flow ring stays empty and DDoS characterization degrades to generic-flood with no discriminating vector. Load nf_conntrack (a firewall or NAT rule does this) so per-flow records are available.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDoctorFlowexportConntrackUnavailable},
	},
	{
		Code:        codeDoctorFlowexportConntrackDestroyUnavailable,
		Title:       "flow-export conntrack destroy listener unavailable",
		Description: "flow-export could not subscribe to kernel NFNLGRP_CONNTRACK_DESTROY events, so conntrack export fell back to periodic table dumps only and short-lived flows ending between dumps may be missed. Verify CAP_NET_ADMIN and nf_conntrack_netlink support with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowWarnings, exampleShowDiagPrefix + codeDoctorFlowexportConntrackDestroyUnavailable},
	},
	{
		Code:        codeDoctorFlowexportSamplingTeardownFailed,
		Title:       "flow-export tc packet-sampling filter teardown failed",
		Description: "flow-export failed to remove an installed tc ingress sample filter when stopping or reloading packet sampling. A stale kernel tc sample filter may remain on the interface until removed manually or replaced by the next sampling configuration. Inspect the interface filters and check `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorFlowexportSamplingTeardownFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorFlowexportConntrackDestroyUnavailable},
	{Code: codeDoctorFlowexportSamplingTeardownFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: flowexport: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: flowexport: doctor event code registration refused: " + err.Error())
		}
	}
}
