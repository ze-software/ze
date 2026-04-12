package iface

import "testing"

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
