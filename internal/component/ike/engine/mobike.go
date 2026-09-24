// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- authenticated endpoint mobility
// Related: inbound.go, established.go -- the single owner and request window
// RFC 4555 Sections 3.2, 3.5-3.9, 4.2 -- see rfc/short/rfc4555.md.
package engine

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// mobikeState belongs to one IKE SA. The owner loop alone changes it after AUTH.
// One pending request shares the normal window, retransmission budget and teardown.
// Ze advertises its current address only; responder selection remains configured.
type mobikeState struct {
	canMigrate bool
	offered    bool
	enabled    bool
	// RFC 4555 Section 1.3 retains the first IKE SA's role across IKE rekeys.
	originalInitiator bool
	local             *net.UDPAddr
	pending           *mobikeRequest
	updatePending     bool
	checkPending      bool
	lastKeepalive     time.Time
	natDestination    []byte
	natRetries        int
}

type mobikeRequest struct {
	cookie        []byte
	msgID         uint32
	local, remote *net.UDPAddr
	changed       bool
	update        bool
}

// mobikeAvailable gates negotiation on the capability captured when the session
// binds its sockets. A missing backend must not advertise live ESP migration.
func mobikeAvailable(sa *SA) bool {
	return sa.mobike.canMigrate && !wantsTransportMode(sa) && !sa.UseTransportMode && sa.nattSocket != nil
}

func mobikeAuthOffer(sa *SA) []wire.PayloadEntry {
	if !mobikeAvailable(sa) {
		return nil
	}
	if !sa.IsInitiator && !sa.mobike.enabled {
		return nil
	}
	// RFC 4555 Section 3.3: implementations supporting MOBIKE and NAT Traversal
	// "MUST change to port 4500 if the correspondent also supports both".
	// An initiator floats before the first IKE_AUTH, where support is announced.
	sa.floatToNATTPort()
	sa.mobike.offered = true
	sa.mobike.originalInitiator = sa.IsInitiator
	if sa.mobike.local == nil {
		local := &net.UDPAddr{IP: net.ParseIP(sa.PeerCfg.LocalAddress)}
		if addr, ok := sa.nattSocket.LocalAddr().(*net.UDPAddr); ok {
			local.Port = addr.Port
		}
		sa.recordMobikeLocal(local)
	}
	// RFC 4555 Section 4.2.1: "The notification data field MUST be left empty
	// (zero-length) when sending".
	return []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyMobikeSupported}}}
}

func (sa *SA) acceptMobikeOffer(inner []wire.PayloadEntry) {
	if !mobikeAvailable(sa) {
		return
	}
	if sa.IsInitiator && !sa.mobike.offered {
		return
	}
	if notifyOf(inner, wire.NotifyMobikeSupported) == nil {
		return
	}
	// RFC 4555 Section 4.2.1: "its contents (if any) MUST be ignored when this
	// notification is received". Presence negotiates support, data does not.
	sa.mobike.enabled = true
	sa.mobike.originalInitiator = sa.IsInitiator
	sa.floatToNATTPort()
}

func (sa *SA) recordMobikeLocal(local *net.UDPAddr) {
	if local == nil || local.IP.IsUnspecified() || local.IP == nil {
		return
	}
	sa.mobike.local = copyUDPAddr(local)
}

// localSendAddr preserves the configured source even when the NAT-T listener
// accepts mobility on a wildcard. An authenticated MOBIKE path supersedes it.
func (sa *SA) localSendAddr(out *transport.UDPTransport) *net.UDPAddr {
	if out == nil {
		return nil
	}
	bound, ok := out.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil
	}
	if sa.mobike.local != nil {
		if sa.mobike.local.Port == bound.Port {
			return sa.mobike.local
		}
		return &net.UDPAddr{IP: sa.mobike.local.IP, Port: bound.Port, Zone: sa.mobike.local.Zone}
	}
	if ip := net.ParseIP(sa.PeerCfg.LocalAddress); ip != nil {
		return &net.UDPAddr{IP: ip, Port: bound.Port}
	}
	return nil
}

// setMobikePath changes only the IKE send path. A successful COOKIE2 exchange MUST
// precede migration of the Child SA. An outstanding message keeps its wire bytes.
func setMobikePath(sa *SA, local, remote *net.UDPAddr) {
	if local == nil || remote == nil {
		return
	}
	changed := sa.mobike.local == nil || sa.peerEndpoint == nil
	if !changed {
		changed = !sameUDPEndpoint(sa.mobike.local, local) || !sameUDPEndpoint(sa.peerEndpoint, remote)
	}
	if !changed {
		return
	}
	sa.mobike.local = copyUDPAddr(local)
	sa.peerEndpoint = copyUDPAddr(remote)
	if sa.mobike.pending != nil {
		sa.mobike.pending.changed = true
	}
	if sa.mobike.originalInitiator {
		sa.mobike.updatePending = true
	} else {
		sa.mobike.checkPending = true
	}
}

func notifyOf(inner []wire.PayloadEntry, kind uint16) *wire.PayloadNotify {
	for _, entry := range inner {
		if n, ok := entry.Payload.(*wire.PayloadNotify); ok && n.NotifyMsgType == kind {
			return n
		}
	}
	return nil
}

// validateMobikeRequest reads the authenticated payloads without changing the SA.
// RFC 4555 Section 4.2.6 NO_NATS_ALLOWED Notification Data layout:
//
//	offset 0       4 (IPv4) / 16 (IPv6)      8 / 32          10 / 34
//	+-------------+-------------------------+---------------+-------------+
//	| source IP   | destination IP          | source port   | dest port   |
//	+-------------+-------------------------+---------------+-------------+
//	Total: 12 or 36 octets; both ports are network-byte-order uint16.
func validateMobikeRequest(sa *SA, inner []wire.PayloadEntry, remote, local *net.UDPAddr) uint16 {
	cookies := 0
	for _, entry := range inner {
		n, ok := entry.Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		switch n.NotifyMsgType {
		case wire.NotifyCookie2:
			if !sa.mobike.enabled {
				continue
			}
			cookies++
			// RFC 4555 Section 4.2.5: "The data associated with this notification
			// MUST be between 8 and 64 octets in length (inclusive)".
			if cookies > 1 || len(n.NotificationData) < 8 || len(n.NotificationData) > 64 {
				return wire.NotifyInvalidSyntax
			}
			if n.ProtocolID != 0 || len(n.SPI) != 0 {
				return wire.NotifyInvalidSyntax
			}
		case wire.NotifyNoNATsAllowed:
			// RFC 4555 Section 3.9: "The exchange responder MUST verify that the
			// contents of the NO_NATS_ALLOWED notification match the addresses in
			// the IP header." The protected ports are checked too (Section 4.2.6).
			if !noNATsAllowedMatches(n, remote, local) {
				return wire.NotifyUnexpectedNATDetected
			}
		}
	}
	if sa.PeerCfg.ProhibitNAT && notifyOf(inner, wire.NotifyNoNATsAllowed) == nil {
		// A peer permitted to traverse NAT may omit NO_NATS_ALLOWED, but it
		// cannot use that permission to override this side's prohibition.
		if notifyOf(inner, wire.NotifyUpdateSAAddresses) != nil ||
			notifyOf(inner, wire.NotifyAdditionalIP4Address) != nil ||
			notifyOf(inner, wire.NotifyAdditionalIP6Address) != nil ||
			notifyOf(inner, wire.NotifyNoAdditionalAddresses) != nil ||
			notifyOf(inner, wire.NotifyMobikeSupported) != nil {
			return wire.NotifyUnexpectedNATDetected
		}
	}
	return 0
}

func noNATsAllowedMatches(n *wire.PayloadNotify, remote, local *net.UDPAddr) bool {
	if remote == nil || local == nil || n.ProtocolID != 0 || len(n.SPI) != 0 {
		return false
	}
	data := n.NotificationData
	width := 16
	if remote.IP.To4() != nil && local.IP.To4() != nil {
		width = 4
	}
	if len(data) != 2*width+4 {
		return false
	}
	if !net.IP(data[:width]).Equal(remote.IP) || !net.IP(data[width:2*width]).Equal(local.IP) {
		return false
	}
	return int(binary.BigEndian.Uint16(data[2*width:])) == remote.Port &&
		int(binary.BigEndian.Uint16(data[2*width+2:])) == local.Port
}

// RFC 4555 Section 3.9: when NAT Traversal is not enabled, the first IKE_AUTH
// request and address-updating INFORMATIONAL requests "MUST also include a
// NO_NATS_ALLOWED notification". This records the actual chosen send tuple.
func noNATsAllowedPayload(local, remote *net.UDPAddr) (*wire.PayloadNotify, error) {
	if local == nil || remote == nil || local.Port <= 0 || local.Port > 65535 || remote.Port <= 0 || remote.Port > 65535 {
		return nil, fmt.Errorf("ike: NAT prohibition needs a concrete UDP tuple")
	}
	if (local.IP.To4() == nil) != (remote.IP.To4() == nil) {
		return nil, fmt.Errorf("ike: NAT prohibition needs same-family addresses")
	}
	source, destination := local.IP.To4(), remote.IP.To4()
	if source == nil {
		source, destination = local.IP.To16(), remote.IP.To16()
	}
	if source == nil || destination == nil || local.IP.IsUnspecified() || remote.IP.IsUnspecified() {
		return nil, fmt.Errorf("ike: NAT prohibition needs concrete addresses")
	}
	width := len(source)
	data := make([]byte, 2*width+4)
	copy(data, source)
	copy(data[width:], destination)
	binary.BigEndian.PutUint16(data[2*width:], uint16(local.Port))
	binary.BigEndian.PutUint16(data[2*width+2:], uint16(remote.Port))
	return &wire.PayloadNotify{NotifyMsgType: wire.NotifyNoNATsAllowed, NotificationData: data}, nil
}

func mobikeNATPayloads(sa *SA, local, remote *net.UDPAddr) []wire.PayloadEntry {
	if local == nil || remote == nil {
		return nil
	}
	return []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionSourceIP,
			NotificationData: transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, local.IP, uint16(local.Port))}},
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionDestIP,
			NotificationData: transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, remote.IP, uint16(remote.Port))}},
	}
}

func (sa *SA) recordMobikeNAT(inner []wire.PayloadEntry, local, remote *net.UDPAddr) bool {
	source := notifyOf(inner, wire.NotifyNATDetectionSourceIP)
	dest := notifyOf(inner, wire.NotifyNATDetectionDestIP)
	if source == nil || dest == nil || local == nil || remote == nil {
		return !sa.PeerCfg.ProhibitNAT || !sa.NATDetected
	}
	peerBehind := !natHashEqual(source.NotificationData,
		transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, remote.IP, uint16(remote.Port)))
	behind := !natHashEqual(dest.NotificationData,
		transport.NATDetectionHash(sa.InitiatorSPI, sa.ResponderSPI, local.IP, uint16(local.Port)))
	if sa.PeerCfg.ProhibitNAT && (peerBehind || behind) {
		return false
	}
	sa.PeerBehindNAT, sa.BehindNAT = peerBehind, behind
	sa.NATDetected = peerBehind || behind
	sa.mobike.natDestination = append(sa.mobike.natDestination[:0], dest.NotificationData...)
	return true
}

// mobikeResponsePayloads runs after validation. COOKIE2 is echoed byte-for-byte,
// whereas NAT detection hashes describe this response's actual packet tuple.
func mobikeResponsePayloads(sa *SA, inner []wire.PayloadEntry) []wire.PayloadEntry {
	if !sa.mobike.enabled {
		return nil
	}
	var reply []wire.PayloadEntry
	if cookie := notifyOf(inner, wire.NotifyCookie2); cookie != nil {
		// RFC 4555 Section 3.7: the recipient "MUST copy the notification as-is
		// to the response".
		reply = append(reply, wire.PayloadEntry{Payload: cookie})
	}
	if notifyOf(inner, wire.NotifyNATDetectionSourceIP) != nil || notifyOf(inner, wire.NotifyNATDetectionDestIP) != nil {
		// RFC 4555 Section 3.8: "if the request includes them, the responder MUST
		// also include them in the response".
		reply = append(reply, mobikeNATPayloads(sa, sa.replyLocal, sa.replyRemote)...)
	}
	return reply
}

// replyToMobikeRetransmit rechecks the observed tuple without reapplying an update.
// NAT prohibition also covers a retransmission translated after its first copy.
func replyToMobikeRetransmit(sa *SA, pkt transport.Packet, inner []wire.PayloadEntry, msgID uint32, exchange uint8, tr *transport.UDPTransport, log *slog.Logger) {
	sa.replyLocal, sa.replyRemote, sa.replyNATT = pkt.LocalAddr, pkt.RemoteAddr, pkt.NATT
	defer func() { sa.replyLocal, sa.replyRemote, sa.replyNATT = nil, nil, false }()
	if code := validateMobikeRequest(sa, inner, pkt.RemoteAddr, pkt.LocalAddr); code != 0 {
		respondMobikeError(sa, inner, msgID, exchange, code, tr, log)
		return
	}
	sendRaw(sa, tr, sa.lastResponse, log)
}

func respondMobikeError(sa *SA, inner []wire.PayloadEntry, msgID uint32, exchange uint8, code uint16, tr *transport.UDPTransport, log *slog.Logger) {
	if sa.lastResponseSet && sa.lastResponseID == msgID && sa.lastResponseMobikeError == code {
		sendRaw(sa, tr, sa.lastResponse, log)
		return
	}
	payloads := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: code}}}
	if cookie := notifyOf(inner, wire.NotifyCookie2); cookie != nil {
		if len(cookie.NotificationData) >= 8 && len(cookie.NotificationData) <= 64 {
			payloads = append(payloads, wire.PayloadEntry{Payload: cookie})
		}
	}
	response, err := buildEncryptedMessageEx(sa, payloads, msgID, exchange, initiatorFlag(sa)|wire.FlagResponse)
	if err != nil {
		log.Warn("ike: MOBIKE error response failed", "peer", sa.PeerName, "error", err)
		return
	}
	// Rejecting a changed retransmission cannot replace an already accepted
	// request's response. A later valid copy still receives the original bytes.
	if !sa.lastResponseSet || sa.lastResponseID != msgID {
		cacheResponse(sa, msgID, response)
		sa.lastResponseMobikeError = code
	}
	sendRaw(sa, tr, response, log)
}

func observeMobikeNATMapping(sa *SA, inner []wire.PayloadEntry) {
	if !sa.mobike.enabled || !sa.mobike.originalInitiator || !sa.BehindNAT {
		return
	}
	if dest := notifyOf(inner, wire.NotifyNATDetectionDestIP); dest != nil {
		if !natHashEqual(dest.NotificationData, sa.mobike.natDestination) {
			sa.mobike.updatePending = true
		}
	}
}

func (ps *PeerSession) acceptMobikeUpdate(sa *SA, inner []wire.PayloadEntry) uint16 {
	if !sa.mobike.enabled || notifyOf(inner, wire.NotifyUpdateSAAddresses) == nil {
		return 0
	}
	if sa.mobike.originalInitiator || sa.UseTransportMode || sa.replyLocal == nil || sa.replyRemote == nil {
		return wire.NotifyUnacceptableAddresses
	}
	if !sa.replyLocal.IP.IsGlobalUnicast() && !sa.replyLocal.IP.IsLoopback() {
		return wire.NotifyUnacceptableAddresses
	}
	if !sa.replyRemote.IP.IsGlobalUnicast() && !sa.replyRemote.IP.IsLoopback() {
		return wire.NotifyUnacceptableAddresses
	}
	if !sa.recordMobikeNAT(inner, sa.replyLocal, sa.replyRemote) {
		return wire.NotifyUnexpectedNATDetected
	}
	setMobikePath(sa, sa.replyLocal, sa.replyRemote)
	// A repeated update on the same pair can carry a changed NAT mapping.
	sa.mobike.checkPending = true
	return 0
}

func (ps *PeerSession) startMobikeRequest(sa *SA, tr *transport.UDPTransport, update bool, log *slog.Logger) error {
	if !sa.mobike.enabled || sa.mobike.pending != nil || !sa.requestWindowAvailable() {
		return nil
	}
	local, remote := sa.mobike.local, sa.remoteUDPAddr()
	if local == nil || remote == nil {
		return fmt.Errorf("ike: MOBIKE has no concrete address pair")
	}
	// RFC 4555 Section 4.2.5: COOKIE2 "MUST be chosen by the exchange initiator
	// in a way that is unpredictable to the exchange responder".
	cookie := make([]byte, 32)
	if _, err := rand.Read(cookie); err != nil {
		return fmt.Errorf("ike: COOKIE2 entropy: %w", err)
	}
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyCookie2, NotificationData: cookie}}}
	if update {
		inner = append(inner,
			wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyUpdateSAAddresses}},
			wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNoAdditionalAddresses}})
		if sa.PeerCfg.ProhibitNAT {
			notification, err := noNATsAllowedPayload(local, remote)
			if err != nil {
				return err
			}
			inner = append(inner, wire.PayloadEntry{Payload: notification})
		}
	}
	inner = append(inner, mobikeNATPayloads(sa, local, remote)...)
	if !sa.reserveRequestWindow() {
		return nil
	}
	msgID := sa.NextMsgID
	msg, err := buildEncryptedMessageEx(sa, inner, msgID, wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		sa.releaseRequestWindow()
		return err
	}
	sa.mobike.pending = &mobikeRequest{cookie: cookie, msgID: msgID,
		local: copyUDPAddr(local), remote: copyUDPAddr(remote), update: update}
	sa.mobike.updatePending = false
	sa.mobike.checkPending = false
	sa.armRequestRetransmit(msg)
	sa.advanceMsgID()
	sendRaw(sa, tr, msg, log)
	return nil
}

func (ps *PeerSession) handleMobikeResponse(sa *SA, inner []wire.PayloadEntry, msgID uint32, dp dataplane.Dataplane, tr *transport.UDPTransport, log *slog.Logger) ownedOutcome {
	pending := sa.mobike.pending
	if pending == nil || pending.msgID != msgID {
		return ownedOutcome{}
	}
	sa.answerAuthenticatedResponse(msgID)
	sa.mobike.pending = nil
	cookie := notifyOf(inner, wire.NotifyCookie2)
	// RFC 4555 Section 3.7: "When processing the response, the original sender
	// MUST verify that the value is the same one as sent. If the values do not
	// match, the IKE_SA MUST be closed." Absence fails the same comparison.
	if cookie == nil || subtle.ConstantTimeCompare(cookie.NotificationData, pending.cookie) != 1 {
		return mobikeFatal(sa, fmt.Errorf("ike: COOKIE2 return routability failed"), log)
	}
	if pending.changed {
		// RFC 4555 Section 3.6: "If the request to update the addresses is
		// retransmitted using several different source addresses, a new INFORMATIONAL
		// request MUST be sent." Section 3.7 also requires a fresh routability check
		// when a request used more than one destination. The old answer proves neither.
		if err := ps.startMobikeRequest(sa, tr, sa.mobike.originalInitiator, log); err != nil {
			return mobikeFatal(sa, err, log)
		}
		return ownedOutcome{}
	}
	if notifyOf(inner, wire.NotifyUnexpectedNATDetected) != nil {
		sa.mobike.natRetries++
		if sa.mobike.natRetries > maxRequestRetransmits {
			return mobikeFatal(sa, fmt.Errorf("ike: unexpected NAT persists"), log)
		}
		if err := ps.startMobikeRequest(sa, tr, pending.update, log); err != nil {
			return mobikeFatal(sa, err, log)
		}
		return ownedOutcome{}
	}
	for _, entry := range inner {
		if n, ok := entry.Payload.(*wire.PayloadNotify); ok && wire.NotifyIsError(n.NotifyMsgType) {
			return mobikeFatal(sa, fmt.Errorf("ike: peer refused MOBIKE update with notify %d", n.NotifyMsgType), log)
		}
	}
	sa.mobike.natRetries = 0
	if !sa.recordMobikeNAT(inner, pending.local, pending.remote) {
		return mobikeFatal(sa, fmt.Errorf("ike: NAT traversal is prohibited for this peer"), log)
	}
	if err := ps.migrateMobikeChild(sa, dp); err != nil {
		return mobikeFatal(sa, err, log)
	}
	log.Info("ike: MOBIKE address update", "peer", sa.PeerName, "local", pending.local, "remote", pending.remote)
	return ownedOutcome{peerAlive: true}
}

func mobikeFatal(sa *SA, err error, log *slog.Logger) ownedOutcome {
	log.Warn("ike: closing MOBIKE SA", "peer", sa.PeerName, "error", err)
	sa.State = StateDead
	return ownedOutcome{reestablish: true}
}

func (ps *PeerSession) migrateMobikeChild(sa *SA, dp dataplane.Dataplane) error {
	// handleOwnedInbound holds childLifecycleMu through this decision and the
	// migration. A completed parallel handshake has already transferred its
	// shared policy templates; the retiring SA must not move them again.
	if ps.getPendingChild() != nil {
		return nil
	}
	current := ps.getChildSA()
	if current != nil {
		policies := []dataplane.SPParams{childPolicyParams(current, dataplane.SADirIn), childPolicyParams(current, dataplane.SADirOut)}
		if err := ps.migrateMobikePair(sa, current, policies, dp); err != nil {
			return err
		}
	}
	// A superseded pair remains installed until Delete. Move distinct policies
	// too, but do not migrate policies it shares with the current pair twice.
	if old := ps.supersededChild; old != nil && old != current {
		var policies []dataplane.SPParams
		if !samePolicySelector(current, old) {
			policies = []dataplane.SPParams{childPolicyParams(old, dataplane.SADirIn), childPolicyParams(old, dataplane.SADirOut)}
		}
		if err := ps.migrateMobikePair(sa, old, policies, dp); err != nil {
			return err
		}
	}
	return nil
}

func (ps *PeerSession) migrateMobikePair(sa *SA, child *ChildSA, policies []dataplane.SPParams, dp dataplane.Dataplane) error {
	local, remote := sa.mobike.local, sa.remoteUDPAddr()
	if local == nil || remote == nil {
		return fmt.Errorf("ike: MOBIKE migration has no endpoint")
	}
	migrator, ok := dp.(dataplane.TunnelMigrator)
	if !ok {
		return fmt.Errorf("ike: MOBIKE migration: %w", dataplane.ErrNotSupported)
	}
	beginDataplaneWrite()
	defer endDataplaneWrite()
	if err := migrator.MigrateTunnel(dataplane.TunnelMigration{
		OldLocal: child.LocalAddr, OldRemote: child.RemoteAddr,
		NewLocal: local.IP, NewRemote: remote.IP,
		InboundSPI: child.InboundSPI, OutboundSPI: child.OutboundSPI,
		IfID: child.IfID, ReqID: child.ReqID,
		Policies:  policies,
		LocalPort: uint16(local.Port), RemotePort: uint16(remote.Port), NATDetected: sa.NATDetected,
	}); err != nil {
		return fmt.Errorf("ike: migrate Child SA: %w", err)
	}
	ps.mu.Lock()
	child.LocalAddr = append(net.IP(nil), local.IP...)
	child.RemoteAddr = append(net.IP(nil), remote.IP...)
	child.UDPEncap = true
	child.NATDetected = sa.NATDetected
	child.udpLocalPort = uint16(local.Port)
	child.udpRemotePort = uint16(remote.Port)
	ps.mu.Unlock()
	return nil
}

// serviceMobike uses the existing one-second owner tick. An unavailable source
// triggers route selection, then a protected update before any Child SA moves.
func (ps *PeerSession) serviceMobike(sa *SA, tr *transport.UDPTransport, dp dataplane.Dataplane, now time.Time, log *slog.Logger) {
	if !sa.mobike.enabled {
		return
	}
	if sa.mobike.originalInitiator {
		local, err := mobikeRouteSource(sa, tr)
		if err != nil {
			log.Debug("ike: MOBIKE source unavailable", "peer", sa.PeerName, "error", err)
		} else {
			setMobikePath(sa, local, sa.remoteUDPAddr())
		}
	}
	if sa.mobike.updatePending || sa.mobike.checkPending {
		if err := ps.startMobikeRequest(sa, tr, sa.mobike.updatePending, log); err != nil {
			mobikeFatal(sa, err, log)
		}
	}
	if sa.NATDetected && now.Sub(sa.mobike.lastKeepalive) >= transport.DefaultKeepaliveInterval {
		out, _ := sa.sendPath(tr)
		if out != nil {
			if err := out.SendFrom([]byte{0xff}, sa.mobike.local, sa.remoteUDPAddr()); err != nil {
				log.Debug("ike: MOBIKE NAT keepalive failed", "peer", sa.PeerName, "error", err)
			}
		}
		sa.mobike.lastKeepalive = now
	}
}

func mobikeRouteSource(sa *SA, tr *transport.UDPTransport) (*net.UDPAddr, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	if sa.mobike.local != nil {
		for _, address := range addresses {
			if network, ok := address.(*net.IPNet); ok && network.IP.Equal(sa.mobike.local.IP) {
				return sa.mobike.local, nil
			}
		}
	}
	remote := sa.remoteUDPAddr()
	if remote == nil {
		return nil, fmt.Errorf("ike: MOBIKE has no peer address")
	}
	conn, err := net.DialUDP("udp4", nil, remote)
	if err != nil {
		return nil, err
	}
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if err := conn.Close(); err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ike: MOBIKE route has no UDP source")
	}
	out, _ := sa.sendPath(tr)
	if out == nil {
		return nil, fmt.Errorf("ike: MOBIKE has no send socket")
	}
	bound, ok := out.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, fmt.Errorf("ike: MOBIKE socket has no UDP port")
	}
	local.Port = bound.Port
	return local, nil
}
