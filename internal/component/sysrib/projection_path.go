// Design: docs/architecture/rib/unified-locrib.md -- selected-route projection.
// Related: projection.go -- validates an entire page before applying these values.

package sysrib

import (
	"fmt"
	"net/netip"
	"reflect"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type projectionOperation struct {
	key    projectionKey
	remove bool
	value  *selectedProjectionPath
}

func (p *selectedProjection) prepare(entry *rpc.RouteViewEntry, op *projectionOperation) error {
	fam := family.Family{AFI: family.AFI(entry.AFI), SAFI: family.SAFI(entry.SAFI)}
	if _, known := p.families[fam]; !known {
		return fmt.Errorf("routing view family %d/%d is not registered; reopen the system routing view", entry.AFI, entry.SAFI)
	}
	if !entry.Prefix.IsValid() {
		return fmt.Errorf("routing view prefix is invalid; read the system routing view again")
	}
	if entry.Prefix != entry.Prefix.Masked() {
		return fmt.Errorf("routing view prefix %s contains host bits; read the system routing view again", entry.Prefix)
	}
	if entry.Prefix.Addr().Is4In6() {
		return fmt.Errorf("routing view prefix %s is IPv4-mapped; read the system routing view again", entry.Prefix)
	}
	if entry.Prefix.Addr().Zone() != "" {
		return fmt.Errorf("routing view prefix %s has a zone; read the system routing view again", entry.Prefix)
	}
	switch fam.AFI {
	case family.AFIIPv4:
		if !entry.Prefix.Addr().Is4() {
			return fmt.Errorf("routing view IPv4 prefix %s has another address family; read the system routing view again", entry.Prefix)
		}
	case family.AFIIPv6:
		if !entry.Prefix.Addr().Is6() {
			return fmt.Errorf("routing view IPv6 prefix %s has another address family; read the system routing view again", entry.Prefix)
		}
	case family.AFIL2VPN, family.AFIBGPLS:
		return fmt.Errorf("routing view family %d has no IP prefix projection; reopen the system routing view", entry.AFI)
	default: // AFI is open at the wire boundary.
		return fmt.Errorf("routing view address family %d is unknown; reopen the system routing view", entry.AFI)
	}
	op.key = projectionKey{family: fam, prefix: entry.Prefix}
	switch entry.Action {
	case rpc.RouteViewActionWithdraw:
		if !reflect.DeepEqual(entry.Path, rpc.RouteViewPath{}) {
			return fmt.Errorf("routing view withdrawal carries a path; read the system routing view again")
		}
		op.remove = true
		return nil
	case rpc.RouteViewActionAdd, rpc.RouteViewActionUpdate:
		op.value = new(selectedProjectionPath)
		return projectionPath(&entry.Path, op.value)
	case routeaction.Unspecified, routeaction.Del:
		return fmt.Errorf("routing view action %d is not a selected-route action; read the system routing view again", entry.Action)
	default: // Action is open at the wire boundary.
		return fmt.Errorf("routing view action %d is unknown; read the system routing view again", entry.Action)
	}
}

func projectionPath(wire *rpc.RouteViewPath, value *selectedProjectionPath) error {
	source, known := redistevents.ProtocolIDOf(wire.Protocol)
	if !known {
		return fmt.Errorf("routing view protocol %q is not registered; reopen the system routing view", wire.Protocol)
	}
	if err := validateProjectionPath(wire); err != nil {
		return err
	}
	value.path = locrib.Path{
		Source: source, Instance: wire.Instance, AdminDistance: wire.Distance, Metric: wire.Metric,
		NextHop: wire.NextHop, Interface: wire.Interface, OnLink: wire.OnLink, Weight: wire.Weight,
		Labels: slices.Clone(wire.Labels), RouteType: routetype.Type(wire.RouteType), SRv6SID: wire.SRv6SID,
		IsEBGP: wire.IsEBGP, IsBGP: wire.IsBGP, AIGP: wire.AIGP, AIGPPresent: wire.AIGPPresent,
		MetricRecursive: wire.MetricRecursive, Tag: wire.Tag, HasTag: wire.HasTag,
		Level: ribpolicy.ISISLevel(wire.Level), OSPFRouteType: ribpolicy.OSPFRouteType(wire.OSPFRouteType),
		BackupNextHop: wire.BackupNextHop, BackupRepairLabels: slices.Clone(wire.BackupRepairLabels),
		BackupInterface: wire.BackupInterface, BackupOnLink: wire.BackupOnLink,
	}
	value.path.ECMP = make([]locrib.NextHop, len(wire.ECMP))
	value.members = make([]projectionMemberMetadata, len(wire.ECMP))
	for i := range wire.ECMP {
		member := &wire.ECMP[i]
		memberSource := source
		if member.Protocol != "" {
			var known bool
			memberSource, known = redistevents.ProtocolIDOf(member.Protocol)
			if !known {
				return routeViewRefusal("A selected forwarding member names an unregistered protocol", "Each member requires a registered source before its FIB permission can be checked", "Register the route producer's protocol with redistevents.RegisterProtocol before inserting its routes")
			}
		}
		value.path.ECMP[i] = locrib.NextHop{Addr: member.NextHop, Interface: member.Interface, OnLink: member.OnLink, Weight: member.Weight}
		value.members[i] = projectionMemberMetadata{source: memberSource, labels: slices.Clone(member.Labels), sid: member.SRv6SID}
	}
	return nil
}

func validateProjectionPath(path *rpc.RouteViewPath) error {
	switch routetype.Type(path.RouteType) {
	case 0, routetype.Unicast, routetype.Blackhole, routetype.Unreachable, routetype.Prohibit:
	default: // Route type is open at the wire boundary.
		return fmt.Errorf("routing view route type %d is unknown; read the system routing view again", path.RouteType)
	}
	if path.Level > uint8(ribpolicy.ISISLevel2) {
		return fmt.Errorf("routing view IS-IS level %d is invalid; read the system routing view again", path.Level)
	}
	if path.OSPFRouteType > uint8(ribpolicy.OSPFNSSA2) {
		return fmt.Errorf("routing view OSPF route type %d is invalid; read the system routing view again", path.OSPFRouteType)
	}
	if err := projectionSID(path.SRv6SID); err != nil {
		return err
	}
	if err := projectionLabels(path.Labels); err != nil {
		return err
	}
	if err := projectionLabels(path.BackupRepairLabels); err != nil {
		return err
	}
	for i := range path.ECMP {
		member := &path.ECMP[i]
		if !member.NextHop.IsValid() {
			if member.Interface == "" {
				return fmt.Errorf("routing view ECMP member %d has no gateway or device; read the system routing view again", i)
			}
		}
		if err := projectionSID(member.SRv6SID); err != nil {
			return err
		}
		if err := projectionLabels(member.Labels); err != nil {
			return err
		}
	}
	return nil
}

func projectionSID(sid netip.Addr) error {
	if !sid.IsValid() {
		return nil
	}
	if !sid.Is6() {
		return fmt.Errorf("routing view Service SID %s is not IPv6; read the system routing view again", sid)
	}
	if sid.Is4In6() {
		return fmt.Errorf("routing view Service SID %s is IPv4-mapped; read the system routing view again", sid)
	}
	if sid.Zone() != "" {
		return fmt.Errorf("routing view Service SID %s has a zone; read the system routing view again", sid)
	}
	return nil
}

func projectionLabels(labels []uint32) error {
	for _, label := range labels {
		if label >= 1<<20 {
			return fmt.Errorf("routing view MPLS label %d exceeds 20 bits; read the system routing view again", label)
		}
	}
	return nil
}
