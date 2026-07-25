//go:build linux

package ifacenetlink

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// TestToDeviceAddressCarriesSecondaryFlag verifies the netlink adapter passes
// the kernel's IFA_F_SECONDARY classification through to the policy, which
// needs it to tell a cascading delete from an isolated one.
//
// VALIDATES: toDeviceAddress maps a netlink address to its prefix and secondary flag.
// PREVENTS: treating every address as primary and rewriting promote_secondaries needlessly.
func TestToDeviceAddressCarriesSecondaryFlag(t *testing.T) {
	primary, err := netlink.ParseAddr("10.77.0.1/24")
	if err != nil {
		t.Fatalf("parse addr: %v", err)
	}
	got, ok := toDeviceAddress(primary)
	if !ok {
		t.Fatal("toDeviceAddress(primary) reported failure")
	}
	if got.Prefix.String() != "10.77.0.1/24" {
		t.Fatalf("prefix = %q, want 10.77.0.1/24", got.Prefix)
	}
	if got.Secondary {
		t.Fatal("primary address reported as secondary")
	}

	secondary, err := netlink.ParseAddr("10.77.0.2/24")
	if err != nil {
		t.Fatalf("parse addr: %v", err)
	}
	secondary.Flags = unix.IFA_F_SECONDARY
	got, ok = toDeviceAddress(secondary)
	if !ok {
		t.Fatal("toDeviceAddress(secondary) reported failure")
	}
	if !got.Secondary {
		t.Fatal("IFA_F_SECONDARY address not reported as secondary")
	}
}

// TestToDeviceAddressRejectsUnusableAddress verifies the adapter reports
// failure rather than fabricating a prefix it cannot represent.
//
// VALIDATES: a nil or IPNet-less netlink address is rejected.
// PREVENTS: a zero-value prefix silently matching a real subnet.
func TestToDeviceAddressRejectsUnusableAddress(t *testing.T) {
	if _, ok := toDeviceAddress(nil); ok {
		t.Fatal("toDeviceAddress(nil) reported success")
	}
	if _, ok := toDeviceAddress(&netlink.Addr{}); ok {
		t.Fatal("toDeviceAddress(no IPNet) reported success")
	}
}

// TestToDeviceAddressNormalizes4In6 verifies an IPv4 address delivered in
// 16-byte form is classified as IPv4, so the primary/secondary policy applies
// to it.
//
// VALIDATES: a 4-in-6 encoded IPv4 address maps to an Is4 prefix.
// PREVENTS: the IPv4 cascade rule being skipped for kernel-reported addresses.
func TestToDeviceAddressNormalizes4In6(t *testing.T) {
	addr := &netlink.Addr{IPNet: &net.IPNet{
		IP:   net.ParseIP("10.77.0.1"), // 16-byte 4-in-6 representation
		Mask: net.CIDRMask(24, 32),
	}}
	got, ok := toDeviceAddress(addr)
	if !ok {
		t.Fatal("toDeviceAddress reported failure")
	}
	if !got.Prefix.Addr().Is4() {
		t.Fatalf("prefix %q is not classified as IPv4", got.Prefix)
	}
	if got.Prefix.String() != "10.77.0.1/24" {
		t.Fatalf("prefix = %q, want 10.77.0.1/24", got.Prefix)
	}
}
