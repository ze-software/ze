// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/server.

package server

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorBGPPluginEventsUndelivered is the doctor diagnostic code
	// "doctor-bgp-plugin-events-undelivered" for BGP EventBus notifications
	// that could not be emitted to subscribed components or plugins.
	CodeDoctorBGPPluginEventsUndelivered = "doctor-bgp-plugin-events-undelivered"

	// PluginEventsUndeliveredFix is the executable CLI command operators run
	// to inspect plugin state when a BGP EventBus notification fails to emit.
	PluginEventsUndeliveredFix = "ze show plugin list"
)

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:  CodeDoctorBGPPluginEventsUndelivered,
		Title: "BGP event could not be delivered on the event bus",
		Description: "The BGP engine or server could not emit a runtime notification " +
			"(such as End-of-RIB, listener-ready, congestion state, or update notification) " +
			"to subscribers on the internal event bus. " +
			"The BGP session keeps running, but plugins or components watching that event may miss a state transition. " +
			"Remedy: inspect running plugins with ze show plugin list and check ze show errors for the failing event.",
		Examples: []string{
			"ze show doctor",
			"ze show errors",
			PluginEventsUndeliveredFix,
			"ze show diagnostic " + CodeDoctorBGPPluginEventsUndelivered,
		},
	}); err != nil {
		panic("BUG: bgp server: doctor code registration refused: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeDoctorBGPPluginEventsUndelivered}); err != nil {
		panic("BUG: bgp server: event code registration refused: " + err.Error())
	}
}
