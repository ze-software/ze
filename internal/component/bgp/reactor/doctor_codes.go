// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/reactor.

package reactor

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorBGPPeerNotReading          = "doctor-bgp-peer-not-reading"
	codeDoctorBGPWritePathHeld           = "doctor-bgp-write-path-held"
	codeDoctorBGPASZeroOutbound          = "doctor-bgp-as-zero-outbound"
	codeDoctorBGPBFDUnavailable          = "doctor-bgp-bfd-unavailable"
	codeDoctorBGPDefaultOriginateNotSent = "doctor-bgp-default-originate-not-sent"
	codeDoctorBGPRouteNotSent            = "doctor-bgp-route-not-sent"
	codeDoctorBGPCachePressure           = "doctor-bgp-cache-pressure"
	codeDoctorBGPMalformedFromPeer       = "doctor-bgp-malformed-from-peer"
	// CodeDoctorBGPRFC8212NoPolicy is the doctor diagnostic code "doctor-bgp-rfc8212-no-policy".
	CodeDoctorBGPRFC8212NoPolicy = "doctor-bgp-rfc8212-no-policy"
	// CodeDoctorBGPCaptureDirectory is the doctor diagnostic code "doctor-bgp-capture-directory".
	CodeDoctorBGPCaptureDirectory = "doctor-bgp-capture-directory"
	exampleShowDoctor             = "ze show doctor"
	exampleShowErrors             = "ze show errors"
	exampleShowBGPPeerList        = "ze show bgp peer list"
	fixBFDUnavailable             = "ze show bfd sessions"
	fixDefaultOriginateNotSent    = exampleShowBGPPeerList
	fixRouteNotSent               = exampleShowBGPPeerList
	fixCachePressure              = "ze show cache"
	fixMalformedFromPeer          = exampleShowBGPPeerList
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
	{
		Code:  codeDoctorBGPBFDUnavailable,
		Title: "BGP BFD session release or operation failed",
		Description: "The BGP reactor could not release or synchronize a peer's BFD session handle with the BFD engine. " +
			"The BGP peer continued tearing down or canceling its start, so a stale BFD hold may remain active until BFD clears it. " +
			"Remedy: inspect active BFD sessions with ze show bfd sessions and check ze show errors for the peer and cause.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, fixBFDUnavailable, "ze show diagnostic " + codeDoctorBGPBFDUnavailable},
	},
	{
		Code:  codeDoctorBGPDefaultOriginateNotSent,
		Title: "BGP default-originate route was not sent to peer",
		Description: "A peer has default-originate configured for an address family, but Ze could not send the originated default route on the established session. " +
			"The peer will not receive the default route until the session recovers or the next-hop and policy configuration is corrected. " +
			"Remedy: check peer state and capabilities with ze show bgp peer list and inspect ze show errors for the peer and family.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, fixDefaultOriginateNotSent, "ze show diagnostic " + codeDoctorBGPDefaultOriginateNotSent},
	},
	{
		Code:  codeDoctorBGPRouteNotSent,
		Title: "BGP route was not sent to peer",
		Description: "Ze could not resolve a next hop or encode a configured static, queued, or plugin route for an established BGP peer, so that route was skipped while the session stayed up. " +
			"The peer will lack the skipped prefix until the route's next hop, negotiated capabilities, or message size limits are corrected. " +
			"Remedy: inspect peer state and capabilities with ze show bgp peer list and check ze show errors for the peer and prefix.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, fixRouteNotSent, "ze show diagnostic " + codeDoctorBGPRouteNotSent},
	},
	{
		Code:  codeDoctorBGPCachePressure,
		Title: "BGP recent-UPDATE cache acknowledgement failed",
		Description: "A plugin batch acknowledgement against the BGP recent-UPDATE cache failed because one or more cached UPDATE entries had already expired or been evicted. " +
			"This indicates cache pressure or a delayed consumer behind the cache window. " +
			"Remedy: inspect retained entries with ze show cache and check ze show errors for the plugin name and update ID.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, fixCachePressure, "ze show diagnostic " + codeDoctorBGPCachePressure},
	},
	{
		Code:  codeDoctorBGPMalformedFromPeer,
		Title: "BGP UPDATE attribute section from peer could not be indexed",
		Description: "A received BGP UPDATE reached publication with an attribute section whose span index could not be built. " +
			"Downstream consumers receive the parse error rather than indexed attributes for that UPDATE. " +
			"Remedy: inspect the peer session with ze show bgp peer list and check ze show errors for the peer and parse failure.",
		Examples: []string{exampleShowDoctor, exampleShowErrors, fixMalformedFromPeer, "ze show diagnostic " + codeDoctorBGPMalformedFromPeer},
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
