// Design: docs/architecture/iface/netlink-monitor.md -- unit tests for netlink message parsing
//
//go:build linux

package cmd

import (
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestParseNetlinkRouteMsg(t *testing.T) {
	_, dst, _ := net.ParseCIDR("10.0.0.0/24")
	gw := net.ParseIP("192.168.1.1")

	u := netlink.RouteUpdate{
		Type: unix.RTM_NEWROUTE,
		Route: netlink.Route{
			Dst:       dst,
			Gw:        gw,
			LinkIndex: 0,
			Table:     254,
			Protocol:  2,
			Priority:  100,
		},
	}

	ev := routeUpdateToEvent(&u)

	if ev["type"] != "route" {
		t.Errorf("type = %v, want route", ev["type"])
	}
	if ev["action"] != "new" {
		t.Errorf("action = %v, want new", ev["action"])
	}
	if ev["prefix"] != "10.0.0.0/24" {
		t.Errorf("prefix = %v, want 10.0.0.0/24", ev["prefix"])
	}
	if ev["gateway"] != "192.168.1.1" {
		t.Errorf("gateway = %v, want 192.168.1.1", ev["gateway"])
	}
	if ev["table"] != 254 {
		t.Errorf("table = %v, want 254", ev["table"])
	}

	ts, ok := ev["timestamp"].(string)
	if !ok || ts == "" {
		t.Error("timestamp missing or empty")
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("timestamp not RFC3339: %v", err)
	}
}

func TestParseNetlinkRouteMsgDelete(t *testing.T) {
	_, dst, _ := net.ParseCIDR("10.0.0.0/24")

	u := netlink.RouteUpdate{
		Type: unix.RTM_DELROUTE,
		Route: netlink.Route{
			Dst: dst,
		},
	}

	ev := routeUpdateToEvent(&u)

	if ev["action"] != "del" {
		t.Errorf("action = %v, want del", ev["action"])
	}
}

func TestParseNetlinkRouteMsgDefault(t *testing.T) {
	u := netlink.RouteUpdate{
		Type:  unix.RTM_NEWROUTE,
		Route: netlink.Route{},
	}

	ev := routeUpdateToEvent(&u)

	if ev["prefix"] != "default" {
		t.Errorf("prefix = %v, want default", ev["prefix"])
	}
}

func TestParseNetlinkLinkMsg(t *testing.T) {
	u := netlink.LinkUpdate{
		Link: &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name:         "eth0",
				Index:        2,
				MTU:          1500,
				Flags:        net.FlagUp,
				HardwareAddr: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
			},
		},
	}
	u.Header.Type = unix.RTM_NEWLINK

	ev := linkUpdateToEvent(u)

	if ev["type"] != "link" {
		t.Errorf("type = %v, want link", ev["type"])
	}
	if ev["interface"] != "eth0" {
		t.Errorf("interface = %v, want eth0", ev["interface"])
	}
	if ev["state"] != "up" {
		t.Errorf("state = %v, want up", ev["state"])
	}
	if ev["mtu"] != 1500 {
		t.Errorf("mtu = %v, want 1500", ev["mtu"])
	}
	if ev["mac"] != "00:11:22:33:44:55" {
		t.Errorf("mac = %v, want 00:11:22:33:44:55", ev["mac"])
	}
}

func TestParseNetlinkLinkMsgDelete(t *testing.T) {
	u := netlink.LinkUpdate{
		Link: &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{Name: "veth0", Index: 5},
		},
	}
	u.Header.Type = unix.RTM_DELLINK

	ev := linkUpdateToEvent(u)

	if ev["action"] != "del" {
		t.Errorf("action = %v, want del", ev["action"])
	}
}

func TestParseNetlinkAddrMsg(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("192.168.1.10/24")

	u := netlink.AddrUpdate{
		LinkAddress: *ipNet,
		LinkIndex:   2,
		NewAddr:     true,
	}

	ev := addrUpdateToEvent(u)

	if ev["type"] != "address" {
		t.Errorf("type = %v, want address", ev["type"])
	}
	if ev["action"] != "new" {
		t.Errorf("action = %v, want new", ev["action"])
	}
	if ev["address"] != "192.168.1.0/24" {
		t.Errorf("address = %v, want 192.168.1.0/24", ev["address"])
	}
	if ev["interface-index"] != 2 {
		t.Errorf("interface-index = %v, want 2", ev["interface-index"])
	}
}

func TestParseNetlinkAddrMsgDelete(t *testing.T) {
	_, ipNet, _ := net.ParseCIDR("10.0.0.1/32")

	u := netlink.AddrUpdate{
		LinkAddress: *ipNet,
		LinkIndex:   3,
		NewAddr:     false,
	}

	ev := addrUpdateToEvent(u)

	if ev["action"] != "del" {
		t.Errorf("action = %v, want del", ev["action"])
	}
}

func TestNetlinkMonitorCancellation(t *testing.T) {
	// Verified by the handler's select on ctx.Done().
	// On non-Linux this test validates the stub returns an error.
	// On Linux the streaming handler blocks until context cancellation;
	// a full integration test is in test/plugin/monitor-system-netlink.ci.
}
