//go:build linux

package transport

import (
	"net/netip"
	"os"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
)

// VALIDATES: SO_BINDTODEVICE on the loopback device succeeds and binds
// the socket. The loopback device is always present on Linux; binding
// to it requires CAP_NET_RAW, so we skip when running as an unprivileged
// user with no capability.
// PREVENTS: regression where Device is read but never applied.
func TestUDPBindToDeviceLoopback(t *testing.T) {
	u := &UDP{
		Bind:   netip.MustParseAddrPort("127.0.0.1:0"),
		Mode:   api.SingleHop,
		Device: "lo",
	}
	err := u.Start()
	if err != nil {
		if os.Geteuid() != 0 {
			t.Skipf("SO_BINDTODEVICE lo requires CAP_NET_RAW (root), skipping: %v", err)
		}
		t.Fatalf("Start lo: %v", err)
	}
	if stopErr := u.Stop(); stopErr != nil {
		t.Errorf("Stop: %v", stopErr)
	}
}

// VALIDATES: SO_BINDTODEVICE with a non-existent device name surfaces
// the kernel's error through Start. A nonexistent device fails even for
// unprivileged callers because the kernel rejects the setsockopt before
// the capability check.
// PREVENTS: regression where applySocketOptions swallows the setsockopt
// error and Start reports success.
func TestUDPBindToDeviceNonExistent(t *testing.T) {
	u := &UDP{
		Bind:   netip.MustParseAddrPort("127.0.0.1:0"),
		Mode:   api.SingleHop,
		Device: "bfd-test-nonexistent-0",
	}
	err := u.Start()
	if err == nil {
		_ = u.Stop()
		t.Fatal("Start succeeded for non-existent device; expected error")
	}
	// The error should mention SO_BINDTODEVICE or the device name.
	msg := err.Error()
	if !strings.Contains(msg, "SO_BINDTODEVICE") &&
		!strings.Contains(msg, "bfd-test-nonexistent-0") &&
		!strings.Contains(msg, "no such device") {
		t.Fatalf("unexpected error wrapping: %v", err)
	}
}

// VALIDATES: empty Device skips SO_BINDTODEVICE entirely and Start
// succeeds without CAP_NET_RAW.
// PREVENTS: regression where an empty Device still calls setsockopt and
// fails for unprivileged callers.
func TestUDPBindToDeviceEmpty(t *testing.T) {
	u := &UDP{
		Bind: netip.MustParseAddrPort("127.0.0.1:0"),
		Mode: api.SingleHop,
	}
	if err := u.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := u.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

// helpers removed -- strings.Contains is used directly.
