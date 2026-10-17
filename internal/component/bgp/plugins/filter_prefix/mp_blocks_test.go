package filter_prefix

import (
	"net/netip"
	"testing"
)

// TestModifyDeltaKeepsEveryBlock proves the modify delta answers block by
// block: each NLRI section keeps its own head, including the mp marker of an
// MP_REACH_NLRI block, and carries only its accepted prefixes.
//
// VALIDATES: a mixed UPDATE with a legacy block and an MP IPv6 block yields one
// block for each, so the engine rewrites each section from its own block.
// PREVENTS: the whole nlri field read as one block, which made "nlri", "mp" and
// the second family parse errors and rejected every mixed UPDATE.
func TestModifyDeltaKeepsEveryBlock(t *testing.T) {
	list := &prefixList{
		name: "CUSTOMERS",
		entries: []prefixEntry{
			{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24, action: actionAccept},
			{prefix: netip.MustParsePrefix("2001:db8::/32"), ge: 32, le: 48, action: actionAccept},
		},
	}
	tests := []struct {
		name  string
		field string
		want  string
	}{
		{
			name:  "legacy-and-mp",
			field: "nlri add 10.0.1.0/24 192.168.1.0/24 mp ipv6/unicast add 2001:db8:1::/48 2001:db9::/48",
			want:  "nlri add 10.0.1.0/24 mp ipv6/unicast add 2001:db8:1::/48",
		},
		{
			name:  "mp-ipv4-block-all-refused-is-left-out",
			field: "nlri add 10.0.1.0/24 mp ipv4/unicast add 192.168.1.0/24",
			want:  "nlri add 10.0.1.0/24",
		},
		{
			name:  "marker-block-echoed",
			field: "mp l2vpn/evpn add mp ipv6/unicast add 2001:db8:1::/48 2001:db9::/48",
			want:  "mp l2vpn/evpn add mp ipv6/unicast add 2001:db8:1::/48",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := list.partitionUpdate(tt.field)
			if p.hadParseError {
				t.Fatalf("partitionUpdate(%q) reported a parse error", tt.field)
			}
			if got := buildModifyDelta(p); got != tt.want {
				t.Errorf("buildModifyDelta = %q, want %q", got, tt.want)
			}
		})
	}
}
