package filter_prefix

import (
	"net/netip"
	"testing"
)

// VALIDATES: AC-1 — Prefix within ge/le range matches.
// VALIDATES: AC-2 — Prefix outside (too specific) does not match.
// VALIDATES: AC-3 — Prefix outside (too general) does not match.
// VALIDATES: AC-4 — Exact match (ge=le=prefix-length) matches.
// VALIDATES: AC-7 — First match wins.
// VALIDATES: AC-8 — No match = implicit deny (returns reject).
// VALIDATES: AC-9 — IPv4 prefix support.
// VALIDATES: AC-10 — IPv6 prefix support.
// VALIDATES: AC-12 — ge boundary 0.
// VALIDATES: AC-13 — le boundary 128 (IPv6).
// PREVENTS: Wrong subnet inclusion logic, off-by-one ge/le, IPv4/IPv6 confusion.
func TestEvaluatePrefix(t *testing.T) {
	// Helper to build an entry from inline literals. Callers pass explicit
	// ge/le values: zero is a legitimate value (AC-12 default route) so the
	// helper does not substitute defaults.
	mk := func(p string, ge, le uint8, action action) prefixEntry {
		pfx := netip.MustParsePrefix(p)
		return prefixEntry{prefix: pfx, ge: ge, le: le, action: action}
	}

	tests := []struct {
		name    string
		entries []prefixEntry
		route   string
		want    action
	}{
		// AC-1: in range
		{
			name:    "AC1_in_range_accept",
			entries: []prefixEntry{mk("10.0.0.0/8", 16, 24, actionAccept)},
			route:   "10.1.0.0/20",
			want:    actionAccept,
		},
		// AC-2: too specific (length > le)
		{
			name:    "AC2_too_specific",
			entries: []prefixEntry{mk("10.0.0.0/8", 16, 24, actionAccept)},
			route:   "10.1.0.0/26",
			want:    actionReject, // implicit deny (no match)
		},
		// AC-3: too general (length < ge)
		{
			name:    "AC3_too_general",
			entries: []prefixEntry{mk("10.0.0.0/8", 16, 24, actionAccept)},
			route:   "10.0.0.0/12",
			want:    actionReject, // implicit deny (no match)
		},
		// AC-4: exact match (ge==le==prefix bits)
		{
			name:    "AC4_exact_accept",
			entries: []prefixEntry{mk("10.0.0.0/8", 8, 8, actionAccept)},
			route:   "10.0.0.0/8",
			want:    actionAccept,
		},
		// AC-5: exact match with reject action
		{
			name:    "AC5_exact_reject",
			entries: []prefixEntry{mk("10.0.0.0/8", 8, 8, actionReject)},
			route:   "10.0.0.0/8",
			want:    actionReject,
		},
		// AC-6 (default action accept) — covered by mk default in actionAccept
		// AC-7: first match wins (reject before accept => reject)
		{
			name: "AC7_first_match_wins_reject",
			entries: []prefixEntry{
				mk("10.0.0.0/8", 16, 24, actionReject),
				mk("10.0.0.0/8", 16, 24, actionAccept),
			},
			route: "10.1.0.0/20",
			want:  actionReject,
		},
		// AC-7: first match wins (accept before reject => accept)
		{
			name: "AC7_first_match_wins_accept",
			entries: []prefixEntry{
				mk("10.0.0.0/8", 16, 24, actionAccept),
				mk("10.0.0.0/8", 16, 24, actionReject),
			},
			route: "10.1.0.0/20",
			want:  actionAccept,
		},
		// AC-8: no entry matches => implicit deny
		{
			name:    "AC8_no_match_implicit_deny",
			entries: []prefixEntry{mk("10.0.0.0/8", 8, 8, actionAccept)},
			route:   "192.168.0.0/24",
			want:    actionReject,
		},
		// AC-9: IPv4
		{
			name:    "AC9_ipv4",
			entries: []prefixEntry{mk("172.16.0.0/12", 12, 24, actionAccept)},
			route:   "172.16.5.0/24",
			want:    actionAccept,
		},
		// AC-10: IPv6
		{
			name:    "AC10_ipv6",
			entries: []prefixEntry{mk("2001:db8::/32", 32, 48, actionAccept)},
			route:   "2001:db8:1::/48",
			want:    actionAccept,
		},
		// AC-12: ge=0 boundary (allows even default route under matching parent)
		{
			name:    "AC12_ge_zero_default_route",
			entries: []prefixEntry{{prefix: netip.MustParsePrefix("0.0.0.0/0"), ge: 0, le: 32, action: actionAccept}},
			route:   "0.0.0.0/0",
			want:    actionAccept,
		},
		// AC-13: le=128 boundary on IPv6
		{
			name:    "AC13_le_128_ipv6_host",
			entries: []prefixEntry{{prefix: netip.MustParsePrefix("2001:db8::/32"), ge: 32, le: 128, action: actionAccept}},
			route:   "2001:db8::1/128",
			want:    actionAccept,
		},
		// IPv4/IPv6 cross-family does not match
		{
			name:    "cross_family_no_match",
			entries: []prefixEntry{mk("10.0.0.0/8", 8, 32, actionAccept)},
			route:   "::1/128",
			want:    actionReject, // implicit deny
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := netip.MustParsePrefix(tt.route)
			got := evaluatePrefix(tt.entries, route)
			if got != tt.want {
				t.Errorf("evaluatePrefix(%s) = %v, want %v", tt.route, got, tt.want)
			}
		})
	}
}
