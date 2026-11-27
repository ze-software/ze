// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/bmp.

package bmp

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// codeDoctorBMPRIBSyncFailed fires when the BMP receiver cannot sync received
	// routes or withdrawals with the BGP RIB.
	codeDoctorBMPRIBSyncFailed = "doctor-bmp-rib-sync-failed"
	// codeDoctorBMPReceiverSessionError fires when an active BMP receiver session
	// fails while reading a message body from a monitored router.
	codeDoctorBMPReceiverSessionError = "doctor-bmp-receiver-session-error"
	// codeDoctorBMPSenderDeliveryFailed fires when the BMP sender cannot queue or
	// write a monitoring message to a configured collector.
	codeDoctorBMPSenderDeliveryFailed = "doctor-bmp-sender-delivery-failed"

	fixShowBMPPeers      = "ze show bmp peers"
	fixShowBMPSessions   = "ze show bmp sessions"
	fixShowBMPCollectors = "ze show bmp collectors"

	exampleDoctorJSON     = "ze doctor --json"
	exampleShowErrors     = "ze show errors"
	exampleShowDiagPrefix = "ze show diagnostic "
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeBMPUnreachable,
		Title:       "BMP collectors unreachable",
		Description: "None of the configured BMP sender collectors accepted a TCP connection within the probe timeout.",
		Examples:    []string{exampleDoctorJSON, exampleShowDiagPrefix + codeBMPUnreachable},
	},
	{
		Code:  codeDoctorBMPRIBSyncFailed,
		Title: "BMP receiver failed to sync monitored routes with the BGP RIB",
		Description: "The BMP receiver could not inject a received Route Monitoring UPDATE into the BGP RIB or could not withdraw " +
			"routes for a terminating router or downed monitored peer, so the BMP RIB view may lag or retain stale routes. " +
			"ze_bmp_route_inject_errors_total counts route injection failures. Remedy: inspect monitored peers with `ze show bmp peers` " +
			"and verify the BGP RIB plugin is healthy.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorBMPRIBSyncFailed},
	},
	{
		Code:  codeDoctorBMPReceiverSessionError,
		Title: "BMP receiver session read failed",
		Description: "Reading the body of an inbound BMP message from a monitored router failed mid-frame, so the receiver closed " +
			"that BMP session and withdrew its routes. Remedy: inspect receiver sessions with `ze show bmp sessions` and check TCP " +
			"connectivity from the monitored router.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorBMPReceiverSessionError},
	},
	{
		Code:  codeDoctorBMPSenderDeliveryFailed,
		Title: "BMP sender failed to deliver a monitoring message to a collector",
		Description: "Writing or queueing a BMP Peer Up, Peer Down, Route Monitoring, Route Mirroring, Loc-RIB, or Statistics Report " +
			"message to a configured collector failed. ze_bmp_message_drops_total counts every dropped message by collector, message " +
			"kind, and reason; warnings are rate-limited per collector and message kind. Remedy: check collector connection state " +
			"and error counters with `ze show bmp collectors`.",
		Examples: []string{exampleDoctorJSON, exampleShowErrors, exampleShowDiagPrefix + codeDoctorBMPSenderDeliveryFailed},
	},
}

// doctorEventCodes declares the transient report-bus event codes this package raises.
var doctorEventCodes = []diagnostic.EventCode{
	{Code: codeDoctorBMPRIBSyncFailed},
	{Code: codeDoctorBMPReceiverSessionError},
	{Code: codeDoctorBMPSenderDeliveryFailed},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: bmp: doctor code registration refused: " + err.Error())
		}
	}
	for _, ev := range doctorEventCodes {
		if err := diagnostic.RegisterEventCode(ev); err != nil {
			panic("BUG: bmp: doctor event code registration refused: " + err.Error())
		}
	}
}
