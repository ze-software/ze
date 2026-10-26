// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- virtual-IP pool utilization
// Related: doctor_virtual_ip.go -- checkIPsecVirtualIPPool
// Related: metrics.go -- publishVirtualIPGauges
// VALIDATES: AC-28: `ze doctor` names a pool at or above 90% held and the leaf to widen,
// stays quiet below it, and the lease gauge reads the registry's live count per pool.
// PREVENTS: a pool that fills in silence until a client is refused.

package engine

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestDoctorVirtualIPPoolUtilization fills a /28 (14 hosts) of the running engine's
// table one lease at a time. Twelve leases (86%) draw no diagnostic; the thirteenth
// (93%) draws exactly one, which names the pool, the held and leasable counts and the
// range leaf. Each step also reads the ze_ipsec_virtual_ip_leases gauge.
func TestDoctorVirtualIPPoolUtilization(t *testing.T) {
	if got := checkIPsecVirtualIPPool(registry.DoctorCheckContext{}); got != nil {
		t.Fatalf("with no engine running the check reported %v, want nothing", got)
	}

	table := NewSATable()
	table.virtualIP.reload(&ipsec.RemoteAccessConfig{Pools: map[string]ipsec.VirtualIPPool{
		"rw": {Name: "rw", Range: "10.20.0.0/28"},
	}}, slogutil.DiscardLogger())
	SetActiveTableForTest(table)
	t.Cleanup(func() { SetActiveTableForTest(nil) })
	pool, ok := table.virtualIP.pool("rw")
	if !ok {
		t.Fatal("pool rw is not built")
	}
	reg := recordingRegistry{values: map[string]float64{}}
	m := RegisterMetrics(reg)

	for i := range 13 {
		if _, err := table.virtualIP.leases.Acquire(pool, "peer-"+string(rune('a'+i)), eap.FamilyIPv4, netip.Addr{}); err != nil {
			t.Fatalf("lease %d: %v", i+1, err)
		}
		m.publishVirtualIPGauges()
		if got := reg.values["ze_ipsec_virtual_ip_leases{rw}"]; got != float64(i+1) {
			t.Fatalf("after %d leases the gauge reads %v", i+1, got)
		}
		diags := checkIPsecVirtualIPPool(registry.DoctorCheckContext{})
		if i+1 < 13 {
			if len(diags) != 0 {
				t.Fatalf("%d of 14 held drew %d diagnostics, want none below 90%%", i+1, len(diags))
			}
			continue
		}
		if len(diags) != 1 {
			t.Fatalf("13 of 14 held drew %d diagnostics, want one", len(diags))
		}
		if diags[0].Code != diagnostic.CodeDoctorIPsecVirtualIPPool {
			t.Fatalf("code %q, want %q", diags[0].Code, diagnostic.CodeDoctorIPsecVirtualIPPool)
		}
		for _, want := range []string{"pool rw", "holds 13 of the 14", "Widen range under vpn ipsec remote-access pool rw"} {
			if !strings.Contains(diags[0].Message, want) {
				t.Fatalf("message %q does not say %q", diags[0].Message, want)
			}
		}
	}
}
