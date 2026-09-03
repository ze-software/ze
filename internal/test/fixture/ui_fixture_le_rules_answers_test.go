package fixture

import (
	"encoding/json"
	"testing"
)

// TestListLengthReadsNullAsNoRows pins the shape every list on these reports
// takes when it is healthy.
//
// The gate map and the lint answer their lists as Go slices with no omitempty,
// so "no dangling binding" and "no empty population" reach the fixture as JSON
// null rather than as []. Reading null as "not a list" made each check fail on
// exactly the state it exists to confirm, and say so with the healthy value in
// the message: "a binding names no point: <nil>".
//
// The unmarshal is the point of the test. Asserting against a hand-written nil
// would prove the branch and not the claim, which is about what encoding/json
// hands this fixture for a nil slice.
//
// VALIDATES: null counts as a list of none; an empty and a populated list count
// as themselves; a value that is not a list is refused.
// PREVENTS: restoring a reading under which the fixture's dangling, empty and
// missing-rationale checks can never pass.
func TestListLengthReadsNullAsNoRows(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantLen  int
		wantList bool
	}{
		{"null_is_no_rows", `{"k":null}`, 0, true},
		{"absent_is_no_rows", `{}`, 0, true},
		{"empty_list", `{"k":[]}`, 0, true},
		{"populated_list", `{"k":["a","b","c"]}`, 3, true},
		{"a_bool_is_not_a_list", `{"k":true}`, 0, false},
		{"a_number_is_not_a_list", `{"k":7}`, 0, false},
		{"a_string_is_not_a_list", `{"k":"none"}`, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var answer map[string]any
			if err := json.Unmarshal([]byte(tt.payload), &answer); err != nil {
				t.Fatalf("unmarshal %s: %v", tt.payload, err)
			}
			count, isList := listLength(answer["k"])
			if isList != tt.wantList {
				t.Errorf("isList = %v, want %v for %s", isList, tt.wantList, tt.payload)
			}
			if count != tt.wantLen {
				t.Errorf("count = %d, want %d for %s", count, tt.wantLen, tt.payload)
			}
		})
	}
}
