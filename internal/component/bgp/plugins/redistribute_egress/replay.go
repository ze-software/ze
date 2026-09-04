// Design: docs/architecture/core-design.md -- redistribute replay-on-request
//
// This file implements the two triggers of the late-join replay and the
// targeting of each. A producer emits once, and whoever was not listening at
// that moment holds nothing, so each trigger names a party that arrived late:
//
//	BGP peer down->up edge      --> replayCoordinator.onPeerUp
//	consumer becomes registered --> replayCoordinator.onConsumerRegistered
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

package redistributeegress

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
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
type replayCoordinator struct {
	mu      sync.Mutex
	gen     uint64
	pending map[uint64]replayEntry
	peerUp  map[string]bool
	ttl     time.Duration
	maxSize int
	// nowFn is injectable so tests can drive TTL eviction deterministically.
	nowFn func() time.Time
}

func newReplayCoordinator() *replayCoordinator {
	return &replayCoordinator{
		pending: make(map[uint64]replayEntry),
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

// onPeerUp handles a BGP peer establishment. On the down->up edge it allocates a
// replayID, records replayID -> peer, and emits ReplayRequest{replayID} so
// producers re-emit their current set for this peer. Returns (replayID, true)
// when a request was fired; (0, false) when the peer was already up (no edge),
// there is no configured evaluator, or no import feeds BGP. The evaluator gate
// avoids a pointless broadcast when redistribute has no BGP destination.
func (c *replayCoordinator) onPeerUp(bus ze.EventBus, peer string) (uint64, bool) {
	if c == nil || bus == nil || peer == "" {
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
	if ev == nil || !ev.HasDestination(bgpDestination) {
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
// The evaluator gate is R-2. A consumer no rule imports into gains nothing from
// a replay, and firing anyway would storm every producer once per consumer on
// every startup.
func (c *replayCoordinator) onConsumerRegistered(bus ze.EventBus, consumer string) (uint64, bool) {
	if c == nil || bus == nil || consumer == "" {
		return 0, false
	}
	ev := configredist.Global()
	if ev == nil || !ev.HasDestination(consumer) {
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
	c.pending[id] = replayEntry{kind: kind, name: name, deadline: c.nowFn().Add(c.ttl)}
	c.mu.Unlock()

	// Emit OUTSIDE the lock: engine subscribers deliver synchronously and the
	// producer re-emit re-enters handleReplayBatch -> lookupTarget, which locks
	// c.mu. Holding the lock here would deadlock that nested dispatch.
	if _, err := redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: id}); err != nil {
		logger().Warn("redistribute-orchestrator: replay-request emit failed", "target", name, "replay-id", id, "error", err)
		return id, true
	}
	logger().Debug("redistribute-orchestrator: replay request fired", "kind", kind, "target", name, "replay-id", id)
	return id, true
}

// onPeerDown clears the up state for a peer so a later re-establishment is seen
// as a fresh down->up edge and fires a new replay.
func (c *replayCoordinator) onPeerDown(peer string) {
	if c == nil || peer == "" {
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
	default:
		// replayKindUnspecified, and any kind a later change adds while this
		// switch stays silent about what it targets. Dropping is the only safe
		// answer. An empty destination matches a destination-agnostic rule, so
		// falling through would deliver the batch to a consumer nobody named.
		logger().Warn("BUG: redistribute-orchestrator: replay entry with no target kind, dropping",
			"replay-id", b.ReplayID, "source", name, "kind", target.kind)
		return
	}

	// Loop prevention (whole-batch drop): a source protocol's batch is never
	// redistributed back into that same protocol's consumer.
	if redistevents.WouldLoop(name, destination) {
		return
	}

	ev := configredist.Global()
	if ev == nil {
		logger().Warn("redistribute-orchestrator: no evaluator configured, dropping replay batch", "source", name)
		return
	}
	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}
	route := configredist.RedistRoute{Origin: name, Family: famVal, Source: name}
	// Destination-scoped: only sources imported under this destination replay.
	if !ev.Accept(route, destination) {
		logger().Debug("redistribute-orchestrator: evaluator rejected replay batch", "source", name, "destination", destination, "family", famVal.String())
		return
	}

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
		dispatchEntryToConsumer(ctx, consumer, famVal, name, peer, b.OriginASN, b.Community, entry)
		if m := getMetrics(); m != nil && m.replayTotal != nil {
			m.replayTotal.With(name).Inc()
		}
	}
	logger().Debug("redistribute-orchestrator: replayed source", "source", name, "destination", destination, "peer", peer, "entries", len(b.Entries))
}
