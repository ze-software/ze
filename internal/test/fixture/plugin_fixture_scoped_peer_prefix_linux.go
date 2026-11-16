//go:build linux

// Design: docs/architecture/bgp/interface-event-reactions.md -- scoped peer prefix refresh.
// Related: plugin_fixture_scoped_peer_prefix_process_linux.go -- causal listener and wire fences.
// Related: plugin_fixture_clamped_path_linux.go -- namespace ownership and child cleanup.
package fixture

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

const (
	scopedPrefixPeer       = "fe80::2%sp"
	scopedPrefixLocal      = "fe80::1%sp"
	scopedPrefixBound      = "fe80::1/128"
	scopedPrefixCover      = "fe80::100/64"
	scopedPrefixWireFile   = "scope-wire-"
	scopedPrefixStatusFile = "scope-status-"
)

// scopedPrefixDriver changes a real connected prefix without deleting the
// socket's bound address or the explicit link route that carries its TCP.
// Every child is reaped before the three owned namespaces are removed.
func scopedPrefixDriver(ctx context.Context, args []string) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		return err
	}
	if len(plan.peers) != 4 {
		return fmt.Errorf("scoped-prefix needs one recipient and three held sources")
	}
	if plan.peers[0].namespace != clampedRoleRouter {
		return fmt.Errorf("scoped-prefix first peer must be the recipient")
	}
	for _, source := range plan.peers[1:] {
		if source.namespace != clampedRoleFar {
			return fmt.Errorf("scoped-prefix sources must run in the far namespace")
		}
	}
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return err
	}
	defer orig.Close() //nolint:errcheck // cleanup after all children have been reaped
	var topology jointSubnetTopology
	defer topology.path.remove()
	if err := topology.create(plan.prefix, orig); err != nil {
		return err
	}
	if err := scopedPrefixTopology(&topology.path); err != nil {
		return err
	}
	return scopedPrefixRun(ctx, plan, &topology.path, orig)
}

// scopedPrefixTopology keeps the only covering link-local prefix on sp under
// fixture control. Without disabling automatic address generation, an unrelated
// EUI-64 /64 on either speaker interface would conceal removal of the subject.
func scopedPrefixTopology(path *clampedPath) error {
	for _, pair := range []jointSubnetLink{
		{path.sender, path.router, "sp", "p0"},
		{path.sender, path.far, "sn", "n0"},
	} {
		veth := &netlink.Veth{Name: pair.leftName, PeerName: pair.rightName, PeerNamespace: netlink.NsFd(int(pair.right.ns))}
		if err := pair.left.handle.LinkAdd(veth); err != nil {
			return fmt.Errorf("scoped-prefix create %s/%s: %w", pair.leftName, pair.rightName, err)
		}
		for _, end := range []struct {
			owner *testNetns
			name  string
		}{
			{pair.left, pair.leftName}, {pair.right, pair.rightName},
		} {
			link, err := end.owner.handle.LinkByName(end.name)
			if err != nil {
				return err
			}
			if err := end.owner.handle.LinkSetIP6AddrGenMode(link, bfdLinkLocalAddrGenModeNone); err != nil {
				return err
			}
			if err := end.owner.configureLink(end.name, 1500); err != nil {
				return err
			}
		}
	}
	for _, entry := range []struct {
		owner     *testNetns
		link      string
		addresses []string
	}{
		// The configured passive listeners must bind before the watch can act.
		{path.sender, "lo", []string{scopedPrefixSentinels[0] + "/128", scopedPrefixSentinels[1] + "/128"}},
		{path.sender, "sp", []string{scopedPrefixBound, scopedPrefixCover}},
		{path.router, "p0", []string{"fe80::2/64"}},
		{path.sender, "sn", []string{"2001:db8:5730::1/64"}},
		{path.far, "n0", []string{"2001:db8:5730::9/64", "2001:db8:5730::a/64", "2001:db8:5730::b/64"}},
	} {
		if err := entry.owner.addAddrsNoDAD(entry.link, entry.addresses...); err != nil {
			return err
		}
	}
	link, err := path.sender.handle.LinkByName("sp")
	if err != nil {
		return err
	}
	peer, err := netlink.ParseIPNet("fe80::2/128")
	if err != nil {
		return err
	}
	if err := path.sender.handle.RouteAdd(&netlink.Route{LinkIndex: link.Attrs().Index, Dst: peer, Scope: netlink.SCOPE_LINK}); err != nil {
		return fmt.Errorf("scoped-prefix retain peer TCP host route: %w", err)
	}
	if err := scopedPrefixSentinelState(path.sender, true); err != nil {
		return err
	}
	return scopedPrefixKernelState(path.sender, true)
}

// scopedPrefixRemoveSentinels removes the startup bind addresses. The caller
// MUST wait for the daemon to observe both removals before reusing either
// address as a post-refresh listener fence.
func scopedPrefixRemoveSentinels(sender *testNetns) error {
	link, err := sender.handle.LinkByName("lo")
	if err != nil {
		return err
	}
	for _, sentinel := range scopedPrefixSentinels {
		address, err := netlink.ParseAddr(sentinel + "/128")
		if err != nil {
			return err
		}
		if err := sender.handle.AddrDel(link, address); err != nil {
			return err
		}
	}
	return scopedPrefixSentinelState(sender, false)
}

// scopedPrefixSentinelState reads back the exact loopback bind addresses. A
// missing or tentative startup address must fail before the daemon starts.
func scopedPrefixSentinelState(sender *testNetns, present bool) error {
	link, err := sender.handle.LinkByName("lo")
	if err != nil {
		return err
	}
	addresses, err := sender.handle.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		return err
	}
	for _, sentinel := range scopedPrefixSentinels {
		found := false
		for _, address := range addresses {
			addr, ok := netip.AddrFromSlice(address.IP)
			if !ok {
				return fmt.Errorf("scoped-prefix unreadable sentinel address %v", address)
			}
			if addr.Unmap() != netip.MustParseAddr(sentinel) {
				continue
			}
			ones, bits := address.Mask.Size()
			if ones != 128 {
				return fmt.Errorf("scoped-prefix sentinel %s has prefix length %d", sentinel, ones)
			}
			if bits != 128 {
				return fmt.Errorf("scoped-prefix sentinel %s has address width %d", sentinel, bits)
			}
			if address.Flags&unix.IFA_F_TENTATIVE != 0 {
				return fmt.Errorf("scoped-prefix sentinel %s is tentative", sentinel)
			}
			found = true
		}
		if found != present {
			return fmt.Errorf("scoped-prefix sentinel %s present=%t, want %t", sentinel, found, present)
		}
	}
	return nil
}

// scopedPrefixChange reads the changed kernel state before adding the fresh
// listener sentinel. addressSync.apply refreshes link scope before it starts
// that sentinel's listener, which supplies the later daemon-side causal fence.
func scopedPrefixChange(sender *testNetns, present bool, sentinel string) error {
	link, err := sender.handle.LinkByName("sp")
	if err != nil {
		return err
	}
	address, err := netlink.ParseAddr(scopedPrefixCover)
	if err != nil {
		return err
	}
	if present {
		if err := sender.addAddrsNoDAD("sp", scopedPrefixCover); err != nil {
			return err
		}
	} else {
		if err := sender.handle.AddrDel(link, address); err != nil {
			return err
		}
	}
	if err := scopedPrefixKernelState(sender, present); err != nil {
		return err
	}
	return sender.addAddrsNoDAD("lo", sentinel+"/128")
}

// scopedPrefixKernelState verifies both the intended address-prefix condition
// and the independent resources that keep the original scoped TCP alive.
func scopedPrefixKernelState(sender *testNetns, present bool) error {
	addresses, err := sender.handle.AddrList(nil, netlink.FAMILY_V6)
	if err != nil {
		return err
	}
	bound := false
	covered := false
	peer := netip.MustParseAddr("fe80::2")
	for _, address := range addresses {
		addr, ok := netip.AddrFromSlice(address.IP)
		if !ok {
			return fmt.Errorf("scoped-prefix unreadable kernel address %v", address)
		}
		ones, _ := address.Mask.Size()
		prefix := netip.PrefixFrom(addr.Unmap(), ones)
		if prefix == netip.MustParsePrefix(scopedPrefixBound) {
			bound = true
		}
		if prefix.Contains(peer) {
			covered = true
		}
	}
	if !bound {
		return fmt.Errorf("scoped-prefix lost the unchanged bound address %s", scopedPrefixBound)
	}
	if covered != present {
		return fmt.Errorf("scoped-prefix kernel membership=%t, want %t", covered, present)
	}
	link, err := sender.handle.LinkByName("sp")
	if err != nil {
		return err
	}
	destination := &net.IPNet{IP: net.ParseIP("fe80::2"), Mask: net.CIDRMask(128, 128)}
	routes, err := sender.handle.RouteListFiltered(netlink.FAMILY_V6, &netlink.Route{Dst: destination}, netlink.RT_FILTER_DST)
	if err != nil {
		return err
	}
	// IPv6 route dumps report UNIVERSE even when RouteAdd requests LINK.
	// The exact host route and absent next-hop alternatives keep TCP on sp.
	for index := range routes {
		route := &routes[index]
		if route.Dst == nil {
			continue
		}
		if !route.Dst.IP.Equal(destination.IP) {
			continue
		}
		ones, bits := route.Dst.Mask.Size()
		if ones != 128 {
			continue
		}
		if bits != 128 {
			continue
		}
		if route.LinkIndex != link.Attrs().Index {
			continue
		}
		if route.Family != netlink.FAMILY_V6 {
			continue
		}
		if route.Table != unix.RT_TABLE_MAIN {
			continue
		}
		if route.Type != unix.RTN_UNICAST {
			continue
		}
		if route.Protocol != unix.RTPROT_BOOT {
			continue
		}
		if route.Scope != netlink.SCOPE_UNIVERSE {
			continue
		}
		if len(route.Gw) != 0 {
			continue
		}
		if len(route.MultiPath) != 0 {
			continue
		}
		if route.Via != nil {
			continue
		}
		if route.NewDst != nil {
			continue
		}
		if route.Encap != nil {
			continue
		}
		if route.MPLSDst != nil {
			continue
		}
		return nil
	}
	return fmt.Errorf("scoped-prefix lost the explicit peer TCP host route: expected sp index=%d destination=%s scope=SCOPE_UNIVERSE(%d); filtered routes=%#v; readable routes=%v", link.Attrs().Index, destination, int(netlink.SCOPE_UNIVERSE), routes, routes)
}
