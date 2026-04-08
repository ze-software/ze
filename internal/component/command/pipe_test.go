package command

import (
	"strings"
	"testing"
)

// VALIDATES: pipe operator parsing splits command from pipe chain.
// PREVENTS: pipe operators being sent to daemon as part of the command.
func TestParsePipe(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		command string
		ops     []pipeOp
	}{
		{
			name:    "no pipe",
			input:   "peer list",
			command: "peer list",
			ops:     nil,
		},
		{
			name:    "match filter",
			input:   "peer list | match established",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeMatch, arg: "established"}},
		},
		{
			name:    "count filter",
			input:   "peer list | count",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeCount}},
		},
		{
			name:    "no-more filter",
			input:   "peer list | no-more",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeNoMore}},
		},
		{
			name:    "json pretty (default)",
			input:   "peer list | json",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeJSON, arg: jsonPretty}},
		},
		{
			name:    "json pretty explicit",
			input:   "peer list | json pretty",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeJSON, arg: jsonPretty}},
		},
		{
			name:    "json compact",
			input:   "peer list | json compact",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeJSON, arg: jsonCompact}},
		},
		{
			name:    "chained pipes",
			input:   "peer list | match established | count",
			command: "peer list",
			ops: []pipeOp{
				{kind: pipeMatch, arg: "established"},
				{kind: pipeCount},
			},
		},
		{
			name:    "whitespace tolerance",
			input:   "peer list  |  match  established ",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeMatch, arg: "established"}},
		},
		{
			name:    "table filter",
			input:   "peer list | table",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeTable}},
		},
		{
			name:    "text filter",
			input:   "peer list | text",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeText}},
		},
		{
			name:    "trailing pipe no operator",
			input:   "peer list |",
			command: "peer list",
			ops:     nil,
		},
		{
			name:    "unknown pipe operator",
			input:   "peer list | bogus",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "bogus"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ops := ParsePipe(tt.input)
			if cmd != tt.command {
				t.Errorf("command = %q, want %q", cmd, tt.command)
			}
			if len(ops) != len(tt.ops) {
				t.Fatalf("got %d ops, want %d", len(ops), len(tt.ops))
			}
			for i, op := range ops {
				if op.kind != tt.ops[i].kind {
					t.Errorf("op[%d].kind = %v, want %v", i, op.kind, tt.ops[i].kind)
				}
				if op.arg != tt.ops[i].arg {
					t.Errorf("op[%d].arg = %q, want %q", i, op.arg, tt.ops[i].arg)
				}
			}
		})
	}
}

// VALIDATES: match filter selects lines containing pattern (case-insensitive).
// PREVENTS: case-sensitive matching that misses operator expectations.
func TestApplyMatch(t *testing.T) {
	input := "peer1 [established]\npeer2 [idle]\npeer3 [Established]\n"

	result := applyMatch(input, "established")
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), result)
	}
	if lines[0] != "peer1 [established]" {
		t.Errorf("line[0] = %q, want %q", lines[0], "peer1 [established]")
	}
	if lines[1] != "peer3 [Established]" {
		t.Errorf("line[1] = %q, want %q", lines[1], "peer3 [Established]")
	}
}

// VALIDATES: count filter returns JSON {"count": N}.
// PREVENTS: count output not being renderable by table/text pipes.
func TestApplyCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"three lines", "a\nb\nc\n", `{"count":3}`},
		{"empty", "", `{"count":0}`},
		{"single line", "hello\n", `{"count":1}`},
		{"no trailing newline", "a\nb\nc", `{"count":3}`},
		{"json array", `[1,2,3]`, `{"count":3}`},
		{"json object wrapper", `{"items":[1,2]}`, `{"count":2}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyCount(tt.input)
			if strings.TrimSpace(result) != tt.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(result), tt.want)
			}
		})
	}
}

// VALIDATES: json compact produces single-line JSON from pretty JSON.
// PREVENTS: multi-line JSON output when compact is requested.
func TestApplyJSONCompact(t *testing.T) {
	input := "{\n  \"address\": \"1.2.3.4\",\n  \"state\": \"established\"\n}"
	result := ApplyJSON(input, jsonCompact)

	if strings.Contains(result, "\n") {
		t.Errorf("compact JSON should be single line, got: %q", result)
	}
	if !strings.Contains(result, `"address"`) {
		t.Error("compact JSON should preserve content")
	}
}

// VALIDATES: json pretty produces indented JSON from compact JSON.
// PREVENTS: unreadable JSON output in default mode.
func TestApplyJSONPretty(t *testing.T) {
	input := `{"address":"1.2.3.4","state":"established"}`
	result := ApplyJSON(input, jsonPretty)

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("pretty JSON should be multi-line, got %d lines: %q", len(lines), result)
	}
}

// VALIDATES: json on non-JSON input passes through unchanged.
// PREVENTS: error when piping non-JSON output through json filter.
func TestApplyJSONNonJSON(t *testing.T) {
	input := "this is not json"
	result := ApplyJSON(input, jsonCompact)

	if result != input {
		t.Errorf("non-JSON should pass through, got %q", result)
	}
}

// VALIDATES: applyPipes chains multiple operators correctly.
// PREVENTS: pipe chain ordering bugs.
func TestApplyPipes(t *testing.T) {
	input := "peer1 [established]\npeer2 [idle]\npeer3 [established]\n"
	ops := []pipeOp{
		{kind: pipeMatch, arg: "established"},
		{kind: pipeCount},
	}

	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if strings.TrimSpace(result) != `{"count":2}` {
		t.Errorf("match + count = %q, want %q", strings.TrimSpace(result), `{"count":2}`)
	}
}

// VALIDATES: unknown pipe operator returns error.
// PREVENTS: silent swallowing of typos in pipe operators.
func TestApplyPipesUnknown(t *testing.T) {
	ops := []pipeOp{{kind: pipeUnknown, arg: "bogus"}}

	_, err := ApplyPipes("input", ops)
	if err == "" {
		t.Fatal("expected error for unknown pipe operator")
	}
	if !strings.Contains(err, "bogus") {
		t.Errorf("error should mention the unknown operator, got: %q", err)
	}
}

// VALIDATES: match with no argument is flagged as error.
// PREVENTS: silent no-op when user forgets the pattern.
func TestParsePipeMatchNoArg(t *testing.T) {
	_, ops := ParsePipe("peer list | match")
	if len(ops) != 1 {
		t.Fatal("expected 1 op")
	}
	if ops[0].kind != pipeMatch {
		t.Error("expected pipeMatch")
	}
	// Match with no arg should still parse; ApplyPipes should error.
	_, err := ApplyPipes("test", ops)
	if err == "" {
		t.Error("expected error for match with no pattern")
	}
}

// VALIDATES: json pretty is idempotent on already-pretty JSON.
// PREVENTS: double-formatting artifacts.
func TestApplyJSONPrettyIdempotent(t *testing.T) {
	input := "{\n  \"address\": \"1.2.3.4\",\n  \"state\": \"established\"\n}"
	result := ApplyJSON(input, jsonPretty)
	if result != input {
		t.Errorf("pretty→pretty should be idempotent:\ngot:  %q\nwant: %q", result, input)
	}
}

// VALIDATES: applyPipes with no operators returns input unchanged.
// PREVENTS: empty operator list altering output.
func TestApplyPipesEmpty(t *testing.T) {
	input := "some output"
	result, err := ApplyPipes(input, nil)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if result != input {
		t.Errorf("nil ops should return input unchanged, got %q", result)
	}
}

// VALIDATES: match with spaces in pattern works correctly.
// PREVENTS: pattern truncation when spaces are present.
func TestApplyMatchWithSpaces(t *testing.T) {
	input := "line with some pattern here\nother line\n"
	result := applyMatch(input, "some pattern")
	if !strings.Contains(result, "some pattern") {
		t.Errorf("match should find multi-word pattern, got %q", result)
	}
	if strings.Contains(result, "other line") {
		t.Error("non-matching line should be excluded")
	}
}

// VALIDATES: json on ANSI-styled input passes through unchanged.
// PREVENTS: crash or garbled output when piping styled error text through json.
func TestApplyJSONANSIPassthrough(t *testing.T) {
	// Simulate lipgloss-styled error output containing ANSI escape codes.
	input := "\x1b[38;5;196mError: unknown command\x1b[0m"
	result := ApplyJSON(input, jsonCompact)
	if result != input {
		t.Errorf("ANSI-styled text should pass through, got %q", result)
	}
}

// VALIDATES: count of count yields 1.
// PREVENTS: double-pipe same operator breaks chain.
func TestApplyPipesCountOfCount(t *testing.T) {
	input := "a\nb\nc\n"
	ops := []pipeOp{
		{kind: pipeCount},
		{kind: pipeCount},
	}
	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// count("a\nb\nc\n") = '{"count":3}\n', count('{"count":3}') = '{"count":1}\n'.
	// Second count: JSON parses → single-key map → unwrap → float64(3) → 1 item.
	if strings.TrimSpace(result) != `{"count":1}` {
		t.Errorf("count | count = %q, want %q", strings.TrimSpace(result), `{"count":1}`)
	}
}

// VALIDATES: FoldServerPipeline folds pipe segments into rib routes command args.
// PREVENTS: server-side pipeline keywords being treated as unknown client ops.
func TestFoldServerPipeline(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		ops        []pipeOp
		wantCmd    string
		wantOpsLen int
	}{
		{
			name:       "non-rib-routes unchanged",
			command:    "peer list",
			ops:        []pipeOp{{kind: pipeMatch, arg: "established"}},
			wantCmd:    "peer list",
			wantOpsLen: 1,
		},
		{
			name:       "bgp rib routes with path filter",
			command:    "bgp rib routes received",
			ops:        []pipeOp{{kind: pipeUnknown, arg: "path 65001"}},
			wantCmd:    "bgp rib routes received path 65001",
			wantOpsLen: 0,
		},
		{
			name:       "bgp rib routes with count terminal",
			command:    "bgp rib routes",
			ops:        []pipeOp{{kind: pipeCount}},
			wantCmd:    "bgp rib routes count",
			wantOpsLen: 0,
		},
		{
			name:       "bgp rib routes with match filter",
			command:    "bgp rib routes received",
			ops:        []pipeOp{{kind: pipeMatch, arg: "10.0.0.0"}},
			wantCmd:    "bgp rib routes received match 10.0.0.0",
			wantOpsLen: 0,
		},
		{
			name:       "bgp rib routes keeps no-more client-side",
			command:    "bgp rib routes",
			ops:        []pipeOp{{kind: pipeUnknown, arg: "path 65001"}, {kind: pipeNoMore}},
			wantCmd:    "bgp rib routes path 65001",
			wantOpsLen: 1,
		},
		{
			name:       "bgp rib routes keeps table client-side",
			command:    "bgp rib routes received",
			ops:        []pipeOp{{kind: pipeCount}, {kind: pipeTable}},
			wantCmd:    "bgp rib routes received count",
			wantOpsLen: 1,
		},
		{
			name:       "bgp rib routes with json terminal",
			command:    "bgp rib routes",
			ops:        []pipeOp{{kind: pipeJSON, arg: jsonPretty}},
			wantCmd:    "bgp rib routes json",
			wantOpsLen: 0,
		},
		{
			name:       "bgp rib show best with path filter",
			command:    "bgp rib show best",
			ops:        []pipeOp{{kind: pipeUnknown, arg: "path 65001"}},
			wantCmd:    "bgp rib show best path 65001",
			wantOpsLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ops := FoldServerPipeline(tt.command, tt.ops)
			if cmd != tt.wantCmd {
				t.Errorf("command = %q, want %q", cmd, tt.wantCmd)
			}
			if len(ops) != tt.wantOpsLen {
				t.Errorf("got %d client ops, want %d", len(ops), tt.wantOpsLen)
			}
		})
	}
}

// VALIDATES: parsePipe preserves full segment text for unknown ops.
// PREVENTS: loss of filter arguments (e.g., "path 65001" becomes just "path").
func TestParsePipeUnknownPreservesArgs(t *testing.T) {
	_, ops := ParsePipe("bgp rib routes | path 65001")
	if len(ops) != 1 {
		t.Fatalf("got %d ops, want 1", len(ops))
	}
	if ops[0].arg != "path 65001" {
		t.Errorf("arg = %q, want %q", ops[0].arg, "path 65001")
	}
}

// VALIDATES: match then json compact on partial JSON lines passes through.
// PREVENTS: crash when json filter receives non-JSON from match output.
func TestApplyPipesMatchThenJSON(t *testing.T) {
	input := "{\n  \"address\": \"1.2.3.4\",\n  \"state\": \"established\"\n}"
	ops := []pipeOp{
		{kind: pipeMatch, arg: "address"},
		{kind: pipeJSON, arg: jsonCompact},
	}
	result, err := ApplyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// match extracts one line → not valid JSON → json compact passes through.
	if !strings.Contains(result, "address") {
		t.Errorf("expected address in output, got %q", result)
	}
}

// VALIDATES: multiple format operators are rejected.
// PREVENTS: confusing silent passthrough when stacking formatters.
func TestApplyPipesMultipleFormats(t *testing.T) {
	ops := []pipeOp{{kind: pipeText}, {kind: pipeJSON, arg: jsonPretty}}
	_, err := ApplyPipes(`{"a":1}`, ops)
	if err == "" {
		t.Fatal("expected error for multiple format operators")
	}
	if !strings.Contains(err, "multiple format") {
		t.Errorf("error should mention multiple formats, got: %q", err)
	}
}

// TestProcessPipesDefaultTable verifies default table format is added when no format pipe present.
//
// VALIDATES: ProcessPipesDefaultTable adds table format by default.
// PREVENTS: Editor command mode showing raw JSON instead of table.
func TestProcessPipesDefaultTable(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCmd   string
		wantTable bool // true if result should contain box-drawing chars (table)
	}{
		{"no pipe adds table", "peer list", "peer list", true},
		{"match only adds table", "peer list | match name", "peer list", true},
		{"explicit json skips table", "peer list | json", "peer list", false},
		{"explicit table keeps table", "peer list | table", "peer list", true},
		{"explicit text skips table", "peer list | text", "peer list", false},
		{"explicit yaml skips table", "peer list | yaml", "peer list", false},
		{"count gets default table", "peer list | count", "peer list", true},
	}

	jsonInput := `[{"name":"a","value":1},{"name":"b","value":2}]`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, format := ProcessPipesDefaultTable(tt.input)
			if cmd != tt.wantCmd {
				t.Errorf("command = %q, want %q", cmd, tt.wantCmd)
			}
			result := format(jsonInput)
			hasTable := strings.Contains(result, "┌") || strings.Contains(result, "│")
			if hasTable != tt.wantTable {
				t.Errorf("hasTable = %v, want %v; result:\n%s", hasTable, tt.wantTable, result)
			}
		})
	}
}

// TestProcessPipesDefaultFunc verifies custom default formatter is used when no format pipe present.
//
// VALIDATES: ProcessPipesDefaultFunc applies the provided default function.
// PREVENTS: Monitor streaming showing raw JSON or table instead of compact one-liner.
func TestProcessPipesDefaultFunc(t *testing.T) {
	customFmt := func(s string) string { return "CUSTOM:" + s }

	tests := []struct {
		name       string
		input      string
		wantCmd    string
		wantCustom bool // true if result should use custom formatter
	}{
		{"no pipe uses custom", "monitor event", "monitor event", true},
		{"explicit json overrides custom", "monitor event | json", "monitor event", false},
		{"explicit table overrides custom", "monitor event | table", "monitor event", false},
		{"explicit text overrides custom", "monitor event | text", "monitor event", false},
		{"explicit yaml overrides custom", "monitor event | yaml", "monitor event", false},
		{"match only uses custom", "monitor event | match state", "monitor event", true},
	}

	jsonInput := `{"key":"value"}`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, format := ProcessPipesDefaultFunc(tt.input, customFmt)
			if cmd != tt.wantCmd {
				t.Errorf("command = %q, want %q", cmd, tt.wantCmd)
			}
			result := format(jsonInput)
			hasCustom := strings.HasPrefix(result, "CUSTOM:")
			if hasCustom != tt.wantCustom {
				t.Errorf("hasCustom = %v, want %v; result: %q", hasCustom, tt.wantCustom, result)
			}
		})
	}
}
