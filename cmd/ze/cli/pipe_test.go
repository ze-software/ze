package cli

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
			cmd, ops := parsePipe(tt.input)
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

// VALIDATES: count filter returns line count.
// PREVENTS: counting empty trailing lines.
func TestApplyCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"three lines", "a\nb\nc\n", "3"},
		{"empty", "", "0"},
		{"single line", "hello\n", "1"},
		{"no trailing newline", "a\nb\nc", "3"},
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
	result := applyJSON(input, jsonCompact)

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
	result := applyJSON(input, jsonPretty)

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("pretty JSON should be multi-line, got %d lines: %q", len(lines), result)
	}
}

// VALIDATES: json on non-JSON input passes through unchanged.
// PREVENTS: error when piping non-JSON output through json filter.
func TestApplyJSONNonJSON(t *testing.T) {
	input := "this is not json"
	result := applyJSON(input, jsonCompact)

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

	result, err := applyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	if strings.TrimSpace(result) != "2" {
		t.Errorf("match + count = %q, want %q", strings.TrimSpace(result), "2")
	}
}

// VALIDATES: unknown pipe operator returns error.
// PREVENTS: silent swallowing of typos in pipe operators.
func TestApplyPipesUnknown(t *testing.T) {
	ops := []pipeOp{{kind: pipeUnknown, arg: "bogus"}}

	_, err := applyPipes("input", ops)
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
	_, ops := parsePipe("peer list | match")
	if len(ops) != 1 {
		t.Fatal("expected 1 op")
	}
	if ops[0].kind != pipeMatch {
		t.Error("expected pipeMatch")
	}
	// Match with no arg should still parse; applyPipes should error.
	_, err := applyPipes("test", ops)
	if err == "" {
		t.Error("expected error for match with no pattern")
	}
}

// VALIDATES: json pretty is idempotent on already-pretty JSON.
// PREVENTS: double-formatting artifacts.
func TestApplyJSONPrettyIdempotent(t *testing.T) {
	input := "{\n  \"address\": \"1.2.3.4\",\n  \"state\": \"established\"\n}"
	result := applyJSON(input, jsonPretty)
	if result != input {
		t.Errorf("pretty→pretty should be idempotent:\ngot:  %q\nwant: %q", result, input)
	}
}

// VALIDATES: applyPipes with no operators returns input unchanged.
// PREVENTS: empty operator list altering output.
func TestApplyPipesEmpty(t *testing.T) {
	input := "some output"
	result, err := applyPipes(input, nil)
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
	result := applyJSON(input, jsonCompact)
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
	result, err := applyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// count("a\nb\nc\n") = "3\n", count("3\n") = "1\n".
	if strings.TrimSpace(result) != "1" {
		t.Errorf("count | count = %q, want %q", strings.TrimSpace(result), "1")
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
	result, err := applyPipes(input, ops)
	if err != "" {
		t.Fatalf("unexpected error: %s", err)
	}
	// match extracts one line → not valid JSON → json compact passes through.
	if !strings.Contains(result, "address") {
		t.Errorf("expected address in output, got %q", result)
	}
}
