// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE_AUTH exchange
// Related: auth.go -- the AUTH builders and verifiers that read the round's credential
// Related: sa.go -- the round bookkeeping fields these accessors read
// RFC: rfc/short/rfc4739.md -- MULTIPLE_AUTH_SUPPORTED, ANOTHER_AUTH_FOLLOWS (Section 3)

package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// authRound names the RFC 4739 authentication round one direction of an IKE SA is
// in. Ze runs at most one extra round per direction.
//
// The zero value is the first round on purpose. Every IKE_AUTH exchange opens with
// it, so an SA no round logic has touched is in round 1, and there is no unset
// state to refuse.
type authRound uint8

const (
	authRoundFirst authRound = iota
	authRoundSecond
)

// errNoSecondRound is the refusal of a move to a round the peer does not configure.
var errNoSecondRound = errors.New("ike auth: no second authentication round is configured")

// localAuth answers the credential this side proves in its current round: the local
// id, the certificate, and the pre-shared key it signs with. Round 1 is the peer's
// own authentication block, round 2 its `authentication second-round local`.
//
// RFC 4739 Section 2.1: "The AUTH payloads are calculated as specified in [IKEv2]
// Sections 2.15 and 2.16, where IDi' refers to the latest IDi payload sent by the
// initiator, and IDr' refers to the latest IDr payload sent by the responder." Every
// round therefore runs the same AUTH code over its own credential, and this accessor
// is how that code finds it. sa.PeerCfg is never rewritten for a round: reload
// compares it, and rekey and DPD read it.
//
// The round-2 answer is never nil. beginLocalSecondRound is the only move to round 2,
// and it refuses a peer whose second-round local credential is absent.
func (sa *SA) localAuth() *ipsec.AuthConfig {
	switch sa.localRound {
	case authRoundFirst:
		return &sa.PeerCfg.Auth
	case authRoundSecond:
		return sa.PeerCfg.Auth.SecondRound().Local
	}
	panic("BUG: unknown local authentication round")
}

// remoteAuth answers the credential the peer must prove in its current round: the
// remote id and its type, the CA the chain must reach, the certificate count
// accepted, and the pre-shared key the AUTH is verified with. Round 1 is the peer's
// own authentication block, round 2 its `authentication second-round remote`.
//
// RFC 4739 Section 2.1 (the latest-ID rule quoted on localAuth) applies to the
// verifier as it does to the signer. The round-2 answer is never nil, for the reason
// localAuth gives, through beginRemoteSecondRound.
func (sa *SA) remoteAuth() *ipsec.AuthConfig {
	switch sa.remoteRound {
	case authRoundFirst:
		return &sa.PeerCfg.Auth
	case authRoundSecond:
		return sa.PeerCfg.Auth.SecondRound().Remote
	}
	panic("BUG: unknown remote authentication round")
}

// beginLocalSecondRound moves this side to its second authentication round. It
// refuses a peer with no `authentication second-round local`, which keeps the
// non-nil promise of localAuth.
func (sa *SA) beginLocalSecondRound() error {
	rounds := sa.PeerCfg.Auth.SecondRound()
	if rounds == nil {
		return fmt.Errorf("%w: peer %q has no authentication second-round local", errNoSecondRound, sa.PeerName)
	}
	if rounds.Local == nil {
		return fmt.Errorf("%w: peer %q has no authentication second-round local", errNoSecondRound, sa.PeerName)
	}
	sa.localRound = authRoundSecond
	return nil
}

// beginRemoteSecondRound moves the peer's side to its second authentication round.
// It refuses a peer with no `authentication second-round remote`, which keeps the
// non-nil promise of remoteAuth.
//
// The certificates the peer sent for its first round are forgotten, so a second
// x509 round is verified only against what the peer sends for it, never against the
// first round's certificate.
func (sa *SA) beginRemoteSecondRound() error {
	rounds := sa.PeerCfg.Auth.SecondRound()
	if rounds == nil {
		return fmt.Errorf("%w: peer %q has no authentication second-round remote", errNoSecondRound, sa.PeerName)
	}
	if rounds.Remote == nil {
		return fmt.Errorf("%w: peer %q has no authentication second-round remote", errNoSecondRound, sa.PeerName)
	}
	sa.remoteRound = authRoundSecond
	sa.RemoteCertRaw, sa.RemoteCertChainRaw = nil, nil
	return nil
}

// multipleAuthNotify builds the MULTIPLE_AUTH_SUPPORTED notification. Every message
// that announces support takes it from here.
//
// RFC 4739 Section 3.1: "The Protocol ID and SPI Size fields MUST be set to zero, and
// there is no data associated with this Notify type."
//
// The Notify payload it encodes (RFC 7296 Section 3.10), eight octets in all:
//
//	                     1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Next Payload  |C|  RESERVED   |         Payload Length = 8    |  offset 0
//	+---------------+---------------+-------------------------------+
//	| Protocol ID=0 |  SPI Size=0   |  Notify Message Type = 16404  |  offset 4
//	+---------------+---------------+-------------------------------+
//
// Protocol ID is octet 4 and SPI Size octet 5. With SPI Size zero no SPI field
// follows, and no Notification Data follows the type.
//
// The two zero octets have two producers on purpose. This builder sets both
// fields to zero, and wire.PayloadNotify.WriteTo writes both as zero whenever no
// SPI follows (RFC 7296 Section 3.10). The "no data" half has one producer: this
// builder leaves NotificationData empty.
func multipleAuthNotify() *wire.PayloadNotify {
	return &wire.PayloadNotify{ProtocolID: 0, SPISize: 0, NotifyMsgType: wire.NotifyMultipleAuthSupported}
}

// anotherAuthFollowsNotify builds the ANOTHER_AUTH_FOLLOWS notification. Every message
// that announces a further round takes it from here.
//
// RFC 4739 Section 3.2: "The Protocol ID and SPI Size fields MUST be set to zero, and
// there is no data associated with this Notify type."
//
// The layout is multipleAuthNotify's with Notify Message Type 16405: Protocol ID zero
// at octet 4, SPI Size zero at octet 5, no SPI, no data, eight octets in all, with
// the same two producers of the zero octets and the one producer of "no data".
func anotherAuthFollowsNotify() *wire.PayloadNotify {
	return &wire.PayloadNotify{ProtocolID: 0, SPISize: 0, NotifyMsgType: wire.NotifyAnotherAuthFollows}
}

// multipleAuthRequestNotifies answers the RFC 4739 notifications of the initiator's
// first IKE_AUTH request. carriesAuth says whether that request carries an AUTH
// payload, which an EAP first round does not.
//
// RFC 4739 Section 2.1: "The peers announce support for this IKEv2 extension by
// including a MULTIPLE_AUTH_SUPPORTED notification in the IKE_SA_INIT response
// (responder) and the first IKE_AUTH request (initiator)." Ze announces it only for a
// peer that configures a second round, in either direction.
//
// The same section lets a peer ask for its own second round only once both sides
// announced support: "If both peers support this extension, either of them can
// announce that it wishes to have a second authentication by including an
// ANOTHER_AUTH_FOLLOWS notification in any IKE_AUTH message that contains an AUTH
// payload." A responder that sent no MULTIPLE_AUTH_SUPPORTED cannot run the round, so
// the round is skipped with a warning naming the fix, as strongSwan does
// (ike_auth.c do_another_auth), and the exchange completes on the first round.
func multipleAuthRequestNotifies(sa *SA, carriesAuth bool) ([]wire.PayloadEntry, error) {
	if !sa.PeerCfg.Auth.HasSecondRound() {
		return nil, nil
	}
	entries := make([]wire.PayloadEntry, 0, 2)
	entries = append(entries, wire.PayloadEntry{Payload: multipleAuthNotify()})
	if !sa.localSecondRoundConfigured() {
		return entries, nil
	}
	if !sa.peerMultipleAuth {
		warnLocalRoundSkipped(sa)
		return entries, nil
	}
	if !carriesAuth {
		// ValidateSecondRounds refuses a second round on an EAP first round at
		// commit, so only a Ze defect reaches here. Refusing keeps the round from
		// being dropped in silence.
		return nil, fmt.Errorf(
			"ike auth: peer %q has authentication second-round local and its first IKE_AUTH "+
				"request carries no AUTH payload to announce the round with. "+
				"Remove authentication second-round local, or use a pre-shared-secret or x509 first round",
			sa.PeerName)
	}
	return append(entries, wire.PayloadEntry{Payload: anotherAuthFollowsNotify()}), nil
}

// warnLocalRoundSkipped writes the one warning of a peer whose second local round
// cannot run, because the peer never announced MULTIPLE_AUTH_SUPPORTED. Both roles
// call it once, from the message that would have announced the round.
func warnLocalRoundSkipped(sa *SA) {
	getLogger().Warn(
		"ike: the peer does not support RFC 4739 multiple authentication, "+
			"so the second-round local credential is not sent and this side authenticates once",
		"peer", sa.PeerName,
		"fix", "remove authentication second-round local for this peer, "+
			"or enable multiple authentication on the peer")
}

// The refusals of an IKE_AUTH phase whose rounds do not match the configuration.
// Every one is answered with AUTHENTICATION_FAILED: by the responder in its
// IKE_AUTH response, by the initiator in an INFORMATIONAL (RFC 7296 Section 2.21.2).
var (
	errRemoteRoundNotConfigured = errors.New(
		"ike auth: the peer announced another authentication round (ANOTHER_AUTH_FOLLOWS) and " +
			"no authentication second-round remote is configured for it. Configure authentication " +
			"second-round remote for this peer, or remove the second round from the peer")
	errRemoteRoundMissing = errors.New(
		"ike auth: the peer finished authenticating without the second round that " +
			"authentication second-round remote requires. Configure the second authentication " +
			"round on the peer, or delete authentication second-round remote")
	errThirdRound = errors.New(
		"ike auth: the peer announced a third authentication round, and Ze runs at most one " +
			"more round each way. Configure the peer with at most two authentication rounds")
	errUnannouncedRound = errors.New(
		"ike auth: the peer sent an authentication round it never announced with " +
			"ANOTHER_AUTH_FOLLOWS. Correct the multiple authentication configuration of the peer")
	errRoundWithoutID = errors.New(
		"ike auth: the peer's second authentication round carries no identification payload. " +
			"Correct the multiple authentication configuration of the peer")
)

// remoteSecondRoundConfigured reports whether the operator requires a second round
// from the peer (`authentication second-round remote`).
func (sa *SA) remoteSecondRoundConfigured() bool {
	rounds := sa.PeerCfg.Auth.SecondRound()
	return rounds != nil && rounds.Remote != nil
}

// localSecondRoundRuns reports whether this side runs its second round on this SA:
// it is configured, and the peer announced MULTIPLE_AUTH_SUPPORTED. A configured
// round the peer cannot run is skipped (warnLocalRoundSkipped).
//
// RFC 4739 Section 2.1: "If both peers support this extension, either of them can
// announce that it wishes to have a second authentication by including an
// ANOTHER_AUTH_FOLLOWS notification in any IKE_AUTH message that contains an AUTH
// payload."
func (sa *SA) localSecondRoundRuns() bool {
	return sa.localSecondRoundConfigured() && sa.peerMultipleAuth
}

// localSecondRoundConfigured reports whether the operator configured a second round
// for this side (`authentication second-round local`).
func (sa *SA) localSecondRoundConfigured() bool {
	rounds := sa.PeerCfg.Auth.SecondRound()
	return rounds != nil && rounds.Local != nil
}

// moreAuthRounds reports, once the first IKE_AUTH exchange has been judged, whether
// a second exchange follows: the peer announced another round, or this side has one.
func (sa *SA) moreAuthRounds() bool {
	if sa.peerRoundOwed {
		return true
	}
	return sa.localRound == authRoundFirst && sa.localSecondRoundRuns()
}

// beginSecondAuthExchange moves the SA into the second IKE_AUTH exchange, and this
// side into its second round when it runs one. The caller MUST have built every
// payload of its first-round message before it calls this: the accessors answer for
// the second round afterwards.
func (sa *SA) beginSecondAuthExchange() error {
	sa.secondAuthExchange = true
	if !sa.localSecondRoundRuns() {
		return nil
	}
	return sa.beginLocalSecondRound()
}

// settleAnotherAuth judges ANOTHER_AUTH_FOLLOWS once a round of the peer verified.
// announced says whether the message of that round carried the notification.
//
// RFC 4739 Section 2.1: "The IKE_AUTH phase is considered successful only if all the
// individual authentication exchanges complete successfully." A round the peer
// announces that the operator did not configure, a configured round the peer does
// not announce, and a third round are each refused, so no partial authentication
// ever establishes. strongSwan's responder refuses the same three
// (ike_auth.c update_cfg_candidates and its strict check).
func (sa *SA) settleAnotherAuth(announced bool) error {
	switch sa.remoteRound {
	case authRoundFirst:
		configured := sa.remoteSecondRoundConfigured()
		if announced && !configured {
			return errRemoteRoundNotConfigured
		}
		if !announced && configured {
			return errRemoteRoundMissing
		}
		if !announced {
			return nil
		}
		if err := sa.beginRemoteSecondRound(); err != nil {
			return err
		}
		sa.peerRoundOwed = true
		return nil
	case authRoundSecond:
		if announced {
			return errThirdRound
		}
		sa.peerRoundOwed = false
		return nil
	}
	panic("BUG: unknown remote authentication round")
}

// verifySecondExchangeRound judges the peer's message of the second IKE_AUTH
// exchange. id, auth and announced are its identification payload, its AUTH and
// whether it carried ANOTHER_AUTH_FOLLOWS, each nil or false when absent. The
// caller MUST have stored the message's certificates first.
//
// A peer that owes no round sends a message with neither ID nor AUTH, RFC 4739
// Section 2.5: "HDR, SK { }". A peer that owes one sends "HDR, SK { IDi, [CERT+],
// AUTH }" (Section 2.4), and its AUTH covers that round's identity, RFC 4739
// Section 2.1: "IDi' refers to the latest IDi payload sent by the initiator, and IDr'
// refers to the latest IDr payload sent by the responder." The identity is recorded
// before verifyRemoteAuth runs, which is how computeSignedOctets reads it.
func verifySecondExchangeRound(sa *SA, id *wire.PayloadID, auth *wire.PayloadAUTH, announced bool, log *slog.Logger) error {
	if !sa.peerRoundOwed {
		if auth != nil {
			return errUnannouncedRound
		}
		if announced {
			return errUnannouncedRound
		}
		return nil
	}
	if auth == nil {
		return errRemoteRoundMissing
	}
	if id == nil {
		return errRoundWithoutID
	}
	sa.RemoteIDPayload = id
	if err := verifyRemoteAuth(sa, auth); err != nil {
		return err
	}
	asserted, _ := assertedIdentity(id)
	log.Info("ike: second authentication round verified", "peer", sa.PeerName, "id", asserted)
	return sa.settleAnotherAuth(announced)
}

// buildAuthRoundRequest builds the initiator's IKE_AUTH request of the second
// exchange: this side's second round when it runs one, otherwise a message with no
// payload at all, RFC 4739 Section 2.5: "HDR, SK { }". The Child SA was offered in
// the first request and is answered in the final response, so no SA, TSi or TSr
// goes here, and neither does MULTIPLE_AUTH_SUPPORTED, which only the first request
// carries.
func buildAuthRoundRequest(sa *SA) ([]byte, error) {
	var inner []wire.PayloadEntry
	if sa.localRound == authRoundSecond {
		auth, err := computeLocalAuth(sa)
		if err != nil {
			return nil, err
		}
		inner, err = roundIdentityPayloads(sa, true, auth)
		if err != nil {
			return nil, err
		}
	}
	return buildEncryptedMessage(sa, inner, sa.NextMsgID)
}

// continueInitiatorAuth sends the initiator's second IKE_AUTH request, once the
// first response verified and announced, or this side owes, another round. The SA
// stays in StateAuthSent and runInitiator retransmits the new request.
func continueInitiatorAuth(sa *SA, tr *transport.UDPTransport, log *slog.Logger) {
	if err := sa.beginSecondAuthExchange(); err != nil {
		log.Warn("ike: start the second IKE_AUTH exchange failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	// RFC 7296 Section 2.2: the second request is a new request, so it takes the next
	// Message ID, as each EAP round does.
	sa.advanceMsgID()
	req, err := buildAuthRoundRequest(sa)
	if err != nil {
		log.Warn("ike: build the second IKE_AUTH request failed", "peer", sa.PeerName, "error", err)
		sa.State = StateDead
		return
	}
	sa.LastSentMsg = req
	sa.RetransmitTime = time.Now().Add(retransmitBase)
	sa.RetransmitCount = 0
	sendRaw(sa, tr, req, log)
	log.Debug("ike: sent the second IKE_AUTH request", "peer", sa.PeerName,
		"local-round", sa.localRound == authRoundSecond, "remote-round", sa.peerRoundOwed)
}

// answerFirstAuthExchange sends the responder's IKE_AUTH response to a first request
// that verified when a second exchange follows. It carries this side's first round
// (IDr, [CERT], AUTH) and, when this side runs a second round, ANOTHER_AUTH_FOLLOWS.
// The Child SA offer is held for the final response.
//
// RFC 4739 Section 2.5 shows the shape: "HDR, SK { IDr, [CERT+], AUTH,
// N(ANOTHER_AUTH_FOLLOWS) }", and SA, TSi and TSr only in the last response. The
// initiator has not finished authenticating, so its endpoint is not adopted here.
func (ps *PeerSession) answerFirstAuthExchange(sa *SA, msgID uint32, remoteSAi2 *wire.PayloadSA, tsi, tsr *wire.PayloadTS, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	inner, err := buildAuthIdentityPayloads(sa, false)
	if err != nil {
		log.Warn("ike: build IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
		ps.respondAuthError(sa, msgID, notifyForRefusal(err), tr, remote, log)
		sa.State = StateDead
		return
	}
	// RFC 4739 Section 3.2: the notification goes in "an IKE_AUTH message containing an
	// AUTH payload", which this response is.
	if sa.localSecondRoundRuns() {
		inner = append(inner, wire.PayloadEntry{Payload: anotherAuthFollowsNotify()})
	}
	resp, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeIKEAuth, wire.FlagResponse)
	if err != nil {
		log.Warn("ike: build IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
		ps.respondAuthError(sa, msgID, notifyForRefusal(err), tr, remote, log)
		sa.State = StateDead
		return
	}
	if err := sa.beginSecondAuthExchange(); err != nil {
		log.Warn("ike: start the second IKE_AUTH exchange failed", "peer", sa.PeerName, "error", err)
		ps.sendAuthFailed(sa, msgID, tr, remote, log)
		sa.State = StateDead
		return
	}
	sa.heldSAi2, sa.heldTSi, sa.heldTSr = remoteSAi2, tsi, tsr
	cacheResponse(sa, msgID, resp)
	sa.LastSentMsg = resp
	// RFC 7296 Section 2.11: answered on the arrival socket, to the observed source.
	if err := sendReplyFrom(tr, resp, sa.replyLocal, remote); err != nil {
		log.Warn("ike: send IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
	}
	sa.State = StateAuthReceived
	log.Debug("ike: answered the first IKE_AUTH exchange, another follows", "peer", sa.PeerName)
}

// handleResponderAuthRound processes the initiator's IKE_AUTH request of the second
// exchange (StateAuthReceived), verifies the round it carries, and sends the final
// response: this side's second round when it runs one, then the Child SA answer to
// the offer of the first request. RFC 4739 Section 2.4: "HDR, SK { SA, TSi, TSr }"
// closes the exchange.
//
// A message that does not decrypt is dropped and the SA kept: RFC 7296 Section 2.21.4
// lets no unprotected message change an SA's state, and the half-open reaper ends a
// handshake the peer abandons.
func (ps *PeerSession) handleResponderAuthRound(sa *SA, msg *wire.Message, rawMsg []byte, tr *transport.UDPTransport, remote *net.UDPAddr, log *slog.Logger) {
	if msg.Header.MessageID != sa.ExpectedMsgID {
		log.Debug("ike: second IKE_AUTH request at an unexpected message id, dropped",
			"peer", sa.PeerName, "msgid", msg.Header.MessageID, "expected", sa.ExpectedMsgID)
		return
	}
	inner, err := decryptAndParse(sa, msg, rawMsg)
	if err != nil {
		log.Warn("ike: second IKE_AUTH request does not decrypt, dropped", "peer", sa.PeerName, "error", err)
		return
	}

	var id *wire.PayloadID
	var auth *wire.PayloadAUTH
	var certPayloads []*wire.PayloadCERT
	// RFC 7296 Section 2.5: the walk only collects, so payload order is never judged.
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadID:
			if p.IDPayloadType == wire.PayloadTypeIDi {
				id = p
			}
		case *wire.PayloadAUTH:
			auth = p
		case *wire.PayloadCERT:
			if acceptedCertEncoding(sa, p) {
				certPayloads = append(certPayloads, p)
			}
		}
	}
	if err := storeRemoteCerts(sa, certPayloads, log); err != nil {
		if errors.Is(err, errCertURLPending) {
			// The lookup runs on a worker, as on the first request: drop and let the
			// initiator's retransmission find the object cached.
			log.Debug("ike: hash-and-url lookup pending, dropping the second AUTH request", "peer", sa.PeerName)
			return
		}
		log.Warn("ike: peer certificate chain refused", "peer", sa.PeerName, "error", err)
		ps.sendAuthFailed(sa, msg.Header.MessageID, tr, remote, log)
		sa.State = StateDead
		return
	}
	announced := notifyOf(inner, wire.NotifyAnotherAuthFollows) != nil
	if err := verifySecondExchangeRound(sa, id, auth, announced, log); err != nil {
		log.Warn("ike: second IKE_AUTH request refused", "peer", sa.PeerName, "error", err)
		ps.sendAuthFailed(sa, msg.Header.MessageID, tr, remote, log)
		sa.State = StateDead
		return
	}

	var authInner []wire.PayloadEntry
	if sa.localRound == authRoundSecond {
		authInner, err = buildAuthIdentityPayloads(sa, false)
		if err != nil {
			log.Warn("ike: build the second IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
			ps.respondAuthError(sa, msg.Header.MessageID, notifyForRefusal(err), tr, remote, log)
			sa.State = StateDead
			return
		}
	}

	// Every round of the initiator verified, so its endpoint is corroborated. It is
	// adopted before the Child SA is installed, for the reason handleAuthRequest gives.
	sa.adoptAuthenticatedEndpoint(remote, tr.IsNATT(), log)
	resp, child, err := ps.buildFinalAuthResponse(sa, msg.Header.MessageID, authInner,
		sa.heldSAi2, sa.heldTSi, sa.heldTSr, false, log)
	if err != nil {
		log.Warn("ike: build the final IKE_AUTH response failed", "peer", sa.PeerName, "error", err)
		ps.respondAuthError(sa, msg.Header.MessageID, notifyForRefusal(err), tr, remote, log)
		sa.State = StateDead
		return
	}
	sa.heldSAi2, sa.heldTSi, sa.heldTSr = nil, nil, nil
	ps.finishResponderEstablish(sa, msg.Header.MessageID, resp, child, tr, remote, log)
}
