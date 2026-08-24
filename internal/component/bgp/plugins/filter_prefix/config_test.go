package filter_prefix

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/configorder"
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
			prefixStr, _ := tt.in["prefix"].(string)
			got, err := parseOneEntry("test-list", prefixStr, tt.in)
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

// TestParseConfig_StringValuedDelivery drives the full parse path
// (parsePrefixLists) with the JSON-string leaf values the plugin config
// framework actually delivers ("16", "24"): Tree.values is map[string]string,
// so numeric leaves arrive as strings. readUint already coerces strings, so
// this is a regression guard at the public entry point (not a fix); it pins the
// behavior end-to-end where TestParseOneEntry only exercises the leaf helper.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	bgpCfg := map[string]any{
		"policy": map[string]any{
			"prefix-list": map[string]any{
				"L": map[string]any{
					"entry": map[string]any{
						"10.0.0.0/8": map[string]any{"ge": "16", "le": "24", "action": "accept"},
					},
				},
			},
		},
	}
	lists, err := parsePrefixLists(bgpCfg)
	if err != nil {
		t.Fatalf("parsePrefixLists: %v", err)
	}
	pl, ok := lists["L"]
	if !ok {
		t.Fatalf("prefix-list L missing; got %v", lists)
	}
	if len(pl.entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(pl.entries))
	}
	e := pl.entries[0]
	if e.ge != 16 {
		t.Errorf("ge = %d, want 16 (string-valued)", e.ge)
	}
	if e.le != 24 {
		t.Errorf("le = %d, want 24 (string-valued)", e.le)
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

// VALIDATES: a multi-entry list delivered with NO order is still rejected.
// The refusal now lives in configorder, because that is where the order is
// read, and it is worded by its new owner. What it guards is unchanged.
// PREVENTS: Silently non-deterministic filter decisions when the config layer
// delivers an ordered-by-user YANG list as a Go map. This is the fail-closed
// half of the fix: carrying the order makes the common case work, and this
// keeps the uncommon one loud instead of arbitrary.
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
	if !strings.Contains(err.Error(), "no order") {
		t.Errorf("error %q does not say the order was missing", err.Error())
	}
	if !strings.Contains(err.Error(), "MULTI") {
		t.Errorf("error %q does not name the offending prefix-list", err.Error())
	}
}

// orderedTwoEntryList builds the payload the plugin-facing lowering delivers
// for a two-entry prefix-list, with the order key beside the list.
//
// The operator writes the specific reject entry first and the catch-all second,
// and the two keys are in the opposite order to the alphabet. So a reader that
// sorted the keyed map would evaluate 0.0.0.0/0 first, accept every route, and
// never reach the reject entry. Sorting cannot pass this fixture by luck.
func orderedTwoEntryList(order []string) map[string]any {
	return map[string]any{
		"policy": map[string]any{
			"prefix-list": map[string]any{
				"ORDERED": map[string]any{
					"name": "ORDERED",
					"entry": map[string]any{
						"10.0.0.0/8": map[string]any{"le": float64(32), "action": "reject"},
						"0.0.0.0/0":  map[string]any{"le": float64(32), "action": "accept"},
					},
					configorder.OrderKey("entry"): order,
				},
			},
		},
	}
}

// TestParsePrefixListsTwoEntriesInNonLexicalOrder loads a two-entry
// prefix-list, then loads the same two entries in the opposite order.
//
// VALIDATES: AC-1 and AC-2. A prefix-list of two or more entries loads, and the
// entry the operator wrote first is the one first-match-wins evaluates first.
// The second half is what discriminates: the same two entries with the order
// reversed produce the reverse decision, so the result is read from the
// delivered order rather than reconstructed from the keys.
// PREVENTS: the reported defect. Before this, a prefix-list of two entries
// could not load at all, which made the feature unusable past one entry.
func TestParsePrefixListsTwoEntriesInNonLexicalOrder(t *testing.T) {
	for _, tc := range []struct {
		name       string
		order      []string
		wantFirst  string
		wantAction action
	}{
		{"as the operator wrote them", []string{"10.0.0.0/8", "0.0.0.0/0"}, "10.0.0.0/8", actionReject},
		{"with the two entries swapped", []string{"0.0.0.0/0", "10.0.0.0/8"}, "0.0.0.0/0", actionAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lists, err := parsePrefixLists(orderedTwoEntryList(tc.order))
			if err != nil {
				t.Fatalf("parsePrefixLists: %v", err)
			}
			ordered, ok := lists["ORDERED"]
			if !ok {
				t.Fatal("ORDERED list missing")
			}
			if len(ordered.entries) != 2 {
				t.Fatalf("got %d entries, want 2", len(ordered.entries))
			}
			if got := ordered.entries[0].prefix.String(); got != tc.wantFirst {
				t.Errorf("first entry is %s, want %s", got, tc.wantFirst)
			}
			if ordered.entries[0].action != tc.wantAction {
				t.Errorf("first entry action is %v, want %v", ordered.entries[0].action, tc.wantAction)
			}
		})
	}
}

// TestParsePrefixListsEvaluatesInTheOperatorsOrder runs the evaluator, not the
// parser, over the same two orderings.
//
// VALIDATES: AC-1 and AC-2 as an observable decision. 10.1.0.0/16 is inside
// 10.0.0.0/8 and inside 0.0.0.0/0, so it matches both entries and only the
// order decides whether it is rejected or accepted.
// PREVENTS: a parser that holds the right order feeding an evaluator that
// ignores it. Asserting on entries alone would not catch that.
//
// The two rows discriminate as a PAIR, and neither would alone. An empty list
// evaluates to actionReject by implicit deny, so the reject row passes even
// when no entry was parsed at all. The accept row is what proves the entries
// are there and reachable, and it can only pass when 0.0.0.0/0 is evaluated
// first.
func TestParsePrefixListsEvaluatesInTheOperatorsOrder(t *testing.T) {
	route := netip.MustParsePrefix("10.1.0.0/16")

	for _, tc := range []struct {
		name  string
		order []string
		want  action
	}{
		{"reject entry written first wins", []string{"10.0.0.0/8", "0.0.0.0/0"}, actionReject},
		{"accept entry written first wins", []string{"0.0.0.0/0", "10.0.0.0/8"}, actionAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lists, err := parsePrefixLists(orderedTwoEntryList(tc.order))
			if err != nil {
				t.Fatalf("parsePrefixLists: %v", err)
			}
			if got := evaluatePrefix(lists["ORDERED"].entries, route); got != tc.want {
				t.Errorf("first match is %v, want %v", got, tc.want)
			}
		})
	}
}
