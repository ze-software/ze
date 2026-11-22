// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/l2tp/pppoe/dataplane/vpp.

package vpp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPPPoEVPPReconcile     = "doctor-pppoe-vpp-reconcile"
	codeDoctorPPPoEVPPSocketCleanup = "doctor-pppoe-vpp-socket-cleanup"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorPPPoEVPPReconcile,
		Title:       "PPPoE VPP resource cleanup is incomplete",
		Description: "A PPPoE session or control mapping retains an uncertain VPP mutation. Ze keeps the exact owner reserved instead of reusing a native index or deleting another owner's resource. Inspect the retained warning and restore the reported VPP connection; only confirmed cleanup or a proven process exit can clear it.",
		Examples:    []string{"ze show doctor", "ze show warnings"},
	},
	{
		Code:        codeDoctorPPPoEVPPSocketCleanup,
		Title:       "PPPoE control socket cleanup is unconfirmed",
		Description: "A canceled PPPoE VPP control-socket attempt could not confirm descriptor cleanup or restoration of its original network namespace. The daemon retains the original attempt and cause. Inspect the warning and repair the reported native condition; another successful attempt cannot confirm this original release.",
		Examples:    []string{"ze show doctor", "ze show warnings"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: vpp: doctor code registration refused: " + err.Error())
		}
	}
}
