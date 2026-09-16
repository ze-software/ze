// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the padded path probe
// RFC: rfc/short/rfc7296.md -- Sections 2.1, 2.3, 2.23, 3.10.1, 3.14
// Related: established.go -- maintainSA, the owner loop that receives a request
// Related: dpd.go -- sendDPD, the model for reserve, build, send, arm, advance
// Related: reconcile.go -- PeerSession.probeRequests, the channel a request rides
// Related: internal/core/ikeprobe/registry.go -- the leaf this engine registers into
package engine

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"syscall"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/probe"
)

// probeRequest is one padded-exchange request on its way to the owner loop, with the
// channel the loop answers on. reply is buffered 1, so the loop never blocks on the
// answer and the caller reads it whenever it comes.
type probeRequest struct {
	req   ikeprobe.Request
	reply chan ikeprobe.Result
}

// probeState is the padded path probe that holds the SA's request window. Owned by
// the maintainSA loop, like pendingRekey, so it needs no lock. It is the probe's OWN
// correlation state: dpdState is never read or written for a probe, so a probe
// response cannot credit liveness and a DPD response cannot answer a probe.
//
// The bytes of the request live in sa.requestMsg, armed through armRequestRetransmit,
// because the DF-clear copy is the ordinary retransmission of RFC 7296 Section 2.1
// and the ordinary slot carries it.
type probeState struct {
	msgID  uint32
	octets uint16
	// dfCleared is true once a copy with Don't Fragment clear has gone out. An answer
	// read after that proves only that the fragmented copy crossed, so it is too-big.
	dfCleared bool
	// mtu is the next-hop figure a router's Fragmentation Needed, or the kernel's
	// own cache, reported for the DF copy. 0 when none was read.
	mtu   uint32
	reply chan ikeprobe.Result
}

// matches reports whether an authenticated INFORMATIONAL response at msgID answers
// this probe. A nil probe matches nothing.
func (p *probeState) matches(msgID uint32) bool {
	return p != nil && p.msgID == msgID
}

// The datagram a probe pads. Every figure is in octets.
//
//	 0                   1                   2                   3
//	+-------------------------------+-------------------------------+
//	| IPv4 header              20   | UDP header                 8  |
//	+-------------------------------+-------------------------------+
//	| non-ESP marker 4 (port 4500 only, RFC 3948 Section 2.2)       |
//	+---------------------------------------------------------------+
//	| IKE header 28 (RFC 7296 Section 3.1)                          |
//	+---------------------------------------------------------------+
//	| SK generic header 4 | IV (16 CBC, 8 AEAD)                     |
//	+---------------------------------------------------------------+
//	| Notify generic header 4 | Protocol ID 1 | SPI Size 1 | Type 2 |
//	+---------------------------------------------------------------+
//	| Notification Data  (the padding, probeNotifyOctets long)      |
//	+---------------------------------------------------------------+
//	| SK Padding (CBC: to the 16-octet block) | Pad Length 1        |
//	+---------------------------------------------------------------+
//	| ICV (CBC: the truncated integrity tag; AEAD: the AEAD tag)    |
//	+---------------------------------------------------------------+
const (
	ipv4HeaderOctets = 20
	udpHeaderOctets  = 8
	// notifyFixedOctets is the Notify payload's own fixed part after its generic
	// header: Protocol ID, SPI Size and Notify Message Type (RFC 7296 Section 3.10).
	notifyFixedOctets = 4
	// skPadLengthOctets is the one-octet Pad Length that ends every Encrypted payload
	// (RFC 7296 Section 3.14).
	skPadLengthOctets = 1
	cbcBlockOctets    = 16
	skAEADIVOctets    = 8
	// probeWireCeiling bounds the whole datagram. RFC 7296 Section 2: "All IKEv2
	// implementations MUST be able to send, receive, and process IKE messages that are
	// up to 1280 octets long, and they SHOULD be able to send, receive, and process
	// messages that are up to 3000 octets long." The transport reads at most
	// MaxMsgSize, so a peer built like Ze reads nothing larger. The engine does not
	// know the interface MTU; a datagram above it is refused by the kernel with
	// EMSGSIZE on the DF copy and read as too-big, never sent oversize.
	probeWireCeiling = transport.MaxMsgSize
)

// probeNotifyOctets answers how many octets of Notification Data pad a datagram to
// the largest size the SA's suite produces at or below wireOctets, and that size,
// given whether the send path frames with the non-ESP marker. The last result is
// false when no data length reaches a size that near: the size is below the
// smallest datagram the suite produces, or above the ceiling.
//
// Under AEAD every size is reachable and the size answered is wireOctets. Under CBC
// the sizes sit on a 16-octet grid: RFC 7296 Section 3.14 makes the Padding "a
// length that makes the combination of the payloads, the Padding, and the Pad
// Length to be a multiple of the encryption block size", so the encrypted span is
// fixed to that grid and no Notification Data length breaks it. An off-grid request
// is rounded DOWN to the grid, never up: a datagram larger than the one asked for is
// the one thing a path probe must never send, while a smaller one that fits is a
// true lower bound the caller reads off the size answered.
//
// The arithmetic mirrors buildSKMessageCBCWithMsgID and buildSKMessageAEADWithMsgID
// (auth.go), which are what the bytes are then built by; answerProbeRequest checks the
// built length against the size answered here before anything leaves, so a drift
// between the two is caught there.
func probeNotifyOctets(sa *SA, natT bool, wireOctets int) (data, sent int, ok bool) {
	if wireOctets > probeWireCeiling {
		return 0, 0, false
	}
	outer := ipv4HeaderOctets + udpHeaderOctets + wire.HeaderLen + wire.GenericHeaderLen
	if natT {
		// RFC 7296 Section 2.23: "The UDP payload of all packets containing IKE
		// messages sent on port 4500 MUST begin with the prefix of four zeros".
		outer += transport.NonESPMarkerLen
	}
	notifyOctets := wire.GenericHeaderLen + notifyFixedOctets
	if sa.Proposal.Encryption.IsAEAD {
		icv, err := ikecrypto.AEADICVOctets(sa.Proposal.Encryption.ID)
		if err != nil {
			return 0, 0, false
		}
		data = wireOctets - outer - skAEADIVOctets - notifyOctets - skPadLengthOctets - icv
		if data < 0 {
			return 0, 0, false
		}
		return data, wireOctets, true
	}
	integ := int(sa.Proposal.Integrity.TruncatedLength)
	if integ == 0 {
		integ = cbcBlockOctets
	}
	encrypted := wireOctets - outer - cbcBlockOctets - integ
	if encrypted < cbcBlockOctets {
		return 0, 0, false
	}
	// Round down to the grid: the octets dropped come off the size sent.
	offGrid := encrypted % cbcBlockOctets
	encrypted -= offGrid
	return encrypted - notifyOctets - skPadLengthOctets, wireOctets - offGrid, true
}

// probePeer is the prober the engine registers into ikeprobe. It runs on the
// caller's goroutine and touches no SA state: it finds the peer's session, hands the
// request to the owner loop over the session's request channel, and waits for the
// loop's answer.
//
// A peer with no session, or whose session owns no established SA, is refused sa-down
// at once, before the channel is touched: the owner loop runs only while an SA is
// established, so a send would otherwise wait on a loop that is not there (AC-8).
// The channel is unbuffered, so at most one request is with the loop at a time.
func probePeer(ctx context.Context, req ikeprobe.Request) (ikeprobe.Result, error) {
	ps := lookupPeerSession(req.Peer)
	if ps == nil {
		return refusedProbe(ikeprobe.RefusalSADown), nil
	}
	if ps.ownedSA.Load() == nil {
		return refusedProbe(ikeprobe.RefusalSADown), nil
	}
	request := probeRequest{req: req, reply: make(chan ikeprobe.Result, 1)}
	select {
	case ps.probeRequests <- request:
	case <-ps.done:
		return refusedProbe(ikeprobe.RefusalSADown), nil
	case <-ctx.Done():
		return ikeprobe.Result{}, ctx.Err()
	}
	select {
	case result := <-request.reply:
		return result, nil
	case <-ps.done:
		return refusedProbe(ikeprobe.RefusalSADown), nil
	case <-ctx.Done():
		return ikeprobe.Result{}, ctx.Err()
	}
}

// refusedProbe is the Result of a request the engine declined for the named reason.
func refusedProbe(why ikeprobe.Refusal) ikeprobe.Result {
	return ikeprobe.Result{Outcome: ikeprobe.OutcomeRefused, Refusal: why}
}

// answerProbeRequest is the owner loop's side of a request. It runs on the maintainSA
// goroutine, the only goroutine that may build under the SA's keys and advance its
// message id. It refuses by name, or builds ONE INFORMATIONAL request carrying one
// status Notify sized to the request, in the order sendDPD uses: reserve the window,
// build, send, advance the id, arm the retransmit slot.
//
// The first copy leaves with the request's Don't Fragment mode. When the kernel refuses
// that copy against its cached path MTU (EMSGSIZE, nothing left the host), the copy
// with DF clear leaves at once: the id is spent either way, and the answer to the
// clear copy reads as too-big with the cache's figure (R-6).
func (ps *PeerSession) answerProbeRequest(sa *SA, tr *transport.UDPTransport, request probeRequest, log *slog.Logger) {
	if sa.State != StateEstablished {
		request.reply <- refusedProbe(ikeprobe.RefusalSADown)
		return
	}
	// RFC 7296 Section 2.3: "An IKE endpoint MUST wait for a response to each of its
	// messages before sending a subsequent message unless it has received a
	// SET_WINDOW_SIZE Notify message allowing multiple outstanding messages." Ze
	// declares a window of one, so a rekey, a Delete, a DPD probe or an earlier path
	// probe that holds it refuses this one by name rather than queuing it (AC-8).
	if ps.pendingRekey != nil {
		request.reply <- refusedProbe(ikeprobe.RefusalRekeyPending)
		return
	}
	if ps.rekeyHoldInForce(time.Now()) {
		request.reply <- refusedProbe(ikeprobe.RefusalRekeyHeld)
		return
	}
	if ps.pendingProbe != nil {
		request.reply <- refusedProbe(ikeprobe.RefusalWindowHeld)
		return
	}
	if !sa.requestWindowAvailable() {
		request.reply <- refusedProbe(ikeprobe.RefusalWindowHeld)
		return
	}
	out, natT := sa.sendPath(tr)
	if out == nil {
		request.reply <- refusedProbe(ikeprobe.RefusalSADown)
		return
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		request.reply <- refusedProbe(ikeprobe.RefusalSADown)
		return
	}
	// The transport is udp4 and the size budget counts a 20-octet IP header, so a
	// peer reached over IPv6 is refused by name until an IPv6 transport exists.
	if remote.IP.To4() == nil {
		request.reply <- refusedProbe(ikeprobe.RefusalFamily)
		return
	}
	asked := int(request.req.WireOctets)
	dataOctets, octets, ok := probeNotifyOctets(sa, natT, asked)
	if !ok {
		log.Debug("ike: path probe refused, the size is not one this SA can produce",
			"peer", ps.peerName, "octets", asked, "natt", natT)
		request.reply <- refusedProbe(ikeprobe.RefusalSize)
		return
	}

	if !sa.reserveRequestWindow() {
		request.reply <- refusedProbe(ikeprobe.RefusalWindowHeld)
		return
	}
	msgID := sa.NextMsgID
	// RFC 7296 Section 3.10.1: "Notify payloads with status types MAY be added to any
	// message and MUST be ignored if not recognized." The one payload of the request
	// is a status Notify of Ze's private-use type; its zeroed Notification Data is the
	// padding, and a peer that does not know the type answers the request as it
	// answers any INFORMATIONAL it does not act on: with an empty response.
	padding := &wire.PayloadNotify{
		NotifyMsgType:    wire.NotifyZePathProbePadding,
		NotificationData: make([]byte, dataOctets),
	}
	msg, err := buildEncryptedMessageEx(sa, []wire.PayloadEntry{{Payload: padding}}, msgID,
		wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Warn("ike: path probe build failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		request.reply <- refusedProbe(ikeprobe.RefusalSendFailed)
		return
	}
	built := len(msg) + ipv4HeaderOctets + udpHeaderOctets
	if natT {
		built += transport.NonESPMarkerLen
	}
	if built != octets {
		// probeNotifyOctets and the SK builders disagree, which is a Ze defect: the
		// datagram is refused rather than sent at a size nobody asked for.
		log.Warn("ike: path probe refused, the built datagram is not the requested size",
			"peer", ps.peerName, "requested", asked, "size", octets, "built", built)
		sa.releaseRequestWindow()
		request.reply <- refusedProbe(ikeprobe.RefusalSize)
		return
	}

	pending := &probeState{msgID: msgID, octets: uint16(octets), reply: request.reply}
	err = sendRawDF(sa, tr, msg, request.req.DF, log)
	if errors.Is(err, syscall.EMSGSIZE) {
		// The kernel refused the DF copy against its cached path MTU, so nothing
		// left the host. The DF-clear copy is the message's first transmission; the
		// LOCAL entry on the transport's refusal channel supplies the cache's figure.
		log.Debug("ike: path probe refused by the kernel's path cache, sending with DF clear",
			"peer", ps.peerName, "msgid", msgID, "octets", octets)
		pending.dfCleared = true
		err = sendRawDF(sa, tr, msg, probe.DFOff, log)
	}
	if err != nil {
		log.Warn("ike: path probe send failed", "peer", ps.peerName, "error", err)
		sa.releaseRequestWindow()
		request.reply <- refusedProbe(ikeprobe.RefusalSendFailed)
		return
	}
	sa.advanceMsgID()
	sa.armRequestRetransmit(msg)
	ps.pendingProbe = pending
	log.Debug("ike: sent path probe", "peer", ps.peerName, "msgid", msgID,
		"asked", asked, "octets", octets, "natt", natT, "df", pending.dfCleared)
}

// rekeyHoldInForce reports whether either TEMPORARY_FAILURE hold (RFC 7296 Section
// 2.25) is still running. The peer is then mid-rekey, and a strongSwan in IKE_REKEYED
// drops an INFORMATIONAL that is not a Delete, so a probe sent now reads as loss.
func (ps *PeerSession) rekeyHoldInForce(now time.Time) bool {
	if rekeyHeld(ps.ikeRekeyHoldUntil, now) {
		return true
	}
	return rekeyHeld(ps.childRekeyHoldUntil, now)
}

// sendProbeDFClear repeats the probe that holds the window with Don't Fragment clear.
// It is the probe's retransmission, on the ordinary schedule (serviceRequestRetransmit)
// or at once on a router's refusal (handleSizeRefusal), and it counts against the
// ordinary budget either way.
//
// RFC 7296 Section 2.1: "A retransmission from the initiator MUST be bitwise identical
// to the original request. That is, everything starting from the IKE header (the IKE
// SA initiator's SPI onwards) must be bitwise identical; items before it (such as the
// IP and UDP headers) do not have to be identical." The bytes are sa.requestMsg, the
// datagram the first copy carried; only the IP header's DF bit differs.
func (ps *PeerSession) sendProbeDFClear(sa *SA, tr *transport.UDPTransport, now time.Time, log *slog.Logger) {
	if err := sendRawDF(sa, tr, sa.requestMsg, probe.DFOff, log); err != nil {
		log.Debug("ike: path probe DF-clear copy send failed", "peer", ps.peerName, "error", err)
	}
	ps.pendingProbe.dfCleared = true
	sa.noteRequestRetransmit(now)
	log.Debug("ike: repeated the path probe with DF clear", "peer", ps.peerName,
		"msgid", sa.requestMsgID, "attempt", sa.requestAttempts)
}

// probeRefusals is the channel of size refusals on the socket the SA sends from, read
// by maintainSA beside the inbound channel. A nil channel, for an SA with no send
// path, blocks that select arm forever, which is the arm's idle state.
func probeRefusals(sa *SA, tr *transport.UDPTransport) <-chan transport.SizeRefusal {
	out, _ := sa.sendPath(tr)
	if out == nil {
		return nil
	}
	return out.Refusals()
}

// handleSizeRefusal acts on one EMSGSIZE the kernel queued for the shared socket. A
// router's Fragmentation Needed for this SA's peer sends the DF-clear copy at once
// rather than at the retransmit timer (AC-11). The event is matched on the refused
// datagram's destination, the SA's own remote endpoint, never on the router that
// answered: the socket is shared by every SA (R-4). A LOCAL entry is the kernel's own
// cache refusing the DF copy; answerProbeRequest already read that from the send's
// error, so the entry only supplies the cache's figure.
func (ps *PeerSession) handleSizeRefusal(sa *SA, tr *transport.UDPTransport, refusal transport.SizeRefusal, log *slog.Logger) {
	pending := ps.pendingProbe
	if pending == nil {
		log.Debug("ike: size refusal with no path probe outstanding, dropped",
			"peer", ps.peerName, "for", refusal.Peer, "offender", refusal.Offender)
		return
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		return
	}
	peer := remote.AddrPort()
	peer = netip.AddrPortFrom(peer.Addr().Unmap(), peer.Port())
	if refusal.Local {
		if refusal.Peer.Addr() != peer.Addr() {
			return
		}
		if pending.mtu == 0 {
			pending.mtu = refusal.MTU
		}
		return
	}
	if refusal.Peer != peer {
		log.Debug("ike: size refusal for another peer, dropped",
			"peer", ps.peerName, "for", refusal.Peer, "offender", refusal.Offender)
		return
	}
	if pending.mtu == 0 {
		pending.mtu = refusal.MTU
	}
	if pending.dfCleared {
		return
	}
	if !sa.requestOutstanding {
		return
	}
	log.Debug("ike: path probe refused by a router, sending with DF clear at once",
		"peer", ps.peerName, "msgid", pending.msgID, "mtu", refusal.MTU,
		"offender", refusal.Offender)
	ps.sendProbeDFClear(sa, tr, time.Now(), log)
}

// settleProbe answers the outstanding probe when an authenticated INFORMATIONAL
// response carries its message id. It is the probe's correlation, the sibling of
// dpdState.matchesProbe for the DPD probe, and it reads nothing of dpdState. The
// window was already freed by the arm that authenticated the response
// (answerAuthenticatedResponse, inbound.go).
//
// An answer read while only the DF copy was out proves the size crossed whole: fits.
// An answer read after a DF-clear copy went out proves only that the fragmented copy
// crossed: too-big.
func (ps *PeerSession) settleProbe(msgID uint32, log *slog.Logger) {
	pending := ps.pendingProbe
	if !pending.matches(msgID) {
		return
	}
	result := ikeprobe.Result{Outcome: ikeprobe.OutcomeFits, WireOctets: pending.octets}
	if pending.dfCleared {
		result = ikeprobe.Result{Outcome: ikeprobe.OutcomeTooBig, WireOctets: pending.octets, MTU: pending.mtu}
	}
	ps.pendingProbe = nil
	pending.reply <- result
	log.Debug("ike: path probe answered", "peer", ps.peerName, "msgid", msgID,
		"octets", pending.octets, "outcome", result.Outcome, "mtu", result.MTU)
}

// retirePendingProbe answers a probe still outstanding when the peer rekeyed the
// IKE SA it left on: the retired SA's window is forgotten with its keys, so no copy
// of the request can be answered and no retransmission can leave. The answer is
// the refusal `rekeyed`, which names the size sent so the exchange is counted, and
// the MTU diagnostic asks the same size again on the SA that replaced it (AC-10).
func (ps *PeerSession) retirePendingProbe(log *slog.Logger) {
	pending := ps.pendingProbe
	if pending == nil {
		return
	}
	ps.pendingProbe = nil
	pending.reply <- ikeprobe.Result{Outcome: ikeprobe.OutcomeRefused, Refusal: ikeprobe.RefusalRekeyed, WireOctets: pending.octets}
	log.Info("ike: path probe retired, the peer rekeyed the SA it left on", "peer", ps.peerName,
		"msgid", pending.msgID, "octets", pending.octets)
}

// failPendingProbe answers sa-failed to a probe still outstanding when the owner loop
// returns. The ordinary exit is serviceRequestWindow deeming the SA failed after the
// full retransmit budget (RFC 7296 Section 2.1) and the loop's StateDead arm tearing
// it down; every other return (a peer Delete, an operator clear, an expired lifetime)
// takes the SA away under the probe just the same, and a caller waiting on the reply
// must learn that rather than wait on a session that reconnects.
func (ps *PeerSession) failPendingProbe(log *slog.Logger) {
	pending := ps.pendingProbe
	if pending == nil {
		return
	}
	ps.pendingProbe = nil
	pending.reply <- ikeprobe.Result{Outcome: ikeprobe.OutcomeSAFailed, WireOctets: pending.octets}
	log.Info("ike: path probe unanswered, the SA is gone", "peer", ps.peerName,
		"msgid", pending.msgID, "octets", pending.octets)
}

// sendRawDF is sendRaw with the Don't Fragment mode df installed for that one
// datagram. The destination and the marker follow sendRaw exactly; only the transport
// call differs, and the error comes back to the caller because a kernel refusal
// (EMSGSIZE) is a signal the probe acts on rather than a dropped message.
func sendRawDF(sa *SA, tr *transport.UDPTransport, msg []byte, df probe.DFMode, log *slog.Logger) error {
	out, natT := sa.sendPath(tr)
	if out == nil {
		log.Warn("ike: no send path for the SA, message dropped",
			"peer", sa.PeerName, "local-port", sa.localPort)
		return errNoSendPath
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		return errNoSendPath
	}
	if natT {
		// RFC 3948 Section 2.2: IKE on port 4500 carries the four-octet non-ESP marker.
		msg = transport.AddNonESPMarker(msg)
	}
	return out.SendDF(msg, remote, df)
}

// errNoSendPath is the send failure of an SA with no socket or no resolvable peer.
var errNoSendPath = errors.New("ike: the SA has no send path")
