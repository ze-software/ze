// Design: docs/architecture/core-design.md -- cross-plugin interfaces for cycle avoidance

package registry

import (
	"context"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// PluginServerAccessor provides the methods that plugins need from the Server
// without importing the server package (which would create import cycles).
type PluginServerAccessor interface {
	ReactorAny() any            // Returns ReactorLifecycle (any to avoid importing plugin types)
	ReactorFor(name string) any // Returns named protocol reactor, or nil
	UpdateProtocolConfig(families, customEvents, customSendTypes []string)
	SetCommitManager(cm any) // Set commit manager (type-asserted by handlers)
}

// ProtocolReactorHandle provides lifecycle methods that any protocol reactor
// exposes to the plugin infrastructure. Protocol-specific handles (like
// BGPReactorHandle) embed this and add protocol-specific methods.
type ProtocolReactorHandle interface {
	SetEventBusAny(eventBus any)
	SetPluginServerAny(server any)
	StartWithContext(ctx context.Context) error
	Stop()
	Wait(ctx context.Context) error
}

// ConfigJournal records transactional apply/undo operations.
// Implemented by pkg/plugin/sdk.Journal.
type ConfigJournal interface {
	Record(apply, undo func() error) error
	Rollback() []error
	Discard()
}

// PeerLifecycleCallback receives peer state change notifications.
// Used by external observers (e.g., HealthRevert) that cannot import bgp/reactor.
type PeerLifecycleCallback interface {
	OnPeerEstablished(peer any)
	OnPeerClosed(peer any, reason string)
}

// MessageCallback receives raw BGP messages without importing bgp/reactor.
// peer is *plugin.PeerInfo, msgType is msgtype.MessageType.
// sent: false = received, true = sent.
type MessageCallback interface {
	OnBGPMessage(peer any, msgType uint8, sent bool, rawBytes []byte)
}

// RIBDumpVisitor receives peer and route data during a RIB snapshot.
// Used by the MRT component to produce TABLE_DUMP_V2 records.
type RIBDumpVisitor struct {
	// OnPeer is called once per peer. Returns the peer index for route entries.
	OnPeer func(peerAddr string, peerASN uint32, bgpID [4]byte, isIPv6 bool) uint16
	// OnRoute is called for each route. attrs contains full wire-format path attributes.
	OnRoute func(peerIndex uint16, afi, safi uint16, prefixLen uint8, prefix []byte, attrs []byte)
}

// RIBDumpCallback iterates all peers and routes for MRT TABLE_DUMP_V2 snapshots.
type RIBDumpCallback interface {
	DumpRIB(visitor RIBDumpVisitor)
}

var (
	ribDumpMu sync.RWMutex
	ribDumpCB RIBDumpCallback
)

// SetRIBDumpCallback registers the RIB snapshot provider. The BGP RIB plugin
// calls this from its own init(), so MRT picks the provider up without either
// side importing the other and without the always-on hub naming a BGP package
// (that import used to pin internal/component/bgp into every binary and defeat
// //go:build ze_bgp). Nil is ignored so a caller cannot clear a live provider.
func SetRIBDumpCallback(cb RIBDumpCallback) {
	if cb == nil {
		return
	}
	ribDumpMu.Lock()
	defer ribDumpMu.Unlock()
	ribDumpCB = cb
}

// GetRIBDumpCallback returns the registered RIB snapshot provider, or nil when
// no RIB is compiled in. MRT reports "no RIB dump provider" in that case rather
// than writing an empty TABLE_DUMP_V2.
func GetRIBDumpCallback() RIBDumpCallback {
	ribDumpMu.RLock()
	defer ribDumpMu.RUnlock()
	return ribDumpCB
}

var (
	mrtCBMu      sync.RWMutex
	mrtMessageCB MessageCallback
	mrtPeerCB    PeerLifecycleCallback
)

// SetMRTMessageCallback registers the MRT raw-message bridge. The MRT plugin
// calls this from its own init(), so the BGP reactor factory
// (bgp/config createReactorFromCoordinator) picks the bridge up via
// GetMRTMessageCallback without the always-on hub importing internal/plugins/mrt
// -- that import used to pin MRT into every binary and defeat //go:build ze_mrt.
// Nil is ignored so a caller cannot clear a live bridge.
func SetMRTMessageCallback(cb MessageCallback) {
	if cb == nil {
		return
	}
	mrtCBMu.Lock()
	defer mrtCBMu.Unlock()
	mrtMessageCB = cb
}

// GetMRTMessageCallback returns the registered MRT message bridge, or nil when
// MRT is compiled out. The reader guards nil (no bridge => no MRT recording).
func GetMRTMessageCallback() MessageCallback {
	mrtCBMu.RLock()
	defer mrtCBMu.RUnlock()
	return mrtMessageCB
}

// SetMRTPeerCallback registers the MRT peer-lifecycle bridge (FSM state-change
// records). Same init()-registration contract as SetMRTMessageCallback.
func SetMRTPeerCallback(cb PeerLifecycleCallback) {
	if cb == nil {
		return
	}
	mrtCBMu.Lock()
	defer mrtCBMu.Unlock()
	mrtPeerCB = cb
}

// GetMRTPeerCallback returns the registered MRT peer bridge, or nil when MRT is
// compiled out. The reader guards nil.
func GetMRTPeerCallback() PeerLifecycleCallback {
	mrtCBMu.RLock()
	defer mrtCBMu.RUnlock()
	return mrtPeerCB
}

// PacketDecoderFunc renders a hex-encoded BGP message as human-readable text
// (or JSON when outputJSON is set). msgType and family disambiguate messages
// whose parse depends on negotiated state; both may be empty for autodetect.
type PacketDecoderFunc func(hexStr, msgType, family string, outputJSON bool) (string, error)

var (
	packetDecoderMu sync.RWMutex
	packetDecoder   PacketDecoderFunc
)

// SetPacketDecoder registers the BGP hex-packet decoder. Called from the gated
// BGP CLI package's init(); the web tool page reaches the decoder through this
// seam so cmd/ze/hub never imports internal/component/bgp/cli.
func SetPacketDecoder(fn PacketDecoderFunc) {
	if fn == nil {
		return
	}
	packetDecoderMu.Lock()
	defer packetDecoderMu.Unlock()
	packetDecoder = fn
}

// GetPacketDecoder returns the registered hex-packet decoder, or nil when BGP
// is compiled out.
func GetPacketDecoder() PacketDecoderFunc {
	packetDecoderMu.RLock()
	defer packetDecoderMu.RUnlock()
	return packetDecoder
}

// BGPReactorHandle extends ProtocolReactorHandle with BGP-specific methods.
// Provides reactor access without importing bgp/reactor (cycle avoidance).
type BGPReactorHandle interface {
	ProtocolReactorHandle
	ConfiguredAutoLoad() (families, events, sendTypes []string)
	SetRestartUntil(t time.Time)
	ReactorLifecycleAdapter() any // Returns ReactorLifecycle (any to avoid importing plugin types)
	StartPeers() error
	AddPeerLifecycleCallback(cb PeerLifecycleCallback)
	AddMessageCallback(cb MessageCallback)
	// Transaction protocol: verify config and return peer change count for budget estimation.
	PeerDiffCount(bgpTree map[string]any) (int, error)
	// Transaction protocol: apply config with journal wrapping for rollback support.
	ReconcilePeersWithJournal(bgpTree map[string]any, j ConfigJournal) error
}

// BGPBootstrap carries the config-load state the hub hands to the BGP reactor
// factory (createReactorFromCoordinator). It replaces a former string-keyed
// coordinator "extra" bag whose 9 values each had a known concrete type at both
// ends. Living in the leaf registry package lets the hub (writer) and bgp/config
// (reader) name it without an import cycle. Callback fields may be nil.
type BGPBootstrap struct {
	ConfigPath string          // config file path ("" or "-" for stdin)
	CLIPlugins []string        // plugin instance names from the config
	ConfigData []byte          // captured config bytes (stdin fallback)
	Store      storage.Storage // blob/file config store
	ChaosSeed  int64           // ze-chaos fault-injection seed (0 = off)
	ChaosRate  float64         // ze-chaos fault rate

	HealthPeerCallback PeerLifecycleCallback // health-revert peer observer
	// MRT message/peer bridges are NOT fields here: MRT self-registers them via
	// registry.SetMRTMessageCallback / SetMRTPeerCallback from its own init(), and
	// bgp/config reads them via the getters. Keeping them off BGPBootstrap is what
	// lets cmd/ze/hub stop importing internal/plugins/mrt (so //go:build ze_mrt can
	// drop MRT). Mirrors the RIB-dump provider seam above.
}

// CoordinatorAccessor provides the methods that plugins need from the Coordinator
// without importing the plugin package.
type CoordinatorAccessor interface {
	SetReactor(r any) error
	RegisterReactor(name string, r any)
	Reactor(name string) any
	Bootstrap() BGPBootstrap
	OnPostStartup(fn func())
}

// ReactorFactoryFunc creates a BGP reactor from coordinator-stored config state.
// Registered by bgp/config at init time, called by bgp/plugin during OnConfigure.
type ReactorFactoryFunc func(coord CoordinatorAccessor) (BGPReactorHandle, error)

var (
	reactorFactoryMu sync.RWMutex
	reactorFactory   ReactorFactoryFunc
)

// RegisterReactorFactory sets the BGP reactor factory function.
func RegisterReactorFactory(fn ReactorFactoryFunc) {
	reactorFactoryMu.Lock()
	defer reactorFactoryMu.Unlock()
	reactorFactory = fn
}

// GetReactorFactory returns the registered reactor factory, or nil.
func GetReactorFactory() ReactorFactoryFunc {
	reactorFactoryMu.RLock()
	defer reactorFactoryMu.RUnlock()
	return reactorFactory
}
