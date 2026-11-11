// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: shellparse.go -- the one parse of a call and its panic containment
// Related: session.go -- commandOutsideReaders, which refuses a line that does not parse
package hookruntime

import (
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

// withBashParser puts parser in the place of the bash parser for one test and
// restores the real one after it. No test of this package runs in parallel,
// so the swap is seen by this test alone.
func withBashParser(t *testing.T, parser func(string) (*syntax.File, error)) {
	t.Helper()
	saved := parseBashLine
	parseBashLine = parser
	t.Cleanup(func() { parseBashLine = saved })
}

// runMainBash sends one Bash command through the pretool-bash entry as the
// main thread, which carries no agent id.
func runMainBash(t *testing.T, command string) (code int, message string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	code, _, message = runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
	return code, message
}

// VALIDATES: AC-3, the part phase 2 owns. The context holds one parse for the
// whole hook call: the first ask parses, a copy of the context (each check
// receives one) reads the same result, and a whole Run of a subagent reader
// line parses its command exactly once.
// PREVENTS: each check parsing the line again, so two checks could judge two
// different readings of one command.
func TestCommandIsParsedOncePerHookCall(t *testing.T) {
	calls := 0
	withBashParser(t, func(line string) (*syntax.File, error) {
		calls++
		return parseBash(line)
	})

	ctx, ok := newContext(Payload{ToolName: toolBash, ToolInput: map[string]any{"command": "cat a | head -1"}}, t.TempDir())
	if !ok {
		t.Fatal("newContext refused a payload with a root")
	}
	copied := ctx
	first := ctx.commandParse()
	second := copied.commandParse()
	if calls != 1 {
		t.Fatalf("two asks on one call's context parsed %d times, want 1", calls)
	}
	if !first.parsed() {
		t.Fatalf("`cat a | head -1` did not parse: %s", first.failure)
	}
	if second.file != first.file {
		t.Errorf("a copy of the context read another tree than the first ask parsed")
	}
	if len(first.file.Stmts) != 1 {
		t.Errorf("`cat a | head -1` parsed into %d statements, want one pipeline", len(first.file.Stmts))
	}

	calls = 0
	code, _, message := runSubagentBash(t, "cat notes.md")
	if code != 0 {
		t.Fatalf("subagent reader line: code = %d, want 0; message: %s", code, message)
	}
	if calls != 1 {
		t.Errorf("one hook call parsed its command %d times, want 1", calls)
	}
}

// VALIDATES: AC-12, the part phase 2 owns. A line that does not parse as bash
// (an unclosed quote, an unterminated here-document, a stray `fi`, an
// operator with no command after it) is no reader line: a subagent is refused
// with the identity line to type, and the main thread's text verdicts stand.
// The two rows the splitter alone read as readers are refused for the parse,
// and say so.
// PREVENTS: a line bash refuses being judged by what the splitter guessed it
// runs, so `cat a |` passed as a reader line.
func TestUnparsedLineKeepsTheTextVerdictAndNeedsTheIdentityLine(t *testing.T) {
	for _, row := range []struct {
		command string
		// parseNamed marks a row only the parse puts outside the readers, so
		// its refusal names the parse failure.
		parseNamed bool
	}{
		{`grep -n 'x notes.md`, false},
		{"cat <<EOF\nx", false},
		{`cat notes.md; fi`, false},
		{`cat notes.md |`, true},
		{`cat notes.md &&`, true},
	} {
		if parse := parseCommandLine(row.command); parse.parsed() {
			t.Errorf("%q parsed as bash; the row needs a line bash refuses", row.command)
			continue
		}
		code, output, message := runSubagentBash(t, row.command)
		if code != 2 {
			t.Errorf("%q: code = %d, want 2: a line the hook cannot read is no reader line", row.command, code)
		}
		if output != "" {
			t.Errorf("%q: standard output = %q, want none", row.command, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: the refusal does not print the identity line to type: %s", row.command, message)
		}
		if row.parseNamed && !strings.Contains(message, "does not parse as bash") {
			t.Errorf("%q: the refusal does not name the parse failure: %s", row.command, message)
		}
	}
	for _, command := range []string{
		`go test ./internal/x 'oops`,
		`rm -f scratch 'oops`,
	} {
		code, message := runMainBash(t, command)
		if code != 2 {
			t.Errorf("main %q: code = %d, want 2: the text verdict stands on a line that does not parse; message: %s", command, code, message)
		}
	}
}

// VALIDATES: AC-13. A parser that panics answers "did not parse": the panic
// is contained in the parse, a subagent's reader line is then refused with the
// identity line as in AC-12, a main-thread text verdict still refuses, and the
// hook reports no failure of its own.
// PREVENTS: a panic in third-party code reaching Run's recover, which answers
// exit 0, so a line the hook could not read would pass every check.
func TestParserPanicIsAnUnparsedLine(t *testing.T) {
	withBashParser(t, func(string) (*syntax.File, error) {
		panic("BUG: parser panic forced by the test")
	})

	parse := parseCommandLine("cat notes.md")
	if parse.parsed() {
		t.Fatal("a parser that panicked answered a statement tree")
	}
	if !strings.Contains(parse.failure, "parser panic forced by the test") {
		t.Errorf("the failure does not carry the panic: %q", parse.failure)
	}

	code, output, message := runSubagentBash(t, "cat notes.md")
	if code != 2 {
		t.Fatalf("subagent reader line under a panicking parser: code = %d, want 2; message: %s", code, message)
	}
	if output != "" {
		t.Errorf("standard output = %q, want none", output)
	}
	if !strings.Contains(message, "does not parse as bash") {
		t.Errorf("the refusal does not say the line did not parse: %s", message)
	}
	if !strings.Contains(message, identityLineOfAgent) {
		t.Errorf("the refusal does not print the identity line to type: %s", message)
	}
	if strings.Contains(message, "internal error") {
		t.Errorf("the panic reached Run's recover: %s", message)
	}

	code, message = runMainBash(t, "cat notes.md")
	if code != 0 {
		t.Errorf("main reader line under a panicking parser: code = %d, want 0; message: %s", code, message)
	}
	if strings.Contains(message, "internal error") {
		t.Errorf("the hook reported a failure of its own: %s", message)
	}
	code, message = runMainBash(t, "go test ./internal/x")
	if code != 2 {
		t.Errorf("main raw go test under a panicking parser: code = %d, want 2; message: %s", code, message)
	}
	if strings.Contains(message, "internal error") {
		t.Errorf("the hook reported a failure of its own: %s", message)
	}
}
