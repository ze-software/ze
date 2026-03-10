// Design: docs/architecture/api/commands.md — CLI table rendering
// Overview: pipe.go — pipe operator framework (table is one operator)
//
// pipe_table.go renders JSON data as nushell-style tables with box-drawing
// characters. Supports nested tables (objects/arrays within cells).
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const emptyMarker = "(empty)\n"

// tableCell holds pre-rendered cell content, potentially multi-line for nested tables.
type tableCell struct {
	lines []string
	width int // max display width across all lines
}

// applyTable parses JSON input and renders it as a table.
// Non-JSON input passes through unchanged.
func applyTable(input string) string {
	var data any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &data); err != nil {
		return input
	}
	return renderValue(data)
}

// renderValue dispatches to the appropriate table renderer based on type.
func renderValue(v any) string {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return emptyMarker
		}
		// Single-key wrapper map (e.g., {"peers": ...}): unwrap and render the value.
		// Common JSON API pattern where the top key is just a namespace.
		if len(val) == 1 {
			for _, inner := range val {
				switch inner.(type) {
				case map[string]any, []any:
					return renderValue(inner)
				}
			}
		}
		// Check if this is a map-of-maps with homogeneous keys (e.g., peers indexed by IP).
		// Render as columnar table with the parent key as first column.
		if childKeys := homogeneousMapOfMapsKeys(val); childKeys != nil {
			return renderMapOfMaps(val, childKeys)
		}
		return renderRecord(val)
	case []any:
		if len(val) == 0 {
			return emptyMarker
		}
		if _, ok := val[0].(map[string]any); ok {
			return renderList(val)
		}
		return renderPrimitiveList(val)
	default:
		return fmt.Sprint(formatNumber(v)) + "\n"
	}
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
		keys := sortedKeys(child)
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
func renderMapOfMaps(m map[string]any, childKeys []string) string {
	parentKeys := sortedKeys(m)

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
				row[colIdx+1] = cellFromValue(v)
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
	var b strings.Builder
	b.WriteString(drawBorder(widths, '┌', '┬', '┐'))

	// Header row.
	headerCells := make([]tableCell, len(allCols))
	for i, col := range allCols {
		headerCells[i] = cellFromString(col)
	}
	writeRow(&b, headerCells, widths)
	b.WriteString(drawBorder(widths, '├', '┼', '┤'))

	// Data rows.
	for _, row := range rows {
		writeRow(&b, row, widths)
	}
	b.WriteString(drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderRecord renders a map as a two-column key-value table.
func renderRecord(m map[string]any) string {
	keys := sortedKeys(m)

	keyCells := make([]tableCell, len(keys))
	valCells := make([]tableCell, len(keys))
	keyWidth, valWidth := 0, 0

	for i, k := range keys {
		keyCells[i] = cellFromString(k)
		valCells[i] = cellFromValue(m[k])
		if keyCells[i].width > keyWidth {
			keyWidth = keyCells[i].width
		}
		if valCells[i].width > valWidth {
			valWidth = valCells[i].width
		}
	}

	widths := []int{keyWidth, valWidth}
	var b strings.Builder
	b.WriteString(drawBorder(widths, '┌', '┬', '┐'))
	for i := range keys {
		writeRow(&b, []tableCell{keyCells[i], valCells[i]}, widths)
	}
	b.WriteString(drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderList renders an array of objects as a columnar table with headers.
func renderList(arr []any) string {
	// Collect union of all keys.
	keySet := make(map[string]bool)
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			for k := range m {
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
					row[colIdx] = cellFromValue(v)
				} else {
					row[colIdx] = cellFromString("")
				}
			} else {
				// Non-object in array — put in first column only.
				if colIdx == 0 {
					row[colIdx] = cellFromValue(item)
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
	var b strings.Builder
	b.WriteString(drawBorder(widths, '┌', '┬', '┐'))

	// Header row.
	headerCells := make([]tableCell, len(keys))
	for i, k := range keys {
		headerCells[i] = cellFromString(k)
	}
	writeRow(&b, headerCells, widths)
	b.WriteString(drawBorder(widths, '├', '┼', '┤'))

	// Data rows.
	for _, row := range rows {
		writeRow(&b, row, widths)
	}
	b.WriteString(drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// renderPrimitiveList renders an array of non-object values as a single-column table.
func renderPrimitiveList(arr []any) string {
	cells := make([]tableCell, len(arr))
	width := 0
	for i, item := range arr {
		cells[i] = cellFromValue(item)
		if cells[i].width > width {
			width = cells[i].width
		}
	}

	widths := []int{width}
	var b strings.Builder
	b.WriteString(drawBorder(widths, '┌', '┬', '┐'))
	for _, c := range cells {
		writeRow(&b, []tableCell{c}, widths)
	}
	b.WriteString(drawBorder(widths, '└', '┴', '┘'))
	return b.String()
}

// cellFromValue creates a table cell from any JSON value.
// Objects and arrays render as nested sub-tables.
func cellFromValue(v any) tableCell {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return cellFromString("")
		}
		return cellFromString(strings.TrimRight(renderRecord(val), "\n"))
	case []any:
		if len(val) == 0 {
			return cellFromString("")
		}
		if _, ok := val[0].(map[string]any); ok {
			return cellFromString(strings.TrimRight(renderList(val), "\n"))
		}
		// Inline array of primitives.
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = fmt.Sprint(formatNumber(item))
		}
		return cellFromString("[" + strings.Join(parts, ", ") + "]")
	case bool:
		return cellFromString(fmt.Sprint(val))
	case nil:
		return cellFromString("")
	default:
		return cellFromString(fmt.Sprint(formatNumber(v)))
	}
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
func drawBorder(widths []int, left, mid, right rune) string {
	var b strings.Builder
	b.WriteRune(left)
	for i, w := range widths {
		for range w + 2 { // +2 for padding spaces
			b.WriteRune('─')
		}
		if i < len(widths)-1 {
			b.WriteRune(mid)
		}
	}
	b.WriteRune(right)
	b.WriteRune('\n')
	return b.String()
}

// writeRow writes a potentially multi-line row to the builder.
func writeRow(b *strings.Builder, cells []tableCell, widths []int) {
	// Find max height across all cells.
	height := 1
	for _, c := range cells {
		if len(c.lines) > height {
			height = len(c.lines)
		}
	}

	for lineIdx := range height {
		b.WriteRune('│')
		for colIdx, c := range cells {
			b.WriteByte(' ')
			line := ""
			if lineIdx < len(c.lines) {
				line = c.lines[lineIdx]
			}
			b.WriteString(line)
			// Pad to column width.
			for range widths[colIdx] - displayWidth(line) {
				b.WriteByte(' ')
			}
			b.WriteString(" │")
		}
		b.WriteByte('\n')
	}
}

// sortedKeys returns map keys sorted alphabetically.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
