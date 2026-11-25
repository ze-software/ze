// Design: docs/architecture/config/transaction-protocol.md -- delivered config reconstruction
package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/configorder"
)

// Reconstructing a delivered section must preserve policy order and leaf-list
// boundaries on both transport forms. Repeated sequence members remain repeated.
func TestTreeFromPluginMapPreservesDeliveredOrder(t *testing.T) {
	root := NewTree()
	policy := NewTree()
	root.SetContainer("policy", policy)
	deny := NewTree()
	deny.Set("action", "deny")
	deny.SetSlice("subjects", []string{"two words", "one", "one"})
	policy.AddListEntry("rule", "z-deny", deny)
	allow := NewTree()
	allow.Set("action", "allow")
	allow.SetSlice("subjects", []string{"one token"})
	policy.AddListEntry("rule", "a-allow", allow)
	for _, wire := range []bool{false, true} {
		input := root.ToPluginMap()
		if wire {
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(encoded, &input); err != nil {
				t.Fatal(err)
			}
		}
		restored, err := TreeFromPluginMap(input)
		if err != nil {
			t.Fatalf("wire=%t: %v", wire, err)
		}
		rules := restored.GetContainer("policy").GetListOrdered("rule")
		if len(rules) != 2 {
			t.Fatalf("wire=%t: rules=%v", wire, rules)
		}
		if rules[0].Key != "z-deny" || rules[1].Key != "a-allow" {
			t.Fatalf("wire=%t: policy order changed: %v", wire, rules)
		}
		if got := rules[0].Value.GetSlice("subjects"); !reflect.DeepEqual(got, []string{"two words", "one", "one"}) {
			t.Fatalf("wire=%t: sequence changed: %q", wire, got)
		}
		if got := rules[1].Value.GetSlice("subjects"); !reflect.DeepEqual(got, []string{"one token"}) {
			t.Fatalf("wire=%t: singleton token changed: %q", wire, got)
		}
		if got, ok := rules[1].Value.Get("action"); !ok || got != "allow" {
			t.Fatalf("wire=%t: scalar action changed: %q, present=%t", wire, got, ok)
		}
		policy, ok := restored.ToPluginMap()["policy"].(map[string]any)
		if !ok {
			t.Fatalf("wire=%t: policy is not a map: %#v", wire, restored.ToPluginMap()["policy"])
		}
		entries, err := configorder.Entries(policy, "rule", "name")
		if err != nil {
			t.Fatal(err)
		}
		if entries[0].Key != "z-deny" || entries[1].Key != "a-allow" {
			t.Fatalf("wire=%t: re-delivery changed order: %v", wire, entries)
		}
	}
}

// Missing order cannot turn into map iteration order; malformed delivered order
// must fail rather than selecting only part of a first-match policy.
func TestTreeFromPluginMapDoesNotInventOrder(t *testing.T) {
	input := map[string]any{"rule": map[string]any{
		"first": map[string]any{"action": "deny"},
		"last":  map[string]any{"action": "allow"},
	}}
	restored, err := TreeFromPluginMap(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configorder.Entries(restored.ToPluginMap(), "rule", "name"); err == nil {
		t.Fatal("missing order became an invented policy order")
	}
	input[configorder.OrderKey("rule")] = []string{"first", "first"}
	if _, err := TreeFromPluginMap(input); err == nil {
		t.Fatal("duplicated order entry was accepted")
	}
}

// Scalar leaves whose values contain whitespace (for example NLRI content or
// descriptions) must remain scalar strings across repeated ToRunningMap ->
// TreeFromPluginMap round-trips, while genuine single-member leaf-lists with
// spaces remain slices so multi-word members are never split.
func TestTreeFromPluginMapPreservesWhitespaceScalarsAcrossRoundTrips(t *testing.T) {
	root := NewTree()
	entry := NewTree()
	entry.Set("content", "add 10.0.1.0/24 10.0.2.0/24")
	entry.SetSlice("allow", []string{"cli:show bgp peer"})
	root.SetContainer("entry", entry)

	cur := root
	for round := 1; round <= 3; round++ {
		m := cur.ToRunningMap()
		entryMap, ok := m["entry"].(map[string]any)
		if !ok {
			t.Fatalf("round %d: entry is not a map: %#v", round, m["entry"])
		}
		if got, ok := entryMap["content"].(string); !ok || got != "add 10.0.1.0/24 10.0.2.0/24" {
			t.Fatalf("round %d: scalar content changed type/value: %#v", round, entryMap["content"])
		}
		if got, ok := entryMap["allow"].([]string); !ok || !reflect.DeepEqual(got, []string{"cli:show bgp peer"}) {
			t.Fatalf("round %d: singleton leaf-list allow changed type/value: %#v", round, entryMap["allow"])
		}
		var err error
		cur, err = TreeFromPluginMap(m)
		if err != nil {
			t.Fatalf("round %d: TreeFromPluginMap: %v", round, err)
		}
		gotScalar, ok := cur.GetContainer("entry").Get("content")
		if !ok || gotScalar != "add 10.0.1.0/24 10.0.2.0/24" {
			t.Fatalf("round %d: reconstructed scalar content = %q (ok=%t)", round, gotScalar, ok)
		}
		if gotSlice := cur.GetContainer("entry").GetSlice("allow"); !reflect.DeepEqual(gotSlice, []string{"cli:show bgp peer"}) {
			t.Fatalf("round %d: reconstructed leaf-list allow = %q", round, gotSlice)
		}
	}
}
