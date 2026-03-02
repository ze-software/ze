// Design: docs/architecture/core-design.md — BGP reactor event loop and peer management
// Detail: reactor_api.go — reactorAPIAdapter for plugin integration
// Detail: reactor_wire.go — zero-allocation wire UPDATE builders
//
// Package reactor implements the BGP reactor - the main orchestrator
// that manages peer sessions, connections, and signal handling.
package reactor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/commit"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/handler"
	bgpserver "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/server"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wireu"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/sim"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// reactorLogger is the reactor subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.reactor environment variable.
var reactorLogger = slogutil.LazyLogger("bgp.reactor")

// routesLogger is the routes subsystem logger (lazy initialization).
// Controlled by ze.log.bgp.routes environment variable.
var routesLogger = slogutil.LazyLogger("bgp.routes")

// collisionResolutionTimeout is the maximum time to wait for an existing
// session to close during connection collision resolution (RFC 4271 §6.8).
const collisionResolutionTimeout = 5 * time.Second

// Reactor errors.
var (
	ErrAlreadyRunning = errors.New("reactor already running")
	ErrNotRunning     = errors.New("reactor not running")
	ErrPeerExists     = errors.New("peer already exists")
	ErrPeerNotFound   = errors.New("peer not found")
	ErrNoConfigPath   = errors.New("config path not set")
	ErrNoReloadFunc   = errors.New("reload function not set")
)

// Config holds reactor configuration.
type Config struct {
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:179").
	//
	// Deprecated: Use Port with per-peer LocalAddress instead.
	ListenAddr string

	// Port is the BGP port to listen on (default 179).
	// Used with per-peer LocalAddress to create listeners.
	Port int

	// RouterID is the local BGP router identifier.
	RouterID uint32

	// LocalAS is the local AS number.
	LocalAS uint32

	// APISocketPath is the path to the Unix socket for API communication.
	// If empty, API server is not started.
	APISocketPath string

	// Plugins defines external plugin processes for API communication.
	Plugins []PluginConfig

	// ConfigDir is the directory containing the config file.
	// Used as working directory for process execution.
	ConfigDir string

	// ConfigPath is the path to the config file.
	// Required for Reload() to work.
	ConfigPath string

	// ConfigTree is the full config as a map for plugin JSON delivery.
	// Plugins request specific roots (e.g., "bgp") and receive that subtree as JSON.
	ConfigTree map[string]any

	// RecentUpdateMax is the maximum number of cached updates (soft limit).
	// Default: 1000000. Zero means no limit (not recommended).
	RecentUpdateMax int

	// MaxSessions limits how many peer sessions can complete before shutdown.
	// When > 0, reactor stops after this many sessions end (useful for testing).
	// Default: 0 (unlimited - run forever).
	MaxSessions int

	// ConfiguredFamilies lists all address families configured on peers.
	// Used for deferred auto-loading of family plugins after explicit plugins register.
	ConfiguredFamilies []string
}

// PluginConfig holds plugin configuration.
type PluginConfig struct {
	Name          string
	Run           string // Command to run (empty for internal plugins)
	Encoder       string
	Respawn       bool
	ReceiveUpdate bool          // Forward received UPDATEs to plugin stdin
	StageTimeout  time.Duration // Per-stage timeout (0 = use default 5s)
	Internal      bool          // If true, run in-process via goroutine (ze.X plugins)
}

// ReloadFunc is called by Reload() to get the list of peers from config file.
// The function should re-parse the config file and return full PeerSettings.
// This ensures reloaded peers have identical configuration to initially loaded peers.
type ReloadFunc func(configPath string) ([]*PeerSettings, error)

// Stats holds reactor statistics.
type Stats struct {
	StartTime time.Time
	Uptime    time.Duration
	PeerCount int
}

// ConnectionCallback is called when a connection is matched to a peer.
type ConnectionCallback func(conn net.Conn, settings *PeerSettings)

// MessageReceiver receives raw BGP messages from peers.
// Messages are passed as any (bgptypes.RawMessage) for on-demand parsing based on format config.
type MessageReceiver interface {
	// OnMessageReceived is called when a BGP message is received from a peer.
	// peer contains full peer information for proper JSON encoding.
	// msg is bgptypes.RawMessage (typed as any to match plugin.Server signature).
	// Returns the count of cache-consumer plugins that successfully received the event.
	OnMessageReceived(peer plugin.PeerInfo, msg any) int

	// OnMessageBatchReceived handles a batch of received BGP messages from the same peer.
	// msgs is []bgptypes.RawMessage (typed as []any to match plugin.Server signature).
	// Returns per-message cache-consumer counts for Activate calls.
	OnMessageBatchReceived(peer plugin.PeerInfo, msgs []any) []int

	// OnMessageSent is called when a BGP message is sent to a peer.
	// Only UPDATE messages trigger sent events.
	// msg is bgptypes.RawMessage (typed as any to match plugin.Server signature).
	OnMessageSent(peer plugin.PeerInfo, msg any)
}

// PeerLifecycleObserver receives peer state change notifications.
// Observers are called synchronously in registration order.
// Implementations MUST NOT block; use goroutine for slow processing.
type PeerLifecycleObserver interface {
	OnPeerEstablished(peer *Peer)
	OnPeerClosed(peer *Peer, reason string)
}

// Reactor is the main BGP orchestrator.
//
// It manages:
//   - Peer connections (outgoing)
//   - Listener for incoming connections
//   - Signal handling
//   - Graceful shutdown
//   - API server for external communication
//   - RIB (Routing Information Base) for route storage
type Reactor struct {
	config *Config

	// Injectable abstractions for simulation.
	clock           sim.Clock
	dialer          sim.Dialer
	listenerFactory sim.ListenerFactory

	peers           map[string]*Peer     // keyed by "addr:port" (PeerKey format)
	listener        *Listener            // deprecated: single listener for backward compat
	listeners       map[string]*Listener // keyed by "addr:port" (local endpoint)
	signals         *SignalHandler
	api             *pluginserver.Server       // API server for CLI and external processes
	eventDispatcher *bgpserver.EventDispatcher // BGP event dispatch to plugins

	// Recent UPDATE cache for efficient forwarding via update-id
	recentUpdates *RecentUpdateCache

	// Per-destination-peer forward pool for async UPDATE forwarding.
	// ForwardUpdate dispatches pre-computed send ops here instead of
	// doing synchronous TCP writes, eliminating head-of-line blocking.
	fwdPool *fwdPool

	// Config tree for plugin JSON delivery
	configTree map[string]any

	connCallback    ConnectionCallback
	messageReceiver MessageReceiver // Receives raw BGP messages

	// Peer lifecycle observers (called on state transitions)
	peerObservers []PeerLifecycleObserver
	observersMu   sync.RWMutex

	running        bool
	startTime      time.Time
	sessionCount   int32 // Number of completed sessions (for MaxSessions)
	sessionCountMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu sync.RWMutex

	// API process synchronization state.
	// Embedded to access fields directly (e.g., r.apiStarted).
	APISyncState

	// reloadFunc is called by Reload() to get the list of peers from config.
	// Set via SetReloadFunc. If nil, Reload() returns an error.
	reloadFunc ReloadFunc
}

// New creates a new reactor with the given configuration.
func New(config *Config) *Reactor {
	maxEntries := config.RecentUpdateMax
	if maxEntries == 0 {
		maxEntries = 1000000 // Default: 1M entries
	}

	// ZE_FWD_CHAN_SIZE overrides the per-destination forward pool channel capacity.
	// Default: 64. Invalid/zero/negative values use default.
	fwdChanSize := 0
	if v := os.Getenv("ZE_FWD_CHAN_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			fwdChanSize = n
		} else {
			reactorLogger().Warn("ignoring invalid ZE_FWD_CHAN_SIZE", "value", v, "error", err)
		}
	}

	r := &Reactor{
		config:          config,
		clock:           sim.RealClock{},
		dialer:          &sim.RealDialer{},
		listenerFactory: sim.RealListenerFactory{},
		peers:           make(map[string]*Peer),
		listeners:       make(map[string]*Listener),
		recentUpdates:   NewRecentUpdateCache(maxEntries),
		fwdPool:         newFwdPool(fwdBatchHandler, fwdPoolConfig{chanSize: fwdChanSize}),
		configTree:      config.ConfigTree,
	}

	// ZE_CACHE_SAFETY_VALVE overrides the safety valve duration for gap-based eviction.
	// Default: 5 minutes. Invalid values are ignored.
	if v := os.Getenv("ZE_CACHE_SAFETY_VALVE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			r.recentUpdates.SetSafetyValveDuration(d)
		} else {
			reactorLogger().Warn("ignoring invalid ZE_CACHE_SAFETY_VALVE", "value", v, "error", err)
		}
	}

	return r
}

// SetClock sets the clock used by the reactor and all child components.
// Must be called before StartWithContext. Propagates to all existing peers
// so that reconnect timers and session polling use the correct clock.
func (r *Reactor) SetClock(c sim.Clock) {
	r.clock = c
	r.recentUpdates.SetClock(c)
	r.fwdPool.SetClock(c)
	for _, p := range r.peers {
		p.SetClock(c)
	}
}

// SetDialer sets the dialer used for outbound connections.
// Must be called before StartWithContext.
func (r *Reactor) SetDialer(d sim.Dialer) {
	r.dialer = d
}

// SetListenerFactory sets the factory used to create listeners.
// Must be called before StartWithContext.
func (r *Reactor) SetListenerFactory(f sim.ListenerFactory) {
	r.listenerFactory = f
}

// findPeerByAddr looks up a peer by address, trying default port first.
// Falls back to iterating peers by IP for non-standard port peers.
// Must be called with r.mu held (RLock or Lock).
func (r *Reactor) findPeerByAddr(addr netip.Addr) (*Peer, bool) {
	// Fast path: default port (standard BGP)
	if peer, ok := r.peers[PeerKeyFromAddrPort(addr, DefaultBGPPort)]; ok {
		return peer, true
	}
	// Slow path: search by IP (custom per-peer ports)
	for _, peer := range r.peers {
		if peer.Settings().Address == addr {
			return peer, true
		}
	}
	return nil, false
}

// findPeerKeyByAddr looks up a peer's map key and peer by address.
// Must be called with r.mu held.
func (r *Reactor) findPeerKeyByAddr(addr netip.Addr) (string, *Peer, bool) {
	key := PeerKeyFromAddrPort(addr, DefaultBGPPort)
	if peer, ok := r.peers[key]; ok {
		return key, peer, true
	}
	for k, peer := range r.peers {
		if peer.Settings().Address == addr {
			return k, peer, true
		}
	}
	return "", nil, false
}

// peerListenPort returns the port to listen on for a peer.
// Peers with custom ports get dedicated listeners; others share the global port.
func (r *Reactor) peerListenPort(s *PeerSettings) int {
	if s.Port != 0 && s.Port != DefaultBGPPort {
		return int(s.Port)
	}
	return r.config.Port
}

// SetReloadFunc sets the function that will be called to reload config.
// This must be set before Start() for SIGHUP reload to work.
func (r *Reactor) SetReloadFunc(fn ReloadFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reloadFunc = fn
}

// SetConfigPath sets the config file path for reload.
func (r *Reactor) SetConfigPath(path string) {
	r.config.ConfigPath = path
}

// Running returns true if the reactor is running.
func (r *Reactor) Running() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// Peers returns all configured peers.
func (r *Reactor) Peers() []*Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peers := make([]*Peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	return peers
}

// PluginNames returns the names of all configured plugins.
func (r *Reactor) PluginNames() []string {
	names := make([]string, len(r.config.Plugins))
	for i, p := range r.config.Plugins {
		names[i] = p.Name
	}
	return names
}

// ListenAddr returns the listener's bound address.
//
// Deprecated: Use ListenAddrs() for multi-listener support.
func (r *Reactor) ListenAddr() net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return legacy listener if set
	if r.listener != nil {
		return r.listener.Addr()
	}
	// Return first listener from multi-listener map (for backward compat)
	for _, l := range r.listeners {
		if addr := l.Addr(); addr != nil {
			return addr
		}
	}
	return nil
}

// ListenAddrs returns all addresses the reactor is listening on.
func (r *Reactor) ListenAddrs() []net.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()

	addrs := make([]net.Addr, 0, len(r.listeners)+1)

	// Include legacy listener if set
	if r.listener != nil {
		if addr := r.listener.Addr(); addr != nil {
			addrs = append(addrs, addr)
		}
	}

	// Include all multi-listeners
	for _, l := range r.listeners {
		if addr := l.Addr(); addr != nil {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

// Stats returns current reactor statistics.
func (r *Reactor) Stats() *Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &Stats{
		StartTime: r.startTime,
		PeerCount: len(r.peers),
	}
	if r.running {
		stats.Uptime = time.Since(r.startTime)
	}
	return stats
}

// SetConnectionCallback sets the callback for matched incoming connections.
func (r *Reactor) SetConnectionCallback(cb ConnectionCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connCallback = cb
}

// SetMessageReceiver sets the receiver for raw BGP messages.
// When set, OnMessageReceived is called with raw wire bytes for all message types.
// This allows the receiver to control parsing based on format configuration.
func (r *Reactor) SetMessageReceiver(receiver MessageReceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageReceiver = receiver
}

// AddPeerObserver registers an observer for peer lifecycle events.
// Observers are called synchronously in registration order.
// MUST NOT block; use goroutine for slow processing.
func (r *Reactor) AddPeerObserver(obs PeerLifecycleObserver) {
	r.observersMu.Lock()
	defer r.observersMu.Unlock()
	r.peerObservers = append(r.peerObservers, obs)
}

// notifyPeerEstablished calls all observers when peer reaches Established.
func (r *Reactor) notifyPeerEstablished(peer *Peer) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerEstablished(peer)
	}
}

// notifyPeerNegotiated sends negotiated capabilities to subscribed plugins.
// Called after OPEN exchange completes and peer reaches Established.
func (r *Reactor) notifyPeerNegotiated(peer *Peer, neg *capability.Negotiated) {
	if r.eventDispatcher == nil || neg == nil {
		return
	}

	peerInfo := plugin.PeerInfo{
		Address:      peer.settings.Address,
		LocalAddress: peer.settings.LocalAddress,
		PeerAS:       peer.settings.PeerAS,
		LocalAS:      peer.settings.LocalAS,
	}

	decoded := format.NegotiatedToDecoded(neg)
	r.eventDispatcher.OnPeerNegotiated(peerInfo, decoded)
}

// notifyPeerClosed calls all observers when peer leaves Established.
func (r *Reactor) notifyPeerClosed(peer *Peer, reason string) {
	r.observersMu.RLock()
	observers := r.peerObservers
	r.observersMu.RUnlock()

	for _, obs := range observers {
		obs.OnPeerClosed(peer, reason)
	}

	// Track session count for MaxSessions feature (tcp.once/tcp.attempts)
	if r.config.MaxSessions > 0 {
		r.sessionCountMu.Lock()
		r.sessionCount++
		count := r.sessionCount
		r.sessionCountMu.Unlock()

		if int(count) >= r.config.MaxSessions {
			// MaxSessions reached - trigger shutdown
			go r.Stop()
		}
	}
}

// notifyMessageReceiver notifies the message receiver of a raw BGP message.
// Called from session when a BGP message is sent or received.
// peerAddr is used to look up full PeerInfo from the peers map.
// wireUpdate is non-nil for received UPDATE messages (zero-copy path).
// ctxID is the encoding context for zero-copy decisions.
// direction is "sent" or "received".
// buf is the pool buffer for received messages (nil for sent).
// Returns true if buf ownership was taken (caller should not return to pool).
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *wireu.WireUpdate, ctxID bgpctx.ContextID, direction string, buf []byte) bool {
	r.mu.RLock()
	receiver := r.messageReceiver
	peer, hasPeer := r.findPeerByAddr(peerAddr)

	// Build PeerInfo while holding lock to avoid race on state
	var peerInfo plugin.PeerInfo
	if hasPeer {
		s := peer.Settings()
		peerInfo = plugin.PeerInfo{
			Address:      s.Address,
			LocalAddress: s.LocalAddress,
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        peer.State().String(),
		}
	} else {
		peerInfo = plugin.PeerInfo{Address: peerAddr}
	}
	r.mu.RUnlock()

	if receiver == nil {
		return false
	}

	// Assign message ID for all message types
	messageID := nextMsgID()
	timestamp := r.clock.Now()

	var msg bgptypes.RawMessage
	var kept bool

	// Zero-copy path for received UPDATE messages
	if wireUpdate != nil {
		// Set messageID on WireUpdate (single source of truth for UPDATEs)
		wireUpdate.SetMessageID(messageID)

		// Derive AttrsWire for observation callback
		// Errors logged but not fatal - handleUpdate() validates separately
		attrsWire, parseErr := wireUpdate.Attrs()
		if parseErr != nil {
			sessionLogger().Debug("WireUpdate.Attrs error", "peer", peerAddr, "error", parseErr)
		}

		// RawMessage uses zero-copy for synchronous callback processing
		msg = bgptypes.RawMessage{
			Type:       msgType,
			RawBytes:   wireUpdate.Payload(), // Zero-copy: valid during callback
			Timestamp:  timestamp,
			Direction:  direction,
			MessageID:  messageID,
			WireUpdate: wireUpdate,
			AttrsWire:  attrsWire, // Derived from WireUpdate
			ParseError: parseErr,  // Propagate parse error to plugins
		}
	} else {
		// Non-UPDATE or sent messages: copy bytes for async processing safety
		bytes := make([]byte, len(rawBytes))
		copy(bytes, rawBytes)

		msg = bgptypes.RawMessage{
			Type:      msgType,
			RawBytes:  bytes,
			Timestamp: timestamp,
			Direction: direction,
			MessageID: messageID,
		}

		// For sent UPDATE messages, create AttrsWire from body if we have a context ID
		if msgType == message.TypeUPDATE && ctxID != 0 && len(bytes) >= 4 {
			// Parse UPDATE body to extract attribute bytes
			// RFC 4271: withdrawnLen(2) + withdrawn(...) + attrLen(2) + attrs(...) + nlri(...)
			withdrawnLen := int(binary.BigEndian.Uint16(bytes[0:2]))
			attrOffset := 2 + withdrawnLen
			if len(bytes) >= attrOffset+2 {
				attrLen := int(binary.BigEndian.Uint16(bytes[attrOffset : attrOffset+2]))
				if len(bytes) >= attrOffset+2+attrLen {
					attrBytes := bytes[attrOffset+2 : attrOffset+2+attrLen]
					msg.AttrsWire = attribute.NewAttributesWire(attrBytes, ctxID)
				}
			}
		}
	}

	// Cache BEFORE event delivery (only received UPDATEs).
	// Entry is inserted with pending=true so it exists when plugins receive the
	// message-id. After dispatch, Activate(id, N) sets the consumer count.
	// If a fast plugin calls "forward" before Activate(), Get() still works
	// (pending entries are accessible) and Decrement() adjusts the count
	// (negative is corrected when Activate adds N).
	if direction == "received" && wireUpdate != nil && buf != nil {
		r.recentUpdates.Add(&ReceivedUpdate{
			WireUpdate:   wireUpdate, // Zero-copy: slices into buf
			poolBuf:      buf,        // Cache owns buf
			SourcePeerIP: peerAddr,
			ReceivedAt:   timestamp,
		})
		kept = true // Cache always accepts
	}

	// Sent messages: synchronous delivery, no async channel.
	if direction == "sent" {
		receiver.OnMessageSent(peerInfo, msg)
		return kept
	}

	// Received UPDATE with per-peer delivery channel: enqueue for async delivery.
	// The delivery goroutine (started by Peer.runOnce) drains a batch and calls
	// OnMessageBatchReceived + Activate per message. This decouples the TCP read
	// goroutine from plugin processing.
	// Non-UPDATE messages (OPEN, KEEPALIVE, NOTIFICATION) stay synchronous
	// because they are infrequent and FSM-critical.
	if hasPeer && peer.deliverChan != nil && msgType == message.TypeUPDATE {
		peer.deliverChan <- deliveryItem{peerInfo: peerInfo, msg: msg}
		return kept
	}

	// Synchronous fallback: no delivery channel or non-UPDATE message.
	consumerCount := receiver.OnMessageReceived(peerInfo, msg)
	if kept {
		r.recentUpdates.Activate(messageID, consumerCount)
	}

	return kept
}

// AddPeer adds a peer to the reactor.
func (r *Reactor) AddPeer(settings *PeerSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize peer Address for consistent lookup (handles IPv4-mapped IPv6)
	// This ensures connections from 10.0.0.1 match peers configured as ::ffff:10.0.0.1
	settings.Address = settings.Address.Unmap()

	key := settings.PeerKey()
	if _, exists := r.peers[key]; exists {
		return ErrPeerExists
	}

	// Validate and normalize LocalAddress (only if set)
	if settings.LocalAddress.IsValid() {
		// Normalize IPv4-mapped IPv6 addresses (e.g., ::ffff:192.168.1.1 -> 192.168.1.1)
		settings.LocalAddress = settings.LocalAddress.Unmap()

		// Check self-referential (Address == LocalAddress)
		// Allow for loopback (127.0.0.0/8 or ::1) to support testing with next-hop self
		isLoopback := settings.Address.IsLoopback() && settings.LocalAddress.IsLoopback()
		if settings.Address == settings.LocalAddress && !isLoopback {
			return fmt.Errorf("peer %s: address cannot equal local-address", settings.Address)
		}

		// Check link-local IPv6 (requires zone ID, not portable)
		if settings.LocalAddress.Is6() && settings.LocalAddress.IsLinkLocalUnicast() {
			return fmt.Errorf("peer %s: link-local addresses not supported for local-address", settings.Address)
		}

		// Check address family mismatch (IPv4 peer with IPv6 LocalAddress or vice versa)
		// Note: Both Address and LocalAddress are already unmapped at this point
		if settings.Address.Is4() != settings.LocalAddress.Is4() {
			return fmt.Errorf("peer %s: address family mismatch (IPv4/IPv6)", settings.Address)
		}
	}

	peer := NewPeer(settings)
	peer.SetClock(r.clock)
	peer.SetDialer(r.dialer)
	peer.SetReactor(r)
	// Set message callback to forward raw bytes to reactor's message receiver
	peer.messageCallback = r.notifyMessageReceiver
	r.peers[key] = peer

	// If reactor is running, start the peer and create listener if needed.
	// Active-only peers dial out and never accept inbound — skip listener.
	if r.running {
		if settings.LocalAddress.IsValid() && settings.Connection.IsPassive() {
			listenPort := r.peerListenPort(settings)
			lkey := net.JoinHostPort(settings.LocalAddress.String(), strconv.Itoa(listenPort))
			if _, hasListener := r.listeners[lkey]; !hasListener {
				if err := r.startListenerForAddressPort(settings.LocalAddress, listenPort, settings.PeerKey()); err != nil {
					// Rollback peer addition
					delete(r.peers, key)
					return err
				}
			}
		}
		peer.StartWithContext(r.ctx)
	}

	return nil
}

// RemovePeer removes a peer from the reactor.
// Looks up by address, trying default port first then searching by IP.
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize address for consistent lookup (handles IPv4-mapped IPv6)
	addr = addr.Unmap()

	key, peer, exists := r.findPeerKeyByAddr(addr)
	if !exists {
		return ErrPeerNotFound
	}

	settings := peer.Settings()
	localAddr := settings.LocalAddress
	listenPort := r.peerListenPort(settings)

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)

	// Check if any other peer uses this listener (same LocalAddress + port)
	if localAddr.IsValid() {
		stillUsed := false
		for _, p := range r.peers {
			ps := p.Settings()
			if ps.LocalAddress == localAddr && r.peerListenPort(ps) == listenPort {
				stillUsed = true
				break
			}
		}

		// Stop listener if no longer needed
		if !stillUsed {
			lkey := net.JoinHostPort(localAddr.String(), strconv.Itoa(listenPort))
			if listener, ok := r.listeners[lkey]; ok {
				listener.Stop()
				waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = listener.Wait(waitCtx)
				cancel()
				delete(r.listeners, lkey)
			}
		}
	}

	return nil
}

// AddDynamicPeer adds a peer with the given configuration from the plugin API.
// Used by "bgp peer <ip> add" command for runtime peer management.
// LocalAS and RouterID default to reactor config if not specified.
func (r *Reactor) AddDynamicPeer(config plugin.DynamicPeerConfig) error {
	// Use reactor defaults for optional fields
	localAS := config.LocalAS
	if localAS == 0 {
		localAS = r.config.LocalAS
	}
	routerID := config.RouterID
	if routerID == 0 {
		routerID = r.config.RouterID
	}

	settings := NewPeerSettings(config.Address, localAS, config.PeerAS, routerID)
	if config.LocalAddress.IsValid() {
		settings.LocalAddress = config.LocalAddress
	}
	if config.HoldTime > 0 {
		settings.HoldTime = config.HoldTime
	}
	if mode, err := ParseConnectionMode(config.Connection); err == nil {
		settings.Connection = mode
	}

	return r.AddPeer(settings)
}

// Start begins the reactor with a background context.
func (r *Reactor) Start() error {
	return r.StartWithContext(context.Background())
}

// StartWithContext begins the reactor with the given context.
func (r *Reactor) StartWithContext(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return ErrAlreadyRunning
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	r.startTime = r.clock.Now()

	// Start background gap scan goroutine for the recent update cache.
	r.recentUpdates.Start()

	// Start legacy listener if ListenAddr is configured (backward compatibility)
	if r.config.ListenAddr != "" {
		r.listener = NewListener(r.config.ListenAddr)
		r.listener.SetClock(r.clock)
		r.listener.SetListenerFactory(r.listenerFactory)
		r.listener.SetHandler(r.handleConnection)
		if err := r.listener.StartWithContext(r.ctx); err != nil {
			r.cancel()
			return err
		}
	}

	// Start multi-listeners based on peer LocalAddresses and ports.
	// Each unique (LocalAddress, port) pair gets a listener.
	// Peers with custom ports get per-peer listeners (direct routing);
	// peers with default port share a listener (remote-IP routing).
	type listenerSpec struct {
		addr    netip.Addr
		port    int
		peerKey string // non-empty for per-peer-port listeners
	}
	seen := make(map[string]struct{})
	var specs []listenerSpec
	for _, peer := range r.peers {
		s := peer.Settings()
		if !s.LocalAddress.IsValid() {
			continue
		}
		// Skip listener for peers that don't accept inbound (passive bit not set).
		if !s.Connection.IsPassive() {
			continue
		}
		listenPort := r.peerListenPort(s)
		lkey := net.JoinHostPort(s.LocalAddress.String(), strconv.Itoa(listenPort))
		if _, ok := seen[lkey]; ok {
			continue
		}
		seen[lkey] = struct{}{}
		peerKey := ""
		if s.Port != 0 && s.Port != DefaultBGPPort {
			peerKey = s.PeerKey() // Per-peer-port: direct routing
		}
		specs = append(specs, listenerSpec{addr: s.LocalAddress, port: listenPort, peerKey: peerKey})
	}

	for _, spec := range specs {
		if err := r.startListenerForAddressPort(spec.addr, spec.port, spec.peerKey); err != nil {
			r.stopAllListeners()
			if r.listener != nil {
				r.listener.Stop()
			}
			r.cancel()
			return err
		}
	}

	// Start API server if configured
	if r.config.APISocketPath != "" {
		apiConfig := &pluginserver.ServerConfig{
			SocketPath:         r.config.APISocketPath,
			ConfiguredFamilies: r.config.ConfiguredFamilies,
			RPCProviders: []func() []pluginserver.RPCRegistration{
				handler.BgpHandlerRPCs,
			},
			RPCFallback:   bgpserver.CodecRPCHandler,
			CommitManager: commit.NewCommitManager(),
		}
		// Convert plugin configs
		for _, pc := range r.config.Plugins {
			apiConfig.Plugins = append(apiConfig.Plugins, plugin.PluginConfig{
				Name:          pc.Name,
				Run:           pc.Run,
				Encoder:       pc.Encoder,
				Respawn:       pc.Respawn,
				WorkDir:       r.config.ConfigDir,
				ReceiveUpdate: pc.ReceiveUpdate,
				StageTimeout:  pc.StageTimeout,
				Internal:      pc.Internal, // Run in-process via goroutine
			})
		}
		r.api = pluginserver.NewServer(apiConfig, &reactorAPIAdapter{r})
		// Create EventDispatcher for BGP event delivery (type-safe, no hooks indirection)
		r.eventDispatcher = bgpserver.NewEventDispatcher(r.api)
		// Set EventDispatcher as message receiver for raw byte access
		r.messageReceiver = r.eventDispatcher
		// Register API state observer for peer lifecycle events
		r.AddPeerObserver(&apiStateObserver{dispatcher: r.eventDispatcher, reactor: r})

		// Set plugin count for API sync - wait for all plugins to send "api ready"
		r.SetAPIProcessCount(len(r.config.Plugins))

		if err := r.api.StartWithContext(r.ctx); err != nil {
			r.stopAllListeners()
			if r.listener != nil {
				r.listener.Stop()
			}
			r.cancel()
			return err
		}
	}

	// Start signal handler
	r.signals = NewSignalHandler()
	r.signals.OnShutdown(func() {
		r.Stop()
	})
	r.signals.OnReload(func() {
		// Use the reload coordinator (verify→apply protocol with plugins)
		// when the API server has a config loader configured. Falls back
		// to direct reload via reloadFunc otherwise (production default
		// until config loader is wired).
		if r.api != nil && r.api.HasConfigLoader() {
			if err := r.api.ReloadFromDisk(r.ctx); err != nil {
				reactorLogger().Error("config reload failed", "error", err)
			} else {
				reactorLogger().Info("config reloaded via coordinator")
			}
		} else {
			adapter := &reactorAPIAdapter{r: r}
			if err := adapter.Reload(); err != nil {
				reactorLogger().Error("config reload failed", "error", err)
			} else {
				reactorLogger().Info("config reloaded")
			}
		}
	})
	r.signals.StartWithContext(r.ctx)

	// Capture peers slice before releasing lock - ensures consistent snapshot
	// even if peers were somehow modified during API wait.
	peersToStart := r.peers

	// Release lock before waiting for API - plugins need RLock in GetPeerCapabilityConfigs()
	// during their startup protocol. Holding the write lock here causes deadlock.
	r.mu.Unlock()

	// Wait for plugin startup to complete (Phase 1 + Phase 2) before validating.
	// This ensures auto-loaded plugins have registered their families.
	r.WaitForPluginStartupComplete()

	// Also wait for individual plugins to signal ready (backwards compat).
	r.WaitForAPIReady()

	// Validate peer families against available plugin decoders.
	// If a peer has explicit family config, all families must have decoders.
	// If no family config, plugin decode families will be used (validated in sendOpen).
	if err := r.validatePeerFamilies(peersToStart); err != nil {
		r.mu.Lock()
		r.stopAllListeners()
		if r.listener != nil {
			r.listener.Stop()
		}
		r.cancel()
		return err
	}

	// Start all peers (passive peers wait for incoming connections).
	// Uses captured slice - each peer has its own synchronization.
	for _, peer := range peersToStart {
		peer.StartWithContext(r.ctx)
	}

	// Re-acquire lock only to set running state
	r.mu.Lock()
	r.running = true

	// Monitor context for shutdown
	r.wg.Add(1)
	go r.monitor()

	return nil
}

// Stop signals the reactor to stop.
func (r *Reactor) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// startListenerForAddressPort creates and starts a listener on addr:port.
// If peerKey is non-empty, the listener is a per-peer-port listener that routes
// directly to that peer (no remote IP matching). Otherwise, it's a shared listener
// that matches incoming connections by remote IP.
// Must be called with r.mu held.
func (r *Reactor) startListenerForAddressPort(addr netip.Addr, port int, peerKey string) error {
	lkey := net.JoinHostPort(addr.String(), strconv.Itoa(port))

	if _, exists := r.listeners[lkey]; exists {
		return nil // Already listening
	}

	listener := NewListener(lkey)
	listener.SetClock(r.clock)
	listener.SetListenerFactory(r.listenerFactory)

	if peerKey != "" {
		// Per-peer-port listener: route directly by peer key
		capturedKey := peerKey
		listener.SetHandler(func(conn net.Conn) {
			r.handleDirectConnection(conn, capturedKey)
		})
	} else {
		// Shared listener: match by remote IP
		localAddr := addr
		listener.SetHandler(func(conn net.Conn) {
			r.handleConnectionWithContext(conn, localAddr)
		})
	}

	if err := listener.StartWithContext(r.ctx); err != nil {
		return fmt.Errorf("listen on %s: %w", lkey, err)
	}

	r.listeners[lkey] = listener
	return nil
}

// stopAllListeners stops all multi-listeners and waits for them to finish.
// Must be called with r.mu held.
func (r *Reactor) stopAllListeners() {
	for key, listener := range r.listeners {
		listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = listener.Wait(waitCtx)
		cancel()
		delete(r.listeners, key)
	}
}

// nativeFamilies are families decoded natively by the engine without plugins.
// Only INET-format families (same prefix encoding as ipv4/unicast) are native.
// All other families are handled by their respective plugins via the registry.
var nativeFamilies = map[string]bool{
	// RFC 4271 - BGP-4 (IPv4 unicast) - truly native
	"ipv4/unicast": true,
	// RFC 4760 - Multiprotocol Extensions (same INET prefix format)
	"ipv6/unicast":   true,
	"ipv4/multicast": true,
	"ipv6/multicast": true,
}

// validatePeerFamilies checks that all explicitly configured peer families have decoders.
// If a peer has a family block, every family must have a plugin OR be a native family.
// If no family block, validation passes (sendOpen will use all plugin decode families).
//
// Returns error if any configured family lacks a decoder, preventing startup.
func (r *Reactor) validatePeerFamilies(peers map[string]*Peer) error {
	// Get available decode families from plugins
	var decodeFamilies []string
	if r.api != nil {
		decodeFamilies = r.api.GetDecodeFamilies()
	}

	// Build lookup set for O(1) checks - include native families
	available := make(map[string]bool)
	for f := range nativeFamilies {
		available[f] = true
	}
	for _, f := range decodeFamilies {
		available[f] = true
	}

	// Check each peer's configured families
	for _, peer := range peers {
		settings := peer.Settings()
		var configuredFamilies []string

		// Extract Multiprotocol capabilities (these are the configured families)
		for _, cap := range settings.Capabilities {
			if mp, ok := cap.(*capability.Multiprotocol); ok {
				fam := nlri.Family{AFI: mp.AFI, SAFI: mp.SAFI}
				configuredFamilies = append(configuredFamilies, fam.String())
			}
		}

		// If no families configured, skip validation (sendOpen uses plugin families)
		if len(configuredFamilies) == 0 {
			continue
		}

		// Validate each configured family has a decoder
		for _, fam := range configuredFamilies {
			if !available[fam] {
				return fmt.Errorf("peer %s: family %s has no decoder plugin\n  available: %v",
					settings.Address, fam, decodeFamilies)
			}
		}
	}

	return nil
}

// Wait waits for the reactor to stop.
func (r *Reactor) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitor watches for shutdown and cleans up.
func (r *Reactor) monitor() {
	defer r.wg.Done()

	<-r.ctx.Done()

	r.cleanup()
}

// cleanup stops all components.
// Signals all components to stop first, then waits for everything concurrently
// under a single shared deadline. This prevents sequential timeouts from
// compounding (e.g., api(1s) + listener(2s) + peers(N×2s) = unbounded).
func (r *Reactor) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Phase 1: Signal everything to stop (non-blocking).
	// Stop forward pool first — drains in-flight dispatches and workers
	// before peers are stopped, avoiding log noise from ErrInvalidState.
	if r.fwdPool != nil {
		r.fwdPool.Stop()
	}
	if r.api != nil {
		r.api.Stop()
	}
	if r.listener != nil {
		r.listener.Stop()
	}
	r.stopAllListeners()
	if r.signals != nil {
		r.signals.Stop()
	}
	for _, peer := range r.peers {
		peer.Stop()
	}

	// Phase 2: Wait for everything concurrently under a single deadline.
	// Components should exit quickly since their contexts are already canceled.
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	var wg sync.WaitGroup

	if r.api != nil {
		wg.Go(func() {
			_ = r.api.Wait(waitCtx)
		})
	}
	if r.listener != nil {
		wg.Go(func() {
			_ = r.listener.Wait(waitCtx)
		})
	}
	if r.signals != nil {
		wg.Go(func() {
			_ = r.signals.Wait(waitCtx)
		})
	}
	for _, peer := range r.peers {
		wg.Go(func() {
			_ = peer.Wait(waitCtx)
		})
	}

	wg.Wait()
	waitCancel()

	// Phase 3: Cleanup remaining resources.
	r.recentUpdates.Stop()

	r.running = false
	r.cancel = nil
}

// handleConnection handles an incoming TCP connection.
// RFC 4271 §6.8: Connection collision detection.
//
// Architecture:
//
//	handleConnection()
//	├── ESTABLISHED → rejectConnectionCollision() [NOTIFICATION 6/7]
//	├── OpenConfirm → SetPendingConnection() + go handlePendingCollision()
//	│                  └── Read OPEN → ResolvePendingCollision()
//	│                       ├── Local wins → rejectConnectionCollision()
//	│                       └── Remote wins → CloseWithNotification() existing
//	│                                        + acceptPendingConnection()
//	└── Other states → normal AcceptConnection()
func (r *Reactor) handleConnection(conn net.Conn) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(peerIP)
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
// This validates that the connection arrived on the expected listener for RFC compliance.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		closeConnQuietly(conn)
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.findPeerByAddr(peerIP)
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	settings := peer.Settings()

	// RFC compliance: verify connection arrived on expected listener
	if settings.LocalAddress.IsValid() && settings.LocalAddress != listenerAddr {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// handleDirectConnection handles a connection on a per-peer-port listener.
// The peerKey directly identifies the target peer (no remote IP matching needed).
// Used when peers have custom ports — the listener port uniquely identifies the peer.
func (r *Reactor) handleDirectConnection(conn net.Conn, peerKey string) {
	r.mu.RLock()
	peer, exists := r.peers[peerKey]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		closeConnQuietly(conn)
		return
	}

	r.acceptOrReject(conn, peer, cb)
}

// acceptOrReject performs collision detection and accepts or rejects an incoming connection.
// Shared by handleConnection, handleConnectionWithContext, and handleDirectConnection.
func (r *Reactor) acceptOrReject(conn net.Conn, peer *Peer, cb ConnectionCallback) {
	settings := peer.Settings()

	if cb != nil {
		cb(conn, settings)
		return
	}

	// Reject inbound if passive bit is not set (active-only peer).
	if !settings.Connection.IsPassive() {
		closeConnQuietly(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with ESTABLISHED session.
	// "collision with existing BGP connection that is in the Established
	// state causes closing of the newly created connection"
	if peer.State() == PeerStateEstablished {
		r.rejectConnectionCollision(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with OpenConfirm session.
	// Queue the connection and wait for OPEN to compare BGP IDs.
	if peer.SessionState() == fsm.StateOpenConfirm {
		if err := peer.SetPendingConnection(conn); err != nil {
			r.rejectConnectionCollision(conn)
			return
		}
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	if err := peer.AcceptConnection(conn); err != nil {
		// If the session can't accept right now and the peer is passive, buffer the
		// connection for the next runOnce() cycle instead of closing it. This handles
		// the race where the remote reconnects faster than our session teardown:
		// - ErrNotConnected: session is nil (not yet created or already cleaned up)
		// - ErrSessionTearingDown: session is shutting down
		// - ErrAlreadyConnected: session still has stale conn ref from previous connection
		if (errors.Is(err, ErrNotConnected) || errors.Is(err, ErrSessionTearingDown) || errors.Is(err, ErrAlreadyConnected)) && peer.Settings().Connection.IsPassive() {
			peer.SetInboundConnection(conn)
			return
		}
		closeConnQuietly(conn)
	}
}

// closeConnQuietly closes a connection, logging any error at debug level.
func closeConnQuietly(conn net.Conn) {
	if err := conn.Close(); err != nil {
		reactorLogger().Debug("close connection", "error", err)
	}
}

// rejectConnectionCollision sends NOTIFICATION Cease/Connection Collision (6/7)
// and closes the connection. RFC 4271 §6.8.
func (r *Reactor) rejectConnectionCollision(conn net.Conn) {
	notif := &message.Notification{
		ErrorCode:    message.NotifyCease,
		ErrorSubcode: message.NotifyCeaseConnectionCollision,
	}
	data := message.PackTo(notif, nil)
	_, _ = conn.Write(data)
	_ = conn.Close()
}

// handlePendingCollision reads OPEN from a pending connection and resolves collision.
// RFC 4271 §6.8: Upon receipt of OPEN, compare BGP IDs and close the loser.
func (r *Reactor) handlePendingCollision(peer *Peer, conn net.Conn) {
	buf := make([]byte, message.MaxMsgLen)

	// Set read deadline - use hold time or 90s default
	holdTime := peer.Settings().HoldTime
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}
	_ = conn.SetReadDeadline(r.clock.Now().Add(holdTime))

	// Read BGP header
	_, err := io.ReadFull(conn, buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	hdr, err := message.ParseHeader(buf[:message.HeaderLen])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Must be OPEN message
	if hdr.Type != message.TypeOPEN {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Read OPEN body
	_, err = io.ReadFull(conn, buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		_ = conn.Close()
		return
	}

	// Parse OPEN
	open, err := message.UnpackOpen(buf[message.HeaderLen:hdr.Length])
	if err != nil {
		peer.ClearPendingConnection()
		r.rejectConnectionCollision(conn)
		return
	}

	// Resolve collision using BGP ID from OPEN
	acceptPending, pendingConn, pendingOpen, waitSession := peer.ResolvePendingCollision(open)

	if !acceptPending {
		// Local wins: close pending with NOTIFICATION
		r.rejectConnectionCollision(pendingConn)
		return
	}

	// Remote wins: existing session is being closed, accept pending
	// We need to wait a bit for the existing session to close, then
	// start a new session with the pending connection
	r.acceptPendingConnection(peer, pendingConn, pendingOpen, waitSession)
}

// acceptPendingConnection accepts a pending connection after collision resolution.
// The existing session has been closed, so we accept the pending connection with its pre-received OPEN.
func (r *Reactor) acceptPendingConnection(peer *Peer, conn net.Conn, open *message.Open, waitSession <-chan struct{}) {
	// Wait for existing session to fully close
	// The CloseWithNotification was called in ResolvePendingCollision
	if waitSession != nil {
		timer := r.clock.NewTimer(collisionResolutionTimeout)
		defer timer.Stop()
		select {
		case <-waitSession:
			// Session closed
		case <-timer.C():
			reactorLogger().Warn("session teardown timed out during collision resolution", "peer", peer.Settings().Address)
		}
	}

	// Accept connection with the pre-received OPEN
	if err := peer.AcceptConnectionWithOpen(conn, open); err != nil {
		// Failed to accept - peer may have been stopped or old session not yet closed
		_ = conn.Close()
	}
}

// PausePeer pauses reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) PausePeer(addr netip.Addr) error {
	r.mu.RLock()
	_, peer, ok := r.findPeerKeyByAddr(addr)
	r.mu.RUnlock()

	if !ok {
		return ErrPeerNotFound
	}

	peer.PauseReading()
	return nil
}

// ResumePeer resumes reading from a specific peer's session.
// Returns ErrPeerNotFound if the peer address is unknown.
func (r *Reactor) ResumePeer(addr netip.Addr) error {
	r.mu.RLock()
	_, peer, ok := r.findPeerKeyByAddr(addr)
	r.mu.RUnlock()

	if !ok {
		return ErrPeerNotFound
	}

	peer.ResumeReading()
	return nil
}

// PauseAllReads pauses reading from all peers' sessions.
func (r *Reactor) PauseAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.PauseReading()
	}
}

// ResumeAllReads resumes reading from all peers' sessions.
func (r *Reactor) ResumeAllReads() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, peer := range r.peers {
		peer.ResumeReading()
	}
}
