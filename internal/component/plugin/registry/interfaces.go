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
	ReactorAny() any // Returns ReactorLifecycle (any to avoid importing plugin types)
	UpdateBGPConfig(families, customEvents, customSendTypes []string)
	SetCommitManager(cm any) // Set commit manager (type-asserted by handlers)
}

// BGPReactorHandle provides the methods the BGP plugin needs from the reactor
// without importing bgp/reactor (which would create import cycles via plugin/server).
type BGPReactorHandle interface {
	SetBusAny(bus any)
	SetPluginServerAny(server any)
	ConfiguredAutoLoad() (families, events, sendTypes []string)
	SetRestartUntil(t time.Time)
	ReactorLifecycleAdapter() any // Returns ReactorLifecycle (any to avoid importing plugin types)
	StartWithContext(ctx context.Context) error
	StartPeers() error
	Stop()
	Wait(ctx context.Context) error
}

// CoordinatorAccessor provides the methods that plugins need from the Coordinator
// without importing the plugin package.
type CoordinatorAccessor interface {
	SetReactor(r any) error
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
