// Design: docs/architecture/core-design.md -- redistribute replay-on-request
//
// This file implements the peer-up trigger and new-peer targeting for the
// late-join replay (spec-redistribute-late-join-replay):
//
//	BGP peer down->up edge --> replayCoordinator.onPeerUp
//	   -> allocate monotonic replayID, record replayID -> peer (bounded + TTL)
//	   -> emit redistevents.ReplayRequest{replayID}
//	producers re-emit RouteChangeBatch{ReplayID: replayID}
//	   -> orchestrator handleBatch sees IsReplay() -> handleReplayBatch
//	   -> lookupTarget(replayID) -> inject to that ONE peer, bgp consumer only
//
// The producer never learns the peer; the coordinator alone holds the
// replayID -> peer mapping, so the returning batch stays peer-agnostic.

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

// bgpDestination is the destination protocol name the late-join replay targets.
// OSPF/ISIS redistribute consumers originate into a flooded/synchronized
// link-state DB and receive routes via database exchange on a new adjacency, so
// they have no late-join gap; only the BGP consumer sends per-peer and needs
// the replay.
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

// replayEntry records the peer a replay request targets and when the mapping
// expires.
type replayEntry struct {
	peer     string
	deadline time.Time
}

// replayCoordinator correlates BGP peer-up events to redistribute replay
// batches. On a peer's down->up edge it allocates a monotonic replayID, records
// replayID -> peer, and emits ReplayRequest{replayID}. Producers re-emit their
// current set tagged with the echoed replayID; the orchestrator's batch handler
// maps the ID back to the peer and targets only that peer. Distinct replayIDs
// per peer-up mean concurrent replays never cross-deliver (R-2).
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

	c.mu.Lock()
	c.evictLocked()
	c.gen++
	id := c.gen
	c.pending[id] = replayEntry{peer: peer, deadline: c.nowFn().Add(c.ttl)}
	c.mu.Unlock()

	// Emit OUTSIDE the lock: engine subscribers deliver synchronously and the
	// producer re-emit re-enters handleReplayBatch -> lookupTarget, which locks
	// c.mu. Holding the lock here would deadlock that nested dispatch.
	if _, err := redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: id}); err != nil {
		logger().Warn("redistribute-orchestrator: replay-request emit failed", "peer", peer, "replay-id", id, "error", err)
		return id, true
	}
	logger().Debug("redistribute-orchestrator: replay request fired on peer-up", "peer", peer, "replay-id", id)
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

// lookupTarget returns the peer a replayID maps to, or ("", false) when the ID
// is unknown or its mapping has expired. It does NOT evict on lookup: several
// producers each re-emit for the same replayID, so the mapping must survive
// until the TTL, not until the first returning batch.
func (c *replayCoordinator) lookupTarget(replayID uint64) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.pending[replayID]
	if !ok {
		return "", false
	}
	if c.nowFn().After(e.deadline) {
		delete(c.pending, replayID)
		return "", false
	}
	return e.peer, true
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

// handleReplayBatch injects a ReplayID-tagged batch to the single peer its
// replayID maps to, via the BGP consumer only. An unknown/expired replayID is
// dropped with a warn (never mis-delivered). Only Add entries are injected: a
// re-emit is a snapshot of the producer's CURRENT live set (all adds); a route
// withdrawn before the peer joined is simply absent (AC-4).
func handleReplayBatch(ctx context.Context, b *redistevents.RouteChangeBatch) {
	coord := getReplayCoordinator()
	name := redistevents.ProtocolName(b.Protocol)
	peer, ok := coord.lookupTarget(b.ReplayID)
	if !ok {
		logger().Warn("redistribute-orchestrator: replay batch with unknown/expired ReplayID, dropping",
			"replay-id", b.ReplayID, "source", name)
		return
	}
	if name == "" {
		logger().Warn("redistribute-orchestrator: replay batch from unregistered ProtocolID", "id", b.Protocol)
		return
	}
	// Loop prevention (whole-batch drop): a bgp-sourced batch is never
	// redistributed back into bgp on the single-destination replay path.
	if redistevents.WouldLoop(name, bgpDestination) {
		return
	}

	ev := configredist.Global()
	if ev == nil {
		logger().Warn("redistribute-orchestrator: no evaluator configured, dropping replay batch", "source", name)
		return
	}
	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}
	route := configredist.RedistRoute{Origin: name, Family: famVal, Source: name}
	// Destination-scoped: only sources imported under `destination bgp` replay.
	if !ev.Accept(route, bgpDestination) {
		logger().Debug("redistribute-orchestrator: evaluator rejected replay batch", "source", name, "family", famVal.String())
		return
	}

	consumer, ok := configredist.LookupConsumer(bgpDestination)
	if !ok {
		logger().Warn("redistribute-orchestrator: bgp consumer not registered, dropping replay batch")
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
	logger().Debug("redistribute-orchestrator: replayed source to new peer", "source", name, "peer", peer, "entries", len(b.Entries))
}
