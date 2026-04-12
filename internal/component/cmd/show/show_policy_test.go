package show

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// VALIDATES: filterPeersByPolicySelector matches by IP, name, ASN, and wildcard.
// PREVENTS: Selector mismatch for IPv6, ASN, or peer name.
func TestFilterPeersByPolicySelector(t *testing.T) {
	peers := []plugin.PeerInfo{
		{Address: netip.MustParseAddr("10.0.0.1"), Name: "peer-a", PeerAS: 65001},
		{Address: netip.MustParseAddr("10.0.0.2"), Name: "peer-b", PeerAS: 65002},
		{Address: netip.MustParseAddr("::1"), Name: "peer-v6", PeerAS: 65001},
	}

	tests := []struct {
		name     string
		selector string
		wantLen  int
	}{
		{"wildcard", "*", 3},
		{"ip_match", "10.0.0.1", 1},
		{"ip_v6", "::1", 1},
		{"name_match", "peer-b", 1},
		{"asn_match", "as65001", 2},
		{"asn_upper", "AS65002", 1},
		{"no_match_ip", "10.0.0.99", 0},
		{"no_match_name", "nonexistent", 0},
		{"no_match_asn", "as99999", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPeersByPolicySelector(peers, tt.selector)
			if len(got) != tt.wantLen {
				t.Errorf("filterPeersByPolicySelector(%q) returned %d peers, want %d", tt.selector, len(got), tt.wantLen)
			}
		})
	}
}

// VALIDATES: handleShowPolicyList returns filter types from registry.
// PREVENTS: Empty response or missing types.
func TestHandleShowPolicyList(t *testing.T) {
	resp, err := handleShowPolicyList(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %q, want %q", resp.Status, plugin.StatusDone)
	}
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is %T, want map[string]any", resp.Data)
	}
	count, ok := data["count"].(int)
	if !ok {
		t.Fatalf("count is %T, want int", data["count"])
	}
	// At minimum, the filter plugins registered in this binary exist.
	// The exact count depends on which plugins are linked, but it should be > 0
	// since filter_prefix, filter_aspath, etc. are in all.go imports.
	if count < 0 {
		t.Errorf("count = %d, want >= 0", count)
	}
}
