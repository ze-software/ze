// Design: docs/architecture/core-design.md -- redistribute replay-on-request
//
// This file implements the two triggers of the late-join replay and the
// targeting of each. A producer emits once, and whoever was not listening at
// that moment holds nothing, so each trigger names a party that arrived late.
// A reload of the import rules reuses the consumer trigger for every consumer,
// because each holds only what the replaced rules let through:
//
//	BGP peer down->up edge      --> replayCoordinator.onPeerUp
//	consumer becomes registered --> replayCoordinator.onConsumerRegistered
//	import rules reloaded       --> replayConsumers -> onConsumerRegistered
//	   -> allocate monotonic replayID, record replayID -> target (bounded + TTL)
//	   -> emit redistevents.ReplayRequest{replayID}
//	producers re-emit RouteChangeBatch{ReplayID: replayID}
//	   -> orchestrator handleBatch sees IsReplay() -> handleReplayBatch
//	   -> lookupTarget(replayID) -> inject to that ONE target
//
// A peer target injects through the BGP consumer, to that peer alone. A
// consumer target injects through that consumer, with the all-peers fan-out an
// incremental batch takes.
//
// The producer never learns the target. The coordinator alone holds the
// replayID -> target mapping, so the returning batch stays target-agnostic.

package redistribute

import (
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// bgpDestination is the destination protocol name the PEER-UP replay targets.
// OSPF/ISIS redistribute consumers originate into a flooded link-state DB, and
// a new adjacency receives that DB by database exchange. A new neighbor
// therefore opens no gap for them. Only the BGP consumer sends per-peer, and
// only it needs a replay when a peer establishes.
//
// A consumer REGISTERING late is the other gap, and it is every destination's:
// see onConsumerRegistered, which targets the consumer rather than this name.
const bgpDestination = "bgp"

// replayTTL bounds how long a replayID -> peer mapping is held after the request
// is emitted. It must outlast the slowest producer's re-emit: engine
// (in-process) producers deliver synchronously, but plugin-process producers
// deliver asynchronously (see internal/core/events/typed.go), so dropping the
// mapping right after Emit would discard their late batch. Held for a generous
// window instead; the map is TTL-evicted and hard-capped so peer flapping
// cannot grow it without bound (R-6).
const replayTTL = 30 * time.Second

// replayMapMaxSize hard-caps the pending replayID -> peer map so a peer flapping
// faster than replayTTL cannot exhaust memory. On overflow the oldest entries
// (lowest replayID) are dropped.
const replayMapMaxSize = 4096

// replayKind names who one replay request was fired for. Zero is never a valid
// target, so an entry nobody filled in cannot be mistaken for a peer.
type replayKind uint8

const (
	// replayKindUnspecified is the Go zero value and names no target.
	replayKindUnspecified replayKind = iota
	// replayKindPeer targets the one BGP peer whose session just established.
	// The returning batch is injected through the BGP consumer, to that peer.
	replayKindPeer
	// replayKindConsumer targets the one destination protocol whose consumer
	// just registered. The returning batch is injected through that consumer,
	// with the ordinary all-peers fan-out.
	replayKindConsumer
)

// replayEntry records who a replay request targets and when the mapping
// expires. name is a peer address for replayKindPeer and a consumer name for
// replayKindConsumer; kind is what says which, and no reader guesses from the
// value.
type replayEntry struct {
	kind     replayKind
	name     string
	deadline time.Time
}

// replayCoordinator correlates a late-join event to the redistribute replay
// batches it produces. On a peer's down->up edge, and on a consumer becoming
// registered, it allocates a monotonic replayID, records replayID -> target,
// and emits ReplayRequest{replayID}.
//
// Producers re-emit their current set tagged with the echoed replayID. The
// orchestrator's batch handler maps the ID back to the target and reaches only
// that target. Distinct replayIDs per trigger mean concurrent replays never
// cross-deliver (R-2).
//
// removed is the orchestrator's half of the guarantee that a replay never
// revives a removed route. A producer snapshots its set to answer a request,
// and a Remove of a route in that snapshot can reach the orchestrator before
// the replay batch carrying the route's Add. removed holds, for each route
// whose most recent incremental action was a Remove handled while a replay
// was pending, the newest replayID minted when that Remove was handled. A
// replay Add for the route is dropped when its replayID is not newer
// (removedSince). An incremental Add of the route clears its entry. Bound:
// the routes removed within replayTTL of the last request; once every pending
// mapping has expired the map is cleared, since no replay can return for it.
type replayCoordinator struct {
	mu           sync.Mutex
	gen          uint64
	pending      map[uint64]replayEntry
	removed      map[sourceRoute]uint64
	lastDeadline time.Time
	peerUp       map[string]bool
	ttl          time.Duration
	maxSize      int
	// nowFn is injectable so tests can drive TTL eviction deterministically.
	nowFn func() time.Time
}

// sourceRoute names one route a producer announces, whatever consumer holds it.
type sourceRoute struct {
	source string
	family family.Family
	prefix netip.Prefix
}

func newReplayCoordinator() *replayCoordinator {
	return &replayCoordinator{
		pending: make(map[uint64]replayEntry),
		removed: make(map[sourceRoute]uint64),
		peerUp:  make(map[string]bool),
		ttl:     replayTTL,
		maxSize: replayMapMaxSize,
		nowFn:   time.Now,
	}
}

// replayCoordPtr is the package-level coordinator singleton, set once in
// runPlugin. handleReplayBatch reads it to correlate a returning batch to its
// target peer. Nil when the plugin is not running (unit tests for the
// incremental path leave it nil and never reach the replay branch).
var replayCoordPtr atomic.Pointer[replayCoordinator]

func setReplayCoordinator(c *replayCoordinator) { replayCoordPtr.Store(c) }

func getReplayCoordinator() *replayCoordinator { return replayCoordPtr.Load() }

// onStateUp routes a peer-up event. An up carrying reason rpc.ReasonAttached
// is a reload attaching this plugin to a session already established: the
// peer's initial routing update was sent without this plugin, so it is not this
// plugin's to replay. The peer is recorded up, so the next down then up is an
// edge again, and no replay fires. Every other up goes to onPeerUp. It answers
// whether a replay fired; no caller reads the replay id onPeerUp also returns.
func (c *replayCoordinator) onStateUp(bus ze.EventBus, peer, reason string) bool {
	if reason == rpc.ReasonAttached {
		c.markPeerUp(peer)
		return false
	}
	_, fired := c.onPeerUp(bus, peer)
	return fired
}

// markPeerUp records the peer up without firing a replay.
func (c *replayCoordinator) markPeerUp(peer string) {
	if c == nil {
		return
	}
	if peer == "" {
		return
	}
	c.mu.Lock()
	c.peerUp[peer] = true
	c.mu.Unlock()
}

// onPeerUp handles a BGP peer establishment. On the down->up edge it allocates a
// replayID, records replayID -> peer, and emits ReplayRequest{replayID} so
// producers re-emit their current set for this peer. Returns (replayID, true)
// when a request was fired; (0, false) when the peer was already up (no edge),
// there is no configured evaluator, or no import feeds BGP. The evaluator gate
// avoids a pointless broadcast when redistribute has no BGP destination.
func (c *replayCoordinator) onPeerUp(bus ze.EventBus, peer string) (uint64, bool) {
	if c == nil {
		return 0, false
	}
	if bus == nil {
		return 0, false
	}
	if peer == "" {
		return 0, false
	}

	c.mu.Lock()
	if c.peerUp[peer] {
		c.mu.Unlock()
		return 0, false // not a down->up edge; already-up peers already have the routes
	}
	c.peerUp[peer] = true
	c.mu.Unlock()

	// Cheap gate: only fire when an import feeds BGP, so a deployment with no
	// bgp redistribute destination does not storm producers on every peer-up.
	ev := configredist.Global()
	if ev == nil {
		return 0, false
	}
	if !ev.HasDestination(bgpDestination) {
		return 0, false
	}

	return c.fire(bus, replayKindPeer, peer)
}

// onConsumerRegistered handles a destination protocol's consumer becoming
// visible in the registry. It allocates a replayID, records it against that
// consumer, and emits ReplayRequest{replayID} so every producer re-emits its
// current set for it. Returns (replayID, true) when a request was fired;
// (0, false) when there is no configured evaluator or no import feeds this
// destination.
//
// This is what makes a late consumer not an empty consumer. The dispatcher
// reads the consumer registry live at event time (handleBatch). A producer that
// emitted before this consumer registered reached every consumer except it, and
// the route was lost with no line in any log.
//
// Startup order decides which case a deployment lands in. The static plugin and
// the IS-IS plugin start in the same tier, and nothing orders them.
//
// A reload of the import rules takes the same path for every registered
// consumer (reconcileConsumers, config.go): the consumer holds what the old
// rules let through, and the replay is how it receives what the new ones do.
//
// The evaluator gate is R-2. A consumer no rule imports into gains nothing from
// a replay, and firing anyway would storm every producer once per consumer on
// every startup.
//
//nolint:unparam // the replayID pairs with onPeerUp's, whose tests read both; this trigger has no test of its own yet.
func (c *replayCoordinator) onConsumerRegistered(bus ze.EventBus, consumer string) (uint64, bool) {
	if c == nil {
		return 0, false
	}
	if bus == nil {
		return 0, false
	}
	if consumer == "" {
		return 0, false
	}
	ev := configredist.Global()
	if ev == nil {
		return 0, false
	}
	if !ev.HasDestination(consumer) {
		return 0, false
	}
	return c.fire(bus, replayKindConsumer, consumer)
}

// fire allocates a replayID for one target, records the mapping, and broadcasts
// the request. It is the one place a replayID is minted, so both triggers share
// the eviction, the TTL and the hard cap.
func (c *replayCoordinator) fire(bus ze.EventBus, kind replayKind, name string) (uint64, bool) {
	c.mu.Lock()
	c.evictLocked()
	c.gen++
	id := c.gen
	deadline := c.nowFn().Add(c.ttl)
	c.pending[id] = replayEntry{kind: kind, name: name, deadline: deadline}
	c.lastDeadline = deadline
	c.mu.Unlock()

	// Emit OUTSIDE the lock: engine subscribers deliver synchronously and the
	// producer re-emit re-enters handleReplayBatch -> lookupTarget, which locks
	// c.mu. Holding the lock here would deadlock that nested dispatch.
	if _, err := redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: id}); err != nil {
		logger().Warn("redistribute-orchestrator: replay-request emit failed", "target", name, "replay-id", id, "error", err)
		// refuse-by-default: guards lookupTarget -- the mapping is recorded before the emit, so a producer that did receive the request is still answered
		return id, true
	}
	logger().Debug("redistribute-orchestrator: replay request fired", "kind", kind, "target", name, "replay-id", id)
	return id, true
}

// onPeerDown clears the up state for a peer so a later re-establishment is seen
// as a fresh down->up edge and fires a new replay.
func (c *replayCoordinator) onPeerDown(peer string) {
	if c == nil {
		return
	}
	if peer == "" {
		return
	}
	c.mu.Lock()
	delete(c.peerUp, peer)
	c.mu.Unlock()
}

// lookupTarget returns who a replayID targets, or a zero entry and false when
// the ID is unknown or its mapping has expired. It does NOT evict on lookup:
// several producers each re-emit for the same replayID, so the mapping must
// survive until the TTL, not until the first returning batch.
func (c *replayCoordinator) lookupTarget(replayID uint64) (replayEntry, bool) {
	if c == nil {
		return replayEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.pending[replayID]
	if !ok {
		return replayEntry{}, false
	}
	if c.nowFn().After(e.deadline) {
		delete(c.pending, replayID)
		return replayEntry{}, false
	}
	return e, true
}

// noteIncremental records the incremental actions of one batch from source for
// removedSince. A Remove is recorded only while a replay is pending, because
// a request minted later is answered from a snapshot taken after the Remove.
// Safe on a nil coordinator, which a unit test of the dispatch alone leaves.
func (c *replayCoordinator) noteIncremental(source string, fam family.Family, entries []redistevents.RouteChangeEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pending) == 0 || c.nowFn().After(c.lastDeadline) {
		// Every mapping has expired: no replay batch can still be answered,
		// so nothing it carries needs judging against a Remove.
		clear(c.removed)
		return
	}
	for i := range entries {
		k := sourceRoute{source: source, family: fam, prefix: entries[i].Prefix}
		//exhaustive:ignore // only the two actions that change a route's presence are recorded; the dispatcher refuses the others
		switch entries[i].Action {
		case redistevents.ActionRemove:
			c.removed[k] = c.gen
		case redistevents.ActionAdd:
			delete(c.removed, k)
		}
	}
}

// removedSince answers whether the most recent incremental action on source's
// route was a Remove handled after replayID was minted, so the replay's Add of
// it is stale.
func (c *replayCoordinator) removedSince(replayID uint64, source string, fam family.Family, prefix netip.Prefix) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	g, ok := c.removed[sourceRoute{source: source, family: fam, prefix: prefix}]
	if !ok {
		return false
	}
	return g >= replayID
}

// evictLocked drops expired entries and, if still over the hard cap, the oldest
// (lowest-replayID) entries. Caller holds c.mu.
func (c *replayCoordinator) evictLocked() {
	now := c.nowFn()
	for id, e := range c.pending {
		if now.After(e.deadline) {
			delete(c.pending, id)
		}
	}
	// Hard cap: replayIDs are monotonic, so the lowest IDs are the oldest.
	// Guard maxSize > 0 so a zero cap cannot spin (delete of a missing key is a
	// no-op that would never shrink the map).
	for c.maxSize > 0 && len(c.pending) >= c.maxSize {
		var oldest uint64
		first := true
		for id := range c.pending {
			if first || id < oldest {
				oldest = id
				first = false
			}
		}
		delete(c.pending, oldest)
	}
}

// handleReplayBatch injects a ReplayID-tagged batch to the target its replayID
// maps to. An unknown/expired replayID is dropped with a warn (never
// mis-delivered). Only Add entries are injected: a re-emit is a snapshot of the
// producer's CURRENT live set (all adds); a route withdrawn before the target
// joined is simply absent (AC-4).
func handleReplayBatch(ctx context.Context, b *redistevents.RouteChangeBatch) {
	coord := getReplayCoordinator()
	name := redistevents.ProtocolName(b.Protocol)
	target, ok := coord.lookupTarget(b.ReplayID)
	if !ok {
		logger().Warn("redistribute-orchestrator: replay batch with unknown/expired ReplayID, dropping",
			"replay-id", b.ReplayID, "source", name)
		return
	}
	if name == "" {
		logger().Warn("redistribute-orchestrator: replay batch from unregistered ProtocolID", "id", b.Protocol)
		return
	}

	// The destination this replay feeds, and the peer selector it feeds it
	// with. A peer-up replay reaches one peer through the BGP consumer; a
	// consumer-registration replay reaches every peer through the one consumer
	// that just registered, which is the ordinary fan-out an incremental batch
	// takes (dispatchEntryToConsumer, redistribute.go).
	var destination, peer string
	switch target.kind {
	case replayKindPeer:
		destination, peer = bgpDestination, target.name
	case replayKindConsumer:
		destination, peer = target.name, ""
	case replayKindUnspecified:
		// An entry nobody filled in. Dropping is the only safe answer: an empty
		// destination matches a destination-agnostic rule, so falling through
		// would deliver the batch to a consumer nobody named.
		logger().Warn("BUG: redistribute-orchestrator: replay entry with no target kind, dropping",
			"replay-id", b.ReplayID, "source", name)
		return
	default:
		panic("BUG: invalid replay target kind")
	}

	// Loop prevention (whole-batch drop): a source protocol's batch is never
	// redistributed back into that same protocol's consumer.
	if redistevents.WouldLoop(name, destination) {
		return
	}

	ev, gen := consumerHeld.evaluator()
	if ev == nil {
		logger().Warn("redistribute-orchestrator: no evaluator configured, dropping replay batch", "source", name)
		return
	}
	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}

	consumer, ok := configredist.LookupConsumer(destination)
	if !ok {
		logger().Warn("redistribute-orchestrator: consumer not registered, dropping replay batch", "destination", destination)
		return
	}

	for i := range b.Entries {
		entry := &b.Entries[i]
		if entry.Action != redistevents.ActionAdd {
			continue // a replay reflects the current live set; only adds are meaningful
		}
		// The producer took this snapshot before it removed the route, and the
		// Remove reached the orchestrator first. Dispatching the stale Add would
		// hand the target a route its source no longer has, with no Remove left
		// to retire it.
		if coord.removedSince(b.ReplayID, name, famVal, entry.Prefix) {
			logger().Debug("redistribute-orchestrator: replay entry removed since the request, dropping", "source", name, "family", famVal.String(), "prefix", entry.Prefix, "replay-id", b.ReplayID)
			continue
		}
		// Destination-scoped, and decided for each route: only sources imported
		// under this destination replay, and an import rule whose chain selects
		// routes (by tag, family, BGP class) replays only the routes it accepts.
		// The incremental path decides the same way.
		route := redistRoute(name, b.Protocol, famVal, entry)
		sets, accepted := ev.Accept(&route, destination)
		if !accepted {
			logger().Debug("redistribute-orchestrator: evaluator rejected replay entry", "source", name, "destination", destination, "family", famVal.String(), "prefix", entry.Prefix, "tag", entry.Tag)
			countChainRejected(ev, &route, destination, name)
		}
		// A peer replay feeds one BGP peer a route the consumer already holds
		// from the incremental path, so it changes nothing the held set records
		// and always sends: the peer holds nothing yet.
		if target.kind != replayKindConsumer {
			if accepted {
				dispatchReplayed(ctx, consumer, famVal, name, peer, b, entry, &route, sets)
			}
			continue
		}
		// A consumer replay is what that consumer holds, whether it just
		// registered or a reload changed its rules, so it settles in the held
		// set as an incremental Add does. It sends only what the consumer does
		// not already hold as it would be sent (heldRoutes.beginAdd).
		key := heldKey{consumer: destination, source: name, family: famVal, prefix: entry.Prefix}
		sent := newSentRoute(sets, entry, b.OriginASN, b.Community)
		dispatched := false
		if accepted {
			dispatched = beginDispatchedAdd(key, &route, &sent, true)
		}
		if dispatched {
			dispatchReplayed(ctx, consumer, famVal, name, peer, b, entry, &route, sets)
		}
		settleDispatchedAdd(ctx, consumer, key, gen, dispatched, accepted, &route, &sent, entry, peer, b.OriginASN, b.Community)
	}
	logger().Debug("redistribute-orchestrator: replayed source", "source", name, "destination", destination, "peer", peer, "entries", len(b.Entries))
}

// dispatchReplayed sends one replayed entry and counts it.
func dispatchReplayed(ctx context.Context, consumer configredist.RedistConsumer, fam family.Family, source, peer string, b *redistevents.RouteChangeBatch, entry *redistevents.RouteChangeEntry, view *configredist.RedistRoute, sets ribpolicy.Sets) {
	dispatchEntryToConsumer(ctx, consumer, fam, source, peer, b.OriginASN, b.Community, entry, view, sets)
	if m := getMetrics(); m != nil && m.replayTotal != nil {
		m.replayTotal.With(source).Inc()
	}
}
