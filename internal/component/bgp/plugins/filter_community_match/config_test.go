package filter_community_match

import (
	"strings"
	"testing"
)

// VALIDATES: Config parsing for community-match entries.
// VALIDATES: Default type is standard, default action is accept.
// PREVENTS: Invalid config silently accepted.
func TestParseOneCommunityEntry(t *testing.T) {
	tests := []struct {
		name      string
		in        map[string]any
		wantComm  string
		wantType  communityType
		wantAct   action
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "standard_defaults",
			in:       map[string]any{"community": "65001:100"},
			wantComm: "65001:100",
			wantType: communityStandard,
			wantAct:  actionAccept,
		},
		{
			name:     "explicit_standard_reject",
			in:       map[string]any{"community": "65001:100", "type": "standard", "action": "reject"},
			wantComm: "65001:100",
			wantType: communityStandard,
			wantAct:  actionReject,
		},
		{
			name:     "large_type",
			in:       map[string]any{"community": "65001:100:200", "type": "large"},
			wantComm: "65001:100:200",
			wantType: communityLarge,
			wantAct:  actionAccept,
		},
		{
			name:     "extended_type",
			in:       map[string]any{"community": "000200010000000a", "type": "extended"},
			wantComm: "000200010000000a",
			wantType: communityExtended,
			wantAct:  actionAccept,
		},
		{
			name:     "well_known_no_export",
			in:       map[string]any{"community": "no-export", "action": "reject"},
			wantComm: "no-export",
			wantType: communityStandard,
			wantAct:  actionReject,
		},
		{
			name:      "missing_community",
			in:        map[string]any{"action": "accept"},
			wantErr:   true,
			errSubstr: "missing community",
		},
		{
			name:      "empty_community",
			in:        map[string]any{"community": ""},
			wantErr:   true,
			errSubstr: "missing community",
		},
		{
			name:      "invalid_type",
			in:        map[string]any{"community": "65001:100", "type": "bogus"},
			wantErr:   true,
			errSubstr: `invalid type "bogus"`,
		},
		{
			name:      "invalid_action",
			in:        map[string]any{"community": "65001:100", "action": "permit"},
			wantErr:   true,
			errSubstr: `invalid action "permit"`,
		},
		{
			name:      "community_too_long",
			in:        map[string]any{"community": strings.Repeat("x", maxCommunityLen+1)},
			wantErr:   true,
			errSubstr: "exceeds maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOneCommunityEntry("test-list", tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.community != tt.wantComm {
				t.Errorf("community = %q, want %q", got.community, tt.wantComm)
			}
			if got.ctype != tt.wantType {
				t.Errorf("type = %v, want %v", got.ctype, tt.wantType)
			}
			if got.action != tt.wantAct {
				t.Errorf("action = %v, want %v", got.action, tt.wantAct)
			}
		})
	}
}

// VALIDATES: parseCommunityLists handles map-form config.
func TestParseCommunityLists_MapForm(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"community-match": map[string]any{
				"NO-EXPORT": map[string]any{
					"name": "NO-EXPORT",
					"entry": map[string]any{
						"no-export": map[string]any{
							"action": "reject",
						},
					},
				},
			},
		},
	}

	lists, err := parseCommunityLists(bgpCfg)
	if err != nil {
		t.Fatalf("parseCommunityLists: %v", err)
	}
	noexp, ok := lists["NO-EXPORT"]
	if !ok {
		t.Fatal("NO-EXPORT list missing")
	}
	if len(noexp.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(noexp.entries))
	}
	e := noexp.entries[0]
	if e.community != "no-export" || e.action != actionReject {
		t.Errorf("got community=%q action=%v, want no-export/reject", e.community, e.action)
	}
}

// VALIDATES: List-form entries preserve order.
func TestParseCommunityLists_ListForm_OrderPreserved(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"community-match": map[string]any{
				"ORDERED": map[string]any{
					"name": "ORDERED",
					"entry": []any{
						map[string]any{"community": "65001:100", "action": "reject"},
						map[string]any{"community": "65001:200", "action": "accept"},
					},
				},
			},
		},
	}

	lists, err := parseCommunityLists(bgpCfg)
	if err != nil {
		t.Fatalf("parseCommunityLists: %v", err)
	}
	ordered := lists["ORDERED"]
	if len(ordered.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ordered.entries))
	}
	if ordered.entries[0].action != actionReject || ordered.entries[1].action != actionAccept {
		t.Errorf("order lost: got %v, %v", ordered.entries[0].action, ordered.entries[1].action)
	}
}

// VALIDATES: Multi-entry map form rejected (ordering loss).
func TestParseCommunityLists_MultiEntryMapFormRejected(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"community-match": map[string]any{
				"MULTI": map[string]any{
					"name": "MULTI",
					"entry": map[string]any{
						"65001:100": map[string]any{"action": "accept"},
						"65001:200": map[string]any{"action": "reject"},
					},
				},
			},
		},
	}

	_, err := parseCommunityLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for multi-entry map form, got nil")
	}
	if !strings.Contains(err.Error(), "would lose order") {
		t.Errorf("error %q does not mention order loss", err.Error())
	}
}

// VALIDATES: Name length limit enforced.
func TestParseCommunityLists_NameTooLong(t *testing.T) {
	longName := strings.Repeat("x", maxNameLen+1)
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"community-match": map[string]any{
				longName: map[string]any{
					"entry": map[string]any{
						"65001:100": map[string]any{"action": "accept"},
					},
				},
			},
		},
	}

	_, err := parseCommunityLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for long name, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error %q does not mention length", err.Error())
	}
}

// VALIDATES: Empty policy block does not error.
func TestParseCommunityLists_NoPolicyBlock(t *testing.T) {
	bgpCfg := map[string]any{}
	lists, err := parseCommunityLists(bgpCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 0 {
		t.Errorf("expected empty map, got %d entries", len(lists))
	}
}
