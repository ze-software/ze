package filter_modify

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
)

// TestModifyKeyFieldCoverage: every key the modify parser accepts maps onto
// the route field it reads or sets, so the per-instance field check
// (modifyInstanceFields) can never pass an operation it does not see. Method:
// compare the parser's key sets with the field maps in both directions, then
// check modifyInstanceFields over one instance of each block.
func TestModifyKeyFieldCoverage(t *testing.T) {
	for key := range setBlockAllowedKeys {
		if _, ok := setKeyFields[key]; !ok {
			t.Errorf("set key %q is accepted but maps onto no field", key)
		}
	}
	for key := range ribSetKeys {
		if _, ok := setKeyFields[key]; !ok {
			t.Errorf("Loc-RIB set key %q maps onto no field", key)
		}
		if setBlockAllowedKeys[key] {
			t.Errorf("Loc-RIB set key %q is also a BGP set key, so parseModifyDefs would apply it to UPDATEs", key)
		}
	}
	for key := range setKeyFields {
		if !setBlockAllowedKeys[key] && !ribSetKeys[key] {
			t.Errorf("field map names set key %q that the parser refuses", key)
		}
	}
	for key := range incDecAttrs {
		if _, ok := incDecKeyFields[key]; !ok {
			t.Errorf("increment/decrement key %q maps onto no field", key)
		}
	}
	for key := range incDecKeyFields {
		if !incDecAttrs[key] {
			t.Errorf("field map names increment/decrement key %q that the parser refuses", key)
		}
	}

	for _, tc := range []struct {
		name        string
		instance    map[string]any
		reads, sets []policy.Field
	}{
		{"set distance", map[string]any{"set": map[string]any{"distance": "90"}}, nil, []policy.Field{policy.FieldDistance}},
		{"set tag", map[string]any{"set": map[string]any{"tag": "7"}}, nil, []policy.Field{policy.FieldTag}},
		{"set community", map[string]any{"set": map[string]any{communityAddDirective: []any{"65000:1"}}}, nil, []policy.Field{policy.FieldCommunity}},
		{"match tag", map[string]any{"match": map[string]any{"tag": []any{"1"}}, "set": map[string]any{"distance": "90"}},
			[]policy.Field{policy.FieldTag}, []policy.Field{policy.FieldDistance}},
		{"match community", map[string]any{"match": map[string]any{"community": []any{"65000:1"}}, "set": map[string]any{"distance": "90"}},
			[]policy.Field{policy.FieldCommunity}, []policy.Field{policy.FieldDistance}},
		{"increment med", map[string]any{"increment": map[string]any{medAttr: "10"}}, nil, []policy.Field{policy.FieldMED}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, sets := modifyInstanceFields(tc.instance)
			for _, f := range tc.reads {
				if !slices.Contains(reads, f) {
					t.Errorf("reads %v, missing %v", reads, f)
				}
			}
			for _, f := range tc.sets {
				if !slices.Contains(sets, f) {
					t.Errorf("sets %v, missing %v", sets, f)
				}
			}
			if len(reads) != len(tc.reads) || len(sets) != len(tc.sets) {
				t.Errorf("reads %v sets %v, want exactly %v and %v", reads, sets, tc.reads, tc.sets)
			}
		})
	}
}
