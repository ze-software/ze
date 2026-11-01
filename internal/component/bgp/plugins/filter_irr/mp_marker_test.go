package filter_irr

import (
	"net/netip"
	"testing"
)

// TestJudgeNLRIOnlyCIDRFamilies proves the list judges only the NLRI of a CIDR
// family (unicast, multicast, labeled unicast) and lets every other family's
// NLRI pass as received.
//
// VALIDATES: judgeNLRI lists and counts the ipv4/unicast NLRI alone when the
// prefix field also carries FlowSpec, L3VPN, EVPN and an unknown family; their
// elements, which are no prefix the list can authorize, are not parsed, so
// they neither fail the route nor count as rejected.
// PREVENTS: a non-CIDR element read as a prefix, which failed or rejected the
// whole UPDATE and its admitted routes with it.
func TestJudgeNLRIOnlyCIDRFamilies(t *testing.T) {
	list := &irrPrefixList{entries: []prefixEntry{{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 8, le: 24}}}
	fields := routeFields(map[string][]string{
		"ipv4/unicast":        {"10.0.1.0/24", "192.168.1.0/24"},
		"ipv4/flow":           {`{"destination":[["192.168.2.0/24"]]}`},
		"ipv4/mpls-vpn":       {`{"rd":"0:65000:1","prefix":"192.168.3.0/24"}`},
		"l2vpn/evpn":          {`{"route-type":"mac-ip-advertisement"}`},
		"ipv4/no-such-family": {"192.168.4.0/24"},
		"ipv4/multicast":      {"10.0.2.0/24"},
		"ipv4/mpls-label":     {`{"labels":[[100,1601]],"prefix":"192.168.5.0/24"}`},
	}, nil)
	judged, err := judgeNLRI(fields, list)
	if err != nil {
		t.Fatalf("judgeNLRI: %v", err)
	}
	var families []string
	for _, block := range judged.families {
		families = append(families, block.family)
	}
	want := []string{"ipv4/mpls-label", "ipv4/multicast", "ipv4/unicast"}
	if len(families) != len(want) {
		t.Fatalf("judged families = %v, want %v", families, want)
	}
	for i := range want {
		if families[i] != want[i] {
			t.Fatalf("judged families = %v, want %v", families, want)
		}
	}
	if judged.accepted != 2 || judged.rejected != 2 {
		t.Errorf("accepted=%d rejected=%d, want 2 and 2 (10.0.1.0/24, 10.0.2.0/24 in; 192.168.1.0/24, 192.168.5.0/24 out)",
			judged.accepted, judged.rejected)
	}
}
