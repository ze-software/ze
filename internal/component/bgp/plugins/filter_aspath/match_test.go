package filter_aspath

import (
	"regexp"
	"testing"
)

// VALIDATES: AC-1 -- Regex matches AS-path string -> accepted
// VALIDATES: AC-2 -- Regex does not match -> next entry evaluated
// VALIDATES: AC-3 -- Accept action on match
// VALIDATES: AC-4 -- Reject action on match
// VALIDATES: AC-5 -- Multiple entries, first match wins
// VALIDATES: AC-6 -- No entry matches -> implicit deny (reject)
// VALIDATES: AC-7 -- Empty AS-path matches ^$
// PREVENTS: Regex not matched when it should; first-match-wins violated.
func TestEvaluateASPath(t *testing.T) {
	tests := []struct {
		name      string
		entries   []aspathEntry
		asPathStr string
		want      action
	}{
		{
			name: "single_asn_accept",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001$`), action: actionAccept},
			},
			asPathStr: "65001",
			want:      actionAccept,
		},
		{
			name: "single_asn_reject",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001$`), action: actionReject},
			},
			asPathStr: "65001",
			want:      actionReject,
		},
		{
			name: "multi_asn_match",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001 65002`), action: actionAccept},
			},
			asPathStr: "65001 65002 65003",
			want:      actionAccept,
		},
		{
			name: "no_match_implicit_deny",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001$`), action: actionAccept},
			},
			asPathStr: "65002",
			want:      actionReject,
		},
		{
			name: "first_match_wins_accept_then_reject",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001`), action: actionAccept},
				{regex: regexp.MustCompile(`^65001`), action: actionReject},
			},
			asPathStr: "65001",
			want:      actionAccept,
		},
		{
			name: "first_match_wins_reject_then_accept",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001`), action: actionReject},
				{regex: regexp.MustCompile(`^65001`), action: actionAccept},
			},
			asPathStr: "65001",
			want:      actionReject,
		},
		{
			name: "second_entry_matches",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^99998$`), action: actionReject},
				{regex: regexp.MustCompile(`^65001$`), action: actionAccept},
			},
			asPathStr: "65001",
			want:      actionAccept,
		},
		{
			name: "empty_aspath_matches_caret_dollar",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^$`), action: actionAccept},
			},
			asPathStr: "",
			want:      actionAccept,
		},
		{
			name: "empty_aspath_no_match",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^65001`), action: actionAccept},
			},
			asPathStr: "",
			want:      actionReject,
		},
		{
			name:      "no_entries_implicit_deny",
			entries:   nil,
			asPathStr: "65001",
			want:      actionReject,
		},
		{
			name: "partial_match_in_middle",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`65002`), action: actionAccept},
			},
			asPathStr: "65001 65002 65003",
			want:      actionAccept,
		},
		{
			name: "transit_path_regex",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^\d+ \d+`), action: actionAccept},
			},
			asPathStr: "65001 65002",
			want:      actionAccept,
		},
		{
			name: "single_hop_only",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^\d+$`), action: actionAccept},
			},
			asPathStr: "65001",
			want:      actionAccept,
		},
		{
			name: "single_hop_rejects_multi",
			entries: []aspathEntry{
				{regex: regexp.MustCompile(`^\d+$`), action: actionAccept},
			},
			asPathStr: "65001 65002",
			want:      actionReject,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateASPath(tt.entries, tt.asPathStr)
			if got != tt.want {
				t.Errorf("evaluateASPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

// VALIDATES: extractASPathField correctly parses the filter text format
// PREVENTS: Wrong AS-path extracted, brackets not stripped.
func TestExtractASPathField(t *testing.T) {
	tests := []struct {
		name       string
		updateText string
		want       string
	}{
		{
			name:       "single_asn",
			updateText: "origin igp as-path 65001 next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001",
		},
		{
			name:       "multi_asn_bracketed",
			updateText: "origin igp as-path [65001 65002 65003] next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001 65002 65003",
		},
		{
			name:       "no_aspath",
			updateText: "origin igp next-hop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "",
		},
		{
			name:       "empty_update",
			updateText: "",
			want:       "",
		},
		{
			name:       "aspath_at_end",
			updateText: "origin igp as-path 65001",
			want:       "65001",
		},
		{
			name:       "bracketed_at_end",
			updateText: "origin igp as-path [65001 65002]",
			want:       "65001 65002",
		},
		{
			name:       "aspath_before_nlri",
			updateText: "as-path 65001 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001",
		},
		{
			name:       "aspath_before_med",
			updateText: "as-path [65001 65002] med 100 nlri ipv4/unicast add 10.0.0.0/24",
			want:       "65001 65002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractASPathField(tt.updateText)
			if got != tt.want {
				t.Errorf("extractASPathField() = %q, want %q", got, tt.want)
			}
		})
	}
}
