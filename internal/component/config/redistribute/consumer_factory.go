// Design: docs/guide/redistribution.md -- sources and destinations
// Related: consumer.go -- RedistConsumer and the consumer registry a built consumer joins
// Related: registry.go -- RouteSource, whose ConfigRoot states the same need for a source

package redistribute

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ErrConsumerFactoryConflict is returned when a destination name is registered
// as a consumer factory twice.
var ErrConsumerFactoryConflict = errors.New("redistribute: consumer factory already registered")

// RouteUpdater is the engine connection the redistribute orchestrator lends a
// consumer it builds: its own plugin connection, whose update-route RPC the
// engine authorizes against each peer's `send [ update ]` grant toward the
// orchestrator. The SDK plugin satisfies it.
type RouteUpdater interface {
	UpdateRoute(ctx context.Context, peerSelector, command string) (uint32, uint32, error)
}

// ConsumerFactory declares a destination whose consumer speaks to the engine
// through the orchestrator's connection rather than from a plugin of its own.
//
// The destination's own package registers it from init(), so the orchestrator
// names no destination and imports no protocol: a build without that package
// carries no factory, and the destination is absent from it.
type ConsumerFactory struct {
	// Name is the destination name an operator writes after `destination`.
	Name string
	// ConfigRoot is the config root the destination's protocol runs from. A
	// rule naming the destination in a config without that root is refused
	// (config.RefuseRedistributeRules), because nothing would receive the
	// routes. Empty means the destination needs no root.
	ConfigRoot string
	// New builds the consumer over the orchestrator's engine connection.
	New func(RouteUpdater) RedistConsumer
}

var (
	factoryMu sync.RWMutex
	factories = map[string]ConsumerFactory{}
)

// RegisterConsumerFactory adds f to the factory registry. A second factory
// under the same name is refused, as is one with no name or no constructor.
func RegisterConsumerFactory(f ConsumerFactory) error {
	if f.Name == "" {
		return fmt.Errorf("%w: a factory with no name", ErrConsumerFactoryConflict)
	}
	if f.New == nil {
		return fmt.Errorf("%w: factory %q has no constructor", ErrConsumerFactoryConflict, f.Name)
	}
	factoryMu.Lock()
	defer factoryMu.Unlock()
	if _, ok := factories[f.Name]; ok {
		return fmt.Errorf("%w: %q", ErrConsumerFactoryConflict, f.Name)
	}
	factories[f.Name] = f
	return nil
}

// LookupConsumerFactory returns the factory registered for a destination name.
func LookupConsumerFactory(name string) (ConsumerFactory, bool) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	f, ok := factories[name]
	return f, ok
}

// ConsumerFactories returns every registered factory, sorted by name.
func ConsumerFactories() []ConsumerFactory {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	out := make([]ConsumerFactory, 0, len(factories))
	for _, f := range factories {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b ConsumerFactory) int { return strings.Compare(a.Name, b.Name) })
	return out
}
