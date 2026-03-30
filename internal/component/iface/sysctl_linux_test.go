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

	// accept_ra with forwarding enabled must be 2, not 1.
	if err := SetIPv6AcceptRA("eth0", true, true); err != nil {
		t.Fatalf("SetIPv6AcceptRA(enabled=true, fwd=true): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "2" {
		t.Errorf("accept_ra(enabled+fwd): got %q, want %q", got, "2")
	}

	// accept_ra without forwarding must be 1.
	if err := SetIPv6AcceptRA("eth0", true, false); err != nil {
		t.Fatalf("SetIPv6AcceptRA(enabled=true, fwd=false): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "1" {
		t.Errorf("accept_ra(enabled, no fwd): got %q, want %q", got, "1")
	}

	// accept_ra disabled must be 0 regardless of forwarding.
	if err := SetIPv6AcceptRA("eth0", false, true); err != nil {
		t.Fatalf("SetIPv6AcceptRA(enabled=false, fwd=true): %v", err)
	}
	got = readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra")
	if got != "0" {
		t.Errorf("accept_ra(disabled): got %q, want %q", got, "0")
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
			if err := SetIPv6AcceptRA(tt.iface, true, false); err == nil {
				t.Error("expected error for invalid interface name, got nil")
			}
		})
	}
}
