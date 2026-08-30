//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

func init() {
	Register("vrrp/vrrp-instance-up-setup", vrrpInstanceSetup)
	Register("vrrp/vrrp-instance-up-driver", vrrpInstanceDriver)
	Register("vrrp/vrrp-macvlan-parent-selector-setup", vrrpSelectorSetup)
	Register("vrrp/vrrp-macvlan-parent-selector-driver", vrrpSelectorDriver)
	Register("vrrp/vrrp-accept-mode-setup", vrrpAcceptModeSetup)
	Register("vrrp/vrrp-accept-mode-driver", vrrpAcceptModeDriver)
}

func addRoutingLink(link netlink.Link) error {
	if err := netlink.LinkAdd(link); err != nil && !errors.Is(err, syscall.EEXIST) {
		return err
	}
	return nil
}

func addRoutingAddress(link netlink.Link, cidr string) error {
	address, err := netlink.ParseAddr(cidr)
	if err != nil {
		return err
	}
	if err := netlink.AddrAdd(link, address); err != nil && !errors.Is(err, syscall.EEXIST) {
		return err
	}
	return nil
}

func vrrpInstanceSetup(context.Context, []string) error {
	if err := addRoutingLink(&netlink.Veth{Name: "zept0", PeerName: "zept0p"}); err != nil {
		return fmt.Errorf("add zept0 veth: %w", err)
	}
	parent, err := netlink.LinkByName("zept0")
	if err != nil {
		return fmt.Errorf("find zept0: %w", err)
	}
	if err := addRoutingAddress(parent, "192.0.2.251/24"); err != nil {
		return fmt.Errorf("address zept0: %w", err)
	}
	if err := netlink.LinkSetUp(parent); err != nil {
		return fmt.Errorf("bring zept0 up: %w", err)
	}
	peer, err := netlink.LinkByName("zept0p")
	if err != nil {
		return fmt.Errorf("find zept0p: %w", err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		return fmt.Errorf("bring zept0p up: %w", err)
	}
	return nil
}

func routingDaemonPID(ctx context.Context) (int, error) {
	if !Poll(ctx, 200, 50*time.Millisecond, func() bool {
		_, pidErr := os.Stat("daemon.pid")
		_, readyErr := os.Stat("daemon.ready")
		return pidErr == nil && readyErr == nil
	}) {
		return 0, fmt.Errorf("daemon readiness files missing")
	}
	contents, err := os.ReadFile("daemon.pid")
	if err != nil {
		return 0, fmt.Errorf("read daemon.pid: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		return 0, fmt.Errorf("parse daemon.pid: %w", err)
	}
	return pid, nil
}

// errLinkAbsent says no link carries the requested hardware address. Absence is
// an expected state here: a fixture waits for a device to appear, and another
// waits for it to go.
var errLinkAbsent = errors.New("no link carries that hardware address")

func routingLinkByMAC(mac string) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if strings.EqualFold(link.Attrs().HardwareAddr.String(), mac) {
			return link, nil
		}
	}
	return nil, errLinkAbsent
}

func routingIPv4Addresses(link netlink.Link) (map[string]bool, error) {
	addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(addresses))
	for _, address := range addresses {
		result[address.IP.String()] = true
	}
	return result, nil
}

func routingAnyAddress(addresses map[string]bool) (bool, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return false, err
	}
	for _, link := range links {
		found, err := routingIPv4Addresses(link)
		if err != nil {
			return false, err
		}
		for address := range addresses {
			if found[address] {
				return true, nil
			}
		}
	}
	return false, nil
}

func vrrpInstanceDriver(ctx context.Context, _ []string) error {
	const virtualMAC = "00:00:5e:00:01:0a"
	vipSet := map[string]bool{addrTestNet1First: true, addrTestNet1Second: true}
	pid, err := routingDaemonPID(ctx)
	if err != nil {
		return err
	}
	var macvlan netlink.Link
	var lookupErr error
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		macvlan, lookupErr = routingLinkByMAC(virtualMAC)
		return lookupErr == nil && macvlan != nil
	}) {
		if lookupErr != nil && !errors.Is(lookupErr, errLinkAbsent) {
			return fmt.Errorf("list links: %w", lookupErr)
		}
		return fmt.Errorf("macvlan with virtual MAC %s never appeared", virtualMAC)
	}
	if !strings.HasPrefix(macvlan.Attrs().Name, "zv4-") {
		return fmt.Errorf("macvlan name %q does not carry the zv4- owned-device prefix", macvlan.Attrs().Name)
	}
	fmt.Printf("MACVLAN-UP %s %s\n", macvlan.Attrs().Name, virtualMAC)

	var installed map[string]bool
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		installed, lookupErr = routingIPv4Addresses(macvlan)
		return lookupErr == nil && installed["192.0.2.1"] && installed["192.0.2.2"] && lenRoutingVIPs(installed, vipSet) == 2
	}) {
		if lookupErr != nil {
			return fmt.Errorf("read VIPs from %s: %w", macvlan.Attrs().Name, lookupErr)
		}
		return fmt.Errorf("both VIPs [192.0.2.1 192.0.2.2] never installed on %s (got %#v)", macvlan.Attrs().Name, installed)
	}
	fmt.Println("VIPS-INSTALLED 192.0.2.1,192.0.2.2")

	const emptyConfig = "interface {\n\tbackend netlink;\n\tethernet zept0 {\n\t\tunit 0 {\n\t\t\tipv4 {\n\t\t\t\taddress [ 192.0.2.251/24 ];\n\t\t\t}\n\t\t}\n\t}\n}\n"
	if err := os.WriteFile("ze-bgp.conf", []byte(emptyConfig), 0o600); err != nil {
		return fmt.Errorf("rewrite config: %w", err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon: %w", err)
	}
	if err := process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload daemon: %w", err)
	}
	var vipPresent bool
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		macvlan, lookupErr = routingLinkByMAC(virtualMAC)
		if lookupErr != nil && !errors.Is(lookupErr, errLinkAbsent) {
			return false
		}
		vipPresent, lookupErr = routingAnyAddress(vipSet)
		return lookupErr == nil && macvlan == nil && !vipPresent
	}) {
		if lookupErr != nil {
			return fmt.Errorf("read teardown state: %w", lookupErr)
		}
		return fmt.Errorf("teardown incomplete after group removal: macvlan=%v vip=%t", macvlan, vipPresent)
	}
	fmt.Println("TEARDOWN-COMPLETE")
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop daemon: %w", err)
	}
	Poll(ctx, 100, 50*time.Millisecond, func() bool { return process.Signal(syscall.Signal(0)) != nil })
	return nil
}

func lenRoutingVIPs(installed, wanted map[string]bool) int {
	count := 0
	for address := range wanted {
		if installed[address] {
			count++
		}
	}
	return count
}

func vrrpSelectorSetup(context.Context, []string) error {
	if err := addRoutingLink(&netlink.Veth{Name: "zevrsel0", PeerName: "zevrsel0p"}); err != nil {
		return fmt.Errorf("add zevrsel0 veth: %w", err)
	}
	selected, err := netlink.LinkByName("zevrsel0")
	if err != nil {
		return fmt.Errorf("find zevrsel0: %w", err)
	}
	mac, err := net.ParseMAC("02:00:00:00:be:22")
	if err != nil {
		return err
	}
	if err := netlink.LinkSetHardwareAddr(selected, mac); err != nil {
		return fmt.Errorf("set zevrsel0 MAC: %w", err)
	}
	if err := addRoutingAddress(selected, "203.0.113.209/28"); err != nil {
		return fmt.Errorf("address zevrsel0: %w", err)
	}
	if err := netlink.LinkSetUp(selected); err != nil {
		return fmt.Errorf("bring zevrsel0 up: %w", err)
	}
	peer, err := netlink.LinkByName("zevrsel0p")
	if err != nil {
		return fmt.Errorf("find zevrsel0p: %w", err)
	}
	if err := netlink.LinkSetUp(peer); err != nil {
		return fmt.Errorf("bring zevrsel0p up: %w", err)
	}
	if err := addRoutingLink(&netlink.Dummy{Name: "zevrwan"}); err != nil {
		return fmt.Errorf("add zevrwan decoy: %w", err)
	}
	decoy, err := netlink.LinkByName("zevrwan")
	if err != nil {
		return fmt.Errorf("find zevrwan: %w", err)
	}
	if err := netlink.LinkSetUp(decoy); err != nil {
		return fmt.Errorf("bring zevrwan up: %w", err)
	}
	return nil
}

func vrrpSelectorDriver(ctx context.Context, _ []string) error {
	const virtualMAC = "00:00:5e:00:01:28"
	if _, err := routingDaemonPID(ctx); err != nil {
		return err
	}
	selected, err := netlink.LinkByName("zevrsel0")
	if err != nil {
		return fmt.Errorf("setup did not build selected device: %w", err)
	}
	decoy, err := netlink.LinkByName("zevrwan")
	if err != nil {
		return fmt.Errorf("setup did not build decoy: %w", err)
	}
	var macvlan netlink.Link
	var lookupErr error
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool {
		macvlan, lookupErr = routingLinkByMAC(virtualMAC)
		return lookupErr == nil && macvlan != nil
	}) {
		if lookupErr != nil && !errors.Is(lookupErr, errLinkAbsent) {
			return fmt.Errorf("list links: %w", lookupErr)
		}
		return fmt.Errorf("no device carrying the virtual MAC %s ever appeared; the group never started", virtualMAC)
	}
	parentIndex := macvlan.Attrs().ParentIndex
	if parentIndex == decoy.Attrs().Index {
		return fmt.Errorf("macvlan %s hangs off the DECOY zevrwan (ifindex %d): the parent was taken from the configured interface name instead of the selector's answer", macvlan.Attrs().Name, parentIndex)
	}
	if parentIndex != selected.Attrs().Index {
		return fmt.Errorf("macvlan %s hangs off ifindex %d, want zevrsel0 (ifindex %d)", macvlan.Attrs().Name, parentIndex, selected.Attrs().Index)
	}
	fmt.Printf("MACVLAN-PARENT %s -> zevrsel0\n", macvlan.Attrs().Name)
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("list links: %w", err)
	}
	for _, link := range links {
		if link.Attrs().ParentIndex == decoy.Attrs().Index {
			return fmt.Errorf("device %s was built on the decoy zevrwan", link.Attrs().Name)
		}
	}
	fmt.Println("DECOY-UNTOUCHED zevrwan")
	return nil
}
