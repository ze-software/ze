// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- where the virtual address lives
// Related: instance.go -- doInstallVIPs, the promotion install under test
// Related: vmac_state_integration_linux_test.go -- the ARP answer that install produces, observed on the wire
package vrrp

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
)

// VALIDATES: promotion installs the IPv4 virtual address on the Virtual Router
// MAC device, with the parent subnet's prefix, and never on the parent.
//
// TestPromotionInstallsIPv4AddressOnVirtualMACDevice checks the install that
// makes a non-owner Active router answer ARP for its virtual address. On Linux
// the answer follows from the address being present on the macvlan with the
// subnet's connected route; TestVRRPNonOwnerMasterAnswersARPWithVirtualMACOnly
// observes that answer on the wire.
//
// Method: a non-owner starts in Backup and installs nothing. Its own
// master-down timer promotes it. The one install call must name the
// virtual-MAC device and exactly the virtual address masked to the parent's
// /24, which is the form the kernel answers ARP for from the macvlan.
//
// RFC requirement: RFC9568-6.4.3-1 positive -- a non-owner promoted to Active installs its IPv4 virtual address on the Virtual Router MAC device, never on the parent, with the parent subnet's prefix, so the kernel answers ARP for that address from that device (doInstallVIPs instance.go, vipCIDRs register.go).
// RFC requirement: RFC5798-6.4.3-1 positive -- a non-owner promoted to Master installs its IPv4 virtual address on the virtual router MAC device, never on the parent, with the parent subnet's prefix, so the kernel answers ARP for that address from that device (doInstallVIPs instance.go, vipCIDRs register.go).
// RFC requirement: RFC3768-6.4.3-1 positive -- a VRRPv2 non-owner promoted to Master installs its virtual address on the virtual router MAC device, never on the parent, with the parent subnet's prefix, so the kernel answers ARP for that address from that device (doInstallVIPs instance.go, vipCIDRs register.go).
func TestPromotionInstallsIPv4AddressOnVirtualMACDevice(t *testing.T) {
	v2 := testSpec()
	v2.Version = versionV2
	for _, tc := range []struct {
		name string
		spec GroupSpec
	}{
		{"v3", testSpec()},
		{"v2", v2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			spec.realAddresses = []netip.Addr{netip.MustParseAddr("192.0.2.254")}
			spec.realPrefixes = []netip.Prefix{netip.MustParsePrefix("192.0.2.254/24")}
			in, f, clk := newTestInstance(t, spec)
			in.dispatch(fsm.Startup{Config: in.fsmConfig()})
			if in.machine.State() != fsm.StateBackup {
				t.Fatalf("state = %v, want Backup", in.machine.State())
			}
			if got := f.snapshot().installs; len(got) != 0 {
				t.Fatalf("a Backup installed %+v, want nothing", got)
			}

			promoteToActive(t, in, clk)
			got := f.snapshot().installs
			if len(got) != 1 {
				t.Fatalf("promotion made %d install calls, want 1: %+v", len(got), got)
			}
			if got[0].dev != "zv4-2-10" {
				t.Fatalf("install device = %q, want the virtual-MAC macvlan zv4-2-10", got[0].dev)
			}
			if got[0].dev == spec.ParentDevice {
				t.Fatalf("the virtual address was installed on the parent %q", spec.ParentDevice)
			}
			if !slices.Equal(got[0].cidrs, []string{"192.0.2.1/24"}) {
				t.Fatalf("installed %v, want exactly [192.0.2.1/24]", got[0].cidrs)
			}
		})
	}
}
