// Design: docs/architecture/core-design.md -- BMP plugin lifecycle
//
// Related: header.go -- wire format encode/decode
// Related: tlv.go -- TLV encode/decode
// Related: msg.go -- message type encode/decode

package bmp

import (
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// maxBMPMsgSize is the upper bound on a single BMP message.
// BGP max (4096) + BMP framing (48) with generous headroom for TLVs.
const maxBMPMsgSize = 65535

// sessionReadDeadline is the read deadline for receiver sessions.
// Ensures sessions are interruptible on shutdown.
const sessionReadDeadline = 30 * time.Second

// maxDedupPerPeer caps the dedup hash set per peer to bound memory.
// A full Internet table is ~1M prefixes; 100k covers realistic churn.
const maxDedupPerPeer = 100_000

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// receiverConfig holds parsed receiver configuration from environment { bmp { ... } }.
// YANG config tree delivers all values as strings (including booleans and numbers).
// YANG list with key is delivered as a map keyed by the key value.
type receiverConfig struct {
	Enabled     string                    `json:"enabled"`
	Servers     map[string]listenerConfig `json:"server"`
	MaxSessions string                    `json:"max-sessions"`
}

type listenerConfig struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

// senderConfig holds parsed sender configuration from bgp { bmp { sender { ... } } }.
// YANG list with key is delivered as a map keyed by the key value.
type senderConfig struct {
	Collectors            map[string]collectorConfig `json:"collector"`
	RouteMonitoringPolicy string                     `json:"route-monitoring-policy"`
	RouteMirroring        string                     `json:"route-mirroring"`
	StatisticsTimeout     string                     `json:"statistics-timeout"`
}

type collectorConfig struct {
	Address string `json:"address"`
	Port    string `json:"port"`
}

// environmentSection wraps the full environment config section.
// ExtractConfigSubtree returns {"environment": {"bmp": {...}}}, so we need
// two levels of wrapping.
type environmentSection struct {
	Environment *struct {
		BMP *receiverConfig `json:"bmp"`
	} `json:"environment"`
}

// bgpSenderSection wraps the full bgp config section.
// ExtractConfigSubtree returns {"bgp": {"bmp": {"sender": {...}}}}.
type bgpSenderSection struct {
	BGP *struct {
		BMP *struct {
			Sender *senderConfig `json:"sender"`
		} `json:"bmp"`
	} `json:"bgp"`
}

// openPair caches the actual BGP OPEN PDU bytes for a peer.
// Populated by OPEN message events, consumed by Peer Up on state change.
// RFC 7854 Section 4.10: Peer Up MUST include sent and received OPEN PDUs.
type openPair struct {
	sent     []byte // complete BGP OPEN (marker + length + type + body)
	received []byte // complete BGP OPEN (marker + length + type + body)
}

// BMPPlugin implements the bgp-bmp plugin.
// It manages both receiver (TCP listener for inbound BMP) and
// sender (outbound TCP to collectors) functionality.
//
// Caller MUST close stopCh and call stopListeners when done.
type BMPPlugin struct {
	plugin *sdk.Plugin
	mu     sync.RWMutex
	state  *bmpState

	// Receiver state.
	listeners []net.Listener
	sessions  sync.WaitGroup

	// Sender state.
	senders            []*senderSession
	routeMonitorPolicy string // "pre-policy", "post-policy", "all"
	routeMirroring     bool

	// openCache stores real OPEN PDUs per peer for Peer Up messages.
	// Key is peer address string. Populated by OPEN message events,
	// consumed by state events. Protected by mu.
	openCache map[string]*openPair

	// dedupState tracks per-peer UPDATE body hashes for Route Monitoring dedup.
	// Key: peer address. Value: set of FNV-64 hashes of RawBytes.
	// Cleared per-peer on peer-down. Protected by mu.
	// Capped at maxDedupPerPeer entries per peer to bound memory.
	dedupState map[string]map[uint64]struct{}

	// dedupHasher is pre-allocated FNV-64a hasher, reused via Reset().
	// Safe without locking: event handler is serial per senderSession.
	dedupHasher hash.Hash64

	// stopCh signals all background goroutines to stop.
	stopCh chan struct{}
}

// RunBMPPlugin is the in-process entry point for the bgp-bmp plugin.
func RunBMPPlugin(conn net.Conn) int {
	logger().Debug("bgp-bmp plugin starting")

	p := sdk.NewWithConn("bgp-bmp", conn)
	defer closeLog(p, "plugin")

	bp := &BMPPlugin{
		plugin:      p,
		state:       newBMPState(),
		openCache:   make(map[string]*openPair),
		dedupState:  make(map[string]map[uint64]struct{}),
		dedupHasher: fnv.New64a(),
		stopCh:      make(chan struct{}),
	}

	defer func() {
		close(bp.stopCh)
		bp.stopSenders()
		bp.stopListeners()
		bp.sessions.Wait()
	}()

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
		return bp.handleCommand(command)
	})

	// Structured event handler: receives peer state changes and UPDATE messages
	// from the reactor via DirectBridge. Used by the sender to stream BMP to collectors.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			bp.handleStructuredEvent(se)
		}
		return nil
	})

	// Subscribe to peer state (up/down), received/sent updates, and OPEN messages.
	// OPEN subscriptions cache real OPEN PDUs for Peer Up (RFC 7854 S4.10).
	// Notification/keepalive/refresh subscriptions support Route Mirroring (RFC 7854 S4.7).
	// All subscribed unconditionally: config loads after subscriptions, and
	// route-mirroring can be toggled via config reload. Cost is one type-check
	// per event when mirroring is disabled.
	p.SetStartupSubscriptions([]string{
		"state",
		"update direction received", "update direction sent",
		"open direction received", "open direction sent",
		"notification direction received", "notification direction sent",
		"keepalive direction received", "keepalive direction sent",
		"refresh direction received", "refresh direction sent",
	}, nil, "full")

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			switch section.Root {
			case "environment":
				rcv, err := parseReceiverConfig(section.Data)
				if err != nil {
					logger().Error("bmp: receiver config parse failed", "error", err)
					return err
				}
				if rcv.Enabled == "true" && len(rcv.Servers) > 0 {
					bp.startReceiver(rcv)
				}
			case "bgp":
				snd, err := parseSenderConfig(section.Data)
				if err != nil {
					logger().Error("bmp: sender config parse failed", "error", err)
					return err
				}
				if snd.RouteMonitoringPolicy != "" {
					bp.routeMonitorPolicy = snd.RouteMonitoringPolicy
				}
				bp.routeMirroring = snd.RouteMirroring == "true"
				if len(snd.Collectors) > 0 {
					bp.startSender(snd)
				}
			}
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "bmp sessions", Description: "Show BMP receiver sessions"},
			{Name: "bmp peers", Description: "Show monitored BGP peers"},
			{Name: "bmp collectors", Description: "Show BMP sender collector status"},
		},
		WantsConfig: []string{"bgp", "environment"},
	})
	if err != nil {
		logger().Error("bgp-bmp plugin failed", "error", err)
		return 1
	}

	return 0
}

// closeLog closes c and logs any error. Used in deferred cleanup.
func closeLog(c interface{ Close() error }, what string) {
	if err := c.Close(); err != nil {
		logger().Debug("bmp: close failed", "what", what, "error", err)
	}
}

// parseReceiverConfig extracts BMP receiver config from the environment section JSON.
// The JSON is {"environment": {"bmp": {...}}} (wrapped by ExtractConfigSubtree).
func parseReceiverConfig(data string) (*receiverConfig, error) {
	var sec environmentSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp receiver config: %w", err)
	}
	if sec.Environment == nil || sec.Environment.BMP == nil {
		return &receiverConfig{}, nil
	}
	return sec.Environment.BMP, nil
}

// parseSenderConfig extracts BMP sender config from the bgp section JSON.
// The JSON is {"bgp": {"bmp": {"sender": {...}}}} (wrapped by ExtractConfigSubtree).
// Returns a zero-value config (no collectors) when BMP sender is not configured.
func parseSenderConfig(data string) (*senderConfig, error) {
	var sec bgpSenderSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp sender config: %w", err)
	}
	if sec.BGP == nil || sec.BGP.BMP == nil || sec.BGP.BMP.Sender == nil {
		return &senderConfig{}, nil
	}
	return sec.BGP.BMP.Sender, nil
}

// startReceiver starts TCP listeners for the BMP receiver.
func (bp *BMPPlugin) startReceiver(cfg *receiverConfig) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	maxSess := parseUint16(cfg.MaxSessions, 100)
	for _, srv := range cfg.Servers {
		addr := net.JoinHostPort(srv.IP, srv.Port)
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			logger().Error("bmp: listener bind failed", "address", addr, "error", err)
			continue
		}
		bp.listeners = append(bp.listeners, ln)
		logger().Info("bmp: receiver listening", "address", addr)

		bp.sessions.Go(func() {
			bp.acceptLoop(ln, maxSess)
		})
	}
}

// stopListeners closes all receiver listeners.
func (bp *BMPPlugin) stopListeners() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, ln := range bp.listeners {
		if err := ln.Close(); err != nil {
			logger().Debug("bmp: listener close", "error", err)
		}
	}
	bp.listeners = nil
}

// startSender starts outbound TCP connections to BMP collectors.
func (bp *BMPPlugin) startSender(cfg *senderConfig) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for name, col := range cfg.Collectors {
		ss := newSenderSession(name, col)
		bp.senders = append(bp.senders, ss)
		bp.sessions.Go(ss.run)
		logger().Info("bmp: sender started", "collector", name, "address", col.Address, "port", col.Port)
	}
}

// stopSenders stops all sender sessions.
func (bp *BMPPlugin) stopSenders() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, ss := range bp.senders {
		ss.stop()
	}
	bp.senders = nil
}

// acceptLoop accepts BMP connections on the listener until it is closed.
func (bp *BMPPlugin) acceptLoop(ln net.Listener, maxSessions uint16) {
	var active atomic.Int32

	for {
		conn, err := ln.Accept()
		if err != nil {
			if bp.isStopping() {
				return
			}
			logger().Warn("bmp: accept failed", "error", err)
			return
		}

		// Increment before goroutine spawn to avoid TOCTOU race at the limit.
		if int(active.Add(1)) > int(maxSessions) {
			active.Add(-1)
			logger().Warn("bmp: max sessions reached, rejecting", "remote", conn.RemoteAddr())
			closeLog(conn, "rejected-conn")
			continue
		}

		bp.sessions.Go(func() {
			defer active.Add(-1)
			bp.handleSession(conn)
		})
	}
}

// isStopping returns true if the stop channel has been closed.
func (bp *BMPPlugin) isStopping() bool {
	select {
	case <-bp.stopCh:
		return true
	default: // active
		return false
	}
}

// handleSession processes a single BMP session from a remote router.
// RFC 7854: unidirectional, router -> receiver.
func (bp *BMPPlugin) handleSession(conn net.Conn) {
	defer closeLog(conn, "session")

	remote := conn.RemoteAddr().String()
	logger().Info("bmp: session started", "remote", remote)
	bp.state.addRouter(remote)
	defer bp.state.removeRouter(remote)
	defer logger().Info("bmp: session ended", "remote", remote)

	headerBuf := make([]byte, CommonHeaderSize)
	for {
		// Set read deadline so the loop is interruptible on shutdown.
		if err := conn.SetReadDeadline(time.Now().Add(sessionReadDeadline)); err != nil {
			return
		}

		// Read 6-byte common header.
		if _, err := io.ReadFull(conn, headerBuf); err != nil {
			if bp.isStopping() {
				return
			}
			logger().Debug("bmp: read header failed", "remote", remote, "error", err)
			return
		}

		ch, _, err := DecodeCommonHeader(headerBuf, 0)
		if err != nil {
			logger().Warn("bmp: bad header", "remote", remote, "error", err)
			return
		}

		msgLen := int(ch.Length)
		if msgLen < CommonHeaderSize {
			logger().Warn("bmp: invalid length", "remote", remote, "length", msgLen)
			return
		}
		if msgLen > maxBMPMsgSize {
			logger().Warn("bmp: message too large", "remote", remote, "length", msgLen, "max", maxBMPMsgSize)
			return
		}

		msgBuf := make([]byte, msgLen)
		copy(msgBuf, headerBuf)
		remaining := msgLen - CommonHeaderSize
		if remaining > 0 {
			if _, err := io.ReadFull(conn, msgBuf[CommonHeaderSize:]); err != nil {
				logger().Debug("bmp: read body failed", "remote", remote, "error", err)
				return
			}
		}

		msg, err := DecodeMsg(msgBuf)
		if err != nil {
			logger().Warn("bmp: decode failed", "remote", remote, "error", err)
			return
		}

		bp.processMessage(remote, msg)
	}
}

// handleCommand dispatches BMP CLI commands to the appropriate handler.
func (bp *BMPPlugin) handleCommand(command string) (string, string, error) {
	switch command {
	case "bmp sessions":
		return bp.state.sessionsCommand()
	case "bmp peers":
		return bp.state.peersCommand()
	case "bmp collectors":
		bp.mu.RLock()
		senders := bp.senders
		bp.mu.RUnlock()
		return bp.state.collectorsCommand(senders)
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
}

// processMessage dispatches a decoded BMP message to the appropriate handler.
func (bp *BMPPlugin) processMessage(remote string, msg any) {
	switch m := msg.(type) {
	case *Initiation:
		bp.processInitiation(remote, m)
	case *Termination:
		bp.processTermination(remote, m)
	case *PeerUp:
		bp.processPeerUp(remote, m)
	case *PeerDown:
		bp.processPeerDown(remote, m)
	case *RouteMonitoring:
		bp.processRouteMonitoring(remote, m)
	case *StatisticsReport:
		bp.processStatisticsReport(remote, m)
	case *RouteMirroring:
		bp.processRouteMirroring(remote, m)
	}
}

func (bp *BMPPlugin) processInitiation(remote string, m *Initiation) {
	var sysName, sysDescr string
	for _, tlv := range m.TLVs {
		switch tlv.Type { //nolint:exhaustive // RFC 7854: unknown TLV types are silently ignored
		case InitTLVSysName:
			sysName = string(tlv.Value)
			logger().Info("bmp: initiation", "remote", remote, "sysName", sysName)
		case InitTLVSysDescr:
			sysDescr = string(tlv.Value)
			logger().Info("bmp: initiation", "remote", remote, "sysDescr", sysDescr)
		case InitTLVString:
			logger().Info("bmp: initiation", "remote", remote, "message", string(tlv.Value))
		}
	}
	bp.state.setRouterInfo(remote, sysName, sysDescr)
}

func (bp *BMPPlugin) processTermination(remote string, _ *Termination) {
	logger().Info("bmp: termination received", "remote", remote)
}

func (bp *BMPPlugin) processPeerUp(remote string, m *PeerUp) {
	bp.state.peerUp(remote, m.Peer)
	logger().Info("bmp: peer up",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"peer-bgp-id", fmt.Sprintf("%08x", m.Peer.PeerBGPID),
		"local-port", m.LocalPort,
		"remote-port", m.RemotePort,
	)
}

func (bp *BMPPlugin) processPeerDown(remote string, m *PeerDown) {
	bp.state.peerDown(remote, m.Peer, m.Reason)
	logger().Info("bmp: peer down",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"reason", m.Reason,
	)
}

func (bp *BMPPlugin) processRouteMonitoring(remote string, m *RouteMonitoring) {
	logger().Debug("bmp: route monitoring",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"update-len", len(m.BGPUpdate),
	)
}

func (bp *BMPPlugin) processStatisticsReport(remote string, m *StatisticsReport) {
	logger().Debug("bmp: statistics report",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"stats-count", len(m.Stats),
	)
}

func (bp *BMPPlugin) processRouteMirroring(remote string, m *RouteMirroring) {
	logger().Debug("bmp: route mirroring",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"tlv-count", len(m.TLVs),
	)
}

// --- Sender event handling ---

// handleStructuredEvent processes a reactor event and forwards it to all sender sessions.
func (bp *BMPPlugin) handleStructuredEvent(se *rpc.StructuredEvent) {
	// Maintain internal state regardless of whether senders are connected.
	// Peers may establish before any collector connects (AC-3).
	switch se.EventType { //nolint:exhaustive // only open and state need pre-sender work
	case rpc.EventKindOpen:
		bp.cacheOpenPDU(se)
	case rpc.EventKindState:
		if se.State == rpc.SessionStateDown {
			bp.mu.Lock()
			delete(bp.openCache, se.PeerAddress)
			delete(bp.dedupState, se.PeerAddress)
			bp.mu.Unlock()
		}
	}

	bp.mu.RLock()
	senders := bp.senders
	bp.mu.RUnlock()

	if len(senders) == 0 {
		return
	}

	switch se.EventType { //nolint:exhaustive // BMP handles state, update, open, notification, keepalive, refresh
	case rpc.EventKindState:
		bp.handleSenderState(se, senders)
	case rpc.EventKindOpen:
		if bp.routeMirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindUpdate:
		// Filter by route-monitoring-policy:
		// "pre-policy" = received only, "post-policy" = sent only, "all" = both.
		policy := bp.routeMonitorPolicy
		if policy == "" {
			policy = "all"
		}
		switch {
		case policy == "all":
			bp.handleSenderUpdate(se, senders)
		case policy == "pre-policy" && se.Direction == rpc.DirectionReceived:
			bp.handleSenderUpdate(se, senders)
		case policy == "post-policy" && se.Direction == rpc.DirectionSent:
			bp.handleSenderUpdate(se, senders)
		}
		if bp.routeMirroring {
			bp.handleSenderMirror(se, senders)
		}
	case rpc.EventKindNotification, rpc.EventKindKeepalive, rpc.EventKindRefresh:
		if bp.routeMirroring {
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
	if rawBytes == nil || msgType != message.TypeOPEN {
		return
	}

	// RFC 7854 S4.10: Peer Up includes complete BGP OPEN messages.
	// Build full PDU: 16-byte marker + 2-byte length + 1-byte type + body.
	pduLen := message.HeaderLen + len(rawBytes)
	pdu := make([]byte, pduLen)
	copy(pdu, message.Marker[:])
	pdu[message.MarkerLen] = byte(pduLen >> 8)     //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+1] = byte(pduLen & 0xFF) //nolint:gosec // pduLen bounded by maxBMPMsgSize
	pdu[message.MarkerLen+2] = byte(message.TypeOPEN)
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

// handleSenderState sends Peer Up or Peer Down to all collectors.
func (bp *BMPPlugin) handleSenderState(se *rpc.StructuredEvent, senders []*senderSession) {
	peer := peerHeaderFromEvent(se)

	switch se.State { //nolint:exhaustive // only up/down are actionable for BMP
	case rpc.SessionStateUp:
		// RFC 7854 S4.10: Peer Up MUST include sent and received OPEN PDUs.
		// Use cached real OPENs from OPEN message events.
		bp.mu.RLock()
		pair := bp.openCache[se.PeerAddress]
		bp.mu.RUnlock()

		var sentOpen, recvOpen []byte
		if pair != nil {
			sentOpen = pair.sent
			recvOpen = pair.received
		}
		if sentOpen == nil || recvOpen == nil {
			logger().Warn("bmp: OPEN cache miss for peer, skipping Peer Up", "peer", se.PeerAddress)
			return
		}

		var localAddr [16]byte
		parseIPInto(se.LocalAddress, &localAddr)

		for _, ss := range senders {
			if err := ss.writePeerUp(peer, localAddr, 179, 0, sentOpen, recvOpen); err != nil {
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
	ip := net.ParseIP(addr)
	if ip == nil {
		return
	}
	ip16 := ip.To16()
	if ip16 != nil {
		copy(out[:], ip16)
	}
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

// parseUint16 parses a string to uint16, returning def on error or empty input.
func parseUint16(s string, def uint16) uint16 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return def
	}
	return uint16(v)
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
func rawUpdateBytes(se *rpc.StructuredEvent) ([]byte, message.MessageType) {
	msg, ok := se.RawMessage.(*bgptypes.RawMessage)
	if !ok || msg == nil {
		return nil, 0
	}
	return msg.RawBytes, msg.Type
}
