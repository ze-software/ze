// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- USE_TRANSPORT_MODE negotiation
// Related: child.go -- the Child SA install that carries the negotiated mode
// Related: ts_narrow.go -- the single-address constraint transport mode puts on selectors
// RFC: rfc/short/rfc7296.md -- USE_TRANSPORT_MODE (Section 1.3.1), transport-mode
// RFC: rfc/short/rfc7296.md -- traffic selectors (Section 2.23.1)

package engine

import (
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// wantsTransportMode reports whether the operator configured this peer for transport
// mode.
//
// RFC 7296 Section 1.3.1: "Except when using this option to negotiate transport mode,
// all Child SAs will use tunnel mode." Tunnel is therefore the answer for every peer that
// did not ask, and the YANG default states the same thing.
func wantsTransportMode(sa *SA) bool {
	return sa.PeerCfg.Mode == dataplane.ModeTransport
}

// transportSelectorPairs rewrites a peer's configured selectors for a transport-mode
// PROPOSAL, pinning every entry to the IKE SA's own address pair.
//
// RFC 7296 Section 2.23.1 MUST: "For transport mode, it MUST use exactly one IP address
// in the TSi and TSr payloads." Under "For the client proposing transport mode" the same
// section is more specific still:
//
//	"The TSi entries MUST have exactly one IP address, and that MUST match the source
//	address of the IKE SA."
//	"The TSr entries MUST have exactly one IP address, and that MUST match the
//	destination address of the IKE SA."
//
// The addresses come from the IKE SA's own OBSERVED pair, so a transport-mode proposal
// cannot name an address the IKE SA does not run on (observedLocalAddress and
// observedRemoteAddress, ts_nat_substitute.go). Reading the configured pair here made the
// claim accidentally true, because the two are equal with no NAT on the path, and false
// the moment the peer moves. The port and protocol of each configured selector SURVIVE:
// the same section allows several selectors in transport mode, "for example, multiple
// port ranges", provided every one carries the single address. The constraint is one
// ADDRESS, never one SELECTOR.
//
// The addresses are this node's OWN view, which is what the two MUSTs ask for: the source
// and the destination of the IKE SA as this stack sees them. Behind a NAT the far end sees
// a different pair, and RFC 7296 Section 2.23.1's substitution is what reconciles the two
// (ts_nat_substitute.go). Proposing the post-NAT address here instead would be a guess
// about a translation this node cannot observe.
//
// It returns nil when either address is unusable, and the caller then proposes nothing
// rather than proposing a wildcard that would violate the MUST.
func transportSelectorPairs(sa *SA, configured []tsPair) []tsPair {
	local := observedLocalAddress(sa)
	remote := observedRemoteAddress(sa)
	if local == nil || remote == nil {
		return nil
	}
	localNet := ipToFullNet(local)
	remoteNet := ipToFullNet(remote)

	if len(configured) == 0 {
		return []tsPair{{
			I: tsSelector{Net: localNet, Port: ipsec.AnyPort()},
			R: tsSelector{Net: remoteNet, Port: ipsec.AnyPort()},
		}}
	}

	out := make([]tsPair, 0, len(configured))
	for _, p := range configured {
		out = append(out, tsPair{
			I: tsSelector{Net: localNet, Port: p.I.Port, Proto: p.I.Proto},
			R: tsSelector{Net: remoteNet, Port: p.R.Port, Proto: p.R.Proto},
		})
	}
	return out
}

// transportModeNotify builds the USE_TRANSPORT_MODE notification.
//
// RFC 7296 Section 1.3.1 names it: "The USE_TRANSPORT_MODE notification MAY be included
// in a request message that also includes an SA payload requesting a Child SA. It
// requests that the Child SA use transport mode rather than tunnel mode for the SA
// created" (rfc/full/rfc7296.txt:802-805).
func transportModeNotify() *wire.PayloadNotify {
	return &wire.PayloadNotify{NotifyMsgType: wire.NotifyUseTransportMode}
}

// decideResponderTransportMode answers the mode of the Child SA this responder creates
// for a request that did (requested) or did not carry USE_TRANSPORT_MODE, and it must
// run before the selectors are narrowed, which judge them in that mode. It writes
// nothing: each exchange holds its own decision, and only IKE_AUTH keeps it on the SA
// (UseTransportMode), for the Child SA its later rounds create.
//
// RFC 7296 Section 1.3.1: "If the request is accepted, the response MUST also include a
// notification of type USE_TRANSPORT_MODE. If the responder declines the request, the
// Child SA will be established in tunnel mode."
//
// Acceptance is a DECISION, never an echo. Ze accepts only when the operator configured
// this peer for transport mode, so a peer cannot talk Ze out of the mode its operator
// chose. A request Ze declines answers tunnel, and the Child SA is established in tunnel
// mode with no notification in the response, which is exactly the outcome the RFC names
// for a decline.
func decideResponderTransportMode(sa *SA, requested bool) uint8 {
	// RFC 7296 Section 1.3.1 lets the responder decline. A pool-bound peer is declined:
	// its leased inner address is a tunnel-mode concept.
	if sa.PeerCfg.VirtualIPPoolName() != "" {
		return modeTunnel
	}
	if !requested {
		return modeTunnel
	}
	if !wantsTransportMode(sa) {
		return modeTunnel
	}
	return modeTransport
}

// adoptAuthResponseNegotiation records what the responder answered on the initiator's
// IKE_AUTH: the Child SA mode, and the narrowed traffic selectors.
//
// It returns ok=false when the SA must be torn down, and the notify type the peer is owed
// for that teardown (0 when no error notification applies).
//
// THE CALLER MUST TELL THE PEER. RFC 7296 Section 2.21.2 lets the initiator report an error
// it found in a response "in a separate INFORMATIONAL exchange", and it closes the set of
// notifications that delete an SA WITHOUT a Delete payload: UNSUPPORTED_CRITICAL_PAYLOAD,
// INVALID_SYNTAX and AUTHENTICATION_FAILED. Neither teardown below is in that set, so both
// owe the peer an INFORMATIONAL carrying a Delete payload (sendIKESATeardown, delete.go).
// Setting State to StateDead and sending nothing leaves the peer encrypting to a node that
// has no SA.
//
// Every caller runs it AFTER the response's AUTH is verified. An unauthenticated message
// must be unable to tear the SA down by omitting a notification, and must be unable to
// choose the traffic the SA protects by naming its own selectors.
//
// childOffered says the response carried SAr2. A response that accepts a Child SA and
// names no traffic selector is refused here (see below), never adopted with none.
func adoptAuthResponseNegotiation(sa *SA, transportAccepted, childOffered bool, tsi, tsr *wire.PayloadTS, log *slog.Logger) (ok bool, notify uint16) {
	// The IKE_AUTH mode is kept on the SA (UseTransportMode) for createFirstChildSA,
	// which builds the Child SA after this returns; no other exchange reads it.
	mode, deleteSA := recordInitiatorTransportMode(sa, transportAccepted)
	sa.UseTransportMode = mode == modeTransport
	if deleteSA {
		log.Warn("ike: peer declined transport mode and transport-required is set, deleting the SA",
			"peer", sa.PeerName)
		// RFC 7296 Section 1.3.1: "If this is unacceptable to the initiator, the initiator
		// MUST delete the SA." The peer broke nothing -- it declined an option, which the
		// section permits -- so the Delete carries no error notification.
		return false, 0
	}
	// RFC 7296 Section 2.9: the responder's TS payloads carry the NARROWED selectors, and
	// they are what this side installs, so both ends program the same traffic. Narrowing is
	// one-way, so an answer that WIDENS the proposal ends the exchange instead of being
	// installed (ts_narrow.go).
	//
	// The floor is nil: this is the FIRST Child SA of the IKE SA, so no scope is in use yet
	// for RFC 7296 Section 2.9.2 to protect. applyChildRekeyResponse (rekey.go) passes the
	// SA being replaced.
	//
	// A response carrying no Child SA carries no selector to adopt: an EAP round, or a
	// childless answer (RFC 7296 Section 2.21.2). A response carrying SAr2 MUST carry both
	// TS payloads. RFC 7296 Section 2.9: "Two TS payloads appear in each of the messages
	// in the exchange that creates a Child SA pair.", and Section 1.2 has the responder
	// complete the Child SA "with the additional fields described below in the
	// CREATE_CHILD_SA exchange". Accepting such a response with no selector left
	// NegotiatedPairs nil, and the Child SA was then installed with host-to-host selectors
	// for every protocol and port, wider than the pairs ze proposed. RFC 4301 Section
	// 4.4.2: "the entry for an inbound SA in the SAD MUST be initially populated with the
	// value or values negotiated at the time the SA was created". Nothing was negotiated,
	// so there is no value to populate it with and the Child SA is refused.
	//
	// INVALID_SYNTAX is the notification RFC 7296 Section 3.10.1 gives a message that "was
	// invalid because some type, length, or value was out of range", and a missing
	// mandatory payload is such a message. Section 2.21.2 makes it end the IKE SA.
	if !childOffered {
		return true, 0
	}
	if tsi == nil {
		log.Warn("ike: the responder accepted the Child SA with no TSi payload, deleting the SA",
			"peer", sa.PeerName, "fix", childResponseFixNoTS)
		return false, wire.NotifyInvalidSyntax
	}
	if tsr == nil {
		log.Warn("ike: the responder accepted the Child SA with no TSr payload, deleting the SA",
			"peer", sa.PeerName, "fix", childResponseFixNoTS)
		return false, wire.NotifyInvalidSyntax
	}
	if err := recordInitiatorSelectorsIn(sa, tsi, tsr, nil, mode); err != nil {
		log.Warn("ike: the responder's traffic selectors cannot be adopted, deleting the SA",
			"peer", sa.PeerName, "error", err)
		// RFC 7296 Section 2.9 names the notification for a traffic-selector answer
		// this node will not accept: TS_UNACCEPTABLE.
		return false, wire.NotifyTSUnacceptable
	}
	return true, 0
}

// recordInitiatorTransportMode reads the responder's answer to a transport-mode request:
// it answers the mode the Child SA installs and whether the SA must be deleted. It writes
// nothing; the caller keeps the mode for the exchange it serves.
//
// RFC 7296 Section 1.3.1: "If the responder declines the request, the Child SA will be
// established in tunnel mode. If this is unacceptable to the initiator, the initiator
// MUST delete the SA."
//
// Only the operator knows whether a downgrade is unacceptable, so the MUST is gated on
// the transport-required leaf. It fails SAFE rather than closed: a peer that declines
// leaves a working tunnel-mode SA unless the operator stated that tunnel mode is worse
// than no tunnel at all. Fail-closed here would tear down every working tunnel whose
// peer does not implement transport mode.
//
// It returns false whenever Ze never asked, so a tunnel-mode peer is unaffected.
func recordInitiatorTransportMode(sa *SA, responseHasNotify bool) (mode uint8, deleteSA bool) {
	if !wantsTransportMode(sa) {
		return modeTunnel, false
	}
	if responseHasNotify {
		return modeTransport, false
	}
	return modeTunnel, sa.PeerCfg.TransportRequired
}
