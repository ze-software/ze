// Design: docs/architecture/config/syntax.md -- internal running intent and public active data.
package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/configorder"
)

// runningIntentTree supplies inactive members at root, container and keyed-list
// entry depth, including a single-entry list whose shape must remain unchanged.
func runningIntentTree() *Tree {
	root, policy, only := NewTree(), NewTree(), NewTree()
	root.SetMemberStates("global-members", []MemberState{{Value: "root-live"}, {Value: "root-off", Inactive: true}})
	root.SetContainer("policy", policy)
	only.SetMemberStates("filters", []MemberState{{Value: "first-off", Inactive: true}, {Value: "operator:keep"}, {Value: "last-off", Inactive: true}})
	policy.AddListEntry("rule", "only", only)
	for _, key := range []string{"z-first", "a-second"} {
		entry := NewTree()
		entry.Set("action", "accept")
		policy.AddListEntry("ordered", key, entry)
	}
	return root
}

// TestRunningMapPreservesInactiveMemberIntent reconstructs the actual map
// projection directly and after JSON transport. Raw member order and inactive
// state survive, while the one-entry and ordered-list shape stay intact.
func TestRunningMapPreservesInactiveMemberIntent(t *testing.T) {
	root := runningIntentTree()
	for _, wire := range []bool{false, true} {
		input := root.ToRunningMap()
		if wire {
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &input); err != nil {
				t.Fatal(err)
			}
		}
		restored, err := TreeFromPluginMap(input)
		if err != nil {
			t.Fatal(err)
		}
		if got := restored.GetMultiValuesState("global-members"); !reflect.DeepEqual(got, root.GetMultiValuesState("global-members")) {
			t.Errorf("wire=%t: root member intent changed: %v", wire, got)
		}
		rules := restored.GetContainer("policy").GetListOrdered("rule")
		if len(rules) != 1 || rules[0].Key != "only" {
			t.Fatalf("wire=%t: single-entry list shape changed: %v", wire, rules)
		}
		want := root.GetContainer("policy").GetList("rule")["only"].GetMultiValuesState("filters")
		if got := rules[0].Value.GetMultiValuesState("filters"); !reflect.DeepEqual(got, want) {
			t.Errorf("wire=%t: filter member order/state changed: %v", wire, got)
		}
	}
}

// TestRunningMapPublicSectionsStayActive exercises whole, nested, direct-list,
// direct-leaf and wildcard public sections. Active members and list order remain
// useful, but no private inactive member or metadata key crosses the boundary.
func TestRunningMapPublicSectionsStayActive(t *testing.T) {
	root := runningIntentTree()
	running := root.ToRunningMap()
	sections, err := BuildPluginConfigSections(running, []string{"*", "policy", "policy/rule", "policy/rule/only/filters"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 4 {
		t.Fatalf("got %d sections, want four", len(sections))
	}
	for _, section := range sections {
		if !strings.Contains(section.Data, "operator:keep") {
			t.Errorf("%s lost its active member: %s", section.Root, section.Data)
		}
		for _, private := range []string{"!inactive-members", "root-off", "first-off", "last-off"} {
			if strings.Contains(section.Data, private) {
				t.Errorf("%s leaked %s: %s", section.Root, private, section.Data)
			}
		}
	}
	public, ok := root.ToPluginMap()["policy"].(map[string]any)
	if !ok {
		t.Fatal("public policy section is not a map")
	}
	entries, err := configorder.Entries(public, "ordered", "name")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Key != "z-first" || entries[1].Key != "a-second" {
		t.Fatalf("public ordered list changed: %v", entries)
	}
}

// TestRunningMetadataPreservesSameNamedEntriesAndMemberTokens distinguishes a
// real map-valued list entry from array metadata and preserves exact member
// tokens, including repeated, empty and multiword values, through JSON as well.
func TestRunningMetadataPreservesSameNamedEntriesAndMemberTokens(t *testing.T) {
	root, entry := NewTree(), NewTree()
	states := []MemberState{{Value: ""}, {Value: "two words"}, {Value: "two words"}, {Value: "off", Inactive: true}}
	entry.SetMemberStates("filters", states)
	root.AddListEntry("entries", "!inactive-members", entry)
	for _, wire := range []bool{false, true} {
		running := root.ToRunningMap()
		if wire {
			body, err := json.Marshal(running)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &running); err != nil {
				t.Fatal(err)
			}
		}
		restored, err := TreeFromPluginMap(running)
		if err != nil {
			t.Fatal(err)
		}
		entries := restored.GetListOrdered("entries")
		if len(entries) != 1 || entries[0].Key != "!inactive-members" {
			t.Fatalf("wire=%t: legitimate same-name entry was erased: %v", wire, entries)
		}
		if got := entries[0].Value.GetMultiValuesState("filters"); !reflect.DeepEqual(got, states) {
			t.Errorf("wire=%t: member tokens/order/inactivity changed: %v", wire, got)
		}
		sections, err := BuildPluginConfigSections(running, []string{"entries/!inactive-members"})
		if err != nil {
			t.Fatal(err)
		}
		if len(sections) != 1 || !strings.Contains(sections[0].Data, `"!inactive-members"`) || !strings.Contains(sections[0].Data, "two words") {
			t.Fatalf("wire=%t: public same-name entry or active tokens were lost: %v", wire, sections)
		}
		if strings.Contains(sections[0].Data, `"off"`) || strings.Contains(sections[0].Data, `"leaf"`) {
			t.Errorf("wire=%t: inactive metadata leaked inside real entry: %s", wire, sections[0].Data)
		}
	}
}

// TestRunningMapRefusesContradictoryOrMalformedIntent checks the reconstruction
// boundary, not a private parser: a failed decode must publish no partial Tree.
func TestRunningMapRefusesContradictoryOrMalformedIntent(t *testing.T) {
	for _, defect := range []string{"active disagreement", "duplicate leaf", "absent inactive member", "duplicate inactive member", "nonstring member"} {
		t.Run(defect, func(t *testing.T) {
			running := runningIntentTree().ToRunningMap()
			records, ok := running["!inactive-members"].([]any)
			if !ok || len(records) != 1 {
				t.Fatalf("running projection did not emit root metadata: %v", running)
			}
			record, ok := records[0].(map[string]any)
			if !ok {
				t.Fatalf("metadata record is %T", records[0])
			}
			switch defect {
			case "active disagreement":
				running["global-members"] = "different-live"
			case "duplicate leaf":
				running["!inactive-members"] = append(records, record)
			case "absent inactive member":
				record["inactive"] = []string{"not-a-member"}
			case "duplicate inactive member":
				record["inactive"] = []string{"root-off", "root-off"}
			case "nonstring member":
				record["members"] = []any{42}
			}
			restored, err := TreeFromPluginMap(running)
			if err == nil {
				t.Fatalf("reconstruction accepted %s", defect)
			}
			if restored != nil {
				t.Fatalf("reconstruction published a partial tree for %s", defect)
			}
		})
	}
}
