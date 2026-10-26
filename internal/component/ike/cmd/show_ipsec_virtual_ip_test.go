// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- show vpn ipsec sa
// VALIDATES: the SA row carries virtual-ip, the leased inner addresses (AC-33).
// PREVENTS: a lease the operator cannot see, and null where the SA holds no lease.

package cmd

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/engine"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// TestShowSAReportsVirtualIP checks the virtual-ip key: present and empty for an SA that
// holds no lease, and each leased address rendered in order by virtualIPText.
func TestShowSAReportsVirtualIP(t *testing.T) {
	row := saToMap(&engine.SA{PeerName: "rw"}, time.Now(), nil, sadCounters{})
	got, ok := row["virtual-ip"].([]string)
	if !ok {
		t.Fatalf("virtual-ip = %#v, want a []string", row["virtual-ip"])
	}
	if len(got) != 0 {
		t.Fatalf("virtual-ip = %v for an SA with no lease, want empty", got)
	}

	text := virtualIPText([]netip.Addr{netip.MustParseAddr("10.10.0.1"), netip.MustParseAddr("fd00:10::1")})
	if len(text) != 2 || text[0] != "10.10.0.1" || text[1] != "fd00:10::1" {
		t.Fatalf("virtualIPText = %v, want [10.10.0.1 fd00:10::1]", text)
	}

	// A leased SA, granted through the engine's own assignVirtualIP, carries both
	// addresses in the row, and `| json` renders the same list.
	leased := &engine.SA{PeerName: "rw"}
	if err := engine.LeaseVirtualIPForTest(leased, ipsec.VirtualIPPool{
		Name: "rw", Range: "10.10.0.0/29", Range6: "fd00:10::/120",
	}); err != nil {
		t.Fatalf("lease: %v", err)
	}
	row = saToMap(leased, time.Now(), nil, sadCounters{})
	got, ok = row["virtual-ip"].([]string)
	if !ok {
		t.Fatalf("virtual-ip = %#v for a leased SA, want a []string", row["virtual-ip"])
	}
	if len(got) != 2 || got[0] != "10.10.0.1" || got[1] != "fd00:10::1" {
		t.Fatalf("virtual-ip = %v, want the leases [10.10.0.1 fd00:10::1]", got)
	}
	out, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal the row: %v", err)
	}
	if !strings.Contains(string(out), `"virtual-ip":["10.10.0.1","fd00:10::1"]`) {
		t.Fatalf("| json renders %s, want virtual-ip [10.10.0.1 fd00:10::1]", out)
	}
}
