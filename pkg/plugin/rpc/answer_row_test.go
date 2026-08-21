package rpc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestJSONArrayLengthCountsTopLevelElements checks the count the arity guard is
// built on. The method: arrays whose values hold the very bytes the scan looks
// for -- a comma inside a string, a bracket inside a nested value, an escaped
// quote -- are counted, and malformed input is refused.
//
// The scan exists because the row reaches the wire unchanged, so the element
// count is the only fact this path needs. A miscount there is a guard that
// passes a row it should refuse.
//
// VALIDATES: the element count is over top-level values alone, and a value that
// is not a well-formed array is refused rather than counted.
// PREVENTS: a comma inside a peer description counting as a column boundary,
// which would refuse every valid row of that answer.
func TestJSONArrayLengthCountsTopLevelElements(t *testing.T) {
	t.Parallel()

	counted := []struct {
		array string
		want  int
	}{
		{array: `[]`, want: 0},
		{array: `[ ]`, want: 0},
		{array: `[1]`, want: 1},
		{array: `[1,2,3]`, want: 3},
		{array: ` [ 1 , 2 ] `, want: 2},
		{array: `["10.0.0.1",65001,"established"]`, want: 3},
		{array: `["a,b,c"]`, want: 1},
		{array: `["a","b,c"]`, want: 2},
		{array: `["]","["]`, want: 2},
		{array: `["a\"b,c"]`, want: 1},
		{array: `["a\\"]`, want: 1},
		{array: `[[1,2],[3,4]]`, want: 2},
		{array: `[{"a":1,"b":2}]`, want: 1},
		{array: `[{"a":[1,2]},null,true]`, want: 3},
		{array: `[null]`, want: 1},
	}
	for _, tc := range counted {
		t.Run(tc.array, func(t *testing.T) {
			t.Parallel()

			got, err := jsonArrayLength([]byte(tc.array))
			if err != nil {
				t.Fatalf("jsonArrayLength(%s): %v", tc.array, err)
			}
			if got != tc.want {
				t.Errorf("jsonArrayLength(%s) is %d, want %d", tc.array, got, tc.want)
			}
		})
	}

	refused := []string{
		``,
		`   `,
		`{"peer":"10.0.0.1"}`,
		`"10.0.0.1"`,
		`17`,
		`[1,2`,
		`[1] and more`,
		`["unterminated]`,
		`[}]`,
		`[[1]]]`,
		`[,]`,
		`[1,]`,
		`[,1]`,
		`[1,,2]`,
	}
	for _, array := range refused {
		t.Run("refuses "+array, func(t *testing.T) {
			t.Parallel()

			if _, err := jsonArrayLength([]byte(array)); !errors.Is(err, ErrRowNotPositional) {
				t.Errorf("jsonArrayLength(%s) returned %v, want %v", array, err, ErrRowNotPositional)
			}
		})
	}
}

// TestZipRowKeepsTheColumnOrder checks that the buffered rendering of a
// positional row states its columns in the order the head declares. The method:
// a schema whose names sort against their column order is zipped, and the
// object is compared byte for byte.
//
// VALIDATES: the column order fields= carries survives into the buffered
// document.
// PREVENTS: a Go map sorting the keys, which would leave the two renderings of
// one answer disagreeing about column order for every schema not already in
// alphabetical order.
func TestZipRowKeepsTheColumnOrder(t *testing.T) {
	t.Parallel()

	names, err := quoteFields([]string{"peer", "as", "state"})
	if err != nil {
		t.Fatalf("quoteFields: %v", err)
	}

	row := []json.RawMessage{json.RawMessage(`"10.0.0.1"`), json.RawMessage(`65001`), json.RawMessage(`"established"`)}
	object, err := zipRow(names, row)
	if err != nil {
		t.Fatalf("zipRow: %v", err)
	}

	want := `{"peer":"10.0.0.1","as":65001,"state":"established"}`
	if string(object) != want {
		t.Errorf("zipRow rendered %s, want %s", object, want)
	}

	if _, err = zipRow(names, row[:2]); !errors.Is(err, ErrRowArity) {
		t.Errorf("zipRow of a short row returned %v, want %v", err, ErrRowArity)
	}
}

// TestQuoteFieldsEscapesTheName checks that a column name reaches the wire as a
// JSON string. The method: a name holding a quote and a backslash is encoded
// and read back.
//
// VALIDATES: a field name needs no restriction to be carried.
// PREVENTS: a name breaking the JSON object a consumer rebuilds from it.
func TestQuoteFieldsEscapesTheName(t *testing.T) {
	t.Parallel()

	names, err := quoteFields([]string{`say "peer"`, `back\slash`})
	if err != nil {
		t.Fatalf("quoteFields: %v", err)
	}

	for i, want := range []string{`say "peer"`, `back\slash`} {
		var got string
		if err = json.Unmarshal(names[i], &got); err != nil {
			t.Fatalf("field %d does not read back as a JSON string: %v", i, err)
		}
		if got != want {
			t.Errorf("field %d reads back as %q, want %q", i, got, want)
		}
		if strings.Contains(string(names[i][1:len(names[i])-1]), `"`) && !strings.Contains(string(names[i]), `\"`) {
			t.Errorf("field %d is not escaped: %s", i, names[i])
		}
	}
}
