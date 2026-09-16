// Design: docs/architecture/core-design.md -- sysctl Linux backend
// Related: backend.go -- Read, the exported kernel read
//
// VALIDATES: Read answers the kernel's current value through the one
// key-to-path mapping, and says so by error when the key is absent or empty.
// PREVENTS: a reader answering "" or "0" for a tunable the kernel does not
// hold, and a second copy of keyToPath outside this package.

//go:build linux

package sysctl

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadAnswersTheKernelValueOrSaysWhy roots the backend at a temporary
// /proc/sys, writes tcp_mtu_probing and a per-interface key there, and reads
// both back through Read; an absent key and an empty key each answer an error.
func TestReadAnswersTheKernelValueOrSaysWhy(t *testing.T) {
	root := t.TempDir()
	previous := procSysRoot
	procSysRoot = root
	t.Cleanup(func() { procSysRoot = previous })

	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("net/ipv4/tcp_mtu_probing", "2\n")
	write("net/ipv4/conf/eth0.100/forwarding", "1\n")

	got, err := Read("net.ipv4.tcp_mtu_probing")
	if err != nil {
		t.Fatalf("Read(tcp_mtu_probing): %v", err)
	}
	if got != "2" {
		t.Errorf("Read(tcp_mtu_probing) = %q, want %q (trimmed)", got, "2")
	}

	got, err = Read("net.ipv4.conf.eth0.100.forwarding")
	if err != nil {
		t.Fatalf("Read(per-interface key): %v", err)
	}
	if got != "1" {
		t.Errorf("Read(per-interface key) = %q, want %q: the VLAN dot must not become a slash", got, "1")
	}

	if got, err := Read("net.ipv4.route.mtu_expires"); err == nil {
		t.Errorf("Read(absent key) = %q with no error; want an error", got)
	}
	if got, err := Read(""); err == nil {
		t.Errorf("Read(\"\") = %q with no error; want an error", got)
	}
	if got, err := Read("net.ipv4/../../etc/passwd"); err == nil {
		t.Errorf("Read(traversal) = %q with no error; want an error", got)
	}
}
