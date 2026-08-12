package iface

import (
	"slices"
	"testing"
)

// VALIDATES: infoToZeType maps netlink/stdlib interface types to Ze YANG types.
// PREVENTS: loopback misclassified as ethernet, unsupported types leaking through.
func TestInfoToZeType(t *testing.T) {
	tests := []struct {
		name string
		info InterfaceInfo
		want string
	}{
		{
			name: "linux loopback by name",
			info: InterfaceInfo{Type: "device", Name: "lo"},
			want: "loopback",
		},
		{
			name: "linux physical ethernet",
			info: InterfaceInfo{Type: "device", Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff"},
			want: "ethernet",
		},
		{
			name: "linux physical with long name",
			info: InterfaceInfo{Type: "device", Name: "enp3s0f1", MAC: "11:22:33:44:55:66"},
			want: "ethernet",
		},
		{
			name: "linux bridge",
			info: InterfaceInfo{Type: "bridge", Name: "br0", MAC: "aa:bb:cc:dd:ee:01"},
			want: "bridge",
		},
		{
			name: "linux veth",
			info: InterfaceInfo{Type: "veth", Name: "veth0", MAC: "aa:bb:cc:dd:ee:02"},
			want: "veth",
		},
		{
			name: "linux dummy",
			info: InterfaceInfo{Type: "dummy", Name: "dummy0", MAC: "aa:bb:cc:dd:ee:03"},
			want: "dummy",
		},
		{
			name: "non-linux loopback by type",
			info: InterfaceInfo{Type: "loopback", Name: "lo0"},
			want: "loopback",
		},
		{
			name: "non-linux fallback with MAC",
			info: InterfaceInfo{Type: "", Name: "en0", MAC: "aa:bb:cc:dd:ee:ff"},
			want: "ethernet",
		},
		{
			name: "unsupported type tun",
			info: InterfaceInfo{Type: "tuntap", Name: "tun0"},
			want: "",
		},
		{
			name: "wireguard",
			info: InterfaceInfo{Type: "wireguard", Name: "wg0"},
			want: "wireguard",
		},
		{
			name: "no type no mac skipped",
			info: InterfaceInfo{Type: "", Name: "sit0"},
			want: "",
		},
		{
			name: "all-zero MAC skipped",
			info: InterfaceInfo{Type: "", Name: "ip6tnl0", MAC: "00:00:00:00:00:00"},
			want: "",
		},
		{
			name: "xfrm interface",
			info: InterfaceInfo{Type: "xfrm", Name: "xfrm0"},
			want: "xfrm",
		},
		{
			// A vxlan netdev is ethernet-shaped and carries a MAC, which
			// linkToInfo (internal/plugins/iface/netlink/show_linux.go) copies
			// into InterfaceInfo.MAC. Until 2026-08-11 "vxlan" was absent from
			// kernelTunnelKinds, so this case fell through to the MAC fallback
			// at the end of infoToZeType and Ze called a vxlan an ethernet
			// port. The MAC here is what makes the case discriminate.
			name: "vxlan",
			info: InterfaceInfo{Type: "vxlan", Name: "vx0", MAC: "aa:bb:cc:dd:ee:04"},
			want: "tunnel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := infoToZeType(&tt.info)
			if got != tt.want {
				t.Errorf("infoToZeType(%+v) = %q, want %q", tt.info, got, tt.want)
			}
		})
	}
}

// TestKernelTunnelKindsCoversEveryModeledKind pins the two halves of one
// mapping to each other: kernelLinkTypes (tunnel.go) says which kernel link
// type each modeled kind produces, and kernelTunnelKinds (discover.go) says
// which kernel link types are Ze tunnels. A name in one and not the other is a
// kind Ze can create and cannot recognize afterwards.
//
// VALIDATES: the two maps hold the same set of kernel link type names.
// PREVENTS:  the 2026-08-11 defect. "vxlan" was in kernelLinkTypes and not in
//
//	kernelTunnelKinds, so zeManageable (config_apply.go) answered false
//	for a vxlan and the Phase 4 prune skipped it: Ze created a vxlan on
//	commit and left it behind when the operator removed it from the
//	config. Adding a tenth kind repeats that unless this test fails.
func TestKernelTunnelKindsCoversEveryModeledKind(t *testing.T) {
	for kind, linkType := range kernelLinkTypes {
		if !kernelTunnelKinds[linkType] {
			t.Errorf("tunnel kind %s produces a %q link and kernelTunnelKinds (discover.go) "+
				"does not list it: zeManageable answers false, so the Phase 4 prune "+
				"never deletes one and infoToZeType misclassifies one on the host",
				kind, linkType)
		}
	}

	produced := make(map[string]bool, len(kernelLinkTypes))
	for _, linkType := range kernelLinkTypes {
		produced[linkType] = true
	}
	for linkType := range kernelTunnelKinds {
		if !produced[linkType] {
			t.Errorf("kernelTunnelKinds (discover.go) lists %q and no kind in kernelLinkTypes "+
				"(tunnel.go) produces it: Ze would delete a link type it cannot create",
				linkType)
		}
	}
}

func TestSupportedTypesIncludesXFRM(t *testing.T) {
	if !slices.Contains(SupportedTypes(), "xfrm") {
		t.Errorf("SupportedTypes() = %v, missing \"xfrm\"", SupportedTypes())
	}
}
