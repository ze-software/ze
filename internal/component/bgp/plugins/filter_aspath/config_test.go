package filter_aspath

import (
	"strings"
	"testing"
)

// VALIDATES: AC-9 -- Invalid regex in config rejected at parse time.
// VALIDATES: AC-10 -- Regex complexity/length limit enforced.
// VALIDATES: Default action is accept when omitted.
// PREVENTS: Silent default substitution or invalid regex silently accepted.
func TestParseOneASPathEntry(t *testing.T) {
	tests := []struct {
		name      string
		in        map[string]any
		wantAct   action
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid_regex_default_action",
			in:      map[string]any{"regex": `^65001$`},
			wantAct: actionAccept,
		},
		{
			name:    "explicit_accept",
			in:      map[string]any{"regex": `^65001$`, "action": "accept"},
			wantAct: actionAccept,
		},
		{
			name:    "explicit_reject",
			in:      map[string]any{"regex": `^65001$`, "action": "reject"},
			wantAct: actionReject,
		},
		{
			name:      "missing_regex",
			in:        map[string]any{"action": "accept"},
			wantErr:   true,
			errSubstr: "missing regex",
		},
		{
			name:      "empty_regex",
			in:        map[string]any{"regex": ""},
			wantErr:   true,
			errSubstr: "missing regex",
		},
		{
			name:      "invalid_regex_syntax",
			in:        map[string]any{"regex": `[invalid`},
			wantErr:   true,
			errSubstr: "invalid regex",
		},
		{
			name:      "regex_too_long",
			in:        map[string]any{"regex": strings.Repeat("a", maxRegexLen+1)},
			wantErr:   true,
			errSubstr: "exceeds maximum length",
		},
		{
			name:    "regex_at_max_length",
			in:      map[string]any{"regex": strings.Repeat("a", maxRegexLen)},
			wantAct: actionAccept,
		},
		{
			name:      "invalid_action",
			in:        map[string]any{"regex": `^65001$`, "action": "permit"},
			wantErr:   true,
			errSubstr: `invalid action "permit"`,
		},
		{
			name:    "complex_regex",
			in:      map[string]any{"regex": `^\d+ \d+ \d+$`},
			wantAct: actionAccept,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOneASPathEntry("test-list", tt.in)
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
			if got.action != tt.wantAct {
				t.Errorf("action = %v, want %v", got.action, tt.wantAct)
			}
			if got.regex == nil {
				t.Error("regex is nil after successful parse")
			}
		})
	}
}

// VALIDATES: parseAsPathLists handles map-form policy/as-path-list config and
// recovers the regex from the list key when the inner map omits it.
func TestParseAsPathLists_MapForm(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"as-path-list": map[string]any{
				"PEERS-ONLY": map[string]any{
					"name": "PEERS-ONLY",
					"entry": map[string]any{
						`^\d+$`: map[string]any{
							"action": "accept",
						},
					},
				},
			},
		},
	}

	lists, err := parseAsPathLists(bgpCfg)
	if err != nil {
		t.Fatalf("parseAsPathLists: %v", err)
	}
	peers, ok := lists["PEERS-ONLY"]
	if !ok {
		t.Fatal("PEERS-ONLY list missing")
	}
	if len(peers.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(peers.entries))
	}
	e := peers.entries[0]
	if e.action != actionAccept {
		t.Errorf("action = %v, want accept", e.action)
	}
	if !e.regex.MatchString("65001") {
		t.Error("regex should match single ASN")
	}
}

// VALIDATES: parseAsPathLists handles list-form (slice of maps) entries and
// preserves order.
func TestParseAsPathLists_ListForm_OrderPreserved(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"as-path-list": map[string]any{
				"ORDERED": map[string]any{
					"name": "ORDERED",
					"entry": []any{
						map[string]any{"regex": `^65001`, "action": "reject"},
						map[string]any{"regex": `^65001`, "action": "accept"},
					},
				},
			},
		},
	}

	lists, err := parseAsPathLists(bgpCfg)
	if err != nil {
		t.Fatalf("parseAsPathLists: %v", err)
	}
	ordered := lists["ORDERED"]
	if len(ordered.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ordered.entries))
	}
	if ordered.entries[0].action != actionReject || ordered.entries[1].action != actionAccept {
		t.Errorf("order lost: got %v, %v", ordered.entries[0].action, ordered.entries[1].action)
	}
}

// VALIDATES: Map form with more than one entry is rejected because
// first-match-wins requires deterministic order.
func TestParseAsPathLists_MultiEntryMapFormRejected(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"as-path-list": map[string]any{
				"MULTI": map[string]any{
					"name": "MULTI",
					"entry": map[string]any{
						`^65001$`: map[string]any{"action": "accept"},
						`^65002$`: map[string]any{"action": "reject"},
					},
				},
			},
		},
	}

	_, err := parseAsPathLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for multi-entry map form, got nil")
	}
	if !strings.Contains(err.Error(), "would lose order") {
		t.Errorf("error %q does not mention order loss", err.Error())
	}
}

// VALIDATES: as-path-list name exceeding maxNameLen is rejected.
func TestParseAsPathLists_NameTooLong(t *testing.T) {
	longName := strings.Repeat("x", maxNameLen+1)
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"as-path-list": map[string]any{
				longName: map[string]any{
					"entry": map[string]any{
						`^65001$`: map[string]any{"action": "accept"},
					},
				},
			},
		},
	}

	_, err := parseAsPathLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for long name, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("error %q does not mention length", err.Error())
	}
}

// VALIDATES: Empty policy block does not error.
func TestParseAsPathLists_NoPolicyBlock(t *testing.T) {
	bgpCfg := map[string]any{}
	lists, err := parseAsPathLists(bgpCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 0 {
		t.Errorf("expected empty map, got %d entries", len(lists))
	}
}
