// Design: plan/spec-static-routes.md -- VPP static route programming via GoVPP

package staticvpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

type ActionType uint8

const (
	ActionForward   ActionType = 1
	ActionBlackhole ActionType = 2
	ActionReject    ActionType = 3
)

// Path is a single ECMP next-hop. Weight is uint8 (VPP limit, max 255);
// callers translating from the parent static package's uint16 Weight must cap.
type Path struct {
	NextHop   netip.Addr
	Weight    uint8
	SwIfIndex uint32
}

type Route struct {
	Prefix netip.Prefix
	Action ActionType
	Paths  []Path
	Metric uint32
}

type Backend struct {
	ch      api.Channel
	tableID uint32
}

func NewBackend(ch api.Channel, tableID uint32) *Backend {
	return &Backend{ch: ch, tableID: tableID}
}

func (b *Backend) ApplyRoute(r Route) error {
	return b.routeAddDel(true, r)
}

func (b *Backend) RemoveRoute(prefix netip.Prefix) error {
	return b.routeAddDel(false, Route{Prefix: prefix})
}

func (b *Backend) Close() error {
	b.ch.Close()
	return nil
}

func (b *Backend) routeAddDel(isAdd bool, r Route) error {
	req := &ip.IPRouteAddDel{
		IsAdd: isAdd,
		Route: ip.IPRoute{
			TableID: b.tableID,
			Prefix:  toVPPPrefix(r.Prefix),
		},
	}

	if !isAdd {
		req.Route.NPaths = 0
		req.Route.Paths = nil
	} else {
		paths := buildFibPaths(r)
		if len(paths) > 255 {
			return fmt.Errorf("too many paths (%d, max 255)", len(paths))
		}
		req.Route.NPaths = uint8(len(paths))
		req.Route.Paths = paths
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

func buildFibPaths(r Route) []fib_types.FibPath {
	switch r.Action {
	case ActionBlackhole:
		return []fib_types.FibPath{{
			Type: fib_types.FIB_API_PATH_TYPE_DROP,
		}}
	case ActionReject:
		return []fib_types.FibPath{{
			Type: fib_types.FIB_API_PATH_TYPE_ICMP_UNREACH,
		}}
	case ActionForward:
		paths := make([]fib_types.FibPath, len(r.Paths))
		for i, p := range r.Paths {
			paths[i] = toFibPath(p)
		}
		return paths
	}
	return nil
}

func toFibPath(p Path) fib_types.FibPath {
	fp := fib_types.FibPath{
		Weight:    p.Weight,
		SwIfIndex: p.SwIfIndex,
	}
	if fp.Weight == 0 {
		fp.Weight = 1
	}
	if p.NextHop.Is4() {
		fp.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP4
		a4 := p.NextHop.As4()
		var ip4 ip_types.IP4Address
		copy(ip4[:], a4[:])
		fp.Nh.Address = ip_types.AddressUnionIP4(ip4)
	} else {
		fp.Proto = fib_types.FIB_API_PATH_NH_PROTO_IP6
		a16 := p.NextHop.As16()
		var ip6 ip_types.IP6Address
		copy(ip6[:], a16[:])
		fp.Nh.Address = ip_types.AddressUnionIP6(ip6)
	}
	return fp
}

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
