// Design: plan/learned/936-isis-11-redistribution.md -- IS-IS redistribution source (producer).
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
			Name:        "isis",
			Protocol:    "isis",
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
// delta is still walked (cheap, and keeps the path uniform).
type Source struct {
	bus ze.EventBus
}

// NewSource constructs a producer Source emitting on bus (may be nil).
func NewSource(bus ze.EventBus) *Source { return &Source{bus: bus} }

// OnSPFChange is the SPF Computer OnChange callback: it turns the route delta into
// a single redistevents batch and emits it (both levels, single source). It is the
// REDISTRIBUTION path only -- the FIB install is the Computer's own Installer
// (Loc-RIB), a separate path.
func (s *Source) OnSPFChange(delta spf.RouteDelta) {
	emitDelta(delta, isisredistevents.ProtocolID, s.send)
}

// send delivers one filled batch on the typed RouteChange handle and is the
// production sink for emitDelta. The pooled batch is owned by emitDelta (acquired
// and released there); send only Emits while it is alive. A nil bus is a no-op.
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

// addEntry maps an SPF RouteEntry to an ActionAdd redistevents entry. The next-hop
// is the first resolved equal-cost next-hop (the redistevents entry carries a
// single NextHop; the BGP consumer maps it to `nhop <addr>`); a zero NextHop means
// "nhop self" on the consumer side. The metric is the SPF path cost narrowed to
// the 32-bit redistevents field.
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
// not leaked). This is the connected-prefix advertisement path: it writes the
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
		out = append(out, lsdb.PrefixInfo{
			Prefix: p.Masked(),
			Metric: types.NewPrefixMetric(metric),
			UpDown: false,
		})
	}
	return out
}
