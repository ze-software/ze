// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/ddos/detect.

package detect

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorDDoSDetectBlind              = "doctor-ddos-detect-blind"
	codeDoctorDDoSDetectNoFlowSource       = "doctor-ddos-detect-no-flow-source"
	codeDoctorDDoSDetectBaselineSaveFailed = "doctor-ddos-detect-baseline-save-failed"
	codeDoctorDDoSDetectBaselineLoadFailed = "doctor-ddos-detect-baseline-load-failed"
	codeDoctorDDoSDetectDirectionFallback  = "doctor-ddos-detect-direction-fallback"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorDDoSDetectBlind,
		Title:       "DDoS detector is armed but its interface rate feed shows no traffic",
		Description: "ddos-detect is enabled, but for a whole baseline window (baseline-window times check-interval seconds) its interface rate feed has either delivered no sample, reported no interface, or reported zero received packets and bytes on every interface. The baseline then holds only zeros: it reads as ready, its threshold sits at the absolute floor, and the detector cannot tell an attack from no traffic. An external ddos-detect never receives the feed, because it is a same-process subscription; remove the `bus { endpoint ddos-detect { external { ... } } }` block so ze runs it internal. Today an external ddos-detect never arms: it fails startup with `ddos-detect: event bus not configured` in the log, because a forked plugin is handed no event bus. Once that is fixed, the external case is reported as soon as the detector is armed, with no window. The input is the per-interface receive rate from the trafficstat service, not the traffic-usage or flow-export flow sources, which only characterize an attack after detection. A lab or standby router with no traffic is a legitimate cause: the warning clears on the first sample with traffic, and `ddos detect enabled false` silences it. Otherwise check the rate feed with `ze show interface rate` and check that interfaces exist with `ze show interface`.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDoctorDDoSDetectBlind},
	},
	{
		Code:        codeDoctorDDoSDetectNoFlowSource,
		Title:       "DDoS characterization has no flow source",
		Description: "ddos-detect is enabled with characterization on, but neither traffic-usage (track-ip) nor flow-export (conntrack) is configured. Detection still works, but mitigation degrades to generic-flood with no target prefix, so responders cannot install a surgical or targeted rule. Enable traffic-usage track-ip and/or flow-export conntrack.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeDoctorDDoSDetectNoFlowSource},
	},
	{
		Code:        codeDoctorDDoSDetectBaselineSaveFailed,
		Title:       "DDoS detector could not save its rolling baseline",
		Description: "ddos-detect failed to write its rolling PPS/BPS baseline snapshot to the daemon managed state store. Detection continues with the in-memory baseline, but a restart or reload will re-warm from zero instead of resuming with the learned threshold. Check store health and free space with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorDDoSDetectBaselineSaveFailed},
	},
	{
		Code:        codeDoctorDDoSDetectBaselineLoadFailed,
		Title:       "DDoS detector could not load its saved baseline",
		Description: "ddos-detect failed to read its persisted baseline snapshot from the daemon managed state store. The detector continues and re-warms from live traffic, which leaves a cold startup-grace window until baseline-window samples accumulate. Check store health with `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorDDoSDetectBaselineLoadFailed},
	},
	{
		Code:        codeDoctorDDoSDetectDirectionFallback,
		Title:       "DDoS detector could not determine whether victim address is local",
		Description: "ddos-detect could not query the interface backend to check whether a characterized attack victim address is local to this router, so it assumed remote (FORWARD hook) as the fail-safe. Mitigation for a locally terminated victim may miss the INPUT hook until interface state is readable again. Check interface backend health with `ze show interface` and `ze show doctor`.",
		Examples:    []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorDDoSDetectDirectionFallback},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorDDoSDetectBaselineSaveFailed},
	{Code: codeDoctorDDoSDetectBaselineLoadFailed},
	{Code: codeDoctorDDoSDetectDirectionFallback},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: detect: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: detect: doctor event code registration refused: " + err.Error())
		}
	}
}
