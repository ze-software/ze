// Design: plan/learned/962-ospf-8-spf-rib.md -- OSPF route install via Loc-RIB insertion.
// OSPF does not program the FIB directly. SPF inserts locrib.Path values into the
// shared Loc-RIB, sysrib arbitrates, and fibkernel programs RTPROT_ZE routes.

package spf

import (
	"net/netip"
	"slices"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
)

var ospfProtocolID = redistevents.RegisterProtocol("ospf")

// ProtocolID returns the single OSPF Loc-RIB source identity. Redistribution in a
// later spec reuses this identity rather than registering a second source.
func ProtocolID() redistevents.ProtocolID { return ospfProtocolID }

// DefaultAdminDistance is the classical OSPF administrative distance.
const DefaultAdminDistance uint8 = 110

// Installer mirrors the BGP and IS-IS Loc-RIB insertion shape: one locrib.Path per
// equal-cost next-hop, with Source=ProtocolID and distinct Instance values.
type Installer struct {
	loc      *locrib.RIB
	fam      family.Family
	distance uint8

	installed map[netip.Prefix]installedRoute

	metricsMu       sync.RWMutex
	routesInstalled metrics.GaugeVec
}

type installedRoute struct {
	entry     RouteEntry
	instances []uint32
}

// NewInstaller constructs an IPv4-unicast OSPF installer. loc may be nil in a
// forked plugin subprocess, in which case install/remove are no-ops but snapshots
// still track the computed route table.
func NewInstaller(loc *locrib.RIB) *Installer {
	return &Installer{
		loc:             loc,
		fam:             family.IPv4Unicast,
		distance:        DefaultAdminDistance,
		installed:       make(map[netip.Prefix]installedRoute),
		routesInstalled: metrics.NopRegistry{}.GaugeVec("", "", nil),
	}
}

// SetMetrics registers ze_ospf_routes_installed{type}. A nil registry is ignored.
func (in *Installer) SetMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	gv := reg.GaugeVec(
		"ze_ospf_routes_installed",
		"Current number of OSPF routes installed into the Loc-RIB, by path type.",
		[]string{"type"},
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
		if in.loc == nil {
			continue
		}
		// Mirror BGP rib_bestchange.go: InsertForward with a value-typed Path and no
		// ForwardHandle. redistevents is not on the FIB install path.
		in.loc.InsertForward(in.fam, r.Prefix, locrib.Path{
			Source:        ospfProtocolID,
			Instance:      instance,
			NextHop:       nh.Addr,
			AdminDistance: in.distance,
			Metric:        metric,
		}, nil)
	}
	if prev, ok := in.installed[r.Prefix]; ok && in.loc != nil {
		for _, old := range prev.instances {
			if !slices.Contains(newInstances, old) {
				in.loc.Remove(in.fam, r.Prefix, ospfProtocolID, old)
			}
		}
	}
	in.installed[r.Prefix] = installedRoute{entry: r, instances: newInstances}
}

func (in *Installer) remove(pfx netip.Prefix) {
	if prev, ok := in.installed[pfx]; ok && in.loc != nil {
		for _, inst := range prev.instances {
			in.loc.Remove(in.fam, pfx, ospfProtocolID, inst)
		}
	}
	delete(in.installed, pfx)
}

// RemoveAll withdraws every OSPF path this installer inserted.
func (in *Installer) RemoveAll() {
	for pfx, prev := range in.installed {
		if in.loc != nil {
			for _, inst := range prev.instances {
				in.loc.Remove(in.fam, pfx, ospfProtocolID, inst)
			}
		}
	}
	in.installed = make(map[netip.Prefix]installedRoute)
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
	gv.With(RouteIntraArea.String()).Set(float64(byType[RouteIntraArea]))
	gv.With(RouteInterArea.String()).Set(float64(byType[RouteInterArea]))
	gv.With(RouteExternalType1.String()).Set(float64(byType[RouteExternalType1]))
	gv.With(RouteExternalType2.String()).Set(float64(byType[RouteExternalType2]))
}

func metricToUint32(m uint64) uint32 {
	const maxU32 = uint64(^uint32(0))
	if m > maxU32 {
		return ^uint32(0)
	}
	return uint32(m)
}
