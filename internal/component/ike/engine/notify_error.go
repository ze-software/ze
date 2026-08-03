// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- error notification emission
// RFC: rfc/short/rfc7296.md -- error handling (Sections 2.21.2, 2.21.3, 2.21.4)
// Related: inbound.go -- the authenticated call sites that answer a refused request
// Related: register.go -- the out-of-SA dispatch branch that answers an unknown SPI
//
// Two senders live here and they MUST NEVER be merged into one helper.
//
// The protected sender answers a request that arrived inside an authenticated IKE SA.
// It holds the SA and it encrypts under SK_e.
// The caller caches its bytes for RFC 7296 Section 2.1 retransmission.
// It needs no rate limit, because an authenticated peer already paid for the exchange.
//
// The unprotected sender answers a datagram that matched no SA.
// RFC 7296 Section 2.21.4 requires the answer to carry no cryptographic protection.
// The function therefore takes no SA and reaches no key material.
// It sends to the observed source address, which an attacker chooses.
// Each emission passes a rate limiter first.
//
// A merged helper that takes an optional SA is the shape that eventually protects the
// unprotected answer. It can also leak key material into the out-of-SA path.
package engine

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// RFC 7296 Section 2.21.4: "A node needs to limit the rate at which it will send
// messages in response to unprotected messages."
//
// The budget is two orders of magnitude tighter than the inbound processing limiter
// (100/s, burst 200, register.go).
// Each datagram this limiter gates is unauthenticated, by definition.
// The legitimate rate is near zero.
// A peer that really crashed sends one request, not a hundred.
const (
	unprotectedNotifyRate  = 1
	unprotectedNotifyBurst = 5
)

// RFC 7296 Section 2.1 bounds a legitimate retransmission burst.
// The cached IKE_AUTH response is several hundred octets.
// The observed source address selects its destination.
// An unbounded replay is therefore an amplifier.
// Three replays per second cover each retransmission schedule the RFC describes.
const (
	cachedReplayRate  = 1
	cachedReplayBurst = 3
)

// outboundNotifyLimiter is a token bucket that bounds how many messages this node
// sends in answer to traffic it has not authenticated.
type outboundNotifyLimiter struct {
	mu     sync.Mutex
	tokens float64
	max    float64
	rate   float64
	last   time.Time
}

// newOutboundNotifyLimiter builds a bucket for one named budget.
//
// The perSecond parameter names the rate RFC 7296 Section 2.21.4 asks for.
// A change to either budget therefore stays a one-constant edit.
// It does not become a signature change.
//
//nolint:unparam // Both budgets run at 1/s today, so perSecond takes one value.
func newOutboundNotifyLimiter(perSecond, burst float64) *outboundNotifyLimiter {
	return &outboundNotifyLimiter{
		tokens: burst,
		max:    burst,
		rate:   perSecond,
		last:   time.Now(),
	}
}

// allow reports whether one more message can go out now.
// It spends a token when it does.
//
// It fails closed. A nil limiter denies.
// A caller that forgot to construct one therefore sends nothing.
// It does not send without a bound (ai/rules/evidence.md).
func (l *outboundNotifyLimiter) allow() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if elapsed := now.Sub(l.last); elapsed > 0 {
		l.tokens = min(l.tokens+elapsed.Seconds()*l.rate, l.max)
		l.last = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens--
	return true
}

// cachedReplayAllowed reports whether this SA can replay its cached response to an
// observed source address once more now.
//
// The limiter is created on first use, rather than at SA construction.
// Each SA built anywhere in the tree therefore carries the guard, and no constructor
// changes. The owner loop and the pre-adoption dispatch path never run for one SA at
// the same time. The lazy creation therefore needs no lock.
func (sa *SA) cachedReplayAllowed() bool {
	if sa == nil {
		return false
	}
	if sa.cachedReplayLimiter == nil {
		sa.cachedReplayLimiter = newOutboundNotifyLimiter(cachedReplayRate, cachedReplayBurst)
	}
	return sa.cachedReplayLimiter.allow()
}

// buildErrorNotifyResponse builds the SK-encrypted response that answers a request Ze
// refused inside an authenticated IKE SA. RFC 7296 Section 2.21.3 MUST: "After the IKE
// SA is authenticated, all requests having errors MUST result in a response notifying
// the other end of the error."
//
// It never sends. The caller MUST cacheResponse then sendRaw.
// A retransmitted request then draws the same bytes back (RFC 7296 Section 2.1).
// The owner loop also stays the single writer of SA state.
//
// The Response flag is always set here.
// That is what stops an error notify from starting a new exchange.
// RFC 7296 Section 2.21.3 answers a request.
// It never sends an error as a fresh INFORMATIONAL request.
// This emitter therefore cannot begin a loop.
func buildErrorNotifyResponse(sa *SA, msgID uint32, exchange uint8, notifyType uint16, data []byte) ([]byte, error) {
	if sa == nil {
		return nil, errors.New("ike: error notify needs an SA")
	}
	notify := &wire.PayloadNotify{NotifyMsgType: notifyType, NotificationData: data}
	inner := []wire.PayloadEntry{{Payload: notify}}
	// initiatorFlag rather than a literal. RFC 7296 Section 3.1 ties the I bit to the
	// original initiator of the IKE SA, not to the sender of this message.
	return buildEncryptedMessageEx(sa, inner, msgID, exchange, initiatorFlag(sa)|wire.FlagResponse)
}

// respondError answers a refused request on an authenticated IKE SA with one error
// notify. It caches the bytes for retransmission.
// It then sends them to the configured peer.
//
// The three-step tail repeats at each refusal site, so it lives here once.
//
// INVALID_SYNTAX ALSO ENDS THIS NODE'S OWN SA, and that is the fourth step.
// RFC 7296 Section 2.21.3 (rfc/full/rfc7296.txt:3341-3345): when a peer returns an
// INVALID_SYNTAX notification, "this error notification is considered fatal in both
// peers, meaning that the IKE SA is deleted without needing an explicit Delete payload."
// The peer discards the SA on receipt, so a node that sends one and keeps its own half is
// encrypting to nobody until dead-peer detection notices.
//
// The rule is attached to the notify TYPE, not to a call site, so it lives at the one
// authenticated-path emitter rather than at each refusal. Every site reaches it:
// respondInnerParseError, the malformed-Delete pre-scan, and the two rekey refusals
// notifyForRefusal maps to INVALID_SYNTAX (inbound.go).
//
// UNSUPPORTED_CRITICAL_PAYLOAD is deliberately NOT fatal here. Section 2.21.2 puts it in
// the set that deletes an SA without a Delete payload only "in an IKE_AUTH exchange, or in
// the INFORMATIONAL exchange immediately following it". Section 2.21.3, which governs an
// authenticated SA, names INVALID_SYNTAX alone.
func (ps *PeerSession) respondError(
	sa *SA, msgID uint32, exchange uint8, notifyType uint16, data []byte,
	tr *transport.UDPTransport, log *slog.Logger,
) {
	resp, err := buildErrorNotifyResponse(sa, msgID, exchange, notifyType, data)
	if err != nil {
		log.Debug("ike: error notify build failed", "peer", ps.peerName,
			"notify", wire.NotifyTypeName(notifyType), "error", err)
		countErrorNotifySuppressed("build-failed")
		return
	}
	cacheResponse(sa, msgID, resp)
	sendRaw(sa, tr, resp, log)
	countErrorNotifySent(notifyType, true)
	log.Info("ike: answered a refused request with an error notify", "peer", ps.peerName,
		"notify", wire.NotifyTypeName(notifyType), "msgid", msgID)
	// After the send, so the peer still receives the notification that tells it to do the
	// same. cacheResponse above keeps the bytes, so a retransmission of the same request is
	// still answered until the owner loop reads StateDead on its next tick
	// (established.go). Every caller runs on that loop's goroutine, so this needs no lock.
	if notifyType == wire.NotifyInvalidSyntax {
		sa.State = StateDead
		log.Warn("ike: answered with INVALID_SYNTAX, which deletes the IKE SA at both ends",
			"peer", ps.peerName, "msgid", msgID)
	}
}

// errMalformedRequest marks a request Ze refused because the message itself was wrong.
// It does not mark a well-formed ask Ze cannot satisfy.
// The two get different notify types.
// The producer that knows which one it saw therefore records it.
var errMalformedRequest = errors.New("ike: malformed request")

// notifyForRefusal maps a refusal to the notify type that reports it.
//
// RFC 7296 Section 3.10.1 makes INVALID_SYNTAX the answer when
// "some type, length, or value was out of range".
// It also names NO_PROPOSAL_CHOSEN as the
// "generic Child SA error when Child SA cannot be created for some other reason".
// A well-formed request that Ze cannot satisfy is the second case.
// Three causes reach it:
//   - a proposal that matched none of ours
//   - an empty local configuration
//   - a dataplane that refused the install
//
// It fails safe rather than closed, and the direction is deliberate.
// An unclassified error reports NO_PROPOSAL_CHOSEN, which tells the peer nothing.
// RFC 7296 Section 3.10.1 asks for exactly that:
// "To avoid leaking information to someone probing a node".
func notifyForRefusal(err error) uint16 {
	if errors.Is(err, errMalformedRequest) {
		return wire.NotifyInvalidSyntax
	}
	// RFC 7296 Section 2.9: "If the responder's policy does not allow it to accept any
	// part of the proposed Traffic Selectors, it responds with a TS_UNACCEPTABLE Notify
	// message." That is a NAMED answer rather than the generic one, so it is mapped
	// before the fallback below.
	if errors.Is(err, errTSUnacceptable) {
		return wire.NotifyTSUnacceptable
	}
	return wire.NotifyNoProposalChosen
}

// sendInvalidIKESPI answers a datagram that matched no IKE SA. RFC 7296 Section
// 2.21.4.
//
// The signature carries the guarantee. There is no SA and no PeerSession, so no key
// material is reachable from any argument and the answer cannot be cryptographically
// protected. That makes "the response MUST NOT be cryptographically protected" a
// property of the function rather than of a branch inside it.
//
// It never parses the payload chain.
// The 28-byte header holds each field the answer copies.
// A parse of an attacker's payloads, to decide whether to answer them, is a larger
// attack surface for no gain.
//
// Every precondition denies by sending nothing (ai/rules/evidence.md).
func sendInvalidIKESPI(
	tr *transport.UDPTransport,
	pkt transport.Packet,
	hdr wire.Header,
	natT bool,
	limiter *outboundNotifyLimiter,
	log *slog.Logger,
) {
	if tr == nil || pkt.RemoteAddr == nil {
		countErrorNotifySuppressed("no-destination")
		return
	}
	// RFC 7296 Section 2.21.4 MUST NOT: "If the message is marked as a response, the
	// node can audit the suspicious event but MUST NOT respond." This is the guard that
	// makes the emitter a fixed point. Its own output carries the Response flag, so
	// feeding that output back here emits nothing and two nodes cannot ping-pong.
	if hdr.Flags&wire.FlagResponse != 0 {
		log.Debug("ike: out-of-SA response audited, not answered",
			"src", pkt.RemoteAddr, "ispi", SPIHex(hdr.InitiatorSPI))
		countErrorNotifySuppressed("marked-response")
		return
	}
	// RFC 7296 Section 2.21.4 scopes the answer to a message received
	// "outside the context of an IKE SA known to it (and the message is not a request to start an IKE SA)".
	// The exchange type is checked here.
	// It is not inferred from a refusal by tryResponderSAInit.
	// That function also refuses an IKE_SA_INIT from an unconfigured source, and one
	// that carries a non-zero responder SPI.
	if hdr.ExchangeType == wire.ExchangeIKESAInit {
		countErrorNotifySuppressed("sa-init")
		return
	}
	if !limiter.allow() {
		log.Debug("ike: out-of-SA notify rate limited", "src", pkt.RemoteAddr)
		countErrorNotifySuppressed("rate-limited")
		return
	}

	// RFC 7296 Section 1.5 gives the construction rule. The answer
	// "is sent to the IP address and port from whence it came with the same IKE SPIs".
	// The
	// "Message ID and Exchange Type are copied from the request".
	// The
	// "Response flag is set to 1, and the version flags are set in the normal fashion".
	// The notification carries no data.
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: hdr.InitiatorSPI,
			ResponderSPI: hdr.ResponderSPI,
			MajorVersion: 2,
			MinorVersion: 0,
			ExchangeType: hdr.ExchangeType,
			Flags:        wire.FlagResponse,
			MessageID:    hdr.MessageID,
		},
		Payloads: []wire.PayloadEntry{{Payload: &wire.PayloadNotify{
			NotifyMsgType: wire.NotifyInvalidIKESPI,
		}}},
	}
	buf := make([]byte, wire.HeaderLen+64)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		log.Debug("ike: INVALID_IKE_SPI build failed", "src", pkt.RemoteAddr, "error", err)
		countErrorNotifySuppressed("build-failed")
		return
	}
	out := buf[:n]
	if natT {
		// RFC 3948 Section 2.2: IKE on port 4500 carries the four-octet non-ESP marker.
		out = transport.AddNonESPMarker(out)
	}
	if err := tr.Send(out, pkt.RemoteAddr); err != nil {
		log.Debug("ike: send INVALID_IKE_SPI failed", "src", pkt.RemoteAddr, "error", err)
		countErrorNotifySuppressed("send-failed")
		return
	}
	countErrorNotifySent(wire.NotifyInvalidIKESPI, false)
	log.Debug("ike: answered an unknown SPI with INVALID_IKE_SPI",
		"src", pkt.RemoteAddr, "ispi", SPIHex(hdr.InitiatorSPI), "rspi", SPIHex(hdr.ResponderSPI))
}

// answerOutOfSA reads the header of a datagram that matched no SA and answers it.
// The dispatch loops call this after tryResponderSAInit declines.
// natT selects port 4500 framing.
func answerOutOfSA(tr *transport.UDPTransport, pkt transport.Packet, natT bool, limiter *outboundNotifyLimiter, log *slog.Logger) {
	var hdr wire.Header
	if err := hdr.ReadFrom(pkt.Data); err != nil {
		countErrorNotifySuppressed("short-header")
		return
	}
	sendInvalidIKESPI(tr, pkt, hdr, natT, limiter, log)
}

// carriesSKPayload reports whether an outer message holds an Encrypted payload.
//
// RFC 7296 Section 1.4 makes every post-IKE_AUTH exchange protected, so a genuine
// retransmission of such a request carries SK by construction. The test is structural,
// needs no key material, and costs one pass over the payload list.
//
// It fails closed. A nil message reads false (ai/rules/evidence.md).
func carriesSKPayload(msg *wire.Message) bool {
	if msg == nil {
		return false
	}
	for i := range msg.Payloads {
		if _, ok := msg.Payloads[i].Payload.(*wire.PayloadSK); ok {
			return true
		}
	}
	return false
}

// unrecognizedErrorNotify returns the first notify in a payload chain whose type
// reports an error this implementation does not recognize.
//
// RFC 7296 Section 3.10.1 MUST:
// "An implementation receiving a Notify payload with one of these types that it does not recognize in a response MUST assume that the corresponding request has failed entirely".
// The caller applies that verdict. This helper only finds the type.
//
// A recognized error type, a status type, and an empty chain all report false.
// The zero return can therefore never read as a real notify type.
func unrecognizedErrorNotify(inner []wire.PayloadEntry) (uint16, bool) {
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if wire.NotifyIsError(n.NotifyMsgType) && !wire.NotifyTypeRecognized(n.NotifyMsgType) {
			return n.NotifyMsgType, true
		}
	}
	return 0, false
}

// errUnrecognizedNotify reports that a response carried an error notify this
// implementation does not recognize, so the request it answers failed entirely.
var errUnrecognizedNotify = errors.New("ike: response carries an unrecognized error notify")

// failIfUnrecognizedErrorNotify applies RFC 7296 Section 3.10.1 to a response. It
// returns an error when the chain carries an unrecognized error type, and nil
// otherwise. An unrecognized error type in a REQUEST, and any status type, are ignored
// here and logged by the caller that walks the chain.
func failIfUnrecognizedErrorNotify(inner []wire.PayloadEntry, peer string, log *slog.Logger) error {
	t, found := unrecognizedErrorNotify(inner)
	if !found {
		return nil
	}
	log.Warn("ike: response carries an unrecognized error notify, treating the request as failed",
		"peer", peer, "notify-type", t)
	return errUnrecognizedNotify
}

// logIgnoredNotifies records the notify types a message carried that this
// implementation does not act on. RFC 7296 Section 3.10.1 MUST:
// "Unrecognized error types in a request and status types in a request or response MUST be ignored".
// The same sentence asks, at SHOULD level, for a log of each one.
func logIgnoredNotifies(inner []wire.PayloadEntry, peer string, isResponse bool, log *slog.Logger) {
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if wire.NotifyTypeRecognized(n.NotifyMsgType) {
			continue
		}
		if wire.NotifyIsError(n.NotifyMsgType) && isResponse {
			// The response path fails the whole request instead of ignoring it.
			continue
		}
		log.Info("ike: ignoring an unrecognized notify", "peer", peer,
			"notify-type", n.NotifyMsgType, "response", isResponse)
	}
}
