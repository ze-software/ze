// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE_SA_INIT retry on COOKIE and INVALID_KE_PAYLOAD
// RFC: rfc/short/rfc7296.md -- corrected retry (Sections 1.2, 2.6, 2.6.1)
// Related: cookie.go -- the responder half that issues the challenge this file answers
// Related: fsm.go -- handleSAInitResponse, which classifies the notify and calls in here
//
// RFC 7296 Section 2.21.1 authorizes acting on these two notifies, and bounds it.
//
// "In an IKE_SA_INIT exchange, any error notification causes the exchange to fail. Note
// that some error notifications such as COOKIE, INVALID_KE_PAYLOAD or
// INVALID_MAJOR_VERSION may lead to a subsequent successful exchange. Because all error
// notifications are completely unauthenticated, the recipient should continue trying for
// some time before giving up. The recipient should not immediately act based on the
// error notification unless corrective actions are defined in this specification, such
// as for COOKIE, INVALID_KE_PAYLOAD, and INVALID_MAJOR_VERSION."
//
// Both notifies are UNAUTHENTICATED. Every retry is therefore bounded, and every value
// a peer supplies passes a guard that denies on doubt before it can steer this node.
package engine

import (
	"encoding/binary"
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
)

// maxSAInitRetries bounds the IKE_SA_INIT retries of one initiator cycle, across BOTH
// causes.
//
// RFC 7296 Section 2.6: "The initiator should limit the number of cookie exchanges it
// tries before giving up, possibly using exponential back-off. An attacker can forge
// multiple cookie responses to the initiator's IKE_SA_INIT message, and each of those
// forged cookie replies will cause two packets to be sent."
//
// A retry is sent only in DIRECT answer to a received notify, never on a timer, so this
// node's send rate can never exceed the rate of notifies it receives. Three retries plus
// the retransmit and reconnect bounds in fsm.go make the exchange strictly convergent.
const maxSAInitRetries = 3

// retryCause names why an IKE_SA_INIT is being retried. Typed rather than a string, so a
// metric label and a switch cannot disagree (ai/rules/go-standards.md).
type retryCause uint8

const (
	// retryCauseNone is the invalid zero. An unset cause never drives a retry.
	retryCauseNone retryCause = iota
	retryCookie
	retryInvalidKE
)

func (c retryCause) String() string {
	switch c {
	case retryCookie:
		return "cookie"
	case retryInvalidKE:
		return "invalid-ke-payload"
	case retryCauseNone:
		return "unset"
	default:
		// The same word wire.NotifyTypeName uses for a type it does not know, so a
		// metric label reads the same way whichever side produced it.
		return "unrecognized"
	}
}

// parseInvalidKEGroup reads the Diffie-Hellman group out of an INVALID_KE_PAYLOAD body.
//
// RFC 7296 Section 1.3: the notification carries "two octets of data ... the accepted
// Diffie-Hellman group number in big endian order".
//
// It returns a bool rather than a bare group, because ipsec.DHGroup's zero means "no
// Diffie-Hellman". A bare return would make a truncated body read as a successful parse
// of group 0 (ai/rules/evidence.md). The caller MUST test the bool.
//
// It denies a body of any other length, a value that does not fit the octet-wide group
// number, and a group outside the valid range. It never narrows a wide value into a
// small one (ai/rules/protocol.md).
func parseInvalidKEGroup(data []byte) (ipsec.DHGroup, bool) {
	if len(data) != 2 {
		return 0, false
	}
	v := binary.BigEndian.Uint16(data)
	if v > 255 {
		return 0, false
	}
	g := ipsec.DHGroup(v)
	if !ipsec.ValidDHGroup(g) {
		return 0, false
	}
	return g, true
}

// groupIsProposed reports whether this node offered the named Diffie-Hellman group.
//
// This is the security guard the RFC leaves implicit. INVALID_KE_PAYLOAD is
// unauthenticated, so without it an off-path attacker who can forge one notify chooses
// this node's Diffie-Hellman group. RFC 7296 Section 1.2 writes down the companion rule
// for the cipher -- the retry "MUST again propose its full set of acceptable
// cryptographic suites ... otherwise an active attacker could trick the endpoints into
// negotiating a weaker suite" -- and assumes an initiator will not build a group it never
// offered. Ze says so explicitly.
func groupIsProposed(ikeGroup ipsec.IKEGroup, g ipsec.DHGroup) bool {
	for _, p := range ikeGroup.Proposals {
		if p.DHGroup == g {
			return true
		}
	}
	return false
}

// retrySAInit rebuilds and re-sends an IKE_SA_INIT request after a COOKIE or an
// INVALID_KE_PAYLOAD, and reports whether the retry went out.
//
// A false return leaves the SA untouched, and the caller kills it exactly as it would
// have without a retry path.
//
// It is scoped to the IKE_SA_INIT exchange alone. The CREATE_CHILD_SA rekey path sends
// its own INVALID_KE_PAYLOAD (respondIKERekey, rekey.go) and RFC 7296 Section 1.3 phrases
// the answer to it as "the initiator will probably retry", which is not a MUST. Routing
// that exchange through here would invent an obligation the RFC does not state.
func retrySAInit(
	sa *SA,
	cause retryCause,
	data []byte,
	table *SATable,
	tr *transport.UDPTransport,
	log *slog.Logger,
) bool {
	if sa == nil || cause == retryCauseNone {
		return false
	}

	// Bound first, so a forged-notify flood spends this node's budget rather than its
	// time. RFC 7296 Section 2.6 describes exactly that attack.
	sa.SAInitRetries++
	countSAInitRetry(sa.PeerName, cause)
	if sa.SAInitRetries > maxSAInitRetries {
		log.Info("ike: IKE_SA_INIT retry budget spent, giving up",
			"peer", sa.PeerName, "cause", cause.String(), "retries", sa.SAInitRetries-1)
		sa.State = StateDead
		return false
	}

	switch cause {
	case retryCookie:
		// RFC 7296 Section 2.6 MUST: the cookie is "between 1 and 64 octets in length
		// (inclusive)". This is that bound on the ECHO path. A peer that sends a
		// 600-octet cookie must not have it reflected, and the refusal names the real
		// fault rather than surfacing later as an oversized message.
		if len(data) < minCookieLen || len(data) > maxCookieLen {
			log.Warn("ike: COOKIE notification data outside the 1..64 octet bound, refusing the retry",
				"peer", sa.PeerName, "length", len(data))
			sa.State = StateDead
			return false
		}
		sa.Cookie = append([]byte(nil), data...)
		// sa.LocalNonce, sa.LocalDH and sa.InitiatorSPI are deliberately untouched.
		// RFC 7296 Section 2.6 MUST: the retry carries "all other payloads unchanged",
		// and the responder minted its cookie over the nonce it already saw.
	case retryInvalidKE:
		group, ok := parseInvalidKEGroup(data)
		if !ok {
			log.Warn("ike: INVALID_KE_PAYLOAD carries no readable group, refusing the retry",
				"peer", sa.PeerName, "length", len(data))
			sa.State = StateDead
			return false
		}
		if !groupIsProposed(sa.IKEGroup, group) {
			log.Warn("ike: INVALID_KE_PAYLOAD names a Diffie-Hellman group we never proposed, refusing the retry",
				"peer", sa.PeerName, "offered-group", uint16(group), "ike-group", sa.IKEGroup.Name)
			sa.State = StateDead
			return false
		}
		dh, err := crypto.NewDHExchange(crypto.DHGroupID(group))
		if err != nil {
			log.Warn("ike: cannot build the Diffie-Hellman group the responder named",
				"peer", sa.PeerName, "group", uint16(group), "error", err)
			sa.State = StateDead
			return false
		}
		if sa.LocalDH != nil {
			sa.LocalDH.Clear()
		}
		sa.LocalDH = dh
		// sa.LocalNonce and sa.Cookie are deliberately untouched. RFC 7296
		// Section 2.6.1's shorter exchange shows exactly this shape: a changed KEi'
		// beside a retained N(COOKIE) and an unchanged Ni.
	case retryCauseNone:
		return false
	}

	// RFC 7296 Section 2.6: "When the IKE_SA_INIT exchange does not result in the
	// creation of an IKE SA due to INVALID_KE_PAYLOAD, NO_PROPOSAL_CHOSEN, or COOKIE,
	// the responder's SPI will be zero also in the response message." A peer that
	// nonetheless set one had it copied into the SA before the notify was classified
	// (handleSAInitResponse), so the retry would claim an IKE SA that does not exist.
	if sa.ResponderSPI != ([8]byte{}) {
		if table != nil {
			table.UpdateKey(sa.ResponderSPI, [8]byte{}, sa)
		}
		sa.ResponderSPI = [8]byte{}
	}

	msg := buildSAInitRequest(sa, sa.IKEGroup)
	// Re-anchoring is not bookkeeping. RFC 7296 Section 2.15 computes the AUTH payload
	// over the first IKE_SA_INIT message, and auth.go reads sa.InitiatorSAInitMsg for
	// exactly that. A retry that left the old bytes here would pass every payload-shape
	// check and then fail authentication two messages later, as an opaque AUTH mismatch.
	sa.InitiatorSAInitMsg = msg
	sa.LastSentMsg = msg

	sa.State = StateSAInitSent
	sa.RetransmitCount = 0
	sa.RetransmitTime = time.Now().Add(retransmitBase)

	log.Info("ike: retrying IKE_SA_INIT",
		"peer", sa.PeerName, "cause", cause.String(), "attempt", sa.SAInitRetries)
	sendRaw(sa, tr, msg, log)
	return true
}
