// Design: docs/architecture/api/commands.md — CLI table rendering
// Overview: pipe.go — pipe operator framework (table is one operator)
//
// pipe_table.go renders JSON data as nushell-style tables with box-drawing
// characters (table mode) or space-aligned columns (text mode).
// Supports nested tables (objects/arrays within cells).
package command

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const emptyMarker = "(empty)\n"
const pipeMetaKey = "pipe"

// tableStyle controls rendering: box-drawing (table) or plain spacing (text).
type tableStyle struct {
	plain   bool          // true = space-aligned columns, no box-drawing
	orders  []ColumnOrder // the command's declared column orders; nil renders alphabetically
	request columnRequest // what `| display` and `| fill` asked for; it replaces orders rather than joining them
}

// tableCell holds pre-rendered cell content, potentially multi-line for nested tables.
type tableCell struct {
	lines []string
	width int // max display width across all lines
}

// ApplyTable parses JSON input and renders it as a box-drawing table with
// alphabetical columns. Non-JSON input passes through unchanged.
func ApplyTable(input string) string {
	return applyTableStyled(input, tableStyle{})
}

// applyTableStyled parses JSON input and renders it with the given style.
// Non-JSON input passes through unchanged.
func applyTableStyled(input string, style tableStyle) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}
	return style.renderValue(data)
}

// renderValue dispatches to the appropriate table renderer based on type.
func (s tableStyle) renderValue(v any) string {
	switch val := v.(type) {
	case nil:
		// A JSON null is an answer that carries nothing, which is what
		// emptyMarker states for an empty map and an empty list already. Without
		// this case it reaches the fmt.Sprint below and an operator reads
		// "<nil>", a Go spelling of a Go zero value, for a command that
		// succeeded and had nothing to report.
		//
		// The value stays null on every machine-facing rendering: | json, | yaml
		// and | raw answer a program, and null is what a program parses. This is
		// the human rendering alone (owner directive, 2026-08-21: returning nil
		// is fine, printing it is not).
		return emptyMarker
	case map[string]any:
		if _, hasPipe := val[pipeMetaKey]; hasPipe && len(val) == 2 {
			for k, inner := range val {
				if k != pipeMetaKey {
					return s.renderValue(inner)
				}
			}
		}
		if len(val) == 0 {
			return emptyMarker
		}
		// Single-key wrapper map (e.g., {"peers": ...}): unwrap and render the value.
		// Common JSON API pattern where the top key is just a namespace.
		if len(val) == 1 {
			for _, inner := range val {
				switch inner.(type) {
				case map[string]any, []any:
					return s.renderValue(inner)
				}
			}
		}
		// Check if this is a map-of-maps with homogeneous keys (e.g., peers indexed by IP).
		// Render as columnar table with the parent key as first column.
		if childKeys := homogeneousMapOfMapsKeys(val); childKeys != nil {
			return s.renderMapOfMaps(val, childKeys)
		}
		return s.renderRecord(val)
	case []any:
		if len(val) == 0 {
			return emptyMarker
		}
		if _, ok := val[0].(map[string]any); ok {
			return s.renderList(val)
		}
		return s.renderPrimitiveList(val)
	}
	var tb textbuf.Buffer
	return tb.Str(fmt.Sprint(formatNumber(v))).Byte('\n').String()
}

// homogeneousMapOfMapsKeys returns the shared child keys if every value in m is a map
// with identical key sets, or nil if the map is not a homogeneous map-of-maps.
func homogeneousMapOfMapsKeys(m map[string]any) []string {
	if len(m) < 2 {
		return nil
	}
	var refKeys []string
	for _, v := range m {
		child, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		keys := tableSortedKeys(child)
		if len(keys) == 0 {
			return nil
		}
		if refKeys == nil {
			refKeys = keys
			continue
		}
		if len(keys) != len(refKeys) {
			return nil
		}
		for i, k := range keys {
			if k != refKeys[i] {
				return nil
			}
		}
	}
	return refKeys
}

// renderMapOfMaps renders a map-of-maps as a columnar table.
// The parent key becomes the first column; child keys become the remaining columns.
func (s tableStyle) renderMapOfMaps(m map[string]any, childKeys []string) string {
	// The parent keys identify rows (a peer address, a family name), so they
	// stay alphabetical. Only the child keys are columns.
	parentKeys := tableSortedKeys(m)
	childKeys = s.orderKeys(childKeys)

	// All columns: parent key header + child key headers.
	allCols := make([]string, 0, 1+len(childKeys))
	allCols = append(allCols, "") // first column has no header (it's the key)
	allCols = append(allCols, childKeys...)

	// Initialize widths from header names.
	widths := make([]int, len(allCols))
	for i, col := range allCols {
		widths[i] = displayWidth(col)
	}

	// Build rows.
	rows := make([][]tableCell, len(parentKeys))
	for rowIdx, parentKey := range parentKeys {
		row := make([]tableCell, len(allCols))
		row[0] = cellFromString(parentKey)
		if row[0].width > widths[0] {
			widths[0] = row[0].width
		}
		child, _ := m[parentKey].(map[string]any)
		for colIdx, childKey := range childKeys {
			if v, ok := child[childKey]; ok {
				row[colIdx+1] = s.cellFromValue(v)
			} else {
				row[colIdx+1] = cellFromString("")
			}
			if row[colIdx+1].width > widths[colIdx+1] {
				widths[colIdx+1] = row[colIdx+1].width
			}
		}
		rows[rowIdx] = row
	}

	// Render.
	var b textbuf.Buffer
	b.WriteString(s.drawBorder(widths, '┌', '┬', '┐'))

	// Header row.
	headerCells := make([]tableCell, len(allCols))
	for i, col := range allCols {
		headerCells[i] = cellFromString(col)
	}
	s.writeRow(&b, headerCells, widths)
	b.WriteString(s.drawBorder(widths, '├', '┼', '┤'))

	// Data rows.
	for _, row := range rows {
		s.writeRow(&b, row, widths)
	}
	b.WriteString(s.drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderRecord renders a map as a two-column key-value table.
func (s tableStyle) renderRecord(m map[string]any) string {
	keys := s.orderKeys(tableSortedKeys(m))

	keyCells := make([]tableCell, len(keys))
	valCells := make([]tableCell, len(keys))
	keyWidth, valWidth := 0, 0

	for i, k := range keys {
		keyCells[i] = cellFromString(k)
		valCells[i] = s.cellFromValue(m[k])
		if keyCells[i].width > keyWidth {
			keyWidth = keyCells[i].width
		}
		if valCells[i].width > valWidth {
			valWidth = valCells[i].width
		}
	}

	widths := []int{keyWidth, valWidth}
	var b textbuf.Buffer
	b.WriteString(s.drawBorder(widths, '┌', '┬', '┐'))
	for i := range keys {
		s.writeRow(&b, []tableCell{keyCells[i], valCells[i]}, widths)
	}
	b.WriteString(s.drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderList renders an array of objects as a columnar table with headers.
func (s tableStyle) renderList(arr []any) string {
	// Collect union of all keys.
	keySet := make(map[string]bool)
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			for k := range m {
				if k == pipeMetaKey {
					continue
				}
				keySet[k] = true
			}
		}
	}
	if len(keySet) == 0 {
		return emptyMarker
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	keys = s.orderKeys(keys)

	// Initialize widths from header names.
	widths := make([]int, len(keys))
	for i, k := range keys {
		widths[i] = displayWidth(k)
	}

	// Build data rows and update widths.
	rows := make([][]tableCell, len(arr))
	for rowIdx, item := range arr {
		row := make([]tableCell, len(keys))
		m, ok := item.(map[string]any)
		for colIdx, k := range keys {
			if ok {
				if v, exists := m[k]; exists {
					row[colIdx] = s.cellFromValue(v)
				} else {
					row[colIdx] = cellFromString("")
				}
			} else {
				// Non-object in array — put in first column only.
				if colIdx == 0 {
					row[colIdx] = s.cellFromValue(item)
				} else {
					row[colIdx] = cellFromString("")
				}
			}
			if row[colIdx].width > widths[colIdx] {
				widths[colIdx] = row[colIdx].width
			}
		}
		rows[rowIdx] = row
	}

	// Render.
	var b textbuf.Buffer
	b.WriteString(s.drawBorder(widths, '┌', '┬', '┐'))

	// Header row.
	headerCells := make([]tableCell, len(keys))
	for i, k := range keys {
		headerCells[i] = cellFromString(k)
	}
	s.writeRow(&b, headerCells, widths)
	b.WriteString(s.drawBorder(widths, '├', '┼', '┤'))

	// Data rows.
	for _, row := range rows {
		s.writeRow(&b, row, widths)
	}
	b.WriteString(s.drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderPrimitiveList renders an array of non-object values as a single-column table.
func (s tableStyle) renderPrimitiveList(arr []any) string {
	cells := make([]tableCell, len(arr))
	width := 0
	for i, item := range arr {
		cells[i] = s.cellFromValue(item)
		if cells[i].width > width {
			width = cells[i].width
		}
	}

	widths := []int{width}
	var b textbuf.Buffer
	b.WriteString(s.drawBorder(widths, '┌', '┬', '┐'))
	for _, c := range cells {
		s.writeRow(&b, []tableCell{c}, widths)
	}
	b.WriteString(s.drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// cellFromValue creates a table cell from any JSON value.
// Objects and arrays render as nested sub-tables.
func (s tableStyle) cellFromValue(v any) tableCell {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return cellFromString("")
		}
		return cellFromString(strings.TrimRight(s.renderRecord(val), "\n"))
	case []any:
		if len(val) == 0 {
			return cellFromString("")
		}
		if _, ok := val[0].(map[string]any); ok {
			return cellFromString(strings.TrimRight(s.renderList(val), "\n"))
		}
		// Inline array of primitives.
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = fmt.Sprint(formatNumber(item))
		}
		var tb textbuf.Buffer
		return cellFromString(tb.Byte('[').Join(parts, ", ").Byte(']').String())
	case bool:
		return cellFromString(fmt.Sprint(val))
	case nil:
		return cellFromString("")
	}
	return cellFromString(fmt.Sprint(formatNumber(v)))
}

// cellFromString wraps a string (possibly multi-line) into a tableCell.
func cellFromString(s string) tableCell {
	if s == "" {
		return tableCell{lines: []string{""}, width: 0}
	}
	lines := strings.Split(s, "\n")
	width := 0
	for _, line := range lines {
		if w := displayWidth(line); w > width {
			width = w
		}
	}
	return tableCell{lines: lines, width: width}
}

// displayWidth returns the terminal display width (rune count).
func displayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// drawBorder creates a horizontal border line using box-drawing characters.
// In plain mode, returns empty string (no borders).
func (s tableStyle) drawBorder(widths []int, left, mid, right rune) string {
	if s.plain {
		return ""
	}
	var b textbuf.Buffer
	b.WriteRune(left)
	for i, w := range widths {
		b.Repeat("─", w+2)
		if i < len(widths)-1 {
			b.WriteRune(mid)
		}
	}
	b.WriteRune(right)
	b.Byte('\n')
	return b.String()
}

// writeRow writes a potentially multi-line row to the builder.
// In box mode: │-delimited cells with padding. In plain mode: space-aligned columns.
func (s tableStyle) writeRow(b *textbuf.Buffer, cells []tableCell, widths []int) {
	// Find max height across all cells.
	height := 1
	for _, c := range cells {
		if len(c.lines) > height {
			height = len(c.lines)
		}
	}

	for lineIdx := range height {
		if !s.plain {
			b.WriteRune('│')
		}
		for colIdx, c := range cells {
			if s.plain && colIdx > 0 {
				b.WriteString("  ")
			}
			if !s.plain {
				b.WriteByte(' ')
			}
			line := ""
			if lineIdx < len(c.lines) {
				line = c.lines[lineIdx]
			}
			b.WriteString(line)
			// Pad to column width. Skip trailing padding on last column in plain mode.
			if !s.plain || colIdx < len(cells)-1 {
				for range widths[colIdx] - displayWidth(line) {
					b.WriteByte(' ')
				}
			}
			if !s.plain {
				b.WriteString(" │")
			}
		}
		b.WriteByte('\n')
	}
}

// orderKeys returns keys in the sequence the answer renders them, and drops
// the ones the operator chose not to see.
//
// Four cases, and the operators are additive. `| display` names the fields the
// answer leads with. `| fill` says whether the fields it did not name appear at
// all and in what sequence.
//
//   - Neither operator: the command's own declaration first, then the rest.
//   - `| fill` alone: every field, in the way it named. Nothing was displayed,
//     so every field is a remaining field.
//   - `| display` alone: the named fields alone, in the order they were named.
//   - Both: the named fields, then the rest in the way fill named.
//
// The two sequences are independent, and they cover disjoint key sets. The
// names `| display` gave order the fields it named. The command's own
// declaration still orders the rest whenever `| fill` names no way of its own.
//
// A BUILT-IN order never hides a field: a reader who did not ask for the order
// reads an absent column as an absent value. An operator who typed `| display`
// DID ask, so a key this method drops is one they chose to drop.
//
// An operator's request replaces the declared orders rather than joining them.
// It names the few columns the question is about. Ranking it against a
// nineteen-name declaration by how many keys each one hits would lose it every
// time.
//
// No caller measures a column to answer this. `| fill overall` ordered by
// rendered width and was removed on 2026-08-19, so every way here is decided
// from the key set alone and this method reads no cell.
func (s tableStyle) orderKeys(keys []string) []string {
	request := s.request
	if len(request.display) == 0 && request.fill == fillNone {
		return s.declaredKeys(keys)
	}

	displayed, rest := splitByOrder(keys, request.display)
	if request.fill == fillNone {
		// A record that shares none of the displayed fields is not the one
		// whose columns the operator was choosing. Emptying it would answer a
		// nested sub-table with a box and no rows.
		if len(displayed) > 0 {
			return displayed
		}
		return keys
	}
	return append(displayed, s.fillKeys(keys, rest)...)
}

// declaredKeys returns keys with the columns the command declared first, in the
// declared order. Every other key comes after them in the order given, which
// every caller has already sorted alphabetically. Nothing is dropped.
//
// A command declares one order per record shape, so the order applied here is
// the one that names the most of these keys. That is what tells the peer rows
// of `show bgp` from the record that carries them. Both hold "uptime",
// and only the key set says which record is being rendered.
func (s tableStyle) declaredKeys(keys []string) []string {
	order := bestColumnOrder(s.orders, keys)
	if len(order) == 0 {
		return keys
	}
	declared, rest := splitByOrder(keys, order)
	return append(declared, rest...)
}

// splitByOrder divides keys into the ones order names, sorted into its
// sequence, and the ones it does not, left in the order they arrived.
func splitByOrder(keys []string, order ColumnOrder) (named, rest []string) {
	if len(order) == 0 {
		return nil, keys
	}

	rank := make(map[string]int, len(order))
	for i, name := range order {
		if _, seen := rank[name]; !seen {
			rank[name] = i
		}
	}

	named = make([]string, 0, len(keys))
	rest = make([]string, 0, len(keys))
	for _, k := range keys {
		if _, ok := rank[k]; ok {
			named = append(named, k)
			continue
		}
		rest = append(rest, k)
	}
	sort.SliceStable(named, func(i, j int) bool { return rank[named[i]] < rank[named[j]] })
	return named, rest
}

// fillKeys returns the remaining keys in the sequence `| fill` asked for.
//
// keys is the whole key set of the record, which is what says WHICH record
// shape is being rendered. rest is the part `| display` did not name, which is
// what gets ordered.
func (s tableStyle) fillKeys(keys, rest []string) []string {
	filled := make([]string, len(rest))
	copy(filled, rest)

	switch s.request.fill {
	case fillAlpha:
		sort.Strings(filled)
	case fillDefault:
		// The command's own declaration orders what the operator did not name.
		// The shape is read from the whole key set, because that is what tells
		// one record shape of a command from another. The declaration is then
		// applied to the part that is left. A command that declared none leaves
		// the remainder as it arrived, which is by name.
		declared, undeclared := splitByOrder(filled, bestColumnOrder(s.orders, keys))
		filled = slices.Concat(declared, undeclared)
	case fillNone:
		// Unreachable: orderKeys answers before it calls this.
	}

	if s.request.reverse {
		slices.Reverse(filled)
	}
	return filled
}

// bestColumnOrder returns the declared order that names the most of keys, and
// nil when no order names any of them. A tie goes to the order declared first.
func bestColumnOrder(orders []ColumnOrder, keys []string) ColumnOrder {
	if len(orders) == 0 {
		return nil
	}

	present := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		present[k] = struct{}{}
	}

	var best ColumnOrder
	bestHits := 0
	for _, order := range orders {
		hits := 0
		for _, name := range order {
			if _, ok := present[name]; ok {
				hits++
			}
		}
		if hits > bestHits {
			best = order
			bestHits = hits
		}
	}
	return best
}

// tableSortedKeys returns map keys sorted alphabetically,
// excluding the "pipe" metadata key which is infrastructure, not user data.
func tableSortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		if k == pipeMetaKey {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
