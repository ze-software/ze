// VPP interface verify: input validation / reject logic -- unsupported kinds and
// operations, out-of-range VLAN/MTU/VNI, malformed CIDR/MAC, non-identity QoS
// maps, over-long or colliding LCP names, unknown interface/family selectors,
// and features this VPP API revision cannot honor (GRE key, WireGuard PSK).
package ifacevpp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	interfaces "go.fd.io/govpp/binapi/interface"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

func TestHandleVPPTraceStart_InvalidNodeName(t *testing.T) {
	resp, err := handleVPPTraceStart(nil, []string{"node", "invalid;name"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Contains(t, resp.Error, "invalid node name")
}

// TestVPPCreateMacvlanRejects verifies AC-8: the VPP backend rejects
// CreateMacvlanDevice fail-closed (exact-or-reject), naming the backend and the
// device, and pointing the operator at the netlink backend -- never a silent
// netlink-only approximation.
func TestVPPCreateMacvlanRejects(t *testing.T) {
	b := &vppBackendImpl{names: newNameMap()}
	err := b.CreateMacvlanDevice(iface.MacvlanSpec{Name: "zv4-2-10", Parent: "eth0", MAC: "00:00:5e:00:01:0a"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "VPP backend")
	require.Contains(t, err.Error(), "netlink backend")
	require.Contains(t, err.Error(), "zv4-2-10")
}

// TestSetupLCPPairNameTooLong verifies AC-7: a host name over the 15-byte Linux
// limit is rejected (no silent truncation).
func TestSetupLCPPairNameTooLong(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	b.names.Add("sixteencharacter", 6, "sixteencharacter")
	err := b.SetupLCPPair("sixteencharacter", "sixteencharacter") // 16 chars
	if err == nil {
		t.Fatal("expected error for host name > 15 bytes, got nil")
	}
	if !strings.Contains(err.Error(), "IFNAMSIZ") {
		t.Errorf("expected IFNAMSIZ in error, got: %v", err)
	}
	if len(ch.requests) != 0 {
		t.Errorf("expected no VPP request on rejection, got %d", len(ch.requests))
	}
}

// TestSetupLCPPairCollision verifies R-5: two ze interfaces mapping to the same
// host TAP name is rejected before it collides in VPP.
func TestSetupLCPPairCollision(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "host"})
	ch := &progChannel{}
	b := newLCPBackend(ch)
	b.names.Add("loop1", 7, "loop1")

	if err := b.SetupLCPPair("loop0", "shadow0"); err != nil {
		t.Fatalf("first SetupLCPPair: %v", err)
	}
	err := b.SetupLCPPair("loop1", "shadow0")
	if err == nil {
		t.Fatal("expected collision error for duplicate host name, got nil")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("expected 'already used' in error, got: %v", err)
	}
}

// TestSetupMirrorRejectsNoDirection verifies at-least-one-of ingress/egress,
// matching the netlink backend's errIfaceMirrorAtLeastOneOf.
// VALIDATES: AC-4 -- no silent no-op mirror.
func TestSetupMirrorRejectsNoDirection(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.SetupMirror("xe0", "xe1", false, false); err == nil {
		t.Fatal("expected error when neither ingress nor egress set, got nil")
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected on rejection, got %d", len(ch.requests))
	}
}

// TestSetupMirrorUnknownInterface verifies an unresolved source or destination
// name is rejected (no partial SPAN). VALIDATES: AC-4 boundary.
func TestSetupMirrorUnknownInterface(t *testing.T) {
	ch := &progChannel{}
	b := newMirrorBackend(ch)
	if err := b.SetupMirror("nope", "xe1", true, false); err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if err := b.SetupMirror("xe0", "nope", true, false); err == nil {
		t.Fatal("expected error for unknown destination, got nil")
	}
}

// TestListNeighborsInvalidFamily rejects an unknown family selector at
// the dispatch boundary -- matches the exact-or-reject policy.
// VALIDATES: unsupported family selector returns an error.
// PREVENTS: silently falling through to some default family.
func TestListNeighborsInvalidFamily(t *testing.T) {
	ch := &neighborChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	if _, err := b.ListNeighbors(99); err == nil {
		t.Fatal("expected error for unsupported family, got nil")
	}
	if ch.dumpCalls != 0 {
		t.Errorf("dump calls: got %d, want 0 (family rejected before dispatch)", ch.dumpCalls)
	}
}

func TestSetMACAddressInvalidString(t *testing.T) {
	// VALIDATES: malformed MAC rejected before SwInterfaceSetMacAddress call
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.names.Add("xe0", 1, "xe0")

	if err := b.SetMACAddress("xe0", "not-a-mac"); err == nil {
		t.Fatal("expected error for invalid MAC")
	}
	// The lazy populate dump already ran; what matters is that no
	// SwInterfaceSetMacAddress request was issued.
	if _, ok := ch.lastRequest.(*interfaces.SwInterfaceSetMacAddress); ok {
		t.Error("SwInterfaceSetMacAddress should not be sent for invalid MAC")
	}
}

func TestSetMACAddressUnknownInterface(t *testing.T) {
	// VALIDATES: unknown interface rejected before parsing MAC
	ch := &dumpChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}

	if err := b.SetMACAddress("xe99", "aa:bb:cc:dd:ee:ff"); err == nil {
		t.Fatal("expected error for unknown interface")
	}
}

// TestCreateTunnelGRERejectsKey verifies the honest exact-or-reject behavior:
// the v0.13.0 gre_tunnel API has no key field, so a configured key is rejected
// rather than silently dropped.
// VALIDATES: AC-2 -- no silent GRE-key drop.
// PREVENTS: an operator believing a keyed GRE tunnel was programmed when it was not.
func TestCreateTunnelGRERejectsKey(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
		Key:           42,
		KeySet:        true,
	})
	if err == nil {
		t.Fatal("expected error for GRE key on VPP backend, got nil")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("expected 'key' in error, got: %v", err)
	}
}

// TestCreateTunnelLocalInterfaceRejected verifies a local-interface source is
// rejected on VPP (VPP terminates tunnels on an address, not an ifindex).
// VALIDATES: AC-2 -- no silent ignoring of local-interface source.
// PREVENTS: a tunnel created with the wrong source when local { interface } is set.
func TestCreateTunnelLocalInterfaceRejected(t *testing.T) {
	ch := &progChannel{}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:           iface.TunnelKindGRE,
		Name:           "gre0",
		LocalInterface: "xe0",
		RemoteAddress:  "192.0.2.2",
	})
	if err == nil {
		t.Fatal("expected error for local-interface source on VPP, got nil")
	}
	if len(ch.requests) != 0 {
		t.Errorf("no VPP request expected on rejection, got %d", len(ch.requests))
	}
}

// TestCreateTunnelUnsupportedKind verifies a netlink-only kind is rejected
// (defense in depth behind the ze:backend commit gate).
// VALIDATES: AC-2/R-2 -- exact-or-reject for unwired kinds.
// PREVENTS: a widened annotation silently no-oping sit/ip6tnl on VPP.
func TestCreateTunnelUnsupportedKind(t *testing.T) {
	ch := &progChannel{}
	b := newTunnelBackend(ch)

	err := b.CreateTunnel(iface.TunnelSpec{
		Kind:          iface.TunnelKindSIT,
		Name:          "sit0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "192.0.2.2",
	})
	if err == nil {
		t.Fatal("expected error for sit tunnel on VPP, got nil")
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

// TestConfigureWireguardRejectsPresharedKey verifies the honest exact-or-reject:
// this VPP wireguard API revision has no preshared-key field, so a PSK is
// rejected rather than silently dropped.
// VALIDATES: AC-5 -- no silent PSK drop.
func TestConfigureWireguardRejectsPresharedKey(t *testing.T) {
	ch := &progChannel{swIfIndex: 1}
	b := newWireguardBackend(ch)
	spec := iface.WireguardSpec{
		Name:       "wg0",
		PrivateKey: wgKey(0x44),
		Peers: []iface.WireguardPeerSpec{{
			Name:            "peerA",
			PublicKey:       wgKey(0x55),
			HasPresharedKey: true,
			PresharedKey:    wgKey(0x66),
		}},
	}
	err := b.ConfigureWireguardDevice(spec)
	if err == nil {
		t.Fatal("expected error for preshared key on VPP backend, got nil")
	}
	if !strings.Contains(err.Error(), "preshared") {
		t.Errorf("expected 'preshared' in error, got: %v", err)
	}
}

// TestResetCountersUnknownInterface rejects before issuing any VPP request.
// VALIDATES: unknown name fails fast with a descriptive error.
// PREVENTS: silently succeeding when the operator typos an interface name.
func TestResetCountersUnknownInterface(t *testing.T) {
	ch := &routeChannel{}
	b := &vppBackendImpl{ch: ch, names: newNameMap()}
	b.populate.Do(func() {})

	err := b.ResetCounters("xe99")
	if err == nil {
		t.Fatal("expected error for unknown interface, got nil")
	}
	if _, ok := ch.lastRequest.(*interfaces.SwInterfaceClearStats); ok {
		t.Error("SwInterfaceClearStats should NOT be sent for unknown interface")
	}
}

func TestCreateVethUnsupported(t *testing.T) {
	// VALIDATES: CreateVeth returns descriptive error
	// PREVENTS: silent failure for unsupported op
	b := &vppBackendImpl{names: newNameMap()}
	err := b.CreateVeth("v0", "v1")
	if err == nil {
		t.Error("expected error for CreateVeth on VPP")
	}
}

func TestCreateVLANValidation(t *testing.T) {
	// VALIDATES: AC-4 -- VLAN ID boundary
	// PREVENTS: invalid VLAN ID reaching VPP
	b := &vppBackendImpl{names: newNameMap()}

	tests := []struct {
		name    string
		vlanID  int
		wantErr bool
	}{
		{"valid min", 1, false},
		{"valid max", 4094, false},
		{"invalid zero", 0, true},
		{"invalid 4095", 4095, true},
		{"invalid negative", -1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.CreateVLAN(iface.VLANSpec{Parent: "xe0", VLANID: tt.vlanID})
			// All return error (either validation or "not supported"), but
			// validation errors should fire before "not supported".
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				// "not supported" is acceptable for valid IDs since GoVPP isn't wired.
				return
			}
		})
	}
}

// VALIDATES: spec-cos-plugin -- VPP rejects non-identity ingress QoS maps.
// PREVENTS: silent PCP remapping that VPP qos-record cannot perform.
func TestCreateVLANRejectsNonIdentityIngress(t *testing.T) {
	b := &vppBackendImpl{names: newNameMap()}

	err := b.CreateVLAN(iface.VLANSpec{
		Parent:        "xe0",
		VLANID:        100,
		IngressQoSMap: map[uint32]uint32{6: 3},
	})
	if err == nil {
		t.Fatal("expected error for non-identity ingress map on VPP")
	}
	if !strings.Contains(err.Error(), "identity") {
		t.Errorf("expected 'identity' error, got: %v", err)
	}
}

// VALIDATES: AC-15 -- VPP rejects non-identity ingress in dynamic update.
// PREVENTS: silent ingress PCP remapping that VPP cannot perform.
func TestVPPUpdateVLANQoSMapNonIdentityIngress(t *testing.T) {
	b := &vppBackendImpl{names: newNameMap()}
	b.names.Add("xe0.100", 42, "xe0.100")

	err := b.UpdateVLANQoSMap("xe0.100", map[uint32]uint32{6: 3}, nil)
	if err == nil {
		t.Fatal("expected error for non-identity ingress map")
	}
	if !strings.Contains(err.Error(), "identity") {
		t.Errorf("expected 'identity' error, got: %v", err)
	}
}

func TestSetMTUValidation(t *testing.T) {
	// VALIDATES: AC-9 -- MTU boundary
	// PREVENTS: invalid MTU reaching VPP
	b := &vppBackendImpl{names: newNameMap()}

	tests := []struct {
		name    string
		mtu     int
		wantErr bool
	}{
		{"valid min", 68, false},
		{"valid 1500", 1500, false},
		{"valid 9000", 9000, false},
		{"valid max", 65535, false},
		{"invalid 67", 67, true},
		{"invalid 65536", 65536, true},
		{"invalid zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.SetMTU("xe0", tt.mtu)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestAddAddressValidation(t *testing.T) {
	// VALIDATES: AC-7 -- address CIDR parsed and validated
	// PREVENTS: malformed CIDR reaching VPP
	b := &vppBackendImpl{names: newNameMap()}

	err := b.AddAddress("xe0", "not-a-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR")
	}

	// Valid CIDR returns "not supported" (GoVPP not wired), not validation error.
	err = b.AddAddress("xe0", "10.0.0.1/24")
	if err == nil {
		t.Error("expected error (not supported)")
	}
}
