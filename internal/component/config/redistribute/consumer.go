// Design: docs/architecture/core-design.md -- redistribution consumer registry

package redistribute

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
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
	// Peer is the single peer selector for a targeted inject, used by the BGP
	// consumer's replay-on-peer-up path: the orchestrator sets it to the address
	// of the newly-established peer so a replayed route reaches only that peer.
	// Empty means the normal fan-out to all peers ("*"). Consumers that inject
	// into a flooded/synchronized DB (OSPF/ISIS) ignore it. Additive: it does
	// not change the RedistConsumer interface signature.
	Peer string
	// OriginASN, when nonzero, is the origin AS the route carries as a
	// single-ASN AS_PATH. The BGP consumer emits `origin igp origin-as
	// <OriginASN>`; consumers that inject into a link-state DB ignore it.
	// Additive: it does not change the RedistConsumer interface signature.
	OriginASN uint32
	// Community, when non-nil, is the standard BGP community list (each packed
	// asn<<16|value) the route carries. The BGP consumer emits
	// `community [ ... ]`; other consumers ignore it. Additive.
	Community []uint32
}

var (
	consumerMu sync.RWMutex
	consumers  = map[string]RedistConsumer{}
	// observer is notified after a consumer becomes visible in the registry.
	// One slot, replaced rather than appended. The redistribute orchestrator is
	// the one dispatcher, and an SDK reconnect re-runs its startup, so
	// appending would leave a handler for a dead engine behind.
	observer atomic.Pointer[func(string)]
)

// SetConsumerObserver installs the function notified after a consumer becomes
// visible in this registry, and replaces any function installed before.
//
// It exists because a consumer that registers after a producer emitted holds
// nothing. The dispatcher reads this registry live at event time, so a batch
// emitted one moment earlier reached every consumer except this one. No stage
// of the chain can tell that from a batch nobody wanted. The observer is what
// lets the dispatcher ask the producers to say it again.
//
// The registry stays protocol-agnostic: it hands out a name, and what a replay
// means is the dispatcher's to decide.
//
// The observer runs on the registering goroutine, AFTER the write and OUTSIDE
// the registry lock, so it MAY call back into LookupConsumer and ConsumerNames.
// A nil fn clears it.
func SetConsumerObserver(fn func(name string)) {
	if fn == nil {
		observer.Store(nil)
		return
	}
	observer.Store(&fn)
}

// announceConsumer runs the observer for a name that has just become visible.
// The caller MUST NOT hold consumerMu: the observer reads this registry back.
func announceConsumer(name string) {
	fn := observer.Load()
	if fn == nil {
		return
	}
	(*fn)(name)
}

// RegisterConsumer adds a redistribution consumer to the registry.
// Re-registration with the same name is rejected.
func RegisterConsumer(c RedistConsumer) error {
	consumerMu.Lock()
	name := c.Name()
	if _, ok := consumers[name]; ok {
		consumerMu.Unlock()
		return fmt.Errorf("%w: %q", ErrConsumerConflict, name)
	}
	consumers[name] = c
	consumerMu.Unlock()

	slog.Debug("redistribute consumer registered", "name", name)
	announceConsumer(name)
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
	name := c.Name()
	_, replaced = consumers[name]
	consumers[name] = c
	consumerMu.Unlock()

	if replaced {
		slog.Debug("redistribute consumer re-registered", "name", name)
	} else {
		slog.Debug("redistribute consumer registered", "name", name)
	}
	announceConsumer(name)
	return replaced
}

// LookupConsumer returns the consumer for the given destination protocol name.
func LookupConsumer(name string) (RedistConsumer, bool) {
	consumerMu.RLock()
	defer consumerMu.RUnlock()
	c, ok := consumers[name]
	return c, ok
}

// ResetConsumersForTest clears the consumer registry and its observer. Test use
// only. The observer is cleared with the map because a test that installed one
// would otherwise see the next test's registrations.
func ResetConsumersForTest() {
	consumerMu.Lock()
	consumers = map[string]RedistConsumer{}
	consumerMu.Unlock()
	observer.Store(nil)
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
