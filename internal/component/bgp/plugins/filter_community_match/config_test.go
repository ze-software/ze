package filter_community_match

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/configorder"
)

// Config spelling must not change the value matched against formatted UPDATEs.
func TestConfiguredCommunityMatchesAttributeValue(t *testing.T) {
	target20 := [8]byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 20}
	target10 := [8]byte{0, 2, 0xfd, 0xe8, 0, 0, 0, 10}
	origin20 := [8]byte{0, 3, 0xfd, 0xe8, 0, 0, 0, 20}
	noExport := standardCommunities(0xffffff01)
	noAdvertise := standardCommunities(0xffffff02)

	for _, tc := range []struct {
		name, configured, kind string
		matching, other        []byte
	}{
		{"standard", "65001:100", "standard",
			standardCommunities(65001<<16 | 100), standardCommunities(65001<<16 | 200)},
		{"well-known name", "no-export", "", noExport, noAdvertise},
		{"numeric well-known", "65535:65281", "", noExport, noAdvertise},
		{"large decimal", "065001:00100:00200", "large",
			largeCommunity(65001, 100, 200), largeCommunity(65001, 100, 201)},
		{"route target", "target:65000:20", "extended", extendedCommunities(target20), extendedCommunities(target10)},
		{"site of origin", "origin:65000:20", "extended", extendedCommunities(origin20), extendedCommunities(target20)},
		{"raw extended hex", "0002FDE800000014", "extended", extendedCommunities(target20), extendedCommunities(target10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]any{}
			if tc.kind != "" {
				entry["type"] = tc.kind
			}
			listCfg := map[string]any{"entry": map[string]any{tc.configured: entry}}
			if _, err := parseCommunityLists(map[string]any{
				"community-match": map[string]any{"SERVICE": listCfg},
			}); err != nil {
				t.Fatal(err)
			}
			if got := judgeList(t, listCfg, tc.matching); got != actionAccept {
				t.Fatalf("matching attribute rejected: %x", tc.matching)
			}
			if got := judgeList(t, listCfg, tc.other); got != actionReject {
				t.Fatalf("different attribute accepted: %x", tc.other)
			}
		})
	}
}

func TestParseOneCommunityEntryRejectsInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name, value, kind, action string
	}{
		{"missing value", "", "", ""},
		{"invalid type", "65001:100", "bogus", ""},
		{"invalid action", "65001:100", "", "permit"},
		{"too long", strings.Repeat("x", maxCommunityLen+1), "", ""},
		{"standard overflow", "65536:1", "standard", ""},
		{"unknown name", "no-exprot", "standard", ""},
		{"large overflow", "1:2:4294967296", "large", ""},
		{"short extended hex", "0002fde8000000", "extended", ""},
		{"invalid extended hex", "0002fde8000000gg", "extended", ""},
		{"unknown extended name", "targte:65000:20", "extended", ""},
		{"extended overflow", "target:65000:4294967296", "extended", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := map[string]any{}
			if tc.kind != "" {
				entry["type"] = tc.kind
			}
			if tc.action != "" {
				entry["action"] = tc.action
			}
			_, err := parseOneCommunityEntry("SERVICE", tc.value, entry)
			if err == nil {
				t.Fatal("invalid community match configuration accepted")
			}
		})
	}
}

// VALIDATES: List-form entries preserve order.
func TestParseCommunityLists_ListForm_OrderPreserved(t *testing.T) {
	bgpCfg := map[string]any{
		"community-match": map[string]any{
			"ORDERED": map[string]any{
				"name": "ORDERED",
				"entry": []any{
					map[string]any{"community": "65001:100", "action": "reject"},
					map[string]any{"community": "65001:200", "action": "accept"},
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
		t.Fatalf("parsed %d entries, want 2", len(ordered.entries))
	}
	listCfg, _ := bgpCfg["community-match"].(map[string]any)["ORDERED"].(map[string]any)
	if got := judgeList(t, listCfg, standardCommunities(65001<<16|100, 65001<<16|200)); got != actionReject {
		t.Fatalf("first matching entry did not reject: %v", got)
	}
	if got := judgeList(t, listCfg, standardCommunities(65001<<16|200)); got != actionAccept {
		t.Fatalf("second matching entry did not accept: %v", got)
	}
}

// VALIDATES: Multi-entry map form rejected (ordering loss).
func TestParseCommunityLists_MultiEntryMapFormRejected(t *testing.T) {
	bgpCfg := map[string]any{
		"community-match": map[string]any{
			"MULTI": map[string]any{
				"name": "MULTI",
				"entry": map[string]any{
					"65001:100": map[string]any{"action": "accept"},
					"65001:200": map[string]any{"action": "reject"},
				},
			},
		},
	}

	_, err := parseCommunityLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for multi-entry map form, got nil")
	}
}

// VALIDATES: Name length limit enforced.
func TestParseCommunityLists_NameTooLong(t *testing.T) {
	longName := strings.Repeat("x", maxNameLen+1)
	bgpCfg := map[string]any{
		"community-match": map[string]any{
			longName: map[string]any{
				"entry": map[string]any{
					"65001:100": map[string]any{"action": "accept"},
				},
			},
		},
	}

	_, err := parseCommunityLists(bgpCfg)
	if err == nil {
		t.Fatal("expected error for long name, got nil")
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

// TestParseCommunityListsTwoEntriesInNonLexicalOrder loads a two-entry
// community-match, then loads the same two entries in the opposite order.
//
// VALIDATES: AC-3. A community-match of two or more entries loads, and the
// entry the operator wrote first is the one evaluated first.
// PREVENTS: the reported defect on this reader. The two keys are in the
// opposite order to the alphabet, so a reader that sorted them would return the
// same answer for both rows.
func TestParseCommunityListsTwoEntriesInNonLexicalOrder(t *testing.T) {
	list := func(order []string) map[string]any {
		return map[string]any{
			"community-match": map[string]any{
				"ORDERED": map[string]any{
					"name": "ORDERED",
					"entry": map[string]any{
						"65001:200": map[string]any{"action": "reject"},
						"65001:100": map[string]any{"action": "accept"},
					},
					configorder.OrderKey("entry"): order,
				},
			},
		}
	}

	for _, tc := range []struct {
		name       string
		order      []string
		wantAction action
	}{
		{"reject entry written first", []string{"65001:200", "65001:100"}, actionReject},
		{"accept entry written first", []string{"65001:100", "65001:200"}, actionAccept},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lists, err := parseCommunityLists(list(tc.order))
			if err != nil {
				t.Fatalf("parseCommunityLists: %v", err)
			}
			if _, ok := lists["ORDERED"]; !ok {
				t.Fatal("ORDERED list missing")
			}
			listCfg, _ := list(tc.order)["community-match"].(map[string]any)["ORDERED"].(map[string]any)
			if got := judgeList(t, listCfg, standardCommunities(65001<<16|100, 65001<<16|200)); got != tc.wantAction {
				t.Errorf("filter action is %v, want %v", got, tc.wantAction)
			}
		})
	}
}
