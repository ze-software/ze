// Design: docs/architecture/core-design.md -- redistribute orchestrator
// Related: redistribute.go -- handleBatch records and consults the held set
// Related: replay.go -- a consumer replay records what the new consumer holds
//
// The held set answers one question the import filter cannot: does this
// consumer hold the route a rejected Add would replace, or a rejected Remove
// withdraws?

package redistributeegress

import (
	"context"
	"net/netip"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// heldKey names one route a consumer holds because the orchestrator dispatched
// an Add for it. The consumer is part of the key because one source's route is
// accepted by one consumer and rejected by another. The source is part of the
// key because two sources can feed the same prefix to one consumer: a rejected
// entry from a source the consumer never held that prefix from withdraws
// nothing. Where the consumer holds the prefix from both, the withdraw names the
// prefix alone (RedistConsumer.WithdrawRoute carries no source), so it removes
// the prefix exactly as an accepted Remove from that source does.
type heldKey struct {
	consumer string
	source   string
	family   family.Family
	prefix   netip.Prefix
}

// heldRoutes records which routes each consumer holds, so that a replacement Add
// the import filter rejects can remove the route it replaces. A source that
// changes a route's attributes announces it again with no Remove first (an
// implicit replace), and the filter judges the new announcement alone. Under BGP
// import-policy semantics the replacement still replaces: when the filter
// rejects it, the consumer is left holding nothing for that prefix.
//
// Bound: one entry for each accepted Add not yet withdrawn, so the set never
// exceeds the routes the producers announce times the registered consumers.
// Safe for concurrent use: every producer's batches arrive on its own bus handler.
type heldRoutes struct {
	mu  sync.Mutex
	set map[heldKey]struct{}
}

// consumerHeld is the orchestrator's one held set.
var consumerHeld = &heldRoutes{set: make(map[heldKey]struct{})}

// recordDispatched updates the held set after an entry was dispatched to the
// consumer named in key. An entry dispatchEntryToConsumer skipped for an invalid
// prefix reached no consumer, so it changes nothing here either.
func recordDispatched(key heldKey, action redistevents.RouteAction) {
	if !key.prefix.IsValid() {
		return
	}
	// The action comes from a producer plugin, so the set is open: an action this
	// orchestrator does not know was not dispatched and changes nothing.
	switch action {
	case redistevents.ActionAdd:
		consumerHeld.hold(key)
	case redistevents.ActionRemove:
		consumerHeld.release(key)
	case redistevents.ActionUnspecified:
	default:
	}
}

// removeReplacedRoute applies BGP import-policy semantics to an entry the
// import filter rejected, and withdraws the held route when the consumer holds
// one. A rejected Add for a held route is a replacement the filter refused: the
// replacement still replaces, so the consumer is left holding nothing. A
// rejected Remove for a held route is still the source withdrawing it: the
// filter judges what may enter, never what must leave, so the consumer MUST NOT
// keep a route its source withdrew. A rejected entry for a route the consumer
// never held is dropped, because a withdraw for a prefix never announced is
// noise every BGP peer would receive.
//
// The withdraw goes through dispatchEntryToConsumer, so it reaches the consumer
// and ze_bgp_redistribute_withdrawals exactly as an accepted Remove does.
func removeReplacedRoute(ctx context.Context, consumer configredist.RedistConsumer, key heldKey, action redistevents.RouteAction) {
	if action != redistevents.ActionAdd && action != redistevents.ActionRemove {
		return
	}
	if !consumerHeld.release(key) {
		return
	}
	logger().Debug(Name+": rejected entry withdraws the held route", "action", action, "source", key.source, "consumer", key.consumer, "family", key.family.String(), "prefix", key.prefix)
	withdraw := redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: key.prefix}
	dispatchEntryToConsumer(ctx, consumer, key.family, key.source, "", 0, nil, &withdraw)
}

// hold records that the consumer named in k now holds k's route.
func (h *heldRoutes) hold(k heldKey) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.set[k] = struct{}{}
}

// release forgets k's route and reports whether the consumer held it. A caller
// that withdraws on true withdraws a route once, however often it is released.
func (h *heldRoutes) release(k heldKey) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.set[k]; ok {
		delete(h.set, k)
		return true
	}
	return false
}

// forgetConsumer drops every route recorded for consumer. A consumer that
// registers again is a new instance holding nothing its predecessor was sent,
// so a rejected Add MUST NOT withdraw at it.
func (h *heldRoutes) forgetConsumer(consumer string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k := range h.set {
		if k.consumer == consumer {
			delete(h.set, k)
		}
	}
}
