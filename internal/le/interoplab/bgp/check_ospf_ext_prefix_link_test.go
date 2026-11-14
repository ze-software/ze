package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// TestOSPFExtPrefixLinkVerdict drives the Extended Prefix/Link checker's verdict on
// answers shaped like Ze's `show ospf database opaque-area` (the generic LSDB rows,
// then the decoded section ext_render.go appends). Each refused case is one wrong
// reading the earlier substring check accepted: Ze's own Extended LSAs alone, FRR
// holding only one of the two types, or an FRR opaque LSA of another type in the
// generic rows. The accepted case holds both types from both routers.
func TestOSPFExtPrefixLinkVerdict(t *testing.T) {
	ze := netip.MustParseAddr("172.30.0.2")
	frr := netip.MustParseAddr("172.30.0.3")
	generic := `{"areas":[{"area":"0.0.0.0","lsas":[` +
		`{"type":"opaque-area","link-state-id":"4.0.0.0","advertising-router":"172.30.0.3"},` +
		`{"type":"opaque-area","link-state-id":"7.0.0.1","advertising-router":"172.30.0.2"}]}],"as-external":[]}`
	decoded := func(prefix, link string) string {
		return "[" + generic + `,{"extended-prefix":[` + prefix + `],"extended-link":[` + link + `]}]`
	}
	row := func(router string) string {
		return `{"advertising-router":"` + router + `","scope":"area","opaque-id":1}`
	}
	cases := []struct {
		name   string
		output string
		refuse string
	}{
		{"only Ze's own Extended LSAs", decoded(row("172.30.0.2"), row("172.30.0.2")), "Extended Prefix LSA advertised by 172.30.0.3"},
		{"FRR Extended Prefix without FRR Extended Link", decoded(row("172.30.0.2")+","+row("172.30.0.3"), row("172.30.0.2")), "Extended Link LSA advertised by 172.30.0.3"},
		{"FRR Extended Link without FRR Extended Prefix", decoded(row("172.30.0.2"), row("172.30.0.2")+","+row("172.30.0.3")), "Extended Prefix LSA advertised by 172.30.0.3"},
		{"FRR's rows without Ze's own", decoded(row("172.30.0.3"), row("172.30.0.3")), "Extended Prefix LSA advertised by 172.30.0.2"},
		{"no decoded section, FRR only in the generic rows", "[" + generic + "]", "Extended Prefix LSA advertised by 172.30.0.3"},
		{"not JSON", "extended-prefix extended-link 172.30.0.3", "decode Ze's opaque-area database"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ospfExtPrefixLinkVerdict(tc.output, frr, ze)
			if err == nil {
				t.Fatalf("verdict accepted %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.refuse) {
				t.Fatalf("verdict = %v, want it to name %q", err, tc.refuse)
			}
		})
	}
	both := row("172.30.0.2") + "," + row("172.30.0.3")
	if err := ospfExtPrefixLinkVerdict(decoded(both, both), frr, ze); err != nil {
		t.Fatalf("verdict refused both Extended types from both routers: %v", err)
	}
}
