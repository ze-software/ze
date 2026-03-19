package command

import (
	"strings"
	"testing"
)

// VALIDATES: key-value record renders as two-column table.
// PREVENTS: unformatted JSON dumped for single objects.
func TestApplyTableRecord(t *testing.T) {
	input := `{"state":"established","address":"1.2.3.4"}`
	result := ApplyTable(input)

	// Keys sorted alphabetically: address, state.
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 4 { // top border + 2 rows + bottom border
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), result)
	}
	if !strings.Contains(lines[0], "┌") || !strings.Contains(lines[0], "┐") {
		t.Errorf("missing top border: %q", lines[0])
	}
	if !strings.Contains(lines[1], "address") || !strings.Contains(lines[1], "1.2.3.4") {
		t.Errorf("missing address row: %q", lines[1])
	}
	if !strings.Contains(lines[2], "state") || !strings.Contains(lines[2], "established") {
		t.Errorf("missing state row: %q", lines[2])
	}
	if !strings.Contains(lines[3], "└") || !strings.Contains(lines[3], "┘") {
		t.Errorf("missing bottom border: %q", lines[3])
	}
}

// VALIDATES: array of objects renders as columnar table with header.
// PREVENTS: array data rendered without column headers.
func TestApplyTableArray(t *testing.T) {
	input := `[{"name":"a","value":1},{"name":"b","value":2}]`
	result := ApplyTable(input)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// top border + header + separator + 2 data rows + bottom border = 6
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d:\n%s", len(lines), result)
	}

	// Header row has column names.
	if !strings.Contains(lines[1], "name") || !strings.Contains(lines[1], "value") {
		t.Errorf("missing header columns: %q", lines[1])
	}

	// Separator between header and data.
	if !strings.Contains(lines[2], "├") || !strings.Contains(lines[2], "┤") {
		t.Errorf("missing header separator: %q", lines[2])
	}

	// Data rows.
	if !strings.Contains(lines[3], "a") {
		t.Errorf("missing first data row: %q", lines[3])
	}
	if !strings.Contains(lines[4], "b") {
		t.Errorf("missing second data row: %q", lines[4])
	}
}

// VALIDATES: nested object renders as sub-table within cell.
// PREVENTS: nested data shown as flat string.
func TestApplyTableNested(t *testing.T) {
	input := `{"peer":"1.2.3.4","caps":{"asn4":true}}`
	result := ApplyTable(input)

	// Should contain nested table markers.
	count := strings.Count(result, "┌")
	if count < 2 {
		t.Errorf("expected at least 2 nested ┌ markers (outer + inner), got %d:\n%s", count, result)
	}

	if !strings.Contains(result, "asn4") {
		t.Error("nested key 'asn4' not found in output")
	}
	if !strings.Contains(result, "true") {
		t.Error("nested value 'true' not found in output")
	}
	if !strings.Contains(result, "peer") || !strings.Contains(result, "1.2.3.4") {
		t.Error("top-level 'peer' key/value not found")
	}
}

// VALIDATES: non-JSON input passes through unchanged.
// PREVENTS: crash on non-JSON input to table formatter.
func TestApplyTableNonJSON(t *testing.T) {
	input := "this is not json"
	result := ApplyTable(input)
	if result != input {
		t.Errorf("non-JSON should pass through, got %q", result)
	}
}

// VALIDATES: empty array shows empty marker.
// PREVENTS: blank output for empty results.
func TestApplyTableEmptyArray(t *testing.T) {
	result := ApplyTable("[]")
	if !strings.Contains(result, "empty") {
		t.Errorf("empty array should show empty marker, got %q", result)
	}
}

// VALIDATES: empty object shows empty marker.
// PREVENTS: blank output for empty results.
func TestApplyTableEmptyObject(t *testing.T) {
	result := ApplyTable("{}")
	if !strings.Contains(result, "empty") {
		t.Errorf("empty object should show empty marker, got %q", result)
	}
}

// VALIDATES: integer numbers display without decimals.
// PREVENTS: JSON float64 showing as "65001.000000" in table cells.
func TestApplyTableNumbers(t *testing.T) {
	input := `{"remote-as":65001,"med":100}`
	result := ApplyTable(input)
	if strings.Contains(result, ".") {
		t.Errorf("integers should not have decimals:\n%s", result)
	}
	if !strings.Contains(result, "65001") {
		t.Error("expected 65001 in output")
	}
}

// VALIDATES: array of objects with different keys fills missing cells.
// PREVENTS: panic or misaligned columns with heterogeneous objects.
func TestApplyTableMissingKeys(t *testing.T) {
	input := `[{"a":"1","b":"2"},{"a":"3","c":"4"}]`
	result := ApplyTable(input)

	// Should have columns: a, b, c.
	if !strings.Contains(result, "a") || !strings.Contains(result, "b") || !strings.Contains(result, "c") {
		t.Errorf("missing columns in header:\n%s", result)
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// top + header + separator + 2 rows + bottom = 6
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d:\n%s", len(lines), result)
	}
}

// VALIDATES: column alignment is consistent across rows.
// PREVENTS: ragged columns with varying content widths.
func TestApplyTableAlignment(t *testing.T) {
	input := `[{"x":"short","y":"a"},{"x":"much longer value","y":"b"}]`
	result := ApplyTable(input)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// All lines should have the same rune width (aligned columns).
	if len(lines) < 4 {
		t.Fatalf("too few lines: %d", len(lines))
	}
	width := len([]rune(lines[0]))
	for i, line := range lines {
		w := len([]rune(line))
		if w != width {
			t.Errorf("line %d width %d != header width %d:\n%s", i, w, width, result)
		}
	}
}

// VALIDATES: scalar JSON values pass through as-is.
// PREVENTS: wrapping simple values in unnecessary table chrome.
func TestApplyTableScalar(t *testing.T) {
	result := ApplyTable(`"hello"`)
	if strings.Contains(result, "┌") {
		t.Errorf("scalar string should not be wrapped in table:\n%s", result)
	}
}

// VALIDATES: array of primitives renders as single-column list.
// PREVENTS: crash when array contains non-objects.
func TestApplyTablePrimitiveArray(t *testing.T) {
	input := `["10.0.0.0/24","10.0.1.0/24"]`
	result := ApplyTable(input)
	if !strings.Contains(result, "10.0.0.0/24") || !strings.Contains(result, "10.0.1.0/24") {
		t.Errorf("primitive array values missing:\n%s", result)
	}
}

// VALIDATES: table pipe integrates correctly in the pipe chain.
// PREVENTS: table output breaking downstream operators.
func TestApplyPipesTable(t *testing.T) {
	input := `[{"state":"established"},{"state":"idle"}]`
	ops := []pipeOp{
		{kind: pipeTable},
		{kind: pipeMatch, arg: "established"},
		{kind: pipeCount},
	}

	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// Table renders "established" in one data row → match finds it → count = {"count":1}.
	if strings.TrimSpace(result) != `{"count":1}` {
		t.Errorf("table | match | count = %q, want %q", strings.TrimSpace(result), `{"count":1}`)
	}
}

// VALIDATES: realistic BGP peer list renders correctly as table.
// PREVENTS: regressions in real-world output formatting.
func TestApplyTableBGPPeerList(t *testing.T) {
	input := `[
		{"address":"192.168.1.2","remote-as":65002,"state":"established","routes-received":45},
		{"address":"10.0.0.1","remote-as":65003,"state":"idle","routes-received":0},
		{"address":"172.16.0.5","remote-as":65004,"state":"established","routes-received":128}
	]`
	result := ApplyTable(input)
	t.Logf("BGP peer list table:\n%s", result)

	// Verify structure.
	if !strings.Contains(result, "address") {
		t.Error("missing 'address' column header")
	}
	if !strings.Contains(result, "├") {
		t.Error("missing header separator")
	}
	if !strings.Contains(result, "192.168.1.2") {
		t.Error("missing peer address")
	}
	if !strings.Contains(result, "65002") {
		t.Error("missing peer AS number")
	}

	// All lines should have same width (aligned).
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	width := len([]rune(lines[0]))
	for i, line := range lines[1:] {
		if len([]rune(line)) != width {
			t.Errorf("line %d width mismatch: %d != %d", i+1, len([]rune(line)), width)
		}
	}
}

// VALIDATES: nested capabilities render as sub-table.
// PREVENTS: nested BGP data flattened or lost.
func TestApplyTableBGPCapabilities(t *testing.T) {
	input := `{
		"peer":"192.168.1.2",
		"state":"established",
		"negotiated":{"asn4":true,"extended-message":true}
	}`
	result := ApplyTable(input)
	t.Logf("BGP capabilities table:\n%s", result)

	// Should have nested table for "negotiated".
	if strings.Count(result, "┌") < 2 {
		t.Error("expected nested table (at least 2 top-left corners)")
	}
	if !strings.Contains(result, "asn4") {
		t.Error("nested key 'asn4' missing")
	}
}

// VALIDATES: text mode renders space-aligned columns without box-drawing.
// PREVENTS: box-drawing characters appearing in text output.
func TestApplyTextRecord(t *testing.T) {
	input := `{"count":3}`
	result := ApplyText(input)

	if strings.Contains(result, "┌") || strings.Contains(result, "│") || strings.Contains(result, "─") {
		t.Errorf("text mode should have no box-drawing characters:\n%s", result)
	}
	if !strings.Contains(result, "count") || !strings.Contains(result, "3") {
		t.Errorf("expected count and 3 in output:\n%s", result)
	}
	// Should be "count  3\n" — key and value separated by spaces.
	if strings.TrimSpace(result) != "count  3" {
		t.Errorf("got %q, want %q", strings.TrimSpace(result), "count  3")
	}
}

// VALIDATES: text mode renders array of objects as space-aligned columns.
// PREVENTS: missing headers or misaligned columns in text mode.
func TestApplyTextArray(t *testing.T) {
	input := `[{"name":"a","value":1},{"name":"b","value":2}]`
	result := ApplyText(input)

	if strings.Contains(result, "┌") || strings.Contains(result, "│") {
		t.Errorf("text mode should have no box-drawing:\n%s", result)
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// header + 2 data rows = 3 lines (no borders)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), result)
	}
	// Header line.
	if !strings.Contains(lines[0], "name") || !strings.Contains(lines[0], "value") {
		t.Errorf("missing headers: %q", lines[0])
	}
	// Data rows.
	if !strings.Contains(lines[1], "a") {
		t.Errorf("missing first data row: %q", lines[1])
	}
	if !strings.Contains(lines[2], "b") {
		t.Errorf("missing second data row: %q", lines[2])
	}
}

// VALIDATES: text mode non-JSON input passes through unchanged.
// PREVENTS: crash on non-JSON input to text formatter.
func TestApplyTextNonJSON(t *testing.T) {
	input := "this is not json"
	result := ApplyText(input)
	if result != input {
		t.Errorf("non-JSON should pass through, got %q", result)
	}
}

// VALIDATES: count | table renders count as a nice key-value table.
// PREVENTS: count output not being displayable by table pipe.
func TestApplyPipesCountThenTable(t *testing.T) {
	input := `[{"name":"a"},{"name":"b"},{"name":"c"}]`
	ops := []pipeOp{
		{kind: pipeCount},
		{kind: pipeTable},
	}

	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// count → {"count":3} → table renders as key-value record.
	if !strings.Contains(result, "count") || !strings.Contains(result, "3") {
		t.Errorf("expected count table with 3:\n%s", result)
	}
	if !strings.Contains(result, "┌") {
		t.Errorf("expected box-drawing table:\n%s", result)
	}
}

// VALIDATES: count | text renders count as plain text.
// PREVENTS: count output not rendering in text mode.
func TestApplyPipesCountThenText(t *testing.T) {
	input := `[{"name":"a"},{"name":"b"},{"name":"c"}]`
	ops := []pipeOp{
		{kind: pipeCount},
		{kind: pipeText},
	}

	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if strings.Contains(result, "┌") || strings.Contains(result, "│") {
		t.Errorf("text mode should have no box-drawing:\n%s", result)
	}
	if strings.TrimSpace(result) != "count  3" {
		t.Errorf("got %q, want %q", strings.TrimSpace(result), "count  3")
	}
}
