// Design: docs/features/interfaces.md -- Interface management via netlink
// Overview: ifacenetlink.go -- package hub
// Related: addr_primary.go -- IPv4 primary/secondary policy applied before AddrDel
// Related: addr_primary_linux.go -- the netlink remover RemoveAddress drives

//go:build linux

package ifacenetlink

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// linkTypeBridge is the netlink link type string for bridge interfaces.
const linkTypeBridge = "bridge"

// VLAN ID range per IEEE 802.1Q.
const (
	minVLANID = 1
	maxVLANID = 4094
)

// MTU limits. 68 is the minimum for IPv4 (RFC 791). 16000 is a practical
// upper bound for common virtual/physical NICs (jumbo frames).
const (
	minMTU = 68
	maxMTU = 16000
)

func validateVLANID(id int) error {
	if id < minVLANID || id > maxVLANID {
		return fmt.Errorf("iface: vlan id %d not in [%d, %d]", id, minVLANID, maxVLANID)
	}
	return nil
}

func validateMTU(mtu int) error {
	if mtu < minMTU || mtu > maxMTU {
		return fmt.Errorf("iface: mtu %d not in [%d, %d]", mtu, minMTU, maxMTU)
	}
	return nil
}

func (b *netlinkBackend) CreateDummy(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}
	link := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create dummy %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up dummy %q: %w", name, err)
	}
	return nil
}

func (b *netlinkBackend) CreateVeth(name, peerName string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create veth %q: %w", name, err)
	}
	if err := iface.ValidateIfaceName(peerName); err != nil {
		return fmt.Errorf("iface: create veth peer %q: %w", peerName, err)
	}
	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name},
		PeerName:  peerName,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create veth %q/%q: %w", name, peerName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up veth %q: %w", name, err)
	}
	peer, err := netlink.LinkByName(peerName)
	if err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: lookup veth peer %q: %w", peerName, err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up veth peer %q: %w", peerName, err)
	}
	return nil
}

func (b *netlinkBackend) CreateBridge(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}
	link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("iface: create bridge %q: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		_ = netlink.LinkDel(link)
		return fmt.Errorf("iface: set up bridge %q: %w", name, err)
	}
	return nil
}

func (b *netlinkBackend) CreateVLAN(spec iface.VLANSpec) error {
	if err := iface.ValidateIfaceName(spec.Parent); err != nil {
		return fmt.Errorf("iface: create vlan on %q: %w", spec.Parent, err)
	}
	if err := validateVLANID(spec.VLANID); err != nil {
		return fmt.Errorf("iface: create vlan on %q: %w", spec.Parent, err)
	}
	if err := validateQoSMap(spec.IngressQoSMap); err != nil {
		return fmt.Errorf("iface: create vlan on %q: ingress-qos-map: %w", spec.Parent, err)
	}
	if err := validateQoSMap(spec.EgressQoSMap); err != nil {
		return fmt.Errorf("iface: create vlan on %q: egress-qos-map: %w", spec.Parent, err)
	}
	parent, err := netlink.LinkByName(spec.Parent)
	if err != nil {
		return fmt.Errorf("iface: create vlan: parent %q not found: %w", spec.Parent, err)
	}
	var bVlan textbuf.Buffer
	vlanName := bVlan.Reset().Str(spec.Parent).Byte('.').Int(int64(spec.VLANID)).String()
	if err := iface.ValidateIfaceName(vlanName); err != nil {
		return fmt.Errorf("iface: create vlan: composed name too long: %w", err)
	}
	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        vlanName,
			ParentIndex: parent.Attrs().Index,
		},
		VlanId: spec.VLANID,
		// Serialized as IFLA_VLAN_INGRESS_QOS / IFLA_VLAN_EGRESS_QOS inside
		// RTM_NEWLINK; nil maps emit no attribute.
		IngressQosMap: spec.IngressQoSMap,
		EgressQosMap:  spec.EgressQoSMap,
	}
	if err := netlink.LinkAdd(vlan); err != nil {
		return fmt.Errorf("iface: create vlan %q: %w", vlanName, err)
	}
	if err := netlink.LinkSetUp(vlan); err != nil {
		_ = netlink.LinkDel(vlan)
		return fmt.Errorf("iface: set up vlan %q: %w", vlanName, err)
	}
	return nil
}

// validateQoSMap checks every entry of an 802.1p QoS map is within the 3-bit
// PCP/priority range. IEEE 802.1Q: the PCP field of the TCI is 3 bits (0-7).
func validateQoSMap(m map[uint32]uint32) error {
	for from, to := range m {
		if from > 7 || to > 7 {
			return fmt.Errorf("entry %d:%d out of range (0-7)", from, to)
		}
	}
	return nil
}

func (b *netlinkBackend) UpdateVLANQoSMap(ifaceName string, ingress, egress map[uint32]uint32) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: update vlan qos %q: %w", ifaceName, err)
	}
	if err := validateQoSMap(ingress); err != nil {
		return fmt.Errorf("iface: update vlan qos %q: ingress: %w", ifaceName, err)
	}
	if err := validateQoSMap(egress); err != nil {
		return fmt.Errorf("iface: update vlan qos %q: egress: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: update vlan qos %q: not found: %w", ifaceName, err)
	}
	vlan, ok := link.(*netlink.Vlan)
	if !ok {
		return fmt.Errorf("iface: update vlan qos %q: not a VLAN device", ifaceName)
	}
	vlan.IngressQosMap = ingress
	vlan.EgressQosMap = egress
	if err := netlink.LinkModify(vlan); err != nil {
		return fmt.Errorf("iface: update vlan qos %q: %w", ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) DeleteInterface(name string) error {
	if err := iface.ValidateIfaceName(name); err != nil {
		return fmt.Errorf("iface: delete %q: %w", name, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("iface: delete %q: not found: %w", name, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("iface: delete %q: %w", name, err)
	}
	return nil
}

func (b *netlinkBackend) AddAddress(ifaceName, cidr string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: add address on %q: %w", ifaceName, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: add address %q on %q: %w", cidr, ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: add address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("iface: add address %q on %q: %w", cidr, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) RemoveAddress(ifaceName, cidr string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: remove address on %q: %w", ifaceName, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: remove address %q on %q: %w", cidr, ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: remove address on %q: not found: %w", ifaceName, err)
	}
	target, ok := toDeviceAddress(addr)
	if !ok {
		return fmt.Errorf("iface: remove address %q on %q: address is not representable", cidr, ifaceName)
	}
	// Deleting a PRIMARY IPv4 address makes the kernel delete every secondary
	// in the same subnet with it. Ze's reconcilers add the new address before
	// removing the old one, so a same-subnet renumber would otherwise take the
	// new address down too and leave the interface bare. removeAddressGuarded
	// performs the delete only after making that impossible -- addr_primary.go.
	return removeAddressGuarded(&netlinkAddrRemover{link: link, addr: addr}, ifaceName, target)
}

// AddAddressP2P installs a point-to-point address pair on ifaceName:
// localCIDR as IFA_LOCAL and peerCIDR as IFA_ADDRESS. Used by PPP NCPs
// after IPCP / IPv6CP negotiation. rtnetlink stores the pair and
// `ip addr show` renders `<local> peer <peer>`.
func (b *netlinkBackend) AddAddressP2P(ifaceName, localCIDR, peerCIDR string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: add p2p address on %q: %w", ifaceName, err)
	}
	local, err := netlink.ParseAddr(localCIDR)
	if err != nil {
		return fmt.Errorf("iface: add p2p address local %q on %q: %w", localCIDR, ifaceName, err)
	}
	peer, err := netlink.ParseAddr(peerCIDR)
	if err != nil {
		return fmt.Errorf("iface: add p2p address peer %q on %q: %w", peerCIDR, ifaceName, err)
	}
	local.Peer = peer.IPNet
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: add p2p address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrAdd(link, local); err != nil {
		return fmt.Errorf("iface: add p2p address %q peer %q on %q: %w",
			localCIDR, peerCIDR, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) ReplaceAddressWithLifetime(ifaceName, cidr string, validLft, preferredLft int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: replace address on %q: %w", ifaceName, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("iface: replace address %q on %q: %w", cidr, ifaceName, err)
	}
	addr.ValidLft = validLft
	addr.PreferedLft = preferredLft
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: replace address on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("iface: replace address %q on %q: %w", cidr, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	// rtproto.Any is a match rule, not a producer. Installing with it would put
	// an unowned route in the kernel, which the matching delete could then only
	// remove blindly -- the failure this stamp exists to remove.
	if proto == rtproto.Any {
		return fmt.Errorf("iface: add route %s on %q: protocol is rtproto.Any, which names no producer", destCIDR, ifaceName)
	}
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: add route on %q: %w", ifaceName, err)
	}
	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		return fmt.Errorf("iface: add route dest %q: %w", destCIDR, err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("iface: add route gateway %q: invalid IP", gateway)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: add route on %q: not found: %w", ifaceName, err)
	}
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
		Priority:  metric,
		Protocol:  netlink.RouteProtocol(proto),
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("iface: add route %s via %s on %q (metric %d): %w", destCIDR, gateway, ifaceName, metric, err)
	}
	return nil
}

func (b *netlinkBackend) RemoveRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: remove route on %q: %w", ifaceName, err)
	}
	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		return fmt.Errorf("iface: remove route dest %q: %w", destCIDR, err)
	}
	gw := net.ParseIP(gateway)
	if gw == nil {
		return fmt.Errorf("iface: remove route gateway %q: invalid IP", gateway)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: remove route on %q: not found: %w", ifaceName, err)
	}
	// A zero Protocol in RTM_DELROUTE is a WILDCARD: the kernel then matches on
	// destination, gateway, link and metric alone and deletes whatever route
	// carries them, whoever installed it. Stamping the delete is what stops this
	// backend removing a static route that shares that four-tuple.
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
		Priority:  metric,
		Protocol:  netlink.RouteProtocol(proto),
	}
	if err := netlink.RouteDel(route); err != nil {
		// ESRCH says one thing only: nothing matched. It carries no cause, so
		// reportRemoveRouteMiss reads the table back to find one instead of
		// naming a cause this code cannot establish. Either way the caller's
		// intent -- that route gone -- is satisfied, so this is not an error.
		if errors.Is(err, syscall.ESRCH) {
			reportRemoveRouteMiss(link, route, ifaceName, destCIDR, gateway, metric, proto)
			return nil
		}
		return fmt.Errorf("iface: remove route %s via %s metric %d on %q: %w", destCIDR, gateway, metric, ifaceName, err)
	}
	return nil
}

// reportRemoveRouteMiss says what a stamped delete that matched nothing means.
// Three outcomes are possible, and each names only what the kernel established:
//
//   - The table read FAILED. A dump of a large FIB can answer ENOBUFS, and an
//     interrupted dump answers ErrDumpInterrupted with no routes at all. Nothing
//     is known about a survivor, so the report says the read failed. Reporting a
//     failed read as an absence would hide the orphan below exactly when the
//     table is too big or too busy to read.
//   - No route carries this destination, gateway, link and metric under any
//     protocol: the ESRCH established that for this backend's own protocol, and
//     the read establishes it for every other one. That is the routine teardown
//     case, a remove of a route another path already took away, and it is
//     reported at DEBUG.
//   - A route with that key IS there, under another protocol. This backend can
//     never remove it, and its caller believes it did. A route installed by a Ze
//     version that stamped nothing lands as RTPROT_BOOT and survives every
//     stamped delete. That orphan is what AC-5 of
//     spec-fixit-route-removal-protocol-blind asks to be observable, so it is a
//     WARN naming the protocol that holds the route.
//
// The read costs one route dump of the family (routeUnderAnotherProtocol), so it
// MUST stay off any path that runs per event. It does: the link handlers in
// internal/component/iface/register.go track the metric their route sits at, and
// an event reporting a state they have already reached deletes nothing. One
// repeat still reaches a delete, and it is the repair: a handler whose own
// AddRoute failed records routeMetricUnknown, so the next event runs the full
// remove-and-add that puts the route back. What reaches this function is a
// delete the kernel refused when the caller believed the route was there.
//
// A blind delete (rtproto.Any) is silent. It matches on the four-tuple alone,
// so its ESRCH already means the route is gone and there is nothing to find.
func reportRemoveRouteMiss(link netlink.Link, route *netlink.Route, ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) {
	if proto == rtproto.Any {
		return
	}
	holder, found, err := routeUnderAnotherProtocol(link, route)
	switch {
	case err != nil:
		logger().Warn("iface: route delete matched nothing and the route table read failed; ze cannot tell whether another protocol holds this route",
			"iface", ifaceName, "dest", destCIDR, "gw", gateway, "metric", metric, "proto", int(proto), "err", err)
	case found:
		logRemoveRouteMiss(ifaceName, destCIDR, gateway, metric, proto, holder)
	default:
		logger().Debug("iface: route delete matched nothing; no route carries this destination, gateway, link and metric",
			"iface", ifaceName, "dest", destCIDR, "gw", gateway, "metric", metric, "proto", int(proto))
	}
}

// listRoutesForMiss reads the route table for routeUnderAnotherProtocol. It is a
// package var because no test can make the kernel's own dump fail, and the
// report of a FAILED read is the behavior that has to be proven.
var listRoutesForMiss = netlink.RouteList

// routeUnderAnotherProtocol reports the rtm_protocol of a route that carries the
// destination, gateway, link, table and metric of the delete that just missed,
// and a different protocol. It returns three states, never two: the route table
// could not be read (err), a survivor was read (found), or the read completed
// and no route carries that key (neither). A report never invents what it could
// not read, so a failed read never returns as an absence.
//
// The read is a dump of every route of the family: netlink.RouteList asks for
// RTM_GETROUTE with NLM_F_DUMP and filters by output interface in userspace
// (vendor/github.com/vishvananda/netlink/route_linux.go, RouteListFilteredIter).
// On a box that redistributes a full table into the kernel that is the whole
// FIB, which is why the caller runs it only on a delete the kernel refused.
func routeUnderAnotherProtocol(link netlink.Link, route *netlink.Route) (holder int, found bool, err error) {
	family := netlink.FAMILY_V6
	if route.Dst != nil && route.Dst.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}
	// A delete that names no table reaches the main table, so that is the table
	// the survivor has to be in for this delete to have been aimed at it.
	table := route.Table
	if table == 0 {
		table = unix.RT_TABLE_MAIN
	}
	routes, err := listRoutesForMiss(link, family)
	if err != nil {
		return 0, false, err
	}
	for i := range routes {
		r := &routes[i]
		if r.Protocol == route.Protocol || r.Table != table {
			continue
		}
		if r.Dst == nil || route.Dst == nil || r.Dst.String() != route.Dst.String() {
			continue
		}
		if !r.Gw.Equal(route.Gw) || r.Priority != route.Priority {
			continue
		}
		return int(r.Protocol), true, nil
	}
	return 0, false, nil
}

// logRemoveRouteMiss reports the route that survived a stamped delete. Every
// field it prints was read from the kernel or from the call that missed, holder
// included. Returning success to the caller satisfies its intent; saying nothing
// would leave a route Ze believes it removed, and the kernel still forwards on,
// invisible to the operator.
func logRemoveRouteMiss(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto, holder int) {
	logger().Warn("iface: route delete matched no route with this protocol; the kernel still holds this route under another one",
		"iface", ifaceName, "dest", destCIDR, "gw", gateway, "metric", metric,
		"proto", int(proto), "held-by", protocolName(holder))
}

func (b *netlinkBackend) ListRoutes(ifaceName, destCIDR string) ([]iface.RouteInfo, error) {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface: list routes on %q: %w", ifaceName, err)
	}
	dst, err := netlink.ParseIPNet(destCIDR)
	if err != nil {
		return nil, fmt.Errorf("iface: list routes dest %q: %w", destCIDR, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("iface: list routes on %q: not found: %w", ifaceName, err)
	}
	family := netlink.FAMILY_V6
	if dst.IP.To4() != nil {
		family = netlink.FAMILY_V4
	}
	routes, err := netlink.RouteList(link, family)
	if err != nil {
		return nil, fmt.Errorf("iface: list routes on %q: %w", ifaceName, err)
	}
	var result []iface.RouteInfo
	for i := range routes {
		r := &routes[i]
		if r.Dst == nil || r.Dst.String() != dst.String() {
			continue
		}
		gw := ""
		if r.Gw != nil {
			gw = r.Gw.String()
		}
		result = append(result, iface.RouteInfo{
			Destination: r.Dst.String(),
			Gateway:     gw,
			Metric:      r.Priority,
		})
	}
	return result, nil
}

func (b *netlinkBackend) SetMTU(ifaceName string, mtu int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set mtu on %q: %w", ifaceName, err)
	}
	if err := validateMTU(mtu); err != nil {
		return fmt.Errorf("iface: set mtu on %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set mtu on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("iface: set mtu %d on %q: %w", mtu, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) SetAdminUp(ifaceName string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set up %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set up %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("iface: set up %q: %w", ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) SetAdminDown(ifaceName string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set down %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set down %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("iface: set down %q: %w", ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) SetMACAddress(ifaceName, mac string) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return fmt.Errorf("iface: set mac on %q: %w", ifaceName, err)
	}
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("iface: set mac %q on %q: %w", mac, ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("iface: set mac on %q: not found: %w", ifaceName, err)
	}
	if err := netlink.LinkSetHardwareAddr(link, hw); err != nil {
		return fmt.Errorf("iface: set mac %q on %q: %w", mac, ifaceName, err)
	}
	return nil
}

func (b *netlinkBackend) GetMACAddress(ifaceName string) (string, error) {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return "", fmt.Errorf("iface: get mac on %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("iface: get mac on %q: not found: %w", ifaceName, err)
	}
	hw := link.Attrs().HardwareAddr
	if len(hw) == 0 {
		return "", nil
	}
	return hw.String(), nil
}

func (b *netlinkBackend) GetStats(ifaceName string) (*iface.InterfaceStats, error) {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: %w", ifaceName, err)
	}
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("iface: get stats on %q: not found: %w", ifaceName, err)
	}
	s := link.Attrs().Statistics
	if s == nil {
		return &iface.InterfaceStats{}, nil
	}
	return &iface.InterfaceStats{
		RxBytes:     s.RxBytes,
		RxPackets:   s.RxPackets,
		RxErrors:    s.RxErrors,
		RxDropped:   s.RxDropped,
		RxMulticast: s.Multicast,
		TxBytes:     s.TxBytes,
		TxPackets:   s.TxPackets,
		TxErrors:    s.TxErrors,
		TxDropped:   s.TxDropped,
	}, nil
}
