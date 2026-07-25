// Pure-function coverage for the VXLAN netlink builder. buildTunnelLink with
// no local-interface source touches no kernel state, so these run on any linux
// host without CAP_NET_ADMIN (the round-trip-against-the-kernel path lives in
// the integration-tagged tunnel_linux_test.go).

//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestBuildVxlanLink verifies AC-3 (A-5): the netlink half of the new vxlan
// kind maps VNI, endpoints, and the default UDP port onto a netlink.Vxlan.
// VALIDATES: AC-3 -- vxlan lands in the netlink backend too.
// PREVENTS: vxlan silently becoming VPP-only, breaking the kind<->case<->backend uniformity.
func TestBuildVxlanLink(t *testing.T) {
	link, err := buildTunnelLink(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           100,
		VNISet:        true,
	})
	if err != nil {
		t.Fatalf("buildTunnelLink vxlan: %v", err)
	}
	vx, ok := link.(*netlink.Vxlan)
	if !ok {
		t.Fatalf("link type: got %T, want *netlink.Vxlan", link)
	}
	if vx.VxlanId != 100 {
		t.Errorf("VxlanId: got %d, want 100", vx.VxlanId)
	}
	if vx.Port != 4789 {
		t.Errorf("Port: got %d, want 4789 (default)", vx.Port)
	}
	if !vx.Group.Equal([]byte{10, 0, 0, 2}) {
		t.Errorf("Group: got %v, want 10.0.0.2", vx.Group)
	}
	if vx.Name != "vx0" {
		t.Errorf("Name: got %q, want vx0", vx.Name)
	}
}

// TestBuildVxlanCustomPort verifies a configured UDP port overrides 4789.
func TestBuildVxlanCustomPort(t *testing.T) {
	link, err := buildTunnelLink(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           7,
		VNISet:        true,
		Port:          8472,
		PortSet:       true,
	})
	if err != nil {
		t.Fatalf("buildTunnelLink vxlan: %v", err)
	}
	vx, ok := link.(*netlink.Vxlan)
	if !ok {
		t.Fatalf("link type: got %T, want *netlink.Vxlan", link)
	}
	if vx.Port != 8472 {
		t.Errorf("Port: got %d, want 8472", vx.Port)
	}
}

// TestBuildVxlanRejectsBadVNI verifies the builder rejects an unset, zero, or
// out-of-range VNI.
// VALIDATES: AC-3 boundaries -- VNI 0 and >2^24-1 rejected in netlink too.
func TestBuildVxlanRejectsBadVNI(t *testing.T) {
	cases := []struct {
		name   string
		vni    uint32
		vniSet bool
	}{
		{"unset", 0, false},
		{"zero", 0, true},
		{"too-big", 16777216, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTunnelLink(iface.TunnelSpec{
				Kind:          iface.TunnelKindVxlan,
				Name:          "vx0",
				LocalAddress:  "10.0.0.1",
				RemoteAddress: "10.0.0.2",
				VNI:           tc.vni,
				VNISet:        tc.vniSet,
			})
			if err == nil {
				t.Fatalf("expected error for VNI %d (set=%v), got nil", tc.vni, tc.vniSet)
			}
		})
	}
}
