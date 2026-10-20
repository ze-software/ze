// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the out-of-SA path and the protected notice over the peer's SA
// RFC: rfc/short/rfc7296.md -- Error Handling Outside IKE SA (Section 2.21.4)
// Related: notify_error.go -- the unprotected INVALID_IKE_SPI the same datagram can draw, and the token bucket
// Related: notify_invalid_msgid.go -- the other emitter that raises a NEW INFORMATIONAL request
// Related: register.go -- the two dispatch loops that call noteSuspicious
//
// RFC 7296 Section 2.21.4 SHOULD: "A node receiving a suspicious message from an IP
// address (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send
// an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA."
//
// The suspicious message this file acts on is the one the same section describes two
// paragraphs earlier: a REQUEST "on UDP port 500 or 4500 outside the context of an IKE
// SA known to it (and the message is not a request to start an IKE SA)". The Notify is
// the type that section defines for it, INVALID_IKE_SPI, which "indicates an IKE message
// was received with an unrecognized destination SPI". It goes out protected, as a new
// INFORMATIONAL request over the SA the peer's address already holds.
//
// A message marked as a response draws nothing here. The section's own MUST NOT ("If
// the message is marked as a response, the node can audit the suspicious event but MUST
// NOT respond") and the MUST NOT on a peer receiving an unprotected INVALID_IKE_SPI
// ("MUST NOT respond") both cover it, and a response is exactly what that unprotected
// notification is.
//
// The work is split across two goroutines. The dispatch goroutine reads only immutable
// session config and an atomic pointer, so it picks the candidate session by its
// configured remote address. The owner loop (maintainSA) then compares the datagram's
// source with the SA's own endpoint, which is owner-loop state, and builds the message.

package engine

import (
	"log/slog"
	"net"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// suspiciousQueueDepth bounds the per-session queue of suspicious messages waiting for
// the owner loop. The rate limit below lets at most suspiciousNotifyBurst through at
// once, so a deeper queue would only hold datagrams the bucket then refuses.
const suspiciousQueueDepth = 4

// RFC 7296 Section 2.21.4: "A node needs to limit the rate at which it will send
// messages in response to unprotected messages."
//
// Every trigger is unauthenticated, so the bound is what makes this emitter safe. It is
// one notification per second per SA with a burst of three, and three facts hold the
// amplification at or below one.
//
//   - The destination is the SA's own authenticated endpoint, never the datagram's
//     source, so a spoofed source cannot aim the notification at a third party.
//   - Each trigger draws at most one notification of a fixed small size, and the bucket
//     caps how many triggers draw one at all.
//   - The SA's request window of one admits it only while no request of ours is
//     outstanding, so it never displaces a liveness probe, a Delete or a rekey.
//
// The budget matches the INVALID_MESSAGE_ID bucket (notify_invalid_msgid.go), the other
// emitter that raises a request over the SA.
const (
	suspiciousNotifyRate  = 1
	suspiciousNotifyBurst = 3
)

// noteSuspicious hands a datagram that matched no IKE SA to the owner loop of the
// session whose peer sent it. The dispatch loops call it after tryResponderSAInit
// declines, on the dispatch goroutine.
//
// It never blocks: a full queue drops the datagram and counts it. Every precondition
// denies by forwarding nothing, and each records the guard that stopped it
// (ai/rules/evidence.md).
func noteSuspicious(pkt transport.Packet) {
	// The callers have already checked the 28-octet header length.
	if len(pkt.Data) < wire.HeaderLen {
		countErrorNotifySuppressed("suspicious-short-header")
		return
	}
	if pkt.RemoteAddr == nil {
		countErrorNotifySuppressed("suspicious-no-source")
		return
	}
	// RFC 7296 Section 2.21.4 MUST NOT: "If the message is marked as a response, the
	// node can audit the suspicious event but MUST NOT respond." Header octet 19 holds
	// the flags (Section 3.1).
	if pkt.Data[19]&wire.FlagResponse != 0 {
		countErrorNotifySuppressed("suspicious-marked-response")
		return
	}
	// RFC 7296 Section 2.21.4 scopes the event to a message that "is not a request to
	// start an IKE SA". Header octet 18 holds the Exchange Type.
	if pkt.Data[18] == wire.ExchangeIKESAInit {
		countErrorNotifySuppressed("suspicious-sa-init")
		return
	}
	ps := establishedSessionFrom(pkt.RemoteAddr.IP)
	if ps == nil {
		countErrorNotifySuppressed("suspicious-no-sa")
		return
	}
	select {
	case ps.suspicious <- pkt:
	default:
		countErrorNotifySuppressed("suspicious-queue-full")
	}
}

// establishedSessionFrom returns the running session that owns an established IKE SA
// and talks to this source address, or nil.
//
// RFC 7296 Section 2.21.4 SHOULD: "A node receiving a suspicious message from an IP
// address (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send
// an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA." The address an
// SA has is the one it talks to now. After NAT rebinding or a MOBIKE address update that
// is no longer the configured remote, so a session is a candidate when either the
// configured remote or the endpoint its owner loop published (saEndpoint) is the source.
//
// It reads the immutable session config under the peers lock, as matchResponderPeer
// does, and the atomic ownedSA and saEndpoint pointers. The owner loop makes the exact
// endpoint comparison, port included (fromSAEndpoint), because the SA's observed
// endpoint is its state alone; this is the candidate filter in front of it.
func establishedSessionFrom(source net.IP) *PeerSession {
	configured := source.String()
	peersMu.RLock()
	defer peersMu.RUnlock()
	for _, ps := range activePeersMap {
		if ps.suspicious == nil {
			continue
		}
		if ps.ownedSA.Load() == nil {
			continue
		}
		if ps.peerCfg.RemoteAddress == configured {
			return ps
		}
		if endpoint := ps.saEndpoint.Load(); endpoint != nil && endpoint.IP.Equal(source) {
			return ps
		}
	}
	return nil
}

// publishSAEndpoint makes the endpoint sa talks to now visible to the dispatch
// goroutine (establishedSessionFrom). It MUST be called on the owner loop, which alone
// reads sa.peerEndpoint. sa.remoteUDPAddr returns a copy, so the published value never
// aliases owner-loop state. A nil answer (an unresolvable configured remote) is
// published as nil, which leaves the configured address as the only match.
func (ps *PeerSession) publishSAEndpoint(sa *SA) {
	ps.saEndpoint.Store(sa.remoteUDPAddr())
}

// suspiciousNotifyAllowed reports whether this SA can raise one more INVALID_IKE_SPI
// notification over itself now.
//
// The limiter is created on first use, as invalidMsgIDAllowed does, so every SA built
// anywhere in the tree carries the guard and no constructor changes. Only the maintainSA
// owner loop reaches this path for a given SA, so the lazy creation needs no lock.
//
// It fails closed: a nil SA denies (ai/rules/evidence.md).
func (sa *SA) suspiciousNotifyAllowed() bool {
	if sa == nil {
		return false
	}
	if sa.suspiciousNotifyLimiter == nil {
		sa.suspiciousNotifyLimiter = newOutboundNotifyLimiter(suspiciousNotifyRate, suspiciousNotifyBurst)
	}
	return sa.suspiciousNotifyLimiter.allow()
}

// fromSAEndpoint reports whether a datagram came from the endpoint this SA talks to.
//
// RFC 7296 Section 2.21.4 names "an IP address (and port, if NAT traversal is used)".
// The address always has to match. The port has to match only once the SA has floated
// to port 4500, because only then does a NAT's port translation identify the peer.
func (sa *SA) fromSAEndpoint(pkt transport.Packet) bool {
	if pkt.RemoteAddr == nil {
		return false
	}
	endpoint := sa.remoteUDPAddr()
	if endpoint == nil {
		return false
	}
	if !endpoint.IP.Equal(pkt.RemoteAddr.IP) {
		return false
	}
	if sa.localPort != transport.NATTPort {
		return true
	}
	return endpoint.Port == pkt.RemoteAddr.Port
}

// informSuspicious tells the peer, over the SA its address already holds, that a
// message from that address named an IKE SA this node does not know.
//
// RFC 7296 Section 2.21.4: "A node receiving a suspicious message from an IP address
// (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send an IKE
// Notify payload in an IKE INFORMATIONAL exchange over that SA."
//
// It runs on the owner loop. It changes no SA state beyond the message id and the
// request window its own request takes: the section's next sentence, "The recipient
// MUST NOT change the state of any SAs as a result", binds the peer, and this node
// keeps to it as well. Every precondition denies by sending nothing, and each records
// the guard that stopped it (ai/rules/evidence.md).
func (ps *PeerSession) informSuspicious(sa *SA, pkt transport.Packet, tr *transport.UDPTransport, log *slog.Logger) {
	if sa == nil || tr == nil {
		countErrorNotifySuppressed("suspicious-no-destination")
		return
	}
	if !sa.fromSAEndpoint(pkt) {
		log.Debug("ike: suspicious message came from another endpoint than the SA's",
			"peer", ps.peerName, "src", pkt.RemoteAddr)
		countErrorNotifySuppressed("suspicious-other-endpoint")
		return
	}
	// RFC 7296 Section 2.21.4: "A node needs to limit the rate at which it will send
	// messages in response to unprotected messages."
	if !sa.suspiciousNotifyAllowed() {
		log.Debug("ike: suspicious-message notification rate limited", "peer", ps.peerName)
		countErrorNotifySuppressed("suspicious-rate-limited")
		return
	}
	// RFC 7296 Section 2.3 allows one self-initiated request at a time. This one is a
	// courtesy, so it never displaces Ze's own DPD probe, Delete or rekey.
	if !sa.reserveRequestWindow() {
		log.Debug("ike: suspicious-message notification dropped, a request is outstanding",
			"peer", ps.peerName)
		countErrorNotifySuppressed("suspicious-window-held")
		return
	}
	// The Notify names no SPI and carries no data, exactly as the unprotected
	// INVALID_IKE_SPI does (notify_error.go). RFC 7296 Section 3.10 leaves the Protocol
	// ID at 0 and the SPI empty for a notification that names no SA by SPI.
	//
	// initiatorFlag rather than a literal, and no FlagResponse: Section 3.1 ties the I
	// bit to the original initiator of the IKE SA, and the cleared Response flag makes
	// this a new exchange.
	notify := &wire.PayloadNotify{NotifyMsgType: wire.NotifyInvalidIKESPI}
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: notify}},
		sa.NextMsgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Debug("ike: suspicious-message notification build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		countErrorNotifySuppressed("suspicious-build-failed")
		return
	}
	sa.advanceMsgID()
	// Kept before it goes out, for the reason sendInvalidMessageID gives: advanceMsgID
	// spent the id, and RFC 7296 Section 2.1 repeats a request under its own Message ID.
	// The owner loop makes the bounded repeats, so a lost notification cannot leave
	// NextMsgID one past the id the peer expects.
	sa.armRequestRetransmit(msg)
	sendRaw(sa, tr, msg, log)
	countErrorNotifySent(wire.NotifyInvalidIKESPI, true)
	log.Debug("ike: informed the peer over its SA of a message naming an unknown IKE SA",
		"peer", ps.peerName, "src", pkt.RemoteAddr)
}
