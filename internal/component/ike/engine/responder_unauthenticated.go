// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the responder handshake
// Related: responder.go -- handleAuthRequest, the first IKE_AUTH request
// Related: responder_eap.go -- handleResponderEAP, each EAP round
// Related: inbound.go -- decryptAndParse and errInnerParse, the distinction this file keeps
// RFC: rfc/short/rfc7296.md -- RFC7296-2.4-3, RFC7296-2.4-12

package engine

import (
	"errors"
	"log/slog"
)

// suppressedUnauthenticatedHandshake is the ze_ipsec_error_notify_suppressed_total reason
// for an IKE_AUTH request that failed its integrity check before the handshake
// authenticated. No notification may answer it (RFC 7296 Section 2.21.2 allows one only
// in a protected response), and the SA it names is left as it was.
const suppressedUnauthenticatedHandshake = "handshake-unauthenticated"

// discardUnauthenticatedHandshakeRequest decides whether an IKE_AUTH request that
// decryptAndParse refused is dropped with the responder's handshake left intact. It
// answers true, after counting and logging the drop, for every error that carries no
// proof the message came from the peer: no SK payload, or an SK payload whose integrity
// check or decryption failed. Anyone who has seen one datagram of the exchange can build
// such a message, because both SPIs and the Message ID travel in the clear.
//
// RFC 7296 Section 2.4: "Since IKE is designed to operate in spite of DoS attacks from the
// network, an endpoint MUST NOT conclude that the other endpoint has failed based on any
// routing information (e.g., ICMP messages) or IKE messages that arrive without
// cryptographic protection (e.g., Notify messages complaining about unknown SPIs)."
//
// It answers false for errInnerParse. That error is set only after the integrity check
// passed, so the message is the peer's own, and the caller keeps ending the handshake on
// it. Nothing here touches sa: not the state, not the Message ID window, not CreatedAt.
// The half-open reaper therefore still ends a handshake the peer abandoned, however many
// forgeries arrive meanwhile, and the inbound token bucket in dispatchInbound bounds how
// often this runs (RFC 7296 Section 2.4, "Implementations MUST limit the rate at which
// they take actions based on unprotected messages").
//
// The drop is counted, not logged at WARN: the counter is the operator's record, and a
// WARN line per forged datagram would let an off-path sender write the log.
func discardUnauthenticatedHandshakeRequest(sa *SA, err error, log *slog.Logger) bool {
	if errors.Is(err, errInnerParse) {
		return false
	}
	countErrorNotifySuppressed(suppressedUnauthenticatedHandshake)
	log.Debug("ike: unauthenticated IKE_AUTH request dropped, handshake kept",
		"peer", sa.PeerName, "state", sa.State, "error", err)
	return true
}
