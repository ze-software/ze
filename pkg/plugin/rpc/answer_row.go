// Design: docs/architecture/api/ipc_protocol.md -- the answer wire grammar
// Related: collapse.go -- the document those rows collapse to
//
// The positional row of a streamed answer.
//
// A streamed answer whose rows share one schema carries the column names once,
// on its head, and each row as an array of values read against them. This file
// holds what that costs: the check that a row and the schema agree, and the
// zip that renders one such row as the object a buffered consumer reads.
//
// It sits beside the appenders that write those rows (message.go), because a
// producer and a consumer of the answer grammar need the same two halves: the
// wire writer holds each row to the schema (writeRecordLine, answer_write.go)
// and a buffered reader rebuilds a document from it (CollapseRecords,
// collapse.go).

package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrRowArity is what a row whose length disagrees with the answer's fields
// earns, on the record path and on the buffered one alike. The two are read
// against each other by POSITION, so a short row would gain a column it never
// carried and a long one would lose a value. Neither is repaired here: a
// producer that miscounts is named at the producer, rather than at whichever
// consumer meets the row (`ai/rules/evidence.md`).
var ErrRowArity = errors.New("answer row length disagrees with the fields the head declares")

// ErrRowNotPositional is what a row that is not a JSON array earns in an answer
// that declares fields. The head says every row is read by position, so an
// object row would be read against the wrong schema rather than not at all.
var ErrRowNotPositional = errors.New("answer row is not a positional array")

// CheckRowArity refuses a row that does not agree with the column schema the
// head declares. An answer that declares no fields carries self-describing
// rows, and there is nothing to disagree with.
func CheckRowArity(item json.RawMessage, fields []string) error {
	if len(fields) == 0 {
		return nil
	}
	values, err := jsonArrayLength(item)
	if err != nil {
		return err
	}
	if values != len(fields) {
		return fmt.Errorf("%w: the row carries %d values and the head declares %d fields", ErrRowArity, values, len(fields))
	}
	return nil
}

// jsonArrayLength counts the top-level elements of the JSON array in item.
//
// It scans rather than decodes, because it runs for every row of a streamed
// answer and that row reaches the wire unchanged: the element count is the only
// fact this path needs, and decoding would copy every value to learn it. The
// scan is bounded by len(item), which the frame reader has already bounded.
func jsonArrayLength(item []byte) (int, error) {
	index := skipJSONSpace(item, 0)
	if index == len(item) || item[index] != '[' {
		return 0, ErrRowNotPositional
	}
	index++

	elements := 0
	depth := 0
	seenValue := false
	for index < len(item) {
		switch item[index] {
		case '"':
			end, closed := jsonStringEnd(item, index)
			if !closed {
				return 0, ErrRowNotPositional
			}
			seenValue = true
			index = end
			continue
		case '[', '{':
			seenValue = true
			depth++
		case '}':
			depth--
		case ']':
			if depth == 0 {
				// A closing bracket after a comma with no value between them is
				// a trailing comma, which is not JSON and would be counted as a
				// column the row does not carry.
				if !seenValue && elements > 0 {
					return 0, ErrRowNotPositional
				}
				if seenValue {
					elements++
				}
				if skipJSONSpace(item, index+1) != len(item) {
					return 0, ErrRowNotPositional
				}
				return elements, nil
			}
			depth--
		case ',':
			if depth == 0 {
				// Each comma closes one element, so a comma with no value
				// before it is a hole rather than a boundary.
				if !seenValue {
					return 0, ErrRowNotPositional
				}
				elements++
				seenValue = false
			}
		case ' ', '\t', '\n', '\r':
		default:
			seenValue = true
		}
		if depth < 0 {
			return 0, ErrRowNotPositional
		}
		index++
	}
	return 0, ErrRowNotPositional
}

// jsonStringEnd returns the index one past the JSON string that starts at
// index, and reports whether that string was closed. A quote that follows a
// backslash belongs to the string, so an escape is stepped over rather than
// read.
func jsonStringEnd(item []byte, index int) (int, bool) {
	for i := index + 1; i < len(item); i++ {
		switch item[i] {
		case '\\':
			i++
		case '"':
			return i + 1, true
		}
	}
	return 0, false
}

// skipJSONSpace returns the index of the first byte at or after index that is
// not insignificant whitespace.
func skipJSONSpace(item []byte, index int) int {
	for index < len(item) {
		switch item[index] {
		case ' ', '\t', '\n', '\r':
			index++
		default:
			return index
		}
	}
	return index
}

// quoteFields encodes each field name as the JSON string a zipped row carries.
// It runs once for the answer rather than once for each row, because the names
// are the same for every row of it.
func quoteFields(fields []string) ([]json.RawMessage, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	names := make([]json.RawMessage, len(fields))
	for i := range fields {
		name, err := json.Marshal(fields[i])
		if err != nil {
			return nil, fmt.Errorf("marshal answer field %q: %w", fields[i], err)
		}
		names[i] = name
	}
	return names, nil
}

// zipRow renders one positional row as the object a buffered consumer reads:
// the answer's field names over the row's values, in the column order the head
// declares. A consumer of the streamed form rebuilds the same object from the
// same two halves, which is what keeps the two renderings of one answer equal
// across the threshold that chooses between them.
//
// The object is built by hand because a Go map sorts its keys, and the column
// order is what the field names exist to carry.
func zipRow(names, row []json.RawMessage) (json.RawMessage, error) {
	if len(row) != len(names) {
		return nil, fmt.Errorf("%w: the row carries %d values and the head declares %d fields", ErrRowArity, len(row), len(names))
	}

	size := len("{}")
	for i := range names {
		size += len(names[i]) + len(row[i]) + len(`:,`)
	}
	object := make([]byte, 0, size)
	object = append(object, '{')
	for i := range names {
		if i > 0 {
			object = append(object, ',')
		}
		object = append(object, names[i]...)
		object = append(object, ':')
		object = append(object, row[i]...)
	}
	return append(object, '}'), nil
}
