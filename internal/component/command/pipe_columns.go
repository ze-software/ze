// Design: docs/architecture/api/commands.md — the `| display` and `| fill` pipe operators
// Overview: pipe.go — the pipe operator framework that parses and runs them
// Related: pipe_table.go — the renderer that applies the sequence half
// Related: column_order.go — the built-in order these operators replace
//
// pipe_columns.go implements the two operators an operator uses to choose the
// columns of an answer.
//
//	| display <field> [<field> ...]   field names only
//	| fill [alpha] [reverse]          keywords only, never a field name
//
// Each one takes ONE type of argument, so no token is a field name in one
// position and a keyword in another.
//
// They are additive and neither changes what the other means. `| display`
// names the fields the answer leads with, and `| fill` says whether the fields
// it did not name appear at all and in what sequence. With no `| display`
// every field is a remaining field, and with no `| fill` nothing is filled in.
//
// WHICH fields the answer carries is a data question, so it is applied here, to
// the payload, and every format that follows sees the result. In WHAT SEQUENCE
// they render is a presentation question, so it is carried on tableStyle and
// applied by the table and text renderers alone.

package command

import (
	"encoding/json"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The words `| fill` accepts. Each one is a keyword, never a field name.
const (
	fillWayAlpha    = "alpha"
	fillWordReverse = "reverse"
)

// fillWay says how the fields `| display` did not name are ordered.
//
// The zero value means the chain carries no `| fill`, so those fields are not
// filled in at all. reverse is carried beside the way rather than doubling the
// constants, because it flips whichever way is in force.
//
// No way orders by rendered column width. There was one, `overall`, and it was
// removed on 2026-08-19: measuring a column means rendering every cell of the
// whole answer, so the first row could not be written until the last row had
// been read. Every way here reads a declaration or a field name, so each one
// decides the sequence from the key set alone.
type fillWay uint8

const (
	fillNone    fillWay = iota // no `| fill`: the answer carries the displayed fields alone
	fillDefault                // `| fill`: the command's own declared order, and by name when it declared none
	fillAlpha                  // `alpha`: by field name, whatever the command declared
)

// columnRequest is what the pipe chain asked for about columns.
//
// display is empty when no `| display` was typed, and every field is then a
// remaining field. fill is fillNone when no `| fill` was typed, and the
// remaining fields are then absent.
type columnRequest struct {
	display ColumnOrder
	fill    fillWay
	reverse bool
}

// selects reports whether the request hides the fields it did not name. Naming
// fields and asking for none of the rest is the only combination that does.
func (r columnRequest) selects() bool {
	return len(r.display) > 0 && r.fill == fillNone
}

// parseDisplay reads a `| display` argument as the field names it lists.
//
// A name is lowercased because a JSON key is lowercase kebab-case
// (ai/rules/cli.md), and an operator types what they read.
func parseDisplay(arg string) ColumnOrder {
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return nil
	}
	display := make(ColumnOrder, 0, len(fields))
	for _, field := range fields {
		display = append(display, strings.ToLower(field))
	}
	return display
}

// parseFill reads a `| fill` argument as the way it names and whether it flips
// that way. ok is false for an argument this function cannot read, and
// fillError turns that into a message.
//
// A `| fill` that names no way is not an error and is not a synonym for
// `alpha`: it asks for the command's own declared order.
func parseFill(arg string) (way fillWay, reverse, ok bool) {
	fields := strings.Fields(arg)
	if last := len(fields) - 1; last >= 0 && fields[last] == fillWordReverse {
		reverse = true
		fields = fields[:last]
	}
	if len(fields) == 0 {
		return fillDefault, reverse, true
	}
	if len(fields) == 1 && fields[0] == fillWayAlpha {
		return fillAlpha, reverse, true
	}
	return fillNone, reverse, false
}

// columnsInChain returns what the chain asked for about columns, reading both
// operators. A chain names each one at most once in practice. A second one
// wins, which is what a reader of left-to-right pipes expects of a modifier
// that replaces rather than accumulates.
func columnsInChain(ops []pipeOp) columnRequest {
	var request columnRequest
	for _, op := range ops {
		switch op.kind { //nolint:exhaustive // only the two column operators carry a request
		case pipeDisplay:
			request.display = parseDisplay(op.arg)
		case pipeFill:
			request.fill, request.reverse, _ = parseFill(op.arg)
		}
	}
	return request
}

// displayError returns the message a `| display` argument earns, and an empty
// string when the argument is good.
func displayError(arg string) string {
	if len(parseDisplay(arg)) == 0 {
		return "display requires at least one field name"
	}
	return ""
}

// fillError returns the message a `| fill` argument earns, and an empty string
// when the argument is good. A `| fill` with no argument is good.
func fillError(arg string) string {
	if _, _, ok := parseFill(arg); ok {
		return ""
	}
	var tb textbuf.Buffer
	return tb.Str("fill does not recognize ").Str(strings.TrimSpace(arg)).
		Str(" (use ").Str(fillWayAlpha).Str(", and ").Str(fillWordReverse).
		Str(" to flip the order)").String()
}

// applyDisplaySelect drops every field `| display` did not name. A `| fill`
// anywhere in the chain asks for those fields back, so the payload then passes
// through and the operators carry sequence alone. Non-JSON input passes through
// as well, as it does for every other data transform in the chain.
//
// Numbers are decoded as json.Number so a re-marshaled payload carries the
// digits the dispatcher wrote, rather than what a float64 round trip leaves.
func applyDisplaySelect(input string, request columnRequest) string {
	if !request.selects() {
		return input
	}

	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(input)))
	decoder.UseNumber()
	var data any
	if err := decoder.Decode(&data); err != nil {
		return input
	}

	keep := make(map[string]struct{}, len(request.display))
	for _, name := range request.display {
		keep[name] = struct{}{}
	}
	out, err := json.Marshal(selectFields(data, keep))
	if err != nil {
		return input
	}
	return string(out)
}

// selectFields keeps the named fields of every record in v.
//
// It walks the shapes tableStyle.renderValue walks, so the fields a table keeps
// and the fields JSON keeps are the same fields. Recursion is over a payload
// this daemon built, and its depth is the nesting of that payload.
func selectFields(v any, keep map[string]struct{}) any {
	switch val := v.(type) {
	case []any:
		// An array element is a row, never a wrapper: renderList reads it the
		// same way.
		out := make([]any, len(val))
		for i, item := range val {
			record, ok := item.(map[string]any)
			if !ok {
				out[i] = item
				continue
			}
			out[i] = selectRecord(record, keep)
		}
		return out
	case map[string]any:
		return selectMap(val, keep)
	}
	return v
}

// selectMap keeps the named fields of a map, whatever the map turns out to be:
//
//   - a pipe-metadata wrapper
//   - a single-key namespace wrapper
//   - peers indexed by address
//   - a record
//
// Only the last one holds fields.
func selectMap(m map[string]any, keep map[string]struct{}) any {
	if len(m) == 0 {
		return m
	}

	if _, hasMeta := m[pipeMetaKey]; hasMeta && len(m) == 2 {
		out := make(map[string]any, len(m))
		for key, inner := range m {
			if key == pipeMetaKey {
				out[key] = inner
				continue
			}
			out[key] = selectFields(inner, keep)
		}
		return out
	}

	if len(m) == 1 {
		for key, inner := range m {
			switch inner.(type) {
			case map[string]any, []any:
				return map[string]any{key: selectFields(inner, keep)}
			}
		}
	}

	// Peers indexed by address: the parent key identifies the row and is not a
	// field, so it survives selection and keeps the row readable.
	if homogeneousMapOfMapsKeys(m) != nil {
		out := make(map[string]any, len(m))
		for key, inner := range m {
			child, ok := inner.(map[string]any)
			if !ok {
				out[key] = inner
				continue
			}
			out[key] = selectRecord(child, keep)
		}
		return out
	}

	return selectRecord(m, keep)
}

// selectRecord returns the fields of one record that the operator named, and
// the record whole when it names none of them. A record that shares no named
// key is not the one whose columns the operator was choosing. Emptying it would
// answer a nested sub-table with a box and no rows.
//
// That is the rule tableStyle.orderKeys applies to the same record, so a
// nested table and the JSON behind it carry the same fields.
//
// The pipe metadata is infrastructure rather than a field, so it stays.
func selectRecord(m map[string]any, keep map[string]struct{}) map[string]any {
	named := false
	for key := range m {
		if key == pipeMetaKey {
			continue
		}
		if _, ok := keep[strings.ToLower(key)]; ok {
			named = true
			break
		}
	}

	out := make(map[string]any, len(m))
	for key, value := range m {
		if key == pipeMetaKey {
			out[key] = value
			continue
		}
		_, wanted := keep[strings.ToLower(key)]
		if named && !wanted {
			continue
		}
		out[key] = selectFields(value, keep)
	}
	return out
}
