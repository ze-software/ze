// Design: docs/architecture/core-design.md -- redistribute orchestrator
// Related: redistribute.go -- handleBatch records and consults the held set
// Related: replay.go -- a consumer replay records what the new consumer holds
// Related: withdrawal_diagnostic.go -- an unconfirmed withdrawal is reported
//
// The held set answers one question the import filter cannot: does this
// consumer hold the route a rejected Add would replace, or a rejected Remove
// withdraws? It also answers where the withdrawal goes: to the consumer
// instance (episode) that accepted the route, which keeps the route until it
// confirms the withdrawal.

package redistribute

import (
	"context"
	"net/netip"
	"slices"
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
// original source instance/table too. BGP removes that accepted contribution;
// native prefix-only adapters continue removing their prefix as before.
type heldKey struct {
	consumer string
	source   string
	family   family.Family
	prefix   netip.Prefix
	instance uint32
	table    uint32
}

// heldRoutes records which routes each consumer holds, so that a replacement Add
// the import filter rejects can remove the route it replaces. A source that
// changes a route's attributes announces it again with no Remove first (an
// implicit replace), and the filter judges the new announcement alone. Under BGP
// import-policy semantics the replacement still replaces: when the filter
// rejects it, the consumer is left holding nothing for that prefix.
//
// Every record names the consumer instance that accepted the route (its
// episode, sentRoute.episode), and a withdrawal goes to that instance, never to
// whichever consumer the registry names later. A record leaves only when its
// episode confirms the withdrawal (settleWithdrawal); a failed one stays as
// heldUnconfirmed, and the next trigger for it (a Remove of the route, a reload
// of the rules) withdraws it at the same episode again. No timer retries it.
//
// set holds the records of the consumer currently registered under each name.
// retired holds, oldest first, the records of earlier instances a re-registration
// replaced (retireEpisode): such an instance still holds what it accepted, and
// the new one holds nothing it was not sent itself.
//
// Bound: one record for each accepted Add whose withdrawal its episode has not
// confirmed, so the two maps never exceed the routes the producers announce
// times the consumer instances that accepted them in this run of the
// orchestrator: a new run starts empty (beginRun), because the episodes of the
// last one ended with it. A confirmed withdrawal leaves no history.
// Safe for concurrent use: every producer's batches arrive on its own bus handler.
// No consumer is called with mu held.
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
//
// inflight counts, for each key, the Adds dispatched (beginAdd) and not yet
// settled. The reload's withdraw scan leaves such a key to its settleAdd, which
// judges it again under the installed rules: withdrawing it from the scan as
// well would send the consumer the same withdraw twice. Bound: one entry for
// each Add between its dispatch and its settlement.
//
// receipts numbers the records, so a withdrawal's outcome settles the record it
// withdrew and never a newer one an Add stored under the same key since.
type heldRoutes struct {
	mu         sync.Mutex
	set        map[heldKey]heldRoute
	retired    map[heldKey][]heldRoute
	inflight   map[heldKey]int
	generation uint64
	receipts   uint64
}

// heldState is where one record stands in its withdrawal.
type heldState uint8

const (
	// heldStateUnspecified is the Go zero value and names no state; every
	// record the orchestrator stores is given one of the states below.
	heldStateUnspecified heldState = iota
	// heldLive: the episode holds the route and owes no withdrawal.
	heldLive
	// heldWithdrawing: a caller claimed the record and is dispatching its
	// withdrawal, so no other caller sends it again.
	heldWithdrawing
	// heldUnconfirmed: the episode answered the withdrawal with an error, so
	// the next trigger withdraws it there again.
	heldUnconfirmed
)

// heldRoute is one route a consumer holds: the view the import filter accepted,
// which a reload judges again, and what the consumer was sent with it, which a
// replay compares to decide whether the consumer already holds what it would
// send.
type heldRoute struct {
	view  configredist.RedistRoute
	sent  sentRoute
	state heldState
}

// sentRoute is what dispatchEntryToConsumer hands a consumer besides the route
// view: the values the accepting chain set, the origin AS, the communities and
// the original route-specific distance preference.
// community is owned: the producer's batch is pooled and reused.
//
// episode is the consumer instance the route was dispatched to, which the
// caller sets, and receipt the number settleAdd gave the record holding it.
type sentRoute struct {
	sets        ribpolicy.Sets
	originASN   uint32
	community   []uint32
	distance    uint8
	hasDistance bool
	generation  uint64
	episode     configredist.RedistConsumer
	receipt     uint64
	// refused is settleAdd's answer that the installed rules refused the
	// route, so the records earlier episodes hold of it are withdrawn too.
	refused bool
}

// heldClaim is one record a caller took to withdraw (heldWithdrawing): the key
// of its route, the episode that holds it, and the receipt that names the
// record. Its outcome MUST be handed to settleWithdrawal (withdrawClaim).
type heldClaim struct {
	key     heldKey
	episode configredist.RedistConsumer
	receipt uint64
}

// newSentRoute answers what a dispatch of entry from a batch carrying originASN
// and community sends with sets. The per-entry origin AS wins over the batch's,
// as dispatchEntryToConsumer decides.
func newSentRoute(sets ribpolicy.Sets, entry *redistevents.RouteChangeEntry, originASN uint32, community []uint32) sentRoute {
	if entry.OriginAS != 0 {
		originASN = entry.OriginAS
	}
	return sentRoute{sets: sets, originASN: originASN, community: slices.Clone(community), distance: entry.DistanceOverride, hasDistance: entry.HasDistanceOverride}
}

// consumerHeld is the orchestrator's one held set.
var consumerHeld = &heldRoutes{set: make(map[heldKey]heldRoute), retired: make(map[heldKey][]heldRoute), inflight: make(map[heldKey]int)}

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
	// installed rules reject, so it is withdrawn. settleAdd claimed the record,
	// and sent names its episode and receipt.
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

// beginAdd records that an Add of k is about to be dispatched, and answers
// whether it must be. A consumer replay passes onlyChanged: when the consumer
// already holds k with the same view and was sent it with the same values, the
// replay sends nothing, so a reload that changes nothing for a route sends its
// consumer nothing for it: the BGP consumer turns every InjectRoute into an
// update-route command to every peer (BGPConsumer.InjectRoute), however often
// it was sent before. A true answer MUST be followed by
// settleAdd with dispatched set, which ends the in-flight record.
func (h *heldRoutes) beginAdd(k heldKey, view *configredist.RedistRoute, sent *sentRoute, onlyChanged bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if onlyChanged {
		if held, ok := h.set[k]; ok && held.matches(view, sent) {
			return false
		}
	}
	h.inflight[k]++
	return true
}

// matches answers whether the episode holding r still holds it as live and was
// sent exactly view with sent. A record whose withdrawal is under way or
// unconfirmed is no longer held as sent.
func (r *heldRoute) matches(view *configredist.RedistRoute, sent *sentRoute) bool {
	if r.state == heldWithdrawing {
		return false
	}
	if r.state == heldUnconfirmed {
		return false
	}
	if r.sent.episode != sent.episode {
		return false
	}
	if r.view != *view {
		return false
	}
	if r.sent.sets != sent.sets {
		return false
	}
	if r.sent.distance != sent.distance {
		return false
	}
	if r.sent.hasDistance != sent.hasDistance {
		return false
	}
	if r.sent.originASN != sent.originASN {
		return false
	}
	return slices.Equal(r.sent.community, sent.community)
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
//
// dispatched says the batch sent this Add (beginAdd answered true) to
// sent.episode; a consumer replay that found the route unchanged, or a
// rejected Add, sent nothing. sent is what a dispatch carries besides the view.
// On settlementWithdraw the record is claimed, and sent.episode and
// sent.receipt name it: the caller MUST withdraw it there (withdrawClaim).
func (h *heldRoutes) settleAdd(k heldKey, route *configredist.RedistRoute, sent *sentRoute, dispatched, accepted bool, gen uint64) (settlement, ribpolicy.Sets) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if dispatched {
		h.endInflightLocked(k)
	}
	sets := sent.sets
	sent.generation = h.generation
	moved := gen != h.generation
	if moved {
		accepted = false
		sets = ribpolicy.Sets{}
		if ev := configredist.Global(); ev != nil {
			sets, accepted = ev.Accept(route, k.consumer)
		}
	}
	sent.refused = !accepted
	if accepted {
		record := *sent
		record.sets = sets
		h.storeLocked(k, route, &record, heldLive)
		if moved {
			return settlementResend, sets
		}
		return settlementSettled, sets
	}
	if dispatched {
		// The batch has just handed sent.episode this Add, so that episode
		// holds it, whatever the set held before, and the withdraw goes there.
		record := *sent
		h.storeLocked(k, route, &record, heldWithdrawing)
		sent.receipt = record.receipt
		return settlementWithdraw, sets
	}
	held, ok := h.set[k]
	if !ok {
		return settlementSettled, sets
	}
	if held.state == heldWithdrawing {
		// Another caller is withdrawing it already; its outcome settles it.
		return settlementSettled, sets
	}
	held.state = heldWithdrawing
	h.set[k] = held
	sent.episode = held.sent.episode
	sent.receipt = held.sent.receipt
	return settlementWithdraw, sets
}

// storeLocked records route as held under k in state, with a new receipt that
// it writes into record. A record another episode holds under k is retired
// first, never overwritten: that episode still holds its version, and only
// its own withdrawal releases it. state MUST name a withdrawal state: storeLocked
// is the one place a record is given one, and its callers pass a constant, so
// heldStateUnspecified reaching it is a Ze defect. Caller holds h.mu.
func (h *heldRoutes) storeLocked(k heldKey, route *configredist.RedistRoute, record *sentRoute, state heldState) {
	if state == heldStateUnspecified {
		panic("BUG: held redistribution route stored with no withdrawal state")
	}
	if previous, ok := h.set[k]; ok {
		if previous.sent.episode != record.episode {
			h.retired[k] = append(h.retired[k], previous)
		}
	}
	h.receipts++
	record.receipt = h.receipts
	h.set[k] = heldRoute{view: *route, sent: *record, state: state}
}

// endInflightLocked ends one in-flight record of k. Caller holds h.mu.
func (h *heldRoutes) endInflightLocked(k heldKey) {
	n := h.inflight[k] - 1
	if n > 0 {
		h.inflight[k] = n
		return
	}
	delete(h.inflight, k)
}

// claimLocked marks r withdrawing and answers its claim, or false when another
// caller is withdrawing it already. Caller holds h.mu.
func claimLocked(k heldKey, r *heldRoute) (heldClaim, bool) {
	if r.state == heldWithdrawing {
		return heldClaim{}, false
	}
	r.state = heldWithdrawing
	return heldClaim{key: k, episode: r.sent.episode, receipt: r.sent.receipt}, true
}

// claimRemove claims every record of k a Remove of its route withdraws: those
// of earlier episodes first, oldest first, then the current one. held answers
// whether any episode holds the route at all, claimed here or being withdrawn
// by another caller.
func (h *heldRoutes) claimRemove(k heldKey) (claims []heldClaim, held bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.retired[k]
	for i := range list {
		held = true
		if claim, claimed := claimLocked(k, &list[i]); claimed {
			claims = append(claims, claim)
		}
	}
	if r, ok := h.set[k]; ok {
		held = true
		if claim, claimed := claimLocked(k, &r); claimed {
			h.set[k] = r
			claims = append(claims, claim)
		}
	}
	return claims, held
}

// claimRetired claims every record of k that earlier episodes hold, oldest
// first: the route's source replaced it with a version the rules refuse, and a
// refused replacement still replaces at every episode holding the old one.
func (h *heldRoutes) claimRetired(k heldKey) []heldClaim {
	h.mu.Lock()
	defer h.mu.Unlock()
	var claims []heldClaim
	list := h.retired[k]
	for i := range list {
		if claim, claimed := claimLocked(k, &list[i]); claimed {
			claims = append(claims, claim)
		}
	}
	return claims
}

// beginRun empties the held set for a new run of the orchestrator. The
// episodes a previous run dispatched to ended with it: the BGP consumer was
// built on that run's engine connection, and the engine retires the
// contributions of an accepting Process that ends (process-protocol.md,
// "Accepted redistribution at BGP input"). A route the new run never held
// still reaches the consumer registered now on its Remove (withdrawAccepted).
// The evaluator generation is kept, so it never moves backwards.
func (h *heldRoutes) beginRun() {
	h.mu.Lock()
	defer h.mu.Unlock()
	clear(h.set)
	clear(h.retired)
	clear(h.inflight)
}

// settleWithdrawal settles the record claim names with the outcome its episode
// answered: nil releases it, an error keeps it unconfirmed for the next
// trigger. A record an Add replaced since the claim is the newer record's, and
// nothing is settled.
func (h *heldRoutes) settleWithdrawal(claim heldClaim, outcome error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.set[claim.key]; ok {
		if r.sent.receipt == claim.receipt {
			if outcome == nil {
				delete(h.set, claim.key)
				return
			}
			r.state = heldUnconfirmed
			h.set[claim.key] = r
			return
		}
	}
	list := h.retired[claim.key]
	for i := range list {
		if list[i].sent.receipt != claim.receipt {
			continue
		}
		if outcome != nil {
			list[i].state = heldUnconfirmed
			return
		}
		list = slices.Delete(list, i, i+1)
		if len(list) == 0 {
			delete(h.retired, claim.key)
			return
		}
		h.retired[claim.key] = list
		return
	}
}

// retireEpisode moves every record the consumer named consumer holds from an
// instance other than current into the retired records, because current is a
// new instance holding nothing its predecessor was sent: a rejected Add MUST
// NOT withdraw at it what it never held. The predecessor still holds what it
// accepted, so a Remove of those routes still reaches it. Consumers are
// registered as pointers, so the comparison is instance identity.
func (h *heldRoutes) retireEpisode(consumer string, current configredist.RedistConsumer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k := range h.set {
		if k.consumer != consumer {
			continue
		}
		if h.set[k].sent.episode == current {
			continue
		}
		h.retired[k] = append(h.retired[k], h.set[k])
		delete(h.set, k)
	}
}

// removeReplacedRoute handles a Remove the import filter rejected, and
// withdraws the route when the consumer holds it. A rejected Remove for a held
// route is still the source withdrawing it: the filter judges what may enter,
// never what must leave, so the consumer MUST NOT keep a route its source
// withdrew. A rejected Remove for a route the consumer never held is dropped,
// because a withdraw for a prefix never announced is noise every BGP peer would
// receive. A rejected Add is settleAdd's, which applies the same semantics to a
// replacement the filter refused. Each withdraw goes to the episode holding the
// route (claimRemove).
func removeReplacedRoute(ctx context.Context, key heldKey) {
	claims, _ := consumerHeld.claimRemove(key)
	for _, claim := range claims {
		logger().Debug(Name+": rejected remove withdraws the held route", "source", key.source, "consumer", key.consumer, "family", key.family.String(), "prefix", key.prefix)
		withdrawClaim(ctx, claim)
	}
}

// withdrawClaim sends the episode of claim the withdraw of its route, through
// the dispatch an accepted Remove takes, so it reaches the consumer and
// ze_bgp_redistribute_withdrawals the same way, then settles the record with
// the episode's answer and reports an unconfirmed one. Called with the held
// set's lock released: the dispatch reaches the consumer and the report a queue.
func withdrawClaim(ctx context.Context, claim heldClaim) {
	if claim.episode == nil {
		panic("BUG: redistribute: a held route records no accepting consumer")
	}
	key := claim.key
	withdraw := redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: key.prefix, SourceInstance: key.instance, Table: key.table}
	err := dispatchEntryToConsumer(ctx, claim.episode, key.family, key.source, "", 0, nil, &withdraw, nil, ribpolicy.Sets{}, 0)
	consumerHeld.settleWithdrawal(claim, err)
	if err != nil {
		reportUnconfirmedWithdrawal(ctx, key, err)
	}
}

// owesWithdrawLocked answers whether the record r of k is to be withdrawn under
// ev: one whose withdrawal is unconfirmed always is, one being withdrawn is
// another caller's, and a live one is when ev rejects it. A nil ev is a reload
// or a rollback that leaves no rules at all, which accepts nothing. Caller
// holds h.mu.
func owesWithdrawLocked(ev *configredist.Evaluator, k heldKey, r *heldRoute) bool {
	if r.state == heldWithdrawing {
		return false
	}
	if r.state == heldUnconfirmed {
		return true
	}
	if ev == nil {
		return true
	}
	_, ok := ev.Accept(&r.view, k.consumer)
	return !ok
}

// rejected answers the keys of the held routes ev no longer accepts for their
// consumer, or whose withdrawal is unconfirmed, at any episode.
func (h *heldRoutes) rejected(ev *configredist.Evaluator) []heldKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []heldKey
	for k := range h.set {
		r := h.set[k]
		if owesWithdrawLocked(ev, k, &r) {
			out = append(out, k)
		}
	}
	for k, list := range h.retired {
		if r, ok := h.set[k]; ok {
			if owesWithdrawLocked(ev, k, &r) {
				continue // listed above
			}
		}
		for i := range list {
			if owesWithdrawLocked(ev, k, &list[i]) {
				out = append(out, k)
				break
			}
		}
	}
	return out
}

// claimRejected claims the records of k that, checked under the lock, still
// owe their withdrawal under ev (owesWithdrawLocked): those of earlier
// episodes first, then the current one when no Add of k is in flight. A batch
// can settle a newer, accepted version of k after the scan listed it, and an
// in-flight Add is judged again by its own settleAdd, so either one leaves the
// current record alone.
func (h *heldRoutes) claimRejected(ev *configredist.Evaluator, k heldKey) []heldClaim {
	h.mu.Lock()
	defer h.mu.Unlock()
	var claims []heldClaim
	list := h.retired[k]
	for i := range list {
		if !owesWithdrawLocked(ev, k, &list[i]) {
			continue
		}
		if claim, claimed := claimLocked(k, &list[i]); claimed {
			claims = append(claims, claim)
		}
	}
	r, ok := h.set[k]
	if !ok {
		return claims
	}
	if h.inflight[k] > 0 {
		return claims
	}
	if !owesWithdrawLocked(ev, k, &r) {
		return claims
	}
	if claim, claimed := claimLocked(k, &r); claimed {
		h.set[k] = r
		claims = append(claims, claim)
	}
	return claims
}

// withdrawRejectedHeld withdraws every held route the reloaded rules reject,
// once, through the dispatch an accepted Remove takes, at the episode that
// holds it, and withdraws again every route whose earlier withdrawal its
// episode did not confirm. Called with the lock released: a dispatch reaches
// the consumer.
func withdrawRejectedHeld(ctx context.Context, ev *configredist.Evaluator) {
	withdrawListed(ctx, ev, consumerHeld.rejected(ev))
}

// withdrawListed withdraws each listed key that ev still rejects when it comes
// to it (claimRejected): the list is a snapshot, and a batch may have settled
// a newer version of a key since it was taken.
func withdrawListed(ctx context.Context, ev *configredist.Evaluator, keys []heldKey) {
	for _, key := range keys {
		for _, claim := range consumerHeld.claimRejected(ev, key) {
			logger().Debug(Name+": reloaded rules withdraw the held route", "source", key.source, "consumer", key.consumer, "family", key.family.String(), "prefix", key.prefix)
			withdrawClaim(ctx, claim)
		}
	}
}
