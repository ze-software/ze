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
