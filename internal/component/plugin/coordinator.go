// Design: docs/architecture/core-design.md -- plugin coordinator for reactor-optional operation

package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

// ErrBGPNotLoaded is returned by BGP-specific methods when no reactor is available.
var ErrBGPNotLoaded = errors.New("bgp not loaded")

// Coordinator implements ReactorLifecycle without requiring a BGP reactor.
// It holds the config tree and lifecycle signaling. BGP-specific methods
// delegate to an optional reactor reference, returning ErrBGPNotLoaded when absent.
//
// Created by the hub at startup. The reactor registers itself via SetReactor
// when BGP loads. Safe for concurrent use.
type Coordinator struct {
	mu          sync.RWMutex
	configTree  map[string]any
	reactor     ReactorLifecycle // nil when BGP not loaded
	extra       map[string]any   // generic key-value store for cross-plugin state
	postStartup func()           // called by SignalPluginStartupComplete (e.g., start peers)
}

// NewCoordinator creates a Coordinator with the given config tree.
func NewCoordinator(configTree map[string]any) *Coordinator {
	return &Coordinator{
		configTree: configTree,
		extra:      make(map[string]any),
	}
}

// SetExtra stores a value by key. Used to pass state between the hub and
// plugins without creating import cycles (e.g., LoadConfigResult, Storage).
func (c *Coordinator) SetExtra(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extra[key] = value
}

// GetExtra retrieves a value by key. Returns nil if not set.
func (c *Coordinator) GetExtra(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extra[key]
}

// SetReactor registers a BGP reactor for delegation.
// Pass nil to unregister. Returns error if r is non-nil but not ReactorLifecycle.
func (c *Coordinator) SetReactor(r any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r == nil {
		c.reactor = nil
		return nil
	}
	rl, ok := r.(ReactorLifecycle)
	if !ok {
		return fmt.Errorf("coordinator: expected ReactorLifecycle, got %T", r)
	}
	c.reactor = rl
	return nil
}

// getReactor returns the current reactor or nil.
func (c *Coordinator) getReactor() ReactorLifecycle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reactor
}

// FullReactor returns the underlying reactor adapter when set (which implements
// both ReactorLifecycle and BGPReactor), or the coordinator itself when no
// reactor is registered. This allows type assertions to BGPReactor to succeed
// when BGP is loaded.
func (c *Coordinator) FullReactor() ReactorLifecycle {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.reactor != nil {
		return c.reactor
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

// VerifyConfig validates peer settings from a BGP config tree.
func (c *Coordinator) VerifyConfig(bgpTree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.VerifyConfig(bgpTree)
	}
	return nil
}

// ApplyConfigDiff applies peer changes from a BGP config tree.
func (c *Coordinator) ApplyConfigDiff(bgpTree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.ApplyConfigDiff(bgpTree)
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
	return ErrBGPNotLoaded
}

// PausePeer pauses reading from a specific peer's session.
func (c *Coordinator) PausePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.PausePeer(addr)
	}
	return ErrBGPNotLoaded
}

// ResumePeer resumes reading from a specific peer's session.
func (c *Coordinator) ResumePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.ResumePeer(addr)
	}
	return ErrBGPNotLoaded
}

// AddDynamicPeer adds a peer from a YANG-parsed config tree.
func (c *Coordinator) AddDynamicPeer(addr netip.Addr, tree map[string]any) error {
	if r := c.getReactor(); r != nil {
		return r.AddDynamicPeer(addr, tree)
	}
	return ErrBGPNotLoaded
}

// RemovePeer removes a peer by address.
func (c *Coordinator) RemovePeer(addr netip.Addr) error {
	if r := c.getReactor(); r != nil {
		return r.RemovePeer(addr)
	}
	return ErrBGPNotLoaded
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
	return ErrBGPNotLoaded
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
// complete (after SignalPluginStartupComplete). Used by BGP to start peers
// only after tier 1+ plugins finish their 5-stage handshake.
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

// --- ReactorCacheCoordinator ---

// RegisterCacheConsumer initializes tracking for a cache-consumer plugin.
func (c *Coordinator) RegisterCacheConsumer(name string, unordered bool) {
	if r := c.getReactor(); r != nil {
		r.RegisterCacheConsumer(name, unordered)
	}
}

// UnregisterCacheConsumer removes a cache-consumer plugin.
func (c *Coordinator) UnregisterCacheConsumer(name string) {
	if r := c.getReactor(); r != nil {
		r.UnregisterCacheConsumer(name)
	}
}
