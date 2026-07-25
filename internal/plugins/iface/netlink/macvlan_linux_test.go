//go:build linux

// Design: docs/features/interfaces.md -- pure macvlan link-builder tests

package ifacenetlink

import (
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestBuildMacvlanLink verifies the pure spec -> netlink.Macvlan translation:
// bridge mode is always set, the parent index and MTU are carried, and the MAC
// and ownership alias land in LinkAttrs (so LinkAdd serializes IFLA_ADDRESS +
// IFLA_IFALIAS atomically).
//
// VALIDATES: AC-1 field mapping (bridge mode, MAC, alias, parent MTU) at the
// builder level, with no kernel call.
// PREVENTS: a macvlan created without bridge mode, without the ownership alias,
// or with the wrong MAC/MTU.
func TestBuildMacvlanLink(t *testing.T) {
	spec := iface.MacvlanSpec{
		Name:   "zv4-42-10",
		Parent: "eth0",
		MAC:    "00:00:5e:00:01:0a",
		Alias:  "ze:owned:redund",
	}
	link, err := buildMacvlanLink(spec, 42, 1500)
	if err != nil {
		t.Fatalf("buildMacvlanLink: %v", err)
	}
	if link.Mode != netlink.MACVLAN_MODE_BRIDGE {
		t.Errorf("mode = %v, want bridge", link.Mode)
	}
	attrs := link.Attrs()
	if attrs.Name != "zv4-42-10" {
		t.Errorf("name = %q, want zv4-42-10", attrs.Name)
	}
	if attrs.ParentIndex != 42 {
		t.Errorf("parent index = %d, want 42", attrs.ParentIndex)
	}
	if attrs.MTU != 1500 {
		t.Errorf("mtu = %d, want 1500", attrs.MTU)
	}
	if attrs.Alias != "ze:owned:redund" {
		t.Errorf("alias = %q, want ze:owned:redund", attrs.Alias)
	}
	if attrs.HardwareAddr.String() != "00:00:5e:00:01:0a" {
		t.Errorf("mac = %q, want 00:00:5e:00:01:0a", attrs.HardwareAddr.String())
	}
}

// TestBuildMacvlanLinkMode verifies the delivery mode is carried from the spec:
// the zero value stays bridge (back-compat), and MacvlanModePrivate maps to the
// netlink private constant -- VRRP requires private so the virtual-MAC device is
// the sole ARP/ND responder for the VIP (spec-vrrp-6).
func TestBuildMacvlanLinkMode(t *testing.T) {
	base := iface.MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"}

	bridge, err := buildMacvlanLink(base, 42, 1500)
	if err != nil {
		t.Fatalf("buildMacvlanLink (default): %v", err)
	}
	if bridge.Mode != netlink.MACVLAN_MODE_BRIDGE {
		t.Errorf("default mode = %v, want bridge", bridge.Mode)
	}

	priv := base
	priv.Mode = iface.MacvlanModePrivate
	link, err := buildMacvlanLink(priv, 42, 1500)
	if err != nil {
		t.Fatalf("buildMacvlanLink (private): %v", err)
	}
	if link.Mode != netlink.MACVLAN_MODE_PRIVATE {
		t.Errorf("mode = %v, want private", link.Mode)
	}
}

// TestBuildMacvlanLink_BadMAC confirms an unparseable MAC is rejected by the
// builder (defense in depth; the registry already validated the caller spec).
func TestBuildMacvlanLink_BadMAC(t *testing.T) {
	spec := iface.MacvlanSpec{Name: "zv4-42-10", Parent: "eth0", MAC: "not-a-mac", Alias: "ze:owned:redund"}
	if _, err := buildMacvlanLink(spec, 42, 1500); err == nil {
		t.Fatal("buildMacvlanLink should reject an unparseable MAC")
	}
}
