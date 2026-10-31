// Design: docs/architecture/isis/isis-11-redistribution.md -- IS-IS redistribution source (producer).
// Related: internal/plugins/isis/redistribute/events -- the redistevents producer wiring.
// Related: internal/component/bgp/redistribute -- the source-registration template (sync.Once).
// RFC: rfc/short/rfc5305.md -- connected/redistributed prefixes are TLV 135 entries
//
// The IS-IS redistribution SOURCE has two jobs:
//
//   - Register the SINGLE config source "isis" (RegisterSource) so the
//     redistribute-source YANG validator and editor completion accept it. This is
//     a no-op for actual route delivery on its own; the redistevents producer
//     (events sub-package) is what makes the orchestrator subscribe (AC-11).
//   - Convert SPF route changes (spec-isis-9 deltas, BOTH levels) into a single
//     redistevents.RouteChangeBatch under the single isis ProtocolID and EMIT it on
//     the typed RouteChange handle, so the orchestrator dispatches IS-IS routes to
//     the BGP consumer (export IS-IS -> BGP). This NEVER installs to the FIB.
//   - Keep the redistributable route set those deltas built, per address family,
//     and re-emit it on a redistevents ReplayRequest tagged with the request's
//     ReplayID, so the orchestrator's late-join replay and its reload and rollback
//     reconciliation reach IS-IS routes without waiting for the next SPF delta.
//
// It also exposes the connected-prefix helper (connectedPrefixInfos) the engine
// uses to advertise its own enabled/passive interface prefixes as internal TLV 135
// reachability (AC-8); that path writes LSPs directly (not the redistevents bus).

package isisredistribute

import (
	"log/slog"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	isisredistevents "github.com/ze-software/ze/internal/plugins/isis/redistribute/events"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
	"github.com/ze-software/ze/internal/plugins/isis/types"
	"github.com/ze-software/ze/pkg/ze"

	"net/netip"
)

var sourcesOnce sync.Once

// RegisterISISSources registers the SINGLE IS-IS redistribute config source
// "isis" (Protocol "isis"). Safe to call multiple times (sync.Once). There is NO
// per-level isis-l1 / isis-l2 source: redistevents.RouteChangeBatch has no level
// field, the orchestrator derives the source from ProtocolName(Protocol), and the
// loop-prevention evaluator matches the consumer's importing name "isis" -- a
// single source keeps self-import auto-rejected (AC-10) and matches the single
// admin distance (umbrella "Redistribution source"). Mirrors BGP RegisterBGPSources.
func RegisterISISSources() {
	sourcesOnce.Do(func() {
		mustRegister(configredist.RouteSource{
			Name:     "isis",
			Protocol: "isis",
			// A rule importing isis with no isis block moves nothing, so it is
			// refused at load and commit (config.RefuseRedistributeRules).
			ConfigRoot:  "isis",
			Description: "IS-IS SPF routes",
		})
	})
}

// mustRegister registers src and logs (does not panic) on error, exactly like the
// BGP source registration: a duplicate/conflicting registration is a programmer
// bug surfaced in the log, not a process kill.
func mustRegister(src configredist.RouteSource) {
	if err := configredist.RegisterSource(src); err != nil {
		slog.Error("BUG: failed to register IS-IS redistribute source", "name", src.Name, "err", err)
	}
}

// Source is the engine-facing producer: it holds the EventBus and emits SPF route
// deltas as redistevents batches. The engine constructs one and wires it as the
// SPF Computer's OnChange callback (so every SPF run that changes the route set
// emits). A nil bus (engine without an event bus) makes Emit a no-op, but the
// delta is still walked and recorded (cheap, and keeps the path uniform).
//
// Safe for concurrent use: OnSPFChange, OnSPFChangeV6 and Replay serialize on mu.
type Source struct {
	bus ze.EventBus
	// mu orders every emit. A delta (either family) and a replay each record and
	// emit while holding it, so a replay can never revive a route a later delta
	// removed, nor miss one an earlier delta added.
	mu sync.Mutex
	// current is the redistributable route set the emitted deltas built, one
	// prefix map per address family: exactly what a replay re-emits. Guarded by mu.
	current map[family.AFI]map[netip.Prefix]redistevents.RouteChangeEntry
}

// replayFamilies is the order Replay emits its per-family batches in.
var replayFamilies = [...]family.AFI{family.AFIIPv4, family.AFIIPv6}

// NewSource constructs a producer Source emitting on bus (may be nil).
func NewSource(bus ze.EventBus) *Source {
	current := make(map[family.AFI]map[netip.Prefix]redistevents.RouteChangeEntry, len(replayFamilies))
	for _, afi := range replayFamilies {
		current[afi] = make(map[netip.Prefix]redistevents.RouteChangeEntry)
	}
	return &Source{bus: bus, current: current}
}

// OnSPFChange is the SPF Computer OnChange callback: it turns the route delta into
// a single redistevents batch and emits it (both levels, single source). It is the
// REDISTRIBUTION path only -- the FIB install is the Computer's own Installer
// (Loc-RIB), a separate path.
func (s *Source) OnSPFChange(delta spf.RouteDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emitDelta(delta, isisredistevents.ProtocolID, s.recordAndSend)
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
// per address family, tagged with replayID, the token of the ReplayRequest it
// answers, the way the static and connected producers' reemitAll do. It emits
// under mu, the lock that orders the incremental deltas. A zero replayID is a
// no-op (the orchestrator only allocates nonzero tokens), and a family with no
// route emits nothing.
func (s *Source) Replay(replayID uint64) {
	if replayID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, afi := range replayFamilies {
		s.replayFamily(afi, replayID)
	}
}

// replayFamily emits one family's current set as a replay batch. The caller MUST
// hold mu.
func (s *Source) replayFamily(afi family.AFI, replayID uint64) {
	routes := s.current[afi]
	if len(routes) == 0 {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = isisredistevents.ProtocolID
	b.AFI = uint16(afi)
	b.SAFI = uint8(family.SAFIUnicast)
	b.ReplayID = replayID
	for _, e := range routes {
		b.Entries = append(b.Entries, e)
	}
	s.send(b)
}

// recordAndSend is the delta sink: it folds the batch into its family's current
// set, then emits it. The caller MUST hold mu.
func (s *Source) recordAndSend(b *redistevents.RouteChangeBatch) {
	routes, ok := s.current[family.AFI(b.AFI)]
	if !ok {
		panic("BUG: isis redist source emitted a batch for a family it does not track")
	}
	recordEntries(routes, b.Entries)
	s.send(b)
}

// recordEntries folds entries into current: an add stores the entry, a remove
// drops the prefix. Every entry comes from emitDeltaFamily, which writes only adds
// and removes, so the switch is closed (class C).
func recordEntries(current map[netip.Prefix]redistevents.RouteChangeEntry, entries []redistevents.RouteChangeEntry) {
	for i := range entries {
		e := entries[i]
		switch e.Action {
		case redistevents.ActionAdd:
			current[e.Prefix] = e
		case redistevents.ActionRemove:
			delete(current, e.Prefix)
		case redistevents.ActionUnspecified:
			panic("BUG: isis redist source recorded an entry with no action")
		}
	}
}

// send delivers one filled batch on the typed RouteChange handle and is the
// production sink for the deltas and Replay. The pooled batch is owned by the
// caller (acquired and released there); send only Emits while it is alive. A nil
// bus is a no-op.
func (s *Source) send(b *redistevents.RouteChangeBatch) {
	if s.bus == nil {
		return
	}
	if _, err := isisredistevents.RouteChange.Emit(s.bus, b); err != nil {
		slog.Warn("isis redist source: route-change emit failed", "error", err)
	}
}

// emitDelta converts an SPF RouteDelta into ONE redistevents.RouteChangeBatch
// (Protocol = the single isis ProtocolID, family ipv4/unicast) and hands it to
// sink. It is the IPv4 wrapper over emitDeltaFamily (ipv6.go holds the IPv6
// wrapper); both share the single isis source and differ only in the batch AFI.
// Added and Changed routes become ActionAdd entries (the consumer treats an add
// as add-or-replace); Removed prefixes become ActionRemove entries. An empty
// delta emits nothing. Both L1 and L2 routes go in the one batch (single source,
// no per-level selector -- AC-2).
func emitDelta(delta spf.RouteDelta, protocol redistevents.ProtocolID, sink func(*redistevents.RouteChangeBatch)) {
	emitDeltaFamily(delta, protocol, family.AFIIPv4, sink)
}

// addEntry maps a forwarding route to an ActionAdd entry. A protocol-suite
// rejection withdraws any previous export: an invalid gateway otherwise means
// "next-hop self" to the consumer, which would advertise unreachable traffic.
// The first resolved equal-cost next hop supplies the export's single gateway.
func addEntry(r *spf.RouteEntry) redistevents.RouteChangeEntry {
	var nh netip.Addr
	if len(r.NextHops) > 0 {
		if r.NextHops[0].Unsupported {
			return redistevents.RouteChangeEntry{Action: redistevents.ActionRemove, Prefix: r.Prefix}
		}
		nh = r.NextHops[0].Addr
	}
	return redistevents.RouteChangeEntry{
		Action:  redistevents.ActionAdd,
		Prefix:  r.Prefix,
		NextHop: nh,
		Metric:  metricToUint32(r.Metric),
	}
}

// metricToUint32 narrows the SPF 64-bit path cost to the uint32 redistevents
// Metric field, saturating rather than wrapping (MaxPathMetric fits in uint32, so
// no clamp occurs in practice; the guard is defense in depth).
func metricToUint32(m uint64) uint32 {
	const maxU32 = uint64(^uint32(0))
	if m > maxU32 {
		return ^uint32(0)
	}
	return uint32(m)
}

// ConnectedPrefixInfos turns a list of interface prefixes into TLV 135 PrefixInfo
// internal-reachability entries advertised at the node's own metric (AC-8). The
// prefix is masked to its network; the up/down bit is 0 (internal reachability,
// not leaked). A zero-network, loopback or multicast address is dropped
// (unroutableIPv4). This is the connected-prefix advertisement path: it writes the
// node's own enabled/passive interface prefixes into its LSPs without an adjacency
// (a passive interface forms no adjacency but is still advertised, RFC 1195). It is
// a pure helper so the engine can build the set at circuit-up and the test can
// assert masking + metric without a live engine.
func ConnectedPrefixInfos(prefixes []netip.Prefix, metric uint32) []lsdb.PrefixInfo {
	out := make([]lsdb.PrefixInfo, 0, len(prefixes))
	for _, p := range prefixes {
		if !p.IsValid() {
			continue
		}
		if unroutableIPv4(p.Addr()) {
			continue
		}
		out = append(out, lsdb.PrefixInfo{
			Prefix: p.Masked(),
			Metric: types.NewPrefixMetric(metric),
			UpDown: false,
		})
	}
	return out
}

// unroutableIPv4 reports whether an interface address is one IS-IS never
// advertises: the zero network 0/8, loopback 127/8 or class D multicast 224/4.
// A passive `lo` carries 127.0.0.1/8, and advertising it makes every neighbor
// install a route to its own loopback range through this router. FRR refuses the
// same set when it collects a circuit's addresses, isis_circuit_add_addr in
// isisd/isis_circuit.c:
//
//	if (IPV4_NET0(addr) || IPV4_NET127(addr) || IN_CLASSD(addr))
//		return;
func unroutableIPv4(addr netip.Addr) bool {
	if !addr.Is4() {
		return false
	}
	if addr.As4()[0] == 0 {
		return true
	}
	if addr.IsLoopback() {
		return true
	}
	return addr.IsMulticast()
}
