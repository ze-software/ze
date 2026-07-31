// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE SA finite state machine
// RFC: rfc/short/rfc7296.md -- IKE_SA_INIT and IKE_AUTH exchanges (Sections 1.2, 2.4)
// Related: sa_init_retry.go -- the corrected IKE_SA_INIT retry handleSAInitResponse calls into
package engine

import (
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errStopped        = errors.New("ike: stopped")
	errAuthFailed     = errors.New("ike: authentication failed")
	errTimeout        = errors.New("ike: retransmit timeout")
	errInvalidMessage = errors.New("ike: invalid message")
	// errSAExpired reports a refusal to protect a message with an IKE SA whose
	// negotiated lifetime has run out (RFC 7296 Section 2.8).
	errSAExpired = errors.New("ike: security association lifetime expired")
)

const (
	nonceLen           = 32
	retransmitBase     = 500 * time.Millisecond
	retransmitMax      = 60 * time.Second
	maxRetransmissions = 7
	reconnectBase      = 1 * time.Second
	reconnectMaxDelay  = 60 * time.Second
	// responderHandshakeTimeout bounds a half-open responder handshake. A peer that
	// sends IKE_SA_INIT, receives our response, then abandons (crash/restart/partition)
	// before IKE_AUTH would otherwise pin responderBusy and the SATable entry forever,
	// wedging every future IKE_SA_INIT from that peer. The initiator self-heals via
	// maxRetransmissions; this is the responder's equivalent. RFC 7296 Section 2.4.
	responderHandshakeTimeout = 30 * time.Second
)

// afterFunc returns a channel that fires after the given duration.
var afterFunc = time.After

// reconnectDelay computes exponential backoff for peer reconnection.
func reconnectDelay(ps *PeerSession) time.Duration {
	if ps.sa == nil {
		return reconnectBase
	}
	attempt := ps.sa.RetransmitCount
	if attempt <= 0 {
		return reconnectBase
	}
	shift := min(attempt-1, 6)
	d := reconnectBase * time.Duration(1<<uint(shift))
	if d > reconnectMaxDelay {
		return reconnectMaxDelay
	}
	return d
}

// runOnce executes a single IKE SA lifecycle for a peer session.
func (ps *PeerSession) runOnce(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	// Not yet owned by maintainSA (ownedSA is nil until runEstablished adopts an SA):
	// (re)handshake packets are handled inline on the dispatch goroutine, not routed
	// to the owner loop.
	if peer.ConnectionType == ipsec.ConnectionInitiate {
		return ps.runInitiator(peer, ikeGroup, table, tr, bus, log)
	}
	return ps.runResponder(peer, ikeGroup, table, tr, bus, log)
}

// runInitiator drives the initiator side of the IKE exchange.
func (ps *PeerSession) runInitiator(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	sa, err := newInitiatorSA(ps.peerName, peer, ikeGroup, ps.espGroup)
	if err != nil {
		return fmt.Errorf("ike: create SA: %w", err)
	}
	// Give the SA both sockets before anything sends. RFC 7296 Section 2.23 MUST: an
	// endpoint that discovers a NAT "MUST send all subsequent traffic from port 4500",
	// and sa.sendPath needs that socket in hand at the moment the verdict lands.
	sa.bindSockets(tr, ps.natt)
	ps.setSA(sa)

	// RFC 7296 Section 2.6: initiator inserts SA with zero responder SPI initially.
	table.Insert(sa)
	// The entry lives exactly as long as this cycle. Without the removal a failed
	// cycle leaves an established-looking SA behind. Every later packet still reaches
	// it, and the IPsec metrics still count it as a live tunnel.
	//
	// The removal names the PEER, not an SPI pair. Go evaluates the arguments of a
	// deferred CALL where the defer is written. A deferred removal by SPI pair
	// therefore captures the responder SPI as the zero newInitiatorSA left. It then
	// deletes a key that handleSAInitResponse already replaced. An IKE rekey swaps
	// in a different SA under a new pair, so even a pair read at return time names
	// the wrong entry.
	//
	// This session owns every entry of its peer name. A removal by name ends the
	// cycle with nothing of its own left behind. ps.sa stays, because reconnectDelay
	// reads its retransmit count.
	defer table.RemoveByPeer(sa.PeerName)

	remote, err := net.ResolveUDPAddr("udp4", ikeAddr(peer.RemoteAddress))
	if err != nil {
		return fmt.Errorf("ike: resolve remote: %w", err)
	}

	initMsg := buildSAInitRequest(sa, ikeGroup)
	sa.InitiatorSAInitMsg = initMsg
	sa.State = StateSAInitSent
	sa.LastSentMsg = initMsg

	if tr != nil {
		if err := tr.Send(initMsg, remote); err != nil {
			return fmt.Errorf("ike: send sa-init: %w", err)
		}
	}
	log.Debug("ike: sent IKE_SA_INIT", "peer", ps.peerName)

	// Wait for responses or retransmit.
	sa.RetransmitTime = time.Now().Add(retransmitBase)
	sa.RetransmitCount = 0

	for sa.State != StateEstablished {
		if ps.stopped() {
			return errStopped
		}

		timeout := time.Until(sa.RetransmitTime)
		if timeout <= 0 {
			if sa.RetransmitCount >= maxRetransmissions {
				return errTimeout
			}
			sa.RetransmitCount++
			// RFC 7296 Section 2.1: exponential backoff.
			delay := retransmitBackoff(sa.RetransmitCount)
			sa.RetransmitTime = time.Now().Add(delay)
			if tr != nil && sa.LastSentMsg != nil {
				if err := tr.Send(sa.LastSentMsg, remote); err != nil {
					log.Warn("ike: retransmit failed", "peer", ps.peerName, "error", err)
				}
			}
			log.Debug("ike: retransmit", "peer", ps.peerName, "attempt", sa.RetransmitCount)
			continue
		}

		select {
		case <-ps.stopCh:
			return errStopped
		case <-afterFunc(timeout):
		}
	}

	log.Info("ike: SA established", "peer", ps.peerName,
		"ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI))

	if bus != nil {
		if _, emitErr := SAUp.Emit(bus, &SAEvent{
			PeerName:      sa.PeerName,
			InitiatorSPI:  SPIHex(sa.InitiatorSPI),
			ResponderSPI:  SPIHex(sa.ResponderSPI),
			RemoteAddress: peer.RemoteAddress,
			AuthMethod:    peer.Auth.Mode.String(),
		}); emitErr != nil {
			log.Warn("ike: emit sa-up failed", "error", emitErr)
		}
	}

	return ps.runEstablished(sa, peer, ikeGroup, table, tr, bus, log)
}

// runResponder accepts remote-initiated IKE exchanges. The handshake itself is
// driven on the shared dispatch goroutine (dispatchInbound creates the responder SA
// and handleResponderInbound advances it); this goroutine polls the published SA and,
// once it reaches StateEstablished, adopts it into the owner loop (runEstablished).
// When the tunnel goes down it cleans up and waits for the next inbound. RFC 7296
// Section 1.2, spec-ipsec-14.
func (ps *PeerSession) runResponder(
	peer ipsec.SiteToSitePeer,
	ikeGroup ipsec.IKEGroup,
	table *SATable,
	tr *transport.UDPTransport,
	bus ze.EventBus,
	log *slog.Logger,
) error {
	ps.responderBusy.Store(false)
	log.Debug("ike: responder ready", "peer", ps.peerName)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ps.stopCh:
			return errStopped
		case <-ticker.C:
		}

		sa := ps.getSA()
		if sa == nil {
			continue
		}

		switch sa.State {
		case StateEstablished:
			log.Info("ike: responder SA established", "peer", ps.peerName,
				"ispi", SPIHex(sa.InitiatorSPI), "rspi", SPIHex(sa.ResponderSPI))
			if bus != nil {
				if _, emitErr := SAUp.Emit(bus, &SAEvent{
					PeerName:      sa.PeerName,
					InitiatorSPI:  SPIHex(sa.InitiatorSPI),
					ResponderSPI:  SPIHex(sa.ResponderSPI),
					RemoteAddress: peer.RemoteAddress,
					AuthMethod:    peer.Auth.Mode.String(),
				}); emitErr != nil {
					log.Warn("ike: emit sa-up failed", "error", emitErr)
				}
			}
			// This half-open handshake is now established: clear the busy gate so a
			// fresh IKE_SA_INIT can be accepted in PARALLEL during this SA's life
			// (RFC 7296 Section 2.4). runEstablished owns the SA until it goes down or
			// is superseded by a re-initiated SA that authenticated.
			ps.responderBusy.Store(false)
			err := ps.runEstablished(sa, peer, ikeGroup, table, tr, bus, log)
			// Tunnel down (peer Delete, DPD timeout, lifetime, operator clear, or
			// supersede): tear this SA down.
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
			emitSADown(bus, sa, log)
			ps.setSA(nil)
			// Resolve any parallel (second-slot) SA. On stop it is freed (its Child SA
			// cannot be promoted-then-leaked); otherwise it is promoted into the primary
			// slot so the poll loop adopts it. Extracted so the operator-clear +
			// parallel-auth race is unit-tested at its entry point (fail-closed-guards).
			if ps.resolvePendingAfterOwnerLoop(table, dataplane.Get(), bus, log) == pendingContinue {
				continue
			}
			if ps.stopped() {
				if err != nil {
					return err
				}
				return errStopped
			}
		case StateDead:
			// Handshake failed before establishment: reset for a fresh attempt.
			table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
			ps.setSA(nil)
			ps.responderBusy.Store(false)
		case StateIdle, StateSAInitSent, StateSAInitReceived, StateAuthSent, StateAuthReceived, StateEAPInProgress:
			// Handshake in progress on the dispatch goroutine. Reap it if the peer
			// abandoned it so responderBusy and the SATable slot free up (Finding 1).
			ps.reapStaleHandshake(sa, table, log)
		}
	}
}

// pendingResolution is the runResponder decision after the owner loop returns.
type pendingResolution int

const (
	pendingContinue pendingResolution = iota // a parallel SA was promoted; keep polling
	pendingReturn                            // nothing to promote; caller checks stop/loop
)

// resolvePendingAfterOwnerLoop runs after the just-owned responder SA is torn down.
// If the session is stopping it frees any parallel (second-slot) SA and its
// make-before-break Child SA -- so a Child SA installed by a parallel handshake that
// authenticated in the operator-clear race is NEVER promoted-then-leaked -- and returns
// pendingReturn. Otherwise it promotes a parallel SA into the primary slot so the poll
// loop adopts it (pendingContinue), or, if none, clears the half-open gate
// (pendingReturn). RFC 7296 Section 2.4.
func (ps *PeerSession) resolvePendingAfterOwnerLoop(table *SATable, dp dataplane.Dataplane, bus ze.EventBus, log *slog.Logger) pendingResolution {
	if ps.stopped() {
		ps.cleanupPendingSA(table, dp, bus, log)
		return pendingReturn
	}
	pending := ps.getPendingSA()
	if pending == nil {
		ps.responderBusy.Store(false)
		return pendingReturn
	}
	// Adopt the parallel SA (authenticated supersede, or half-open relocated because this
	// SA died independently) so the poll loop tracks it and it is not orphaned. Its Child
	// SA -- installed only once it authenticated -- moves with it (make-before-break).
	ps.setPendingSA(nil)
	if pc := ps.getPendingChild(); pc != nil {
		ps.setChildSA(pc)
		ps.setPendingChild(nil)
	}
	ps.setSA(pending)
	log.Info("ike: promoting re-initiated SA", "peer", ps.peerName,
		"ispi", SPIHex(pending.InitiatorSPI), "rspi", SPIHex(pending.ResponderSPI),
		"state", pending.State.String())
	return pendingContinue
}

// reapStaleHandshake tears down a responder SA whose handshake the peer abandoned
// (stuck in a pre-established state past responderHandshakeTimeout), freeing the
// responderBusy gate and the SATable entry so the peer can reconnect with a fresh
// IKE_SA_INIT. Returns true if it reaped. RFC 7296 Section 2.4.
func (ps *PeerSession) reapStaleHandshake(sa *SA, table *SATable, log *slog.Logger) bool {
	if time.Since(sa.CreatedAt) <= responderHandshakeTimeout {
		return false
	}
	// The dispatch goroutine may have completed the handshake between runResponder's
	// state switch and here; never tear down an SA that just established, or we would
	// orphan the tunnel (runResponder never adopts it) and leak the installed Child
	// SA. runResponder picks it up on the next tick (review pass 2, Finding 1).
	if sa.State == StateEstablished {
		return false
	}
	log.Warn("ike: responder handshake timed out, tearing down",
		"peer", ps.peerName, "state", sa.State.String())
	table.Remove(sa.InitiatorSPI, sa.ResponderSPI)
	ps.setSA(nil)
	ps.responderBusy.Store(false)
	return true
}

// handleInbound processes an inbound packet for an existing SA.
func handleInbound(sa *SA, pkt transport.Packet, table *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	var msg wire.Message
	if err := msg.ReadFrom(pkt.Data); err != nil {
		log.Debug("ike: parse error", "error", err)
		return
	}

	// Responder handshake: the SA was created by dispatchInbound and is advanced on
	// this goroutine until it establishes and runResponder adopts it (spec-ipsec-14).
	if !sa.IsInitiator {
		ps := lookupPeerSession(sa.PeerName)
		if ps == nil {
			log.Debug("ike: no session for responder SA", "peer", sa.PeerName)
			return
		}
		ps.handleResponderInbound(sa, &msg, pkt, tr, log)
		return
	}

	switch sa.State {
	case StateSAInitSent:
		if msg.Header.ExchangeType == wire.ExchangeIKESAInit &&
			msg.Header.Flags&wire.FlagResponse != 0 {
			handleSAInitResponse(sa, &msg, pkt.Data, table, tr, pkt.RemoteAddr, log)
		}
	case StateAuthSent:
		if msg.Header.ExchangeType == wire.ExchangeIKEAuth &&
			msg.Header.Flags&wire.FlagResponse != 0 {
			handleAuthResponse(sa, &msg, pkt.Data, table, tr, log)
			// StateEstablished is reached only after verifyRemoteAuth accepted the
			// responder's AUTH, so this observation is corroborated by an
			// authenticated message. RFC 7296 Section 2.23 names that trigger: a
			// packet "whose integrity protection validates". It is the initiator's
			// first authenticated sight of where the peer really answers from, which
			// under a NAT is neither the configured address nor port.
			//
			// The state IS the authentication signal here. That coupling is
			// deliberate. handleAuthResponse establishes the SA at exactly one place,
			// and only after the AUTH check. Moving the establish out of that path
			// breaks this, so keep the two together.
			if sa.State == StateEstablished {
				sa.adoptAuthenticatedEndpoint(pkt.RemoteAddr, pkt.NATT, log)
			}
		}
	case StateEAPInProgress:
		// RFC 7296 Section 2.16: EAP exchange rounds within IKE_AUTH.
		if msg.Header.ExchangeType == wire.ExchangeIKEAuth &&
			msg.Header.Flags&wire.FlagResponse != 0 {
			handleEAPResponse(sa, &msg, pkt.Data, tr, log)
		}
	case StateEstablished:
		// handleOwnedInbound processes every post-establishment message on the owner
		// loop, and nothing else does. That loop owns sa.SKKeys and the cached
		// response, so a handler here would write owner state from the shared
		// dispatch goroutine. This arm is reached only before maintainSA adopts the
		// SA. RFC 7296 Section 2.1 makes the peer retransmit, and the retransmission
		// reaches the owner loop.
		log.Debug("ike: established-SA packet arrived before the owner loop adopted it, dropping",
			"peer", sa.PeerName, "exchange", msg.Header.ExchangeType)
	case StateIdle, StateSAInitReceived, StateAuthReceived, StateDead:
		log.Debug("ike: message in unexpected state", "state", sa.State)
	}
}

// handleSAInitResponse processes an IKE_SA_INIT response for an initiator.
func handleSAInitResponse(
	sa *SA,
	msg *wire.Message,
	rawMsg []byte,
	table *SATable,
	tr *transport.UDPTransport,
	// The observed source of the IKE_SA_INIT response is deliberately NOT read. That
	// message is UNAUTHENTICATED, and RFC 7296 Section 2.23 lets an endpoint move only
	// on a packet "whose integrity protection validates". The initiator adopts an
	// endpoint one exchange later, from the IKE_AUTH response (handleInbound).
	_ *net.UDPAddr,
	log *slog.Logger,
) {
	copy(sa.ResponderSPI[:], msg.Header.ResponderSPI[:])
	sa.ResponderSAInitMsg = make([]byte, len(rawMsg))
	copy(sa.ResponderSAInitMsg, rawMsg)

	// Update table key now that we know the responder SPI.
	table.UpdateKey([8]byte{}, sa.ResponderSPI, sa)

	var remoteSA *wire.PayloadSA
	var remoteKE *wire.PayloadKE
	var remoteNonce *wire.PayloadNonce

	// A retry cause is RECORDED here and acted on after the loop, never inside it. A
	// message may legally carry several notifies, and acting mid-loop would let payload
	// order decide which cause wins. RFC 7296 Section 2.6.1's shorter exchange shows
	// COOKIE and INVALID_KE_PAYLOAD in the same exchange, so the precedence has to be
	// explicit: COOKIE first, because a cookie challenge means the responder committed
	// no state and never evaluated the KE payload at all.
	retryFor := retryCauseNone
	var retryData []byte

	for _, pe := range msg.Payloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadSA:
			remoteSA = p
		case *wire.PayloadKE:
			remoteKE = p
		case *wire.PayloadNonce:
			remoteNonce = p
		case *wire.PayloadNotify:
			if p.NotifyMsgType == wire.NotifyNoProposalChosen {
				log.Warn("ike: remote sent NO_PROPOSAL_CHOSEN", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
			// RFC 7296 Section 2.6 MUST: "If the IKE_SA_INIT response includes the
			// COOKIE notification, the initiator MUST then retry the IKE_SA_INIT
			// request, and include the COOKIE notification containing the received
			// data as the first payload, and all other payloads unchanged."
			if p.NotifyMsgType == wire.NotifyCookie {
				retryFor = retryCookie
				retryData = p.NotificationData
			}
			// RFC 7296 Section 1.2 MUST: "If the initiator guesses wrong, the
			// responder will respond with a Notify payload of type
			// INVALID_KE_PAYLOAD indicating the selected group. In this case, the
			// initiator MUST retry the IKE_SA_INIT with the corrected Diffie-Hellman
			// group."
			if p.NotifyMsgType == wire.NotifyInvalidKEPayload && retryFor == retryCauseNone {
				retryFor = retryInvalidKE
				retryData = p.NotificationData
			}
			if p.NotifyMsgType == wire.NotifySignatureHashAlgorithms {
				sa.RemoteHashAlgos = parseHashAlgoNotify(p.NotificationData)
			}
			// RFC 7296 Section 2.23: check NAT detection notify payloads.
			// SOURCE_IP from responder: hash of responder's own address.
			// Mismatch means responder is behind NAT.
			// RFC 7296 Section 2.23 MUST: the initiator, on a mismatch,
			// "MUST tunnel all future IKE and ESP packets associated with this IKE SA over UDP port 4500".
			// floatToNATTPort takes that decision once. Every later sender reads it
			// through sa.sendPath, so the IKE_AUTH below and the whole established
			// lifetime leave from 4500.
			if p.NotifyMsgType == wire.NotifyNATDetectionSourceIP {
				remoteIP := net.ParseIP(sa.PeerCfg.RemoteAddress)
				if remoteIP != nil {
					expected := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, remoteIP, transport.IKEPort)
					if !natHashEqual(p.NotificationData, expected) {
						sa.NATDetected = true
						sa.floatToNATTPort()
					}
				}
			}
			// DESTINATION_IP from responder: hash of us as the responder sees us.
			// Mismatch means our address was translated (we are behind NAT).
			if p.NotifyMsgType == wire.NotifyNATDetectionDestIP {
				localIP := net.ParseIP(sa.PeerCfg.LocalAddress)
				if localIP != nil {
					expected := transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, localIP, transport.IKEPort)
					if !natHashEqual(p.NotificationData, expected) {
						sa.NATDetected = true
						sa.BehindNAT = true
						sa.floatToNATTPort()
					}
				}
			}
		}
	}

	// The retry runs BEFORE the completeness gate below, because a notify-only response
	// carries no SA, KE or Nonce by design. RFC 7296 Section 2.21.1 names both notifies
	// as ones that "may lead to a subsequent successful exchange", and retrySAInit
	// bounds how many times this node acts on an unauthenticated one.
	if retryFor != retryCauseNone {
		retrySAInit(sa, retryFor, retryData, table, tr, log)
		return
	}

	if remoteSA == nil || remoteKE == nil || remoteNonce == nil {
		log.Warn("ike: incomplete IKE_SA_INIT response", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 3.3.1: an IKE_SA_INIT response belongs to the initial IKE SA
	// negotiation, so its proposals MUST carry an SPI Size of zero. The exchange is what
	// makes the rule apply, and the parse layer never sees the exchange.
	if err := remoteSA.ValidateInitialSPISize(); err != nil {
		log.Warn("ike: IKE_SA_INIT response carries an SPI in its proposals",
			"peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 3.4: the Diffie-Hellman Group Num of the KE payload "MUST match
	// a Diffie-Hellman group specified in a proposal in the SA payload that is sent in
	// the same message", and where no proposal specifies a group "the KE payload MUST
	// NOT be present". The responder's SAr1 carries the one proposal it accepted, so a
	// KEr in any other group is a responder that answered in a group it never named.
	// Deriving a shared secret from it would key the SA under a group this node never
	// agreed to, so the exchange stops here.
	if err := remoteSA.ValidateKEGroup(remoteKE); err != nil {
		log.Warn("ike: IKE_SA_INIT response KE group disagrees with its SA payload",
			"peer", sa.PeerName, "ke-group", remoteKE.DHGroup, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 3.3.6: the initiator checks the accepted offer against its own
	// proposals. It stops the exchange when the two disagree.
	offer, err := verifyAcceptedOffer(remoteSA, sa.IKEGroup, sa.ESPGroup)
	if err != nil {
		log.Warn("ike: IKE_SA_INIT accepted offer rejected", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	chosen := offer.IKE
	sa.Proposal = chosen

	// Process DH exchange.
	sa.RemoteNonce = remoteNonce.NonceData
	sa.RemoteDHPub = remoteKE.KeyExchangeData

	sharedSecret, err := sa.LocalDH.SharedSecret(sa.RemoteDHPub)
	if err != nil {
		log.Warn("ike: DH shared secret failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// Derive keys.
	skeyseed, err := crypto.DeriveSKEYSEED(chosen.PRF.ID, sa.LocalNonce, sa.RemoteNonce, sharedSecret)
	if err != nil {
		log.Warn("ike: SKEYSEED derivation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	skKeys, err := crypto.DeriveSKKeys(
		chosen.PRF.ID, skeyseed,
		sa.LocalNonce, sa.RemoteNonce,
		sa.InitiatorSPI[:], sa.ResponderSPI[:],
		chosen.Encryption, chosen.Integrity,
	)
	if err != nil {
		log.Warn("ike: SK key derivation failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.SKKeys = skKeys
	sa.LocalDH.Clear()
	clear(sharedSecret)
	clear(skeyseed)

	// Build and send IKE_AUTH request.
	// RFC 7296 Section 2.2: first IKE_AUTH uses message ID 1.
	sa.NextMsgID = 1
	authReq, err := buildAuthRequest(sa)
	if err != nil {
		log.Warn("ike: build IKE_AUTH failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	sa.State = StateAuthSent
	sa.LastSentMsg = authReq
	sa.RetransmitTime = time.Now().Add(retransmitBase)
	sa.RetransmitCount = 0

	// The IKE_AUTH is a REQUEST this node raises, not an answer to one, so it goes on
	// the SA's own send path. RFC 7296 Section 2.23 MUST: an initiator whose
	// NAT_DETECTION payloads did not match the outer packet
	// "MUST tunnel all future IKE and ESP packets associated with this IKE SA over UDP port 4500".
	// The payload walk above took that verdict and called floatToNATTPort, so sendRaw
	// reads the float here.
	sendRaw(sa, tr, authReq, log)
	log.Debug("ike: sent IKE_AUTH", "peer", sa.PeerName)
}

// handleAuthResponse processes an IKE_AUTH response for an initiator.
// RFC 7296 Section 2.16: if the response contains an EAP payload, the responder
// is requesting EAP authentication. The initiator verifies the server's AUTH first,
// then enters the EAP exchange loop.
func handleAuthResponse(sa *SA, msg *wire.Message, rawMsg []byte, _ *SATable, tr *transport.UDPTransport, log *slog.Logger) {
	var skPayload *wire.PayloadSK
	for _, pe := range msg.Payloads {
		if sk, ok := pe.Payload.(*wire.PayloadSK); ok {
			skPayload = sk
		}
	}
	if skPayload == nil {
		log.Warn("ike: AUTH response has no SK payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	plaintext, err := decryptSKPayload(sa, rawMsg, skPayload)
	if err != nil {
		log.Warn("ike: AUTH response decryption failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	innerPayloads, err := wire.ParsePayloadChain(plaintext, skPayload.InnerNextPayload)
	if err != nil {
		log.Warn("ike: AUTH response inner payload parse failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 3.10.1 MUST: an error notify type this implementation does not
	// recognize, arriving in a response, means "the corresponding request has failed
	// entirely". The IKE_AUTH request is that request, so the exchange ends here rather
	// than continuing into an AUTH verification the response was never going to satisfy.
	if err := failIfUnrecognizedErrorNotify(innerPayloads, sa.PeerName, log); err != nil {
		sa.State = StateDead
		return
	}
	// RFC 7296 Section 3.10.1: every other unrecognized notify is ignored, and logged.
	logIgnoredNotifies(innerPayloads, sa.PeerName, true, log)

	// RFC 7296 Section 2.5 says: implementations MUST NOT reject as invalid a message
	// with those payloads in any other order.
	// Section 1.7 removed the earlier allowance to do so.
	// The walk below therefore only COLLECTS.
	// AUTH verification reads the identification and certificate payloads.
	// A verification call inside the walk refuses a peer that sent AUTH before IDr.
	// handleAuthRequest (responder.go) has the same shape for the same reason.
	var authPayload *wire.PayloadAUTH
	var eapPayload *wire.PayloadEAP
	var childOffer *wire.PayloadSA
	var certPayloads []*wire.PayloadCERT
	var setWindowSize *wire.PayloadNotify
	for _, pe := range innerPayloads {
		switch p := pe.Payload.(type) {
		case *wire.PayloadNotify:
			if p.NotifyMsgType == wire.NotifyAuthenticationFailed {
				log.Warn("ike: remote sent AUTHENTICATION_FAILED", "peer", sa.PeerName)
				sa.State = StateDead
				return
			}
			// RFC 7296 Section 2.3: the peer states how many outstanding requests it
			// keeps. IKE_AUTH is the read point, because "The window size is always one
			// until the initial exchanges complete".
			if p.NotifyMsgType == wire.NotifySetWindowSize {
				setWindowSize = p
			}
		case *wire.PayloadID:
			if p.IDPayloadType == wire.PayloadTypeIDr {
				sa.RemoteIDPayload = p
			}
		case *wire.PayloadAUTH:
			authPayload = p
		case *wire.PayloadCERT:
			if acceptedCertEncoding(sa, p) {
				certPayloads = append(certPayloads, p)
			}
		case *wire.PayloadEAP:
			eapPayload = p
		case *wire.PayloadSA:
			childOffer = p
			for _, prop := range p.Proposals {
				if prop.ProtocolID == wire.ProtocolESP && prop.SPISize == 4 && len(prop.SPI) >= 4 {
					sa.ChildOutboundSPI = binary.BigEndian.Uint32(prop.SPI[:4])
				}
			}
		case *wire.PayloadTS:
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				sa.NegotiatedTSi = tsToIPNet(p.TrafficSelectors)
			case wire.PayloadTypeTSr:
				sa.NegotiatedTSr = tsToIPNet(p.TrafficSelectors)
			}
		}
	}
	if err := storeRemoteCerts(sa, certPayloads, log); err != nil {
		log.Warn("ike: peer certificate chain refused", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	if err := recordPeerWindowSize(sa, setWindowSize); err != nil {
		log.Warn("ike: peer SET_WINDOW_SIZE refused", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	if authPayload == nil {
		log.Warn("ike: AUTH response missing AUTH payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}
	if err := verifyRemoteAuth(sa, authPayload); err != nil {
		log.Warn("ike: remote AUTH verification failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	// RFC 7296 Section 3.3.6: the initiator checks the accepted Child SA offer against
	// the ESP proposals it sent. It stops the exchange when the two disagree. An EAP
	// round carries no SAr2, so the check runs on the response that holds one.
	if childOffer != nil {
		if _, err := verifyAcceptedOffer(childOffer, sa.IKEGroup, sa.ESPGroup); err != nil {
			log.Warn("ike: IKE_AUTH accepted offer rejected", "peer", sa.PeerName, "error", err)
			sa.State = StateDead
			return
		}
	}

	// RFC 7296 Section 2.16: EAP payload present means the responder requests EAP.
	if eapPayload != nil {
		startEAPExchange(sa, eapPayload, tr, log)
		return
	}

	sa.State = StateEstablished
	sa.EstablishedAt = time.Now()
	// RFC 7296 §2.2: advance past the IKE_AUTH (or final EAP) message ID so the
	// first post-establishment exchange (rekey, DPD, Delete) uses the next free
	// ID. Until now NextMsgID held the last-used value; without this the first
	// CREATE_CHILD_SA reused the IKE_AUTH ID and the peer rejected it
	// (INVALID_MESSAGE_ID / "expected N, ignored").
	sa.advanceMsgID()
}

// startEAPExchange initializes the EAP peer session and processes the first EAP request.
// RFC 7296 Section 2.16: server AUTH is already verified before this is called.
func startEAPExchange(sa *SA, eapPayload *wire.PayloadEAP, tr *transport.UDPTransport, log *slog.Logger) {
	identity := sa.PeerCfg.Auth.LocalID
	if identity == "" {
		identity = sa.PeerName
	}

	var ps *eap.PeerSession
	switch sa.PeerCfg.Auth.Mode {
	case ipsec.AuthEAPMSCHAPv2:
		ps = eap.NewPeerSession(eap.TypeMSCHAPv2, identity, sa.PeerCfg.Auth.PSK)
	case ipsec.AuthEAPTLS:
		tlsCfg := buildPeerTLSConfig(sa, log)
		if tlsCfg == nil {
			sa.State = StateDead
			return
		}
		ps = eap.NewPeerSessionTLS(identity, tlsCfg)
	default:
		log.Warn("ike: server sent EAP but auth mode is not EAP", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}
	sa.EAPSession = ps

	parsed := wireEAPToPacket(eapPayload)
	result := ps.Process(parsed)
	if result.Err != nil {
		log.Warn("ike: EAP process failed", "peer", sa.PeerName, "error", result.Err)
		sa.State = StateDead
		return
	}

	if result.Response != nil {
		sa.advanceMsgID()
		sendEAPResponsePacket(sa, result.Response, tr, log)
	}

	sa.State = StateEAPInProgress
	sa.RetransmitTime = time.Now().Add(retransmitMax)
	sa.RetransmitCount = 0
	log.Debug("ike: EAP exchange started", "peer", sa.PeerName)
}

// handleEAPResponse processes an IKE_AUTH response during an active EAP exchange.
func handleEAPResponse(sa *SA, msg *wire.Message, rawMsg []byte, tr *transport.UDPTransport, log *slog.Logger) {
	var skPayload *wire.PayloadSK
	for _, pe := range msg.Payloads {
		if sk, ok := pe.Payload.(*wire.PayloadSK); ok {
			skPayload = sk
		}
	}
	if skPayload == nil {
		log.Warn("ike: EAP response has no SK payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	plaintext, err := decryptSKPayload(sa, rawMsg, skPayload)
	if err != nil {
		log.Warn("ike: EAP response decryption failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	innerPayloads, err := wire.ParsePayloadChain(plaintext, skPayload.InnerNextPayload)
	if err != nil {
		log.Warn("ike: EAP response inner parse failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}

	var eapPayload *wire.PayloadEAP
	for _, pe := range innerPayloads {
		if p, ok := pe.Payload.(*wire.PayloadEAP); ok {
			eapPayload = p
		}
	}
	if eapPayload == nil {
		log.Warn("ike: EAP response missing EAP payload", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	ps, ok := sa.EAPSession.(*eap.PeerSession)
	if !ok || ps == nil {
		log.Warn("ike: no EAP peer session", "peer", sa.PeerName)
		sa.State = StateDead
		return
	}

	parsed := wireEAPToPacket(eapPayload)
	log.Debug("ike: EAP received", "peer", sa.PeerName, "code", parsed.Code, "type", parsed.Type, "id", parsed.Identifier)
	result := ps.Process(parsed)
	if result.Err != nil {
		log.Warn("ike: EAP failed", "peer", sa.PeerName, "error", result.Err)
		sa.State = StateDead
		return
	}

	if result.Done {
		// RFC 7296 Section 2.16: EAP succeeded, send AUTH derived from MSK.
		sa.EAPMSK = result.MSK
		sa.advanceMsgID()
		authMsg, err := buildEAPAuthMessage(sa)
		if err != nil {
			log.Warn("ike: build EAP AUTH failed", "peer", sa.PeerName, "error", err)
			sa.State = StateDead
			return
		}
		sa.LastSentMsg = authMsg
		sa.State = StateAuthSent
		sa.RetransmitTime = time.Now().Add(retransmitBase)
		sa.RetransmitCount = 0

		// A request this node raises, so it uses the SA's own send path.
		sendRaw(sa, tr, authMsg, log)
		log.Debug("ike: EAP success, sent AUTH from MSK", "peer", sa.PeerName)
		return
	}

	if result.Response != nil {
		sa.advanceMsgID()
		sendEAPResponsePacket(sa, result.Response, tr, log)
	}

	sa.RetransmitTime = time.Now().Add(retransmitMax)
	sa.RetransmitCount = 0
}

// sendEAPResponsePacket builds and sends an encrypted IKE_AUTH message containing an EAP response.
func sendEAPResponsePacket(sa *SA, resp *eap.Packet, tr *transport.UDPTransport, log *slog.Logger) {
	eapData := resp.Encode()
	log.Debug("ike: sending EAP response", "peer", sa.PeerName, "code", resp.Code, "type", resp.Type, "msgID", sa.NextMsgID)
	msg, err := buildEAPResponse(sa, eapData)
	if err != nil {
		log.Warn("ike: build EAP response failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.LastSentMsg = msg
	// An EAP response is carried in an IKE_AUTH REQUEST the initiator raises, so it
	// uses the SA's own send path (RFC 7296 Section 2.16).
	sendRaw(sa, tr, msg, log)
}

// retransmitBackoff computes the delay for retransmission attempt n.
// RFC 7296 Section 2.1: exponential backoff capped at retransmitMax.
func retransmitBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return retransmitBase
	}
	shift := min(attempt-1, 7)
	d := retransmitBase * time.Duration(1<<uint(shift))
	if d > retransmitMax {
		return retransmitMax
	}
	return d
}

// buildPeerTLSConfig loads client certificate material from the PKI store for EAP-TLS.
func buildPeerTLSConfig(sa *SA, log *slog.Logger) *eap.PeerTLSConfig {
	certName := sa.PeerCfg.Auth.Certificate
	if certName == "" {
		log.Warn("ike: EAP-TLS requires a client certificate", "peer", sa.PeerName)
		return nil
	}
	entry := pki.GetCertificate(certName)
	if entry == nil {
		log.Warn("ike: client certificate not found in PKI store", "peer", sa.PeerName, "cert", certName)
		return nil
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: entry.Raw})
	if entry.PrivateKey == nil {
		log.Warn("ike: client certificate has no private key", "peer", sa.PeerName, "cert", certName)
		return nil
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(entry.PrivateKey)
	if err != nil {
		log.Warn("ike: marshal client private key failed", "peer", sa.PeerName, "error", err)
		return nil
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	cfg := &eap.PeerTLSConfig{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
	}

	// RFC 5216 Section 5.3: "Both sides MUST perform certificate path validation."
	// EAP carries no server hostname, so the configured CA is the peer's ONLY way
	// to validate the authenticator. Without it the TLS client would accept any
	// certificate (peer.go startTLSClient attaches VerifyPeerCertificate only when
	// a trust anchor is present), so a missing or unresolvable CA fails the session
	// here rather than silently downgrading to no verification.
	caName := sa.PeerCfg.Auth.CACertificate
	if caName == "" {
		log.Warn("ike: EAP-TLS requires a ca-certificate to validate the authenticator", "peer", sa.PeerName)
		return nil
	}
	ca := pki.GetCA(caName)
	if ca == nil {
		log.Warn("ike: EAP-TLS ca-certificate not found in PKI store", "peer", sa.PeerName, "ca", caName)
		return nil
	}
	cfg.CACertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})

	return cfg
}

// wireEAPToPacket converts a wire.PayloadEAP to an eap.Packet without allocation.
func wireEAPToPacket(p *wire.PayloadEAP) *eap.Packet {
	pkt := &eap.Packet{
		Code:       p.Code,
		Identifier: p.Identifier,
	}
	if len(p.EAPData) > 0 {
		pkt.Type = p.EAPData[0]
		if len(p.EAPData) > 1 {
			pkt.TypeData = p.EAPData[1:]
		}
	}
	return pkt
}
