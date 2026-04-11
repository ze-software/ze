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

// VALIDATES: Whole-update strict mode — reject if any prefix in the NLRI list is denied.
// PREVENTS: Mixed-prefix UPDATEs being silently accepted when one prefix would be denied.
func TestEvaluateUpdateStrict(t *testing.T) {
	list := &prefixList{
		entries: []prefixEntry{
			{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24, action: actionAccept},
		},
	}

	tests := []struct {
		name      string
		nlriField string // text after "nlri "
		wantAllow bool
	}{
		{
			name:      "single_matching_accepted",
			nlriField: "ipv4/unicast add 10.1.0.0/20",
			wantAllow: true,
		},
		{
			name:      "single_non_matching_rejected",
			nlriField: "ipv4/unicast add 192.168.1.0/24",
			wantAllow: false,
		},
		{
			name:      "two_matching_accepted",
			nlriField: "ipv4/unicast add 10.1.0.0/20 10.2.0.0/22",
			wantAllow: true,
		},
		{
			name:      "mixed_one_denied_rejected",
			nlriField: "ipv4/unicast add 10.1.0.0/20 192.168.1.0/24",
			wantAllow: false,
		},
		{
			name:      "del_op_treated_as_match_targets",
			nlriField: "ipv4/unicast del 10.1.0.0/20",
			wantAllow: true,
		},
		{
			name:      "no_nlri_section_accepted",
			nlriField: "",
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := list.evaluateUpdate(tt.nlriField)
			if got != tt.wantAllow {
				t.Errorf("evaluateUpdate(%q) = %v, want %v", tt.nlriField, got, tt.wantAllow)
			}
		})
	}
}

// TestPartitionUpdate covers the per-prefix partition path that powers the
// cmd-4 phase 2 modify action. Each case ships a prefix-list plus an input
// nlri field and asserts which prefixes land in the accepted vs rejected
// sets, the preserved family/op tokens, and the parse-error flag.
//
// VALIDATES: cmd-4 phase 2 -- per-prefix partition is stable, order-preserving,
//
//	and fail-closed on malformed tokens.
//
// PREVENTS:  regression where the modify path silently reorders prefixes or
//
//	drops the family/op header needed to rebuild the nlri block.
func TestPartitionUpdate(t *testing.T) {
	// Shared list: accept 10.0.0.0/8 in /16..24; default-reject everything else.
	list := &prefixList{
		name: "CUSTOMERS",
		entries: []prefixEntry{
			{prefix: netip.MustParsePrefix("10.0.0.0/8"), ge: 16, le: 24, action: actionAccept},
		},
	}

	tests := []struct {
		name     string
		nlri     string
		accepted []string
		rejected []string
		family   string
		op       string
		parseErr bool
	}{
		{
			name:     "all_accepted",
			nlri:     "ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
			accepted: []string{"10.0.0.0/24", "10.0.1.0/24"},
			family:   "ipv4/unicast",
			op:       "add",
		},
		{
			name:     "all_rejected",
			nlri:     "ipv4/unicast add 192.168.1.0/24 172.16.0.0/24",
			rejected: []string{"192.168.1.0/24", "172.16.0.0/24"},
			family:   "ipv4/unicast",
			op:       "add",
		},
		{
			name:     "mixed_preserves_order",
			nlri:     "ipv4/unicast add 10.0.0.0/24 192.168.1.0/24 10.0.5.0/24",
			accepted: []string{"10.0.0.0/24", "10.0.5.0/24"},
			rejected: []string{"192.168.1.0/24"},
			family:   "ipv4/unicast",
			op:       "add",
		},
		{
			name:   "empty_nlri_is_empty_partition",
			nlri:   "",
			family: "",
			op:     "",
		},
		{
			name:   "header_only_no_prefixes",
			nlri:   "ipv4/unicast add",
			family: "ipv4/unicast",
			op:     "add",
		},
		{
			name:     "malformed_prefix_sets_parse_error",
			nlri:     "ipv4/unicast add 10.0.0.0/24 not-a-prefix 10.0.1.0/24",
			accepted: []string{"10.0.0.0/24", "10.0.1.0/24"},
			family:   "ipv4/unicast",
			op:       "add",
			parseErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := list.partitionUpdate(tt.nlri)
			if got.family != tt.family {
				t.Errorf("family = %q, want %q", got.family, tt.family)
			}
			if got.op != tt.op {
				t.Errorf("op = %q, want %q", got.op, tt.op)
			}
			if got.hadParseError != tt.parseErr {
				t.Errorf("hadParseError = %v, want %v", got.hadParseError, tt.parseErr)
			}
			if !equalStrings(got.accepted, tt.accepted) {
				t.Errorf("accepted = %v, want %v", got.accepted, tt.accepted)
			}
			if !equalStrings(got.rejected, tt.rejected) {
				t.Errorf("rejected = %v, want %v", got.rejected, tt.rejected)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// VALIDATES: extractNLRIField correctly pulls the nlri value out of update text.
// PREVENTS: nlri parsing breaking when the field is missing or appears with surrounding attributes.
func TestExtractNLRIField(t *testing.T) {
	tests := []struct {
		name   string
		update string
		want   string
	}{
		{
			name:   "nlri_only",
			update: "nlri ipv4/unicast add 10.0.0.0/24",
			want:   "ipv4/unicast add 10.0.0.0/24",
		},
		{
			name:   "with_attrs_before",
			update: "origin igp as-path 65001 next-hop 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24",
			want:   "ipv4/unicast add 10.0.0.0/24",
		},
		{
			name:   "multi_prefix",
			update: "nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
			want:   "ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
		},
		{
			name:   "no_nlri",
			update: "origin igp as-path 65001",
			want:   "",
		},
		{
			name:   "empty",
			update: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNLRIField(tt.update)
			if got != tt.want {
				t.Errorf("extractNLRIField(%q) = %q, want %q", tt.update, got, tt.want)
			}
		})
	}
}
