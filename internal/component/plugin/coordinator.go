// Design: docs/architecture/core-design.md -- plugin coordinator for reactor-optional operation

package plugin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ErrNoReactor is returned by protocol-specific methods when no reactor is registered.
var ErrNoReactor = errors.New("no reactor loaded")

// Coordinator manages protocol reactors and shared plugin state.
// It holds the config tree, lifecycle signaling, and a registry of named
// protocol reactors. Protocol-specific methods delegate to the appropriate
// reactor, returning ErrNoReactor when absent.
//
// Any protocol (BGP, OSPF, IS-IS) registers its reactor via RegisterReactor.
// The BGP reactor also integrates via SetReactor for ReactorLifecycle delegation.
//
// Created by the hub at startup. Safe for concurrent use.
type Coordinator struct {
	mu          sync.RWMutex
	configTree  map[string]any
	reactors    map[string]any        // named protocol reactors (e.g., "bgp", "ospf")
	bootstrap   registry.BGPBootstrap // config-load state handed to the BGP reactor factory
	postStartup func()                // called by SignalPluginStartupComplete (e.g., start peers)

	// cacheConsumers records every live cache-consumer declaration: plugin name
	// to its ack ordering (true = unordered). A config-path plugin declares this
	// in startup Stage 1, and the BGP reactor is built in the bgp plugin's
	// Stage 2 OnConfigure, so those declarations are all made while no reactor
	// is attached. Holding the set here and replaying it from SetReactor is what
	// makes a declaration independent of that order; a plugin started later
	// declares straight onto the attached reactor and is recorded just the same.
	// Dropping the early ones left bgp-rs registered nowhere: the cache then
	// treated it as a FIFO consumer, and one cumulative ack evicted an entry
	// bgp-rs had batched but not yet forwarded.
	//
	// consumerMu covers the set AND the reactor calls it produces, so a
	// declaration reaches the reactor exactly once however it races SetReactor.
	// Lock order: consumerMu above mu, never the reverse. Holding it across a
	// reactor call is safe because the reactor's cache path never re-enters the
	// Coordinator.
	consumerMu     sync.Mutex
	cacheConsumers map[string]bool
}

// NewCoordinator creates a Coordinator with the given config tree.
func NewCoordinator(configTree map[string]any) *Coordinator {
	return &Coordinator{
		configTree: configTree,
		reactors:   make(map[string]any),
	}
}

// SetBootstrap stores the BGP bootstrap state. Used to pass typed config-load
// state from the hub to the BGP reactor factory without an import cycle
// (formerly a string-keyed extra bag).
func (c *Coordinator) SetBootstrap(bs registry.BGPBootstrap) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bootstrap = bs
}

// Bootstrap returns the stored BGP bootstrap state (zero value if unset).
func (c *Coordinator) Bootstrap() registry.BGPBootstrap {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bootstrap
}

// RegisterReactor stores a named protocol reactor. Any protocol (BGP, OSPF,
// IS-IS) can register its reactor here. Callers retrieve it with Reactor()
// and type-assert to the protocol-specific interface they need.
// Pass nil to unregister.
//
// The name "bgp" is delegated to SetReactor. Both write the same map row, and
// only SetReactor replays the recorded cache-consumer declarations onto the
// arriving reactor; attaching the BGP reactor through this door instead would
// silently reinstate the defect cacheConsumers exists to prevent.
func (c *Coordinator) RegisterReactor(name string, r any) {
	if name == "bgp" {
		if err := c.SetReactor(r); err != nil {
			coordinatorLogger().Error("coordinator: bgp reactor rejected by RegisterReactor", "error", err)
		}
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if r == nil {
		delete(c.reactors, name)
	} else {
		c.reactors[name] = r
	}
}

// Reactor returns the named protocol reactor, or nil if not registered.
// Callers type-assert to the protocol-specific interface they need.
func (c *Coordinator) Reactor(name string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reactors[name]
}

// SetReactor registers the BGP reactor for ReactorLifecycle delegation.
// Pass nil to unregister. Returns error if r is non-nil but not ReactorLifecycle.
// Stores the reactor under the name "bgp" in the named reactor map.
// Attaching a reactor also replays the cache-consumer declarations recorded so
// far, which is the only moment the early ones can reach it: see cacheConsumers.
func (c *Coordinator) SetReactor(r any) error {
	// consumerMu spans the attach AND the replay, so a declaration that arrives
	// in between is either in the snapshot or sees the attached reactor itself,
	// never both and never neither.
	c.consumerMu.Lock()
	defer c.consumerMu.Unlock()

	c.mu.Lock()
	if r == nil {
		delete(c.reactors, "bgp")
		c.mu.Unlock()
		return nil
	}
	lifecycle, ok := r.(ReactorLifecycle)
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("coordinator: expected ReactorLifecycle, got %T", r)
	}
	c.reactors["bgp"] = r
	replay := make(map[string]bool, len(c.cacheConsumers))
	maps.Copy(replay, c.cacheConsumers)
	c.mu.Unlock()

	for name, unordered := range replay {
		lifecycle.RegisterCacheConsumer(name, unordered)
	}
	return nil
}

// getReactor returns the BGP reactor from the named map, or nil.
func (c *Coordinator) getReactor() ReactorLifecycle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if r, ok := c.reactors["bgp"].(ReactorLifecycle); ok {
		return r
	}
	return nil
}

// FullReactor returns the underlying reactor adapter when set (which implements
// both ReactorLifecycle and BGPReactor), or the coordinator itself when no
// reactor is registered. This allows type assertions to BGPReactor to succeed
// when BGP is loaded.
func (c *Coordinator) FullReactor() ReactorLifecycle {
	if r := c.getReactor(); r != nil {
		return r
	}
	return c
}

// --- ReactorConfigurator ---

// GetConfigTree returns the full config as a map for plugin config delivery.
func (c *Coordinator) GetConfigTree() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.configTree
}

// SetConfigTree replaces the running config tree after a successful reload.
func (c *Coordinator) SetConfigTree(tree map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configTree = tree
}

// Reload reloads configuration. Delegates to reactor if present.
func (c *Coordinator) Reload() error {
	if r := c.getReactor(); r != nil {
		return r.Reload()
	}
	return nil
}

// VerifyConfig validates protocol-specific settings from a config tree.
func (c *Coordinator) VerifyConfig(configTree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.VerifyConfig(configTree)
	}
	return nil
}

// ApplyConfigDiff applies incremental changes from a protocol config tree.
func (c *Coordinator) ApplyConfigDiff(configTree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.ApplyConfigDiff(configTree)
	}
	return nil
}

// --- ReactorIntrospector ---

// Peers returns information about all configured peers.
func (c *Coordinator) Peers() []PeerInfo {
	if r := c.getReactor(); r != nil {
		return r.Peers()
	}
	return nil
}

// Stats returns reactor-level statistics.
func (c *Coordinator) Stats() ReactorStats {
	if r := c.getReactor(); r != nil {
		return r.Stats()
	}
	return ReactorStats{}
}

// PeerNegotiatedCapabilities returns negotiated capabilities for a peer.
func (c *Coordinator) PeerNegotiatedCapabilities(addr netip.Addr) *PeerCapabilitiesInfo {
	if r := c.getReactor(); r != nil {
		return r.PeerNegotiatedCapabilities(addr)
	}
	return nil
}

// GetPeerProcessBindings returns process bindings for a specific peer.
func (c *Coordinator) GetPeerProcessBindings(peerAddr netip.Addr) []PeerProcessBinding {
	if r := c.getReactor(); r != nil {
		return r.GetPeerProcessBindings(peerAddr)
	}
	return nil
}

// GetPeerCapabilityConfigs returns capability configurations for all peers.
func (c *Coordinator) GetPeerCapabilityConfigs() []PeerCapabilityConfig {
	if r := c.getReactor(); r != nil {
		return r.GetPeerCapabilityConfigs()
	}
	return nil
}

// --- ReactorPeerController ---

// Stop signals the reactor to shut down.
func (c *Coordinator) Stop() {
	if r := c.getReactor(); r != nil {
		r.Stop()
	}
}

// TeardownPeer gracefully closes a peer session with NOTIFICATION.
func (c *Coordinator) TeardownPeer(addr netip.Addr, subcode uint8, shutdownMsg string) error {
	if r := c.getReactor(); r != nil {
		return r.TeardownPeer(addr, subcode, shutdownMsg)
	}
	return ErrNoReactor
}

// PausePeer pauses reading from a specific peer's session.
func (c *Coordinator) PausePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.PausePeer(addr)
	}
	return ErrNoReactor
}

// ResumePeer resumes reading from a specific peer's session.
func (c *Coordinator) ResumePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.ResumePeer(addr)
	}
	return ErrNoReactor
}

// AddDynamicPeer adds a peer from a YANG-parsed config tree.
func (c *Coordinator) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.AddDynamicPeer(addr, tree)
	}
	return ErrNoReactor
}

// RemovePeer removes a peer by address.
func (c *Coordinator) RemovePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.RemovePeer(addr)
	}
	return ErrNoReactor
}

// FlushForwardPool blocks until all forward pool workers have drained.
func (c *Coordinator) FlushForwardPool(ctx context.Context) error {
	if r := c.getReactor(); r != nil {
		return r.FlushForwardPool(ctx)
	}
	return nil
}

// FlushForwardPoolPeer blocks until the forward pool worker for a specific peer has drained.
func (c *Coordinator) FlushForwardPoolPeer(ctx context.Context, addr string) error {
	if r := c.getReactor(); r != nil {
		return r.FlushForwardPoolPeer(ctx, addr)
	}
	return ErrNoReactor
}

// DrainPeerSync blocks until every Established peer has finished initial route
// sync. No-op without a reactor.
func (c *Coordinator) DrainPeerSync(ctx context.Context) error {
	if r := c.getReactor(); r != nil {
		return r.DrainPeerSync(ctx)
	}
	return nil
}

// --- ReactorStartupCoordinator ---

// SignalAPIReady signals that an API process is ready. No-op without reactor.
func (c *Coordinator) SignalAPIReady() {
	if r := c.getReactor(); r != nil {
		r.SignalAPIReady()
	}
}

// AddAPIProcessCount adds to the number of API processes to wait for. No-op without reactor.
func (c *Coordinator) AddAPIProcessCount(count int) {
	if r := c.getReactor(); r != nil {
		r.AddAPIProcessCount(count)
	}
}

// SignalPluginStartupComplete signals that all plugin phases are done. No-op without reactor.
func (c *Coordinator) SignalPluginStartupComplete() {
	if r := c.getReactor(); r != nil {
		r.SignalPluginStartupComplete()
	}
	c.mu.RLock()
	fn := c.postStartup
	c.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// OnPostStartup registers a callback invoked when all plugin startup phases
// complete (after SignalPluginStartupComplete). Used by protocol reactors
// to defer peer/neighbor establishment until plugins finish their handshake.
func (c *Coordinator) OnPostStartup(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.postStartup = fn
}

// SignalPeerAPIReady signals that a peer-specific API initialization is complete. No-op without reactor.
func (c *Coordinator) SignalPeerAPIReady(peerAddr string) {
	if r := c.getReactor(); r != nil {
		r.SignalPeerAPIReady(peerAddr)
	}
}

// SetPeerUpBarrier declares how many barrier plugins a peer's peer-up event is
// being delivered to. No-op without reactor.
func (c *Coordinator) SetPeerUpBarrier(peerAddr string, expected int) {
	if r := c.getReactor(); r != nil {
		r.SetPeerUpBarrier(peerAddr, expected)
	}
}

// SignalPeerUpBarrier records that one barrier plugin has taken delivery of a
// peer's peer-up event. No-op without reactor.
func (c *Coordinator) SignalPeerUpBarrier(peerAddr string) {
	if r := c.getReactor(); r != nil {
		r.SignalPeerUpBarrier(peerAddr)
	}
}

// --- ReactorCacheCoordinator ---

// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
// The declaration is recorded whether or not a reactor is attached yet, and
// SetReactor replays it: a plugin declares in startup Stage 1, and the BGP
// reactor does not exist until Stage 2 (see cacheConsumers).
func (c *Coordinator) RegisterCacheConsumer(name string, unordered bool) {
	c.consumerMu.Lock()
	defer c.consumerMu.Unlock()

	c.mu.Lock()
	if c.cacheConsumers == nil {
		c.cacheConsumers = make(map[string]bool, 4)
	}
	c.cacheConsumers[name] = unordered
	c.mu.Unlock()

	if r := c.getReactor(); r != nil {
		r.RegisterCacheConsumer(name, unordered)
	}
}

// UnregisterCacheConsumer removes a cache-consumer plugin. The recorded
// declaration is dropped too, so a later SetReactor does not resurrect a
// consumer that has gone.
func (c *Coordinator) UnregisterCacheConsumer(name string) {
	c.consumerMu.Lock()
	defer c.consumerMu.Unlock()

	c.mu.Lock()
	delete(c.cacheConsumers, name)
	c.mu.Unlock()

	if r := c.getReactor(); r != nil {
		r.UnregisterCacheConsumer(name)
	}
}

// ForwardUpdatesDirect forwards cached UPDATEs to explicit destinations.
// Returns ErrNoReactor when no BGP reactor is registered.
func (c *Coordinator) ForwardUpdatesDirect(updateIDs []uint64, destinations []netip.AddrPort, pluginName string, sender Sender) error {
	r := c.getReactor()
	if r == nil {
		return ErrNoReactor
	}
	return r.ForwardUpdatesDirect(updateIDs, destinations, pluginName, sender)
}

// ReleaseUpdates acks cached UPDATEs for pluginName without forwarding.
// Returns ErrNoReactor when no BGP reactor is registered.
func (c *Coordinator) ReleaseUpdates(updateIDs []uint64, pluginName string) error {
	r := c.getReactor()
	if r == nil {
		return ErrNoReactor
	}
	return r.ReleaseUpdates(updateIDs, pluginName)
}

// --- ReactorRelayCoordinator ---

// RelayStoredRoute relays a plugin's stored routes to one destination peer.
// Returns ErrNoReactor when no BGP reactor is registered.
//
// This delegation is not optional plumbing: the plugin server holds the
// Coordinator as its ReactorLifecycle, so a coordinator method that does not
// exist makes the server's type assertion fail and the whole replay degrade to a
// per-route warning. ReactorLifecycle now composes ReactorRelayCoordinator so
// that omission is a compile error rather than a runtime one.
func (c *Coordinator) RelayStoredRoute(destination netip.Addr, routes []rpc.StoredRoute, sender Sender) error {
	r := c.getReactor()
	if r == nil {
		return ErrNoReactor
	}
	return r.RelayStoredRoute(destination, routes, sender)
}
