// Design: docs/architecture/core-design.md -- redistribute orchestrator
// Related: redistribute.go -- handleBatch records and consults the held set
// Related: replay.go -- a consumer replay records what the new consumer holds
//
// The held set answers one question the import filter cannot: does this
// consumer hold the route a rejected Add would replace, or a rejected Remove
// withdraws?

package redistribute

import (
	"context"
	"net/netip"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
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
//
// Each entry keeps the route view the import filter accepted, so a reload can
// judge it again under the new rules (withdrawRejectedHeld).
//
// generation counts the evaluators install has published. A batch reads the
// evaluator and its generation together (evaluator), judges and dispatches
// with the lock released, then settles the Add here (settleAdd). When the
// generation moved in between, a reload installed other rules after the batch
// judged the route, possibly after the reload's withdraw scan had already run,
// so settleAdd judges the route again under the installed rules.
type heldRoutes struct {
	mu         sync.Mutex
	set        map[heldKey]configredist.RedistRoute
	generation uint64
}

// consumerHeld is the orchestrator's one held set.
var consumerHeld = &heldRoutes{set: make(map[heldKey]configredist.RedistRoute)}

// settlement is what settleAdd tells the dispatcher to send after it recorded
// an Add. Zero is never an answer, so a caller that forgot to read it cannot
// pass for one that was told to send nothing.
type settlement uint8

const (
	// settlementUnspecified is the Go zero value and names no outcome.
	settlementUnspecified settlement = iota
	// settlementSettled: the consumer already holds what the installed rules
	// decide, and nothing more is sent.
	settlementSettled
	// settlementResend: a reload moved the generation and the installed rules
	// accept the route, so it is sent again with the sets they apply.
	settlementResend
	// settlementWithdraw: the consumer was sent, or holds, a route the
	// installed rules reject, so it is withdrawn.
	settlementWithdraw
)

// install publishes ev as the evaluator every batch reads, and moves the
// generation in the same critical section, so a batch that read the previous
// evaluator is always seen as stale by settleAdd.
func (h *heldRoutes) install(ev *configredist.Evaluator) {
	h.mu.Lock()
	defer h.mu.Unlock()
	configredist.SetGlobal(ev)
	h.generation++
}

// evaluator answers the installed evaluator and its generation, read together.
// A batch judges every entry with this evaluator and hands the generation to
// settleAdd.
func (h *heldRoutes) evaluator() (*configredist.Evaluator, uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return configredist.Global(), h.generation
}

// settleAdd records an Add the evaluator of generation gen judged for the
// consumer named in k, accepted or not, and answers what the consumer must now
// be sent. When gen is still the installed generation the verdict stands: an
// accepted route is held, and a rejected one releases what the consumer held,
// which is withdrawn. That is BGP import-policy semantics: a source that
// changes a route announces it again with no Remove first, and a replacement
// the filter refuses still replaces, so the consumer is left holding nothing.
// A rejected Add for a route the consumer never held sends nothing.
// When a reload installed other rules since, the route is judged again under
// them: accepted, it is held and sent again with their sets; rejected, it is
// withdrawn if the consumer was sent it or held it.
//
// The re-judgement runs under the same lock install takes, so no third
// evaluator can slip between the judgement and the record.
func (h *heldRoutes) settleAdd(k heldKey, route *configredist.RedistRoute, accepted bool, gen uint64) (settlement, ribpolicy.Sets) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var sets ribpolicy.Sets
	sent := accepted
	moved := gen != h.generation
	if moved {
		accepted = false
		if ev := configredist.Global(); ev != nil {
			sets, accepted = ev.Accept(route, k.consumer)
		}
	}
	if accepted {
		h.set[k] = *route
		if moved {
			return settlementResend, sets
		}
		return settlementSettled, sets
	}
	_, held := h.set[k]
	delete(h.set, k)
	if held {
		return settlementWithdraw, sets
	}
	if sent {
		return settlementWithdraw, sets
	}
	return settlementSettled, sets
}

// removeReplacedRoute handles a Remove the import filter rejected, and
// withdraws the route when the consumer holds it. A rejected Remove for a held
// route is still the source withdrawing it: the filter judges what may enter,
// never what must leave, so the consumer MUST NOT keep a route its source
// withdrew. A rejected Remove for a route the consumer never held is dropped,
// because a withdraw for a prefix never announced is noise every BGP peer would
// receive. A rejected Add is settleAdd's, which applies the same semantics to a
// replacement the filter refused.
func removeReplacedRoute(ctx context.Context, consumer configredist.RedistConsumer, key heldKey) {
	if !consumerHeld.release(key) {
		return
	}
	logger().Debug(Name+": rejected remove withdraws the held route", "source", key.source, "consumer", key.consumer, "family", key.family.String(), "prefix", key.prefix)
	withdrawHeld(ctx, consumer, key)
}

// withdrawHeld sends consumer the withdraw of key's prefix, through the
// dispatch an accepted Remove takes, so it reaches the consumer and
// ze_bgp_redistribute_withdrawals the same way.
func withdrawHeld(ctx context.Context, consumer configredist.RedistConsumer, key heldKey) {
	withdraw := redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: key.prefix}
	dispatchEntryToConsumer(ctx, consumer, key.family, key.source, "", 0, nil, &withdraw, nil, ribpolicy.Sets{})
}

// rejected answers the held routes ev no longer accepts for their consumer. A
// nil ev is a reload or a rollback that leaves no rules at all, which accepts
// nothing.
func (h *heldRoutes) rejected(ev *configredist.Evaluator) []heldKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []heldKey
	for k, route := range h.set {
		if ev == nil {
			out = append(out, k)
			continue
		}
		if _, ok := ev.Accept(&route, k.consumer); !ok {
			out = append(out, k)
		}
	}
	return out
}

// withdrawRejectedHeld withdraws every held route the reloaded rules reject,
// once, through the dispatch an accepted Remove takes. A consumer that is no
// longer registered holds nothing to withdraw, so its routes are only
// forgotten. Called with the lock released: a dispatch reaches the consumer.
func withdrawRejectedHeld(ctx context.Context, ev *configredist.Evaluator) {
	for _, key := range consumerHeld.rejected(ev) {
		if !consumerHeld.release(key) {
			continue
		}
		consumer, ok := configredist.LookupConsumer(key.consumer)
		if !ok {
			continue
		}
		logger().Debug(Name+": reloaded rules withdraw the held route", "source", key.source, "consumer", key.consumer, "family", key.family.String(), "prefix", key.prefix)
		withdrawHeld(ctx, consumer, key)
	}
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
