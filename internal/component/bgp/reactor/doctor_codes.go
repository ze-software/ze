// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/reactor.

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorBGPPeerNotReading = "doctor-bgp-peer-not-reading"
	codeDoctorBGPWritePathHeld  = "doctor-bgp-write-path-held"
	codeDoctorBGPASZeroOutbound = "doctor-bgp-as-zero-outbound"
	// CodeDoctorBGPRFC8212NoPolicy is the doctor diagnostic code "doctor-bgp-rfc8212-no-policy".
	CodeDoctorBGPRFC8212NoPolicy = "doctor-bgp-rfc8212-no-policy"
	// CodeDoctorBGPCaptureDirectory is the doctor diagnostic code "doctor-bgp-capture-directory".
	CodeDoctorBGPCaptureDirectory = "doctor-bgp-capture-directory"
	exampleShowDoctor             = "ze show doctor"
	exampleShowErrors             = "ze show errors"
	exampleShowBGPPeerList        = "ze show bgp peer list"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorBGPPeerNotReading,
		Title: "BGP session closed: the peer accepted no data for the write budget",
		Description: "ze had data to send to a BGP peer, and the peer accepted none of it for the whole write budget: its TCP connection stayed open and its receive window stayed shut. " +
			"A peer in that state is not processing BGP messages, and a write that waits for it holds the session, so ze closed the connection without a NOTIFICATION, which the peer could not have read. " +
			"ze connects to the peer again by itself, and sends its routes again once the session is Established. " +
			"The event names the peer and the budget that passed. The budget of a route write is ze.fwd.write.deadline (30s by default); a KEEPALIVE, OPEN or NOTIFICATION has a third of the hold time, between 10s and 30s. " +
			"Remedy: on the peer, find why its BGP process stopped reading (CPU load, a stalled process, a full input queue). " +
			"Raise ze.fwd.write.deadline only when the peer is known to be slow and healthy.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show bgp peer", "ze show diagnostic doctor-bgp-peer-not-reading"},
	},
	{
		Code:  codeDoctorBGPWritePathHeld,
		Title: "BGP session closed while another writer held its write path",
		Description: "ze had to close a BGP session, and a writer of that session still held the session's write path when the wait for it ran out. " +
			"The wait is one second when a hold timer or a send hold timer expired, and the control budget (a third of the hold time, between 10s and 30s) for every other close. " +
			"ze does not wait longer for a writer that the close itself would unblock: it fails that writer's socket write, closes the connection, and drops what the writer had not sent. " +
			"When a NOTIFICATION was due and the write path was held for its whole wait, the peer is not told why the session ended. " +
			"The usual cause is a peer that stopped reading with its connection open, so a route write to it was blocked. " +
			"The other cause is a plugin that does not take the events ze delivers to it, which keeps a writer waiting: the peer then connects again only when that plugin drains or stops. " +
			"Remedy: on the peer, find why its BGP process stopped reading (CPU load, a stalled process). When the peer reads normally, run `ze show doctor`: its plugin rows name a plugin that stopped, crashed, or whose event batches failed or were slow to complete. A plugin that still holds one batch has no such row until that batch ends.",
		Examples:     []string{exampleShowDoctor, exampleShowErrors, "ze show bgp peer", "ze show diagnostic doctor-bgp-write-path-held"},
		RelatedCodes: []string{codeDoctorBGPPeerNotReading},
	},
	{
		Code:  codeDoctorBGPASZeroOutbound,
		Title: "BGP route withdrawn because an AS attribute contains zero",
		Description: "A route source or export policy supplied AS zero in AS_PATH, AS4_PATH, AGGREGATOR or AS4_AGGREGATOR. " +
			"RFC 7607 forbids advertising that route, so ze withheld the announcement and withdrew any older advertised version through the normal ownership checks. " +
			"The BGP session stays open. The event names the peer and attribute; repeated events are rate limited per session. " +
			"Remedy: remove AS zero from the route source, plugin or export policy and send the corrected route again. " +
			"Use ze show errors to find the affected peer and attribute.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, "ze show diagnostic doctor-bgp-as-zero-outbound"},
	},
	{
		Code:        CodeDoctorBGPRFC8212NoPolicy,
		Title:       "An external BGP direction has no explicit policy",
		Description: "RFC8212 Section3 prevents an external peer's unconfigured import direction from contributing learned routes or its unconfigured export direction from sending routes. Configure an explicit policy with the peer or group's editor command and commit it. An explicit rejecting final also resolves the absence. The doctor result describes its supplied saved configuration.",
		Examples:    []string{"ze doctor --json", exampleShowBGPPeerList},
	},
	{
		Code:         CodeDoctorBGPCaptureDirectory,
		Title:        "BGP capture directory not usable",
		Description:  "A BGP peer sets capture{enabled true} but the configured capture directory cannot be created or written by the ze user. The session will still establish: protocol event capture is a diagnostic aid and never blocks BGP. It will simply record nothing, which is indistinguishable from a quiet peer unless doctor says so. Remedy: create the directory and give the ze user write access, name a writable path in the peer's capture{directory ...}, or set capture{enabled false}.",
		Examples:     []string{"ze doctor --json", "ze show diagnostic doctor-bgp-capture-directory"},
		RelatedCodes: []string{"doctor-write-destination"},
	},
}

// registerDoctorCodes registers this package's doctor diagnostic codes.
func registerDoctorCodes() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: reactor: doctor code registration refused: " + err.Error())
		}
	}
}
