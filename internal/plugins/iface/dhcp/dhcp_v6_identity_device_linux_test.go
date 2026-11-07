// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_lease_linux_test.go -- newV6Rig, the rig these tests build on
// Related: dhcp_v4_rig_linux_test.go -- the loop backend whose links answer the identity device
// RFC: rfc/short/rfc9915.md

//go:build linux

package ifacedhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/iana"

	"github.com/ze-software/ze/internal/component/iface"
)

// v6IdentityMAC is the hardware address of the device a DHCPv6 identity is
// named for. Every octet the IAID takes differs from v6TestMAC, the client's
// own device's, so an identity built from the wrong device cannot match.
var v6IdentityMAC = net.HardwareAddr{0x02, 0x00, 0x5e, 0xaa, 0xbb, 0xcc}

// v6IdentityLogMsg is the Error identify logs when the identity device cannot
// give a hardware address.
const v6IdentityLogMsg = "the device that names the DHCPv6 identity cannot be read"

// newV6IdentityRig is a v6 rig whose identity device is a logical name, plus a
// worker that has not run. identity is the logical name; kernel is the kernel
// device the test may bind it to. Neither exists on the host, so only the
// shared resolver, through the loop backend, can answer either.
func newV6IdentityRig(t *testing.T, duid string) (r *v6Rig, w *v6Worker, identity, kernel string) {
	t.Helper()
	seq := loopNameSeq.Add(1)
	identity = fmt.Sprintf("pppsrc%d", seq)
	kernel = fmt.Sprintf("zeidtest%d", seq)
	r = newV6Rig(t, dHCPConfig{DUID: duid, DUIDDevice: identity})
	// A failed socket open retries on the fake clock; the timeout ends it, so
	// a defect fails the test instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	w = &v6Worker{c: r.client, ctx: ctx}
	return r, w, identity, kernel
}

// TestV6IdentityDeviceResolvesLogicalName proves that identify takes the DHCPv6
// identity (RFC 9915 Section 11: the DUID-LL and the IAIDs) from the device the
// shared resolver binds the identity device's LOGICAL name to.
//
// Method: the identity device (a PPPoE client's source-interface) is a logical
// name that only the os-name selector maps to a kernel device. The loop backend
// answers that kernel device with v6IdentityMAC and answers nothing under the
// logical name, and no device of either name exists on the host. Reading the
// name with net.InterfaceByName, asking the backend for the logical name
// without the selector, or keeping the client's own device MAC (v6TestMAC, from
// v6Dial) each fails identify or yields other IAIDs than the literals asserted.
func TestV6IdentityDeviceResolvesLogicalName(t *testing.T) {
	explicit := "00:03:00:01:02:00:00:00:00:99"
	cases := []struct {
		name     string
		duid     string
		wantDUID dhcpv6.DUID
	}{
		{
			name:     "no duid: DUID-LL from the identity device",
			wantDUID: &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: v6IdentityMAC},
		},
		{
			name:     "explicit duid: IAIDs still from the identity device",
			duid:     explicit,
			wantDUID: &dhcpv6.DUIDLL{HWType: iana.HWTypeEthernet, LinkLayerAddr: net.HardwareAddr{0x02, 0, 0, 0, 0, 0x99}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, w, identity, kernel := newV6IdentityRig(t, tc.duid)
			r.backend.mu.Lock()
			r.backend.links = map[string]iface.InterfaceInfo{
				kernel: {Name: kernel, OsName: kernel, Index: 9, State: iface.LinkStateUp, MAC: v6IdentityMAC.String()},
			}
			r.backend.mu.Unlock()
			t.Cleanup(iface.SetResolverMappingForConsumerTest(map[string]string{identity: kernel}))

			if !w.identify() {
				t.Fatalf("identify = false, want true; error logs: %d", r.logs.count(slog.LevelError, ""))
			}
			// The last four octets of v6IdentityMAC; the IA_PD IAID flips the first.
			if want := [4]byte{0x5e, 0xaa, 0xbb, 0xcc}; w.iaidNA != want {
				t.Errorf("IA_NA IAID = %x, want %x (from the identity device, not the client's %x)", w.iaidNA, want, v6ClientIAID)
			}
			if want := [4]byte{0xa1, 0xaa, 0xbb, 0xcc}; w.iaidPD != want {
				t.Errorf("IA_PD IAID = %x, want %x", w.iaidPD, want)
			}
			if w.duid == nil || !w.duid.Equal(tc.wantDUID) {
				t.Errorf("DUID = %v, want %v", w.duid, tc.wantDUID)
			}
		})
	}
}

// TestV6IdentityDeviceUnreadableTurnsDHCPv6Off proves that an identity device
// that gives no hardware address turns DHCPv6 off with one Error, and never
// falls back to the client's own device MAC.
//
// Method: in one case the logical name binds to nothing the backend answers;
// in the other the selector binds it to a device with no link-layer address,
// as a PPP session device has. identify must answer false, log the Error once,
// and settle no DUID; a fallback to v6TestMAC would answer true.
func TestV6IdentityDeviceUnreadableTurnsDHCPv6Off(t *testing.T) {
	cases := []struct {
		name  string
		bound bool // the selector binds the name to a device with no MAC
	}{
		{name: "the name resolves to no device"},
		{name: "the device has no hardware address", bound: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, w, identity, kernel := newV6IdentityRig(t, "")
			mapping := map[string]string{}
			if tc.bound {
				r.backend.mu.Lock()
				r.backend.links = map[string]iface.InterfaceInfo{
					kernel: {Name: kernel, OsName: kernel, Index: 9, State: iface.LinkStateUp},
				}
				r.backend.mu.Unlock()
				mapping[identity] = kernel
			}
			t.Cleanup(iface.SetResolverMappingForConsumerTest(mapping))

			if w.identify() {
				t.Fatalf("identify = true, want false: DUID %v, IA_NA IAID %x", w.duid, w.iaidNA)
			}
			if n := r.logs.count(slog.LevelError, v6IdentityLogMsg); n != 1 {
				t.Errorf("Error %q logged %d times, want 1", v6IdentityLogMsg, n)
			}
			if w.duid != nil {
				t.Errorf("DUID = %v, want none settled", w.duid)
			}
		})
	}
}
