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
	"errors"
	"io"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
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
		// Only display and fill contribute to the column request.
		//exhaustive:ignore
		switch op.kind {
		case pipeDisplay:
			request.display = narrowDisplay(request.display, parseDisplay(op.arg))
		case pipeFill:
			request.fill, request.reverse, _ = parseFill(op.arg)
		}
	}
	return request
}

// narrowDisplay composes two `| display` requests: the second NARROWS the
// first, keeping the fields both name, in the order the second asked for.
//
// It used to assign, so the last request simply replaced the first, and
// `display state | display address` WIDENED — it answered address, a field the
// first request had already dropped. Narrowing is what a chain means everywhere
// else: `match X | match Y` keeps the rows matching both.
//
// An empty intersection is refused by the validator rather than answered here,
// so the operator hears why instead of receiving an answer with no fields.
func narrowDisplay(current, next ColumnOrder) ColumnOrder {
	if len(current) == 0 {
		return next
	}
	if len(next) == 0 {
		return current
	}
	have := make(map[string]bool, len(current))
	for _, field := range current {
		have[field] = true
	}
	narrowed := make(ColumnOrder, 0, len(next))
	for _, field := range next {
		if have[field] {
			narrowed = append(narrowed, field)
		}
	}
	return narrowed
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

// applyDisplaySelect drops every top-level row field `| display` did not name.
// A `| fill` anywhere in the chain asks for those fields back, so the payload
// then passes through and the operators carry sequence alone. Non-JSON input
// passes through as it does for every other data transform in the chain.
//
// A sequence of records is selected ONE RECORD AT A TIME. selectSequence
// decodes an element, selects its fields, writes it out and forgets it, so an
// answer of a million rows never becomes a million decoded rows in memory.
//
// A named table explicitly selects its containing record. Otherwise the record
// holding the most requested fields is selected; a tie is refused rather than
// silently dropping one matching record because it appears at another depth.
func applyDisplaySelect(input string, request columnRequest) (string, string) {
	if !request.selects() {
		return input, ""
	}

	keep := keepFields(request.display)
	payload := strings.TrimSpace(input)
	selectedSequence, matched, empty, sequence, refusal := selectSequence(payload, keep)
	if refusal != "" {
		return "", refusal
	}
	if sequence {
		if displaySelectionMiss(matched, empty) {
			return "", displayNoFieldError(request.display)
		}
		return selectedSequence, ""
	}

	var data any
	if err := decodePipeJSON(strings.NewReader(payload), &data); err != nil {
		return input, ""
	}
	selected, matches, refusal := selectFields(data, keep)
	if refusal != "" {
		return "", refusal
	}
	rows, _, hasRows := rowsIn(data)
	if displayDecodedSelectionMiss(matches > 0, hasRows, len(rows)) {
		return "", displayNoFieldError(request.display)
	}
	out, err := json.Marshal(selected)
	if err != nil {
		return input, ""
	}
	return string(out), ""
}

func displaySelectionMiss(matched, empty bool) bool {
	if matched {
		return false
	}
	return !empty
}

func displayDecodedSelectionMiss(matched, hasRows bool, rows int) bool {
	if matched {
		return false
	}
	if !hasRows {
		return true
	}
	return rows > 0
}

func displayNoFieldError(display ColumnOrder) string {
	var tb textbuf.Buffer
	return tb.Str("display selects no field: none of ").
		Str(textbuf.Join(display, ", ")).
		Str(" matches a top-level row field; check the field name").String()
}

// keepFields is the set of field names the operator displayed, in the spelling
// selectRecord compares against.
func keepFields(display ColumnOrder) map[string]struct{} {
	keep := make(map[string]struct{}, len(display))
	for _, name := range display {
		keep[name] = struct{}{}
	}
	return keep
}

// selectSequence writes the selection of a sequence of records, decoding one
// record at a time. ok is false for a payload that is not a sequence, and the
// caller then decodes that payload whole; whatever was written before the shape
// was ruled out is discarded with it.
//
// A second key, a value that is not an array, and a token after the value each
// read as "not a sequence", so a shape this function does not recognize reaches
// selectFields unchanged rather than being answered wrongly.
func selectSequence(payload string, keep map[string]struct{}) (selected string, matched, empty, ok bool, refusal string) {
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()

	open, err := decoder.Token()
	if err != nil {
		return "", false, false, false, ""
	}
	delim, isDelim := open.(json.Delim)
	if !isDelim {
		return "", false, false, false, ""
	}

	var b textbuf.Buffer
	var rows int
	switch delim {
	case '[':
		var arrayOK bool
		arrayOK, matched, rows, refusal = selectArray(decoder, keep, &b)
		if refusal != "" {
			return "", false, false, true, refusal
		}
		if !arrayOK {
			return "", false, false, false, ""
		}
	case '{':
		var envelopeOK bool
		envelopeOK, matched, rows = selectEnvelope(decoder, keep, &b)
		if !envelopeOK {
			return "", false, false, false, ""
		}
	default:
		return "", false, false, false, ""
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return "", false, false, false, ""
	}
	return b.String(), matched, rows == 0, true, ""
}

// selectEnvelope writes `{"<key>":[...]}`, the shape an answer takes under the
// envelope key its head named. The opening brace is already read.
func selectEnvelope(decoder *json.Decoder, keep map[string]struct{}, b *textbuf.Buffer) (bool, bool, int) {
	if !decoder.More() {
		return false, false, 0
	}
	name, err := decoder.Token()
	if err != nil {
		return false, false, 0
	}
	key, isString := name.(string)
	if !isString {
		return false, false, 0
	}
	open, err := decoder.Token()
	if err != nil {
		return false, false, 0
	}
	delim, isDelim := open.(json.Delim)
	if !isDelim || delim != '[' {
		return false, false, 0
	}

	encoded, err := json.Marshal(key)
	if err != nil {
		return false, false, 0
	}
	b.Byte('{')
	b.Write(encoded) //nolint:errcheck // textbuf.Write never fails
	b.Byte(':')
	arrayOK, matched, rows, refusal := selectArray(decoder, keep, b)
	if refusal != "" {
		// The unread remainder may hold a second key, so this has not been
		// established as a one-key envelope. Judge the complete decoded
		// record before deciding whether its nested ambiguity is relevant.
		return false, false, 0
	}
	if !arrayOK {
		return false, false, 0
	}
	if decoder.More() {
		// A second key. selectMap decides what such a map is by reading every
		// entry, so the caller reads it whole.
		return false, false, 0
	}
	if _, err := decoder.Token(); err != nil { // the closing brace
		return false, false, 0
	}
	b.Byte('}')
	return true, matched, rows
}

// selectArray writes the selection of each element of an array whose opening
// bracket is already read. One element is decoded, selected, written and
// forgotten, so the memory cost is one record rather than the whole answer.
func selectArray(decoder *json.Decoder, keep map[string]struct{}, b *textbuf.Buffer) (bool, bool, int, string) {
	b.Byte('[')
	written := 0
	matched := false
	for decoder.More() {
		var element any
		if err := decoder.Decode(&element); err != nil {
			return false, false, 0, ""
		}
		selected, matches, refusal := selectElement(element, keep)
		if refusal != "" {
			return false, false, 0, refusal
		}
		matched = matched || matches > 0
		encoded, err := json.Marshal(selected)
		if err != nil {
			return false, false, 0, ""
		}
		if written > 0 {
			b.Byte(',')
		}
		b.Write(encoded) //nolint:errcheck // textbuf.Write never fails
		written++
	}
	if _, err := decoder.Token(); err != nil { // the closing bracket
		return false, false, 0, ""
	}
	b.Byte(']')
	return true, matched, written, ""
}

// selectElement keeps the named fields of ONE record of a sequence.
//
// An array element is a row, never a wrapper: renderList reads it the same way,
// and so does the record path, where one rpc.Record carries one element.
func selectElement(v any, keep map[string]struct{}) (any, int, string) {
	record, isRecord := v.(map[string]any)
	if !isRecord {
		return nil, 0, ""
	}
	return selectRecord(record, keep)
}

// selectFields keeps the named fields of every record in v.
//
// It walks the shapes tableStyle.renderValue walks, so the fields a table keeps
// and the fields JSON keeps are the same fields. The match count identifies a
// record shape by the requested fields it holds, rather than by its depth.
// Recursion visits only descendants in the finite decoded input tree.
func selectFields(v any, keep map[string]struct{}) (any, int, string) {
	switch val := v.(type) {
	case []any:
		out := make([]any, len(val))
		matches := 0
		for i, item := range val {
			selected, itemMatches, refusal := selectElement(item, keep)
			if refusal != "" {
				return nil, itemMatches, refusal
			}
			out[i] = selected
			matches = max(matches, itemMatches)
		}
		return out, matches, ""
	case map[string]any:
		return selectMap(val, keep)
	}
	return nil, 0, ""
}

// selectMap keeps the named fields of a map, whatever the map turns out to be:
//
//   - a pipe-metadata wrapper
//   - a single-key namespace wrapper
//   - peers indexed by address
//   - a record
//
// Only the last one holds fields.
func selectMap(m map[string]any, keep map[string]struct{}) (any, int, string) {
	if len(m) == 0 {
		return m, 0, ""
	}

	if _, hasMeta := m[pipeMetaKey]; hasMeta && len(m) == 2 {
		out := make(map[string]any, len(m))
		matches := 0
		for key, inner := range m {
			if key == pipeMetaKey {
				out[key] = inner
				continue
			}
			selected, innerMatches, refusal := selectFields(inner, keep)
			if refusal != "" {
				return nil, innerMatches, refusal
			}
			out[key] = selected
			matches = innerMatches
		}
		return out, matches, ""
	}

	if len(m) == 1 {
		for key, inner := range m {
			switch inner.(type) {
			case map[string]any, []any:
				selected, matches, refusal := selectFields(inner, keep)
				if refusal != "" {
					return nil, matches, refusal
				}
				return map[string]any{key: selected}, matches, ""
			}
		}
	}

	// Peers indexed by address: the parent key identifies the row and is not a
	// field, so it survives selection and keeps the row readable.
	if homogeneousMapOfMapsKeys(m) != nil {
		out := make(map[string]any, len(m))
		matches := 0
		for key, inner := range m {
			child, ok := inner.(map[string]any)
			if !ok {
				continue
			}
			selected, childMatches, refusal := selectRecord(child, keep)
			if refusal != "" {
				return nil, childMatches, refusal
			}
			out[key] = selected
			matches = max(matches, childMatches)
		}
		return out, matches, ""
	}

	return selectRecord(m, keep)
}

// selectRecord selects the record holding the most requested fields. A tied
// match refuses rather than choosing a depth and silently dropping other rows.
//
// Naming a container explicitly chooses this record. Its nested value is
// narrowed when it carries another requested field, and otherwise kept whole.
// This preserves explicit aggregate and row selections such as the aliases.
func selectRecord(m map[string]any, keep map[string]struct{}) (map[string]any, int, string) {
	out := make(map[string]any, len(keep)+1)
	matches := 0
	explicit := false
	for key, value := range m {
		if key == pipeMetaKey {
			out[key] = value
			continue
		}
		if _, wanted := keep[strings.ToLower(key)]; !wanted {
			continue
		}
		matches++
		switch value.(type) {
		case map[string]any, []any:
			explicit = true
		}
		selected, nestedMatches, refusal := selectFields(value, keep)
		if refusal != "" {
			return nil, matches, refusal
		}
		if nestedMatches > 0 {
			out[key] = selected
		} else {
			out[key] = value
		}
	}
	if explicit {
		return out, matches, ""
	}

	candidates := 0
	if matches > 0 {
		candidates = 1
	}
	refusal := ""
	for key, value := range m {
		if key == pipeMetaKey {
			continue
		}
		if _, wanted := keep[strings.ToLower(key)]; wanted {
			continue
		}
		selected, nestedMatches, nestedRefusal := selectFields(value, keep)
		if nestedMatches > matches {
			out = map[string]any{key: selected}
			if metadata, present := m[pipeMetaKey]; present {
				out[pipeMetaKey] = metadata
			}
			matches = nestedMatches
			candidates = 1
			refusal = nestedRefusal
			continue
		}
		if nestedMatches == 0 {
			continue
		}
		if nestedMatches == matches {
			candidates++
		}
	}
	if candidates > 1 {
		return nil, matches, displayAmbiguousError(keep)
	}
	if refusal != "" {
		return nil, matches, refusal
	}
	return out, matches, ""
}

// displayAmbiguousError names only requested keys, never values from either
// record. An explicit containing field resolves which record the reader wants.
func displayAmbiguousError(keep map[string]struct{}) string {
	fields := make([]string, 0, len(keep))
	for field := range keep {
		fields = append(fields, field)
	}
	slices.Sort(fields)
	var problem, fix textbuf.Buffer
	problem.Str("display cannot choose one record for fields ").Str(textbuf.Join(fields, ", "))
	fix.Str("display <record> | display ").Str(textbuf.Join(fields, " "))
	return opererr.New(problem.String(),
		"These fields occur in more than one record; name the containing record before selecting its fields.",
		fix.String()).Error()
}
