package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestZipRowRoundTrip checks that a column schema and the values read against
// it survive the journey the wire puts between them. The names are written once
// on the head. Each row travels as values alone. A consumer rebuilds the object
// the two halves make.
//
// The method is to take one answer in its two forms. A schema-declaring
// producer writes the names on the head and each row as an array. A
// schema-less producer writes each row as an object carrying its own names. The
// two collapse to the same document.
//
// The declaring side reads its names back off the head BYTES rather than off
// the slice the head was built from, so a head that carried the wrong names is
// read here rather than assumed.
//
// The values name their own column, so a zip that paired name i with value j
// would put "as-value" under "peer". Two of the four are the same JSON type. A
// transposition between those two stays a well-formed row, so the comparison
// catches it rather than a decode failure.
//
// VALIDATES: R-6 of spec-record-answers-3-zero-alloc -- the names and the
// values round-trip, so a consumer zips the names the producer declared.
// PREVENTS: the positional row and the head's column names drifting, which
// checkRowArity cannot see: an arity that matches says nothing about which
// value belongs to which name.
func TestZipRowRoundTrip(t *testing.T) {
	t.Parallel()

	const key = "peers"
	fields := []string{"peer", "as", "state", "uptime"}
	rows := [][]string{
		{`"peer-value-1"`, `65001`, `"state-value-1"`, `11`},
		{`"peer-value-2"`, `65002`, `"state-value-2"`, `22`},
	}

	// The head as it reaches the wire, and the names as a consumer reads them
	// back off it.
	encoded, err := marshalAnswerFields(fields)
	if err != nil {
		t.Fatalf("marshalAnswerFields: %v", err)
	}
	head := AppendAnswerHead(nil, 7, AnswerTypeTable, key, encoded)
	_, kind, payload, err := ParseLine(head)
	if err != nil {
		t.Fatalf("ParseLine(%q): %v", head, err)
	}
	tail, err := ParseAnswerTail(kind, payload)
	if err != nil {
		t.Fatalf("ParseAnswerTail(%q): %v", head, err)
	}
	if tail.Type != AnswerTypeTable {
		t.Fatalf("the head states type %q, want %q", tail.Type, AnswerTypeTable)
	}
	if !slices.Equal(tail.Fields, fields) {
		t.Fatalf("the head carries fields %q, want %q", tail.Fields, fields)
	}

	// One row at a time: the names off the head over the values off the row.
	names, err := quoteFields(tail.Fields)
	if err != nil {
		t.Fatalf("quoteFields: %v", err)
	}
	for i, row := range rows {
		values := make([]json.RawMessage, len(row))
		for j := range row {
			values[j] = json.RawMessage(row[j])
		}
		object, zipErr := zipRow(names, values)
		if zipErr != nil {
			t.Fatalf("zipRow row %d: %v", i, zipErr)
		}
		if got, want := string(object), objectRow(fields, row); got != want {
			t.Errorf("row %d zipped to %s, want %s", i, got, want)
		}
	}

	// The whole answer: the two forms collapse to one document.
	positional := make([]Record, len(rows))
	selfDescribing := make([]Record, len(rows))
	for i, row := range rows {
		positional[i] = Record{Item: json.RawMessage(arrayRow(row))}
		selfDescribing[i] = Record{Item: json.RawMessage(objectRow(fields, row))}
	}
	fromColumns, err := CollapseRecords(key, tail.Fields, slices.Values(positional))
	if err != nil {
		t.Fatalf("CollapseRecords over positional rows: %v", err)
	}
	fromObjects, err := CollapseRecords(key, nil, slices.Values(selfDescribing))
	if err != nil {
		t.Fatalf("CollapseRecords over self-describing rows: %v", err)
	}
	if !bytes.Equal(fromColumns, fromObjects) {
		t.Errorf("the positional answer collapsed to\n%s\nwant\n%s", fromColumns, fromObjects)
	}

	// A row that does not agree with the schema is refused rather than zipped
	// against the wrong names.
	refused := []struct {
		name string
		row  []string
		want error
	}{
		{name: "one value short", row: rows[0][:3], want: ErrRowArity},
		{name: "one value long", row: append(slices.Clone(rows[0]), `"extra"`), want: ErrRowArity},
		{name: "no array at all", row: nil, want: ErrRowNotPositional},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			item := json.RawMessage(arrayRow(tc.row))
			if tc.row == nil {
				item = json.RawMessage(objectRow(fields, rows[0]))
			}
			_, err := CollapseRecords(key, fields, slices.Values([]Record{{Item: item}}))
			if !errors.Is(err, tc.want) {
				t.Errorf("CollapseRecords returned %v, want %v", err, tc.want)
			}
		})
	}
}

// arrayRow spells one positional row: the values, in column order, and nothing
// else.
func arrayRow(values []string) string {
	return "[" + strings.Join(values, ",") + "]"
}

// objectRow spells the same row as a self-describing producer writes it: each
// value under its own name, in the same column order, which is the order zipRow
// must answer in.
func objectRow(fields, values []string) string {
	var b strings.Builder
	b.WriteByte('{')
	for i := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		name, err := json.Marshal(fields[i])
		if err != nil {
			panic("BUG: a field name is not a JSON string: " + fields[i])
		}
		b.Write(name)
		b.WriteByte(':')
		b.WriteString(values[i])
	}
	b.WriteByte('}')
	return b.String()
}

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
