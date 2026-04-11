package filter_prefix

import (
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: AC-6 — Default action is accept when omitted.
// VALIDATES: ge defaults to prefix length when omitted.
// VALIDATES: le defaults to 32 (IPv4) / 128 (IPv6) when omitted.
// VALIDATES: AC-14 — ge > le rejected by config validation.
// PREVENTS: Silent default substitution in the wrong direction.
func TestParseOneEntry(t *testing.T) {
	tests := []struct {
		name      string
		in        map[string]any
		wantGE    uint8
		wantLE    uint8
		wantAct   action
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "ipv4_defaults",
			in:      map[string]any{"prefix": "10.0.0.0/8"},
			wantGE:  8,
			wantLE:  32,
			wantAct: actionAccept,
		},
		{
			name:    "ipv6_defaults",
			in:      map[string]any{"prefix": "2001:db8::/32"},
			wantGE:  32,
			wantLE:  128,
			wantAct: actionAccept,
		},
		{
			name:    "explicit_ge_le_action",
			in:      map[string]any{"prefix": "10.0.0.0/8", "ge": float64(16), "le": float64(24), "action": "reject"},
			wantGE:  16,
			wantLE:  24,
			wantAct: actionReject,
		},
		{
			name:    "string_numeric_ge_le",
			in:      map[string]any{"prefix": "10.0.0.0/8", "ge": "16", "le": "24"},
			wantGE:  16,
			wantLE:  24,
			wantAct: actionAccept,
		},
		{
			name:      "missing_prefix",
			in:        map[string]any{"ge": float64(8)},
			wantErr:   true,
			errSubstr: "missing prefix",
		},
		{
			name:      "ge_gt_le_invalid",
			in:        map[string]any{"prefix": "10.0.0.0/8", "ge": float64(24), "le": float64(16)},
			wantErr:   true,
			errSubstr: "ge 24 > le 16",
		},
		{
			name:      "ge_exceeds_ipv4_max",
			in:        map[string]any{"prefix": "10.0.0.0/8", "ge": float64(33)},
			wantErr:   true,
			errSubstr: "exceeds family max 32",
		},
		{
			name:      "le_exceeds_ipv4_max",
			in:        map[string]any{"prefix": "10.0.0.0/8", "le": float64(40)},
			wantErr:   true,
			errSubstr: "exceeds family max 32",
		},
		{
			name:      "ge_exceeds_ipv6_max",
			in:        map[string]any{"prefix": "2001:db8::/32", "ge": float64(129)},
			wantErr:   true,
			errSubstr: "exceeds family max 128",
		},
		{
			name:    "ge_max_boundary_ipv4",
			in:      map[string]any{"prefix": "10.0.0.0/8", "ge": float64(32), "le": float64(32)},
			wantGE:  32,
			wantLE:  32,
			wantAct: actionAccept,
		},
		{
			name:    "le_max_boundary_ipv6",
			in:      map[string]any{"prefix": "2001:db8::/32", "ge": float64(32), "le": float64(128)},
			wantGE:  32,
			wantLE:  128,
			wantAct: actionAccept,
		},
		{
			name:      "invalid_action",
			in:        map[string]any{"prefix": "10.0.0.0/8", "action": "permit"},
			wantErr:   true,
			errSubstr: `invalid action "permit"`,
		},
		{
			name:      "invalid_prefix",
			in:        map[string]any{"prefix": "not-a-cidr"},
			wantErr:   true,
			errSubstr: `invalid prefix`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOneEntry("test-list", tt.in)
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
			if got.ge != tt.wantGE {
				t.Errorf("ge = %d, want %d", got.ge, tt.wantGE)
			}
			if got.le != tt.wantLE {
				t.Errorf("le = %d, want %d", got.le, tt.wantLE)
			}
			if got.action != tt.wantAct {
				t.Errorf("action = %v, want %v", got.action, tt.wantAct)
			}
		})
	}
}

// VALIDATES: parsePrefixLists handles map-form policy/prefix-list config and
// recovers the prefix from the list key when the inner map omits it.
func TestParsePrefixLists_MapForm(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"prefix-list": map[string]any{
				"CUSTOMERS": map[string]any{
					"name": "CUSTOMERS",
					"entry": map[string]any{
						"10.0.0.0/8": map[string]any{
							"ge":     float64(16),
							"le":     float64(24),
							"action": "accept",
						},
					},
				},
			},
		},
	}

	lists, err := parsePrefixLists(bgpCfg)
	if err != nil {
		t.Fatalf("parsePrefixLists: %v", err)
	}
	cust, ok := lists["CUSTOMERS"]
	if !ok {
		t.Fatal("CUSTOMERS list missing")
	}
	if len(cust.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cust.entries))
	}
	e := cust.entries[0]
	want := netip.MustParsePrefix("10.0.0.0/8")
	if e.prefix != want {
		t.Errorf("prefix = %v, want %v", e.prefix, want)
	}
	if e.ge != 16 || e.le != 24 || e.action != actionAccept {
		t.Errorf("ge=%d le=%d action=%v, want 16/24/accept", e.ge, e.le, e.action)
	}
}

// VALIDATES: parsePrefixLists handles list-form (slice of maps) entries and
// preserves order.
func TestParsePrefixLists_ListForm_OrderPreserved(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"prefix-list": map[string]any{
				"ORDERED": map[string]any{
					"name": "ORDERED",
					"entry": []any{
						map[string]any{"prefix": "10.0.0.0/8", "action": "reject"},
						map[string]any{"prefix": "10.0.0.0/8", "ge": float64(16), "le": float64(24), "action": "accept"},
					},
				},
			},
		},
	}

	lists, err := parsePrefixLists(bgpCfg)
	if err != nil {
		t.Fatalf("parsePrefixLists: %v", err)
	}
	ordered := lists["ORDERED"]
	if len(ordered.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ordered.entries))
	}
	if ordered.entries[0].action != actionReject || ordered.entries[1].action != actionAccept {
		t.Errorf("order lost: got %v, %v", ordered.entries[0].action, ordered.entries[1].action)
	}
}

// VALIDATES: ISSUE 2 fix -- map form with more than one entry is rejected
// because first-match-wins requires deterministic order.
// PREVENTS: Silently non-deterministic filter decisions when the config layer
// delivers an ordered-by-user YANG list as a Go map.
func TestParsePrefixLists_MultiEntryMapFormRejected(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"prefix-list": map[string]any{
				"MULTI": map[string]any{
					"name": "MULTI",
					"entry": map[string]any{
						"10.0.0.0/8":     map[string]any{"action": "accept"},
						"192.168.0.0/16": map[string]any{"action": "reject"},
					},
				},
			},
		},
	}

	_, err := parsePrefixLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for multi-entry map form, got nil")
	}
	if !strings.Contains(err.Error(), "would lose order") {
		t.Errorf("error %q does not mention order loss", err.Error())
	}
}
