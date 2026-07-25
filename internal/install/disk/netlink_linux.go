// Design: plan/learned/1024-installer-initrd-pure-go.md -- netlink link/addr/route for installer

//go:build linux

package disk

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	ops := realNetlinkOps()
	linkUp = ops.linkUp
	dhcpAcquireApply = func(ifName string) error {
		lease, err := dhcpAcquire(ifName)
		if err != nil {
			return err
		}
		return applyLease(ops, lease)
	}
	flushIface = func(ifName string) error {
		return sysFlushIface(ops, ifName)
	}
}

type dhcpLease struct {
	IP     net.IP
	Mask   net.IPMask
	Router net.IP
	Iface  string
}

type netlinkOps struct {
	linkUp       func(ifName string) error
	addrReplace  func(ifName, cidr string) error
	routeReplace func(ifName, dst, gw string) error
	addrFlush    func(ifName string) error
	routeFlush   func(ifName string) error
}

func realNetlinkOps() *netlinkOps {
	return &netlinkOps{
		linkUp: func(ifName string) error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("link %s: %w", ifName, err)
			}
			return netlink.LinkSetUp(link)
		},
		addrReplace: func(ifName, cidr string) error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("link %s: %w", ifName, err)
			}
			addr, err := netlink.ParseAddr(cidr)
			if err != nil {
				return fmt.Errorf("parse %s: %w", cidr, err)
			}
			return netlink.AddrReplace(link, addr)
		},
		routeReplace: func(ifName, dst, gw string) error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("link %s: %w", ifName, err)
			}
			dstNet, err := netlink.ParseIPNet(dst)
			if err != nil {
				return fmt.Errorf("parse dst %s: %w", dst, err)
			}
			gwIP := net.ParseIP(gw)
			if gwIP == nil {
				return fmt.Errorf("invalid gw %s", gw)
			}
			return netlink.RouteReplace(&netlink.Route{
				LinkIndex: link.Attrs().Index,
				Dst:       dstNet,
				Gw:        gwIP,
			})
		},
		addrFlush: func(ifName string) error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("link %s: %w", ifName, err)
			}
			addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				return fmt.Errorf("addr list %s: %w", ifName, err)
			}
			for i := range addrs {
				netlink.AddrDel(link, &addrs[i]) //nolint:errcheck // best-effort flush
			}
			return nil
		},
		routeFlush: func(ifName string) error {
			link, err := netlink.LinkByName(ifName)
			if err != nil {
				return fmt.Errorf("link %s: %w", ifName, err)
			}
			routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
			if err != nil {
				return fmt.Errorf("route list %s: %w", ifName, err)
			}
			for i := range routes {
				netlink.RouteDel(&routes[i]) //nolint:errcheck // best-effort flush
			}
			return nil
		},
	}
}

func applyLease(ops *netlinkOps, lease *dhcpLease) error {
	ones, _ := lease.Mask.Size()
	var tb textbuf.Buffer
	cidr := tb.Str(lease.IP.String()).Byte('/').Int(int64(ones)).String()

	slog.Info("netlink: applying lease", "iface", lease.Iface, "cidr", cidr, "router", lease.Router)
	if err := ops.addrReplace(lease.Iface, cidr); err != nil {
		return fmt.Errorf("addr replace %s on %s: %w", cidr, lease.Iface, err)
	}

	if lease.Router != nil && !lease.Router.IsUnspecified() {
		if err := ops.routeReplace(lease.Iface, "0.0.0.0/0", lease.Router.String()); err != nil {
			return fmt.Errorf("route %s via %s: %w", "0.0.0.0/0", lease.Router, err)
		}
	}
	return nil
}

func sysFlushIface(ops *netlinkOps, ifName string) error {
	slog.Info("netlink: flushing interface", "iface", ifName)
	if err := ops.addrFlush(ifName); err != nil {
		return fmt.Errorf("flush addr %s: %w", ifName, err)
	}
	if err := ops.routeFlush(ifName); err != nil {
		return fmt.Errorf("flush route %s: %w", ifName, err)
	}
	return nil
}
