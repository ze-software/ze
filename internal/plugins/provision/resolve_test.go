// Design: docs/architecture/cli/plugin-modes.md -- provision resolve tests

package provision

import (
	"net"
	"net/netip"
	"testing"
)

func loopbackName(t *testing.T) string {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatalf("list interfaces: %v", err)
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagLoopback != 0 {
			return ifi.Name
		}
	}
	t.Fatal("no loopback interface found")
	return ""
}

func TestResolveOrConfigureIPExistingAddress(t *testing.T) {
	t.Parallel()

	lo := loopbackName(t)
	ip, cidr, err := resolveOrConfigureIP(lo, "", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("unexpected error on %s: %v", lo, err)
	}
	if ip != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %s", ip)
	}
	if cidr != "" {
		t.Errorf("expected empty addedCIDR when address exists, got %s", cidr)
	}
}

func TestResolveOrConfigureIPAddressOverride(t *testing.T) {
	t.Parallel()

	lo := loopbackName(t)
	ip, cidr, err := resolveOrConfigureIP(lo, "10.99.0.1", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("unexpected error with override: %v", err)
	}
	if ip != "10.99.0.1" {
		t.Errorf("expected 10.99.0.1, got %s", ip)
	}
	if cidr != "" {
		t.Errorf("expected empty addedCIDR with override, got %s", cidr)
	}
}

func TestServerAddrFromPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		network  string
		wantIP   string
		wantCIDR string
	}{
		{
			name:     "host address preserved",
			network:  "198.19.255.1/24",
			wantIP:   "198.19.255.1",
			wantCIDR: "198.19.255.1/24",
		},
		{
			name:     "network address bumped to first host",
			network:  "192.168.1.0/24",
			wantIP:   "192.168.1.1",
			wantCIDR: "192.168.1.1/24",
		},
		{
			name:     "class A network address",
			network:  "10.0.0.0/8",
			wantIP:   "10.0.0.1",
			wantCIDR: "10.0.0.1/8",
		},
		{
			name:     "point-to-point network address",
			network:  "10.1.1.0/30",
			wantIP:   "10.1.1.1",
			wantCIDR: "10.1.1.1/30",
		},
		{
			name:     "mid-range host address",
			network:  "172.16.0.50/16",
			wantIP:   "172.16.0.50",
			wantCIDR: "172.16.0.50/16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefix := netip.MustParsePrefix(tt.network)
			gotIP, gotCIDR := serverAddrFromPrefix(prefix)

			if gotIP != tt.wantIP {
				t.Fatalf("serverIP: got %s, want %s", gotIP, tt.wantIP)
			}
			if gotCIDR != tt.wantCIDR {
				t.Errorf("addedCIDR: got %s, want %s", gotCIDR, tt.wantCIDR)
			}
		})
	}
}
