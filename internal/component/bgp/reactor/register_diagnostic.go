// Design: docs/architecture/doctor-and-health-checks.md -- event codes an owner raises on the report bus
// Related: session_write_budget.go -- reportWriteBudgetExpiry, which raises the code registered here
// Related: session_rfc7607.go -- final admission reports routes carrying AS zero
// Related: pending_refusal_diagnostic.go -- pending-connection rejection failures

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

// The reactor raises write failures and final route refusals on the report
// bus, so it declares their codes to the recent-events
// doctor check, which then reports each event under its code in `show doctor`.
func init() {
	registerDoctorCodes()
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codePendingRefusalUndelivered,
		Title:       "BGP rejection preparation or delivery failed",
		Description: "Ze could not prepare the socket, set the control-write deadline or send a rejection on a pending BGP connection. It still attempts the rejection after a socket-preparation failure, then closes only that connection. Inspect the reported transport error, then check peer states with ze show bgp peer list.",
		Examples:    []string{pendingRefusalFailureFix, "ze show diagnostic " + codePendingRefusalUndelivered},
	}); err != nil {
		panic("BUG: bgp reactor: pending refusal diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codePendingRefusalUndelivered}); err != nil {
		panic("BUG: bgp reactor: pending refusal event registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPPeerNotReading}); err != nil {
		panic("BUG: bgp reactor: write budget expiry event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPWritePathHeld}); err != nil {
		panic("BUG: bgp reactor: held write path event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPASZeroOutbound}); err != nil {
		panic("BUG: bgp reactor: outbound AS zero event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPBFDUnavailable}); err != nil {
		panic("BUG: bgp reactor: BFD unavailable event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPDefaultOriginateNotSent}); err != nil {
		panic("BUG: bgp reactor: default-originate unsent event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPRouteNotSent}); err != nil {
		panic("BUG: bgp reactor: route not sent event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPCachePressure}); err != nil {
		panic("BUG: bgp reactor: cache pressure event code registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPMalformedFromPeer}); err != nil {
		panic("BUG: bgp reactor: malformed UPDATE from peer event code registration failed: " + err.Error())
	}
}
