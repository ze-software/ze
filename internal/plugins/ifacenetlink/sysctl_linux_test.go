//go:build linux

package ifacenetlink

import (
	"os"
	"path/filepath"
	"testing"
)

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
	b := &netlinkBackend{}

	if err := b.SetIPv4Forwarding("eth0", true); err != nil {
		t.Fatalf("SetIPv4Forwarding(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/forwarding"); got != "1" {
		t.Errorf("forwarding=true: got %q, want %q", got, "1")
	}

	if err := b.SetIPv4Forwarding("eth0", false); err != nil {
		t.Fatalf("SetIPv4Forwarding(false): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/forwarding"); got != "0" {
		t.Errorf("forwarding=false: got %q, want %q", got, "0")
	}
}

func TestSysctlIPv6ForwardingAcceptRA(t *testing.T) {
	root := testSysctlDir(t, "eth0")
	b := &netlinkBackend{}

	if err := b.SetIPv6Forwarding("eth0", true); err != nil {
		t.Fatalf("SetIPv6Forwarding(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv6/conf/eth0/forwarding"); got != "1" {
		t.Errorf("forwarding=true: got %q, want %q", got, "1")
	}

	if err := b.SetIPv6AcceptRA("eth0", 2); err != nil {
		t.Fatalf("SetIPv6AcceptRA(2): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv6/conf/eth0/accept_ra"); got != "2" {
		t.Errorf("accept_ra(2): got %q, want %q", got, "2")
	}
}

func TestSysctlIPv4ProxyARP(t *testing.T) {
	root := testSysctlDir(t, "eth0")
	b := &netlinkBackend{}

	if err := b.SetIPv4ProxyARP("eth0", true); err != nil {
		t.Fatalf("SetIPv4ProxyARP(true): %v", err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0/proxy_arp"); got != "1" {
		t.Errorf("proxy_arp=true: got %q, want %q", got, "1")
	}
}

func TestSysctlIPv6AcceptRABoundary(t *testing.T) {
	_ = testSysctlDir(t, "eth0")
	b := &netlinkBackend{}

	tests := []struct {
		name    string
		level   int
		wantErr bool
	}{
		{"level 0", 0, false},
		{"level 1", 1, false},
		{"level 2", 2, false},
		{"negative", -1, true},
		{"too high", 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.SetIPv6AcceptRA("eth0", tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetIPv6AcceptRA(%d) error = %v, wantErr %v", tt.level, err, tt.wantErr)
			}
		})
	}
}

func TestSysctlVlanUnit(t *testing.T) {
	// VALIDATES: Sysctl works with VLAN subinterface names.
	// PREVENTS: Path construction failing for dotted interface names.
	const vlanIface = "eth0.100"
	root := testSysctlDir(t, vlanIface)
	b := &netlinkBackend{}

	if err := b.SetIPv4Forwarding(vlanIface, true); err != nil {
		t.Fatalf("SetIPv4Forwarding(%q, true): %v", vlanIface, err)
	}
	if got := readSysctl(t, root, "net/ipv4/conf/eth0.100/forwarding"); got != "1" {
		t.Errorf("vlan forwarding: got %q, want %q", got, "1")
	}
}

func TestSysctlInvalidName(t *testing.T) {
	b := &netlinkBackend{}
	if err := b.SetIPv4Forwarding("", true); err == nil {
		t.Error("expected error for empty name")
	}
	if err := b.SetIPv6Forwarding("abcdefghijklmnop", true); err == nil {
		t.Error("expected error for too-long name")
	}
}
