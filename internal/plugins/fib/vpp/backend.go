// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming via GoVPP
// Overview: fibvpp.go -- FIB VPP plugin event processing

package fibvpp

import (
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/fib_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
)

// vppBackend abstracts VPP FIB programming via GoVPP.
type vppBackend interface {
	addRoute(prefix netip.Prefix, nextHop netip.Addr) error
	delRoute(prefix netip.Prefix) error
	replaceRoute(prefix netip.Prefix, nextHop netip.Addr) error
	addMultiPath(prefix netip.Prefix, nextHops []netip.Addr, tableID uint32) error
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

func (b *govppBackend) addMultiPath(prefix netip.Prefix, nextHops []netip.Addr, tableID uint32) error {
	if len(nextHops) > 255 {
		return fmt.Errorf("IPRouteAddDel multipath: %d paths exceeds uint8 limit", len(nextHops))
	}
	paths := make([]fib_types.FibPath, len(nextHops))
	for i, nh := range nextHops {
		paths[i] = toFibPath(nh)
	}
	tbl := b.tableID
	if tableID != 0 {
		tbl = tableID
	}
	req := &ip.IPRouteAddDel{
		IsAdd: true,
		Route: ip.IPRoute{
			TableID: tbl,
			Prefix:  toVPPPrefix(prefix),
			NPaths:  uint8(len(paths)),
			Paths:   paths,
		},
	}
	reply := &ip.IPRouteAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("IPRouteAddDel multipath: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("IPRouteAddDel multipath retval=%d", reply.Retval)
	}
	return nil
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

// multiPathOp records a multi-path route operation.
type multiPathOp struct {
	prefix   netip.Prefix
	nextHops []netip.Addr
	tableID  uint32
}

// mockBackend is a test double that records calls for verification.
type mockBackend struct {
	adds       []routeOp
	dels       []netip.Prefix
	replaces   []routeOp
	multiPaths []multiPathOp
	closed     bool
	err        error // if set, all operations return this error
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

func (m *mockBackend) addMultiPath(prefix netip.Prefix, nextHops []netip.Addr, tableID uint32) error {
	if m.err != nil {
		return m.err
	}
	cp := make([]netip.Addr, len(nextHops))
	copy(cp, nextHops)
	m.multiPaths = append(m.multiPaths, multiPathOp{prefix, cp, tableID})
	return nil
}

func (m *mockBackend) close() error {
	m.closed = true
	return nil
}
