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
			Name:        "ospf",
			Protocol:    "ospf",
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
// still walked (cheap, and keeps the path uniform).
type Source struct {
	bus ze.EventBus
}

// NewSource constructs a producer Source emitting on bus (may be nil).
func NewSource(bus ze.EventBus) *Source { return &Source{bus: bus} }

// OnSPFChange is the SPF Computer OnChange callback: it turns the route delta into a
// single redistevents batch and emits it (intra + inter, single source). It is the
// REDISTRIBUTION path only -- the FIB install is the Computer's own Installer
// (Loc-RIB), a separate path.
func (s *Source) OnSPFChange(delta spf.RouteDelta) {
	emitDelta(delta, ospfredistevents.ProtocolID, s.send)
}

// send delivers one filled batch on the typed RouteChange handle and is the
// production sink for emitDelta. The pooled batch is owned by emitDelta (acquired
// and released there); send only Emits while it is alive. A nil bus is a no-op.
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
