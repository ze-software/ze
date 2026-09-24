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
	"encoding/binary"
	"hash/fnv"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// --- Sender event handling ---

// handleStructuredEvent processes a reactor event and forwards it to all sender sessions.
func (bp *BMPPlugin) handleStructuredEvent(se *rpc.StructuredEvent) {
	bp.eventMu.Lock()
	defer bp.eventMu.Unlock()

	// cause is the Peer Down this event owes a collector, and only the down arm
	// below can build it: RFC 7854 Section 4.9 makes the Data field the
	// NOTIFICATION PDU that ended the session, and the same teardown drops that
	// PDU from the cache. The down arm of the sender switch is its only reader,
	// so the zero value this declaration holds for every other event is never
	// looked at.
	var cause peerDownCause

	// Maintain internal state regardless of whether senders are connected.
	// Peers may establish before any collector connects (AC-3).
	switch se.EventType { //nolint:exhaustive // only open, notification and state need pre-sender work
	case rpc.EventKindOpen:
		bp.cacheOpenPDU(se)
	case rpc.EventKindNotification:
		bp.cacheNotificationPDU(se)
	case rpc.EventKindState:
		switch se.State { //nolint:exhaustive // only up/down carry peer state
		case rpc.SessionStateUp:
			// Recorded even when no collector is connected: a collector that
			// connects later still has to be told this peer is up.
			bp.recordPeerUp(se)
		case rpc.SessionStateDown:
			cause = bp.clearPeerState(se)
		}
	}

	// ONE snapshot of the sender set and the three config leaves that decide what
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
	//
	// statistics-timeout joins them because it decides what is MEASURED rather
	// than what is emitted: the duplicate detector that feeds RFC 7854 Section
	// 4.8 Stat Type 13 has to run for a received UPDATE the policy does not
	// stream (handleSenderUpdate).
	bp.mu.RLock()
	senders := bp.senders
	mirroring := bp.routeMirroring
	policy := bp.routeMonitorPolicy
	statistics := bp.statisticsInterval > 0
	bp.mu.RUnlock()

	if se.EventType == rpc.EventKindUpdate {
		if err := bp.cacheAdjUpdate(se); err != nil {
			bp.replayFailure(se, err, senders)
			return
		}
	}

	if len(senders) == 0 {
		return
	}

	switch se.EventType { //nolint:exhaustive // BMP handles state, update, open, notification, keepalive, refresh
	case rpc.EventKindState:
		bp.handleSenderState(se, senders, cause)
	case rpc.EventKindOpen:
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindUpdate:
		bp.handleSenderUpdate(se, senders, policyStreams(policy, se.Direction), statistics)
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindNotification, rpc.EventKindKeepalive, rpc.EventKindRefresh:
		if mirroring {
			bp.handleSenderMirror(se, senders)
		}
	}
}

// bgpPDU builds a complete BGP message from a message body: the 16-byte
// marker, the 2-byte length, the 1-byte type, then the body (RFC 4271 Section
// 4.1). A StructuredEvent carries the body alone, and BMP asks for whole PDUs
// inside a Peer Up (RFC 7854 Section 4.10) and inside a Peer Down (Section
// 4.9), so both cache paths below build one here.
func bgpPDU(msgType msgtype.MessageType, body []byte) []byte {
	pduLen := message.HeaderLen + len(body)
	pdu := make([]byte, pduLen)
	copy(pdu, message.Marker[:])
	pdu[message.MarkerLen] = byte(pduLen >> 8)     //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+1] = byte(pduLen & 0xFF) //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+2] = byte(msgType)
	copy(pdu[message.HeaderLen:], body)
	return pdu
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
	pdu := bgpPDU(msgtype.TypeOPEN, rawBytes)

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

// cacheNotificationPDU caches the BGP NOTIFICATION PDU a peer exchanged, with
// the direction it traveled in, so the peer-down event that follows can carry
// it as the Data of its Peer Down.
//
// RFC 7854 Section 4.9: "Reason 1: The local system closed the session.
// Following the Reason is a BGP PDU containing a BGP NOTIFICATION message that
// would have been sent to the peer." A BGP PDU is the whole message, so the
// 19-byte header StructuredEvent does not carry is synthesized here, exactly as
// cacheOpenPDU does for the OPEN.
//
// The last NOTIFICATION wins. RFC 4271 Section 6 closes the connection as soon
// as one is sent or received, so a second one only exists when the two crossed
// on the wire, and the later event is the one that describes this teardown.
func (bp *BMPPlugin) cacheNotificationPDU(se *rpc.StructuredEvent) {
	rawBytes, msgType := rawUpdateBytes(se)
	if rawBytes == nil || msgType != msgtype.TypeNOTIFICATION {
		return
	}

	notify := &notifyPDU{
		pdu:  bgpPDU(msgtype.TypeNOTIFICATION, rawBytes),
		sent: se.Direction == rpc.DirectionSent,
	}

	bp.mu.Lock()
	if bp.notifyCache == nil {
		bp.notifyCache = make(map[string]*notifyPDU)
	}
	bp.notifyCache[se.PeerAddress] = notify
	bp.mu.Unlock()
}

// peerDownCause is the RFC 7854 Section 4.9 Reason and the Data that reason
// obliges, for one peer that has left Established.
type peerDownCause struct {
	peer   PeerHeader
	reason uint8
	data   []byte
}

// fsmEventNone is the Data of a reason 2 Peer Down.
//
// RFC 7854 Section 4.9: "Reason 2: The local system closed the session.  No
// notification message was sent.  Following the reason code is a 2-byte field
// containing the code corresponding to the Finite State Machine (FSM) Event
// that caused the system to close the session (see Section 8.1 of [RFC4271]).
// Two bytes both set to 0 are used to indicate that no relevant Event code is
// defined."
//
// Ze reports the zero form because the close reason it is handed names no FSM
// event: Peer.Run sends "session closed" or "connection lost"
// (internal/component/bgp/reactor/peer_run.go), and neither is an RFC 4271
// Section 8.1 event code. Reporting an event code ze did not observe would put
// a number the collector can act on behind a guess.
//
// Read-only: writePeerDown copies it into the message buffer.
var fsmEventNone = []byte{0, 0}

// clearPeerState drops everything the plugin holds for a peer that has left
// Established, and answers with the Peer Down that event owes the collectors.
//
// One function for the two jobs because they read one piece of state: the Data
// of a reason 1 or reason 3 Peer Down is the NOTIFICATION PDU, and this
// teardown is what removes it. Splitting them would leave the cache entry alive
// for a peer with no collector attached, or make the Peer Down read a cache the
// teardown had already emptied.
func (bp *BMPPlugin) clearPeerState(se *rpc.StructuredEvent) peerDownCause {
	bp.mu.Lock()
	notify := bp.notifyCache[se.PeerAddress]
	peer := peerHeaderFromEvent(se)
	if st := bp.peerUps[se.PeerAddress]; st != nil {
		// A teardown event can arrive after the reactor has cleared its
		// negotiated identity. Close the peer previously reported to BMP.
		peer = st.peer
		peer.TimestampSec, peer.TimestampUsec = 0, 0
	}
	delete(bp.openCache, se.PeerAddress)
	delete(bp.notifyCache, se.PeerAddress)
	delete(bp.dedupState, se.PeerAddress)
	delete(bp.dedupCount, se.PeerAddress)
	bp.forgetAdjRoutes(bp.peerUps[se.PeerAddress])
	delete(bp.peerUps, se.PeerAddress)
	bp.mu.Unlock()

	cause := peerDownFor(notify, se.Reason)
	cause.peer = peer
	return cause
}

// peerDownFor chooses the RFC 7854 Section 4.9 Reason, and the Data that reason
// makes mandatory, from what ze OBSERVED on the session.
//
// The NOTIFICATION decides, because the RFC ties the two together. Section 4.9
// reason 1 is "The local system closed the session.  Following the Reason is a
// BGP PDU containing a BGP NOTIFICATION message that would have been sent to
// the peer", and reason 3 is the same sentence for one "as received from the
// peer". A reason ze cannot supply the Data for is a reason ze must not send,
// so the cached PDU is what makes reason 1 and reason 3 reachable at all.
//
// The close-reason string answers the one case no NOTIFICATION describes: a
// peer the operator removed from the configuration. Reason 5 is "Information
// for this peer will no longer be sent to the monitoring station for
// configuration reasons", and the figure in the same section gives it no Data.
//
// Everything else is reason 2 with the two-byte zero event code (fsmEventNone).
func peerDownFor(notify *notifyPDU, reason string) peerDownCause {
	if notify != nil {
		if notify.sent {
			return peerDownCause{reason: PeerDownLocalNotify, data: notify.pdu}
		}
		return peerDownCause{reason: PeerDownRemoteNotify, data: notify.pdu}
	}
	if reason == rpc.ReasonPeerRemoved {
		return peerDownCause{reason: PeerDownDeconfigured}
	}
	return peerDownCause{reason: PeerDownLocalNoNotify, data: fsmEventNone}
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
		// RFC 7854 Section 4.10 reports the established TCP endpoints,
		// including the ephemeral port of an active opener.
		localPort:  se.LocalPort,
		remotePort: se.RemotePort,
		sentOpen:   sentOpen,
		recvOpen:   recvOpen,
	}
	parseIPInto(se.LocalAddress, &st.localAddr)

	bp.mu.Lock()
	if bp.peerUps == nil {
		bp.peerUps = make(map[string]*peerUpState)
	}
	bp.forgetAdjRoutes(bp.peerUps[se.PeerAddress])
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
	// run holds eventMu before writeMu. No peer transition or UPDATE can
	// interleave with the snapshot, even between storing state and fan-out.
	bp.mu.RLock()
	for _, st := range bp.peerUps {
		if err := ss.writePeerUpLocked(st.peer, st.localAddr, st.localPort, st.remotePort, st.sentOpen, st.recvOpen, nil); err != nil {
			bp.mu.RUnlock()
			ss.abortAdjReplay(err)
			return
		}
	}
	for _, st := range bp.peerUps {
		if err := bp.replayPeerLocked(ss, st, bp.routeMonitorPolicy); err != nil {
			bp.mu.RUnlock()
			ss.abortAdjReplay(err)
			return
		}
	}
	locRIB := bp.locRIBUnsub != nil
	bp.mu.RUnlock()
	if locRIB {
		bp.primeLocRIBPeerUp(ss)
	}
}

// bounceMonitoredPeers re-announces every established BGP peer on sessions that
// a config change altered the behavior of.
//
// RFC 8671 Section 7.2: "In case of any change that results in the alteration
// of behavior of an existing BMP session (i.e., changes to filtering and table
// names), the session MUST be bounced with a Peer Down/Peer Up sequence." What
// is bounced is each peer, not the transport: the BMP session stays up, so the
// collector keeps its connection and re-reads each peer's state under the new
// configuration. Ending the session instead would cost every collector a full
// re-dump of everything, including the state the change did not touch.
//
// A Peer Down implicitly withdraws the collector's table. The replacement
// Peer Up is therefore followed by a current snapshot and its per-family EOR,
// including when no routes changed while the configuration was applied.
//
// The reason code is PeerDownDeconfigured (RFC 7854 Section 4.9 reason 5,
// "Information for this peer will no longer be sent to the monitoring station
// for configuration reasons"). It is the only defined reason that reports a
// LOCAL configuration decision, and the only one whose text says outright that
// it "does not, strictly speaking, indicate that the peer has gone down". The
// BGP session is untouched here, so reason 2 (the local system closed the
// session) would tell the collector something false about the peer.
//
// A Peer Down implicitly withdraws every route the collector holds for that
// peer (RFC 7854 Section 4.9), so the per-peer Route Monitoring dedup state is
// dropped with it. Keeping it would suppress the next UPDATE that repeats a
// body the collector has just been made to forget.
//
// Peers are snapshotted under bp.mu and written outside it, which is a lock
// ORDER requirement rather than a preference: primeSender takes bp.mu while
// holding a session's writeMu, so taking writeMu under bp.mu here would invert
// the two. The snapshot cannot go stale meanwhile, because a peer state event
// and a config apply reach the plugin on the same delivery goroutine.
func (bp *BMPPlugin) bounceMonitoredPeers(senders []*senderSession) {
	bp.eventMu.Lock()
	defer bp.eventMu.Unlock()
	if len(senders) == 0 {
		return
	}

	bp.mu.Lock()
	states := make([]*peerUpState, 0, len(bp.peerUps))
	for address, st := range bp.peerUps {
		states = append(states, st)
		delete(bp.dedupState, address)
		delete(bp.dedupCount, address)
	}
	policy := bp.routeMonitorPolicy
	bp.mu.Unlock()

	if len(states) == 0 {
		return
	}

	for _, ss := range senders {
		// One critical section for the whole sequence: every other producer
		// takes writeMu before it can enqueue, so none can put a Route
		// Monitoring message between a peer's Peer Down and its Peer Up.
		ss.writeMu.Lock()
		for _, st := range states {
			if err := ss.writePeerDownLocked(st.peer, PeerDownDeconfigured, nil); err != nil {
				logger().Debug("bmp: peer bounce down failed", "collector", ss.name, "error", err)
				continue
			}
			if err := ss.writePeerUpLocked(st.peer, st.localAddr, st.localPort, st.remotePort, st.sentOpen, st.recvOpen, nil); err != nil {
				logger().Debug("bmp: peer bounce up failed", "collector", ss.name, "error", err)
				ss.abortAdjReplay(err)
				break
			}
			if err := bp.replayPeerLocked(ss, st, policy); err != nil {
				ss.abortAdjReplay(err)
				break
			}
		}
		ss.writeMu.Unlock()

		logger().Info("bmp: bounced peers after a sender configuration change",
			"collector", ss.name, "peers", len(states))
	}
}

// handleSenderState sends Peer Up or Peer Down to all collectors.
//
// cause carries the Peer Down reason and its Data, built by clearPeerState for
// this same event. It is read on the down arm alone.
func (bp *BMPPlugin) handleSenderState(se *rpc.StructuredEvent, senders []*senderSession, cause peerDownCause) {
	switch se.State { //nolint:exhaustive // only up/down are actionable for BMP
	case rpc.SessionStateUp:
		// RFC 7854 S4.10: Peer Up MUST include sent and received OPEN PDUs.
		// recordPeerUp (run just before this, from handleStructuredEvent) built
		// the message content from the cached real OPENs, and returns nothing
		// when either OPEN is missing.
		bp.mu.RLock()
		st := bp.peerUps[se.PeerAddress]
		policy := bp.routeMonitorPolicy
		bp.mu.RUnlock()
		if st == nil {
			logger().Warn("bmp: OPEN cache miss for peer, skipping Peer Up", "peer", se.PeerAddress)
			return
		}

		for _, ss := range senders {
			ss.writeMu.Lock()
			if err := ss.writePeerUpLocked(st.peer, st.localAddr, st.localPort, st.remotePort, st.sentOpen, st.recvOpen, nil); err != nil {
				logger().Debug("bmp: sender peer up failed", "collector", ss.name, "error", err)
			} else if err := bp.replayPeerLocked(ss, st, policy); err != nil {
				ss.abortAdjReplay(err)
			}
			ss.writeMu.Unlock()
		}
	case rpc.SessionStateDown:
		// RFC 7854 Section 4.9: "Data (present if Reason = 1, 2 or 3)". The
		// Data is not decoration: a collector that decodes to the figure reads
		// the NOTIFICATION PDU, or the 2-byte FSM event code, straight after
		// the reason byte, so a Peer Down that omits it for one of those three
		// reasons runs the collector off the end of the message. peerDownFor
		// pairs each reason with the Data it obliges.
		for _, ss := range senders {
			if err := ss.writePeerDown(cause.peer, cause.reason, cause.data); err != nil {
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

// policyStreams reports whether route-monitoring-policy streams direction as
// Route Monitoring: "pre-policy" carries the received direction (Adj-RIB-In),
// "post-policy" the sent direction (Adj-RIB-Out, RFC 8671), "all" carries both.
//
// An empty policy is the YANG default, which is "all". parseSenderConfig fills
// it, so an empty string here means no configuration has been installed yet.
func policyStreams(policy string, direction rpc.MessageDirection) bool {
	switch policy {
	case "", policyAll:
		return true
	case policyPrePolicy:
		return direction == rpc.DirectionReceived
	case policyPostPolicy:
		return direction == rpc.DirectionSent
	}
	return false
}

// handleSenderUpdate accounts for one UPDATE event and, when the
// route-monitoring policy streams its direction, sends Route Monitoring to
// every collector. The O flag in the per-peer header distinguishes the two
// directions.
//
// stream says the policy carries this direction. statistics says a periodic
// Statistics Report is configured, and it is what makes the duplicate detector
// run over a RECEIVED UPDATE the policy does not stream: the count it produces
// is RFC 7854 Section 4.8 Stat Type 13, and a counter that stopped being
// measured under `post-policy` would report a zero ze never measured
// (ai/rules/principles.md).
//
// Dedup: a body this peer already sent in this direction produces no Route
// Monitoring (AC-7). A body with different attributes passes (AC-8).
func (bp *BMPPlugin) handleSenderUpdate(se *rpc.StructuredEvent, senders []*senderSession, stream, statistics bool) {
	received := se.Direction == rpc.DirectionReceived
	measure := statistics && received
	if !stream && !measure {
		return
	}

	rawBytes, msgType := rawUpdateBytes(se)
	if rawBytes == nil {
		return
	}
	if bp.duplicateUpdate(se.PeerAddress, received, withdrawsRoutes(rawBytes), rawBytes) {
		return
	}
	if !stream {
		return
	}

	peer := peerHeaderFromEvent(se)
	for _, ss := range senders {
		if err := ss.writeRouteMonitoring(peer, msgType, rawBytes); err != nil {
			logger().Debug("bmp: sender route monitoring failed", "collector", ss.name, "error", err)
		}
	}
}

// withdrawsRoutes reports whether an UPDATE body retracts anything: a non-empty
// Withdrawn Routes field, or an MP_UNREACH_NLRI attribute.
//
// RFC 4271 Section 4.3 fixes the body layout this walks: "Withdrawn Routes
// Length: This 2-octet unsigned integer indicates the total length of the
// Withdrawn Routes field in octets." RFC 4760 Section 3 gives the
// multiprotocol form: "MP_UNREACH_NLRI (Type Code 15): This is an optional
// non-transitive attribute that can be used for the purpose of withdrawing
// multiple unfeasible routes from service."
//
// A body that does not parse answers TRUE rather than false. The answer gates
// a dedup memo, and the two mistakes do not cost the same: forgetting a body ze
// has seen costs one redundant Route Monitoring message, while keeping one
// costs the collector a route it believes withdrawn (ai/rules/principles.md).
// Nothing here indexes past len(body), so a hostile peer reaches an answer and
// never a panic.
func withdrawsRoutes(body []byte) bool {
	// Withdrawn Routes Length (2) and Total Path Attribute Length (2).
	if len(body) < 4 {
		return true
	}
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	if withdrawnLen > 0 {
		return true
	}

	off := 2 + withdrawnLen
	if off+2 > len(body) {
		return true
	}
	end := off + 2 + int(binary.BigEndian.Uint16(body[off:off+2]))
	off += 2
	if end > len(body) {
		return true
	}

	// RFC 4271 Section 4.3: each path attribute is "a triple <attribute type,
	// attribute length, attribute value>", where the type is two octets and
	// the Extended Length bit of the flags octet says whether the length is
	// one octet or two.
	for off < end {
		if off+2 > end {
			return true
		}
		extended := attribute.AttributeFlags(body[off])&attribute.FlagExtLength != 0
		code := body[off+1]
		off += 2

		var length int
		if extended {
			if off+2 > end {
				return true
			}
			length = int(binary.BigEndian.Uint16(body[off : off+2]))
			off += 2
		} else {
			if off+1 > end {
				return true
			}
			length = int(body[off])
			off++
		}

		if code == byte(attribute.AttrMPUnreachNLRI) {
			return true
		}
		off += length
		if off > end {
			return true
		}
	}
	return false
}

// duplicateUpdate reports whether this peer already carried this UPDATE body in
// this direction, and records the body when it did not. A duplicate in the
// RECEIVED direction also increments the peer's RFC 7854 Section 4.8 Stat Type
// 13 counter, "Number of duplicate update messages received".
//
// withdraws says this body retracts a route (withdrawsRoutes), and it is what
// keeps the memo from outliving the state it describes. RFC 7854 Section 5:
// "Ongoing monitoring is accomplished by propagating route changes in BGP
// Update PDUs and forwarding those PDUs to the monitoring station." A
// withdrawal contradicts every body the memo holds for that peer, so announce
// P, withdraw P, re-announce the byte-identical P must reach the collector;
// suppressing the third message leaves the collector holding the withdrawal
// while ze advertises the route. A body that WITHDRAWS therefore empties the
// peer's set before recording itself, which keeps the compression Section 4.6
// permits ("Route monitoring messages are state-compressed") for the repeat of
// the withdrawal itself and for any announcement that follows it.
//
// The set is emptied for both directions at once, because a peer's two
// directions share one map (dedupSentSalt). That errs toward sending: the
// Adj-RIB-Out stream loses some compression on a peer that withdraws, and no
// stream loses a change.
//
// The two directions occupy one hash set per peer, separated by dedupSentSalt,
// and that set is capped at maxDedupPerPeer entries. Past the cap a repeated
// body is neither suppressed nor counted, so the counter is a floor rather than
// an exact total on a peer churning more than 100k distinct bodies.
//
// Caller MUST NOT hold bp.mu. The hasher is reused without a lock because only
// the plugin's one event-delivery goroutine reaches this function (BMPPlugin,
// dedupHasher).
func (bp *BMPPlugin) duplicateUpdate(address string, received, withdraws bool, rawBytes []byte) bool {
	if bp.dedupState == nil {
		return false
	}
	if bp.dedupHasher == nil {
		bp.dedupHasher = fnv.New64a()
	}
	bp.dedupHasher.Reset()
	bp.dedupHasher.Write(rawBytes) //nolint:errcheck // hash.Hash.Write never returns an error, which its own documentation states
	key := dedupKey(bp.dedupHasher.Sum64(), received)

	bp.mu.Lock()
	defer bp.mu.Unlock()

	seen, ok := bp.dedupState[address]
	if !ok {
		seen = make(map[uint64]struct{})
		bp.dedupState[address] = seen
	}
	if _, duplicate := seen[key]; duplicate {
		if received {
			if bp.dedupCount == nil {
				bp.dedupCount = make(map[string]uint32)
			}
			bp.dedupCount[address]++
		}
		return true
	}
	if withdraws {
		clear(seen)
	}
	if len(seen) < maxDedupPerPeer {
		seen[key] = struct{}{}
	}
	return false
}

// peerHeaderFromEvent builds a BMP PeerHeader from a StructuredEvent.
// Sets flags based on event metadata:
//   - V flag: IPv6 peer address
//   - L flag: post-policy (sent direction)
//   - O flag: Adj-RIB-Out (sent direction, RFC 8671)
func peerHeaderFromEvent(se *rpc.StructuredEvent) PeerHeader {
	ph := PeerHeader{
		PeerType:  PeerTypeGlobal,
		PeerAS:    se.PeerAS,
		PeerBGPID: se.RemoteRouterID,
	}
	// RFC 7854 Section 5: "Otherwise, the BMP Timestamp field MUST be set
	// to 0, indicating that time is not available." Delivery time is not
	// receipt time: only the message's timestamp can date the routes.
	if msg, ok := se.RawMessage.(*bgptypes.RawMessage); ok && msg != nil {
		if !msg.Timestamp.IsZero() {
			ph.TimestampSec = uint32(msg.Timestamp.Unix())               //nolint:gosec // BMP timestamps are uint32 Unix seconds.
			ph.TimestampUsec = uint32(msg.Timestamp.Nanosecond() / 1000) //nolint:gosec // bounded to 999999.
		}
		if msg.WireUpdate != nil {
			ctx := bgpctx.Registry.Get(msg.WireUpdate.SourceCtxID())
			if ctx != nil && !ctx.ASN4() {
				ph.Flags |= PeerFlagA
			}
		}
	}

	if parseIPInto(se.PeerAddress, &ph.Address) {
		ph.Flags |= PeerFlagV
	}

	// RFC 8671: set O flag for Adj-RIB-Out (sent direction).
	// Also set L flag (post-policy) since sent updates have passed export policy.
	if se.Direction == rpc.DirectionSent {
		ph.Flags |= PeerFlagO | PeerFlagL
	}

	return ph
}

// parseIPInto writes the BMP address field and reports whether it is IPv6.
// RFC 7854 Section 4.2 requires IPv4's "12 most significant bytes zero-filled".
func parseIPInto(addr string, out *[16]byte) bool {
	parsed, err := netip.ParseAddr(addr)
	if err != nil {
		return false
	}
	parsed = parsed.Unmap()
	if parsed.Is4() {
		clear(out[:12])
		v4 := parsed.As4()
		copy(out[12:], v4[:])
		return false
	}
	*out = parsed.As16()
	return true
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
