// Design: docs/architecture/rib/unified-locrib.md -- selected value ownership.
// Related: route_view_feed.go -- scans and journals retain only these values.

package sysrib

import (
	"context"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func routeViewScan(ctx context.Context, rib *locrib.RIB, visit routeViewVisitor) error {
	for _, fam := range rib.Families() {
		var scanErr error
		rib.Iterate(fam, func(prefix netip.Prefix, group locrib.PathGroup) bool {
			if err := ctx.Err(); err != nil {
				scanErr = err
				return false
			}
			var entry rpc.RouteViewEntry
			if err := routeViewCopyEntry(fam, prefix, &group, &entry); err != nil {
				scanErr = err
				return false
			}
			if entry.Action == rpc.RouteViewActionWithdraw {
				return true
			}
			scanErr = visit(&entry)
			return scanErr == nil
		})
		if scanErr != nil {
			return scanErr
		}
	}
	return ctx.Err()
}

func routeViewInspect(rib *locrib.RIB, key overflowKey) (rpc.RouteViewEntry, error) {
	entry := rpc.RouteViewEntry{AFI: uint16(key.fam.AFI), SAFI: uint8(key.fam.SAFI), Prefix: key.prefix, Action: rpc.RouteViewActionWithdraw}
	var copyErr error
	rib.Inspect(key.fam, key.prefix, func(group locrib.PathGroup) {
		copyErr = routeViewCopyEntry(key.fam, key.prefix, &group, &entry)
	})
	return entry, copyErr
}

func routeViewCopyEntry(fam family.Family, prefix netip.Prefix, group *locrib.PathGroup, entry *rpc.RouteViewEntry) error {
	*entry = rpc.RouteViewEntry{AFI: uint16(fam.AFI), SAFI: uint8(fam.SAFI), Prefix: prefix, Action: rpc.RouteViewActionWithdraw}
	if group.Best < 0 {
		return nil
	}
	if group.Best >= len(group.Paths) {
		return nil
	}
	best := &group.Paths[group.Best]
	protocol := redistevents.ProtocolName(best.Source)
	if protocol == "" {
		return routeViewRefusal("A selected route names an unregistered protocol", "Protocol identities must have a registered name before crossing the process boundary", "Register the route producer's protocol with redistevents.RegisterProtocol before inserting its routes")
	}
	entry.Action = rpc.RouteViewActionAdd
	entry.Path = rpc.RouteViewPath{
		Protocol: protocol, Instance: best.Instance, Distance: best.AdminDistance, Metric: best.Metric,
		NextHop: best.NextHop, Interface: best.Interface, OnLink: best.OnLink, Weight: best.Weight,
		Labels: slices.Clone(best.Labels), RouteType: uint8(best.RouteType), SRv6SID: best.SRv6SID,
		IsEBGP: best.IsEBGP, IsBGP: best.IsBGP, AIGP: best.AIGP, AIGPPresent: best.AIGPPresent,
		MetricRecursive: best.MetricRecursive, Tag: best.Tag, HasTag: best.HasTag,
		Level: uint8(best.Level), OSPFRouteType: uint8(best.OSPFRouteType),
		BackupNextHop: best.BackupNextHop, BackupRepairLabels: slices.Clone(best.BackupRepairLabels),
		BackupInterface: best.BackupInterface, BackupOnLink: best.BackupOnLink,
	}
	ecmp := group.ForwardingMembers(*best)
	if len(ecmp) > 0 {
		entry.Path.ECMP = make([]rpc.RouteViewNextHop, len(ecmp))
		for i := range ecmp {
			hop := &ecmp[i]
			entry.Path.ECMP[i] = rpc.RouteViewNextHop{NextHop: hop.Addr, Interface: hop.Interface, OnLink: hop.OnLink, Weight: hop.Weight, Labels: slices.Clone(hop.Labels), SRv6SID: hop.SRv6SID}
			if hop.Source != best.Source {
				protocol := redistevents.ProtocolName(hop.Source)
				if protocol == "" {
					return routeViewRefusal("A selected forwarding member names an unregistered protocol", "Forwarding permissions require the member's registered owner", "Register the route producer's protocol with redistevents.RegisterProtocol before inserting its routes")
				}
				entry.Path.ECMP[i].Protocol = protocol
			}
		}
	}
	return nil
}

func routeViewEntryKey(entry *rpc.RouteViewEntry) overflowKey {
	return overflowKey{fam: family.Family{AFI: family.AFI(entry.AFI), SAFI: family.SAFI(entry.SAFI)}, prefix: entry.Prefix}
}

func routeViewCompareKeys(a, b overflowKey) int {
	if family.FamilyLess(a.fam, b.fam) {
		return -1
	}
	if family.FamilyLess(b.fam, a.fam) {
		return 1
	}
	if compared := a.prefix.Addr().Compare(b.prefix.Addr()); compared != 0 {
		return compared
	}
	return a.prefix.Bits() - b.prefix.Bits()
}

func routeViewEntryBytes(entry *rpc.RouteViewEntry) int {
	// Conservative retained-value accounting includes fixed headers, text, and
	// every owned slice. Snapshot storage is one table bounded by its authority;
	// only the delta journal additionally has this fixed byte ceiling.
	path := &entry.Path
	bytes := 512 + len(path.Protocol) + len(path.Interface) + len(path.BackupInterface)
	bytes += 4 * (len(path.Labels) + len(path.BackupRepairLabels))
	for i := range path.ECMP {
		hop := &path.ECMP[i]
		bytes += 128 + len(hop.Protocol) + len(hop.Interface) + 4*len(hop.Labels)
	}
	return bytes
}
