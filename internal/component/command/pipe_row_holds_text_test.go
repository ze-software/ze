// Design: docs/architecture/api/commands.md -- the one definition of what `| match` keeps
//
// Related: pipe.go -- RowHoldsText, rowHoldsText, matchRows

package command

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRowHoldsTextIsTheGenericMatch holds RowHoldsText, the predicate a
// command's `match` override calls over one row, to what the generic `| match`
// keeps over an answer of rows.
//
// METHOD: one answer of four rows. Each pattern is asked of every row through
// RowHoldsText, and of the whole answer through applyMatch, the generic. The
// rows RowHoldsText says yes to must be the rows applyMatch keeps, in order,
// and each case states those rows so no comparison is between two empty sets.
//
// The cases are the rules of the generic: a value is searched and a field
// name is not, case does not count, a number is searched by the text it was
// written with, a boolean by its word, a nested value is searched, and a text
// that spans two values is found when the values follow each other in the
// sorted order of their field names.
//
// MUTATION: make RowHoldsText search json.Marshal of the row (names and
// punctuation included) and `a field name` goes red; make rowHoldsText skip
// the space between two values and `text spanning two values` goes red.
func TestRowHoldsTextIsTheGenericMatch(t *testing.T) {
	rows := []string{
		`{"peer":"192.0.2.1","state":"Established","med":{"optional":true,"value":100}}`,
		`{"peer":"192.0.2.2","state":"idle","as-path":[65001,65002]}`,
		`{"peer":"192.0.2.3","state":"idle","counter":18446744073709551615}`,
		`{"peer":"192.0.2.4","note":"med"}`,
	}
	answer := `{"peers":[` + rows[0] + `,` + rows[1] + `,` + rows[2] + `,` + rows[3] + `]}`

	cases := []struct {
		name    string
		pattern string
		keeps   []int
	}{
		{"a value", "192.0.2.2", []int{1}},
		{"a field name", "state", nil},
		{"a name that is another row's value", "med", []int{3}},
		{"case", "ESTABLISHED", []int{0}},
		{"a number", "100", []int{0}},
		{"a number wider than a float", "18446744073709551615", []int{2}},
		{"a boolean", "true", []int{0}},
		{"a nested list", "65002", []int{1}},
		{"text spanning two values", "65001 65002 192.0.2.2 idle", []int{1}},
		{"text spanning a nested object", "true 100 192.0.2.1", []int{0}},
		{"every row", "192.0.2.", []int{0, 1, 2, 3}},
		{"no row", "absent", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var held []int
			for i, row := range rows {
				holds, err := RowHoldsText([]byte(row), tc.pattern)
				require.NoError(t, err)
				if holds {
					held = append(held, i)
				}
			}
			assert.Equal(t, tc.keeps, held, "the rows RowHoldsText says hold %q", tc.pattern)

			matched, refusal := applyMatch(answer, tc.pattern)
			require.Empty(t, refusal)
			var kept map[string][]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(matched), &kept), matched)
			require.Len(t, kept["peers"], len(tc.keeps), "the rows the generic keeps for %q", tc.pattern)
			for position, index := range tc.keeps {
				assert.JSONEq(t, rows[index], string(kept["peers"][position]),
					"the generic keeps the rows RowHoldsText says hold %q", tc.pattern)
			}
		})
	}
}

// TestRowHoldsTextRefusesWhatIsNotARow verifies RowHoldsText answers an error,
// and never false, for bytes that are not one JSON document.
//
// VALIDATES: a truncated document, two documents and no bytes at all are each
// an error.
// PREVENTS: an override reading "the row could not be read" as "the row does
// not hold the text" and dropping the route silently.
func TestRowHoldsTextRefusesWhatIsNotARow(t *testing.T) {
	for _, row := range []string{`{"peer":"192.0.2.1"`, `{"a":1} {"b":2}`, ``} {
		holds, err := RowHoldsText([]byte(row), "192.0.2.1")
		require.Error(t, err, "row %q", row)
		assert.False(t, holds)
	}
}
