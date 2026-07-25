// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming via GoVPP
// Overview: fibvpp.go -- FIB VPP plugin event processing

package fibvpp

import (
	"fmt"
	"net/netip"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// vppRichRoute carries all attributes needed for full VPP FIB programming.
type vppRichRoute struct {
	Prefix    netip.Prefix
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
		if r.NextHop.IsValid() {
			p := toFibPath(r.NextHop)
			p.Type = pathType
			p.Weight = defaultWeight()
			paths = append(paths, p)
		}
		for _, ep := range r.ECMPPaths {
			if !ep.NextHop.IsValid() {
				continue
			}
			p := toFibPath(ep.NextHop)
			p.Type = pathType
			if ep.Weight > 0 {
				p.Weight = ep.Weight
			} else {
				p.Weight = defaultWeight()
			}
			paths = append(paths, p)
		}
	case r.NextHop.IsValid():
		p := toFibPath(r.NextHop)
		p.Type = pathType
		p.Weight = defaultWeight()
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
			NPaths:  1,
			Paths:   []fib_types.FibPath{toFibPath(nextHop)},
		},
	}
	if !isAdd {
		// For delete, no paths needed, just the prefix.
		req.Route.NPaths = 0
		req.Route.Paths = nil
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
func toFibPath(nextHop netip.Addr) fib_types.FibPath {
	path := fib_types.FibPath{
		Weight: 1,
	}
	if nextHop.Is4() {
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP4
		a4 := nextHop.As4()
		var ip4 ip_types.IP4Address
		copy(ip4[:], a4[:])
		path.Nh.Address = ip_types.AddressUnionIP4(ip4)
	} else {
		path.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP6
		a16 := nextHop.As16()
		var ip6 ip_types.IP6Address
		copy(ip6[:], a16[:])
		path.Nh.Address = ip_types.AddressUnionIP6(ip6)
	}
	return path
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
