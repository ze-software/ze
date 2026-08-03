// Design: plan/learned/1069-ipsec-13-rekey-wire.md -- RFC 7296 Section 2.3 message-ID handling
// RFC: rfc/short/rfc7296.md -- INVALID_MESSAGE_ID (Section 2.3)
// Related: inbound.go -- the out-of-window arm that calls this emitter
// Related: notify_error.go -- the token bucket this emitter reuses, and the two senders it is NOT
//
// This is a THIRD sender, and it is neither of the two notify_error.go describes.
//
// It answers nothing. RFC 7296 Section 2.3 MUST NOT: the notification
// "MUST NOT be sent in a response".
// And "the invalid request MUST NOT be acknowledged".
// The RFC then says to
// "inform the other side by initiating an INFORMATIONAL exchange".
// So this emitter RAISES A NEW REQUEST, under a Message ID of its own, and the invalid
// request is never acknowledged.
//
// The same sentence makes the sending OPTIONAL and the rate limit a MUST. The obligation
// is therefore the bound, not the emission, and each guard below is fully conformant.

package engine

import (
	"encoding/binary"
	"log/slog"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// RFC 7296 Section 2.3 MUST: "notifications of this type MUST be rate limited."
//
// The notification is a courtesy to a peer whose Message ID counter has already
// drifted. A conforming peer draws one and then resynchronizes, so one per second
// with a burst of three covers every legitimate case.
//
// The budget matches the cached-replay bucket beside it (notify_error.go) rather than
// the inbound processing limiter, because both bound what leaves this node.
const (
	invalidMsgIDNotifyRate  = 1
	invalidMsgIDNotifyBurst = 3
)

// invalidMsgIDDataLen is the length RFC 7296 Section 2.3 fixes for the Notification
// Data: "the four-octet invalid Message ID". The emitter builds a [4]byte, so no
// caller can vary it.
const invalidMsgIDDataLen = 4

// invalidMsgIDAllowed reports whether this SA can raise one more INVALID_MESSAGE_ID now.
//
// The limiter is created on first use, as cachedReplayAllowed does, so every SA built
// anywhere in the tree carries the guard and no constructor changes. Only the maintainSA
// owner loop reaches this path for a given SA, so the lazy creation needs no lock.
//
// It fails closed: a nil SA denies (ai/rules/evidence.md).
func (sa *SA) invalidMsgIDAllowed() bool {
	if sa == nil {
		return false
	}
	if sa.invalidMsgIDLimiter == nil {
		sa.invalidMsgIDLimiter = newOutboundNotifyLimiter(invalidMsgIDNotifyRate, invalidMsgIDNotifyBurst)
	}
	return sa.invalidMsgIDLimiter.allow()
}

// sendInvalidMessageID informs the peer that one of its requests carried a Message ID
// outside the window. RFC 7296 Section 2.3: "inform the other side by initiating an
// INFORMATIONAL exchange with Notification Data containing the four-octet invalid
// Message ID."
//
// The caller MUST have authenticated the offending request first. That bound keeps this
// emitter out of an off-path attacker's reach. The SPI pair and the Message ID both
// travel in the clear. An unauthenticated trigger would therefore let one forged
// datagram spend this SA's request window.
//
// Every precondition denies by sending nothing, and each records the guard that stopped
// it (ai/rules/evidence.md).
func (ps *PeerSession) sendInvalidMessageID(sa *SA, badID uint32, tr *transport.UDPTransport, log *slog.Logger) {
	if sa == nil || tr == nil {
		countErrorNotifySuppressed("invalid-msgid-no-destination")
		return
	}
	// RFC 7296 Section 2.3 MUST, and the whole obligation of this emitter.
	if !sa.invalidMsgIDAllowed() {
		log.Debug("ike: INVALID_MESSAGE_ID rate limited", "peer", ps.peerName, "msgid", badID)
		countErrorNotifySuppressed("invalid-msgid-rate-limited")
		return
	}
	// RFC 7296 Section 2.3 allows one self-initiated request at a time. This one is a
	// courtesy, so it never displaces Ze's own DPD probe, Delete or rekey. A replayed
	// request that lands out of window therefore cannot stall this SA for the whole
	// requestWindowTimeout, which the rate limit alone would not prevent.
	if !sa.reserveRequestWindow() {
		log.Debug("ike: INVALID_MESSAGE_ID dropped, a request is outstanding",
			"peer", ps.peerName, "msgid", badID)
		countErrorNotifySuppressed("invalid-msgid-window-held")
		return
	}
	var data [invalidMsgIDDataLen]byte
	binary.BigEndian.PutUint32(data[:], badID)
	// The Notify is about the IKE SA itself, so RFC 7296 Section 3.10 leaves the
	// Protocol ID at 0 and the SPI empty.
	//
	// initiatorFlag rather than a literal, and no FlagResponse. Section 3.1 ties the I
	// bit to the original initiator of the IKE SA. The cleared Response flag makes this
	// a new exchange, and not an acknowledgement of the invalid request.
	notify := &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifyInvalidMessageID,
		NotificationData: data[:],
	}
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: notify}},
		sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: INVALID_MESSAGE_ID build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		countErrorNotifySuppressed("invalid-msgid-build-failed")
		return
	}
	sa.advanceMsgID()
	// The datagram is kept BEFORE it goes out. advanceMsgID has already spent the id it
	// carries, and RFC 7296 Section 2.1 repeats a request under its own Message ID. This
	// emitter has no retransmission machine of its own, unlike the rekey and the DPD
	// probe.
	//
	// Without the copy, a lost notify leaves NextMsgID one past the id the peer still
	// expects. Every later request on this SA then falls outside the peer's window of
	// one, and the SA stalls until the liveness budget ends it.
	//
	// The repeats are bounded by maxRequestRetransmits, and only the owner loop makes
	// them. A peer replaying a captured request therefore cannot turn this courtesy into
	// an amplifier. The rate limit above bounds how often a notify is RAISED, and the
	// request window holds it to one outstanding at a time.
	sa.armRequestRetransmit(msg)
	sendRaw(sa, tr, msg, log)
	countErrorNotifySent(wire.NotifyInvalidMessageID, true)
	log.Debug("ike: informed the peer of an out-of-window Message ID",
		"peer", ps.peerName, "invalid-msgid", badID)
}
