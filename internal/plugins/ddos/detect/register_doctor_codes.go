// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ddos/detect.

package detect

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorDDoSDetectBlind        = "doctor-ddos-detect-blind"
	codeDoctorDDoSDetectNoFlowSource = "doctor-ddos-detect-no-flow-source"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorDDoSDetectBlind,
		Title:       "DDoS detector is armed but its interface rate feed shows no traffic",
		Description: "ddos-detect is enabled, but for a whole baseline window (baseline-window times check-interval seconds) its interface rate feed has either delivered no sample, reported no interface, or reported zero received packets and bytes on every interface. The baseline then holds only zeros: it reads as ready, its threshold sits at the absolute floor, and the detector cannot tell an attack from no traffic. An external ddos-detect never receives the feed, because it is a same-process subscription; remove the `plugin { external ddos-detect { ... } }` block so ze runs it internal. Today an external ddos-detect never arms: it fails startup with `ddos-detect: event bus not configured` in the log, because a forked plugin is handed no event bus. Once that is fixed, the external case is reported as soon as the detector is armed, with no window. The input is the per-interface receive rate from the trafficstat service, not the traffic-usage or flow-export flow sources, which only characterize an attack after detection. A lab or standby router with no traffic is a legitimate cause: the warning clears on the first sample with traffic, and `ddos detect enabled false` silences it. Otherwise check the rate feed with `ze show interface rate` and check that interfaces exist with `ze show interface`.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ddos-detect-blind"},
	},
	{
		Code:        codeDoctorDDoSDetectNoFlowSource,
		Title:       "DDoS characterization has no flow source",
		Description: "ddos-detect is enabled with characterization on, but neither traffic-usage (track-ip) nor flow-export (conntrack) is configured. Detection still works, but mitigation degrades to generic-flood with no target prefix, so responders cannot install a surgical or targeted rule. Enable traffic-usage track-ip and/or flow-export conntrack.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-ddos-detect-no-flow-source"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: detect: doctor code registration refused: " + err.Error())
		}
	}
}
