// Design: docs/architecture/core-design.md -- cross-plugin interfaces for cycle avoidance

package registry

import (
	"context"
	"sync"
	"time"
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

// BGPReactorHandle extends ProtocolReactorHandle with BGP-specific methods.
// Provides reactor access without importing bgp/reactor (cycle avoidance).
type BGPReactorHandle interface {
	ProtocolReactorHandle
	ConfiguredAutoLoad() (families, events, sendTypes []string)
	SetRestartUntil(t time.Time)
	ReactorLifecycleAdapter() any // Returns ReactorLifecycle (any to avoid importing plugin types)
	StartPeers() error
	// Transaction protocol: verify config and return peer change count for budget estimation.
	PeerDiffCount(bgpTree map[string]any) (int, error)
	// Transaction protocol: apply config with journal wrapping for rollback support.
	ReconcilePeersWithJournal(bgpTree map[string]any, j ConfigJournal) error
}

// CoordinatorAccessor provides the methods that plugins need from the Coordinator
// without importing the plugin package.
type CoordinatorAccessor interface {
	SetReactor(r any) error
	RegisterReactor(name string, r any)
	Reactor(name string) any
	GetExtra(key string) any
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
