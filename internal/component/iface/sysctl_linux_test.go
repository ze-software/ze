package iface

import (
	"os"
	"path/filepath"
	"testing"
)

// testSysctlDir creates a temporary /proc/sys structure for testing and
// overrides sysctlRoot to point at it. The original value is restored
// when the test completes.
func testSysctlDir(t *testing.T, ifaceName string) string {
	t.Helper()

	dir := t.TempDir()

	ipv4Dir := filepath.Join(dir, "net", "ipv4", "conf", ifaceName)
	ipv6Dir := filepath.Join(dir, "net", "ipv6", "conf", ifaceName)

	if err := os.MkdirAll(ipv4Dir, 0o755); err != nil {
		t.Fatalf("create ipv4 sysctl dir: %v", err)
	}
	if err := os.MkdirAll(ipv6Dir, 0o755); err != nil {
		t.Fatalf("create ipv6 sysctl dir: %v", err)
	}

	old := sysctlRoot
	sysctlRoot = dir
	t.Cleanup(func() { sysctlRoot = old })

	return dir
}

// readSysctl is a test helper that reads a sysctl file relative to the test root.
func readSysctl(t *testing.T, root, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read sysctl %s: %v", relPath, err)
	}
	return string(data)
}

func TestSysctlIPv4Forwarding(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	if err := SetIPv4Forwarding("eth0", true); err != nil {
		t.Fatalf("SetIPv4Forwarding(true): %v", err)
	}
	got := readSysctl(t, root, "net/ipv4/conf/eth0/forwarding")
	if got != "1" {
		t.Errorf("forwarding=true: got %q, want %q", got, "1")
	}

	if err := SetIPv4Forwarding("eth0", false); err != nil {
		t.Fatalf("SetIPv4Forwarding(false): %v", err)
	}
	got = readSysctl(t, root, "net/ipv4/conf/eth0/forwarding")
	if got != "0" {
		t.Errorf("forwarding=false: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv4ArpFilter(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	if err := SetIPv4ArpFilter("eth0", true); err != nil {
		t.Fatalf("SetIPv4ArpFilter(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_filter"); got != "1" {
		t.Errorf("arp_filter=true: got %q, want %q", got, "1")
	}

	if err := SetIPv4ArpFilter("eth0", false); err != nil {
		t.Fatalf("SetIPv4ArpFilter(false): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_filter"); got != "0" {
		t.Errorf("arp_filter=false: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv4ArpAccept(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	if err := SetIPv4ArpAccept("eth0", true); err != nil {
		t.Fatalf("SetIPv4ArpAccept(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_accept"); got != "1" {
		t.Errorf("arp_accept=true: got %q, want %q", got, "1")
	}

	if err := SetIPv4ArpAccept("eth0", false); err != nil {
		t.Fatalf("SetIPv4ArpAccept(false): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_accept"); got != "0" {
		t.Errorf("arp_accept=false: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv6ForwardingAcceptRA(t *testing.T) {
	root := testSysctlDir(t, "eth0")

	// Enable forwarding.
	if err := SetIPv6Forwarding("eth0", true); err != nil {
		t.Fatalf("SetIPv6Forwarding(true): %v", err)
	}
	got := readSysctl(t, root, "net/ipv6/conf/eth0/forwarding")
	if got != "1" {
		t.Errorf("forwarding=true: got %q, want %q", got, "1")
	}

	// accept_ra=2: accept even when forwarding is enabled.
	if err := SetIPv6AcceptRA("eth0", 2); err != nil {
		t.Fatalf("SetIPv6AcceptRA(2): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "2" {
		t.Errorf("accept_ra(2): got %q, want %q", got, "2")
	}

	// accept_ra=1: accept only when not forwarding.
	if err := SetIPv6AcceptRA("eth0", 1); err != nil {
		t.Fatalf("SetIPv6AcceptRA(1): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "1" {
		t.Errorf("accept_ra(1): got %q, want %q", got, "1")
	}

	// accept_ra=0: disabled.
	if err := SetIPv6AcceptRA("eth0", 0); err != nil {
		t.Fatalf("SetIPv6AcceptRA(0): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "0" {
		t.Errorf("accept_ra(0): got %q, want %q", got, "0")
	}
}

func TestSysctlVlanUnit(t *testing.T) {
	const vlanIface = "eth0.100"

	root := testSysctlDir(t, vlanIface)

	if err := SetIPv4Forwarding(vlanIface, true); err != nil {
		t.Fatalf("SetIPv4Forwarding(%q, true): %v", vlanIface, err)
	}
	got := readSysctl(t, root, "net/ipv4/conf/eth0.100/forwarding")
	if got != "1" {
		t.Errorf("vlan forwarding: got %q, want %q", got, "1")
	}

	if err := SetIPv6Autoconf(vlanIface, false); err != nil {
		t.Fatalf("SetIPv6Autoconf(%q, false): %v", vlanIface, err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0.100/autoconf")
	if got != "0" {
		t.Errorf("vlan autoconf: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv6AcceptRABoundary(t *testing.T) {
	// VALIDATES: SetIPv6AcceptRA rejects levels outside [0, 2].
	// PREVENTS: Invalid RA levels reaching the kernel.
	root := testSysctlDir(t, "eth0")

	tests := []struct {
		name    string
		level   int
		want    string
		wantErr bool
	}{
		{"level 0 disable", 0, "0", false},
		{"level 1 normal", 1, "1", false},
		{"level 2 even if forwarding", 2, "2", false},
		{"negative invalid", -1, "", true},
		{"too high invalid", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetIPv6AcceptRA("eth0", tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetIPv6AcceptRA(%d) error = %v, wantErr %v", tt.level, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
				if got != tt.want {
					t.Errorf("accept_ra level %d: got %q, want %q", tt.level, got, tt.want)
				}
			}
		})
	}
}

func TestSysctlIPv4ProxyARP(t *testing.T) {
	// VALIDATES: SetIPv4ProxyARP writes to proxy_arp sysctl.
	// PREVENTS: Proxy ARP configuration not reaching kernel.
	root := testSysctlDir(t, "eth0")

	if err := SetIPv4ProxyARP("eth0", true); err != nil {
		t.Fatalf("SetIPv4ProxyARP(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/proxy_arp"); got != "1" {
		t.Errorf("proxy_arp=true: got %q, want %q", got, "1")
	}

	if err := SetIPv4ProxyARP("eth0", false); err != nil {
		t.Fatalf("SetIPv4ProxyARP(false): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/proxy_arp"); got != "0" {
		t.Errorf("proxy_arp=false: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv4ArpAnnounce(t *testing.T) {
	// VALIDATES: SetIPv4ArpAnnounce writes level values 0-2.
	// PREVENTS: Invalid ARP announce levels reaching kernel.
	root := testSysctlDir(t, "eth0")

	tests := []struct {
		name    string
		level   int
		want    string
		wantErr bool
	}{
		{"level 0 any", 0, "0", false},
		{"level 1 prefer subnet", 1, "1", false},
		{"level 2 best only", 2, "2", false},
		{"negative invalid", -1, "", true},
		{"too high invalid", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetIPv4ArpAnnounce("eth0", tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetIPv4ArpAnnounce(%d) error = %v, wantErr %v", tt.level, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_announce")
				if got != tt.want {
					t.Errorf("arp_announce level %d: got %q, want %q", tt.level, got, tt.want)
				}
			}
		})
	}
}

func TestSysctlIPv4ArpIgnore(t *testing.T) {
	// VALIDATES: SetIPv4ArpIgnore writes level values 0-2.
	// PREVENTS: Invalid ARP ignore levels reaching kernel.
	root := testSysctlDir(t, "eth0")

	tests := []struct {
		name    string
		level   int
		want    string
		wantErr bool
	}{
		{"level 0 reply any", 0, "0", false},
		{"level 1 reply incoming only", 1, "1", false},
		{"level 2 reply incoming plus source", 2, "2", false},
		{"negative invalid", -1, "", true},
		{"too high invalid", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetIPv4ArpIgnore("eth0", tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetIPv4ArpIgnore(%d) error = %v, wantErr %v", tt.level, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := readSysctl(t, root, "net/ipv4/conf/eth0/arp_ignore")
				if got != tt.want {
					t.Errorf("arp_ignore level %d: got %q, want %q", tt.level, got, tt.want)
				}
			}
		})
	}
}

func TestSysctlIPv4RPFilter(t *testing.T) {
	// VALIDATES: SetIPv4RPFilter writes level values 0-2.
	// PREVENTS: Invalid RPF levels reaching kernel.
	root := testSysctlDir(t, "eth0")

	tests := []struct {
		name    string
		level   int
		want    string
		wantErr bool
	}{
		{"level 0 disabled", 0, "0", false},
		{"level 1 strict", 1, "1", false},
		{"level 2 loose", 2, "2", false},
		{"negative invalid", -1, "", true},
		{"too high invalid", 3, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetIPv4RPFilter("eth0", tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetIPv4RPFilter(%d) error = %v, wantErr %v", tt.level, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := readSysctl(t, root, "net/ipv4/conf/eth0/rp_filter")
				if got != tt.want {
					t.Errorf("rp_filter level %d: got %q, want %q", tt.level, got, tt.want)
				}
			}
		})
	}
}

func TestSysctlInvalidName(t *testing.T) {
	tests := []struct {
		name  string
		iface string
	}{
		{name: "empty", iface: ""},
		{name: "too long", iface: "abcdefghijklmnop"}, // 16 chars, exceeds 15
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetIPv4Forwarding(tt.iface, true); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv6Forwarding(tt.iface, true); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv6AcceptRA(tt.iface, 1); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv4ProxyARP(tt.iface, true); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv4ArpAnnounce(tt.iface, 1); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv4ArpIgnore(tt.iface, 1); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
			if err := SetIPv4RPFilter(tt.iface, 1); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
		})
	}
}
