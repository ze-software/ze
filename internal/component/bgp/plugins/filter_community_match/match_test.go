package filter_community_match

import "testing"

// VALIDATES: AC-1 -- standard community match triggers action.
// VALIDATES: AC-2 -- large community match triggers action.
// VALIDATES: AC-3 -- extended community match triggers action.
// VALIDATES: AC-4 -- accept action on match.
// VALIDATES: AC-5 -- reject action on match.
// VALIDATES: AC-6 -- no entry matches -> implicit deny (reject).
// VALIDATES: AC-7 -- well-known community name no-export matches.
// VALIDATES: AC-8 -- well-known community name no-advertise matches.
// VALIDATES: AC-10 -- multiple entries, first match wins.
// PREVENTS: Community not found when present; first-match-wins violated.
func TestEvaluateCommunities(t *testing.T) {
	tests := []struct {
		name       string
		entries    []communityEntry
		updateText string
		want       action
	}{
		{
			name: "standard_accept",
			entries: []communityEntry{
				{community: "65001:100", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "standard_reject",
			entries: []communityEntry{
				{community: "65001:100", ctype: communityStandard, action: actionReject},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "standard_multi_values",
			entries: []communityEntry{
				{community: "65001:200", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp community [65001:100 65001:200 65001:300] nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "standard_no_match",
			entries: []communityEntry{
				{community: "65001:999", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "no_community_attr",
			entries: []communityEntry{
				{community: "65001:100", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "large_community_accept",
			entries: []communityEntry{
				{community: "65001:100:200", ctype: communityLarge, action: actionAccept},
			},
			updateText: "origin igp large-community 65001:100:200 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "large_community_multi",
			entries: []communityEntry{
				{community: "65001:100:300", ctype: communityLarge, action: actionAccept},
			},
			updateText: "origin igp large-community [65001:100:200 65001:100:300] nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "extended_community_accept",
			entries: []communityEntry{
				{community: "000200010000000a", ctype: communityExtended, action: actionAccept},
			},
			updateText: "origin igp extended-community 000200010000000a nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "well_known_no_export",
			entries: []communityEntry{
				{community: "no-export", ctype: communityStandard, action: actionReject},
			},
			updateText: "origin igp community [65001:100 no-export] nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "well_known_no_advertise",
			entries: []communityEntry{
				{community: "no-advertise", ctype: communityStandard, action: actionReject},
			},
			updateText: "origin igp community no-advertise nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "first_match_wins",
			entries: []communityEntry{
				{community: "65001:100", ctype: communityStandard, action: actionAccept},
				{community: "65001:100", ctype: communityStandard, action: actionReject},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name: "second_entry_matches",
			entries: []communityEntry{
				{community: "65001:999", ctype: communityStandard, action: actionReject},
				{community: "65001:100", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name:       "no_entries_implicit_deny",
			entries:    nil,
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "wrong_type_no_match",
			entries: []communityEntry{
				{community: "65001:100", ctype: communityLarge, action: actionAccept},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionReject,
		},
		{
			name: "mixed_types_second_matches",
			entries: []communityEntry{
				{community: "65001:100:200", ctype: communityLarge, action: actionReject},
				{community: "65001:100", ctype: communityStandard, action: actionAccept},
			},
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       actionAccept,
		},
		{
			name:       "empty_update",
			entries:    []communityEntry{{community: "65001:100", ctype: communityStandard, action: actionAccept}},
			updateText: "",
			want:       actionReject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCommunities(tt.entries, tt.updateText)
			if got != tt.want {
				t.Errorf("evaluateCommunities() = %v, want %v", got, tt.want)
			}
		})
	}
}

// VALIDATES: extractCommunityField correctly parses the filter text format.
// PREVENTS: Wrong community values extracted, brackets not handled.
func TestExtractCommunityField(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		ctype      communityType
		want       []string
	}{
		{
			name:       "single_standard",
			updateText: "origin igp community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "multi_standard",
			updateText: "origin igp community [65001:100 no-export 65002:200] nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       []string{"65001:100", "no-export", "65002:200"},
		},
		{
			name:       "single_large",
			updateText: "origin igp large-community 65001:100:200 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityLarge,
			want:       []string{"65001:100:200"},
		},
		{
			name:       "single_extended",
			updateText: "origin igp extended-community 000200010000000a nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityExtended,
			want:       []string{"000200010000000a"},
		},
		{
			name:       "no_community",
			updateText: "origin igp as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       nil,
		},
		{
			name:       "empty_update",
			updateText: "",
			ctype:      communityStandard,
			want:       nil,
		},
		{
			name:       "community_at_end",
			updateText: "origin igp community 65001:100",
			ctype:      communityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "bracketed_at_end",
			updateText: "origin igp community [65001:100 65001:200]",
			ctype:      communityStandard,
			want:       []string{"65001:100", "65001:200"},
		},
		{
			name:       "standard_not_fooled_by_extended",
			updateText: "origin igp extended-community 000200010000000a nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       nil,
		},
		{
			name:       "standard_not_fooled_by_large",
			updateText: "origin igp large-community 65001:1:2 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       nil,
		},
		{
			name:       "standard_with_extended_also_present",
			updateText: "origin igp community 65001:100 extended-community 000200010000000a nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "standard_with_large_also_present",
			updateText: "origin igp community 65001:100 large-community 65001:1:2 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       []string{"65001:100"},
		},
		{
			name:       "community_at_start_of_string",
			updateText: "community 65001:100 nlri ipv4/unicast add 10.0.0.0/24",
			ctype:      communityStandard,
			want:       []string{"65001:100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCommunityField(tt.updateText, tt.ctype)
			if len(got) != len(tt.want) {
				t.Fatalf("extractCommunityField() = %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractCommunityField()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
