//go:build darwin

package sysctl

import (
	"testing"
)

func TestBackendDarwin(t *testing.T) {
	// VALIDATES: AC-16 -- Darwin: forwarding keys use SYS_SYSCTLBYNAME syscall.
	// PREVENTS: Darwin backend silently failing or using wrong syscall.
	b := newBackend()

	// Read a known key. net.inet.ip.forwarding always exists on macOS.
	val, err := b.read("net.inet.ip.forwarding")
	if err != nil {
		t.Fatalf("read net.inet.ip.forwarding: %v", err)
	}
	if val != "0" && val != "1" {
		t.Errorf("net.inet.ip.forwarding: got %q, want 0 or 1", val)
	}
}

func TestBackendDarwinUnavailableKey(t *testing.T) {
	// VALIDATES: AC-17 -- Darwin: non-available key rejected.
	// PREVENTS: Linux-only keys silently accepted on Darwin.
	b := newBackend()

	_, err := b.read("net.ipv4.conf.all.rp_filter")
	if err == nil {
		t.Error("expected error for Linux-only key on Darwin")
	}

	err = b.write("net.ipv4.conf.all.rp_filter", "1")
	if err == nil {
		t.Error("expected error for Linux-only key write on Darwin")
	}
}
