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
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/capability"
	bgpctx "codeberg.org/thomas-mangin/ze/internal/plugin/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/plugin/vpn"
	"codeberg.org/thomas-mangin/ze/internal/selector"
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
	ErrAlreadyRunning        = errors.New("reactor already running")
	ErrNotRunning            = errors.New("reactor not running")
	ErrPeerExists            = errors.New("peer already exists")
	ErrPeerNotFound          = errors.New("peer not found")
	ErrWatchdogRouteNotFound = errors.New("watchdog route not found")
	ErrNoConfigPath          = errors.New("config path not set")
	ErrNoReloadFunc          = errors.New("reload function not set")
)

// Address family names for error messages.
const (
	familyIPv4 = "IPv4"
	familyIPv6 = "IPv6"
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

	// RecentUpdateTTL is how long update-ids remain valid for forwarding.
	// Default: 60s. Zero disables caching (forwarding won't work).
	RecentUpdateTTL time.Duration

	// RecentUpdateMax is the maximum number of cached updates.
	// Default: 100000. Zero means no limit (not recommended).
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
// Messages are passed as raw wire bytes for on-demand parsing based on format config.
type MessageReceiver interface {
	// OnMessageReceived is called when a BGP message is received from a peer.
	// peer contains full peer information for proper JSON encoding.
	// msg contains raw wire bytes - parsing is done by receiver based on format config.
	OnMessageReceived(peer plugin.PeerInfo, msg plugin.RawMessage)

	// OnMessageSent is called when a BGP message is sent to a peer.
	// Only UPDATE messages trigger sent events.
	// Used by RIB plugin to track routes for replay on reconnect.
	OnMessageSent(peer plugin.PeerInfo, msg plugin.RawMessage)
}

// PeerLifecycleObserver receives peer state change notifications.
// Observers are called synchronously in registration order.
// Implementations MUST NOT block; use goroutine for slow processing.
type PeerLifecycleObserver interface {
	OnPeerEstablished(peer *Peer)
	OnPeerClosed(peer *Peer, reason string)
}

// apiStateObserver emits ExaBGP-compatible state messages to API server.
type apiStateObserver struct {
	server  *plugin.Server
	reactor *Reactor
}

func (o *apiStateObserver) OnPeerEstablished(peer *Peer) {
	if o.server == nil {
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.server.OnPeerStateChange(peerInfo, "up")
}

func (o *apiStateObserver) OnPeerClosed(peer *Peer, reason string) {
	if o.server == nil {
		return
	}
	s := peer.Settings()
	peerInfo := plugin.PeerInfo{
		Address:      s.Address,
		LocalAddress: s.LocalAddress,
		LocalAS:      s.LocalAS,
		PeerAS:       s.PeerAS,
		RouterID:     s.RouterID,
		State:        peer.State().String(),
	}
	o.server.OnPeerStateChange(peerInfo, "down")
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
//   - Watchdog pools for API-controlled route groups
type Reactor struct {
	config *Config

	peers     map[string]*Peer         // keyed by peer address
	listener  *Listener                // deprecated: single listener for backward compat
	listeners map[netip.Addr]*Listener // one listener per unique LocalAddress
	signals   *SignalHandler
	api       *plugin.Server // API server for CLI and external processes

	// RIB components
	ribIn    *rib.IncomingRIB // Adj-RIB-In
	ribStore *rib.RouteStore  // Global deduplication store

	// Watchdog pools for API-created routes
	watchdog *WatchdogManager

	// Recent UPDATE cache for efficient forwarding via update-id
	recentUpdates *RecentUpdateCache

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

// reactorAPIAdapter implements plugin.ReactorInterface for the Reactor.
type reactorAPIAdapter struct {
	r *Reactor
}

// Peers returns peer information for the API.
func (a *reactorAPIAdapter) Peers() []plugin.PeerInfo {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]plugin.PeerInfo, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		s := p.Settings()
		info := plugin.PeerInfo{
			Address:      s.Address,
			LocalAddress: netip.Addr{}, // TODO: get from session
			LocalAS:      s.LocalAS,
			PeerAS:       s.PeerAS,
			RouterID:     s.RouterID,
			State:        p.State().String(),
		}
		if p.State() == PeerStateEstablished {
			info.Uptime = time.Since(a.r.startTime) // TODO: track per-peer
		}
		result = append(result, info)
	}
	return result
}

// GetPeerCapabilityConfigs returns capability configurations for all peers.
// Used by plugin protocol Stage 2 to deliver matching config.
// Extracts known capability values into a flexible map for pattern matching.
func (a *reactorAPIAdapter) GetPeerCapabilityConfigs() []plugin.PeerCapabilityConfig {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	result := make([]plugin.PeerCapabilityConfig, 0, len(a.r.peers))
	for _, p := range a.r.peers {
		s := p.Settings()
		cfg := plugin.PeerCapabilityConfig{
			Address:        s.Address.String(),
			Values:         make(map[string]string),
			CapabilityJSON: s.CapabilityConfigJSON,
		}

		// Extract capability values via ConfigProvider interface.
		// Each capability that implements ConfigProvider returns its own
		// scoped key-value pairs (e.g., "rfc4724:restart-time" or "draft-xxx:field").
		// This allows new capabilities to be added without modifying this code.
		for _, cap := range s.Capabilities {
			if provider, ok := cap.(capability.ConfigProvider); ok {
				for k, v := range provider.ConfigValues() {
					cfg.Values[k] = v
				}
			}
		}

		// Also include raw capability config values for plugin-declared capabilities.
		// Format: "<name>:<field>" → value (RFC-style scoping, matches ConfigProvider pattern).
		// Server.go adds "capability " prefix when building path.
		for capName, fields := range s.RawCapabilityConfig {
			for fieldName, value := range fields {
				key := capName + ":" + fieldName
				cfg.Values[key] = value
			}
		}

		result = append(result, cfg)
	}
	return result
}

// GetConfigTree returns the full config as a map for plugin config delivery.
func (a *reactorAPIAdapter) GetConfigTree() map[string]any {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()
	return a.r.configTree
}

// Stats returns reactor statistics for the API.
func (a *reactorAPIAdapter) Stats() plugin.ReactorStats {
	stats := a.r.Stats()
	return plugin.ReactorStats{
		StartTime: stats.StartTime,
		Uptime:    stats.Uptime,
		PeerCount: stats.PeerCount,
	}
}

// Stop signals the reactor to stop.
func (a *reactorAPIAdapter) Stop() {
	a.r.Stop()
}

// AnnounceRoute announces a route to matching peers.
// If a peer is in transaction, queues to its Adj-RIB-Out; otherwise sends immediately.
func (a *reactorAPIAdapter) AnnounceRoute(peerSelector string, route plugin.RouteSpec) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI
	var n nlri.NLRI
	if route.Prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
	}

	// Build attributes from RouteSpec.Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		var err error
		attrs, err = route.Wire.All()
		if err != nil {
			return fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy
		nextHopAddr, nhErr := peer.resolveNextHop(route.NextHop, n.Family())
		if nhErr != nil {
			// Log but continue - skip this peer if next-hop can't be resolved
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		// Build AS_PATH: empty for iBGP, prepend LocalAS for eBGP
		// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
		var asPath *attribute.ASPath
		switch {
		case len(userASPath) > 0:
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: userASPath},
				},
			}
		case isIBGP:
			asPath = &attribute.ASPath{Segments: nil}
		default:
			asPath = &attribute.ASPath{
				Segments: []attribute.ASPathSegment{
					{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
				},
			}
		}

		ribRoute := rib.NewRouteWithASPath(n, nextHopAddr, attrs, asPath)

		// Create resolved route spec for buildAnnounceUpdate
		resolvedRoute := route
		resolvedRoute.NextHop = plugin.NewNextHopExplicit(nextHopAddr)

		if !peer.ShouldQueue() {
			// RFC 4271 Section 4.3 - Send UPDATE immediately (zero-allocation path)
			if err := peer.SendAnnounce(resolvedRoute, a.r.config.LocalAS); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// WithdrawRoute withdraws a route from matching peers.
func (a *reactorAPIAdapter) WithdrawRoute(peerSelector string, prefix netip.Prefix) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI for queueing
	var n nlri.NLRI
	if prefix.Addr().Is4() {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
	} else {
		n = nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
	}

	var lastErr error
	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// RFC 4271 Section 4.3 - Send UPDATE immediately (zero-allocation path)
			if err := peer.SendWithdraw(prefix); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// sendToMatchingPeers sends an UPDATE to peers matching the selector.
// Supports: "*" (all peers), exact IP, or glob patterns (e.g., "192.168.*.*").
func (a *reactorAPIAdapter) sendToMatchingPeers(selector string, update *message.Update) error {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var lastErr error
	sentCount := 0

	for addrStr, peer := range a.r.peers {
		// Check if this peer matches the selector using glob matching
		if !ipGlobMatch(selector, addrStr) {
			continue
		}

		// Only send to established peers
		if peer.State() != PeerStateEstablished {
			continue
		}

		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		} else {
			sentCount++
		}
	}

	if sentCount == 0 && lastErr == nil {
		// No peers matched or were established
		return errors.New("no established peers to send to")
	}

	return lastErr
}

// Zero-allocation attribute writers.
// These functions write attributes directly to the buffer without allocating structs.

// writeOriginAttr writes ORIGIN attribute directly to buf.
// RFC 4271 §5.1.1: Well-known mandatory, 1 byte value.
func writeOriginAttr(buf []byte, off int, origin uint8) int {
	// Header: Transitive(0x40) | code(1) | len(1)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrOrigin)
	buf[off+2] = 1
	buf[off+3] = origin
	return 4
}

// writeASPathAttr writes AS_PATH attribute directly to buf.
// RFC 4271 §5.1.2: Well-known mandatory.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers.
// RFC 4271 §4.3: Handles segment splitting for >255 ASNs and extended length.
func writeASPathAttr(buf []byte, off int, asns []uint32, asn4 bool) int {
	start := off
	asnSize := 2
	if asn4 {
		asnSize = 4
	}

	// RFC 4271: Max 255 ASNs per segment, split if needed
	// Calculate total value length accounting for segment splitting
	var valueLen int
	remaining := len(asns)
	for remaining > 0 {
		chunk := remaining
		if chunk > attribute.MaxASPathSegmentLength {
			chunk = attribute.MaxASPathSegmentLength
		}
		valueLen += 2 + chunk*asnSize // type(1) + count(1) + asns
		remaining -= chunk
	}
	// Empty AS_PATH for iBGP has valueLen=0

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	if valueLen > 255 {
		buf[off] = byte(attribute.FlagTransitive | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrASPath)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec
		off += 4
	} else {
		buf[off] = byte(attribute.FlagTransitive)
		buf[off+1] = byte(attribute.AttrASPath)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	// Value: write segments, splitting at 255 ASNs
	remaining = len(asns)
	idx := 0
	for remaining > 0 {
		chunk := remaining
		if chunk > attribute.MaxASPathSegmentLength {
			chunk = attribute.MaxASPathSegmentLength
		}

		buf[off] = byte(attribute.ASSequence) // Type
		buf[off+1] = byte(chunk)              // Count
		off += 2

		for i := 0; i < chunk; i++ {
			asn := asns[idx+i]
			if asn4 {
				binary.BigEndian.PutUint32(buf[off:], asn)
				off += 4
			} else {
				// RFC 6793: Map to AS_TRANS if > 65535
				if asn > 65535 {
					binary.BigEndian.PutUint16(buf[off:], 23456) // AS_TRANS
				} else {
					binary.BigEndian.PutUint16(buf[off:], uint16(asn)) //nolint:gosec
				}
				off += 2
			}
		}

		idx += chunk
		remaining -= chunk
	}

	return off - start
}

// writeNextHopAttr writes NEXT_HOP attribute directly to buf.
// RFC 4271 §5.1.3: Well-known mandatory, 4 bytes for IPv4.
func writeNextHopAttr(buf []byte, off int, addr netip.Addr) int {
	// Header: Transitive(0x40) | code(3) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrNextHop)
	buf[off+2] = 4
	a4 := addr.As4()
	copy(buf[off+3:], a4[:])
	return 7
}

// writeMEDAttr writes MED attribute directly to buf.
// RFC 4271 §5.1.4: Optional non-transitive, 4 bytes.
func writeMEDAttr(buf []byte, off int, med uint32) int {
	// Header: Optional(0x80) | code(4) | len(4)
	buf[off] = byte(attribute.FlagOptional)
	buf[off+1] = byte(attribute.AttrMED)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], med)
	return 7
}

// writeLocalPrefAttr writes LOCAL_PREF attribute directly to buf.
// RFC 4271 §5.1.5: Well-known for iBGP, 4 bytes.
func writeLocalPrefAttr(buf []byte, off int, localPref uint32) int {
	// Header: Transitive(0x40) | code(5) | len(4)
	buf[off] = byte(attribute.FlagTransitive)
	buf[off+1] = byte(attribute.AttrLocalPref)
	buf[off+2] = 4
	binary.BigEndian.PutUint32(buf[off+3:], localPref)
	return 7
}

// writeCommunitiesAttr writes COMMUNITIES attribute directly to buf.
// RFC 1997: Optional transitive, 4 bytes per community.
// RFC 4271 §4.3: Uses extended length for >63 communities (>255 bytes).
func writeCommunitiesAttr(buf []byte, off int, communities []uint32) int {
	start := off
	valueLen := len(communities) * 4

	// RFC 4271 §4.3: Use extended length if > 255 bytes
	flags := attribute.FlagOptional | attribute.FlagTransitive
	if valueLen > 255 {
		buf[off] = byte(flags | attribute.FlagExtLength)
		buf[off+1] = byte(attribute.AttrCommunity)
		binary.BigEndian.PutUint16(buf[off+2:], uint16(valueLen)) //nolint:gosec
		off += 4
	} else {
		buf[off] = byte(flags)
		buf[off+1] = byte(attribute.AttrCommunity)
		buf[off+2] = byte(valueLen)
		off += 3
	}

	for _, c := range communities {
		binary.BigEndian.PutUint32(buf[off:], c)
		off += 4
	}

	return off - start
}

// WriteAnnounceUpdate writes a complete BGP UPDATE message for announcing a route
// directly into buf at offset off. Returns total bytes written.
//
// True zero-allocation: writes all attributes directly to the buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
// RFC 6793: asn4 determines 2-byte vs 4-byte AS numbers in AS_PATH.
func WriteAnnounceUpdate(buf []byte, off int, route plugin.RouteSpec, localAS uint32, isIBGP bool, asn4, addPath bool) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := 0; i < message.MarkerLen; i++ {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder (backfill after body)
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(message.TypeUPDATE)
	off++

	// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (announce, not withdraw)
	buf[off] = 0
	buf[off+1] = 0
	off += 2

	// Path Attributes Length placeholder (backfill after attrs)
	attrLenPos := off
	off += 2
	attrStart := off

	// Extract attributes from Wire (wire-first approach)
	origin := uint8(attribute.OriginIGP)
	var med *uint32
	var localPref *uint32
	var communities []uint32
	var largeCommunities []attribute.LargeCommunity
	var extCommunities []attribute.ExtendedCommunity
	var userASPath []uint32

	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil && originAttr != nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				origin = uint8(o)
			}
		}
		// Extract AS_PATH (all segments)
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok {
				for _, seg := range asp.Segments {
					userASPath = append(userASPath, seg.ASNs...)
				}
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil && medAttr != nil {
			if m, ok := medAttr.(attribute.MED); ok {
				v := uint32(m)
				med = &v
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil && lpAttr != nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				v := uint32(lp)
				localPref = &v
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				communities = make([]uint32, len(comms))
				for i, c := range comms {
					communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lc, ok := lcAttr.(attribute.LargeCommunities); ok {
				largeCommunities = lc
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ec, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				extCommunities = ec
			}
		}
	}

	// 1. ORIGIN - RFC 4271 §5.1.1: Well-known mandatory attribute.
	off += writeOriginAttr(buf, off, origin)

	// 2. AS_PATH - RFC 4271 §5.1.2: Well-known mandatory attribute.
	// Zero-alloc: write directly without creating ASPath struct.
	var asPathASNs []uint32
	switch {
	case len(userASPath) > 0:
		asPathASNs = userASPath // Use caller's slice directly
	case isIBGP:
		asPathASNs = nil // Empty AS_PATH for iBGP
	default:
		// eBGP: prepend local AS - use stack-allocated array
		asPathASNs = []uint32{localAS}
	}
	off += writeASPathAttr(buf, off, asPathASNs, asn4)

	isIPv6 := route.Prefix.Addr().Is6()
	nhAddr := route.NextHop.Addr

	// 3. NEXT_HOP - RFC 4271 §5.1.3 (IPv4 only; IPv6 uses MP_REACH_NLRI)
	if !isIPv6 {
		off += writeNextHopAttr(buf, off, nhAddr)
	}

	// 4. MED - RFC 4271 §5.1.4: Optional non-transitive attribute.
	if med != nil {
		off += writeMEDAttr(buf, off, *med)
	}

	// 5. LOCAL_PREF - RFC 4271 §5.1.5: Well-known attribute for iBGP only.
	if isIBGP {
		lpVal := uint32(100)
		if localPref != nil {
			lpVal = *localPref
		}
		off += writeLocalPrefAttr(buf, off, lpVal)
	}

	// 6. COMMUNITY - RFC 1997: Optional transitive attribute.
	if len(communities) > 0 {
		off += writeCommunitiesAttr(buf, off, communities)
	}

	// 7. LARGE_COMMUNITY - RFC 8092: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(largeCommunities) > 0 {
		lcomms := attribute.LargeCommunities(largeCommunities)
		off += attribute.WriteAttrTo(lcomms, buf, off)
	}

	// 8. EXTENDED_COMMUNITIES - RFC 4360: Optional transitive attribute.
	// Type conversion only, no allocation.
	if len(extCommunities) > 0 {
		extComms := attribute.ExtendedCommunities(extCommunities)
		off += attribute.WriteAttrTo(extComms, buf, off)
	}

	// NLRI handling - MP_REACH_NLRI (14) goes at end per our pattern
	if !isIPv6 {
		// IPv4: Write NLRI directly after attributes (zero-alloc)
		// Backfill attr length first
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)

		// RFC 7911: WriteNLRI handles ADD-PATH encoding
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)
	} else {
		// RFC 4760 Section 3 - IPv6: Write MP_REACH_NLRI directly (zero-alloc)
		// Wire format: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(16) + Reserved(1) + NLRI(var)
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, route.Prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)
		nhLen := 16 // IPv6 next-hop
		mpValueLen := 2 + 1 + 1 + nhLen + 1 + nlriPayloadLen

		// RFC 4760 Section 3 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPReachNLRI, uint16(mpValueLen)) //nolint:gosec

		// RFC 4760 Section 3 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 3 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 3 - Length of Next Hop (1 octet)
		buf[off] = byte(nhLen)
		off++

		// RFC 4760 Section 3 - Network Address of Next Hop (variable)
		off += copy(buf[off:], nhAddr.AsSlice())

		// RFC 4760 Section 3 - Reserved (1 octet, MUST be 0)
		buf[off] = 0
		off++

		// RFC 4760 Section 3 - NLRI (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// Backfill attr length (no inline NLRI for IPv6)
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}

// WriteWithdrawUpdate writes a complete BGP UPDATE message for withdrawing a route
// directly into buf at offset off. Returns total bytes written.
//
// Eliminates large buffer allocations by writing directly to the provided buffer.
//
// RFC 4271 Section 4.3 - UPDATE message format.
// RFC 4760 Section 4: IPv6 withdrawals use MP_UNREACH_NLRI attribute.
// RFC 7911: addPath indicates ADD-PATH capability for NLRI encoding.
func WriteWithdrawUpdate(buf []byte, off int, prefix netip.Prefix, addPath bool) int {
	start := off

	// RFC 4271 Section 4.1 - BGP Header: 16-byte marker (all 0xFF)
	for i := 0; i < message.MarkerLen; i++ {
		buf[off+i] = 0xFF
	}
	off += message.MarkerLen

	// Length placeholder
	lengthPos := off
	off += 2

	// Type = UPDATE
	buf[off] = byte(message.TypeUPDATE)
	off++

	if prefix.Addr().Is4() {
		// RFC 4271 Section 4.3 - IPv4: Use WithdrawnRoutes field (zero-alloc)
		// Withdrawn Routes Length placeholder
		withdrawnLenPos := off
		off += 2
		withdrawnStart := off

		// RFC 4271 Section 4.3 - Withdrawn Routes: list of IP address prefixes
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}, prefix, 0)
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// RFC 4271 Section 4.3 - Backfill Withdrawn Routes Length
		withdrawnLen := off - withdrawnStart
		buf[withdrawnLenPos] = byte(withdrawnLen >> 8)
		buf[withdrawnLenPos+1] = byte(withdrawnLen)

		// RFC 4271 Section 4.3 - Total Path Attribute Length = 0 (withdrawal only)
		buf[off] = 0
		buf[off+1] = 0
		off += 2
	} else {
		// RFC 4760 Section 4 - IPv6: Use MP_UNREACH_NLRI attribute (zero-alloc)
		// RFC 4271 Section 4.3 - Withdrawn Routes Length = 0 (using MP_UNREACH instead)
		buf[off] = 0
		buf[off+1] = 0
		off += 2

		// RFC 4271 Section 4.3 - Path Attributes Length placeholder
		attrLenPos := off
		off += 2
		attrStart := off

		// RFC 4760 Section 4 - MP_UNREACH_NLRI wire format:
		//   AFI(2) + SAFI(1) + Withdrawn_NLRI(var)
		inet := nlri.NewINET(nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}, prefix, 0)
		nlriPayloadLen := nlri.LenWithContext(inet, addPath)
		mpValueLen := 2 + 1 + nlriPayloadLen

		// RFC 4760 Section 4 - Attribute header (Optional, non-transitive)
		off += attribute.WriteHeaderTo(buf, off, attribute.FlagOptional, attribute.AttrMPUnreachNLRI, uint16(mpValueLen)) //nolint:gosec

		// RFC 4760 Section 4 - AFI (2 octets)
		buf[off] = 0
		buf[off+1] = byte(attribute.AFIIPv6)
		off += 2

		// RFC 4760 Section 4 - SAFI (1 octet)
		buf[off] = byte(attribute.SAFIUnicast)
		off++

		// RFC 4760 Section 4 - Withdrawn Routes (variable)
		// RFC 7911: WriteNLRI handles ADD-PATH encoding when negotiated
		off += nlri.WriteNLRI(inet, buf, off, addPath)

		// Backfill attr length
		attrLen := off - attrStart
		buf[attrLenPos] = byte(attrLen >> 8)
		buf[attrLenPos+1] = byte(attrLen)
	}

	// Backfill total message length
	totalLen := off - start
	buf[lengthPos] = byte(totalLen >> 8)
	buf[lengthPos+1] = byte(totalLen)

	return totalLen
}

// Reload reloads the configuration.
// It re-parses the config file and diffs peers:
// - New peers in config are added
// - Peers not in new config are removed
// - Peers with changed settings are removed and re-added
// Requires ConfigPath to be set and SetReloadFunc to be called.
func (a *reactorAPIAdapter) Reload() error {
	r := a.r

	// Check config path is set.
	configPath := r.config.ConfigPath
	if configPath == "" {
		return ErrNoConfigPath
	}

	// Check reload function is set.
	r.mu.RLock()
	reloadFn := r.reloadFunc
	r.mu.RUnlock()
	if reloadFn == nil {
		return ErrNoReloadFunc
	}

	// Get new peer configs from config file.
	newPeers, err := reloadFn(configPath)
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}

	// Build map of new peer settings for quick lookup.
	newPeerSettings := make(map[string]*PeerSettings)
	for _, p := range newPeers {
		newPeerSettings[p.Address.String()] = p
	}

	// Get current peer addresses and settings snapshot.
	r.mu.RLock()
	currentPeers := make(map[string]*PeerSettings)
	for addr, peer := range r.peers {
		currentPeers[addr] = peer.Settings()
	}
	r.mu.RUnlock()

	// Categorize peers: to remove, to add, unchanged.
	var toRemove []string
	var toAdd []*PeerSettings

	for addr := range currentPeers {
		newSettings, exists := newPeerSettings[addr]
		if !exists {
			// Peer removed from config.
			toRemove = append(toRemove, addr)
		} else if !peerSettingsEqual(currentPeers[addr], newSettings) {
			// Peer settings changed - remove and re-add.
			toRemove = append(toRemove, addr)
			toAdd = append(toAdd, newSettings)
			reactorLogger().Debug("reload: peer settings changed", "peer", addr)
		}
	}

	for addr, settings := range newPeerSettings {
		if _, exists := currentPeers[addr]; !exists {
			// New peer in config.
			toAdd = append(toAdd, settings)
		}
	}

	// Remove peers.
	for _, addr := range toRemove {
		r.mu.Lock()
		if peer, ok := r.peers[addr]; ok {
			peer.Stop()
			delete(r.peers, addr)
			reactorLogger().Debug("reload: removed peer", "peer", addr)
		}
		r.mu.Unlock()
	}

	// Add peers.
	var addErrors []error
	for _, settings := range toAdd {
		if err := r.AddPeer(settings); err != nil {
			addErrors = append(addErrors, fmt.Errorf("add peer %s: %w", settings.Address, err))
		} else {
			reactorLogger().Debug("reload: added peer", "peer", settings.Address)
		}
	}

	if len(addErrors) > 0 {
		return errors.Join(addErrors...)
	}

	return nil
}

// peerSettingsEqual compares two PeerSettings for reload diffing.
// Returns true if the settings are functionally equivalent.
func peerSettingsEqual(a, b *PeerSettings) bool {
	if a == nil || b == nil {
		return a == b
	}

	// Compare identity fields.
	if a.Address != b.Address ||
		a.LocalAS != b.LocalAS ||
		a.PeerAS != b.PeerAS ||
		a.RouterID != b.RouterID {
		return false
	}

	// Compare connectivity fields.
	if a.LocalAddress != b.LocalAddress ||
		a.Port != b.Port ||
		a.Passive != b.Passive {
		return false
	}

	// Compare behavior fields.
	if a.HoldTime != b.HoldTime ||
		a.GroupUpdates != b.GroupUpdates ||
		a.IgnoreFamilyMismatch != b.IgnoreFamilyMismatch ||
		a.DisableASN4 != b.DisableASN4 {
		return false
	}

	// Compare static routes count (deep comparison would be expensive).
	if len(a.StaticRoutes) != len(b.StaticRoutes) {
		return false
	}

	// Compare capabilities count (deep comparison would be expensive).
	if len(a.Capabilities) != len(b.Capabilities) {
		return false
	}

	return true
}

// AnnounceFlowSpec announces a FlowSpec route to matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceFlowSpec(_ string, _ plugin.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// WithdrawFlowSpec withdraws a FlowSpec route from matching peers.
// TODO: Implement when FlowSpec RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawFlowSpec(_ string, _ plugin.FlowSpecRoute) error {
	return errors.New("flowspec: not implemented")
}

// AnnounceVPLS announces a VPLS route to matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceVPLS(_ string, _ plugin.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// WithdrawVPLS withdraws a VPLS route from matching peers.
// TODO: Implement when VPLS RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawVPLS(_ string, _ plugin.VPLSRoute) error {
	return errors.New("vpls: not implemented")
}

// AnnounceL2VPN announces an L2VPN/EVPN route to matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) AnnounceL2VPN(_ string, _ plugin.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// WithdrawL2VPN withdraws an L2VPN/EVPN route from matching peers.
// TODO: Implement when L2VPN/EVPN RIB integration is complete.
func (a *reactorAPIAdapter) WithdrawL2VPN(_ string, _ plugin.L2VPNRoute) error {
	return errors.New("l2vpn: not implemented")
}

// AnnounceL3VPN announces an L3VPN (MPLS VPN) route to matching peers.
// RFC 4364 - BGP/MPLS IP Virtual Private Networks.
//
// Behavior:
//   - Established peer: sends UPDATE immediately
//   - Non-established peer: queues to peer's operation queue (sent on connect)
func (a *reactorAPIAdapter) AnnounceL3VPN(peerSelector string, route plugin.L3VPNRoute) error {
	// RFC 4364: L3VPN routes require RD
	if route.RD == "" {
		return errors.New("l3vpn route requires route-distinguisher (rd)")
	}
	// RFC 4364: L3VPN routes require labels
	if len(route.Labels) == 0 {
		return errors.New("l3vpn route requires at least one label")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build VPNParams once (peer-independent)
	params, err := a.buildL3VPNParams(route)
	if err != nil {
		return fmt.Errorf("invalid route: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		if !peer.ShouldQueue() {
			// Send immediately
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
			if route.Prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			asn4 := peer.asn4()
			addPath := peer.addPathFor(family)

			// Build UPDATE using UpdateBuilder for immediate send
			ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, asn4, addPath)
			update := ub.BuildVPN(params)

			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			ribRoute, err := a.buildL3VPNRIBRoute(route, isIBGP)
			if err != nil {
				lastErr = err
				continue
			}
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// WithdrawL3VPN withdraws an L3VPN route from matching peers.
// RFC 4364 - Uses MP_UNREACH_NLRI with SAFI 128.
//
// Behavior:
//   - Established peer: sends UPDATE with MP_UNREACH_NLRI immediately
//   - Non-established peer: queues withdrawal (sent on connect)
func (a *reactorAPIAdapter) WithdrawL3VPN(peerSelector string, route plugin.L3VPNRoute) error {
	// RFC 4364: RD required to identify the VPN route
	if route.RD == "" {
		return errors.New("l3vpn withdrawal requires route-distinguisher (rd)")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Parse RD for NLRI
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return fmt.Errorf("invalid rd: %w", err)
	}

	// Use first label from stack for withdrawal (RFC allows - prefix identifies route)
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0x800000} // RFC 3107 withdrawal label
	}

	// Build NLRI for withdrawal
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	n := vpn.NewVPN(family, rd, labels[:1], route.Prefix, 0) // Single label for withdrawal

	// Build StaticRoute for withdrawal
	staticRoute := StaticRoute{
		Prefix: route.Prefix,
		RD:     route.RD,
		Labels: labels[:1],
	}
	copy(staticRoute.RDBytes[:], rd.Bytes())

	var lastErr error
	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Build MP_UNREACH_NLRI for VPN
			attrBuf := getBuildBuf()
			update := buildMPUnreachVPN(attrBuf, staticRoute)
			sendErr := peer.SendUpdate(update)
			putBuildBuf(attrBuf)
			if sendErr != nil {
				lastErr = sendErr
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// buildL3VPNParams converts an plugin.L3VPNRoute to message.VPNParams.
// RFC 4364 - VPN route parameters.
func (a *reactorAPIAdapter) buildL3VPNParams(route plugin.L3VPNRoute) (message.VPNParams, error) {
	// Parse RD
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return message.VPNParams{}, fmt.Errorf("invalid rd: %w", err)
	}

	params := message.VPNParams{
		Prefix:  route.Prefix,
		NextHop: route.NextHop,
		Labels:  route.Labels,
		Origin:  attribute.OriginIGP,
	}

	// Copy RD bytes
	rdBytes := rd.Bytes()
	copy(params.RDBytes[:], rdBytes)

	// Extract optional attributes from Wire (wire-first approach)
	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				params.Origin = o
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				params.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				params.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				params.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				params.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					params.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				params.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					params.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ecs, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				params.ExtCommunityBytes = append(params.ExtCommunityBytes, ecs.Pack()...)
			}
		}
	}

	// Handle RT (Route Target) - convert to extended community
	if route.RT != "" {
		rtBytes, err := parseRouteTarget(route.RT)
		if err != nil {
			return message.VPNParams{}, fmt.Errorf("invalid rt: %w", err)
		}
		params.ExtCommunityBytes = append(params.ExtCommunityBytes, rtBytes...)
	}

	return params, nil
}

// buildL3VPNRIBRoute creates a rib.Route from an plugin.L3VPNRoute for queueing.
// RFC 4364: VPN routes include RD + labels in NLRI.
func (a *reactorAPIAdapter) buildL3VPNRIBRoute(route plugin.L3VPNRoute, isIBGP bool) (*rib.Route, error) {
	// Parse RD
	rd, err := nlri.ParseRDString(route.RD)
	if err != nil {
		return nil, fmt.Errorf("invalid rd: %w", err)
	}

	// Build NLRI
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIVPN} // RFC 4364
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	n := vpn.NewVPN(family, rd, route.Labels, route.Prefix, 0)

	// Build attributes from Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		attrs, err = route.Wire.All()
		if err != nil {
			return nil, fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	// Handle RT (Route Target) - convert to extended community
	if route.RT != "" {
		rtBytes, err := parseRouteTarget(route.RT)
		if err != nil {
			return nil, fmt.Errorf("invalid rt: %w", err)
		}
		ec, err := attribute.ParseExtendedCommunities(rtBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid rt extended community: %w", err)
		}
		attrs = append(attrs, ec)
	}

	// Build AS_PATH
	var asPath *attribute.ASPath
	switch {
	case len(userASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}

	return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath), nil
}

// Extended community type codes per RFC 4360 Section 3.
const (
	ecTypeTransitive2ByteAS = 0x00 // 2-byte AS, transitive
	ecTypeTransitiveIPv4    = 0x01 // IPv4 address, transitive
	ecTypeTransitive4ByteAS = 0x02 // 4-byte AS, transitive
	ecSubtypeRouteTarget    = 0x02 // Route Target subtype
)

// parseRouteTarget parses a Route Target string to extended community bytes.
//
// RFC 4360 Section 3 - Extended Community format.
// Supported formats:
//   - "target:ASN:NN" or "ASN:NN" - 2-byte ASN with 4-byte value
//   - "target:IP:NN" or "IP:NN" - IPv4 address with 2-byte value
//   - 4-byte ASN automatically uses Type 2 format
func parseRouteTarget(s string) ([]byte, error) {
	// Remove "target:" prefix if present
	s = strings.TrimPrefix(s, "target:")

	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid rt format: %s (expected ASN:NN or IP:NN)", s)
	}

	// Check if first part is an IP address (Type 1 format)
	if ip, err := netip.ParseAddr(parts[0]); err == nil && ip.Is4() {
		val, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid rt value %q (must be 0-65535 for IP:NN format)", parts[1])
		}
		b := ip.As4()
		return []byte{
			ecTypeTransitiveIPv4, ecSubtypeRouteTarget,
			b[0], b[1], b[2], b[3],
			byte(val >> 8), byte(val),
		}, nil
	}

	// Parse as ASN:NN format
	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid ASN in rt: %s", parts[0])
	}

	val, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid value in rt: %s", parts[1])
	}

	// RFC 4360 Section 3 - Extended Community encoding
	if asn <= 65535 {
		// Type 0: 2-byte ASN, 4-byte value
		return []byte{
			ecTypeTransitive2ByteAS, ecSubtypeRouteTarget,
			byte(asn >> 8), byte(asn),
			byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val),
		}, nil
	}

	// ASN > 65535: Use Type 2 (4-byte ASN) if value fits in 16 bits
	if val > 65535 {
		return nil, fmt.Errorf("invalid rt: 4-byte ASN requires value <= 65535, got %d", val)
	}
	return []byte{
		ecTypeTransitive4ByteAS, ecSubtypeRouteTarget,
		byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
		byte(val >> 8), byte(val),
	}, nil
}

// AnnounceLabeledUnicast announces an MPLS labeled unicast route (SAFI 4).
// RFC 8277 - Using BGP to Bind MPLS Labels to Address Prefixes.
//
// Supports three modes like AnnounceRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and tracks for re-announcement
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) AnnounceLabeledUnicast(peerSelector string, route plugin.LabeledUnicastRoute) error {
	// RFC 8277: Labeled unicast routes require at least one label
	if len(route.Labels) == 0 {
		return errors.New("labeled unicast route requires at least one label")
	}

	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	var lastErr error
	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Build rib.Route with ALL attributes (not just Origin like AnnounceRoute bug)
		ribRoute, err := a.buildLabeledUnicastRIBRoute(route, isIBGP)
		if err != nil {
			lastErr = err
			continue
		}

		if !peer.ShouldQueue() {
			// Send immediately
			family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
			if route.Prefix.Addr().Is6() {
				family.AFI = nlri.AFIIPv6
			}
			addPath := peer.addPathFor(family)
			asn4 := peer.asn4()

			// Build UPDATE using UpdateBuilder for immediate send
			ub := message.NewUpdateBuilder(a.r.config.LocalAS, isIBGP, asn4, addPath)
			params := a.buildLabeledUnicastParams(route)
			update := ub.BuildLabeledUnicast(params)

			if err := peer.SendUpdate(update); err != nil {
				lastErr = err
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueAnnounce(ribRoute)
		}
	}
	return lastErr
}

// buildLabeledUnicastParams converts an API route to message.LabeledUnicastParams.
func (a *reactorAPIAdapter) buildLabeledUnicastParams(route plugin.LabeledUnicastRoute) message.LabeledUnicastParams {
	params := message.LabeledUnicastParams{
		Prefix:  route.Prefix,
		PathID:  route.PathID, // RFC 7911 ADD-PATH
		NextHop: route.NextHop,
		Labels:  route.Labels, // RFC 8277: Multi-label support
		Origin:  attribute.OriginIGP,
	}

	// Extract optional attributes from Wire (wire-first approach)
	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				params.Origin = o
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				params.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				params.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				params.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				params.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					params.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				params.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					params.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
		// Extract EXTENDED_COMMUNITIES
		if ecAttr, err := route.Wire.Get(attribute.AttrExtCommunity); err == nil {
			if ecs, ok := ecAttr.(attribute.ExtendedCommunities); ok {
				params.ExtCommunityBytes = ecs.Pack()
			}
		}
	}

	return params
}

// buildLabeledUnicastRIBRoute creates a rib.Route from a LabeledUnicastRoute.
// Unlike AnnounceRoute which only stores OriginIGP, this stores ALL attributes.
// This ensures attributes are preserved when routes are queued and replayed.
//
// RFC 8277: Labeled unicast routes include MPLS labels in the NLRI.
// RFC 7911: PathID is included when ADD-PATH is negotiated.
func (a *reactorAPIAdapter) buildLabeledUnicastRIBRoute(route plugin.LabeledUnicastRoute, isIBGP bool) (*rib.Route, error) {
	// 1. Build NLRI with nlri.LabeledUnicast
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label if not specified
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0}
	}

	n := nlri.NewLabeledUnicast(family, route.Prefix, labels, route.PathID)

	// 2. Build attributes from Wire (wire-first approach)
	var attrs []attribute.Attribute
	var userASPath []uint32

	if route.Wire != nil {
		// Parse attributes from wire format
		var err error
		attrs, err = route.Wire.All()
		if err != nil {
			return nil, fmt.Errorf("failed to parse route attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	} else {
		// No wire attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	// 3. Build AS-PATH
	// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS
	var asPath *attribute.ASPath
	switch {
	case len(userASPath) > 0:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}

	return rib.NewRouteWithASPath(n, route.NextHop, attrs, asPath), nil
}

// WithdrawLabeledUnicast withdraws an MPLS labeled unicast route.
// RFC 8277 - Uses MP_UNREACH_NLRI with SAFI 4.
//
// Supports three modes like WithdrawRoute:
//   - Transaction mode: queues to Adj-RIB-Out (sent on commit)
//   - Established: sends immediately and removes from sent cache
//   - Not established: queues to peer's operation queue.
func (a *reactorAPIAdapter) WithdrawLabeledUnicast(peerSelector string, route plugin.LabeledUnicastRoute) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Build NLRI for queueing
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIMPLSLabel}
	if route.Prefix.Addr().Is6() {
		family.AFI = nlri.AFIIPv6
	}

	// Default label for withdrawal
	labels := route.Labels
	if len(labels) == 0 {
		labels = []uint32{0x800000} // RFC 8277 withdrawal label
	}

	n := nlri.NewLabeledUnicast(family, route.Prefix, labels, route.PathID)

	var lastErr error
	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Send immediately
			addPath := peer.addPathFor(family)

			// Build withdrawal using existing helper
			staticRoute := StaticRoute{
				Prefix: route.Prefix,
				Labels: labels,
			}

			attrBuf := getBuildBuf()
			update := buildMPUnreachLabeledUnicast(attrBuf, staticRoute, addPath)
			sendErr := peer.SendUpdate(update)
			putBuildBuf(attrBuf)
			if sendErr != nil {
				lastErr = sendErr
			}
		} else {
			// Session not established or queue draining: queue to preserve order
			peer.QueueWithdraw(n)
		}
	}
	return lastErr
}

// AnnounceMUPRoute announces a MUP route (SAFI 85) to matching peers.
// draft-mpmz-bess-mup-safi - Mobile User Plane.
func (a *reactorAPIAdapter) AnnounceMUPRoute(peerSelector string, spec plugin.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, false)
}

// WithdrawMUPRoute withdraws a MUP route from matching peers.
// Uses MP_UNREACH_NLRI with SAFI 85.
func (a *reactorAPIAdapter) WithdrawMUPRoute(peerSelector string, spec plugin.MUPRouteSpec) error {
	return a.sendMUPRoute(peerSelector, spec, true)
}

// sendMUPRoute is a common helper for announce/withdraw MUP routes.
func (a *reactorAPIAdapter) sendMUPRoute(peerSelector string, spec plugin.MUPRouteSpec, isWithdraw bool) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return errors.New("no peers match selector")
	}

	// Convert API spec to reactor MUPRoute
	mupRoute, err := convertAPIMUPRoute(spec)
	if err != nil {
		return fmt.Errorf("convert MUP route: %w", err)
	}

	var lastErr error
	for _, peer := range peers {
		if peer.State() != PeerStateEstablished {
			continue
		}

		// Check if MUP family is negotiated
		nc := peer.negotiated.Load()
		if nc == nil {
			continue
		}
		if spec.IsIPv6 && !nc.Has(nlri.IPv6MUP) {
			continue // Skip peer that doesn't support IPv6 MUP
		}
		if !spec.IsIPv6 && !nc.Has(nlri.IPv4MUP) {
			continue // Skip peer that doesn't support IPv4 MUP
		}

		family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: safiMUP}
		if spec.IsIPv6 {
			family.AFI = nlri.AFIIPv6
		}
		addPath := peer.addPathFor(family)
		asn4 := peer.asn4()

		// Build UPDATE using UpdateBuilder
		ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
		var update *message.Update
		if isWithdraw {
			update = ub.BuildMUPWithdraw(toMUPParams(mupRoute))
		} else {
			update = ub.BuildMUP(toMUPParams(mupRoute))
		}

		if err := peer.SendUpdate(update); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// AnnounceNLRIBatch announces a batch of NLRIs with shared attributes.
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (a *reactorAPIAdapter) AnnounceNLRIBatch(peerSelector string, batch plugin.NLRIBatch) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return plugin.ErrNoPeersMatch
	}

	// Build attributes for RIB route (used for queueing non-established peers)
	// Prefer Wire (forwarding) over Attrs (builder) when available
	var attrs []attribute.Attribute
	var userASPath []uint32

	switch {
	case batch.Wire != nil:
		// Parse attributes from wire format
		var err error
		attrs, err = batch.Wire.All()
		if err != nil {
			return fmt.Errorf("failed to parse batch attributes: %w", err)
		}
		// Extract AS_PATH if present
		if asPathAttr, err := batch.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				userASPath = asp.Segments[0].ASNs
			}
		}
	case batch.Attrs != nil:
		// Use Builder for new routes
		attrs = batch.Attrs.ToAttributes()
		userASPath = batch.Attrs.ASPathSlice()
	default:
		// No attributes - use defaults
		attrs = append(attrs, attribute.OriginIGP)
	}

	var lastErr error
	var acceptedCount int

	for _, peer := range peers {
		isIBGP := peer.Settings().IsIBGP()

		// Resolve next-hop per peer using RouteNextHop policy
		nextHop, nhErr := peer.resolveNextHop(batch.NextHop, batch.Family)
		if nhErr != nil {
			// Log but continue - skip this peer if next-hop can't be resolved
			routesLogger().Debug("next-hop resolution failed", "peer", peer.Settings().Address, "error", nhErr)
			continue
		}

		// Build AS_PATH per peer (iBGP vs eBGP)
		asPath := a.buildBatchASPath(userASPath, isIBGP)

		if !peer.ShouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// Get max message size from capabilities
			// RFC 8654: 65535 if ExtendedMessage, else 4096
			maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
			addPath := peer.addPathFor(batch.Family)
			asn4 := peer.asn4()

			// Build UPDATE message for this batch using pooled buffers
			attrBuf := getBuildBuf()
			nlriBuf := getBuildBuf()
			update := a.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, nextHop, isIBGP, asn4, addPath)

			// Send with splitting for large batches
			// RFC 4271: Each split UPDATE is self-contained with full attributes
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, batch.Family); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				ribRoute := rib.NewRouteWithASPath(n, nextHop, attrs, asPath)
				peer.QueueAnnounce(ribRoute)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Return warning-level error if no peers accepted (all skipped due to family)
	if acceptedCount == 0 {
		return plugin.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// WithdrawNLRIBatch withdraws a batch of NLRIs.
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) WithdrawNLRIBatch(peerSelector string, batch plugin.NLRIBatch) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return plugin.ErrNoPeersMatch
	}

	var lastErr error
	var acceptedCount int

	for _, peer := range peers {
		if !peer.ShouldQueue() {
			// Check family negotiation
			nc := peer.negotiated.Load()
			if nc == nil || !nc.Has(batch.Family) {
				continue // Skip peer that doesn't support this family
			}

			// Get max message size from capabilities
			maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, nc.ExtendedMessage))
			addPath := peer.addPathFor(batch.Family)

			// Build withdraw UPDATE for this batch using pooled buffers
			attrBuf := getBuildBuf()
			nlriBuf := getBuildBuf()
			update := a.buildBatchWithdrawUpdate(attrBuf, nlriBuf, batch, addPath)

			// Send with splitting for large batches
			if err := peer.sendUpdateWithSplit(update, maxMsgSize, batch.Family); err != nil {
				lastErr = err
			} else {
				acceptedCount++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
		} else {
			// Session not established or queue draining: queue to preserve order
			for _, n := range batch.NLRIs {
				peer.QueueWithdraw(n)
			}
			acceptedCount++ // Queued counts as accepted
		}
	}

	// Return warning-level error if no peers accepted (all skipped due to family)
	if acceptedCount == 0 {
		return plugin.ErrNoPeersAcceptedFamily
	}
	return lastErr
}

// buildBatchASPath builds AS_PATH for batch operations.
// RFC 4271 §5.1.2: iBGP SHALL NOT modify AS_PATH; eBGP prepends local AS.
func (a *reactorAPIAdapter) buildBatchASPath(userASPath []uint32, isIBGP bool) *attribute.ASPath {
	switch {
	case len(userASPath) > 0:
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: userASPath},
			},
		}
	case isIBGP:
		return &attribute.ASPath{Segments: nil}
	default:
		return &attribute.ASPath{
			Segments: []attribute.ASPathSegment{
				{Type: attribute.ASSequence, ASNs: []uint32{a.r.config.LocalAS}},
			},
		}
	}
}

// buildBatchAnnounceUpdate builds an UPDATE message for a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: UPDATE Message Format.
// RFC 4760: MP_REACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchAnnounceUpdate(attrBuf, nlriBuf []byte, batch plugin.NLRIBatch, nextHop netip.Addr, isIBGP bool, asn4, addPath bool) *message.Update {
	// Pack NLRIs into caller-provided buffer
	nlriOff := 0
	for _, n := range batch.NLRIs {
		nlriOff += nlri.WriteNLRI(n, nlriBuf, nlriOff, addPath)
	}
	nlriBytes := nlriBuf[:nlriOff]

	// Wire mode: ensure mandatory attributes present, then add NEXT_HOP or MP_REACH_NLRI
	if batch.Wire != nil {
		attrOff := a.writeMandatoryAttrs(attrBuf, batch.Wire, isIBGP, asn4)
		return a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
	}

	// Builder mode or default: build attributes from Builder or defaults
	var builtBytes []byte
	if batch.Attrs != nil {
		builtBytes = batch.Attrs.Build()
	} else {
		// Default: just ORIGIN=IGP
		b := attribute.NewBuilder()
		b.SetOrigin(uint8(attribute.OriginIGP))
		builtBytes = b.Build()
	}

	// Ensure ORIGIN and AS_PATH are present (Builder may not include AS_PATH)
	wire := attribute.NewAttributesWire(builtBytes, 0)
	attrOff := a.writeMandatoryAttrs(attrBuf, wire, isIBGP, asn4)

	// Add NEXT_HOP or MP_REACH_NLRI
	return a.buildWireModeUpdate(attrBuf, attrOff, nlriBytes, batch.Family, nextHop, isIBGP)
}

// buildWireModeUpdate builds UPDATE using pre-written attribute bytes in attrBuf[:attrOff].
// Inserts NEXT_HOP (IPv4 unicast) or appends MP_REACH_NLRI (other families).
// attrBuf[:attrOff] must contain mandatory attrs from writeMandatoryAttrs.
// RFC 4271: NEXT_HOP (type 3) must come after AS_PATH (type 2) but before other attrs.
// RFC 4271 §5.1.5: LOCAL_PREF is well-known mandatory for iBGP sessions.
func (a *reactorAPIAdapter) buildWireModeUpdate(attrBuf []byte, attrOff int, nlriBytes []byte, family nlri.Family, nextHop netip.Addr, isIBGP bool) *message.Update {
	isIPv4Unicast := family == nlri.IPv4Unicast

	if isIPv4Unicast {
		// IPv4 unicast: insert NEXT_HOP after AS_PATH for correct type code order
		wireAttrs := attrBuf[:attrOff]
		insertPos := a.findNextHopInsertPosition(wireAttrs)
		hasLocalPref := a.hasAttribute(wireAttrs, attribute.AttrLocalPref)

		nhSize := 7 // NEXT_HOP is 7 bytes (3 header + 4 IP)

		// Shift tail right to make room for NEXT_HOP (copy handles overlap)
		copy(attrBuf[insertPos+nhSize:], attrBuf[insertPos:attrOff])

		// Write NEXT_HOP at insert position
		nh := &attribute.NextHop{Addr: nextHop}
		attribute.WriteAttrTo(nh, attrBuf, insertPos)
		attrOff += nhSize

		// Append LOCAL_PREF=100 at end if needed for iBGP
		if isIBGP && !hasLocalPref {
			lp := attribute.LocalPref(100)
			attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
		}

		return &message.Update{
			PathAttributes: attrBuf[:attrOff],
			NLRI:           nlriBytes,
		}
	}

	// Non-IPv4 unicast: append LOCAL_PREF and MP_REACH_NLRI to existing attrs
	hasLocalPref := a.hasAttribute(attrBuf[:attrOff], attribute.AttrLocalPref)
	if isIBGP && !hasLocalPref {
		lp := attribute.LocalPref(100)
		attrOff += attribute.WriteAttrTo(lp, attrBuf, attrOff)
	}

	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{nextHop},
		NLRI:     nlriBytes,
	}
	attrOff += attribute.WriteAttrTo(mpReach, attrBuf, attrOff)

	return &message.Update{
		PathAttributes: attrBuf[:attrOff],
	}
}

// hasAttribute checks if an attribute type is present in wire attrs.
func (a *reactorAPIAdapter) hasAttribute(wireAttrs []byte, typeCode attribute.AttributeCode) bool {
	pos := 0
	for pos < len(wireAttrs) {
		if pos+2 > len(wireAttrs) {
			break
		}
		flags := wireAttrs[pos]
		tc := wireAttrs[pos+1]
		_ = flags // used for length calculation below

		if attribute.AttributeCode(tc) == typeCode {
			return true
		}

		// Calculate attribute length to skip to next
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+4 > len(wireAttrs) {
				break
			}
			attrLen = 4 + int(binary.BigEndian.Uint16(wireAttrs[pos+2:]))
		} else {
			if pos+3 > len(wireAttrs) {
				break
			}
			attrLen = 3 + int(wireAttrs[pos+2])
		}
		pos += attrLen
	}
	return false
}

// writeMandatoryAttrs ensures ORIGIN and AS_PATH are present in wire attributes,
// writing the result into buf. Returns bytes written.
// RFC 4271 Section 5.1.1: ORIGIN is a well-known mandatory attribute.
// RFC 4271 Section 5.1.2: AS_PATH is a well-known mandatory attribute.
// RFC 4271 Section 5.1: Attributes must appear in type code order.
// If missing, adds defaults: ORIGIN=IGP, AS_PATH per iBGP/eBGP rules.
func (a *reactorAPIAdapter) writeMandatoryAttrs(buf []byte, wire *attribute.AttributesWire, isIBGP, asn4 bool) int {
	hasOrigin, _ := wire.Has(attribute.AttrOrigin)
	hasASPath, _ := wire.Has(attribute.AttrASPath)
	packed := wire.Packed()

	if hasOrigin && hasASPath {
		copy(buf, packed)
		return len(packed)
	}

	off := 0

	// Case 1: Both missing - prepend ORIGIN + AS_PATH
	if !hasOrigin && !hasASPath {
		// ORIGIN=IGP
		buf[off] = 0x40 // Transitive
		buf[off+1] = 1  // ORIGIN
		buf[off+2] = 1  // Length
		buf[off+3] = 0  // IGP
		off += 4

		// AS_PATH
		off += a.writeASPath(buf[off:], isIBGP, asn4)

		copy(buf[off:], packed)
		return off + len(packed)
	}

	// Case 2: Only ORIGIN missing - prepend ORIGIN, copy rest
	if !hasOrigin {
		buf[0] = 0x40 // Transitive
		buf[1] = 1    // ORIGIN
		buf[2] = 1    // Length
		buf[3] = 0    // IGP
		copy(buf[4:], packed)
		return 4 + len(packed)
	}

	// Case 3: Only AS_PATH missing - insert after ORIGIN
	// RFC 4271: attributes must be in type code order (ORIGIN=1, AS_PATH=2)
	originEnd := 4 // ORIGIN is always 4 bytes
	copy(buf, packed[:originEnd])
	off = originEnd

	// Insert AS_PATH
	off += a.writeASPath(buf[off:], isIBGP, asn4)

	// Copy remaining attributes
	copy(buf[off:], packed[originEnd:])
	return off + len(packed) - originEnd
}

// findNextHopInsertPosition finds where to insert NEXT_HOP (type 3) in wire attrs.
// RFC 4271: attributes should be in type code order.
// Returns position after AS_PATH (type 2) or at end if no attrs with type > 2.
func (a *reactorAPIAdapter) findNextHopInsertPosition(wireAttrs []byte) int {
	pos := 0
	for pos < len(wireAttrs) {
		if pos+2 > len(wireAttrs) {
			break
		}
		flags := wireAttrs[pos]
		typeCode := wireAttrs[pos+1]

		// If we find an attr with type >= 3, insert NEXT_HOP here
		if typeCode >= 3 {
			return pos
		}

		// Calculate attribute length
		var attrLen int
		if flags&0x10 != 0 { // Extended length
			if pos+4 > len(wireAttrs) {
				break
			}
			attrLen = 4 + int(binary.BigEndian.Uint16(wireAttrs[pos+2:]))
		} else {
			if pos+3 > len(wireAttrs) {
				break
			}
			attrLen = 3 + int(wireAttrs[pos+2])
		}

		pos += attrLen
	}
	// No attr with type >= 3 found, insert at end
	return pos
}

// writeASPath writes AS_PATH attribute to buf, returning bytes written.
func (a *reactorAPIAdapter) writeASPath(buf []byte, isIBGP, asn4 bool) int {
	switch {
	case isIBGP:
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 0    // Length = 0 (empty)
		return 3
	case asn4:
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 6    // Length: 2 (segment header) + 4 (ASN)
		buf[3] = byte(attribute.ASSequence)
		buf[4] = 1 // Count = 1
		binary.BigEndian.PutUint32(buf[5:], a.r.config.LocalAS)
		return 9
	default:
		buf[0] = 0x40 // Transitive
		buf[1] = 2    // AS_PATH
		buf[2] = 4    // Length: 2 (segment header) + 2 (ASN)
		buf[3] = byte(attribute.ASSequence)
		buf[4] = 1                                                      // Count = 1
		binary.BigEndian.PutUint16(buf[5:], uint16(a.r.config.LocalAS)) //nolint:gosec
		return 7
	}
}

// buildBatchWithdrawUpdate builds an UPDATE message for withdrawing a batch of NLRIs.
// attrBuf and nlriBuf are caller-provided buffers (from buildBufPool).
// RFC 4271 Section 4.3: Withdrawn Routes field.
// RFC 4760: MP_UNREACH_NLRI for non-IPv4-unicast families.
func (a *reactorAPIAdapter) buildBatchWithdrawUpdate(attrBuf, nlriBuf []byte, batch plugin.NLRIBatch, addPath bool) *message.Update {
	// Pack NLRIs into caller-provided buffer
	nlriOff := 0
	for _, n := range batch.NLRIs {
		nlriOff += nlri.WriteNLRI(n, nlriBuf, nlriOff, addPath)
	}
	nlriBytes := nlriBuf[:nlriOff]

	if batch.Family == nlri.IPv4Unicast {
		// IPv4 unicast: Use WithdrawnRoutes field
		return &message.Update{
			WithdrawnRoutes: nlriBytes,
		}
	}

	// Non-IPv4 unicast: Use MP_UNREACH_NLRI (RFC 4760)
	mpUnreach := &attribute.MPUnreachNLRI{
		AFI:  attribute.AFI(batch.Family.AFI),
		SAFI: attribute.SAFI(batch.Family.SAFI),
		NLRI: nlriBytes,
	}
	attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
	return &message.Update{
		PathAttributes: attrBuf[:attrLen],
	}
}

// TeardownPeer gracefully closes a peer session with NOTIFICATION.
// Sends Cease (6) with the specified subcode per RFC 4486.
func (a *reactorAPIAdapter) TeardownPeer(addr netip.Addr, subcode uint8) error {
	a.r.mu.RLock()
	peer, exists := a.r.peers[addr.String()]
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	// Signal teardown with subcode - peer will send NOTIFICATION and close.
	// If session exists, teardown happens immediately.
	// If not connected, teardown is queued to maintain operation order.
	peer.Teardown(subcode)
	return nil
}

// AddDynamicPeer adds a peer with the given configuration.
// Delegates to reactor's AddDynamicPeer which handles defaults.
func (a *reactorAPIAdapter) AddDynamicPeer(config plugin.DynamicPeerConfig) error {
	return a.r.AddDynamicPeer(config)
}

// RemovePeer removes a peer by address.
func (a *reactorAPIAdapter) RemovePeer(addr netip.Addr) error {
	return a.r.RemovePeer(addr)
}

// AnnounceEOR sends an End-of-RIB marker for the given address family.
func (a *reactorAPIAdapter) AnnounceEOR(peerSelector string, afi uint16, safi uint8) error {
	update := message.BuildEOR(nlri.Family{AFI: nlri.AFI(afi), SAFI: nlri.SAFI(safi)})
	return a.sendToMatchingPeers(peerSelector, update)
}

// SendBoRR sends a Beginning of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "Before the speaker starts a route refresh...
// the speaker MUST send a BoRR message.".
func (a *reactorAPIAdapter) SendBoRR(peerSelector string, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(peerSelector, afi, safi, message.RouteRefreshBoRR)
}

// SendEoRR sends an End of Route Refresh marker to matching peers.
// RFC 7313 Section 4: "After the speaker completes the re-advertisement
// of the entire Adj-RIB-Out to the peer, it MUST send an EoRR message.".
func (a *reactorAPIAdapter) SendEoRR(peerSelector string, afi uint16, safi uint8) error {
	return a.sendRouteRefresh(peerSelector, afi, safi, message.RouteRefreshEoRR)
}

// sendRouteRefresh sends a ROUTE-REFRESH message with the specified subtype.
// RFC 7313 Section 3.2 - Message Subtype values:
//   - 0: Normal Route Refresh (RFC 2918)
//   - 1: Beginning of Route Refresh (BoRR)
//   - 2: End of Route Refresh (EoRR)
//
// RFC 7313: "If peer did not advertise Enhanced Route Refresh Capability:
// Do NOT send BoRR or EoRR." Only subtype 0 is allowed without Enhanced RR.
func (a *reactorAPIAdapter) sendRouteRefresh(peerSelector string, afi uint16, safi uint8, subtype message.RouteRefreshSubtype) error {
	// RFC 7313: BoRR/EoRR require Enhanced Route Refresh capability
	requiresEnhancedRR := subtype == message.RouteRefreshBoRR || subtype == message.RouteRefreshEoRR

	rr := &message.RouteRefresh{
		AFI:     message.AFI(afi),
		SAFI:    message.SAFI(safi),
		Subtype: subtype,
	}

	// WriteTo includes the BGP header
	data := message.PackTo(rr, nil)

	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	var lastErr error
	for addrStr, peer := range a.r.peers {
		if !ipGlobMatch(peerSelector, addrStr) {
			continue
		}

		if peer.State() != PeerStateEstablished {
			continue
		}

		// RFC 7313: "If peer did not advertise Enhanced Route Refresh Capability:
		// Do NOT send BoRR or EoRR."
		if requiresEnhancedRR {
			neg := peer.negotiated.Load()
			if neg == nil || !neg.EnhancedRouteRefresh {
				continue // Skip peers without Enhanced Route Refresh
			}
		}

		// Send full packet (msgType=0 means data includes header)
		if err := peer.SendRawMessage(0, data); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// AnnounceWatchdog announces all routes in the named watchdog group.
// Routes are moved from withdrawn (-) to announced (+) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) AnnounceWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			localAddr := peer.Settings().LocalAddress
			routes := a.r.watchdog.AnnouncePool(name, peerAddr)
			for _, pr := range routes {
				// RFC 4271 Section 4.3 - Send UPDATE (zero-allocation path)
				spec := staticRouteToSpec(pr.StaticRoute, localAddr)
				if err := peer.SendAnnounce(spec, a.r.config.LocalAS); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.AnnounceWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// WithdrawWatchdog withdraws all routes in the named watchdog group.
// Routes are moved from announced (+) to withdrawn (-) state.
// Checks global pools first, then per-peer WatchdogGroups.
// Returns error only for send failures, not for missing groups.
func (a *reactorAPIAdapter) WithdrawWatchdog(peerSelector, name string) error {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return nil // No matching peers
	}

	// Check global pool first
	globalPool := a.r.watchdog.GetPool(name)
	if globalPool != nil {
		var lastErr error
		for _, peer := range peers {
			if peer.State() != PeerStateEstablished {
				continue
			}
			peerAddr := peer.Settings().Address.String()
			routes := a.r.watchdog.WithdrawPool(name, peerAddr)
			for _, pr := range routes {
				// RFC 4271 Section 4.3 - Send withdrawal UPDATE (zero-allocation path)
				if err := peer.SendWithdraw(pr.Prefix); err != nil {
					lastErr = err
				}
			}
		}
		return lastErr
	}

	// Fall back to per-peer WatchdogGroups
	var lastErr error
	found := false
	for _, peer := range peers {
		err := peer.WithdrawWatchdog(name)
		if err != nil {
			if errors.Is(err, ErrWatchdogNotFound) {
				// This peer doesn't have the group - skip, try others
				continue
			}
			// Real error (send failure) - record but continue with other peers
			lastErr = err
		} else {
			found = true
		}
	}

	// If no peer had the group, return not found error
	if !found && lastErr == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}
	return lastErr
}

// AddWatchdogRoute adds a route to a global watchdog pool.
// Implements plugin.ReactorInterface.
func (a *reactorAPIAdapter) AddWatchdogRoute(route plugin.RouteSpec, poolName string) error {
	// Convert plugin.RouteSpec to StaticRoute
	sr := StaticRoute{
		Prefix:  route.Prefix,
		NextHop: route.NextHop, // Already plugin.RouteNextHop
	}

	// Extract attributes from Wire (wire-first approach)
	if route.Wire != nil {
		// Extract ORIGIN
		if originAttr, err := route.Wire.Get(attribute.AttrOrigin); err == nil {
			if o, ok := originAttr.(attribute.Origin); ok {
				sr.Origin = uint8(o)
			}
		}
		// Extract LOCAL_PREF
		if lpAttr, err := route.Wire.Get(attribute.AttrLocalPref); err == nil {
			if lp, ok := lpAttr.(attribute.LocalPref); ok {
				sr.LocalPreference = uint32(lp)
			}
		}
		// Extract MED
		if medAttr, err := route.Wire.Get(attribute.AttrMED); err == nil {
			if m, ok := medAttr.(attribute.MED); ok {
				sr.MED = uint32(m)
			}
		}
		// Extract AS_PATH
		if asPathAttr, err := route.Wire.Get(attribute.AttrASPath); err == nil {
			if asp, ok := asPathAttr.(*attribute.ASPath); ok && len(asp.Segments) > 0 {
				sr.ASPath = asp.Segments[0].ASNs
			}
		}
		// Extract COMMUNITY
		if commAttr, err := route.Wire.Get(attribute.AttrCommunity); err == nil {
			if comms, ok := commAttr.(attribute.Communities); ok {
				sr.Communities = make([]uint32, len(comms))
				for i, c := range comms {
					sr.Communities[i] = uint32(c)
				}
			}
		}
		// Extract LARGE_COMMUNITY
		if lcAttr, err := route.Wire.Get(attribute.AttrLargeCommunity); err == nil {
			if lcs, ok := lcAttr.(attribute.LargeCommunities); ok {
				sr.LargeCommunities = make([][3]uint32, len(lcs))
				for i, c := range lcs {
					sr.LargeCommunities[i] = [3]uint32{c.GlobalAdmin, c.LocalData1, c.LocalData2}
				}
			}
		}
	}

	return a.r.AddWatchdogRoute(sr, poolName)
}

// RemoveWatchdogRoute removes a route from a global watchdog pool.
// Implements plugin.ReactorInterface.
func (a *reactorAPIAdapter) RemoveWatchdogRoute(routeKey, poolName string) error {
	return a.r.RemoveWatchdogRoute(routeKey, poolName)
}

// staticRouteToSpec converts a StaticRoute to plugin.RouteSpec.
// localAddress is used to resolve "next-hop self" routes.
func staticRouteToSpec(route StaticRoute, localAddress netip.Addr) plugin.RouteSpec {
	// Resolve next-hop from RouteNextHop policy
	var nextHop netip.Addr
	if route.NextHop.IsSelf() && localAddress.IsValid() {
		nextHop = localAddress
	} else if route.NextHop.IsExplicit() {
		nextHop = route.NextHop.Addr
	}
	// If neither, nextHop remains zero value (invalid)

	spec := plugin.RouteSpec{
		Prefix:  route.Prefix,
		NextHop: plugin.NewNextHopExplicit(nextHop),
	}

	// Build wire-format attributes using Builder (wire-first approach)
	b := attribute.NewBuilder()

	// Origin (0=IGP by default)
	b.SetOrigin(route.Origin)

	// LocalPreference
	if route.LocalPreference != 0 {
		b.SetLocalPref(route.LocalPreference)
	}

	// MED
	if route.MED != 0 {
		b.SetMED(route.MED)
	}

	// ASPath
	if len(route.ASPath) > 0 {
		b.SetASPath(route.ASPath)
	}

	// Communities
	for _, c := range route.Communities {
		b.AddCommunityValue(c)
	}

	// LargeCommunities
	for _, lc := range route.LargeCommunities {
		b.AddLargeCommunity(lc[0], lc[1], lc[2])
	}

	// Build wire bytes and wrap
	wireBytes := b.Build()
	if len(wireBytes) > 0 {
		spec.Wire = attribute.NewAttributesWire(wireBytes, bgpctx.APIContextID)
	}

	return spec
}

// RIBInRoutes returns routes from Adj-RIB-In.
func (a *reactorAPIAdapter) RIBInRoutes(peerID string) []rib.RouteJSON {
	if a.r.ribIn == nil {
		return nil
	}

	var routes []rib.RouteJSON

	// If peerID specified, get routes for that peer only
	if peerID != "" {
		for _, route := range a.r.ribIn.GetPeerRoutes(peerID) {
			routes = append(routes, route.JSON(peerID))
		}
		return routes
	}

	// Get routes from all peers
	a.r.mu.RLock()
	peerIDs := make([]string, 0, len(a.r.peers))
	for id := range a.r.peers {
		peerIDs = append(peerIDs, id)
	}
	a.r.mu.RUnlock()

	for _, id := range peerIDs {
		for _, route := range a.r.ribIn.GetPeerRoutes(id) {
			routes = append(routes, route.JSON(id))
		}
	}

	return routes
}

// RIBOutRoutes returns routes from Adj-RIB-Out.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns nil.
func (a *reactorAPIAdapter) RIBOutRoutes() []rib.RouteJSON {
	return nil
}

// RIBStats returns RIB statistics.
func (a *reactorAPIAdapter) RIBStats() plugin.RIBStatsInfo {
	stats := plugin.RIBStatsInfo{}

	if a.r.ribIn != nil {
		inStats := a.r.ribIn.Stats()
		stats.InPeerCount = inStats.PeerCount
		stats.InRouteCount = inStats.RouteCount
	}

	// Note: Adj-RIB-Out tracking removed. OutPending/OutWithdrawls/OutSent always 0.

	return stats
}

// ClearRIBIn clears all routes from Adj-RIB-In.
func (a *reactorAPIAdapter) ClearRIBIn() int {
	if a.r.ribIn == nil {
		return 0
	}
	return a.r.ribIn.ClearAll()
}

// ClearRIBOut queues withdrawals for all routes in Adj-RIB-Out.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns 0.
func (a *reactorAPIAdapter) ClearRIBOut() int {
	return 0
}

// FlushRIBOut re-queues all sent routes for re-announcement.
//
// Deprecated: Adj-RIB-Out tracking removed. Returns 0.
func (a *reactorAPIAdapter) FlushRIBOut() int {
	return 0
}

// GetPeerProcessBindings returns process bindings for a specific peer.
// Returns nil if peer not found.
// Resolves encoding inheritance: peer binding → plugin encoder → "text" default.
func (a *reactorAPIAdapter) GetPeerProcessBindings(peerAddr netip.Addr) []plugin.PeerProcessBinding {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	peer, ok := a.r.peers[peerAddr.String()]
	if !ok {
		return nil
	}

	settings := peer.Settings()
	result := make([]plugin.PeerProcessBinding, 0, len(settings.ProcessBindings))
	for _, b := range settings.ProcessBindings {
		// Resolve encoding: peer override → plugin default → "text"
		encoding := b.Encoding
		if encoding == "" {
			encoding = a.getPluginEncoder(b.PluginName)
		}
		if encoding == "" {
			encoding = "text"
		}

		// Resolve format: peer override → "parsed"
		format := b.Format
		if format == "" {
			format = "parsed"
		}

		result = append(result, plugin.PeerProcessBinding{
			PluginName:          b.PluginName,
			Encoding:            encoding,
			Format:              format,
			ReceiveUpdate:       b.ReceiveUpdate,
			ReceiveOpen:         b.ReceiveOpen,
			ReceiveNotification: b.ReceiveNotification,
			ReceiveKeepalive:    b.ReceiveKeepalive,
			ReceiveRefresh:      b.ReceiveRefresh,
			ReceiveState:        b.ReceiveState,
			ReceiveSent:         b.ReceiveSent,
			ReceiveNegotiated:   b.ReceiveNegotiated,
			SendUpdate:          b.SendUpdate,
			SendRefresh:         b.SendRefresh,
		})
	}
	return result
}

// getPluginEncoder returns the encoder for a plugin, or empty if not found.
func (a *reactorAPIAdapter) getPluginEncoder(name string) string {
	for _, pc := range a.r.config.Plugins {
		if pc.Name == name {
			return pc.Encoder
		}
	}
	return ""
}

// Transaction support for commit-based batching.
// Note: Per-peer Adj-RIB-Out transactions removed. Use CommitManager instead.

// BeginTransaction starts a new transaction for batched route updates.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> start".
func (a *reactorAPIAdapter) BeginTransaction(peerSelector, label string) error {
	return errors.New("per-peer transactions removed; use 'commit <name> start' instead")
}

// CommitTransaction commits the current transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> end".
func (a *reactorAPIAdapter) CommitTransaction(peerSelector string) (plugin.TransactionResult, error) {
	return plugin.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> end' instead")
}

// CommitTransactionWithLabel commits, verifying the label matches.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> end".
func (a *reactorAPIAdapter) CommitTransactionWithLabel(peerSelector, label string) (plugin.TransactionResult, error) {
	return plugin.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> end' instead")
}

// RollbackTransaction discards all queued routes in the transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Use CommitManager via "commit <name> rollback".
func (a *reactorAPIAdapter) RollbackTransaction(peerSelector string) (plugin.TransactionResult, error) {
	return plugin.TransactionResult{}, errors.New("per-peer transactions removed; use 'commit <name> rollback' instead")
}

// getMatchingPeers returns peers matching the selector.
// Supports: "*" (all peers), exact IP, or glob patterns (e.g., "192.168.*.*").
func (a *reactorAPIAdapter) getMatchingPeers(selector string) []*Peer {
	a.r.mu.RLock()
	defer a.r.mu.RUnlock()

	// Fast path: all peers
	if selector == "*" || selector == "" {
		peers := make([]*Peer, 0, len(a.r.peers))
		for _, peer := range a.r.peers {
			peers = append(peers, peer)
		}
		return peers
	}

	// Fast path: exact match (no wildcards)
	if peer, ok := a.r.peers[selector]; ok {
		return []*Peer{peer}
	}

	// Glob pattern match
	var peers []*Peer
	for addr, peer := range a.r.peers {
		if ipGlobMatch(selector, addr) {
			peers = append(peers, peer)
		}
	}
	return peers
}

// ipGlobMatch checks if an IP address matches a glob pattern.
// Pattern "*" matches any IP (IPv4 or IPv6).
// For IPv4, each octet can be "*" to match any value 0-255.
// Examples: "192.168.*.*", "10.*.0.1", "*.*.*.1".
func ipGlobMatch(pattern, ip string) bool {
	// "*" or empty matches everything
	if pattern == "*" || pattern == "" {
		return true
	}

	// Check if pattern looks like IPv4 glob (contains dots)
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		patternParts := strings.Split(pattern, ".")
		ipParts := strings.Split(ip, ".")

		if len(patternParts) != 4 || len(ipParts) != 4 {
			return false
		}

		for i := 0; i < 4; i++ {
			if patternParts[i] == "*" {
				continue // wildcard matches any octet
			}
			if patternParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}

	// For IPv6 or exact match, just compare strings
	return pattern == ip
}

// InTransaction returns true if any matching peer has an active transaction.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Always returns false.
func (a *reactorAPIAdapter) InTransaction(peerSelector string) bool {
	return false
}

// TransactionID returns the transaction label for the first matching peer.
//
// Deprecated: Per-peer Adj-RIB-Out removed. Always returns empty string.
func (a *reactorAPIAdapter) TransactionID(peerSelector string) string {
	return ""
}

// SendRoutes sends routes directly to matching peers using CommitService.
// This bypasses OutgoingRIB transaction and is used for named commits.
func (a *reactorAPIAdapter) SendRoutes(peerSelector string, routes []*rib.Route, withdrawals []nlri.NLRI, sendEOR bool) (plugin.TransactionResult, error) {
	peers := a.getMatchingPeers(peerSelector)
	if len(peers) == 0 {
		return plugin.TransactionResult{}, errors.New("no peers match selector")
	}

	var totalResult plugin.TransactionResult

	// Collect families for EOR (from both routes and withdrawals)
	seen := make(map[nlri.Family]bool)
	for _, r := range routes {
		seen[r.NLRI().Family()] = true
	}
	for _, n := range withdrawals {
		seen[n.Family()] = true
	}
	families := make([]nlri.Family, 0, len(seen))
	for f := range seen {
		families = append(families, f)
	}

	// Track stats once (not per-peer)
	totalResult.RoutesAnnounced = len(routes)
	totalResult.RoutesWithdrawn = len(withdrawals)

	for _, peer := range peers {
		// Get encoding context for CommitService
		ctx := peer.SendContext()
		if ctx == nil {
			continue // Peer not established
		}

		// Use CommitService with two-level grouping for announcements
		cs := rib.NewCommitService(peer, ctx, true)

		// Send announcements
		if len(routes) > 0 {
			stats, err := cs.Commit(routes, rib.CommitOptions{SendEOR: false})
			if err != nil {
				continue
			}
			totalResult.UpdatesSent += stats.UpdatesSent
		}

		// Send withdrawals
		if len(withdrawals) > 0 {
			updatesSent := a.sendWithdrawals(peer, withdrawals)
			totalResult.UpdatesSent += updatesSent
		}

		// Send EOR for each family if requested
		if sendEOR {
			for _, f := range families {
				eor := message.BuildEOR(f)
				if err := peer.SendUpdate(eor); err == nil {
					totalResult.UpdatesSent++
				}
			}
		}
	}

	// Build family strings for result
	familyStrs := make([]string, len(families))
	for i, f := range families {
		familyStrs[i] = f.String()
	}
	totalResult.Families = familyStrs

	return totalResult, nil
}

// sendWithdrawals sends withdrawal UPDATE messages for the given NLRIs.
// Groups by family for efficient packing.
// RFC 7911: Uses WriteNLRI for ADD-PATH aware encoding.
func (a *reactorAPIAdapter) sendWithdrawals(peer *Peer, withdrawals []nlri.NLRI) int {
	if len(withdrawals) == 0 {
		return 0
	}

	// Group withdrawals by family
	byFamily := make(map[nlri.Family][]nlri.NLRI)
	for _, n := range withdrawals {
		f := n.Family()
		byFamily[f] = append(byFamily[f], n)
	}

	updatesSent := 0
	ipv4Unicast := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	for family, nlris := range byFamily {
		// RFC 7911: Get ADD-PATH encoding setting
		addPath := peer.addPathFor(family)
		var update *message.Update

		// Pack NLRIs into pooled buffer
		nlriBuf := getBuildBuf()
		off := 0
		for _, n := range nlris {
			off += nlri.WriteNLRI(n, nlriBuf, off, addPath)
		}
		nlriBytes := nlriBuf[:off]

		if family == ipv4Unicast {
			// IPv4 unicast: use WithdrawnRoutes field
			update = &message.Update{
				WithdrawnRoutes: nlriBytes,
			}
		} else {
			// Other families: use MP_UNREACH_NLRI attribute
			mpUnreach := &attribute.MPUnreachNLRI{
				AFI:  attribute.AFI(family.AFI),
				SAFI: attribute.SAFI(family.SAFI),
				NLRI: nlriBytes,
			}
			attrBuf := getBuildBuf()
			attrLen := attribute.WriteAttrTo(mpUnreach, attrBuf, 0)
			update = &message.Update{
				PathAttributes: attrBuf[:attrLen],
			}
			// Send then return attr buffer (nlri already copied into attrBuf by WriteAttrTo)
			if err := peer.SendUpdate(update); err == nil {
				updatesSent++
			}
			putBuildBuf(attrBuf)
			putBuildBuf(nlriBuf)
			continue
		}

		if err := peer.SendUpdate(update); err == nil {
			updatesSent++
		}
		putBuildBuf(nlriBuf)
	}

	return updatesSent
}

// ForwardUpdate forwards a cached UPDATE to peers matching the selector.
// Looks up the update by ID from the cache and sends to matching peers.
// One-shot: deletes from cache after forwarding completes.
//
// Zero-copy optimization: When source and destination encoding contexts match
// (same ASN4, ADD-PATH capabilities), the raw UPDATE bytes are forwarded
// directly without re-encoding.
//
// RFC 8654 compliance: If the UPDATE exceeds a peer's max message size
// (4096 without Extended Message, 65535 with), it is split into multiple
// smaller UPDATEs that each fit within the limit.
func (a *reactorAPIAdapter) ForwardUpdate(sel *selector.Selector, updateID uint64) error {
	// Take ownership of update from cache (removes from cache)
	// Caller must call Release() when done
	update, ok := a.r.recentUpdates.Take(updateID)
	if !ok {
		return ErrUpdateExpired
	}
	defer update.Release() // Return buffer to pool when done

	// Get matching peers
	a.r.mu.RLock()
	var matchingPeers []*Peer
	for _, peer := range a.r.peers {
		addr := peer.Settings().Address
		if sel.Matches(addr) && addr != update.SourcePeerIP {
			// Don't forward back to source peer (implicit loop prevention)
			matchingPeers = append(matchingPeers, peer)
		}
	}
	a.r.mu.RUnlock()

	if len(matchingPeers) == 0 {
		return fmt.Errorf("no peers match selector %s", sel)
	}

	// Calculate update size once (header + body)
	updateSize := message.HeaderLen + len(update.WireUpdate.Payload())

	// Forward to all matching peers, collect errors
	// Lazy parsing: only parse if we need to re-encode
	var parsedUpdate *message.Update
	var errs []error
	var sentCount int

	for _, peer := range matchingPeers {
		if peer.State() != PeerStateEstablished {
			continue // Skip non-established peers
		}

		sentCount++

		// Get max message size for this peer (RFC 8654)
		nc := peer.negotiated.Load()
		extendedMessage := nc != nil && nc.ExtendedMessage
		maxMsgSize := int(message.MaxMessageLength(message.TypeUPDATE, extendedMessage))

		// Check if UPDATE exceeds peer's max message size
		if updateSize > maxMsgSize {
			// Wire-level split: get source context for per-family ADD-PATH lookup
			srcCtxID := update.WireUpdate.SourceCtxID()
			srcCtx := bgpctx.Registry.Get(srcCtxID) // May be nil if not registered

			maxBodySize := maxMsgSize - message.HeaderLen
			splits, err := plugin.SplitWireUpdate(update.WireUpdate, maxBodySize, srcCtx)
			if err != nil {
				errs = append(errs, fmt.Errorf("peer %s: split failed: %w", peer.Settings().Address, err))
				continue
			}
			for _, split := range splits {
				if err := peer.SendRawUpdateBody(split.Payload()); err != nil {
					errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
				}
			}
		} else {
			// Normal path: UPDATE fits based on original size
			destCtxID := peer.SendContextID()

			// Zero-copy path: use raw bytes when contexts match
			// Both must be non-zero (registered) and equal
			srcCtxID := update.WireUpdate.SourceCtxID()
			if srcCtxID != 0 && destCtxID != 0 && srcCtxID == destCtxID {
				if err := peer.SendRawUpdateBody(update.WireUpdate.Payload()); err != nil {
					errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
				}
			} else {
				// Re-encode path: parse (lazily) and send
				if parsedUpdate == nil {
					var parseErr error
					parsedUpdate, parseErr = message.UnpackUpdate(update.WireUpdate.Payload())
					if parseErr != nil {
						return fmt.Errorf("parsing cached update: %w", parseErr)
					}
				}

				// Check repacked size - may differ from original due to ASN4 encoding changes
				// Size = Header(19) + WithdrawnLen(2) + Withdrawn + AttrLen(2) + Attrs + NLRI
				repackedSize := message.HeaderLen + 4 + len(parsedUpdate.WithdrawnRoutes) +
					len(parsedUpdate.PathAttributes) + len(parsedUpdate.NLRI)
				if repackedSize > maxMsgSize {
					// Split via parsed UPDATE using destination's ADD-PATH state
					// TODO: SplitUpdateWithAddPath uses single addPath for all families.
					// For mixed-family UPDATEs, this may be incorrect. Consider updating
					// SplitUpdateWithAddPath to accept EncodingContext in future.
					destSendCtx := peer.SendContext()
					addPath := destSendCtx != nil && destSendCtx.AddPathFor(nlri.IPv4Unicast)

					chunks, splitErr := message.SplitUpdateWithAddPath(parsedUpdate, maxMsgSize, addPath)
					if splitErr != nil {
						errs = append(errs, fmt.Errorf("peer %s: split failed: %w", peer.Settings().Address, splitErr))
					} else {
						for _, chunk := range chunks {
							if err := peer.SendUpdate(chunk); err != nil {
								errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
							}
						}
					}
				} else {
					if err := peer.SendUpdate(parsedUpdate); err != nil {
						errs = append(errs, fmt.Errorf("peer %s: %w", peer.Settings().Address, err))
					}
				}
			}
		}
	}

	// Buffer released by deferred update.Release()

	if sentCount == 0 {
		return fmt.Errorf("no established peers to forward to")
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendRoutesWithLimit sends routes in batches that fit within maxMsgSize.
//
// When GroupUpdates is enabled (default), routes with identical attributes are
// grouped into single UPDATE messages. This reduces UPDATE count from O(routes)
// to O(routes/capacity), dramatically improving efficiency for large route sets.
//
// When GroupUpdates is disabled, routes are sent individually (legacy behavior).
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendRoutesWithLimit(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Fall back to individual sending if grouping disabled
	if !peer.settings.GroupUpdates {
		return a.sendRoutesIndividually(peer, routes, maxMsgSize)
	}

	// Group routes by attributes + AS_PATH
	attrGroups := rib.GroupByAttributesTwoLevel(routes)

	var errs []error
	for _, attrGroup := range attrGroups {
		for _, aspGroup := range attrGroup.ByASPath {
			if err := a.sendASPathGroup(peer, &attrGroup, &aspGroup, maxMsgSize); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendRoutesIndividually sends routes one at a time (legacy behavior).
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendRoutesIndividually(peer *Peer, routes []*rib.Route, maxMsgSize int) error {
	var errs []error

	for _, route := range routes {
		family := route.NLRI().Family()
		addPath := peer.addPathFor(family)
		asn4 := peer.asn4()
		attrBuf := getBuildBuf()
		update := buildRIBRouteUpdate(attrBuf, route, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
		sendErr := peer.sendUpdateWithSplit(update, maxMsgSize, family)
		putBuildBuf(attrBuf)
		if sendErr != nil {
			errs = append(errs, fmt.Errorf("route %s: %w", route.NLRI(), sendErr))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// sendASPathGroup sends routes in an AS_PATH group as efficiently as possible.
// For IPv4 unicast: uses BuildGroupedUnicastWithLimit to pack multiple NLRIs.
// For MP families: builds UPDATE with MP_REACH_NLRI containing grouped NLRIs.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendASPathGroup(peer *Peer, attrGroup *rib.AttributeGroup, aspGroup *rib.ASPathGroup, maxMsgSize int) error {
	if len(aspGroup.Routes) == 0 {
		return nil
	}

	family := attrGroup.Family
	addPath := peer.addPathFor(family)
	asn4 := peer.asn4()

	// IPv4 unicast: use BuildGroupedUnicastWithLimit
	if family.AFI == nlri.AFIIPv4 && family.SAFI == nlri.SAFIUnicast {
		return a.sendGroupedIPv4Unicast(peer, aspGroup.Routes, asn4, addPath, maxMsgSize)
	}

	// MP families: build UPDATE with MP_REACH_NLRI containing grouped NLRIs
	return a.sendGroupedMPFamily(peer, aspGroup.Routes, family, asn4, addPath, maxMsgSize)
}

// sendGroupedIPv4Unicast sends grouped IPv4 unicast routes using BuildGroupedUnicastWithLimit.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendGroupedIPv4Unicast(peer *Peer, routes []*rib.Route, asn4, addPath bool, maxMsgSize int) error {
	// Check if any route has complex AS_PATH (AS_SET, CONFED, multiple segments)
	// that can't be represented in UnicastParams.ASPath (which is just []uint32).
	// Fall back to individual sending for such routes.
	for _, route := range routes {
		if hasComplexASPath(route) {
			return a.sendRoutesIndividually(peer, routes, maxMsgSize)
		}
	}

	// Convert to UnicastParams
	params := make([]message.UnicastParams, len(routes))
	for i, route := range routes {
		params[i] = toRIBRouteUnicastParams(route)
	}

	// Build grouped UPDATEs respecting size limits
	ub := message.NewUpdateBuilder(peer.settings.LocalAS, peer.settings.IsIBGP(), asn4, addPath)
	updates, err := ub.BuildGroupedUnicastWithLimit(params, maxMsgSize)
	if err != nil {
		return fmt.Errorf("building grouped IPv4 unicast: %w", err)
	}

	// Send all UPDATEs
	var errs []error
	for _, update := range updates {
		if err := peer.SendUpdate(update); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// hasComplexASPath returns true if the route's AS_PATH can't be represented
// as a simple []uint32 (has AS_SET, CONFED segments, or multiple segments).
func hasComplexASPath(route *rib.Route) bool {
	asPath := route.ASPath()
	if asPath == nil || len(asPath.Segments) == 0 {
		return false
	}

	// Multiple segments = complex
	if len(asPath.Segments) > 1 {
		return true
	}

	// Single segment: only AS_SEQUENCE is simple
	seg := asPath.Segments[0]
	return seg.Type != attribute.ASSequence
}

// sendGroupedMPFamily sends grouped MP family routes (IPv6, VPN, etc.).
// Packs multiple NLRIs into MP_REACH_NLRI attribute.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendGroupedMPFamily(peer *Peer, routes []*rib.Route, family nlri.Family, asn4, addPath bool, maxMsgSize int) error {
	if len(routes) == 0 {
		return nil
	}

	// Pack all NLRIs into pooled buffer
	nlriBuf := getBuildBuf()
	defer putBuildBuf(nlriBuf)
	off := 0
	for _, route := range routes {
		off += nlri.WriteNLRI(route.NLRI(), nlriBuf, off, addPath)
	}
	nlriBytes := nlriBuf[:off]

	// Build grouped UPDATE with all NLRIs
	firstRoute := routes[0]
	attrBuf := getBuildBuf()
	defer putBuildBuf(attrBuf)
	groupedUpdate := a.buildGroupedMPUpdate(attrBuf, firstRoute, nlriBytes, family, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4)

	// Check actual size of grouped update
	msgSize := message.HeaderLen + 4 + len(groupedUpdate.PathAttributes)
	if msgSize <= maxMsgSize {
		return peer.SendUpdate(groupedUpdate)
	}

	// Need to split - calculate available space for NLRI
	// MP_REACH_NLRI overhead: header(3-4) + AFI(2) + SAFI(1) + NH-len(1) + NH + Reserved(1)
	// Next-hop sizes: IPv4=4, IPv6=16 or 32 (global+link-local), VPN=12 or 24
	nhLen := nextHopLength(family, firstRoute.NextHop())
	mpReachOverhead := 4 + 2 + 1 + 1 + nhLen + 1 // extended header + AFI + SAFI + NH-len + NH + reserved

	// Base attributes (without MP_REACH_NLRI's NLRI portion)
	baseAttrSize := len(groupedUpdate.PathAttributes) - len(nlriBytes)
	availableNLRISpace := maxMsgSize - message.HeaderLen - 4 - baseAttrSize - mpReachOverhead

	if availableNLRISpace <= 0 {
		return fmt.Errorf("attributes too large for MP family: %d bytes, max %d", baseAttrSize+mpReachOverhead, maxMsgSize-message.HeaderLen-4)
	}

	// Split NLRIs into chunks
	chunks, err := message.ChunkMPNLRI(nlriBytes, family.AFI, family.SAFI, addPath, availableNLRISpace)
	if err != nil {
		return fmt.Errorf("chunking MP NLRI: %w", err)
	}

	var errs []error
	for _, chunk := range chunks {
		chunkUpdate := a.buildGroupedMPUpdate(attrBuf, firstRoute, chunk, family, peer.settings.LocalAS, peer.settings.IsIBGP(), asn4)
		if err := peer.SendUpdate(chunkUpdate); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// nextHopLength returns the wire length of next-hop for a given family.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func nextHopLength(family nlri.Family, nh netip.Addr) int {
	switch {
	case family.AFI == nlri.AFIIPv4:
		return 4
	case family.AFI == nlri.AFIIPv6:
		// Could be 16 (global only) or 32 (global + link-local)
		// Conservative: assume 32 for safety
		return 32
	case family.SAFI == nlri.SAFIVPN:
		// VPN: RD (8) + address (4 or 16)
		if family.AFI == nlri.AFIIPv4 {
			return 12 // RD + IPv4
		}
		return 24 // RD + IPv6
	default:
		// Conservative default
		if nh.Is6() {
			return 32
		}
		return 4
	}
}

// buildGroupedMPUpdate builds an UPDATE with MP_REACH_NLRI containing multiple NLRIs.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) buildGroupedMPUpdate(attrBuf []byte, templateRoute *rib.Route, nlriBytes []byte, family nlri.Family, localAS uint32, isIBGP bool, asn4 bool) *message.Update {
	off := 0

	// Create encoding context for ASPath encoding
	dstCtx := bgpctx.EncodingContextForASN4(asn4)

	// 1. ORIGIN
	origin := attribute.OriginIGP
	for _, attr := range templateRoute.Attributes() {
		if o, ok := attr.(attribute.Origin); ok {
			origin = o
			break
		}
	}
	off += attribute.WriteAttrTo(origin, attrBuf, off)

	// 2. AS_PATH
	storedASPath := templateRoute.ASPath()
	hasStoredASPath := storedASPath != nil && len(storedASPath.Segments) > 0

	var asPath *attribute.ASPath
	switch {
	case hasStoredASPath:
		asPath = storedASPath
	case isIBGP || localAS == 0:
		asPath = &attribute.ASPath{Segments: nil}
	default:
		asPath = &attribute.ASPath{
			Segments: []attribute.ASPathSegment{{
				Type: attribute.ASSequence,
				ASNs: []uint32{localAS},
			}},
		}
	}
	off += attribute.WriteAttrToWithContext(asPath, attrBuf, off, nil, dstCtx)

	// MP_REACH_NLRI with grouped NLRIs
	mpReach := &attribute.MPReachNLRI{
		AFI:      attribute.AFI(family.AFI),
		SAFI:     attribute.SAFI(family.SAFI),
		NextHops: []netip.Addr{templateRoute.NextHop()},
		NLRI:     nlriBytes,
	}
	off += attribute.WriteAttrTo(mpReach, attrBuf, off)

	// LOCAL_PREF for iBGP
	if isIBGP {
		off += attribute.WriteAttrTo(attribute.LocalPref(100), attrBuf, off)
	}

	// Copy optional attributes
	for _, attr := range templateRoute.Attributes() {
		switch attr.(type) {
		case attribute.Origin, *attribute.ASPath, *attribute.NextHop, attribute.LocalPref:
			continue
		case attribute.MED, attribute.Communities,
			attribute.ExtendedCommunities, attribute.LargeCommunities,
			attribute.IPv6ExtendedCommunities,
			attribute.AtomicAggregate, *attribute.Aggregator,
			attribute.OriginatorID, attribute.ClusterList:
			off += attribute.WriteAttrTo(attr, attrBuf, off)
		}
	}

	return &message.Update{
		PathAttributes: attrBuf[:off],
	}
}

// toRIBRouteUnicastParams converts a RIB route to UnicastParams for grouped building.
// Extracts attributes from the route's attribute slice for use with BuildGroupedUnicastWithLimit.
func toRIBRouteUnicastParams(route *rib.Route) message.UnicastParams {
	params := message.UnicastParams{
		NextHop: route.NextHop(),
		Origin:  attribute.OriginIGP, // Default
	}

	// Extract prefix and path-id from NLRI
	if n := route.NLRI(); n != nil {
		if inet, ok := n.(*nlri.INET); ok {
			params.Prefix = inet.Prefix()
			params.PathID = inet.PathID()
		}
	}

	// Extract AS_PATH if present
	if asPath := route.ASPath(); asPath != nil {
		for _, seg := range asPath.Segments {
			if seg.Type == attribute.ASSequence {
				params.ASPath = append(params.ASPath, seg.ASNs...)
			}
		}
	}

	// Extract attributes from the route's attribute slice
	for _, attr := range route.Attributes() {
		switch a := attr.(type) {
		case attribute.Origin:
			params.Origin = a
		case attribute.MED:
			params.MED = uint32(a)
		case attribute.LocalPref:
			params.LocalPreference = uint32(a)
		case attribute.Communities:
			params.Communities = make([]uint32, len(a))
			for i, c := range a {
				params.Communities[i] = uint32(c)
			}
		case attribute.ExtendedCommunities:
			params.ExtCommunityBytes = a.Pack()
		case attribute.LargeCommunities:
			params.LargeCommunities = make([][3]uint32, len(a))
			for i, lc := range a {
				params.LargeCommunities[i] = [3]uint32{lc.GlobalAdmin, lc.LocalData1, lc.LocalData2}
			}
		case attribute.AtomicAggregate:
			params.AtomicAggregate = true
		case *attribute.Aggregator:
			params.HasAggregator = true
			params.AggregatorASN = a.ASN
			if a.Address.Is4() {
				params.AggregatorIP = a.Address.As4()
			}
		case attribute.OriginatorID:
			if addr := netip.Addr(a); addr.Is4() {
				ip4 := addr.As4()
				params.OriginatorID = uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
			}
		case attribute.ClusterList:
			params.ClusterList = make([]uint32, len(a))
			copy(params.ClusterList, a)
		}
	}

	return params
}

// sendWithdrawalsWithLimit sends withdrawals using SplitUpdate for size limiting.
// Groups withdrawals by family to ensure correct Add-Path detection for each.
// Uses the same splitting infrastructure as announcements for consistency.
//
//nolint:unused // Orphaned: was called by sendSplitUpdate (deleted), may be useful for future adj-rib-out features
func (a *reactorAPIAdapter) sendWithdrawalsWithLimit(peer *Peer, withdraws []nlri.NLRI, maxMsgSize int) error {
	if len(withdraws) == 0 {
		return nil
	}

	// Group withdrawals by family for correct Add-Path detection
	// BGP spec requires same-family NLRIs in each UPDATE, and Add-Path is per-family
	byFamily := make(map[nlri.Family][]byte)
	for _, n := range withdraws {
		family := n.Family()
		byFamily[family] = append(byFamily[family], n.Bytes()...)
	}

	var errs []error
	for family, withdrawnBytes := range byFamily {
		// Build withdrawal-only UPDATE for this family
		update := &message.Update{
			WithdrawnRoutes: withdrawnBytes,
		}

		// Use sendUpdateWithSplit for consistent splitting and Add-Path handling
		if err := peer.sendUpdateWithSplit(update, maxMsgSize, family); err != nil {
			errs = append(errs, fmt.Errorf("sending %s withdrawals: %w", family, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// DeleteUpdate removes an update from the cache without forwarding.
// Used when controller decides not to forward (filtering).
func (a *reactorAPIAdapter) DeleteUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Delete(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// RetainUpdate prevents eviction of a cached UPDATE.
// Used by API for graceful restart - retain routes for replay.
func (a *reactorAPIAdapter) RetainUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Retain(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// ReleaseUpdate allows eviction of a previously retained UPDATE.
// Resets TTL to default expiration time.
func (a *reactorAPIAdapter) ReleaseUpdate(updateID uint64) error {
	if !a.r.recentUpdates.Release(updateID) {
		return ErrUpdateExpired
	}
	return nil
}

// ListUpdates returns all cached msg-ids (retained or non-expired).
func (a *reactorAPIAdapter) ListUpdates() []uint64 {
	return a.r.recentUpdates.List()
}

// SignalAPIReady signals that an API process is ready.
func (a *reactorAPIAdapter) SignalAPIReady() {
	a.r.SignalAPIReady()
}

// AddAPIProcessCount adds to the number of API processes to wait for.
func (a *reactorAPIAdapter) AddAPIProcessCount(count int) {
	a.r.AddAPIProcessCount(count)
}

// SignalPluginStartupComplete signals that all plugin phases are done.
func (a *reactorAPIAdapter) SignalPluginStartupComplete() {
	a.r.SignalPluginStartupComplete()
}

// SignalPeerAPIReady signals that a peer-specific API initialization is complete.
func (a *reactorAPIAdapter) SignalPeerAPIReady(peerAddr string) {
	a.r.SignalPeerAPIReady(peerAddr)
}

// SendRawMessage sends raw bytes to a peer.
// If msgType is 0, payload is a full BGP packet (user provides marker+header).
// If msgType is non-zero, payload is message body (we add the header).
func (a *reactorAPIAdapter) SendRawMessage(peerAddr netip.Addr, msgType uint8, payload []byte) error {
	a.r.mu.RLock()
	peer, exists := a.r.peers[peerAddr.String()]
	a.r.mu.RUnlock()

	if !exists {
		return ErrPeerNotFound
	}

	return peer.SendRawMessage(msgType, payload)
}

// New creates a new reactor with the given configuration.
func New(config *Config) *Reactor {
	// Apply defaults for recent update cache
	ttl := config.RecentUpdateTTL
	if ttl == 0 {
		ttl = 60 * time.Second // Default: 60 seconds
	}
	maxEntries := config.RecentUpdateMax
	if maxEntries == 0 {
		maxEntries = 100000 // Default: 100k entries
	}

	return &Reactor{
		config:        config,
		peers:         make(map[string]*Peer),
		listeners:     make(map[netip.Addr]*Listener),
		ribIn:         rib.NewIncomingRIB(),
		ribStore:      rib.NewRouteStore(100), // Buffer size for dedup workers
		watchdog:      NewWatchdogManager(),
		recentUpdates: NewRecentUpdateCache(ttl, maxEntries),
		configTree:    config.ConfigTree,
	}
}

// WatchdogManager returns the global watchdog pool manager.
func (r *Reactor) WatchdogManager() *WatchdogManager {
	return r.watchdog
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

// AddWatchdogRoute adds a route to a global watchdog pool.
// Creates the pool if it doesn't exist.
// The route will be announced to all peers when "watchdog announce <name>" is called.
// Returns ErrRouteExists if a route with the same key already exists in the pool.
func (r *Reactor) AddWatchdogRoute(route StaticRoute, poolName string) error {
	_, err := r.watchdog.AddRoute(poolName, route)
	return err
}

// RemoveWatchdogRoute removes a route from a global watchdog pool.
// Returns ErrWatchdogNotFound if pool doesn't exist.
// Returns ErrWatchdogRouteNotFound if route doesn't exist in pool.
// Sends withdrawals to all peers that had the route announced.
func (r *Reactor) RemoveWatchdogRoute(routeKey, poolName string) error {
	// Check pool exists first (for better error message)
	if r.watchdog.GetPool(poolName) == nil {
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, poolName)
	}

	// Atomically remove route and get its data for withdrawals
	removedRoute, ok := r.watchdog.RemoveAndGetRoute(poolName, routeKey)
	if !ok {
		return fmt.Errorf("%w: %s", ErrWatchdogRouteNotFound, routeKey)
	}

	// Send withdrawals to all peers that had this route announced
	// Route is already removed from pool, so no race condition
	r.mu.RLock()
	for _, peer := range r.peers {
		if peer.State() != PeerStateEstablished {
			continue
		}
		peerAddr := peer.Settings().Address.String()
		// Note: removedRoute.announced is no longer protected by pool mutex,
		// but it's safe because the route is now orphaned (no concurrent access)
		if removedRoute.announced[peerAddr] {
			// RFC 4271 Section 4.3 - Send withdrawal UPDATE (zero-allocation path)
			// Best effort, continue on error
			_ = peer.SendWithdraw(removedRoute.Prefix)
		}
	}
	r.mu.RUnlock()

	return nil
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
	if r.api == nil || neg == nil {
		return
	}

	peerInfo := plugin.PeerInfo{
		Address:      peer.settings.Address,
		LocalAddress: peer.settings.LocalAddress,
		PeerAS:       peer.settings.PeerAS,
		LocalAS:      peer.settings.LocalAS,
	}

	decoded := plugin.NegotiatedToDecoded(neg)
	r.api.OnPeerNegotiated(peerInfo, decoded)
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
func (r *Reactor) notifyMessageReceiver(peerAddr netip.Addr, msgType message.MessageType, rawBytes []byte, wireUpdate *plugin.WireUpdate, ctxID bgpctx.ContextID, direction string, buf []byte) bool {
	r.mu.RLock()
	receiver := r.messageReceiver
	peer, hasPeer := r.peers[peerAddr.String()]

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
	timestamp := time.Now()

	var msg plugin.RawMessage
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
		msg = plugin.RawMessage{
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

		msg = plugin.RawMessage{
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

	// Route to handler FIRST (while buf is definitely valid)
	if direction == "sent" {
		receiver.OnMessageSent(peerInfo, msg)
	} else {
		receiver.OnMessageReceived(peerInfo, msg)
	}

	// THEN cache for later forwarding (only received UPDATEs)
	// Add() returns true if accepted (cache owns buf), false if rejected (caller keeps buf)
	if direction == "received" && wireUpdate != nil && buf != nil {
		kept = r.recentUpdates.Add(&ReceivedUpdate{
			WireUpdate:   wireUpdate, // Zero-copy: slices into buf
			poolBuf:      buf,        // Cache owns buf if accepted
			SourcePeerIP: peerAddr,
			ReceivedAt:   timestamp,
		})
		// If rejected (kept=false), session returns buf after handleUpdate completes
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

	key := settings.Address.String()
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
	peer.SetGlobalWatchdog(r.watchdog)
	peer.SetReactor(r)
	// Set message callback to forward raw bytes to reactor's message receiver
	peer.messageCallback = r.notifyMessageReceiver
	r.peers[key] = peer

	// If reactor is running, start the peer and create listener if needed
	if r.running {
		// Start listener for this LocalAddress if it doesn't exist
		if settings.LocalAddress.IsValid() {
			if _, hasListener := r.listeners[settings.LocalAddress]; !hasListener {
				if err := r.startListenerForAddress(settings.LocalAddress); err != nil {
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
func (r *Reactor) RemovePeer(addr netip.Addr) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Normalize address for consistent lookup (handles IPv4-mapped IPv6)
	addr = addr.Unmap()

	key := addr.String()
	peer, exists := r.peers[key]
	if !exists {
		return ErrPeerNotFound
	}

	localAddr := peer.Settings().LocalAddress

	// Stop peer if running
	peer.Stop()

	delete(r.peers, key)

	// Check if any other peer uses this LocalAddress
	if localAddr.IsValid() {
		stillUsed := false
		for _, p := range r.peers {
			if p.Settings().LocalAddress == localAddr {
				stillUsed = true
				break
			}
		}

		// Stop listener if no longer needed
		if !stillUsed {
			if listener, ok := r.listeners[localAddr]; ok {
				listener.Stop()
				waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = listener.Wait(waitCtx)
				cancel()
				delete(r.listeners, localAddr)
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
	settings.Passive = config.Passive

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
	r.startTime = time.Now()

	// Start legacy listener if ListenAddr is configured (backward compatibility)
	if r.config.ListenAddr != "" {
		r.listener = NewListener(r.config.ListenAddr)
		r.listener.SetHandler(r.handleConnection)
		if err := r.listener.StartWithContext(r.ctx); err != nil {
			r.cancel()
			return err
		}
	}

	// Start multi-listeners based on peer LocalAddresses (new behavior)
	// Collect unique LocalAddresses from peers
	localAddrs := make(map[netip.Addr]struct{})
	for _, peer := range r.peers {
		localAddr := peer.Settings().LocalAddress
		if localAddr.IsValid() {
			localAddrs[localAddr] = struct{}{}
		}
	}

	// Create listener for each unique LocalAddress
	for addr := range localAddrs {
		if err := r.startListenerForAddress(addr); err != nil {
			// Cleanup already-started listeners on error
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
		apiConfig := &plugin.ServerConfig{
			SocketPath:         r.config.APISocketPath,
			ConfiguredFamilies: r.config.ConfiguredFamilies,
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
		r.api = plugin.NewServer(apiConfig, &reactorAPIAdapter{r})
		// Set API server as message receiver for raw byte access
		r.messageReceiver = r.api
		// Register API state observer for peer lifecycle events
		r.AddPeerObserver(&apiStateObserver{server: r.api, reactor: r})

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
		adapter := &reactorAPIAdapter{r: r}
		if err := adapter.Reload(); err != nil {
			reactorLogger().Error("config reload failed", "error", err)
		} else {
			reactorLogger().Info("config reloaded")
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

// startListenerForAddress creates and starts a listener for the given local address.
// Must be called with r.mu held.
func (r *Reactor) startListenerForAddress(addr netip.Addr) error {
	// Check if listener already exists for this address
	if _, exists := r.listeners[addr]; exists {
		return nil // Already listening on this address
	}

	// Use config.Port directly (0 means ephemeral port for testing)
	// Production configs should set Port explicitly (typically 179)
	listenAddr := net.JoinHostPort(addr.String(), strconv.Itoa(r.config.Port))
	listener := NewListener(listenAddr)

	// Capture addr in closure so handleConnectionWithContext knows which listener accepted
	localAddr := addr
	listener.SetHandler(func(conn net.Conn) {
		r.handleConnectionWithContext(conn, localAddr)
	})

	if err := listener.StartWithContext(r.ctx); err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	r.listeners[addr] = listener
	return nil
}

// stopAllListeners stops all multi-listeners and waits for them to finish.
// Must be called with r.mu held.
func (r *Reactor) stopAllListeners() {
	for addr, listener := range r.listeners {
		listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = listener.Wait(waitCtx)
		cancel()
		delete(r.listeners, addr)
	}
}

// nativeFamilies are RFC families handled natively by the engine without plugins.
// These don't require decoder plugins - the engine decodes them directly.
//
// TODO: Migrate non-RFC-4271 families to plugins as we implement family plugins.
// Only ipv4/unicast is truly "native" per RFC 4271. Others are here temporarily
// until we have plugins for them (like we do for flowspec).
var nativeFamilies = map[string]bool{
	// RFC 4271 - BGP-4 (IPv4 unicast) - truly native
	"ipv4/unicast": true,
	// RFC 4760 - Multiprotocol Extensions - TODO: migrate to plugin
	"ipv6/unicast":   true,
	"ipv4/multicast": true,
	"ipv6/multicast": true,
	// RFC 8277 - MPLS Labels (labeled unicast) - TODO: migrate to plugin
	"ipv4/mpls-label": true,
	"ipv6/mpls-label": true,
	// RFC 4364/4659 - VPN (MPLS VPN) - TODO: migrate to plugin
	"ipv4/vpn": true,
	"ipv6/vpn": true,
	// RFC 7752 - BGP-LS - TODO: migrate to plugin
	"bgp-ls/bgp-ls":    true,
	"bgp-ls/bgp-ls-sr": true,
	// RFC 6514 - MVPN - TODO: migrate to plugin
	"ipv4/mvpn": true,
	"ipv6/mvpn": true,
	// RFC 7432 - EVPN - TODO: migrate to plugin
	"l2vpn/evpn": true,
	// RFC 4761/4762 - VPLS - TODO: migrate to plugin
	"l2vpn/vpls": true,
	// draft-ietf-bess-mup - MUP - TODO: migrate to plugin
	"ipv4/mup": true,
	"ipv6/mup": true,
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
func (r *Reactor) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Stop API server
	if r.api != nil {
		r.api.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.api.Wait(waitCtx)
		cancel()
	}

	// Stop legacy listener
	if r.listener != nil {
		r.listener.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = r.listener.Wait(waitCtx)
		cancel()
	}

	// Stop all multi-listeners
	r.stopAllListeners()

	// Stop signal handler
	if r.signals != nil {
		r.signals.Stop()
		waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = r.signals.Wait(waitCtx)
		cancel()
	}

	// Stop all peers
	for _, peer := range r.peers {
		peer.Stop()
	}

	// Wait for all peers
	for _, peer := range r.peers {
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = peer.Wait(waitCtx)
		cancel()
	}

	// Stop RIB store workers
	if r.ribStore != nil {
		r.ribStore.Stop()
	}

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
		_ = conn.Close()
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.peers[peerIP.String()]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		// Unknown peer, close connection
		_ = conn.Close()
		return
	}

	settings := peer.Settings()

	// Call callback if set
	if cb != nil {
		cb(conn, settings)
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
			// Already have a pending connection, reject this one
			r.rejectConnectionCollision(conn)
			return
		}
		// Start goroutine to read OPEN and resolve collision
		go r.handlePendingCollision(peer, conn)
		return
	}

	// Accept connection on peer's session.
	// For passive peers, this triggers the BGP handshake.
	if err := peer.AcceptConnection(conn); err != nil {
		_ = conn.Close()
	}
}

// handleConnectionWithContext handles an incoming TCP connection with listener context.
// listenerAddr is the local address the listener is bound to.
// This validates that the connection arrived on the expected listener for RFC compliance.
func (r *Reactor) handleConnectionWithContext(conn net.Conn, listenerAddr netip.Addr) {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		_ = conn.Close()
		return
	}
	peerIP, _ := netip.AddrFromSlice(remoteAddr.IP)
	peerIP = peerIP.Unmap() // Handle IPv4-mapped IPv6

	r.mu.RLock()
	peer, exists := r.peers[peerIP.String()]
	cb := r.connCallback
	r.mu.RUnlock()

	if !exists {
		// Unknown peer, close connection
		_ = conn.Close()
		return
	}

	settings := peer.Settings()

	// RFC compliance: verify connection arrived on expected listener
	// This validates peer connected to the correct LocalAddress
	if settings.LocalAddress.IsValid() && settings.LocalAddress != listenerAddr {
		// Connection to wrong local address - misconfiguration or routing anomaly
		// Log and reject (logging infrastructure not used here, just close)
		_ = conn.Close()
		return
	}

	// Call callback if set
	if cb != nil {
		cb(conn, settings)
		return
	}

	// RFC 4271 §6.8: Check for collision with ESTABLISHED session.
	if peer.State() == PeerStateEstablished {
		r.rejectConnectionCollision(conn)
		return
	}

	// RFC 4271 §6.8: Check for collision with OpenConfirm session.
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
		_ = conn.Close()
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
	_ = conn.SetReadDeadline(time.Now().Add(holdTime))

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
		timer := time.NewTimer(collisionResolutionTimeout)
		defer timer.Stop()
		select {
		case <-waitSession:
			// Session closed
		case <-timer.C:
			reactorLogger().Warn("session teardown timed out during collision resolution", "peer", peer.Settings().Address)
		}
	}

	// Accept connection with the pre-received OPEN
	if err := peer.AcceptConnectionWithOpen(conn, open); err != nil {
		// Failed to accept - peer may have been stopped or old session not yet closed
		_ = conn.Close()
	}
}

// convertAPIMUPRoute converts an plugin.MUPRouteSpec to a reactor.MUPRoute.
// This function parses the string fields in the API spec into wire-format bytes.
func convertAPIMUPRoute(spec plugin.MUPRouteSpec) (MUPRoute, error) {
	route := MUPRoute{
		IsIPv6: spec.IsIPv6,
	}

	// Convert route type string to numeric
	switch spec.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default:
		return route, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse NextHop
	if spec.NextHop != "" {
		ip, err := netip.ParseAddr(spec.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build MUP NLRI bytes (simplified - reuse from config/loader pattern)
	nlriBytes, err := buildAPIMUPNLRI(spec)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse extended communities if present
	if spec.ExtCommunity != "" {
		ecBytes, err := parseAPIExtCommunity(spec.ExtCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ecBytes
	}

	// Parse SRv6 Prefix-SID if present
	if spec.PrefixSID != "" {
		sidBytes, err := parseAPIPrefixSIDSRv6(spec.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sidBytes
	}

	return route, nil
}

// buildAPIMUPNLRI builds MUP NLRI bytes from API spec.
func buildAPIMUPNLRI(spec plugin.MUPRouteSpec) ([]byte, error) {
	// Determine route type code
	var routeType nlri.MUPRouteType
	switch spec.RouteType {
	case "mup-isd":
		routeType = nlri.MUPISD
	case "mup-dsd":
		routeType = nlri.MUPDSD
	case "mup-t1st":
		routeType = nlri.MUPT1ST
	case "mup-t2st":
		routeType = nlri.MUPT2ST
	default:
		return nil, fmt.Errorf("unknown MUP route type: %s", spec.RouteType)
	}

	// Parse RD
	var rd nlri.RouteDistinguisher
	if spec.RD != "" {
		parsed, err := parseRD(spec.RD)
		if err != nil {
			return nil, fmt.Errorf("invalid RD %q: %w", spec.RD, err)
		}
		rd = parsed
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case nlri.MUPISD:
		if spec.Prefix == "" {
			return nil, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid ISD prefix %q: %w", spec.Prefix, err)
		}
		// Validate family match
		if spec.IsIPv6 != prefix.Addr().Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
		}
		data = make([]byte, mupPrefixLen(prefix))
		writeMUPPrefix(data, 0, prefix)

	case nlri.MUPDSD:
		if spec.Address == "" {
			return nil, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid DSD address %q: %w", spec.Address, err)
		}
		// Validate family match
		if spec.IsIPv6 != addr.Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("address %q is not %s", spec.Address, expected)
		}
		data = addr.AsSlice()

	case nlri.MUPT1ST:
		if spec.Prefix == "" {
			return nil, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid T1ST prefix %q: %w", spec.Prefix, err)
		}
		// Validate family match
		if spec.IsIPv6 != prefix.Addr().Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
		}
		data = make([]byte, mupPrefixLen(prefix))
		writeMUPPrefix(data, 0, prefix)
		// TODO: Add TEID, QFI, endpoint if needed

	case nlri.MUPT2ST:
		if spec.Address == "" {
			return nil, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(spec.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid T2ST endpoint %q: %w", spec.Address, err)
		}
		// Validate family match
		if spec.IsIPv6 != ep.Is6() {
			expected := familyIPv4
			if spec.IsIPv6 {
				expected = familyIPv6
			}
			return nil, fmt.Errorf("address %q is not %s", spec.Address, expected)
		}
		epBytes := ep.AsSlice()
		teid, bits := parseTEIDFieldWithBits(spec.TEID)
		teidLen := teidFieldLen(bits)
		data = make([]byte, 1+len(epBytes)+teidLen)
		data[0] = byte(len(epBytes)*8 + bits) // combined: endpoint bits + TEID bits
		copy(data[1:], epBytes)
		writeTEIDFieldWithBits(data, 1+len(epBytes), teid, bits)
	}

	// Determine AFI
	afi := nlri.AFIIPv4
	if spec.IsIPv6 {
		afi = nlri.AFIIPv6
	}

	mup := nlri.NewMUPFull(afi, nlri.MUPArch3GPP5G, routeType, rd, data)
	buf := make([]byte, mup.Len())
	mup.WriteTo(buf, 0)
	return buf, nil
}

// writeMUPPrefix writes a MUP prefix into buf at off.
func writeMUPPrefix(buf []byte, off int, prefix netip.Prefix) {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	buf[off] = byte(bits)
	copy(buf[off+1:], addrBytes[:prefixBytes])
}

// mupPrefixLen returns the encoded byte length of a MUP prefix.
func mupPrefixLen(prefix netip.Prefix) int {
	return 1 + (prefix.Bits()+7)/8
}

// parseTEIDFieldWithBits parses a TEID string "value/bits" into numeric TEID and bit length.
func parseTEIDFieldWithBits(s string) (uint32, int) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		v, _ := strconv.ParseUint(s, 10, 32)
		return uint32(v), 32
	}
	v, _ := strconv.ParseUint(parts[0], 10, 32)
	bits, err := strconv.Atoi(parts[1])
	if err != nil {
		bits = 32
	}
	return uint32(v), bits
}

// writeTEIDFieldWithBits writes TEID with the specified bit length into buf at off.
// Returns bytes written.
func writeTEIDFieldWithBits(buf []byte, off int, teid uint32, bits int) int {
	if bits <= 0 {
		return 0
	}
	byteLen := (bits + 7) / 8
	for i := range byteLen {
		shift := (byteLen - 1 - i) * 8
		buf[off+i] = byte(teid >> shift)
	}
	return byteLen
}

// teidFieldLen returns the encoded byte length for a TEID field.
func teidFieldLen(bits int) int {
	if bits <= 0 {
		return 0
	}
	return (bits + 7) / 8
}

// parseRD parses a Route Distinguisher string.
// Delegates to nlri.ParseRDString for the actual parsing.
func parseRD(s string) (nlri.RouteDistinguisher, error) {
	return nlri.ParseRDString(s)
}

// parseAPIExtCommunity parses extended community string to bytes.
func parseAPIExtCommunity(s string) ([]byte, error) {
	// Strip brackets if present: "[target:10:10]" -> "target:10:10"
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)

	// Parse "type:ASN:value" format (e.g., "target:10:10")
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid extended community: %s", s)
	}

	ecType := strings.ToLower(parts[0])
	switch ecType {
	case "target":
		// Route Target: Type 0x00, Subtype 0x02
		if len(parts) != 3 {
			return nil, fmt.Errorf("target requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid target values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x02}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	case "origin":
		// Route Origin: Type 0x00, Subtype 0x03
		if len(parts) != 3 {
			return nil, fmt.Errorf("origin requires ASN:value format")
		}
		var asn, val uint64
		if _, err := fmt.Sscanf(parts[1]+":"+parts[2], "%d:%d", &asn, &val); err != nil {
			return nil, fmt.Errorf("invalid origin values: %s:%s", parts[1], parts[2])
		}
		ec := [8]byte{0x00, 0x03}
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(val >> 24)
		ec[5] = byte(val >> 16)
		ec[6] = byte(val >> 8)
		ec[7] = byte(val)
		return ec[:], nil

	default:
		return nil, fmt.Errorf("unknown extended community type: %s", ecType)
	}
}

// parseAPIPrefixSIDSRv6 parses SRv6 Prefix-SID string to bytes.
func parseAPIPrefixSIDSRv6(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}

	// Parse service type
	var serviceType byte
	switch {
	case strings.HasPrefix(s, "l3-service"):
		serviceType = 5 // TLV Type 5: SRv6 L3 Service
		s = strings.TrimPrefix(s, "l3-service")
	case strings.HasPrefix(s, "l2-service"):
		serviceType = 6 // TLV Type 6: SRv6 L2 Service
		s = strings.TrimPrefix(s, "l2-service")
	default:
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected l3-service or l2-service")
	}
	s = strings.TrimSpace(s)

	// Parse IPv6 address
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: missing IPv6 address")
	}
	ipv6, err := netip.ParseAddr(fields[0])
	if err != nil || !ipv6.Is6() {
		return nil, fmt.Errorf("invalid srv6 prefix-sid: expected IPv6 address, got %q", fields[0])
	}

	var behavior byte
	var sidStruct []byte

	// Parse optional behavior (0xNN format)
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "0x") || strings.HasPrefix(f, "0X") {
			behVal, err := parseHexByte(f)
			if err != nil {
				return nil, fmt.Errorf("invalid srv6 behavior %q: %w", f, err)
			}
			behavior = behVal
		} else if strings.HasPrefix(f, "[") {
			// Parse SID structure [LB,LN,Func,Arg,TransLen,TransOffset]
			structStr := strings.TrimPrefix(f, "[")
			structStr = strings.TrimSuffix(structStr, "]")
			parts := strings.Split(structStr, ",")
			if len(parts) != 6 {
				return nil, fmt.Errorf("invalid srv6 SID structure: expected 6 values")
			}
			for _, p := range parts {
				v, err := parseUint8(strings.TrimSpace(p))
				if err != nil {
					return nil, fmt.Errorf("invalid srv6 SID structure value %q: %w", p, err)
				}
				sidStruct = append(sidStruct, v)
			}
		}
	}

	// Build wire format per RFC 9252
	var innerValue []byte
	innerValue = append(innerValue, 0) // reserved
	innerValue = append(innerValue, ipv6.AsSlice()...)
	innerValue = append(innerValue, 0)        // flags
	innerValue = append(innerValue, 0)        // reserved
	innerValue = append(innerValue, behavior) // behavior

	if len(sidStruct) == 6 {
		innerValue = append(innerValue, 0, 1)
		innerValue = append(innerValue, 0, byte(len(sidStruct)))
		innerValue = append(innerValue, sidStruct...)
	}

	innerLen := len(innerValue)
	innerTLV := make([]byte, 0, 4+innerLen)
	innerTLV = append(innerTLV, 0, 1, byte(innerLen>>8), byte(innerLen))
	innerTLV = append(innerTLV, innerValue...)

	outerLen := len(innerTLV)
	result := make([]byte, 0, 3+outerLen)
	result = append(result, serviceType, byte(outerLen>>8), byte(outerLen))
	result = append(result, innerTLV...)

	return result, nil
}

func parseHexByte(s string) (byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	var v uint64
	_, err := fmt.Sscanf(s, "%x", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid hex byte: %s", s)
	}
	return byte(v), nil
}

func parseUint8(s string) (byte, error) {
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("invalid uint8: %s", s)
	}
	return byte(v), nil
}
