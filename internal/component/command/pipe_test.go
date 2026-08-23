package command

import (
	"encoding/json"
	"iter"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		// Rows beside aggregates: the rows are counted, not the keys. This
		// answered 6 on `show bgp`, which is the number of top-level keys.
		{"rows beside aggregates", `{"peers":[{"a":1},{"a":2}],"total":2,"up":1}`, `{"count":2}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, msg := applyCount(tt.input)
			if msg != "" {
				t.Fatalf("refused: %s", msg)
			}
			if strings.TrimSpace(result) != tt.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(result), tt.want)
			}
		})
	}
}

// TestApplyCountRefusesWhatItCannotCount holds count to refusing rather than
// answering a plausible wrong number. Both cases shipped an answer before.
func TestApplyCountRefusesWhatItCannotCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "one document has no rows",
			input: `{"version":"ze dev","built":"unknown"}`,
			want:  "it holds one document",
		},
		{
			name:  "several lists name the candidates",
			input: `{"peers":[{"a":1}],"routes":[{"b":2}],"total":2}`,
			want:  "several lists (peers, routes)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, msg := applyCount(tt.input)
			if msg == "" {
				t.Fatalf("count answered %q instead of refusing", out)
			}
			if !strings.Contains(msg, tt.want) {
				t.Errorf("refusal %q does not say %q", msg, tt.want)
			}
			if !strings.HasPrefix(msg, "count ") {
				t.Errorf("refusal %q does not name the operator", msg)
			}
		})
	}
}

// TestApplyTakeKeepsTheEnvelope proves first and last shorten the ROWS and
// leave the aggregates beside them. `show bgp | first 1` answered the whole
// payload, because the old truncateItems unwrapped a map of exactly one key
// and returned a map of six untouched.
func TestApplyTakeKeepsTheEnvelope(t *testing.T) {
	const envelope = `{"peers":[{"n":1},{"n":2},{"n":3}],"total":3}`

	got, msg := applyFirst(envelope, "2")
	if msg != "" {
		t.Fatalf("first refused: %s", msg)
	}
	rows, key, ok := rowsIn(decode(t, got))
	if !ok || key != "peers" {
		t.Fatalf("first lost the envelope: %s", got)
	}
	if len(rows) != 2 {
		t.Errorf("first 2 kept %d rows, want 2: %s", len(rows), got)
	}
	if !strings.Contains(got, `"total":3`) {
		t.Errorf("first dropped the aggregate beside the rows: %s", got)
	}

	got, msg = applyLast(envelope, "1")
	if msg != "" {
		t.Fatalf("last refused: %s", msg)
	}
	if !strings.Contains(got, `"n":3`) {
		t.Errorf("last 1 kept the wrong row: %s", got)
	}
}

// TestApplyTakeRefusesOneValue is the measured defect: `show version | first 1`
// answered the version string, dropping the key the bare command prints.
func TestApplyTakeRefusesOneValue(t *testing.T) {
	out, msg := applyFirst(`{"version":"ze dev","built":"unknown"}`, "1")
	if msg == "" {
		t.Fatalf("first answered %q instead of refusing", out)
	}
	if !strings.HasPrefix(msg, "first ") {
		t.Errorf("refusal %q does not name the operator", msg)
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

	result, err := ApplyPipes(input, ops, nil, nil)
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

	_, err := ApplyPipes("input", ops, nil, nil)
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
	_, err := ApplyPipes("test", ops, nil, nil)
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
	result, err := ApplyPipes(input, nil, nil, nil)
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

// VALIDATES: count is refused when repeated, by name.
// PREVENTS: a chain answering a number nobody asked for.
//
// This test asserted `count | count` yields 1, which was the old behavior:
// the second count unwrapped the first one's single-key answer and counted the
// number inside it. The catalog now declares count as RepeatRefuse, because
// counting a count has no honest answer, and the validator refuses the chain
// before it runs (plan/spec-cli-pipe-operator-coverage.md, AC-5).
func TestApplyPipesCountOfCount(t *testing.T) {
	ops := []pipeOp{
		{kind: pipeCount},
		{kind: pipeCount},
	}
	msg := ValidatePipes(ops)
	if msg == "" {
		t.Fatal("count | count was accepted; it must be refused")
	}
	if !strings.HasPrefix(msg, "count cannot be repeated") {
		t.Errorf("refusal %q does not name the operator and the reason", msg)
	}
}

// VALIDATES: foldFilters folds registered filters into command args.
// PREVENTS: filters being hardcoded in generic pipe code.
func TestFoldFilters(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"},
		PipeFilter{Name: "first", Description: "first source", Leading: true},
		PipeFilter{Name: "second", Description: "second source", Leading: true},
		PipeFilter{Name: "path", Description: "AS path", TakesArg: true},
		PipeFilter{Name: "match", Description: "structured match", TakesArg: true},
		PipeFilter{Name: "count", Description: "count routes"},
	)
	RegisterPipeFilters([]string{"show routes status"})

	tests := []struct {
		name    string
		command string
		ops     []pipeOp
		wantCmd string
		wantOps []pipeKind
	}{
		{
			name:    "unregistered command unchanged",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeMatch, arg: "established"}},
			wantCmd: "peer list",
			wantOps: []pipeKind{pipeMatch},
		},
		{
			name:    "registered command with path filter",
			command: "show routes first",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "path 65001"}},
			wantCmd: "show routes first path 65001",
		},
		{
			name:    "registered command rejects unknown filter",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "bogus"}},
			wantCmd: "show routes",
			wantOps: []pipeKind{pipeInvalid},
		},
		{
			name:    "registered command rejects missing filter argument",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "path"}},
			wantCmd: "show routes",
			wantOps: []pipeKind{pipeInvalid},
		},
		{
			name:    "registered command rejects extra flag argument",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "first unexpected"}},
			wantCmd: "show routes",
			wantOps: []pipeKind{pipeInvalid},
		},
		{
			name:    "registered command with count terminal",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeCount}},
			wantCmd: "show routes count",
		},
		{
			name:    "registered command reorders leading selector",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "path 65001"}, {kind: pipeUnknown, arg: "second"}},
			wantCmd: "show routes second path 65001",
		},
		{
			name:    "match folded server-side when registered",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeMatch, arg: "10.0.0.0"}},
			wantCmd: "show routes match 10.0.0.0",
		},
		{
			name:    "match stays client-side when not registered",
			command: "peer list",
			ops:     []pipeOp{{kind: pipeMatch, arg: "established"}},
			wantCmd: "peer list",
			wantOps: []pipeKind{pipeMatch},
		},
		{
			name:    "registered command keeps no-more client-side",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "path 65001"}, {kind: pipeNoMore}},
			wantCmd: "show routes path 65001",
			wantOps: []pipeKind{pipeNoMore},
		},
		{
			name:    "registered command keeps table client-side",
			command: "show routes first",
			ops:     []pipeOp{{kind: pipeCount}, {kind: pipeTable}},
			wantCmd: "show routes first count",
			wantOps: []pipeKind{pipeTable},
		},
		{
			name:    "registered command keeps json client-side",
			command: "show routes",
			ops:     []pipeOp{{kind: pipeJSON, arg: jsonPretty}},
			wantCmd: "show routes",
			wantOps: []pipeKind{pipeJSON},
		},
		{
			name:    "more specific command with empty registration keeps count client-side",
			command: "show routes status",
			ops:     []pipeOp{{kind: pipeCount}},
			wantCmd: "show routes status",
			wantOps: []pipeKind{pipeCount},
		},
		{
			name:    "more specific command with empty registration rejects route filters as generic unknowns",
			command: "show routes status",
			ops:     []pipeOp{{kind: pipeUnknown, arg: "first"}},
			wantCmd: "show routes status",
			wantOps: []pipeKind{pipeUnknown},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ops, _ := foldFilters(tt.command, tt.ops)
			if cmd != tt.wantCmd {
				t.Errorf("command = %q, want %q", cmd, tt.wantCmd)
			}
			if len(ops) != len(tt.wantOps) {
				t.Fatalf("got %d client ops, want %d", len(ops), len(tt.wantOps))
			}
			for i, want := range tt.wantOps {
				if ops[i].kind != want {
					t.Errorf("client op %d = %v, want %v", i, ops[i].kind, want)
				}
			}
		})
	}
}

// VALIDATES: pipe validation reports metadata-derived errors.
// PREVENTS: typos being forwarded as server arguments after a pipe.
func TestFoldFiltersValidationErrors(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"},
		PipeFilter{Name: "received", Description: "received source", Leading: true},
		PipeFilter{Name: "path", Description: "AS path", TakesArg: true},
	)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unknown filter lists registered filters",
			input: "show routes | bogus",
			want:  "unknown pipe filter for show routes: bogus (valid: path, received)",
		},
		{
			name:  "missing argument",
			input: "show routes | path",
			want:  "pipe filter path requires an argument",
		},
		{
			name:  "extra argument",
			input: "show routes | received now",
			want:  "pipe filter received does not accept an argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ops := ParsePipe(tt.input)
			_, ops, _ = foldFilters(cmd, ops)
			if got := ValidatePipes(ops); got != tt.want {
				t.Fatalf("validation error = %q, want %q", got, tt.want)
			}
		})
	}
}

// VALIDATES: checked pipe processing returns validation errors before formatting.
// PREVENTS: invalid filters dispatching the base command first.
// TestHasFormatPipe covers the predicate that gives an explicit format operator
// precedence over a caller's own default.
//
// VALIDATES: spec-netlab-integration AC-10, the pipe-engine half.
// PREVENTS:  a caller reading "count" or "match" as a format and stepping aside
//
//	when it should still apply its default. Both are data transforms
//	whose result is JSON for a downstream formatter, which is the
//	distinction hasFormatOp already draws for the interactive path.
func TestHasFormatPipe(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"show bgp peer list", false},
		{"show bgp peer list | json", true},
		{"show bgp peer list | json compact", true},
		{"show bgp peer list | ndjson", true},
		{"show bgp peer list | yaml", true},
		{"show bgp peer list | table", true},
		{"show bgp peer list | text", true},
		{"show bgp peer list | count", false},
		{"show bgp peer list | match established", false},
		{"show bgp peer list | match established | json", true},
	}
	for _, tt := range tests {
		if got := HasFormatPipe(tt.input); got != tt.want {
			t.Errorf("HasFormatPipe(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestProcessPipesChecked_InvalidFilter(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"}, PipeFilter{Name: "path", Description: "AS path", TakesArg: true})

	cmd, format, errMsg := ProcessPipesChecked("show routes | bogus")
	if cmd != "show routes" {
		t.Fatalf("command = %q, want show routes", cmd)
	}
	if format != nil {
		t.Fatal("format should be nil on validation error")
	}
	if errMsg != "unknown pipe filter for show routes: bogus (valid: path)" {
		t.Fatalf("error = %q", errMsg)
	}
}

// VALIDATES: display pipes stay client-side when server filters are folded.
// PREVENTS: explicit JSON output being reformatted by the interactive default formatter.
func TestProcessPipesDefaultFormat_FilterKeepsJSON(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"}, PipeFilter{Name: "count", Description: "count routes"})
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	cmd, format, _ := ProcessPipesDefaultFormatChecked("show routes | count | json", "")
	if cmd != "show routes count" {
		t.Fatalf("command = %q, want %q", cmd, "show routes count")
	}

	result := format(`{"count":2}`)
	if !strings.Contains(result, `"count": 2`) {
		t.Fatalf("expected JSON output, got %q", result)
	}
}

// VALIDATES: parsePipe preserves full segment text for unknown ops.
// PREVENTS: loss of filter arguments (e.g., "path 65001" becomes just "path").
func TestParsePipeUnknownPreservesArgs(t *testing.T) {
	_, ops := ParsePipe("show routes | path 65001")
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
	result, err := ApplyPipes(input, ops, nil, nil)
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
	_, err := ApplyPipes(`{"a":1}`, ops, nil, nil)
	if err == "" {
		t.Fatal("expected error for multiple format operators")
	}
	if !strings.Contains(err, "multiple format") {
		t.Errorf("error should mention multiple formats, got: %q", err)
	}
}

// TestProcessPipesDefaultFormat verifies default format is applied when no format pipe present.
//
// VALIDATES: ProcessPipesDefaultFormatChecked uses configuredDefault() (text by default).
// PREVENTS: Editor command mode showing raw JSON instead of formatted output.
func TestProcessPipesDefaultFormat(t *testing.T) {
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	tests := []struct {
		name      string
		input     string
		wantCmd   string
		wantTable bool // true if result should contain box-drawing chars (table)
	}{
		{"no pipe uses text default", "peer list", "peer list", false},
		{"match only uses text default", "peer list | match name", "peer list", false},
		{"explicit json skips default", "peer list | json", "peer list", false},
		{"explicit table produces table", "peer list | table", "peer list", true},
		{"explicit text produces text", "peer list | text", "peer list", false},
		{"explicit yaml skips default", "peer list | yaml", "peer list", false},
		{"count uses text default", "peer list | count", "peer list", false},
	}

	jsonInput := `[{"name":"a","value":1},{"name":"b","value":2}]`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, format, _ := ProcessPipesDefaultFormatChecked(tt.input, "")
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

// TestConfiguredDefault verifies configuredDefault() maps env values to pipeKind.
func TestConfiguredDefault(t *testing.T) {
	tests := []struct {
		envVal      string
		wantTable   bool
		wantContain string
	}{
		{"text", false, "a"},
		{"table", true, ""},
		{"json", false, "\"name\""},
		{"yaml", false, "name:"},
		{"ndjson", false, "{\"name\""},
	}

	jsonInput := `[{"name":"a","value":1}]`

	for _, tt := range tests {
		t.Run(tt.envVal, func(t *testing.T) {
			t.Setenv("ze.cli.format", tt.envVal)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			cmd, format, _ := ProcessPipesDefaultFormatChecked("peer list", "")
			if cmd != "peer list" {
				t.Errorf("command = %q, want peer list", cmd)
			}
			result := format(jsonInput)
			hasTable := strings.Contains(result, "┌") || strings.Contains(result, "│")
			if hasTable != tt.wantTable {
				t.Errorf("env=%q hasTable = %v, want %v", tt.envVal, hasTable, tt.wantTable)
			}
			if tt.wantContain != "" && !strings.Contains(result, tt.wantContain) {
				t.Errorf("env=%q result missing %q; got:\n%s", tt.envVal, tt.wantContain, result)
			}
		})
	}
}

// TestSessionFormatOverridesConfiguredDefault verifies precedence: a session's
// `set cli format` override beats the configured default, and an empty override
// leaves the configured default in force.
//
// VALIDATES: AC-16, AC-17 -- session override > config/env > built-in default.
// PREVENTS: The leak fix breaking the config default, or the override being ignored.
func TestSessionFormatOverridesConfiguredDefault(t *testing.T) {
	jsonInput := `[{"name":"a","value":1}]`
	isTable := func(s string) bool { return strings.Contains(s, "┌") || strings.Contains(s, "│") }

	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	// No session override: the configured default (table) applies.
	_, format, _ := ProcessPipesDefaultFormatChecked("peer list", "")
	if !isTable(format(jsonInput)) {
		t.Errorf("no override: want the configured default (table)")
	}

	// Session override wins over the configured default.
	_, format, _ = ProcessPipesDefaultFormatChecked("peer list", "json")
	result := format(jsonInput)
	if isTable(result) {
		t.Errorf("session override ignored; got table:\n%s", result)
	}
	if !strings.Contains(result, "\"name\"") {
		t.Errorf("session override should produce json; got:\n%s", result)
	}

	// One session's override must not alter what the next resolves.
	_, format, _ = ProcessPipesDefaultFormatChecked("peer list", "")
	if !isTable(format(jsonInput)) {
		t.Errorf("a session override must not persist into a session that has none")
	}
}

// TestConfiguredDefaultInvalid verifies configuredDefault() falls back to pipeText for invalid values.
func TestConfiguredDefaultInvalid(t *testing.T) {
	t.Setenv("ze.cli.format", "bogus")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	jsonInput := `[{"name":"a","value":1}]`
	_, format, _ := ProcessPipesDefaultFormatChecked("peer list", "")
	result := format(jsonInput)
	hasTable := strings.Contains(result, "┌") || strings.Contains(result, "│")
	if hasTable {
		t.Errorf("invalid env value should fall back to text, got table output")
	}
}

// TestProcessPipesDefaultFormat_Configured verifies explicit pipe wins over configured default.
func TestProcessPipesDefaultFormat_Configured(t *testing.T) {
	t.Setenv("ze.cli.format", "json")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	jsonInput := `[{"name":"a","value":1}]`

	// Explicit | table should produce table even when default is json.
	_, format, _ := ProcessPipesDefaultFormatChecked("peer list | table", "")
	result := format(jsonInput)
	hasTable := strings.Contains(result, "┌") || strings.Contains(result, "│")
	if !hasTable {
		t.Errorf("explicit | table should produce table even with json default; result:\n%s", result)
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

func TestParsePipe_Resolve(t *testing.T) {
	cmd, ops := ParsePipe("show traceroute 1.1.1.1 | resolve")
	if cmd != "show traceroute 1.1.1.1" {
		t.Errorf("command = %q, want %q", cmd, "show traceroute 1.1.1.1")
	}
	if len(ops) != 1 || ops[0].kind != pipeResolve {
		t.Errorf("ops = %v, want [resolve]", ops)
	}
}

func TestParsePipe_ResolveAndJSON(t *testing.T) {
	cmd, ops := ParsePipe("show traceroute 1.1.1.1 | resolve | json")
	if cmd != "show traceroute 1.1.1.1" {
		t.Errorf("command = %q, want %q", cmd, "show traceroute 1.1.1.1")
	}
	if len(ops) != 2 || ops[0].kind != pipeResolve || ops[1].kind != pipeJSON {
		t.Errorf("ops = %v, want [resolve, json]", ops)
	}
}

func TestApplyJSON_UnwrapsSingleKeyArray(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"10.0.0.1","rtt-ms":1.5},{"ttl":2,"addr":"10.0.0.2","rtt-ms":2.5}]}`
	result := ApplyJSON(input, "compact")
	if !strings.HasPrefix(result, "[") {
		t.Errorf("compact JSON should be a valid array: %s", result)
	}
	if !strings.Contains(result, "10.0.0.1") || !strings.Contains(result, "10.0.0.2") {
		t.Errorf("should contain both IPs: %s", result)
	}
}

func TestApplyJSON_PrettyIsValidJSON(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"10.0.0.1"},{"ttl":2,"addr":"10.0.0.2"}]}`
	result := ApplyJSON(input, "pretty")
	if !strings.HasPrefix(strings.TrimSpace(result), "[") {
		t.Errorf("pretty JSON should be a valid array: %s", result)
	}
}

func TestApplyNDJSON(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"10.0.0.1","rtt-ms":1.5},{"ttl":2,"addr":"10.0.0.2","rtt-ms":2.5}]}`
	result := applyNDJSON(input)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 NDJSON lines, got %d: %q", len(lines), result)
	}
	if !strings.Contains(lines[0], "10.0.0.1") {
		t.Errorf("line 0 should contain 10.0.0.1: %s", lines[0])
	}
	if !strings.Contains(lines[1], "10.0.0.2") {
		t.Errorf("line 1 should contain 10.0.0.2: %s", lines[1])
	}
}

type mockPTRResolver struct {
	results map[string][]string
}

func (m *mockPTRResolver) ResolvePTR(address string) ([]string, error) {
	if r, ok := m.results[address]; ok {
		return r, nil
	}
	return nil, nil
}

func TestApplyResolve_UsesSystemResolver(t *testing.T) {
	mock := &mockPTRResolver{results: map[string][]string{
		"154.54.74.6": {"router.example.com."},
		"10.0.0.1":    {"gw.example.com."},
	}}
	SetPTRResolver(mock)
	defer SetPTRResolver(nil)

	input := `{"hops":[{"ttl":1,"addr":"10.0.0.1","rtt-ms":1.0},{"ttl":2,"addr":"154.54.74.6","rtt-ms":5.0}]}`
	result := applyResolve(input)
	if !strings.Contains(result, "gw.example.com") {
		t.Errorf("expected gw.example.com in result: %s", result)
	}
	if !strings.Contains(result, "router.example.com") {
		t.Errorf("expected router.example.com in result: %s", result)
	}
}

func TestApplyResolve_FallbackReverseLookup(t *testing.T) {
	SetPTRResolver(nil)
	input := `{"addr":"127.0.0.1"}`
	result := applyResolve(input)
	t.Logf("fallback result: %s", result)
	if !strings.Contains(result, "addr-name") {
		t.Errorf("should add addr-name field: %s", result)
	}
}

func TestApplyResolve_AddsNameField(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"127.0.0.1","rtt-ms":0.1}]}`
	result := applyResolve(input)
	if !strings.Contains(result, "addr-name") {
		t.Errorf("resolve should add addr-name field: %s", result)
	}
}

func TestApplyResolve_SkipsStar(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"*","rtt-ms":null}]}`
	result := applyResolve(input)
	if strings.Contains(result, "addr-name") {
		t.Errorf("resolve should skip '*' addresses: %s", result)
	}
}

func TestApplyPipes_ResolveThenJSON(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"127.0.0.1","rtt-ms":0.1}]}`
	ops := []pipeOp{{kind: pipeResolve}, {kind: pipeJSON, arg: "compact"}}
	result, errMsg := ApplyPipes(input, ops, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(result, "addr-name") {
		t.Errorf("result should contain addr-name: %s", result)
	}
	if !strings.Contains(result, "127.0.0.1") {
		t.Errorf("result should contain IP: %s", result)
	}
}

func TestApplyPipes_JSONThenResolve(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"127.0.0.1","rtt-ms":0.1},{"ttl":2,"addr":"127.0.0.1","rtt-ms":0.2}]}`
	ops := []pipeOp{{kind: pipeJSON, arg: "compact"}, {kind: pipeResolve}}
	result, errMsg := ApplyPipes(input, ops, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(result, "addr-name") {
		t.Errorf("result should contain addr-name: %s", result)
	}
}

func TestProcessPipesDetectLog_HasFormat(t *testing.T) {
	_, _, flags, errMsg := ProcessPipesDetectLog("monitor ping 1.1.1.1 | log | json", "")
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !flags.Log {
		t.Error("expected Log flag")
	}
	if !flags.HasFormat {
		t.Error("expected HasFormat flag for explicit | json")
	}
}

func TestProcessPipesDetectLog_NoExplicitFormat(t *testing.T) {
	_, _, flags, errMsg := ProcessPipesDetectLog("monitor ping 1.1.1.1 | log", "")
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !flags.Log {
		t.Error("expected Log flag")
	}
	if flags.HasFormat {
		t.Error("expected HasFormat=false when no explicit format pipe")
	}
}

func TestProcessPipesDetectLog_NDJSON(t *testing.T) {
	_, formatFn, flags, errMsg := ProcessPipesDetectLog("monitor ping 1.1.1.1 | log | ndjson", "")
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !flags.HasFormat {
		t.Error("expected HasFormat flag for explicit | ndjson")
	}
	result := formatFn(`{"seq":1,"status":"ok","rtt-ms":1.234}`)
	if !strings.Contains(result, `"seq"`) {
		t.Errorf("expected JSON output, got: %s", result)
	}
}

func TestApplyPipes_NDJSONThenResolve(t *testing.T) {
	input := `{"hops":[{"ttl":1,"addr":"127.0.0.1","rtt-ms":0.1},{"ttl":2,"addr":"127.0.0.1","rtt-ms":0.2}]}`
	ops := []pipeOp{{kind: pipeNDJSON}, {kind: pipeResolve}}
	result, errMsg := ApplyPipes(input, ops, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(result, "addr-name") {
		t.Errorf("result should contain addr-name: %s", result)
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 NDJSON lines, got %d: %q", len(lines), result)
	}
}

func TestParsePipeFirst(t *testing.T) {
	cmd, ops := ParsePipe("show routes | first 10")
	if cmd != "show routes" {
		t.Errorf("command = %q, want %q", cmd, "show routes")
	}
	if len(ops) != 1 || ops[0].kind != pipeFirst || ops[0].arg != "10" {
		t.Errorf("ops = %+v, want [{pipeFirst 10}]", ops)
	}
}

func TestParsePipeLast(t *testing.T) {
	cmd, ops := ParsePipe("show routes | last 5")
	if cmd != "show routes" {
		t.Errorf("command = %q, want %q", cmd, "show routes")
	}
	if len(ops) != 1 || ops[0].kind != pipeLast || ops[0].arg != "5" {
		t.Errorf("ops = %+v, want [{pipeLast 5}]", ops)
	}
}

func TestApplyFirst(t *testing.T) {
	input := `[{"a":1},{"a":2},{"a":3},{"a":4},{"a":5}]`
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeFirst, arg: "3"}}, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	expected := `[{"a":1},{"a":2},{"a":3}]`
	trimmed := strings.TrimSpace(result)
	if trimmed != expected {
		t.Errorf("got %q, want %q", trimmed, expected)
	}
}

func TestApplyLast(t *testing.T) {
	input := `[{"a":1},{"a":2},{"a":3},{"a":4},{"a":5}]`
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeLast, arg: "2"}}, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	expected := `[{"a":4},{"a":5}]`
	trimmed := strings.TrimSpace(result)
	if trimmed != expected {
		t.Errorf("got %q, want %q", trimmed, expected)
	}
}

func TestApplyFirstUnderCount(t *testing.T) {
	input := `[{"a":1},{"a":2}]`
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeFirst, arg: "10"}}, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	expected := `[{"a":1},{"a":2}]`
	trimmed := strings.TrimSpace(result)
	if trimmed != expected {
		t.Errorf("got %q, want %q", trimmed, expected)
	}
}

// TestApplyFirstNonArray asserted that `first` passed a rowless answer through
// unchanged. That silence is the defect: `show version | first 1` answered the
// version string and dropped the key the bare command prints, and a caller had
// no way to learn the operator had not applied. It is now refused by name
// (plan/spec-cli-pipe-operator-coverage.md, AC-3).
func TestApplyFirstNonArray(t *testing.T) {
	input := `{"count":42}`
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeFirst, arg: "3"}}, nil, nil)
	if errMsg == "" {
		t.Fatalf("first answered %q instead of refusing a rowless answer", result)
	}
	if !strings.HasPrefix(errMsg, "first ") {
		t.Errorf("refusal %q does not name the operator", errMsg)
	}
}

func TestFirstValidation(t *testing.T) {
	tests := []struct {
		name string
		ops  []pipeOp
		want string
	}{
		{"first no arg", []pipeOp{{kind: pipeFirst}}, "first requires a numeric argument"},
		{"first zero", []pipeOp{{kind: pipeFirst, arg: "0"}}, "first requires a positive number"},
		{"first negative", []pipeOp{{kind: pipeFirst, arg: "-1"}}, "first requires a positive number"},
		{"first non-numeric", []pipeOp{{kind: pipeFirst, arg: "abc"}}, "first requires a positive number"},
		{"last no arg", []pipeOp{{kind: pipeLast}}, "last requires a numeric argument"},
		{"last zero", []pipeOp{{kind: pipeLast, arg: "0"}}, "last requires a positive number"},
		{"first valid", []pipeOp{{kind: pipeFirst, arg: "5"}}, ""},
		{"last valid", []pipeOp{{kind: pipeLast, arg: "5"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidatePipes(tt.ops)
			if got != tt.want {
				t.Errorf("ValidatePipes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFoldFiltersFirst(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"},
		PipeFilter{Name: "first", Description: "Take first N", TakesArg: true},
		PipeFilter{Name: "last", Description: "Take last N", TakesArg: true},
	)

	cmd, ops, _ := foldFilters("show routes", []pipeOp{{kind: pipeFirst, arg: "10"}})
	if cmd != "show routes first 10" {
		t.Errorf("command = %q, want %q", cmd, "show routes first 10")
	}
	if len(ops) != 0 {
		t.Errorf("expected 0 client ops, got %d", len(ops))
	}
}

func TestFoldFiltersFirstNotRegistered(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)

	cmd, ops, _ := foldFilters("peer list", []pipeOp{{kind: pipeFirst, arg: "5"}})
	if cmd != "peer list" {
		t.Errorf("command = %q, want %q", cmd, "peer list")
	}
	if len(ops) != 1 || ops[0].kind != pipeFirst {
		t.Errorf("expected 1 client pipeFirst op, got %+v", ops)
	}
}

func TestPipeMetadataInjected(t *testing.T) {
	meta := map[string]any{"received": true, "family": "ipv4-unicast", "first": 100}

	// Map output: metadata injected directly.
	mapInput := `{"count":42}`
	result, errMsg := ApplyPipes(mapInput, nil, meta, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !strings.Contains(result, `"pipe"`) {
		t.Errorf("expected pipe metadata in map output, got %s", result)
	}
	if !strings.Contains(result, `"received":true`) {
		t.Errorf("expected received:true in pipe metadata, got %s", result)
	}
	if !strings.Contains(result, `"first":100`) {
		t.Errorf("expected first:100 in pipe metadata, got %s", result)
	}

	// Array output: wrapped as {"data": [...], "pipe": {...}}.
	arrInput := `[{"a":1},{"a":2}]`
	result2, errMsg2 := ApplyPipes(arrInput, nil, meta, nil)
	if errMsg2 != "" {
		t.Fatalf("unexpected error: %s", errMsg2)
	}
	if !strings.Contains(result2, `"pipe"`) {
		t.Errorf("expected pipe metadata in wrapped array output, got %s", result2)
	}
	if !strings.Contains(result2, `"data"`) {
		t.Errorf("expected data key wrapping array, got %s", result2)
	}

	// Count pipe on array: count produces map, metadata injected into map.
	result3, errMsg3 := ApplyPipes(arrInput, []pipeOp{{kind: pipeCount}}, meta, nil)
	if errMsg3 != "" {
		t.Fatalf("unexpected error: %s", errMsg3)
	}
	if !strings.Contains(result3, `"pipe"`) {
		t.Errorf("expected pipe metadata in count output, got %s", result3)
	}
}

func TestPipeMetadataAbsentWhenNoPipes(t *testing.T) {
	input := `{"count":5}`
	result, errMsg := ApplyPipes(input, nil, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if strings.Contains(result, `"pipe"`) {
		t.Errorf("no pipe metadata expected when no modifiers, got %s", result)
	}
}

func TestPipeMetadataTableSkipped(t *testing.T) {
	input := `[{"name":"a","value":1},{"name":"b","value":2}]`
	meta := map[string]any{"first": 2}
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeTable}}, meta, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	// Table output should NOT contain "pipe" as a column header or data.
	if strings.Contains(result, "pipe") {
		t.Errorf("table renderer should skip pipe key, got:\n%s", result)
	}
}

func TestPipeMetadataIdentityPath(t *testing.T) {
	ResetPipeFiltersForTest()
	t.Cleanup(ResetPipeFiltersForTest)
	RegisterPipeFilters([]string{"show routes"},
		PipeFilter{Name: "count", Description: "count"},
	)

	_, format, errMsg := ProcessPipesChecked("show routes | count")
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	result := format(`{"count":42}`)
	if !strings.Contains(result, `"pipe"`) {
		t.Errorf("identity path should inject pipe metadata, got %s", result)
	}
	if !strings.Contains(result, `"count":true`) {
		t.Errorf("expected count:true in pipe metadata, got %s", result)
	}
}

func TestApplyFirstSingleKeyWrapper(t *testing.T) {
	input := `{"routes":[{"a":1},{"a":2},{"a":3}]}`
	result, errMsg := ApplyPipes(input, []pipeOp{{kind: pipeFirst, arg: "2"}}, nil, nil)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	trimmed := strings.TrimSpace(result)
	if trimmed != `{"routes":[{"a":1},{"a":2}]}` {
		t.Errorf("got %q, want single-key wrapper with 2 items", trimmed)
	}
}

// TestRawPipeReturnsTheDispatcherJSONUnchanged pins the identity property the
// `| raw` operator exists to provide.
//
// The SSH exec channel is both an operator surface and ze's own RPC transport.
// Since the daemon renders every exec answer in the configured format, a caller
// that parses the answer needs one way to ask for the bytes the dispatcher
// produced. `| json` cannot serve: unwrapSingleKeyArray reshapes a single-key
// wrapper into a bare array, so it is a renderer, not an identity.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9 -- `| raw` answers
// the dispatcher's JSON byte for byte, whatever ze.cli.format says, and injects
// no pipe metadata into it.
// PREVENTS: an in-tree RPC caller unmarshalling a table, which every one of them
// absorbs into a silent graceful fallback.
func TestRawPipeReturnsTheDispatcherJSONUnchanged(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	tests := []struct {
		name     string
		input    string
		response string
		want     string
		wantCmd  string
	}{
		{
			name:     "raw beats the configured default",
			input:    "show version | raw",
			response: `{"version":"1.2.3"}`,
			want:     `{"version":"1.2.3"}`,
			wantCmd:  "show version",
		},
		{
			name: "raw keeps a single-key wrapper that | json would unwrap",
			// This case is why the RPC callers cannot simply ask for | json:
			// buildRuntimeTree unmarshals into a struct with a "commands" field.
			input:    "system command list | raw",
			response: `{"commands":[{"name":"show"}]}`,
			want:     `{"commands":[{"name":"show"}]}`,
			wantCmd:  "system command list",
		},
		{
			name:     "raw passes a non-JSON answer through untouched",
			input:    "show version | raw",
			response: "plain text",
			want:     "plain text",
			wantCmd:  "show version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, format, errMsg := ProcessPipesDefaultFormatChecked(tt.input, "")
			if errMsg != "" {
				t.Fatalf("ProcessPipesDefaultFormatChecked(%q) refused it: %s", tt.input, errMsg)
			}
			if command != tt.wantCmd {
				t.Errorf("command = %q, want %q", command, tt.wantCmd)
			}
			if got := format(tt.response); got != tt.want {
				t.Errorf("format(%q) = %q, want %q", tt.response, got, tt.want)
			}
		})
	}
}

// TestRawPipeSuppressesPipeMetadata shows both sides of the metadata rule.
//
// A data-shaping operator normally records itself under a "pipe" key inside the
// answer, so a renderer can say what was filtered. That key is display
// information, and a caller that unmarshals the answer never asked for it.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9 -- `| raw` returns
// the payload with no injected key, while the same chain without it keeps one.
// PREVENTS: raw quietly ceasing to be an identity the day a caller combines it
// with a filter.
func TestRawPipeSuppressesPipeMetadata(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	const response = `{"peers":{"a":{"state":"up"},"b":{"state":"down"}}}`

	_, rawFormat, errMsg := ProcessPipesDefaultFormatChecked("show bgp peer list | count | raw", "")
	if errMsg != "" {
		t.Fatalf("the raw chain was refused: %s", errMsg)
	}
	const wantCount = "{\"count\":2}\n" // applyCount terminates its own answer.
	if got := rawFormat(response); got != wantCount {
		t.Errorf("raw answer = %q, want %q", got, wantCount)
	}

	_, jsonFormat, errMsg := ProcessPipesDefaultFormatChecked("show bgp peer list | count | json compact", "")
	if errMsg != "" {
		t.Fatalf("the json chain was refused: %s", errMsg)
	}
	if got := jsonFormat(response); !strings.Contains(got, `"pipe"`) {
		t.Errorf("the comparison arm lost its pipe metadata, so this test proves nothing: %q", got)
	}
}

// TestRawPipeIsRefusedBesideAnotherFormat proves `raw` joined the mutually
// exclusive format set rather than becoming a silent extra operator.
//
// VALIDATES: `| raw | json` is refused by the validator, not run.
// PREVENTS: a chain naming two answers, where the winner depends on order.
func TestRawPipeIsRefusedBesideAnotherFormat(t *testing.T) {
	_, ops := ParsePipe("show version | raw | json")
	if msg := ValidatePipes(ops); msg == "" {
		t.Fatal("ValidatePipes accepted two format operators")
	} else if !strings.Contains(msg, "raw") {
		t.Errorf("the refusal does not name raw: %q", msg)
	}
}

// TestRawPipeSurvivesFilterFolding covers the one path where a new operator can
// disappear without a compile error.
//
// foldFilters splits a chain into what the command executes server-side and what
// stays a client-side operator. Its switch is exhaustive-exempt, so an operator
// it does not name falls out of both lists. The chain then names no format, the
// configured default is appended, and an RPC caller quietly gets a rendering.
//
// VALIDATES: spec-fixit-cli-format-default-everywhere AC-9 -- `| raw` survives a
// command that owns pipe filters of its own.
// PREVENTS: raw working for every command except the ones with registered
// filters, which are the route-heavy ones a script is most likely to parse.
func TestRawPipeSurvivesFilterFolding(t *testing.T) {
	t.Setenv("ze.cli.format", "table")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	RegisterPipeFilters([]string{"show routes"}, PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true})

	command, format, errMsg := ProcessPipesDefaultFormatChecked("show routes | peer 192.0.2.1 | raw", "")
	if errMsg != "" {
		t.Fatalf("the chain was refused: %s", errMsg)
	}
	if command != "show routes peer 192.0.2.1" {
		t.Errorf("command = %q, want the filter folded into it", command)
	}
	const response = `{"routes":[{"prefix":"192.0.2.0/24"}]}`
	if got := format(response); got != response {
		t.Errorf("format(%q) = %q, want it unchanged", response, got)
	}
}

// VALIDATES: `| json` and `| ndjson` are byte-identical whatever a command
// declares (AC-5).
// PREVENTS: the column order reaching the payload a program parses. encoding/json
// sorts map keys with no override, so an ordered JSON would mean hand-rolled
// marshaling for a reader that cannot use it.
func TestApplyJSONIgnoresColumnOrder(t *testing.T) {
	ResetColumnsForTest()
	t.Cleanup(ResetColumnsForTest)

	payload := `{"peers":[{"address":"192.0.2.1","description":"transit","state":"established","uptime":"1h0m0s"}]}`

	for _, input := range []string{"show test peers | json", "show test peers | ndjson", "show test peers | raw"} {
		ResetColumnsForTest()
		_, before, errMsg := ProcessPipesChecked(input)
		if errMsg != "" {
			t.Fatalf("ProcessPipesChecked(%q): %s", input, errMsg)
		}
		undeclared := before(payload)

		RegisterColumns([]string{"show test peers"}, ColumnOrder{"state", "address"})
		_, after, errMsg := ProcessPipesChecked(input)
		if errMsg != "" {
			t.Fatalf("ProcessPipesChecked(%q): %s", input, errMsg)
		}
		if declared := after(payload); declared != undeclared {
			t.Errorf("%q changed under a declared column order:\ngot  %q\nwant %q", input, declared, undeclared)
		}
		if !strings.Contains(undeclared, `"address"`) {
			t.Errorf("%q did not answer JSON: %q", input, undeclared)
		}
	}
	requireTextOrderingIsLive(t, payload)
}

// requireTextOrderingIsLive fails when the registration the caller made leaves
// `| text` unchanged. An exclusion test asserts that a format did NOT move, and
// that assertion passes just as well when the whole feature is inert. This is
// what tells the two apart.
func requireTextOrderingIsLive(t *testing.T, payload string) {
	t.Helper()

	ResetColumnsForTest()
	_, plain, errMsg := ProcessPipesChecked("show test peers | text")
	if errMsg != "" {
		t.Fatalf("ProcessPipesChecked: %s", errMsg)
	}
	alphabetical := plain(payload)

	RegisterColumns([]string{"show test peers"}, ColumnOrder{"state", "address"})
	_, ordered, errMsg := ProcessPipesChecked("show test peers | text")
	if errMsg != "" {
		t.Fatalf("ProcessPipesChecked: %s", errMsg)
	}
	if ordered(payload) == alphabetical {
		t.Fatal("the declared order changed nothing in | text, so this exclusion test proves nothing")
	}
}

// VALIDATES: AC-8, R-3. An alias used on a command that owns pipe filters
// expands and applies. `show bgp rib` is that command, and its real filter set
// is what this test registers.
// PREVENTS: the expansion vanishing inside foldFilters. That switch splits a
// chain into the ops folded into server arguments and the ops the client still
// runs. A kind it named nowhere used to reach neither side, and nothing
// reported the loss. The assertion is that the answer CHANGED.
func TestAliasSurvivesFoldFiltersOnFilteredCommand(t *testing.T) {
	resetAliasTables(t)

	RegisterPipeFilters([]string{"show bgp rib"},
		PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
		PipeFilter{Name: "prefix", Description: "Filter by prefix", TakesArg: true},
		PipeFilter{Name: "count", Description: "Count matching routes"},
		PipeFilter{Name: "first", Description: "Take first N routes", TakesArg: true},
	)
	RegisterAliases([]string{"show bgp rib"}, Alias{Name: "prefixes", Expansion: "display prefix"})

	const payload = `{"routes":[{"prefix":"10.10.1.0/24","aspath":"64501 64502","origin":"igp"}]}`

	command, format, errMsg := ProcessPipesChecked("show bgp rib | peer 192.0.2.1 | prefixes | json")
	if errMsg != "" {
		t.Fatalf("the chain was refused: %s", errMsg)
	}
	if command != "show bgp rib peer 192.0.2.1" {
		t.Errorf("command = %q, want the command's own filter folded into it", command)
	}

	got := format(payload)
	if !strings.Contains(got, "10.10.1.0/24") {
		t.Errorf("the alias dropped the field it displayed: %s", got)
	}
	for _, dropped := range []string{"aspath", "origin"} {
		if strings.Contains(got, dropped) {
			t.Errorf("%q survived the alias, so the expansion never reached the client: %s", dropped, got)
		}
	}
}

// VALIDATES: a validation error produced while the chain is folded is reported
// on a command that owns filters, exactly as it is on a command that owns none.
// PREVENTS: the silent drop returning through the other door. An alias given an
// argument becomes a pipeInvalid op, and the classification switch names that
// kind nowhere.
func TestFoldFiltersKeepsAnInvalidOpOnAFilteredCommand(t *testing.T) {
	resetAliasTables(t)

	RegisterPipeFilters([]string{"show bgp rib"},
		PipeFilter{Name: "peer", Description: "Filter by peer", TakesArg: true},
	)
	RegisterAliases([]string{"show bgp rib"}, Alias{Name: "prefixes", Expansion: "display prefix"})

	_, _, errMsg := ProcessPipesChecked("show bgp rib | peer 192.0.2.1 | prefixes wide")
	if errMsg == "" {
		t.Fatal("the refusal was dropped by the fold, so the operator sees an unfiltered answer")
	}
	if !strings.Contains(errMsg, "prefixes") {
		t.Errorf("the refusal does not name the alias: %s", errMsg)
	}
}

// TestFirstNStopsTheGenerator checks that `| first 10` bounds the walk that
// produces the records, not just the records the operator sees. The method: a
// generator counts the rows it produces, the chain takes ten, and both counts
// are read.
//
// VALIDATES: AC-14 / R-3 -- the generator stops after ten rows and the
// remaining rows are never produced.
// PREVENTS: `show bgp rib | first 10` walking a whole RIB to throw all but ten
// rows away, which is the cost this protocol exists to remove.
func TestFirstNStopsTheGenerator(t *testing.T) {
	const (
		available = 1000
		wanted    = 10
	)

	produced := 0
	rows := func(yield func(rpc.Record) bool) {
		for i := range available {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(textbuf.StrIntStr(`{"row":`, int64(i), `}`))}) {
				return
			}
		}
	}

	kept := 0
	for range ApplyPipesRecords("show bgp rib | first 10", rows) {
		kept++
	}

	if kept != wanted {
		t.Errorf("chain kept %d records, want %d", kept, wanted)
	}
	if produced != wanted {
		t.Errorf("generator produced %d rows, want %d: a consumer that stops must stop the walk", produced, wanted)
	}
}

// The size of the answer the two memory tests below walk. 4000 records of 8 KiB
// is 32 MiB, and an operator that collects them holds all of it at the last
// record. The budget is an eighth of that: three orders of magnitude below what
// collecting costs, and far above the noise of one measurement.
const (
	memoryWalkRecords = 4000
	memoryWalkPayload = 8192
	memoryWalkTotal   = memoryWalkRecords * memoryWalkPayload
	memoryWalkBudget  = memoryWalkTotal / 8
)

// recordWalk is the generator half of a memory measurement. It counts the
// records it produced and reads the live heap at the moment it produced the
// last one, which is the moment an operator that collects records is holding
// every one of them.
type recordWalk struct {
	produced int
	baseline uint64
	atLast   uint64
}

// records answers a generator of count records, each carrying payload bytes of
// its own. Each record is a fresh allocation, so a consumer that keeps one
// keeps its bytes and this measurement can see it.
func (w *recordWalk) records(count, payload int) iter.Seq[rpc.Record] {
	return func(yield func(rpc.Record) bool) {
		for i := range count {
			if i == 0 {
				w.baseline = liveHeapBytes()
			}
			var b textbuf.Buffer
			b.Str(`{"row":`).Int(int64(i)).Str(`,"pad":"`).Repeat("x", payload).Str(`"}`)
			w.produced++
			if i == count-1 {
				w.atLast = liveHeapBytes()
			}
			if !yield(rpc.Record{Item: json.RawMessage(b.String())}) {
				return
			}
		}
	}
}

// heldAtLastRecord answers the bytes the consumer was still holding when the
// walk produced its last record. A negative delta reads as zero: the heap can
// end smaller than it started, and that is not a consumer holding records.
func (w *recordWalk) heldAtLastRecord() uint64 {
	if w.atLast < w.baseline {
		return 0
	}
	return w.atLast - w.baseline
}

// liveHeapBytes answers the live heap after a full collection, so what it
// reports is what something still references rather than what has been
// allocated since.
func liveHeapBytes() uint64 {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

// collectRecords drains a record sequence into a slice, which is what a test
// asserts over.
func collectRecords(records iter.Seq[rpc.Record]) []rpc.Record {
	var collected []rpc.Record
	for record := range records {
		collected = append(collected, record)
	}
	return collected
}

// TestCountConsumesWithoutBuffering checks that `| count` answers the number of
// records without holding them. The method: a generator produces 32 MiB of
// records and reads the live heap at the last one. A count that collected the
// records is holding all 32 MiB at that moment, and a count that reads one
// record and forgets it is holding none of it.
//
// VALIDATES: O(1) memory for `| count`, whatever the answer's size.
// PREVENTS: a record path that counts by collecting. It answers the same
// number, so an assertion over the answer alone would pass over it, and the
// memory this protocol exists to save would still be paid.
func TestCountConsumesWithoutBuffering(t *testing.T) {
	var walk recordWalk
	kept := collectRecords(ApplyPipesRecords("show test rows | count", walk.records(memoryWalkRecords, memoryWalkPayload)))

	if len(kept) != 1 {
		t.Fatalf("| count answered %d records, want 1", len(kept))
	}
	if want := `{"count":4000}`; string(kept[0].Item) != want {
		t.Errorf("| count answered %q, want %q", kept[0].Item, want)
	}
	if walk.produced != memoryWalkRecords {
		t.Errorf("generator produced %d records, want %d: a count reads every one", walk.produced, memoryWalkRecords)
	}
	if held := walk.heldAtLastRecord(); held > memoryWalkBudget {
		t.Errorf("| count held %d bytes at the last record, want under %d of the %d walked: the records are being collected rather than counted",
			held, uint64(memoryWalkBudget), uint64(memoryWalkTotal))
	}
}

// TestLastNKeepsRingBufferOnly checks that `| last N` holds N records and not
// the answer. The method is TestCountConsumesWithoutBuffering's: 32 MiB walked,
// the live heap read at the last record, and eight records asked for.
//
// VALIDATES: O(N) memory for `| last N`, with N the operator's argument rather
// than the answer's size.
// PREVENTS: an implementation that collects every record and slices the tail.
// It answers the same eight rows, so only the memory tells the two apart.
func TestLastNKeepsRingBufferOnly(t *testing.T) {
	const wanted = 8

	var walk recordWalk
	kept := collectRecords(ApplyPipesRecords("show test rows | last 8", walk.records(memoryWalkRecords, memoryWalkPayload)))

	if len(kept) != wanted {
		t.Fatalf("| last 8 answered %d records, want %d", len(kept), wanted)
	}
	for i, record := range kept {
		want := textbuf.StrIntStr(`{"row":`, int64(memoryWalkRecords-wanted+i), `,`)
		if !strings.HasPrefix(string(record.Item), want) {
			t.Errorf("record %d = %.20q..., want the record starting %q", i, record.Item, want)
		}
	}
	if walk.produced != memoryWalkRecords {
		t.Errorf("generator produced %d records, want %d: the last N are known only at the end", walk.produced, memoryWalkRecords)
	}
	if held := walk.heldAtLastRecord(); held > memoryWalkBudget {
		t.Errorf("| last 8 held %d bytes at the last record, want under %d of the %d walked: the ring is holding more than the 8 records it was asked for",
			held, uint64(memoryWalkBudget), uint64(memoryWalkTotal))
	}
}

// TestRecordChainMatchesAndRefuses checks the two answers a record chain gives
// that are not a transform of the rows: `| match` keeps the records that carry
// the pattern, and a chain ValidatePipes refuses answers one fault and pulls
// nothing.
//
// VALIDATES: `| match` reads one record at a time, and an unreadable chain
// fails closed.
// PREVENTS: a chain nobody could validate reaching an operator as an empty
// answer, which reads exactly like a command that found no data.
func TestRecordChainMatchesAndRefuses(t *testing.T) {
	rows := []string{
		`{"address":"192.0.2.1","state":"established"}`,
		`{"address":"192.0.2.2","state":"idle"}`,
		`{"address":"192.0.2.3","state":"established"}`,
	}
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		for _, row := range rows {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(row)}) {
				return
			}
		}
	}

	matched := collectRecords(ApplyPipesRecords("show test rows | match established", records))
	if len(matched) != 2 {
		t.Fatalf("| match kept %d records, want 2", len(matched))
	}
	for i, want := range []string{rows[0], rows[2]} {
		if string(matched[i].Item) != want {
			t.Errorf("record %d = %q, want %q", i, matched[i].Item, want)
		}
	}

	produced = 0
	refused := collectRecords(ApplyPipesRecords("show test rows | first zero", records))
	if len(refused) != 1 {
		t.Fatalf("a refused chain answered %d records, want one fault", len(refused))
	}
	if len(refused[0].Item) > 0 {
		t.Errorf("a refused chain answered a result %q, want a fault", refused[0].Item)
	}
	if want := `{"message":"first requires a positive number"}`; string(refused[0].Fault) != want {
		t.Errorf("the fault reads %q, want %q", refused[0].Fault, want)
	}
	if produced != 0 {
		t.Errorf("a refused chain walked %d records, want none", produced)
	}
}

// TestRecordChainStopsThroughEveryStage checks that a chain of two operators
// stops the walk as one of them does. The method: `| match` in front of
// `| first 1`, over rows whose first one matches, so the answer is complete
// after one row and every stage has to agree to stop.
//
// VALIDATES: R-3 through a chain rather than through one operator.
// PREVENTS: a stage that keeps pulling after the stage behind it stopped, which
// leaves the generator walking for a consumer that has gone.
func TestRecordChainStopsThroughEveryStage(t *testing.T) {
	rows := []string{
		`{"address":"192.0.2.1","state":"established"}`,
		`{"address":"192.0.2.2","state":"established"}`,
		`{"address":"192.0.2.3","state":"established"}`,
	}
	produced := 0
	records := func(yield func(rpc.Record) bool) {
		for _, row := range rows {
			produced++
			if !yield(rpc.Record{Item: json.RawMessage(row)}) {
				return
			}
		}
	}

	kept := collectRecords(ApplyPipesRecords("show test rows | match established | first 1", records))
	if len(kept) != 1 {
		t.Fatalf("the chain kept %d records, want 1", len(kept))
	}
	if string(kept[0].Item) != rows[0] {
		t.Errorf("the chain kept %q, want %q", kept[0].Item, rows[0])
	}
	if produced != 1 {
		t.Errorf("the generator produced %d rows, want 1: a stop must reach it through every stage", produced)
	}
}

// VALIDATES: AC-14 over a DECLARED name. A word after a pipe alias a caller
// outside this repository registered is refused, and the refusal names the
// alias and says it takes no argument.
// PREVENTS: the word being dropped in silence. expandAliases reads no owner, so
// TestAliasTakesNoArgument proves the mechanism for an in-tree name and nothing
// asserts it over a declared one. A declared alias is the name an operator
// meets on a plugin command, and a dropped word there answers the whole table
// to somebody who typed a filter.
func TestPipeAliasArgumentRefused(t *testing.T) {
	const declaring = "show probe counters"

	resetAliasTables(t)

	if err := RegisterPluginAliases("declaring-owner", []string{declaring}, []PluginAlias{{
		Command: declaring,
		Alias: Alias{
			Name: "totals", Description: "The counters alone", Expansion: "display vrp-count",
		},
	}}); err != nil {
		t.Fatalf("the declaration was refused: %v", err)
	}

	// The name resolves with no word after it, so the refusal below measures
	// the ARGUMENT rather than an alias that never registered.
	payload := `{"vrp-count":7,"servers":[{"address":"192.0.2.101"}]}`
	got := renderThroughPipes(t, declaring+" | totals | json", payload)
	if !strings.Contains(got, "vrp-count") || strings.Contains(got, "192.0.2.101") {
		t.Fatalf("the declared alias does not answer, so the refusal below proves nothing: %s", got)
	}

	_, _, errMsg := ProcessPipesChecked(declaring + " | totals established")
	if errMsg == "" {
		t.Fatal("a word after a declared alias was accepted, so the word went nowhere")
	}
	requireMentions(t, errMsg, "totals", "argument")
}

// TestRefusalIsDistinguishableFromData proves a caller can tell a refusal from
// an answer. An apply-time refusal arrives as the formatted string, so without
// this prefix a script would parse the refusal as data and exit 0.
func TestRefusalIsDistinguishableFromData(t *testing.T) {
	_, format, errMsg := ProcessPipesChecked("show version | count")
	if errMsg != "" {
		t.Fatalf("the chain was refused at validation, not at apply: %s", errMsg)
	}
	refusal := format(`{"version":"ze dev","built":"unknown"}`)
	if !IsPipeError(refusal) {
		t.Errorf("a refusal is not recognizable as one: %q", refusal)
	}

	answer := format(`{"peers":[{"a":1},{"a":2}]}`)
	if IsPipeError(answer) {
		t.Errorf("an answer reads as a refusal: %q", answer)
	}
}

// TestApplyTakeKeepsIdentityKeys proves an identity-keyed answer stays keyed.
// `show bgp peer list` answers a map keyed by peer address, so writing an array
// back over it would answer the taken peers with their addresses gone.
func TestApplyTakeKeepsIdentityKeys(t *testing.T) {
	const answer = `{"peers":{"192.0.2.1":{"state":"up"},"192.0.2.2":{"state":"idle"},"192.0.2.3":{"state":"up"}}}`

	got, msg := applyFirst(answer, "2")
	if msg != "" {
		t.Fatalf("first refused: %s", msg)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("answer does not parse: %v", err)
	}
	peers, isMap := decoded["peers"].(map[string]any)
	if !isMap {
		t.Fatalf("peers is no longer keyed by identity: %s", got)
	}
	if len(peers) != 2 {
		t.Errorf("first 2 kept %d peers, want 2: %s", len(peers), got)
	}
	// Sorted order, so the two kept are the first two addresses.
	if _, ok := peers["192.0.2.1"]; !ok {
		t.Errorf("first 2 dropped the first peer: %s", got)
	}
	if _, ok := peers["192.0.2.3"]; ok {
		t.Errorf("first 2 kept the third peer: %s", got)
	}

	got, msg = applyLast(answer, "1")
	if msg != "" {
		t.Fatalf("last refused: %s", msg)
	}
	if !strings.Contains(got, "192.0.2.3") {
		t.Errorf("last 1 kept the wrong peer: %s", got)
	}
}
