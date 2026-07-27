// RFC: rfc/short/rfc7854.md
// Design: docs/architecture/core-design.md -- BMP plugin lifecycle
//
// Overview: bmp.go -- plugin lifecycle, config, receiver, sender set
// Related: sender.go -- the per-collector session these events are written to
//
// Turning reactor events into BMP messages: the OnStructuredEvent delivery loop
// and everything downstream of it (Peer Up/Down, Route Monitoring, Route
// Mirroring), plus the per-peer state a collector that connects later has to be
// told about.

package bmp

import (
	"hash/fnv"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// --- Sender event handling ---

// handleStructuredEvent processes a reactor event and forwards it to all sender sessions.
func (bp *BMPPlugin) handleStructuredEvent(se *rpc.StructuredEvent) {
	// Maintain internal state regardless of whether senders are connected.
	// Peers may establish before any collector connects (AC-3).
	switch se.EventType { //nolint:exhaustive // only open and state need pre-sender work
	case rpc.EventKindOpen:
		bp.cacheOpenPDU(se)
	case rpc.EventKindState:
		switch se.State { //nolint:exhaustive // only up/down carry peer state
		case rpc.SessionStateUp:
			// Recorded even when no collector is connected: a collector that
			// connects later still has to be told this peer is up.
			bp.recordPeerUp(se)
		case rpc.SessionStateDown:
			bp.mu.Lock()
			delete(bp.openCache, se.PeerAddress)
			delete(bp.dedupState, se.PeerAddress)
			delete(bp.peerUps, se.PeerAddress)
			bp.mu.Unlock()
		}
	}

	// ONE snapshot of the sender set and the two config leaves that decide what
	// this event produces, taken together under a single read lock.
	//
	// Together, not one atomic each: an event must be processed under one
	// configuration. Reading route-monitoring-policy from the config that was
	// live a microsecond ago and route-mirroring from the one that replaced it
	// would emit a message set matching neither -- Route Monitoring filtered by
	// the old policy alongside Route Mirroring enabled by the new one. Folding
	// them into the lock the sender set already needs also makes the config
	// snapshot coherent with the sessions it will be written to, and costs
	// nothing: it is the same critical section.
	bp.mu.RLock()
	senders := bp.senders
	mirroring := bp.routeMirroring
	policy := bp.routeMonitorPolicy
	bp.mu.RUnlock()

	if len(senders) == 0 {
		return
	}

	switch se.EventType { //nolint:exhaustive // BMP handles state, update, open, notification, keepalive, refresh
	case rpc.EventKindState:
		bp.handleSenderState(se, senders)
	case rpc.EventKindOpen:
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindUpdate:
		// Filter by route-monitoring-policy:
		// "pre-policy" = received only, "post-policy" = sent only, "all" = both.
		if policy == "" {
			policy = policyAll
		}
		switch {
		case policy == policyAll:
			bp.handleSenderUpdate(se, senders)
		case policy == policyPrePolicy && se.Direction == rpc.DirectionReceived:
			bp.handleSenderUpdate(se, senders)
		case policy == policyPostPolicy && se.Direction == rpc.DirectionSent:
			bp.handleSenderUpdate(se, senders)
		}
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindNotification, rpc.EventKindKeepalive, rpc.EventKindRefresh:
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	}
}

// cacheOpenPDU caches a real BGP OPEN PDU from an OPEN message event.
// RawMessage.RawBytes is the OPEN body (no 19-byte BGP header); we synthesize
// the full BGP OPEN PDU (marker + length + type + body) for Peer Up.
// Non-UPDATE RawBytes are independently allocated copies (reactor_notify.go),
// safe to hold beyond the event handler.
func (bp *BMPPlugin) cacheOpenPDU(se *rpc.StructuredEvent) {
	rawBytes, msgType := rawUpdateBytes(se)
	if rawBytes == nil || msgType != msgtype.TypeOPEN {
		return
	}

	// RFC 7854 S4.10: Peer Up includes complete BGP OPEN messages.
	// Build full PDU: 16-byte marker + 2-byte length + 1-byte type + body.
	pduLen := message.HeaderLen + len(rawBytes)
	pdu := make([]byte, pduLen)
	copy(pdu, message.Marker[:])
	pdu[message.MarkerLen] = byte(pduLen >> 8)     //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+1] = byte(pduLen & 0xFF) //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+2] = byte(msgtype.TypeOPEN)
	copy(pdu[message.HeaderLen:], rawBytes)

	bp.mu.Lock()
	pair, ok := bp.openCache[se.PeerAddress]
	if !ok {
		pair = &openPair{}
		bp.openCache[se.PeerAddress] = pair
	}
	if se.Direction == rpc.DirectionSent {
		pair.sent = pdu
	} else {
		pair.received = pdu
	}
	bp.mu.Unlock()
}

// recordPeerUp captures the Peer Up state of a peer that has just reached
// Established, so it can be emitted to any collector that is connected now and
// re-emitted to any collector that connects later. Records nothing when either
// OPEN PDU is missing: RFC 7854 Section 4.10 requires both, and a Peer Up
// without them would be malformed.
func (bp *BMPPlugin) recordPeerUp(se *rpc.StructuredEvent) {
	// The openPair FIELDS are read inside the lock, not just the map lookup:
	// cacheOpenPDU assigns pair.sent / pair.received under the write lock, so
	// reading them after releasing would be an unguarded read of shared state.
	// It happens to be safe today -- both run on the one event-delivery
	// goroutine -- but that is a property of the caller, not of this code.
	bp.mu.RLock()
	var sentOpen, recvOpen []byte
	if pair := bp.openCache[se.PeerAddress]; pair != nil {
		sentOpen, recvOpen = pair.sent, pair.received
	}
	bp.mu.RUnlock()

	if sentOpen == nil || recvOpen == nil {
		return
	}

	st := &peerUpState{
		peer: peerHeaderFromEvent(se),
		// Local port 179 and remote port 0 are what ze has always reported
		// here; StructuredEvent carries no port numbers.
		localPort:  179,
		remotePort: 0,
		sentOpen:   sentOpen,
		recvOpen:   recvOpen,
	}
	parseIPInto(se.LocalAddress, &st.localAddr)

	bp.mu.Lock()
	if bp.peerUps == nil {
		bp.peerUps = make(map[string]*peerUpState)
	}
	bp.peerUps[se.PeerAddress] = st
	bp.mu.Unlock()
}

// primeSender queues everything a freshly connected collector must be told
// before it can make sense of anything else: a Peer Up for every BGP peer that
// is currently established, and (when Loc-RIB monitoring is on) the RFC 9069
// Loc-RIB Peer Up. A BMP session carries no state across TCP connections, so
// without this the collector receives Route Monitoring for peers it never saw
// come up, and never learns about a peer that established while it was away.
//
// Runs on the session goroutine with ss.writeMu HELD (senderSession.onPrimed),
// which is what makes the ordering a guarantee rather than a race: every
// producer must take writeMu before it can enqueue, so none can get a message
// in front of these. Nothing here blocks on the socket -- the writes go into
// the session's transmit queue.
//
// Caller (run) MUST hold ss.writeMu.
func (bp *BMPPlugin) primeSender(ss *senderSession) {
	// The read lock is held ACROSS the writes, not just across a snapshot, so a
	// peer that goes down mid-prime is either still in the map (and gets its
	// Peer Up, immediately followed by the Peer Down the state event produces)
	// or already removed (and gets neither).
	//
	// Residual window, stated rather than hidden: the peer-down handler emits
	// its Peer Down without holding bp.mu, so a Peer Down enqueued between the
	// map delete and our write can still reach the collector before this Peer
	// Up. The collector then believes a dead peer is up until its next state
	// change. Closing that would need per-session send sequencing across two
	// unrelated code paths; it is not worth the coupling for a window this size.
	bp.mu.RLock()
	peers := 0
	for _, st := range bp.peerUps {
		if err := ss.writePeerUpLocked(st.peer, st.localAddr, st.localPort, st.remotePort, st.sentOpen, st.recvOpen); err != nil {
			logger().Debug("bmp: peer up resync failed", "collector", ss.name, "error", err)
			continue
		}
		peers++
	}
	locRIB := bp.locRIBUnsub != nil
	bp.mu.RUnlock()

	if peers > 0 {
		logger().Info("bmp: replayed peer up to collector session", "collector", ss.name, "peers", peers)
	}

	if locRIB {
		bp.primeLocRIBPeerUp(ss)
	}
}

// handleSenderState sends Peer Up or Peer Down to all collectors.
func (bp *BMPPlugin) handleSenderState(se *rpc.StructuredEvent, senders []*senderSession) {
	peer := peerHeaderFromEvent(se)

	switch se.State { //nolint:exhaustive // only up/down are actionable for BMP
	case rpc.SessionStateUp:
		// RFC 7854 S4.10: Peer Up MUST include sent and received OPEN PDUs.
		// recordPeerUp (run just before this, from handleStructuredEvent) built
		// the message content from the cached real OPENs, and returns nothing
		// when either OPEN is missing.
		bp.mu.RLock()
		st := bp.peerUps[se.PeerAddress]
		bp.mu.RUnlock()
		if st == nil {
			logger().Warn("bmp: OPEN cache miss for peer, skipping Peer Up", "peer", se.PeerAddress)
			return
		}

		for _, ss := range senders {
			if err := ss.writePeerUp(st.peer, st.localAddr, st.localPort, st.remotePort, st.sentOpen, st.recvOpen); err != nil {
				logger().Debug("bmp: sender peer up failed", "collector", ss.name, "error", err)
			}
		}
	case rpc.SessionStateDown:
		reason := peerDownReasonFromString(se.Reason)
		for _, ss := range senders {
			if err := ss.writePeerDown(peer, reason, nil); err != nil {
				logger().Debug("bmp: sender peer down failed", "collector", ss.name, "error", err)
			}
		}
	}
}

// handleSenderMirror sends a Route Mirroring message wrapping the verbatim
// BGP PDU to all collectors. RFC 7854 Section 4.7: TLV type 0 carries the
// complete BGP message (marker + length + type + body).
// Unlike Route Monitoring, nil body is valid (e.g. KEEPALIVE = header only).
func (bp *BMPPlugin) handleSenderMirror(se *rpc.StructuredEvent, senders []*senderSession) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil {
		return
	}

	peer := peerHeaderFromEvent(se)
	rawBytes := msg.RawBytes
	msgType := msg.Type
	for _, ss := range senders {
		if err := ss.writeRouteMirroring(peer, msgType, rawBytes); err != nil {
			logger().Debug("bmp: sender route mirroring failed", "collector", ss.name, "error", err)
		}
	}
}

// handleSenderUpdate sends Route Monitoring to all collectors.
// Handles both received (pre-policy, Adj-RIB-In) and sent (post-policy,
// Adj-RIB-Out per RFC 8671) updates. The O flag in the Per-Peer Header
// distinguishes the two directions.
// Per-NLRI dedup: suppresses Route Monitoring when the UPDATE body hash
// is unchanged for a given peer (AC-7). Different attributes pass (AC-8).
func (bp *BMPPlugin) handleSenderUpdate(se *rpc.StructuredEvent, senders []*senderSession) {
	rawBytes, msgType := rawUpdateBytes(se)
	if rawBytes == nil {
		return
	}

	if bp.dedupState != nil {
		if bp.dedupHasher == nil {
			bp.dedupHasher = fnv.New64a()
		}
		bp.dedupHasher.Reset()
		bp.dedupHasher.Write(rawBytes)
		sum := bp.dedupHasher.Sum64()

		bp.mu.Lock()
		peerMap, ok := bp.dedupState[se.PeerAddress]
		if !ok {
			peerMap = make(map[uint64]struct{})
			bp.dedupState[se.PeerAddress] = peerMap
		}
		if _, dup := peerMap[sum]; dup {
			bp.mu.Unlock()
			return
		}
		if len(peerMap) < maxDedupPerPeer {
			peerMap[sum] = struct{}{}
		}
		bp.mu.Unlock()
	}

	peer := peerHeaderFromEvent(se)
	for _, ss := range senders {
		if err := ss.writeRouteMonitoring(peer, msgType, rawBytes); err != nil {
			logger().Debug("bmp: sender route monitoring failed", "collector", ss.name, "error", err)
		}
	}
}

// peerHeaderFromEvent builds a BMP PeerHeader from a StructuredEvent.
// Sets flags based on event metadata:
//   - V flag: IPv6 peer address
//   - L flag: post-policy (sent direction)
//   - O flag: Adj-RIB-Out (sent direction, RFC 8671)
func peerHeaderFromEvent(se *rpc.StructuredEvent) PeerHeader {
	ph := PeerHeader{
		PeerType:     PeerTypeGlobal,
		PeerAS:       se.PeerAS,
		TimestampSec: uint32(time.Now().Unix()),
	}

	parseIPInto(se.PeerAddress, &ph.Address)

	// Check if IPv6 by looking for ':' in the address.
	for _, c := range se.PeerAddress {
		if c == ':' {
			ph.Flags |= PeerFlagV
			break
		}
	}

	// RFC 8671: set O flag for Adj-RIB-Out (sent direction).
	// Also set L flag (post-policy) since sent updates have passed export policy.
	if se.Direction == rpc.DirectionSent {
		ph.Flags |= PeerFlagO | PeerFlagL
	}

	return ph
}

// parseIPInto parses an IP string into a 16-byte BMP address field.
// IPv4 is stored as ::ffff:x.x.x.x per RFC 7854.
func parseIPInto(addr string, out *[16]byte) {
	parsed, err := netip.ParseAddr(addr)
	if err != nil {
		return
	}
	*out = parsed.As16()
}

// peerDownReasonFromString maps a ze close reason string to a BMP Peer Down reason code.
func peerDownReasonFromString(reason string) uint8 {
	switch reason {
	case "notification":
		return PeerDownLocalNotify
	case "tcp-failure", "timer-expired":
		return PeerDownLocalNoNotify
	case "remote-notification":
		return PeerDownRemoteNotify
	case "remote-close":
		return PeerDownRemoteNoData
	case "config-changed", "deconfigured":
		return PeerDownDeconfigured
	}
	return PeerDownLocalNoNotify // default for unknown reasons
}

// rawUpdateBytes returns the BGP message body bytes (without the 19-byte BGP
// header) and the BGP message type from a StructuredEvent, or (nil, 0) if
// not available. The BGP message header is synthesized downstream by
// writeRouteMonitoring using the returned msgType.
//
// se.RawMessage is interface{}-typed for SDK-protocol reasons, but in
// production it is always *bgptypes.RawMessage (set by server/events.go
// getStructuredEvent); msg.RawBytes is documented as the message body without
// marker/header, matching session_read.go body and session_write.go body.
func rawUpdateBytes(se *rpc.StructuredEvent) ([]byte, msgtype.MessageType) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil {
		return nil, 0
	}
	return msg.RawBytes, msg.Type
}
