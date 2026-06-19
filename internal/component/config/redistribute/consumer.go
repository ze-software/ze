// Design: docs/architecture/core-design.md -- redistribution consumer registry

package redistribute

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// ErrConsumerConflict is returned when a consumer name is re-registered.
var ErrConsumerConflict = errors.New("redistribute: consumer already registered")

// RedistConsumer is the interface that destination protocols implement to receive
// redistributed routes. Each protocol registers one consumer at startup.
type RedistConsumer interface {
	Name() string
	InjectRoute(ctx context.Context, fam family.Family, entry RouteEntry)
	WithdrawRoute(ctx context.Context, fam family.Family, prefix string)
}

// RouteEntry carries the fields needed to inject a redistributed route.
type RouteEntry struct {
	Prefix  string
	NextHop string
	// Source is the originating source name ("connected", "static", "bgp", ...)
	// the orchestrator resolved from the producing protocol. It is informational
	// for consumers that label by source (e.g. the IS-IS consumer's
	// ze_isis_redist_injected_total{source}); the BGP consumer ignores it. Empty
	// when the orchestrator could not resolve a name. Adding it is additive: it
	// does not change the RedistConsumer interface signature.
	Source string
}

var (
	consumerMu sync.RWMutex
	consumers  = map[string]RedistConsumer{}
)

// RegisterConsumer adds a redistribution consumer to the registry.
// Re-registration with the same name is rejected.
func RegisterConsumer(c RedistConsumer) error {
	consumerMu.Lock()
	defer consumerMu.Unlock()
	name := c.Name()
	if _, ok := consumers[name]; ok {
		return fmt.Errorf("%w: %q", ErrConsumerConflict, name)
	}
	consumers[name] = c
	slog.Debug("redistribute consumer registered", "name", name)
	return nil
}

// ReregisterConsumer installs c under its name, replacing any consumer already
// registered under that name. Unlike RegisterConsumer it never returns
// ErrConsumerConflict: it is the idempotent rewire path for a destination
// protocol whose engine instance is recreated (e.g. an SDK reconnect re-fires
// OnStarted). Without it, the second RegisterConsumer would fail and
// redistribution into that protocol would silently stop for the new instance.
// Returns true when an existing consumer was replaced (informational; callers
// may log the rewire).
func ReregisterConsumer(c RedistConsumer) (replaced bool) {
	consumerMu.Lock()
	defer consumerMu.Unlock()
	name := c.Name()
	_, replaced = consumers[name]
	consumers[name] = c
	if replaced {
		slog.Debug("redistribute consumer re-registered", "name", name)
	} else {
		slog.Debug("redistribute consumer registered", "name", name)
	}
	return replaced
}

// LookupConsumer returns the consumer for the given destination protocol name.
func LookupConsumer(name string) (RedistConsumer, bool) {
	consumerMu.RLock()
	defer consumerMu.RUnlock()
	c, ok := consumers[name]
	return c, ok
}

// ResetConsumersForTest clears the consumer registry. Test use only.
func ResetConsumersForTest() {
	consumerMu.Lock()
	defer consumerMu.Unlock()
	consumers = map[string]RedistConsumer{}
}

// ConsumerNames returns all registered consumer names in sorted order.
func ConsumerNames() []string {
	consumerMu.RLock()
	defer consumerMu.RUnlock()
	names := make([]string, 0, len(consumers))
	for n := range consumers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
