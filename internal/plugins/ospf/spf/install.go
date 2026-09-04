// Design: docs/architecture/ospf/ospf-8-spf-rib.md -- OSPF route install via Loc-RIB insertion.
// OSPF does not program the FIB directly. SPF inserts locrib.Path values into the
// shared Loc-RIB, sysrib arbitrates, and fibkernel programs RTPROT_ZE routes.

package spf

import (
	"net/netip"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

var ospfProtocolID = redistevents.RegisterProtocol("ospf")

// ProtocolID returns the single OSPF Loc-RIB source identity. Redistribution in a
// later spec reuses this identity rather than registering a second source.
func ProtocolID() redistevents.ProtocolID { return ospfProtocolID }

// DefaultAdminDistance is the classical OSPF administrative distance.
const DefaultAdminDistance uint8 = 110

// RouteSink receives Loc-RIB install/remove operations when the installer has no
// local Loc-RIB (a forked subprocess, where locrib.Default() returns nil). The
// forked wiring installs one via SetRemoteSink; it ships each op to the engine
// over RPC (internal/core/rib/routeinstall). In-process (loc != nil) the local RIB
// is always preferred and the sink is unused.
type RouteSink interface {
	InsertForward(fam family.Family, prefix netip.Prefix, p locrib.Path)
	Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32)
	// Flush sends buffered ops to the engine; the installer calls it once per
	// Apply/RemoveAll so a whole delta travels in one RPC batch (R-1).
	Flush()
}

// Installer mirrors the BGP and IS-IS Loc-RIB insertion shape: one locrib.Path per
// equal-cost next-hop, with Source=ProtocolID and distinct Instance values.
type Installer struct {
	loc      *locrib.RIB
	remote   RouteSink
	fam      family.Family
	afLabel  string
	distance uint8

	installed map[netip.Prefix]installedRoute

	// suppress, when set and true, makes Apply and RemoveAll no-ops. OSPF Graceful Restart
	// (RFC 3623 sec 2/2.1) sets it so the restarting router neither churns routes while in
	// restart nor withdraws them on the graceful stop, retaining the pre-restart FIB.
	suppress func() bool

	metricsMu       sync.RWMutex
	routesInstalled metrics.GaugeVec
}

// setSuppress installs the graceful-restart install-suppression predicate (RFC 3623). A nil
// predicate (the default) never suppresses.
func (in *Installer) setSuppress(fn func() bool) { in.suppress = fn }

// suppressed reports whether route install/removal is currently suppressed.
func (in *Installer) suppressed() bool { return in.suppress != nil && in.suppress() }

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

type installedRoute struct {
	entry     RouteEntry
	instances []uint32
}

// NewInstaller constructs an IPv4-unicast OSPF installer (the OSPFv2 family). loc may be
// nil in a forked plugin subprocess; the forked wiring then calls SetRemoteSink so
// install/remove ship to the engine over RPC (internal/core/rib/routeinstall). With neither
// a local Loc-RIB nor a remote sink, install/remove are no-ops but snapshots still track
// the computed route table.
func NewInstaller(loc *locrib.RIB) *Installer {
	return NewInstallerFamily(loc, family.IPv4Unicast)
}

// NewInstallerFamily constructs an OSPF installer that inserts into fam's Loc-RIB. RFC 5838
// maps each OSPFv3 address family to its own Loc-RIB family (IPv6-unicast, IPv4-unicast,
// IPv6-multicast, IPv4-multicast); the family-parameterised constructor routes each AF's
// SPF routes to the right family and fixes the IPv6-base hardcode to family.IPv4Unicast.
func NewInstallerFamily(loc *locrib.RIB, fam family.Family) *Installer {
	return &Installer{
		loc:             loc,
		fam:             fam,
		afLabel:         famAFLabel(fam),
		distance:        DefaultAdminDistance,
		installed:       make(map[netip.Prefix]installedRoute),
		routesInstalled: metrics.NopRegistry{}.GaugeVec("", "", nil),
	}
}

// famAFLabel renders a family as the "<afi>-<safi>" label used by the per-AF metric
// series (e.g. "ipv6-unicast", "ipv4-unicast"), matching the RFC 5838 AF names.
func famAFLabel(fam family.Family) string {
	buf := make([]byte, 0, 16)
	buf = fam.AFI.AppendTo(buf)
	buf = append(buf, '-')
	buf = fam.SAFI.AppendTo(buf)
	return string(buf)
}

// SetMetrics registers ze_ospf_routes_installed{type,af}. A nil registry is ignored. The
// `af` label (RFC 5838) distinguishes per-address-family install counts on the shared series.
func (in *Installer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	gv := reg.GaugeVec(
		"ze_ospf_routes_installed",
		"Current number of OSPF routes installed into the Loc-RIB, by path type and address family.",
		[]string{"type", "af"},
	)
	in.metricsMu.Lock()
	in.routesInstalled = gv
	in.metricsMu.Unlock()
}

func (in *Installer) gauge() metrics.GaugeVec {
	in.metricsMu.RLock()
	defer in.metricsMu.RUnlock()
	return in.routesInstalled
}

// Apply diffs cur against the previous installed set, inserts added/changed
// routes, and forward-removes lost prefixes or shrunk ECMP instances.
func (in *Installer) Apply(cur []RouteEntry) RouteDelta {
	// Graceful restart: skip route programming while suppressed so the pre-restart FIB is
	// retained without churn (RFC 3623 sec 2). The installed set is left unchanged.
	if in.suppressed() {
		return RouteDelta{}
	}
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

func (in *Installer) insert(r RouteEntry) {
	newInstances := make([]uint32, 0, len(r.NextHops))
	metric := metricToUint32(r.Metric)
	for i, nh := range r.NextHops {
		if !nh.Addr.IsValid() {
			continue
		}
		instance := uint32(i) //nolint:gosec // ECMP width is capped far below uint32.
		newInstances = append(newInstances, instance)
		if !in.hasSink() {
			continue
		}
		// RFC 5286 / TI-LFA: carry this primary's pre-computed backup (address +
		// optional SR repair label stack) as carry-through metadata on the Path. It
		// is excluded from arbitration and installed by the FIB as a link-down/backup
		// next-hop. Shared, never mutated, mirroring the primary Labels contract.
		var backupNH netip.Addr
		var repair []uint32
		if i < len(r.Backups) && r.Backups[i].Valid() {
			backupNH = r.Backups[i].NextHop
			repair = r.Backups[i].RepairLabels
		}
		// Mirror BGP rib_bestchange.go: InsertForward with a value-typed Path and no
		// ForwardHandle. redistevents is not on the FIB install path.
		in.insertPath(r.Prefix, locrib.Path{
			Source:   ospfProtocolID,
			Instance: instance,
			NextHop:  nh.Addr,
			// The DECLARATION decides. locrib.selectBest ranks paths on what is
			// stamped here and runs before sysrib sees the route, so
			// `rib { distance { ospf N } }` has to reach this line to change
			// cross-protocol selection. in.distance is the bootstrap value,
			// reachable only before the first configure. Read HERE rather than
			// at construction so a reload takes effect.
			AdminDistance:      ribdistance.OrDefault("ospf", in.distance),
			Metric:             metric,
			BackupNextHop:      backupNH,
			BackupRepairLabels: repair,
		})
	}
	if prev, ok := in.installed[r.Prefix]; ok && in.hasSink() {
		for _, old := range prev.instances {
			if !slices.Contains(newInstances, old) {
				in.removePath(r.Prefix, ospfProtocolID, old)
			}
		}
	}
	in.installed[r.Prefix] = installedRoute{entry: r, instances: newInstances}
}

func (in *Installer) remove(pfx netip.Prefix) {
	if prev, ok := in.installed[pfx]; ok && in.hasSink() {
		for _, inst := range prev.instances {
			in.removePath(pfx, ospfProtocolID, inst)
		}
	}
	delete(in.installed, pfx)
}

// RemoveAll withdraws every OSPF path this installer inserted, unless graceful-restart
// suppression is active (RFC 3623 sec 2.1: a graceful stop must NOT withdraw routes -- the
// FIB is retained across the restart).
func (in *Installer) RemoveAll() {
	if in.suppressed() {
		return
	}
	for pfx, prev := range in.installed {
		if in.hasSink() {
			for _, inst := range prev.instances {
				in.removePath(pfx, ospfProtocolID, inst)
			}
		}
	}
	in.installed = make(map[netip.Prefix]installedRoute)
	in.flushRemote()
	in.publishCounts()
}

// Installed returns a copy of the current installed route set.
func (in *Installer) Installed() []RouteEntry {
	out := make([]RouteEntry, 0, len(in.installed))
	for _, ir := range in.installed {
		out = append(out, ir.entry)
	}
	return out
}

func (in *Installer) publishCounts() {
	byType := map[RouteType]int{}
	for _, ir := range in.installed {
		byType[ir.entry.Type]++
	}
	gv := in.gauge()
	gv.With(RouteIntraArea.String(), in.afLabel).Set(float64(byType[RouteIntraArea]))
	gv.With(RouteInterArea.String(), in.afLabel).Set(float64(byType[RouteInterArea]))
	gv.With(RouteExternalType1.String(), in.afLabel).Set(float64(byType[RouteExternalType1]))
	gv.With(RouteExternalType2.String(), in.afLabel).Set(float64(byType[RouteExternalType2]))
}

func metricToUint32(m uint64) uint32 {
	const maxU32 = uint64(^uint32(0))
	if m > maxU32 {
		return ^uint32(0)
	}
	return uint32(m)
}
