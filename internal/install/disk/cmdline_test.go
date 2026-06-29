// VALIDATES: AC-4 (ze.mac parsed), AC-7 (ze.shell-auth parsed)
// PREVENTS: ze.mac and ze.shell-auth silently ignored on the kernel cmdline

package disk

import (
	"os"
	"testing"
)

func TestParseCmdlineMacAuth(t *testing.T) {
	line := "ze.source=http ze.server=10.0.0.1 ze.mac=aa:bb:cc:dd:ee:ff ze.shell-auth=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	cfg := parseCmdlineString(line)

	if cfg.Mac != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("Mac = %q, want %q", cfg.Mac, "aa:bb:cc:dd:ee:ff")
	}
	wantAuth := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	if cfg.ShellAuth != wantAuth {
		t.Fatalf("ShellAuth = %q, want %q", cfg.ShellAuth, wantAuth)
	}
}

func TestParseCmdlineMacAuthEmpty(t *testing.T) {
	line := "ze.source=http ze.server=10.0.0.1"
	cfg := parseCmdlineString(line)

	if cfg.Mac != "" {
		t.Fatalf("Mac = %q, want empty", cfg.Mac)
	}
	if cfg.ShellAuth != "" {
		t.Fatalf("ShellAuth = %q, want empty", cfg.ShellAuth)
	}
}

func TestIfaceForMac(t *testing.T) {
	dir := t.TempDir()

	createFakeIface(t, dir, "lo", "00:00:00:00:00:00")
	createFakeIface(t, dir, "eth0", "aa:bb:cc:dd:ee:ff")
	createFakeIface(t, dir, "eth1", "11:22:33:44:55:66")

	tests := []struct {
		mac  string
		want string
		ok   bool
	}{
		{"aa:bb:cc:dd:ee:ff", "eth0", true},
		{"AA:BB:CC:DD:EE:FF", "eth0", true},
		{"11:22:33:44:55:66", "eth1", true},
		{"00:00:00:00:00:00", "", false},
		{"ff:ff:ff:ff:ff:ff", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got, err := ifaceForMAC(tt.mac, dir)
		if tt.ok {
			if err != nil {
				t.Errorf("ifaceForMAC(%q): unexpected error: %v", tt.mac, err)
			} else if got != tt.want {
				t.Errorf("ifaceForMAC(%q) = %q, want %q", tt.mac, got, tt.want)
			}
		} else {
			if err == nil {
				t.Errorf("ifaceForMAC(%q) = %q, want error", tt.mac, got)
			}
		}
	}
}

func createFakeIface(t *testing.T, dir, name, mac string) {
	t.Helper()
	ifDir := dir + "/" + name
	if err := os.MkdirAll(ifDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ifDir+"/address", []byte(mac+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
