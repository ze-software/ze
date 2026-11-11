// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the handshake, both roles
// Related: auth.go -- decryptSKPayload, which marks its post-checksum failures errInnerParse
// Related: inbound.go -- errInnerParse, and handleOwnedInbound, the established SA's four callers
// Related: held_successor.go -- handleHeldSuccessorInbound, the held successor's caller
// Related: fsm.go -- handleAuthResponse and handleEAPResponse, the initiator's callers
// Related: responder.go -- handleAuthRequest, the first IKE_AUTH request
// Related: responder_eap.go -- handleResponderEAP, each EAP round
// Related: multiple_auth.go -- handleResponderAuthRound, the RFC 4739 second exchange
// RFC: rfc/short/rfc7296.md -- RFC7296-2.4-3, RFC7296-2.4-12

package engine

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/wire"
)

// suppressedUnauthenticatedHandshake is the ze_ipsec_error_notify_suppressed_total reason
// for an IKE_AUTH message, request or response, that failed its integrity check before
// the handshake authenticated. No notification may answer it (RFC 7296 Section 2.21.2
// allows one only in a protected response), and the SA it names is left as it was.
const suppressedUnauthenticatedHandshake = "handshake-unauthenticated"

// The reasons an established SA's openers drop under. Each is the name of the guard that
// fired, so an operator can tell which one did (docs/guide/ipsec.md, "Health and metrics").
const (
	// suppressedUnauthenticatedEstablished: a message on an established SA, in its window,
	// at the cached Message ID of a MOBIKE SA, or an out-of-window INFORMATIONAL response.
	suppressedUnauthenticatedEstablished = "established-unauthenticated"
	// suppressedInvalidMsgIDUnauthenticated: an out-of-window request, which would draw
	// INVALID_MESSAGE_ID had it authenticated.
	suppressedInvalidMsgIDUnauthenticated = "invalid-msgid-unauthenticated"
	// suppressedHeldSuccessorUnauthenticated: a request on a held successor IKE SA.
	suppressedHeldSuccessorUnauthenticated = "held-successor-decrypt-failed"
)

// errInnerChain marks an authenticated message whose decrypted payload chain does not
// parse, as opposed to one whose Padding, Pad Length or block alignment is wrong. Both
// wrap errInnerParse. The RFC 4739 second exchange reacts to the two differently
// (handleAuthResponse), so a caller can ask errors.Is(err, errInnerChain).
var errInnerChain = fmt.Errorf("%w does not parse", errInnerParse)

// errNoSKPayload is the reason the drop line names for a message that carries no
// Encrypted payload at all.
var errNoSKPayload = errors.New("ike: no Encrypted payload")

// protectedPayload returns the Encrypted payload of msg: the FIRST one. RFC 7296 Section
// 3.14: "The Encrypted payload, if present in a message, MUST be the last payload in the
// message." wire.Message.ReadFrom does not enforce that, and walks on past an Encrypted
// payload whose Length leaves octets after it, so a chain can hold two. Every opener
// takes the same one.
func protectedPayload(msg *wire.Message) *wire.PayloadSK {
	for i := range msg.Payloads {
		if sk, ok := msg.Payloads[i].Payload.(*wire.PayloadSK); ok {
			return sk
		}
	}
	return nil
}

// openProtected opens the SK-protected message msg (raw is its octets) for sa. It is
// the one function every handler, in the handshake and on an established SA, opens such
// a message through (TestOneFunctionOpensEveryProtectedMessage), and it answers
// exactly one of three outcomes:
//
//   - opened: the integrity check verified and the inner chain parsed. inner holds it
//     (possibly empty), dropped is false and err is nil.
//   - dropped: nothing in the message proves it came from the peer. There is no
//     Encrypted payload, the message is too short for a checksum, the associated-data
//     span is out of range, or the checksum or AEAD tag fails. Any other failure before
//     the integrity check verified lands here too, never in opened. The drop is counted
//     under reason and logged at debug, and dropped is true with a nil err. Nothing of
//     sa is written and nothing is sent.
//   - authenticated and malformed: the integrity check verified and the Padding, Pad
//     Length, block alignment or inner chain is wrong. err wraps errInnerParse and the
//     cause, and also errInnerChain when the chain was the step that failed. The
//     message is the peer's own, so the caller keeps the reaction it owes the peer.
//
// RFC 7296 Section 2.4: "Since IKE is designed to operate in spite of DoS attacks from the
// network, an endpoint MUST NOT conclude that the other endpoint has failed based on any
// routing information (e.g., ICMP messages) or IKE messages that arrive without
// cryptographic protection (e.g., Notify messages complaining about unknown SPIs)."
//
// The drop writes nothing to the SA, not the state, the Message ID window, CreatedAt or
// the retransmit schedule. The half-open reaper and the initiator's retransmit loop
// therefore end a handshake the peer abandoned however many forgeries arrive meanwhile,
// and the inbound token bucket in dispatchInbound bounds how often this runs.
//
// RFC 7296 Section 2.4: "Implementations MUST limit the rate at which they take actions
// based on unprotected messages."
//
// The drop is counted, not logged at WARN: the counter is the operator's record, and a
// WARN line per forged datagram would let an off-path sender write the log.
func openProtected(sa *SA, msg *wire.Message, raw []byte, reason string, log *slog.Logger) (inner []wire.PayloadEntry, dropped bool, err error) {
	sk := protectedPayload(msg)
	// RFC 7296 Section 2.21.4: "A peer receiving such an unprotected Notify payload MUST
	// NOT respond and MUST NOT change the state of any existing SAs." A message with no
	// Encrypted payload is unprotected whatever it carries.
	if sk == nil {
		dropUnauthenticated(sa, msg, reason, errNoSKPayload, log)
		return nil, true, nil
	}
	plain, err := decryptSKPayload(sa, raw, sk)
	if err != nil {
		if errors.Is(err, errInnerParse) {
			return nil, false, err
		}
		dropUnauthenticated(sa, msg, reason, err, log)
		return nil, true, nil
	}
	inner, err = wire.ParsePayloadChain(plain, sk.InnerNextPayload)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %w", errInnerChain, err)
	}
	return inner, false, nil
}

// dropUnauthenticated counts and logs one message openProtected dropped. The line is
// debug only, for the reason openProtected gives.
func dropUnauthenticated(sa *SA, msg *wire.Message, reason string, cause error, log *slog.Logger) {
	countErrorNotifySuppressed(reason)
	sa.countUnauthenticatedDrop()
	log.Debug("ike: unauthenticated message dropped, SA kept",
		"peer", sa.PeerName, "exchange", msg.Header.ExchangeType,
		"response", msg.Header.Flags&wire.FlagResponse != 0,
		"msgid", msg.Header.MessageID, "state", sa.State, "error", cause)
}
