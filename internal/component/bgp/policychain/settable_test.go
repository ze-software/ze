package policychain

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
)

// TestSettableFieldsAreTheSetKinds proves the settable half of D-9: the
// fields a filter may set at a BGP attachment are exactly the fields a Sets
// kind writes, minus the per-route intents a member answers by reading
// (prefix, family), so no second list of settable fields can disagree with
// what the chain applies. Method: every kind is classified from the kind
// table itself, and SettableFields must hold each writable one once and no
// other field.
func TestSettableFieldsAreTheSetKinds(t *testing.T) {
	settable := SettableFields()
	var want []policy.Field
	for kind := setKindLocalPreference; kind < setKindCount; kind++ {
		field := setKindFields[kind]
		if intentsOfReads([]policy.Field{field}) != 0 {
			if slices.Contains(settable, field) {
				t.Errorf("SettableFields holds %s, a per-route intent of a read, not a set", field)
			}
			continue
		}
		want = append(want, field)
	}
	if len(want) == 0 {
		t.Fatal("no writable set kind: the kind table is empty")
	}
	if len(settable) != len(want) {
		t.Fatalf("SettableFields = %v, want the writable set kinds %v", settable, want)
	}
	for _, field := range want {
		if !slices.Contains(settable, field) {
			t.Errorf("SettableFields lacks %s, which a Sets kind writes", field)
		}
	}
	for _, field := range settable {
		if maskOf([]policy.Field{field}) == 0 {
			t.Errorf("SettableFields holds %s, which no Sets kind writes", field)
		}
	}
}
