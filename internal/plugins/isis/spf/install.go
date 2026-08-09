// Design: docs/architecture/isis/isis-9-spf-rib.md -- FIB install via Loc-RIB insertion.
// IS-IS does NOT invent route installation: SPF results are INSERTED into the
// shared cross-protocol Loc-RIB exactly as BGP mirrors its best path
// (internal/component/bgp/plugins/rib/rib_bestchange.go:813
// r.locRIB.InsertForward(fam, pfx, locrib.Path{...})). sysrib consumes the
// Loc-RIB via loc.OnChange, applies admin distance, recomputes the system best,
// and fibkernel programs the kernel route as RTPROT_ZE. This is NOT redistevents
// (that path feeds the redistribute-orchestrator for redistribution to BGP and
// is owned by isis-11; it NEVER installs to the FIB) -- see umbrella Shared
// Contracts "Route install vs redistribution".
//
// Admin distance: IS-IS sets a single AdminDistance (115) on every locrib.Path,
// looked up by sysrib effectivePriority from the existing rib.admin-distance.isis
// leaf. locrib.Path has no protoType/level field, so the L1-over-L2 preference is
// resolved INSIDE SPF (route.go) before exactly one Path per prefix is published
// (umbrella A-3).
//
// ECMP (spec AC-3): one locrib.Path per equal-cost next-hop, distinct Instance.
// Because sysrib keys s.routes[key] by protocol string and the Loc-RIB Change
// carries only the single best Path, the equal-cost siblings survive to the
// kernel via the sysrib/locrib path-group expansion into BestChangeEntry.ECMPPaths
// (internal/component/sysrib, this spec's committed deliverable, umbrella A-2).

package spf

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// isisProtocolID is the Loc-RIB / redistribute source identity for IS-IS,
// allocated once at init (mirrors bgpProtocolID in the BGP RIB plugin). It is the
// Source set on every locrib.Path SPF inserts and the key sysrib uses to look up
// the IS-IS admin distance. The single name "isis" matches the existing
// rib.admin-distance.isis leaf and the single redistribution source (isis-11).
var isisProtocolID = redistevents.RegisterProtocol("isis")

// ProtocolID returns the registered IS-IS Loc-RIB / redistribute protocol ID.
// Exposed so the redistribution spec (isis-11) reuses the SAME identity rather
// than registering a second one (the registry is idempotent on name, but a
// single accessor keeps the contract explicit).
func ProtocolID() redistevents.ProtocolID { return isisProtocolID }

// DefaultAdminDistance is the IS-IS administrative distance set on every
// locrib.Path (classical default 115). sysrib overrides it from the
// rib.admin-distance.isis leaf via effectivePriority; this is the value placed on
// the Path so that, absent config, IS-IS ranks at 115 against other protocols.
const DefaultAdminDistance uint8 = 115

// RouteSink receives Loc-RIB install/remove operations when the installer has no
// local Loc-RIB (a forked subprocess, where locrib.Default() returns nil). The
// forked wiring installs one via SetRemoteSink; it ships each op to the engine over
// RPC (internal/core/rib/routeinstall). In-process (loc != nil) the local RIB is
// always preferred and the sink is unused.
type RouteSink interface {
	InsertForward(fam family.Family, prefix netip.Prefix, p locrib.Path)
	Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32)
	// Flush sends buffered ops to the engine; the installer calls it once per
	// Apply/RemoveAll so a whole delta travels in one RPC batch (R-1).
	Flush()
}

// Installer inserts SPF routes into the shared Loc-RIB and tracks the installed
// set so a subsequent run's diff can add/change/remove precisely. It mirrors the
// BGP install shape: InsertForward on add/change, Remove (forward-remove) on
// loss. With neither a local Loc-RIB nor a remote sink (an unwired test), every
// operation is a no-op, exactly as the BGP RIB is nil-safe; a forked subprocess
// gets a RouteSink (SetRemoteSink) so ops reach the engine over RPC.
type Installer struct {
	loc      *locrib.RIB
	remote   RouteSink
	fam      family.Family
	afi      string // metric label ("ipv4"|"ipv6") for ze_isis_routes_installed
	distance uint8

	// installed is the last set of routes pushed to the Loc-RIB, keyed by prefix,
	// so the next run diffs against it. Holds the per-prefix next-hop Instances
	// actually inserted (so a shrinking ECMP set removes the dropped Instances).
	installed map[netip.Prefix]installedRoute

	// metricsMu guards routesInstalled. SetMetrics writes the handle while
	// publishCounts (called on every Apply/RemoveAll) reads it. In production these
	// run on the single SPF goroutine so no race is observable, but the handle is
	// lock-guarded for defense in depth (mirrors the LSDB, which guards its gauge
	// handles under its RWMutex) so a future caller wiring metrics off-thread does
	// not introduce a data race on the interface value.
	metricsMu       sync.RWMutex
	routesInstalled metrics.GaugeVec
}

// installedRoute records what was actually inserted for one prefix: the
// per-next-hop Instance IDs (so Remove can drop exactly the right (Source,
// Instance) pairs) plus the route metadata for diffing.
type installedRoute struct {
	entry     RouteEntry
	instances []uint32 // Instance assigned to each inserted next-hop, index-aligned
}

// NewInstaller constructs an Installer for the IPv4-unicast family over loc (may
// be nil in a forked subprocess). distance is the admin distance to stamp on
// each Path (DefaultAdminDistance unless overridden). Metrics start as no-ops
// until SetMetrics wires a registry.
func NewInstaller(loc *locrib.RIB) *Installer {
	return newInstaller(loc, family.IPv4Unicast, "ipv4")
}

// NewInstallerV6 constructs an Installer for the IPv6-unicast family (isis-12).
// It is identical to the IPv4 Installer except for the Loc-RIB family and the
// ze_isis_routes_installed afi label; the Loc-RIB insertion path, admin distance
// (115), ECMP Instance handling, and diff logic are shared.
func NewInstallerV6(loc *locrib.RIB) *Installer {
	return newInstaller(loc, family.IPv6Unicast, "ipv6")
}

// newInstaller is the shared constructor for both address families.
func newInstaller(loc *locrib.RIB, fam family.Family, afi string) *Installer {
	return &Installer{
		loc:             loc,
		fam:             fam,
		afi:             afi,
		distance:        DefaultAdminDistance,
		installed:       make(map[netip.Prefix]installedRoute),
		routesInstalled: metrics.NopRegistry{}.GaugeVec("", "", nil),
	}
}

// SetRemoteSink installs the forked route sink used when the local Loc-RIB is nil
// (locrib.Default() returned nil in a forked subprocess). If loc is non-nil
// (in-process), the local RIB is always preferred and the sink stays unused.
func (in *Installer) SetRemoteSink(sink RouteSink) { in.remote = sink }

// hasSink reports whether the installer has somewhere to install routes: the local
// Loc-RIB (in-process) or a remote sink (forked). With neither, install/remove
// no-op and only the snapshot (in.installed) is maintained.
func (in *Installer) hasSink() bool { return in.loc != nil || in.remote != nil }

// insertPath sends one Path to the local Loc-RIB (preferred) or the remote sink.
func (in *Installer) insertPath(pfx netip.Prefix, p locrib.Path) {
	if in.loc != nil {
		in.loc.InsertForward(in.fam, pfx, p, nil)
		return
	}
	if in.remote != nil {
		in.remote.InsertForward(in.fam, pfx, p)
	}
}

// removePath withdraws one (source, instance) path from the local Loc-RIB or the
// remote sink.
func (in *Installer) removePath(pfx netip.Prefix, source redistevents.ProtocolID, instance uint32) {
	if in.loc != nil {
		in.loc.Remove(in.fam, pfx, source, instance)
		return
	}
	if in.remote != nil {
		in.remote.Remove(in.fam, pfx, source, instance)
	}
}

// flushRemote ships the remote sink's buffered ops (a whole Apply/RemoveAll delta
// in one RPC batch, R-1). No-op for the local Loc-RIB (writes already applied).
func (in *Installer) flushRemote() {
	if in.remote != nil {
		in.remote.Flush()
	}
}

// SetMetrics registers the ze_isis_routes_installed{level,afi} gauge owned by
// this spec (umbrella canonical Metrics table). Other ze_isis_* SPF series are
// registered on the Computer (SetMetrics there). A nil registry is ignored.
func (in *Installer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	gv := reg.GaugeVec(
		"ze_isis_routes_installed",
		"Current number of IS-IS routes installed into the Loc-RIB, by level and address family.",
		[]string{"level", "afi"},
	)
	in.metricsMu.Lock()
	in.routesInstalled = gv
	in.metricsMu.Unlock()
}

// gauge returns the current ze_isis_routes_installed handle under the read lock,
// so a concurrent SetMetrics swap is observed atomically (never a torn interface
// value). Defense in depth: see metricsMu.
func (in *Installer) gauge() metrics.GaugeVec {
	in.metricsMu.RLock()
	defer in.metricsMu.RUnlock()
	return in.routesInstalled
}

// Apply installs the route set cur, diffing against the previously installed set:
// added/changed prefixes are inserted (one locrib.Path per equal-cost next-hop,
// distinct Instance, mirroring BGP rib_bestchange.go:813); removed prefixes (and
// dropped ECMP next-hops on a shrinking set) are forward-removed. It returns the
// applied delta for the caller's logging/metrics. Safe with a nil Loc-RIB (every
// Insert/Remove is skipped, the installed set is still tracked for the snapshot).
func (in *Installer) Apply(cur []RouteEntry) RouteDelta {
	curIdx := IndexByPrefix(cur)
	prevIdx := make(map[netip.Prefix]RouteEntry, len(in.installed))
	for pfx, ir := range in.installed {
		prevIdx[pfx] = ir.entry
	}
	delta := DiffRoutes(prevIdx, curIdx)

	for _, r := range delta.Added {
		in.insert(r)
	}
	for _, r := range delta.Changed {
		in.insert(r)
	}
	for _, pfx := range delta.Removed {
		in.remove(pfx)
	}

	in.flushRemote()
	in.publishCounts()
	return delta
}

// insert pushes one prefix's equal-cost next-hops into the Loc-RIB as one
// locrib.Path each (Source = IS-IS ProtocolID, distinct Instance, AdminDistance
// = the IS-IS distance, Metric = the 32-bit-derived path cost truncated to the
// Path's uint32 Metric field). It first removes any Instances from a previous
// insert of the same prefix that are no longer present (a shrinking ECMP set), so
// the Loc-RIB path-group ends up with exactly the current next-hops. L1-over-L2
// is already resolved (route.go), so exactly one RouteEntry reaches here.
func (in *Installer) insert(r RouteEntry) {
	newInstances := make([]uint32, 0, len(r.NextHops))
	metric := metricToUint32(r.Metric)

	for i, nh := range r.NextHops {
		if !nh.Addr.IsValid() {
			continue
		}
		instance := uint32(i) //nolint:gosec // ECMP width is bounded well below 2^32
		newInstances = append(newInstances, instance)
		if !in.hasSink() {
			continue
		}
		// Mirror BGP rib_bestchange.go:813: InsertForward with a value-typed Path
		// and no ForwardHandle (IS-IS has no shared wire buffer; untyped nil per
		// the ForwardHandle nil contract). Routed through insertPath so a forked
		// installer ships to the engine over RPC instead of the local Loc-RIB.
		in.insertPath(r.Prefix, locrib.Path{
			Source:        isisProtocolID,
			Instance:      instance,
			NextHop:       nh.Addr,
			AdminDistance: in.distance,
			Metric:        metric,
		})
	}

	// Drop any Instance from the prior insert of this prefix that the new set no
	// longer uses (e.g. ECMP shrank from 2 next-hops to 1), so no stale Path
	// lingers in the path-group.
	if prev, ok := in.installed[r.Prefix]; ok && in.hasSink() {
		for _, old := range prev.instances {
			if !slices.Contains(newInstances, old) {
				in.removePath(r.Prefix, isisProtocolID, old)
			}
		}
	}

	in.installed[r.Prefix] = installedRoute{entry: r, instances: newInstances}
}

// remove forward-removes every Instance previously inserted for prefix (all
// equal-cost next-hops) and drops it from the installed set (R-4: a lost
// neighbor's prefixes leave the Loc-RIB and the kernel).
func (in *Installer) remove(pfx netip.Prefix) {
	if prev, ok := in.installed[pfx]; ok && in.hasSink() {
		for _, inst := range prev.instances {
			in.removePath(pfx, isisProtocolID, inst)
		}
	}
	delete(in.installed, pfx)
}

// RemoveAll forward-removes every installed prefix (engine shutdown / NET
// removal) so IS-IS leaves no stale routes in the Loc-RIB.
func (in *Installer) RemoveAll() {
	for pfx, prev := range in.installed {
		if in.hasSink() {
			for _, inst := range prev.instances {
				in.removePath(pfx, isisProtocolID, inst)
			}
		}
	}
	in.installed = make(map[netip.Prefix]installedRoute)
	in.flushRemote()
	in.publishCounts()
}

// Installed returns the current installed route set (a copy of the entries) for
// the `show isis route` snapshot and for tests. Order is unspecified; the
// snapshot sorts.
func (in *Installer) Installed() []RouteEntry {
	out := make([]RouteEntry, 0, len(in.installed))
	for _, ir := range in.installed {
		out = append(out, ir.entry)
	}
	return out
}

// publishCounts updates the ze_isis_routes_installed gauge with the current
// installed prefix count per level for this Installer's address family (in.afi).
// Counts are grouped by the level the winning route came from.
func (in *Installer) publishCounts() {
	byLevel := map[Level]int{}
	for _, ir := range in.installed {
		byLevel[ir.entry.Level]++
	}
	gv := in.gauge()
	// Always set both levels so a drained level reports 0 rather than going stale.
	gv.With(Level1.String(), in.afi).Set(float64(byLevel[Level1]))
	gv.With(Level2.String(), in.afi).Set(float64(byLevel[Level2]))
}

// metricToUint32 narrows the 64-bit SPF path cost to the uint32 Metric field of
// locrib.Path, clamping at the uint32 max so a (clamped-at-MaxPathMetric, which
// is < 2^32) cost is preserved exactly and any larger value saturates rather
// than wrapping. MaxPathMetric (0xFE000000) fits in uint32, so in practice no
// clamp occurs; the guard is defense in depth.
func metricToUint32(m uint64) uint32 {
	const maxU32 = uint64(^uint32(0))
	if m > maxU32 {
		return ^uint32(0)
	}
	return uint32(m)
}
