// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- OSPF redistribution source (producer).
// Related: internal/plugins/ospf/redistribute/events -- the redistevents producer wiring.
// Related: internal/plugins/isis/redistribute -- the source-registration template (sync.Once).
//
// The OSPF redistribution SOURCE has two jobs:
//
//   - Register the SINGLE config source "ospf" (RegisterSource) so the
//     redistribute-source YANG validator and editor completion accept it. This is a
//     no-op for actual route delivery on its own; the redistevents producer (events
//     sub-package) is what makes the orchestrator subscribe (AC-14).
//   - Convert SPF route changes (spec-ospf-8/9 deltas, intra + inter) into a single
//     redistevents.RouteChangeBatch under the single ospf ProtocolID and EMIT it on
//     the typed RouteChange handle, so the orchestrator dispatches OSPF routes to the
//     BGP consumer (export OSPF -> BGP). This NEVER installs to the FIB.
//   - Keep the redistributable route set those deltas built, and re-emit it on a
//     redistevents ReplayRequest tagged with the request's ReplayID, so the
//     orchestrator's late-join replay and its reload and rollback reconciliation
//     reach OSPF routes without waiting for the next SPF delta.

package ospfredistribute

import (
	"log/slog"
	"net/netip"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ospfredistevents "github.com/ze-software/ze/internal/plugins/ospf/redistribute/events"
	"github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/pkg/ze"
)

var sourcesOnce sync.Once

// RegisterOSPFSources registers the SINGLE OSPF redistribute config source "ospf"
// (Protocol "ospf"). Safe to call multiple times (sync.Once). There is NO per-area
// source: redistevents.RouteChangeBatch has no area/path-type field, the
// orchestrator derives the source from ProtocolName(Protocol), and the
// loop-prevention evaluator matches the consumer's importing name "ospf" -- a
// single source keeps self-import auto-rejected (AC-13) and matches the single
// admin distance (umbrella "Redistribution source"). Mirrors IS-IS
// RegisterISISSources.
func RegisterOSPFSources() {
	sourcesOnce.Do(func() {
		mustRegister(configredist.RouteSource{
			Name:     "ospf",
			Protocol: "ospf",
			// A rule importing ospf with no ospf block moves nothing, so it is
			// refused at load and commit (config.RefuseRedistributeRules).
			ConfigRoot:  "ospf",
			Description: "OSPF SPF routes",
		})
	})
}

// mustRegister registers src and logs (does not panic) on error, exactly like the
// IS-IS source registration: a duplicate/conflicting registration is a programmer
// bug surfaced in the log, not a process kill.
func mustRegister(src configredist.RouteSource) {
	if err := configredist.RegisterSource(src); err != nil {
		slog.Error("BUG: failed to register OSPF redistribute source", "name", src.Name, "err", err)
	}
}

// Source is the engine-facing producer: it holds the EventBus and emits SPF route
// deltas as redistevents batches. The engine constructs one and wires it as the SPF
// Computer's OnChange callback (so every SPF run that changes the route set emits).
// A nil bus (engine without an event bus) makes Emit a no-op, but the delta is
// still walked and recorded (cheap, and keeps the path uniform).
//
// Safe for concurrent use: OnSPFChange and Replay serialize on mu.
type Source struct {
	bus ze.EventBus
	// mu orders every emit. A delta and a replay each record and emit while
	// holding it, so a replay can never revive a route a later delta removed,
	// nor miss one an earlier delta added.
	mu sync.Mutex
	// current is the redistributable route set the emitted deltas built, keyed
	// by prefix: exactly what a replay re-emits. Guarded by mu.
	current map[netip.Prefix]redistevents.RouteChangeEntry
}

// NewSource constructs a producer Source emitting on bus (may be nil).
func NewSource(bus ze.EventBus) *Source {
	return &Source{bus: bus, current: make(map[netip.Prefix]redistevents.RouteChangeEntry)}
}

// OnSPFChange is the SPF Computer OnChange callback: it turns the route delta into a
// single redistevents batch and emits it (intra + inter, single source). It is the
// REDISTRIBUTION path only -- the FIB install is the Computer's own Installer
// (Loc-RIB), a separate path.
func (s *Source) OnSPFChange(delta spf.RouteDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emitDelta(delta, ospfredistevents.ProtocolID, s.recordAndSend)
}

// SubscribeReplay subscribes the Source to the shared redistribute ReplayRequest
// broadcast and returns the unsubscribe function, which the caller MUST call when
// the Source is replaced or the plugin stops. A nil bus subscribes nothing.
func (s *Source) SubscribeReplay() (unsubscribe func()) {
	if s.bus == nil {
		return func() {}
	}
	return redistevents.ReplayRequestEvent.Subscribe(s.bus, func(r *redistevents.ReplayRequest) {
		s.Replay(r.ReplayID)
	})
}

// Replay re-emits the current redistributable route set as one ActionAdd batch
// tagged with replayID, the token of the ReplayRequest it answers, the way the
// static and connected producers' reemitAll do. It emits under mu, the lock that
// orders the incremental deltas. A zero replayID is a no-op (the orchestrator only
// allocates nonzero tokens), and an empty set emits nothing.
func (s *Source) Replay(replayID uint64) {
	if replayID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.current) == 0 {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = ospfredistevents.ProtocolID
	b.AFI = uint16(family.AFIIPv4)
	b.SAFI = uint8(family.SAFIUnicast)
	b.ReplayID = replayID
	for _, e := range s.current {
		b.Entries = append(b.Entries, e)
	}
	s.send(b)
}

// recordAndSend is OnSPFChange's sink: it folds the batch into current, then
// emits it. The caller MUST hold mu.
func (s *Source) recordAndSend(b *redistevents.RouteChangeBatch) {
	recordEntries(s.current, b.Entries)
	s.send(b)
}

// recordEntries folds entries into current: an add stores the entry, a remove
// drops the prefix. Every entry comes from emitDelta, which writes only adds and
// removes, so the switch is closed (class C).
func recordEntries(current map[netip.Prefix]redistevents.RouteChangeEntry, entries []redistevents.RouteChangeEntry) {
	for i := range entries {
		e := entries[i]
		switch e.Action {
		case redistevents.ActionAdd:
			current[e.Prefix] = e
		case redistevents.ActionRemove:
			delete(current, e.Prefix)
		case redistevents.ActionUnspecified:
			panic("BUG: ospf redist source recorded an entry with no action")
		}
	}
}

// send delivers one filled batch on the typed RouteChange handle and is the
// production sink for emitDelta and Replay. The pooled batch is owned by the
// caller (acquired and released there); send only Emits while it is alive. A nil
// bus is a no-op.
func (s *Source) send(b *redistevents.RouteChangeBatch) {
	if s.bus == nil {
		return
	}
	if _, err := ospfredistevents.RouteChange.Emit(s.bus, b); err != nil {
		slog.Warn("ospf redist source: route-change emit failed", "error", err)
	}
}

// emitDelta converts an SPF RouteDelta into ONE redistevents.RouteChangeBatch
// (Protocol = the single ospf ProtocolID, family ipv4/unicast) and hands it to
// sink. OSPFv2 is IPv4-only, so there is one family. Added and Changed routes
// become ActionAdd entries (the consumer treats an add as add-or-replace); Removed
// prefixes become ActionRemove entries. An empty delta emits nothing. Both intra-
// and inter-area routes go in the one batch (single source, no per-area selector --
// AC-2).
func emitDelta(delta spf.RouteDelta, protocol redistevents.ProtocolID, sink func(*redistevents.RouteChangeBatch)) {
	if delta.Empty() {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = protocol
	b.AFI = uint16(family.AFIIPv4)
	b.SAFI = uint8(family.SAFIUnicast)
	for i := range delta.Added {
		b.Entries = append(b.Entries, addEntry(&delta.Added[i]))
	}
	for i := range delta.Changed {
		b.Entries = append(b.Entries, addEntry(&delta.Changed[i]))
	}
	for _, pfx := range delta.Removed {
		b.Entries = append(b.Entries, redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: pfx})
	}
	if len(b.Entries) == 0 {
		return
	}
	sink(b)
}

// addEntry maps an SPF RouteEntry to an ActionAdd redistevents entry. The next-hop
// is the first resolved equal-cost next-hop (the redistevents entry carries a
// single NextHop; the BGP consumer maps it to `nhop <addr>`); a zero NextHop means
// "nhop self" on the consumer side. The metric is the SPF path cost narrowed to the
// 32-bit redistevents field.
func addEntry(r *spf.RouteEntry) redistevents.RouteChangeEntry {
	var nh netip.Addr
	if len(r.NextHops) > 0 {
		nh = r.NextHops[0].Addr
	}
	return redistevents.RouteChangeEntry{
		Action:  redistevents.ActionAdd,
		Prefix:  r.Prefix,
		NextHop: nh,
		Metric:  metricToUint32(r.Metric),
	}
}

// metricToUint32 narrows the SPF 64-bit path cost to the uint32 redistevents Metric
// field, saturating rather than wrapping (LSInfinity fits in uint32, so no clamp
// occurs in practice; the guard is defense in depth).
func metricToUint32(m uint64) uint32 {
	const maxU32 = uint64(^uint32(0))
	if m > maxU32 {
		return ^uint32(0)
	}
	return uint32(m)
}
