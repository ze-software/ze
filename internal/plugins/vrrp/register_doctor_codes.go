// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/vrrp.
// Related: telemetry.go -- publishStateChange emit-failure warning site

package vrrp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorVRRPStateEventEmitFailed = "doctor-vrrp-state-event-emit-failed"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// vrrpEventDiagnosticCodes is the explanation metadata for VRRP report-bus event codes.
var vrrpEventDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorVRRPStateEventEmitFailed,
		Title: "VRRP could not emit a state-change event",
		Description: "A VRRP virtual router transitioned state, but publishing the vrrp:state-change event on the " +
			"daemon event bus failed. Downstream subscribers watching VRRP mastership changes may miss the " +
			"transition until the next state change. Check event bus health with `ze show vrrp` and `ze show doctor`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorVRRPStateEventEmitFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorVRRPStateEventEmitFailed},
}

func init() {
	for _, meta := range vrrpEventDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: vrrp: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: vrrp: doctor event code registration refused: " + err.Error())
		}
	}
}
