// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming via GoVPP
// Overview: fibvpp.go -- FIB VPP plugin event processing

package fibvpp

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/iface"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// vppRichRoute carries all attributes needed for full VPP FIB programming.
type vppRichRoute struct {
	Prefix netip.Prefix
	// Interface is the outgoing device name for NextHop and Weight is NextHop's
	// share of the path list ECMPPaths completes. Both come from an
	// operator-configured route; a protocol-learned route leaves them empty.
	Interface string
	Weight    uint8
	NextHop   netip.Addr
	RouteType sysribevents.RouteType
	Metric    uint32
	TableID   uint32
	ECMPPaths []sysribevents.ECMPPath
}

// vppBackend abstracts VPP FIB programming via GoVPP.
type vppBackend interface {
	addRoute(prefix netip.Prefix, nextHop netip.Addr) error
	delRoute(prefix netip.Prefix) error
	replaceRoute(prefix netip.Prefix, nextHop netip.Addr) error
	addRichRoute(r vppRichRoute) error
	delRichRoute(prefix netip.Prefix, tableID uint32) error
	replaceRichRoute(r vppRichRoute) error
	close() error
}

// govppBackend implements vppBackend using GoVPP binary API.
type govppBackend struct {
	ch      api.Channel
	tableID uint32
}

func newGovppBackend(ch api.Channel, tableID uint32) *govppBackend {
	return &govppBackend{ch: ch, tableID: tableID}
}

func (b *govppBackend) addRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	return b.routeAddDel(true, prefix, nextHop)
}

func (b *govppBackend) delRoute(prefix netip.Prefix) error {
	return b.routeAddDel(false, prefix, netip.Addr{})
}

func (b *govppBackend) replaceRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	// VPP IPRouteAddDel with IsAdd=true replaces existing route.
	return b.routeAddDel(true, prefix, nextHop)
}

func (b *govppBackend) addRichRoute(r vppRichRoute) error {
	return b.richRouteAddDel(true, r)
}

func (b *govppBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error {
	tbl := b.tableID
	if tableID != 0 {
		tbl = tableID
	}
	req := &ip.IPRouteAddDel{
		IsAdd: false,
		Route: ip.IPRoute{
			TableID: tbl,
			Prefix:  toVPPPrefix(prefix),
			NPaths:  0,
		},
	}
	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("IPRouteAddDel rich del: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("IPRouteAddDel rich del retval=%d", reply.Retval)
	}
	return nil
}

func (b *govppBackend) replaceRichRoute(r vppRichRoute) error {
	return b.richRouteAddDel(true, r)
}

func (b *govppBackend) richRouteAddDel(isAdd bool, r vppRichRoute) error {
	pathType := routeTypeToVPP(r.RouteType)
	tbl := b.tableID
	if r.TableID != 0 {
		tbl = r.TableID
	}

	var paths []fib_types.FibPath
	switch {
	case pathType == fib_types.FIB_API_PATH_TYPE_DROP ||
		pathType == fib_types.FIB_API_PATH_TYPE_ICMP_UNREACH ||
		pathType == fib_types.FIB_API_PATH_TYPE_ICMP_PROHIBIT:
		paths = []fib_types.FibPath{{Type: pathType, Weight: 1}}
	case len(r.ECMPPaths) > 0:
		paths = make([]fib_types.FibPath, 0, len(r.ECMPPaths)+1)
		if namesATarget(r.NextHop, r.Interface) {
			p, err := toFibPath(r.NextHop, r.Interface, r.Weight, r.Prefix)
			if err != nil {
				return err
			}
			p.Type = pathType
			paths = append(paths, p)
		}
		for _, ep := range r.ECMPPaths {
			if !namesATarget(ep.NextHop, ep.Interface) {
				continue
			}
			p, err := toFibPath(ep.NextHop, ep.Interface, ep.Weight, r.Prefix)
			if err != nil {
				return err
			}
			p.Type = pathType
			paths = append(paths, p)
		}
	case namesATarget(r.NextHop, r.Interface):
		p, err := toFibPath(r.NextHop, r.Interface, r.Weight, r.Prefix)
		if err != nil {
			return err
		}
		p.Type = pathType
		paths = []fib_types.FibPath{p}
	}

	if len(paths) == 0 {
		return fmt.Errorf("IPRouteAddDel rich: no paths for prefix %v", r.Prefix)
	}
	if len(paths) > 255 {
		return fmt.Errorf("IPRouteAddDel rich: %d paths exceeds uint8 limit", len(paths))
	}

	req := &ip.IPRouteAddDel{
		IsAdd: isAdd,
		Route: ip.IPRoute{
			TableID: tbl,
			Prefix:  toVPPPrefix(r.Prefix),
			NPaths:  uint8(len(paths)),
			Paths:   paths,
		},
	}
	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("IPRouteAddDel rich: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("IPRouteAddDel rich retval=%d", reply.Retval)
	}
	return nil
}

func routeTypeToVPP(rt sysribevents.RouteType) fib_types.FibPathType {
	switch rt {
	case sysribevents.RouteTypeBlackhole:
		return fib_types.FIB_API_PATH_TYPE_DROP
	case sysribevents.RouteTypeUnreachable:
		return fib_types.FIB_API_PATH_TYPE_ICMP_UNREACH
	case sysribevents.RouteTypeProhibit:
		return fib_types.FIB_API_PATH_TYPE_ICMP_PROHIBIT
	default:
		return fib_types.FIB_API_PATH_TYPE_NORMAL
	}
}

func defaultWeight() uint8 {
	return 1
}

func (b *govppBackend) close() error {
	b.ch.Close()
	return nil
}

func (b *govppBackend) routeAddDel(isAdd bool, prefix netip.Prefix, nextHop netip.Addr) error {
	req := &ip.IPRouteAddDel{
		IsAdd: isAdd,
		Route: ip.IPRoute{
			TableID: b.tableID,
			Prefix:  toVPPPrefix(prefix),
		},
	}
	// A delete names the prefix and no path, so it is not asked for a next-hop
	// and must not be refused for the want of one.
	if isAdd {
		path, err := gatewayPath(nextHop, prefix)
		if err != nil {
			return err
		}
		req.Route.NPaths = 1
		req.Route.Paths = []fib_types.FibPath{path}
	}

	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("IPRouteAddDel: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("IPRouteAddDel retval=%d", reply.Retval)
	}
	return nil
}

// toVPPPrefix converts a Go netip.Prefix to a VPP ip_types.Prefix.
func toVPPPrefix(p netip.Prefix) ip_types.Prefix {
	addr := p.Addr()
	if addr.Is4() {
		a4 := addr.As4()
		var ip4 ip_types.IP4Address
		copy(ip4[:], a4[:])
		return ip_types.Prefix{
			Address: ip_types.Address{
				Af: ip_types.ADDRESS_IP4,
				Un: ip_types.AddressUnionIP4(ip4),
			},
			Len: uint8(p.Bits()),
		}
	}
	a16 := addr.As16()
	var ip6 ip_types.IP6Address
	copy(ip6[:], a16[:])
	return ip_types.Prefix{
		Address: ip_types.Address{
			Af: ip_types.ADDRESS_IP6,
			Un: ip_types.AddressUnionIP6(ip6),
		},
		Len: uint8(p.Bits()),
	}
}

// toFibPath converts a next-hop address to a VPP fib_types.FibPath.
// namesATarget reports whether a next-hop names somewhere to forward to: a
// gateway address, an outgoing device, or both. A path that names neither is not
// a next-hop and never enters the path list.
func namesATarget(nextHop netip.Addr, ifaceName string) bool {
	return nextHop.IsValid() || ifaceName != ""
}

// toFibPath renders one FIB path: a gateway, an outgoing interface, or both,
// with the share the producer declared.
//
// dst is the ROUTE's prefix and it decides the protocol for an interface-only
// path. A zero netip.Addr reports Is4() == false, so deriving the family from an
// absent next-hop would encode an IPv4 route as PROTO_IP6 with an all-zero IPv6
// gateway (docs/architecture/static-routes.md).
//
// REQUIRES: namesATarget reports true for (nextHop, ifaceName).
func toFibPath(nextHop netip.Addr, ifaceName string, weight uint8, dst netip.Prefix) (fib_types.FibPath, error) {
	path := fib_types.FibPath{Weight: weight}
	if path.Weight == 0 {
		path.Weight = defaultWeight()
	}
	if ifaceName != "" {
		idx, err := iface.ResolveVPPIndex(ifaceName)
		if err != nil {
			return fib_types.FibPath{}, fmt.Errorf("fib/vpp: %w", err)
		}
		path.SwIfIndex = idx
	}
	switch {
	case nextHop.Is4():
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP4
		a4 := nextHop.As4()
		var ip4 ip_types.IP4Address
		copy(ip4[:], a4[:])
		path.Nh.Address = ip_types.AddressUnionIP4(ip4)
	case nextHop.Is6():
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP6
		a16 := nextHop.As16()
		var ip6 ip_types.IP6Address
		copy(ip6[:], a16[:])
		path.Nh.Address = ip_types.AddressUnionIP6(ip6)
	case dst.Addr().Is4():
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP4
	default:
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP6
	}
	return path, nil
}

// gatewayPath renders the one path of a plain route: a gateway address, no
// device, no declared weight. It is the caller that has a next-hop by
// construction; an invalid one is a programmer error the caller cannot recover
// from, so it is reported rather than programmed as a path to nowhere.
func gatewayPath(nextHop netip.Addr, dst netip.Prefix) (fib_types.FibPath, error) {
	if !namesATarget(nextHop, "") {
		return fib_types.FibPath{}, fmt.Errorf("fib/vpp: route %v names no next-hop", dst)
	}
	return toFibPath(nextHop, "", 0, dst)
}

// richRouteOp records a rich route operation for test verification.
type richRouteOp struct {
	prefix    netip.Prefix
	nextHop   netip.Addr
	routeType sysribevents.RouteType
	metric    uint32
	tableID   uint32
	ecmpPaths []sysribevents.ECMPPath
}

// richDelOp records a rich route deletion.
type richDelOp struct {
	prefix  netip.Prefix
	tableID uint32
}

// mockBackend is a test double that records calls for verification.
type mockBackend struct {
	adds         []routeOp
	dels         []netip.Prefix
	replaces     []routeOp
	richAdds     []richRouteOp
	richDels     []richDelOp
	richReplaces []richRouteOp
	closed       bool
	err          error // if set, all operations return this error
}

type routeOp struct {
	prefix  netip.Prefix
	nextHop netip.Addr
}

func (m *mockBackend) addRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	if m.err != nil {
		return m.err
	}
	m.adds = append(m.adds, routeOp{prefix, nextHop})
	return nil
}

func (m *mockBackend) delRoute(prefix netip.Prefix) error {
	if m.err != nil {
		return m.err
	}
	m.dels = append(m.dels, prefix)
	return nil
}

func (m *mockBackend) replaceRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	if m.err != nil {
		return m.err
	}
	m.replaces = append(m.replaces, routeOp{prefix, nextHop})
	return nil
}

func (m *mockBackend) addRichRoute(r vppRichRoute) error {
	if m.err != nil {
		return m.err
	}
	m.richAdds = append(m.richAdds, richRouteOp{
		prefix:    r.Prefix,
		nextHop:   r.NextHop,
		routeType: r.RouteType,
		metric:    r.Metric,
		tableID:   r.TableID,
		ecmpPaths: r.ECMPPaths,
	})
	return nil
}

func (m *mockBackend) delRichRoute(prefix netip.Prefix, tableID uint32) error {
	if m.err != nil {
		return m.err
	}
	m.richDels = append(m.richDels, richDelOp{prefix, tableID})
	return nil
}

func (m *mockBackend) replaceRichRoute(r vppRichRoute) error {
	if m.err != nil {
		return m.err
	}
	m.richReplaces = append(m.richReplaces, richRouteOp{
		prefix:    r.Prefix,
		nextHop:   r.NextHop,
		routeType: r.RouteType,
		metric:    r.Metric,
		tableID:   r.TableID,
		ecmpPaths: r.ECMPPaths,
	})
	return nil
}

func (m *mockBackend) close() error {
	m.closed = true
	return nil
}
