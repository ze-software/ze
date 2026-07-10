package ifacevpp

import (
	"testing"

	"go.fd.io/govpp/binapi/vxlan"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// TestCreateTunnelVxlanVPP verifies AC-3: a vxlan tunnel under the vpp backend
// issues a vxlan_add_del_tunnel_v3 carrying the VNI, endpoints, and the
// default UDP port 4789, and registers the name->SwIfIndex mapping.
// VALIDATES: AC-3 -- VXLAN programmed on VPP.
// PREVENTS: regression / silent no-op for the new tunnel kind.
func TestCreateTunnelVxlanVPP(t *testing.T) {
	ch := &progChannel{swIfIndex: 11}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           100,
		VNISet:        true,
	})
	if err != nil {
		t.Fatalf("CreateTunnel vxlan: %v", err)
	}
	req, ok := ch.requests[0].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[0])
	}
	if !req.IsAdd {
		t.Error("IsAdd: got false, want true")
	}
	if req.Vni != 100 {
		t.Errorf("Vni: got %d, want 100", req.Vni)
	}
	if req.DstPort != 4789 {
		t.Errorf("DstPort: got %d, want 4789 (default)", req.DstPort)
	}
	if got := req.DstAddress.ToIP().String(); got != "10.0.0.2" {
		t.Errorf("DstAddress: got %s, want 10.0.0.2", got)
	}
	if idx, ok := b.names.LookupIndex("vx0"); !ok || idx != 11 {
		t.Errorf("name map: got (%d,%v), want (11,true)", idx, ok)
	}
}

// TestCreateTunnelVxlanCustomPort verifies a configured UDP port overrides the
// 4789 default.
// VALIDATES: AC-3 -- vxlan port honored.
// PREVENTS: the configured port being ignored.
func TestCreateTunnelVxlanCustomPort(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
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
		t.Fatalf("CreateTunnel vxlan: %v", err)
	}
	req, ok := ch.requests[0].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[0])
	}
	if req.DstPort != 8472 {
		t.Errorf("DstPort: got %d, want 8472", req.DstPort)
	}
}

// TestCreateTunnelVxlanRejectsBadVNI verifies backend-side VNI validation
// (defense in depth behind the YANG range 1..16777215).
// VALIDATES: AC-3 boundaries -- VNI 0 and >2^24-1 rejected.
// PREVENTS: an out-of-range VNI reaching VPP.
func TestCreateTunnelVxlanRejectsBadVNI(t *testing.T) {
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
			ch := &progChannel{}
			b := newTunnelBackend(ch)
			err := b.CreateTunnel(iface.TunnelSpec{
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
			if len(ch.requests) != 0 {
				t.Errorf("no VPP request expected on rejection, got %d", len(ch.requests))
			}
		})
	}
}

// TestDeleteTunnelVxlan verifies the delete path issues
// vxlan_add_del_tunnel_v3 with IsAdd=false and clears the name map.
// VALIDATES: AC-3 -- clean vxlan delete path.
// PREVENTS: stale VPP vxlan tunnels after config removal.
func TestDeleteTunnelVxlan(t *testing.T) {
	ch := &progChannel{swIfIndex: 4}
	b := newTunnelBackend(ch)

	if err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindVxlan,
		Name:          "vx0",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		VNI:           100,
		VNISet:        true,
	}); err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if err := b.DeleteInterface("vx0"); err != nil {
		t.Fatalf("DeleteInterface: %v", err)
	}
	last, ok := ch.requests[len(ch.requests)-1].(*vxlan.VxlanAddDelTunnelV3)
	if !ok {
		t.Fatalf("delete request type: got %T, want *vxlan.VxlanAddDelTunnelV3", ch.requests[len(ch.requests)-1])
	}
	if last.IsAdd {
		t.Error("delete: IsAdd got true, want false")
	}
	if _, ok := b.names.LookupIndex("vx0"); ok {
		t.Error("name map still has vx0 after delete")
	}
}
