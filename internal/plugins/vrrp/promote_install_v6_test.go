// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- where the virtual address lives
// Related: instance.go -- doInstallVIPs, the promotion install under test
// Related: promote_install_test.go -- the IPv4 counterpart
package vrrp

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// VALIDATES: when a non-owner's down timer fires, it installs every IPv6
// virtual address on the Virtual Router MAC device, the install from which
// Linux computes and joins each address's Solicited-Node multicast group.
//
// TestPromotionInstallsIPv6AddressesOnVirtualMACDevice checks the boundary Ze
// owns for the Solicited-Node join. Ze builds no MLD report and no multicast
// membership itself: the kernel joins ff02::1:ffXX:XXXX (RFC 4291 Section
// 2.7.1) for an address when that address is added to a device (addrconf).
// What Ze produces is the add, so the test asserts the add: which device,
// which addresses, and that none happens before the timer fires.
//
// Method: an IPv6 non-owner with fe80::1 and 2001:db8::1 starts in Backup and
// installs nothing. Its own down timer promotes it. The one install call must
// name the virtual-MAC device, never the parent, and carry both addresses.
//
// RFC requirement: RFC9568-6.4.2-7 positive -- when the Active_Down_Timer fires, the IPv6 non-owner installs both of its Virtual Router IPv6 addresses (fe80::1, 2001:db8::1) on the Virtual Router MAC device and never on the parent, the address add from which Linux joins each address's Solicited-Node multicast group; nothing is installed before the timer fires (doInstallVIPs instance.go).
// RFC requirement: RFC5798-6.4.2-7 positive -- when the Master_Down_Timer fires, the IPv6 non-owner installs both of its virtual router IPv6 addresses (fe80::1, 2001:db8::1) on the virtual router MAC device and never on the parent, the address add from which Linux joins each address's Solicited-Node multicast group; nothing is installed before the timer fires (doInstallVIPs instance.go).
func TestPromotionInstallsIPv6AddressesOnVirtualMACDevice(t *testing.T) {
	spec := testSpecV6()
	in, f, clk := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v, want Backup", in.machine.State())
	}
	if got := f.snapshot().installs; len(got) != 0 {
		t.Fatalf("a Backup installed %+v before its down timer fired, want nothing", got)
	}

	promoteToActive(t, in, clk)
	got := f.snapshot().installs
	if len(got) != 1 {
		t.Fatalf("promotion made %d install calls, want 1: %+v", len(got), got)
	}
	if got[0].dev != "zv4-2-10" {
		t.Fatalf("install device = %q, want the virtual-MAC macvlan zv4-2-10 the test instance was built with", got[0].dev)
	}
	if got[0].dev == spec.ParentDevice {
		t.Fatalf("the virtual addresses were installed on the parent %q", spec.ParentDevice)
	}
	installed := make([]netip.Addr, 0, len(got[0].cidrs))
	for _, cidr := range got[0].cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			t.Fatalf("installed %q, not a prefix: %v", cidr, err)
		}
		installed = append(installed, prefix.Addr())
	}
	want := []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::1")}
	if len(installed) != len(want) {
		t.Fatalf("installed %v, want exactly %v", installed, want)
	}
	for i := range want {
		if installed[i] != want[i] {
			t.Fatalf("installed %v, want exactly %v", installed, want)
		}
	}
}
