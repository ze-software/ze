// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: shellparse.go -- the one parse of a call and its panic containment
// Related: session.go -- commandOutsideReaders, which refuses a line that does not parse
// Related: verdicts_test.go -- the recorded corpus the bash comparisons read
package hookruntime

import (
	stdcontext "context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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
		// The parser drops this carriage return and bash keeps it, so the
		// tree is not bash's reading; the byte allow-list refuses it first.
		{"cat notes.md\r\ncat x", false},
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

// wordReadingLines are the rows the word-value reading owes beyond the
// recorded corpus: rows for each of its rules, and one for each mark of an
// expansion. The bash run tells a missed mark apart only because its working
// directory holds files a pattern matches (bashWorkFiles), and because HOME is
// that directory, so a missed mark gives a value bash does not pass.
//
//nolint:gochecknoglobals // the comparison's own rows
var wordReadingLines = []string{
	// A backslash outside quotes drops and keeps the next character, and a
	// backslash-newline joins the lines.
	`grep -n a\\b f`,
	`grep -n \a\b f`,
	`grep -n a\'b f`,
	"grep -n a\\\nb f",
	// Inside double quotes a backslash drops only before $, a backtick, ", \
	// and a newline.
	`grep -n "a\b" f`,
	`grep -n "a\\b" f`,
	`grep -n "a\$b" f`,
	`grep -n "a\"b" f`,
	"grep -n \"a\\`b\" f",
	"grep -n \"a\\\nb\" f",
	// A single-quoted part is taken whole, a backslash-newline included.
	`grep -n 'a\b' f`,
	"grep -n 'a\\\nb' f",
	`grep -n a"b"'c'd f`,
	// A $ the parser leaves in a literal is that character.
	`grep -n "foo$" f`,
	`grep -n foo$ f`,
	`grep -n $ f`,
	// An empty quoted word is an argument.
	`grep -n "" f ''`,
	// UTF-8, a # inside a word or quotes, a brace and a bracket bash leaves
	// alone, and a tilde outside a prefix.
	`grep -n 'é' café`,
	`grep -n "#" a#b '#'`,
	`grep -n a{b} '{x,y}' [`,
	`grep -n --x=~ x:~`,
	// A here-document is data; the words around it are arguments.
	"grep -n x <<EOF\na $HOME b\nEOF",
	"grep -n x <<'EOF'\n$HOME\nEOF",
	// ANSI-C quoting: the value is not read (TestWordValuesMatchBash checks the
	// answer).
	`grep -n $'x\ty' f`,
	// Each mark of an expansion, on a line that holds nothing else.
	`grep -n x *`,
	`grep -n x ?`,
	`grep -n x [ab]`,
	`grep -n x a{b,c}`,
	`grep -n x {1..2}`,
	`grep -n x ~`,
	`grep -n x y=~`,
	`grep -n x a=b:~`,
	`grep -n x $HOME`,
	`grep -n x "$HOME"`,
}

// bashWorkFiles are the files of every bash run's working directory: the
// names the corpus reads from, which also give a glob something to match.
//
//nolint:gochecknoglobals // the comparison's fixture
var bashWorkFiles = []string{"a", "b", "f", "x", "list", "notes.md"}

// bashUnroutedWords are command words bash does not hand to a function of
// that name, or whose function would change what the line does around it, so
// a line holding one is not run.
//
//nolint:gochecknoglobals // the comparison's fixture
var bashUnroutedWords = []string{
	"command", "builtin", "exec", "eval", "source", ".", "enable", "unset",
	"export", "declare", "local", "readonly", "typeset", "coproc", "let",
	"trap", "set", "shopt", "alias", "unalias",
	"break", "continue", "return", "exit", "wait", "command_not_found_handle",
}

const (
	// bashCallsMax bounds the stub calls of one shell, so a loop whose
	// condition is a stub that always succeeds (`while true`) ends.
	bashCallsMax = 200
	// bashLineTimeout bounds one bash run.
	bashLineTimeout = 10 * time.Second
)

// bashCorpusLines answers the lines the bash comparisons and the fuzz seeds
// read, each once: the line of every row of the recorded verdict table, the
// spec's construct rows, and wordReadingLines.
func bashCorpusLines(tb testing.TB) []string {
	tb.Helper()
	rows, err := readVerdictTable(verdictsTablePath)
	if err != nil {
		tb.Fatalf("read the verdict table: %v", err)
	}
	seen := make(map[string]bool, len(rows))
	lines := make([]string, 0, len(rows))
	add := func(line string) {
		if seen[line] {
			return
		}
		seen[line] = true
		lines = append(lines, line)
	}
	for _, row := range rows {
		add(row.Line)
	}
	for _, line := range specConstructLines {
		add(line)
	}
	for _, line := range wordReadingLines {
		add(line)
	}
	return lines
}

// bashLine is what the comparisons read from one parsed line before bash
// runs it.
type bashLine struct {
	// calls holds the words of every simple command the tree names, in the
	// order of the tree, and names the command word of each, read.
	calls [][]*syntax.Word
	names []string
	// straight is set when bash runs every command of the tree exactly once:
	// no loop, conditional, `&&`, `||`, test, arithmetic, expansion operator,
	// process substitution (bash does not wait for it) or input from outside
	// the working directory (a missing file stops its command).
	straight bool
	// skip names why the line is not run, and is empty when it is.
	skip string
	// inputs and outputs are the relative redirection targets, which the run
	// creates in its working directory so each redirection succeeds.
	inputs  []string
	outputs []string
}

// analyseBashLine reads a parsed line into the commands the comparisons
// expect bash to run.
func analyseBashLine(file *syntax.File) bashLine {
	line := bashLine{straight: true}
	refuse := func(reason string) {
		if line.skip == "" {
			line.skip = reason
		}
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch node := node.(type) {
		case *syntax.CallExpr:
			line.call(node, refuse)
		case *syntax.Redirect:
			line.redirect(node, refuse)
		case *syntax.DeclClause:
			refuse("a declaration")
		case *syntax.FuncDecl:
			refuse("a function definition")
		case *syntax.CoprocClause:
			refuse("a coprocess")
		case *syntax.LetClause:
			refuse("let")
		case *syntax.ExtGlob:
			// The Bash tool runs bash with extglob off (A-3), so bash refuses
			// the line the parser reads; judgeReaderLine puts it outside.
			refuse("an extended glob, which bash refuses with extglob off")
		case *syntax.IfClause, *syntax.WhileClause, *syntax.ForClause, *syntax.CaseClause,
			*syntax.ProcSubst, *syntax.TestClause, *syntax.ArithmCmd, *syntax.ArithmExp:
			line.straight = false
		case *syntax.BinaryCmd:
			if node.Op == syntax.AndStmt || node.Op == syntax.OrStmt {
				line.straight = false
			}
		case *syntax.ParamExp:
			// An indirection names its target by a variable's value, so
			// bash refuses it when that variable is unset, as it is here;
			// judgeReaderLine puts it outside.
			if node.Excl {
				refuse("an indirect expansion, which bash refuses for an unset name")
			}
			// A default, an alternative or a replacement expands its word
			// only when the parameter's value calls for it.
			if node.Exp != nil {
				line.straight = false
			}
			if node.Repl != nil {
				line.straight = false
			}
		}
		return true
	})
	return line
}

// call reads one simple command's command word, and refuses the line when
// bash would not route that word to a stub function.
func (line *bashLine) call(call *syntax.CallExpr, refuse func(string)) {
	if len(call.Args) == 0 {
		return
	}
	word := call.Args[0]
	name, err := readWord(word)
	if err != nil {
		refuse("a command word whose value is not read")
		return
	}
	if name.expands {
		refuse("a command word the shell expands")
		return
	}
	if first, ok := word.Parts[0].(*syntax.Lit); ok && strings.HasPrefix(first.Value, `\`) {
		refuse("a command word with a leading backslash")
		return
	}
	if slices.Contains(bashUnroutedWords, name.text) {
		refuse("a command word bash does not route to a function")
		return
	}
	if strings.Contains(name.text, "/") {
		refuse("a command word with a slash, which bash runs as a path")
		return
	}
	if !stubbableName(name.text) {
		refuse("a command word no stub function is named after")
		return
	}
	line.calls = append(line.calls, call.Args)
	line.names = append(line.names, name.text)
}

// redirect sorts one redirection: a here-document or a descriptor copy
// touches no file, and a file target is read by target.
func (line *bashLine) redirect(redirect *syntax.Redirect, refuse func(string)) {
	if redirect.N != nil && strings.HasPrefix(redirect.N.Value, "{") {
		refuse("a redirection that names its descriptor")
		return
	}
	writes := false
	switch redirect.Op {
	case syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
		return
	case syntax.DplIn, syntax.DplOut:
		target, err := readWord(redirect.Word)
		if err == nil && !target.expands && descriptorWord(target.text) {
			return
		}
		writes = redirect.Op == syntax.DplOut
	case syntax.RdrIn:
		writes = false
	case syntax.RdrOut, syntax.AppOut, syntax.RdrClob, syntax.RdrAll, syntax.AppAll, syntax.RdrInOut:
		writes = true
	case syntax.AppClob, syntax.RdrAllClob, syntax.AppAllClob:
		refuse("a redirection of another shell")
		return
	}
	line.target(redirect.Word, writes, refuse)
}

// target reads one redirection's file. A write may land only inside the
// run's working directory, or on /dev/null.
func (line *bashLine) target(word *syntax.Word, writes bool, refuse func(string)) {
	value, err := readWord(word)
	unread := err != nil
	if !unread && value.expands {
		unread = true
	}
	if unread {
		if writes {
			refuse("a write to a target the shell expands or the hook does not read")
			return
		}
		line.straight = false
		return
	}
	path := value.text
	if path == "/dev/null" {
		return
	}
	if filepath.IsAbs(path) {
		if writes {
			refuse("a write outside the working directory")
			return
		}
		line.straight = false
		return
	}
	if slices.Contains(strings.Split(path, "/"), "..") {
		refuse("a redirection that leaves the working directory")
		return
	}
	if writes {
		line.outputs = append(line.outputs, path)
		return
	}
	line.inputs = append(line.inputs, path)
}

// descriptorWord reports whether a `<&` or `>&` target is a descriptor
// number or `-`, which copies or closes a descriptor and opens no file.
func descriptorWord(text string) bool {
	if text == "-" {
		return true
	}
	_, err := strconv.Atoi(text)
	return err == nil
}

// stubbableName reports whether `function <name> { ...; }` defines a
// function bash then calls for that command word.
func stubbableName(name string) bool {
	if name == "[" {
		return true
	}
	for index, character := range name {
		letter := character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
		digit := character >= '0' && character <= '9'
		if letter || digit {
			continue
		}
		if index > 0 && strings.ContainsRune(".+-", character) {
			continue
		}
		return false
	}
	return name != ""
}

// bashView is what bash did with one line under the stub functions.
type bashView struct {
	// calls holds the argv of each stub call, the command word first, in the
	// order bash recorded them.
	calls [][]string
	// notFound holds each command word bash looked up and did not find: a
	// command the tree does not name, since every name it does is a stub.
	notFound []string
	// capped is set when a shell reached bashCallsMax and the run was ended.
	capped bool
	// syntaxError is set when bash refused part of the line as a syntax
	// error. eval runs the commands before the error, so the run is a prefix.
	syntaxError bool
}

// lookBash answers the bash the comparisons run. Production is Linux, where
// bash is always present, so a missing bash fails the test rather than
// leaving the parser uncompared.
func lookBash(t *testing.T) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is not on PATH (%v): the comparison of the parser with bash needs a bash to compare with", err)
	}
	return bash
}

// runBashLine runs one line through a real bash the way the Bash tool runs a
// command (A-3 of spec-agent-economy-6-shell-parser: `bash -c`, extglob off,
// the line handed to eval with stdin from /dev/null), with no program
// reachable: the environment is empty but for a PATH naming a directory that
// does not exist and a HOME inside the run's directory, and every command word
// the tree names is a shell function that records its argv on descriptor 3
// and runs nothing. Functions shadow builtins too, so `echo`, `cd`, `[`,
// `read` and `true` record rather than act; read and false answer 1, every
// other stub 0. command_not_found_handle records any other command bash
// looks for. reason is empty when the line ran, and otherwise says why not.
func runBashLine(t *testing.T, bash, line string, shape bashLine) (view bashView, reason string) {
	t.Helper()
	if strings.ContainsRune(line, 0) {
		return bashView{}, "a NUL byte, which no argv carries"
	}
	dir := t.TempDir()
	if err := prepareBashDir(dir, shape); err != nil {
		return bashView{}, "a redirection target that clashes with the working files"
	}
	recordPath := filepath.Join(t.TempDir(), "calls")
	record, err := os.OpenFile(recordPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open the call record: %v", err)
	}
	defer record.Close() //nolint:errcheck // the record is read by path below

	ctx, cancel := stdcontext.WithTimeout(t.Context(), bashLineTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-c", bashScript(line, shape.names))
	command.Dir = dir
	command.Env = []string{"PATH=" + filepath.Join(dir, "no-such-directory"), "HOME=" + dir}
	command.ExtraFiles = []*os.File{record}
	// The cap ends a run with `kill -KILL 0`, so bash leads a process group of
	// its own, and a timeout kills that whole group.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = time.Second
	var stderr strings.Builder
	command.Stderr = &stderr
	runErr := command.Run()
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		t.Fatalf("start bash for %q: %v", line, runErr)
	}

	body, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read the call record: %v", err)
	}
	view, ready, err := readBashRecords(string(body))
	if err != nil {
		t.Errorf("%q: the call record does not read: %v", line, err)
		return bashView{}, "an unreadable call record"
	}
	if !ready {
		t.Errorf("%q: bash did not reach the line, so a stub definition failed; stderr: %s", line, stderr.String())
		return bashView{}, "a run that did not reach the line"
	}
	if ctx.Err() != nil && !view.capped {
		t.Errorf("%q: bash ran past %s without reaching the call cap; stderr: %s", line, bashLineTimeout, stderr.String())
		return bashView{}, "a run past the timeout"
	}
	view.syntaxError = strings.Contains(stderr.String(), "syntax error")
	if view.syntaxError {
		t.Logf("%q: bash refused part of the line the parser read; stderr: %s", line, stderr.String())
	}
	return view, ""
}

// prepareBashDir writes the working files and makes every redirection the
// line holds succeed: a directory for each output, a file for each input.
func prepareBashDir(dir string, shape bashLine) error {
	for _, name := range bashWorkFiles {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			return err
		}
	}
	for _, path := range shape.outputs {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, path)), 0o700); err != nil {
			return err
		}
	}
	for _, path := range shape.inputs {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(full, os.O_CREATE|os.O_RDONLY, 0o600)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

// bashScript writes the script runBashLine hands to `bash -c`. Each record
// on descriptor 3 is NUL-separated: `f`, the argument count, the function's
// name and its arguments for a stub call; `n`, the argument count and the
// arguments, the command word first, for a command bash did not find; `x`
// when a shell reached the cap; `R` once the stubs are defined.
func bashScript(line string, names []string) string {
	var script strings.Builder
	script.WriteString("shopt -u extglob\n")
	script.WriteString(`command_not_found_handle() { builtin printf '%s\0' n "$#" "$@" >&3; return 127; }` + "\n")
	defined := map[string]bool{}
	for _, name := range names {
		if defined[name] {
			continue
		}
		defined[name] = true
		status := "0"
		if name == "read" || name == "false" {
			status = "1"
		}
		script.WriteString("function " + name + " { (( ++__ze_calls > " + strconv.Itoa(bashCallsMax) + " )) && { builtin printf 'x\\0' >&3; builtin kill -KILL 0; }; ")
		script.WriteString(`builtin printf '%s\0' f "$#" "$FUNCNAME" "$@" >&3; return ` + status + "; }\n")
	}
	script.WriteString(`builtin printf 'R\0' >&3` + "\n")
	script.WriteString("eval '" + strings.ReplaceAll(line, "'", `'\''`) + "' < /dev/null\n")
	script.WriteString("builtin wait\n")
	return script.String()
}

// readBashRecords reads the call record bashScript's functions write. ready
// reports that bash defined every stub and reached the line.
func readBashRecords(body string) (view bashView, ready bool, err error) {
	fields := strings.Split(body, "\x00")
	// Every record ends with a NUL, so the last field is empty.
	fields = fields[:len(fields)-1]
	for index := 0; index < len(fields); {
		kind := fields[index]
		index++
		switch kind {
		case "R":
			ready = true
			continue
		case "x":
			view.capped = true
			continue
		case "f", "n":
		default:
			return bashView{}, false, errors.New("record kind " + strconv.Quote(kind) + " is none bashScript writes")
		}
		if index >= len(fields) {
			return bashView{}, false, errors.New("a " + kind + " record ends before its count")
		}
		count, countErr := strconv.Atoi(fields[index])
		if countErr != nil {
			return bashView{}, false, errors.New("a " + kind + " record's count " + strconv.Quote(fields[index]) + " is no number")
		}
		index++
		if kind == "f" {
			count++ // the function's name comes before its arguments
		}
		if count < 1 || index+count > len(fields) {
			return bashView{}, false, errors.New("a " + kind + " record holds fewer words than its count")
		}
		argv := slices.Clone(fields[index : index+count])
		index += count
		if kind == "f" {
			view.calls = append(view.calls, argv)
			continue
		}
		view.notFound = append(view.notFound, argv[0])
	}
	return view, ready, nil
}

// readCallValues reads every word of each call with readWord. outside is
// empty when every word reads to a value bash passes unchanged, and says why
// not otherwise. A word whose value is not read must say so in its error.
func readCallValues(t *testing.T, line string, calls [][]*syntax.Word) (argvs [][]string, outside string) {
	t.Helper()
	for _, words := range calls {
		argv := make([]string, 0, len(words))
		for _, word := range words {
			value, err := readWord(word)
			if err != nil {
				if _, isNotRead := errors.AsType[wordNotRead](err); !isNotRead {
					t.Errorf("%q: readWord answered %v, not a wordNotRead", line, err)
				}
				if !strings.Contains(err.Error(), "its value is not read") {
					t.Errorf("%q: the refusal %q does not say the value is not read", line, err.Error())
				}
				return nil, "a word whose value is not read"
			}
			if value.expands {
				return nil, "a word the shell expands"
			}
			argv = append(argv, value.text)
		}
		argvs = append(argvs, argv)
	}
	return argvs, ""
}

// joinedSorted answers each list joined on NUL, sorted, so two runs of the
// same commands compare as equal whatever order a pipeline ran them in.
func joinedSorted(lists [][]string) []string {
	joined := make([]string, 0, len(lists))
	for _, list := range lists {
		joined = append(joined, strings.Join(list, "\x00"))
	}
	slices.Sort(joined)
	return joined
}

// logBashSkips logs how many lines a comparison ran and why it left the rest.
func logBashSkips(t *testing.T, compared int, skipped map[string]int) {
	t.Helper()
	reasons := make([]string, 0, len(skipped))
	for reason := range skipped {
		reasons = append(reasons, reason)
	}
	slices.Sort(reasons)
	t.Logf("compared %d lines with bash", compared)
	for _, reason := range reasons {
		t.Logf("not run, %s: %d lines", reason, skipped[reason])
	}
}

// VALIDATES: A-2 and R-5 of spec-agent-economy-6-shell-parser. For each
// corpus line whose every simple command reads to words the shell does not
// expand, the argv bash passes to each command equals the values readWord
// gives, word for word in order. On a straight line (bashLine.straight, bash
// reaching the end with no syntax error and no cap) the two lists of argvs
// are equal as multisets, because a pipeline runs its commands in no fixed
// order; on any other line every argv bash passed is one the tree gives,
// since a loop or a branch runs a body zero or more times. A word readWord
// cannot read answers wordNotRead and says its value is not read.
// METHOD: the corpus (bashCorpusLines) runs through runBashLine, which runs
// no program; the working files and HOME make a missed expansion mark give a
// value bash does not pass. A line bash cannot be made to run under the
// stubs is counted, logged, and left out; the test fails when none is
// compared.
// PREVENTS: a judge admitting an option bash does not pass, because the hook
// kept a backslash bash drops or took an expanded word for its text.
func TestWordValuesMatchBash(t *testing.T) {
	bash := lookBash(t)

	ansiC := parseCommandLine(`grep -n $'x\ty' f`)
	if !ansiC.parsed() {
		t.Fatalf("the ANSI-C row did not parse: %s", ansiC.failure)
	}
	call, ok := ansiC.file.Stmts[0].Cmd.(*syntax.CallExpr)
	if !ok {
		t.Fatalf("the ANSI-C row is not one simple command")
	}
	if _, err := readWord(call.Args[2]); err == nil {
		t.Errorf("readWord gave a value for ANSI-C quoting, which bash decodes and the hook does not")
	}

	skipped := map[string]int{}
	compared := 0
	for _, line := range bashCorpusLines(t) {
		parse := parseCommandLine(line)
		if !parse.parsed() {
			skipped["a line that does not parse"]++
			continue
		}
		shape := analyseBashLine(parse.file)
		if shape.skip != "" {
			skipped[shape.skip]++
			continue
		}
		argvs, outside := readCallValues(t, line, shape.calls)
		if outside != "" {
			skipped[outside]++
			continue
		}
		view, reason := runBashLine(t, bash, line, shape)
		if reason != "" {
			skipped[reason]++
			continue
		}
		compared++
		compareBashArgvs(t, line, argvs, view, shape.straight)
	}
	logBashSkips(t, compared, skipped)
	if compared == 0 {
		t.Fatal("no corpus line was compared with bash")
	}
}

// compareBashArgvs holds the argvs bash passed against the ones the tree
// gives, by the rule TestWordValuesMatchBash states.
func compareBashArgvs(t *testing.T, line string, argvs [][]string, view bashView, straight bool) {
	t.Helper()
	read := joinedSorted(argvs)
	passed := joinedSorted(view.calls)
	if straight && !view.capped && !view.syntaxError {
		if !slices.Equal(read, passed) {
			t.Errorf("%q: bash passed %q, the hook reads %q", line, passed, read)
		}
		return
	}
	for _, argv := range passed {
		if !slices.Contains(read, argv) {
			t.Errorf("%q: bash passed %q, which no command the hook reads gives (it reads %q)", line, argv, read)
		}
	}
}

// VALIDATES: A-2 of spec-agent-economy-6-shell-parser: the commands of the
// tree are the commands bash runs, with no hidden one. For every corpus line
// bash runs under the stubs, (1) bash looked up no command word the tree does
// not name (command_not_found_handle recorded none, and every stub call names
// a command word of the tree), and (2) on a straight line the command words
// bash ran equal the tree's as multisets. A loop or a branch runs a body zero
// or more times, so on any other line only (1) holds.
// METHOD: as TestWordValuesMatchBash, over every line whose command words
// are literal, whatever its arguments.
// PREVENTS: a line bash reads as running a command the hook's tree does not
// hold, which the predicate would then never judge.
func TestParsedCommandsMatchBash(t *testing.T) {
	bash := lookBash(t)
	skipped := map[string]int{}
	compared, straight := 0, 0
	for _, line := range bashCorpusLines(t) {
		parse := parseCommandLine(line)
		if !parse.parsed() {
			skipped["a line that does not parse"]++
			continue
		}
		shape := analyseBashLine(parse.file)
		if shape.skip != "" {
			skipped[shape.skip]++
			continue
		}
		view, reason := runBashLine(t, bash, line, shape)
		if reason != "" {
			skipped[reason]++
			continue
		}
		compared++
		if len(view.notFound) > 0 {
			t.Errorf("%q: bash looked up %q, which the tree does not name (it names %q)", line, view.notFound, shape.names)
		}
		ran := make([]string, 0, len(view.calls))
		for _, argv := range view.calls {
			ran = append(ran, argv[0])
			if !slices.Contains(shape.names, argv[0]) {
				t.Errorf("%q: bash ran %q, which the tree does not name (it names %q)", line, argv[0], shape.names)
			}
		}
		if !shape.straight || view.capped || view.syntaxError {
			continue
		}
		straight++
		named := slices.Clone(shape.names)
		slices.Sort(named)
		slices.Sort(ran)
		if !slices.Equal(named, ran) {
			t.Errorf("%q: on a line with no branch bash ran %q, the tree names %q", line, ran, named)
		}
	}
	logBashSkips(t, compared, skipped)
	t.Logf("of which %d straight lines compared as multisets", straight)
	if compared == 0 {
		t.Fatal("no corpus line was compared with bash")
	}
	if straight == 0 {
		t.Fatal("no straight corpus line was compared with bash, so rule (2) never ran")
	}
}

// FuzzShellParse holds R-2 of spec-agent-economy-6-shell-parser, the phase 2
// part: no line makes the raw parser panic (parseBashLine is called directly,
// outside parseCommandLine's recover, so a panic fails the target), the
// contained parse answers exactly one of a tree and a failure and agrees with
// the raw one except on a line holding a carriage return, which it refuses,
// and readWord answers every word of a tree with a value or a
// wordNotRead. Phase 3 adds the predicate's property: it never answers
// "reader line" for a line holding a command outside the reader set; and no
// walk of the line (the reader line with and without the identity line, the
// after-line identity walk, the deletion guard's two readings) panics. Phase 4
// adds its walks: the reader line with every write recorded, the tree
// judgments of the raw-job and piped-command guards, and the scratch guard's
// here-document blanking.
func FuzzShellParse(f *testing.F) {
	for _, line := range bashCorpusLines(f) {
		f.Add(line)
	}
	f.Fuzz(func(t *testing.T, line string) {
		_, rawErr := parseBashLine(line)
		parse := parseCommandLine(line)
		if parse.parsed() == (parse.failure != "") {
			t.Fatalf("%q: the parse answered tree=%v and failure %q, want exactly one", line, parse.parsed(), parse.failure)
		}
		// Review I4: the walks are called without the recover their
		// callers put around them, so a panic in any of them fails the
		// target instead of being answered on the refusing side.
		readReaderLine(line, parse, "", readerWalk{source: line, admit: readerAdmit})
		readReaderLine(line, parse, "", readerWalk{source: line, admit: readerAdmit, recordWrites: true})
		readPlainRmArguments(line, parse)
		readDeletionWords(line, parse)
		readTreeRawHeavy(line, parse, t.TempDir())
		readTreeLossyPipe(line, parse)
		readWithoutReaderData(line, parse, "")
		owned := identityLineOfAgent + line
		ownedParse := parseCommandLine(owned)
		readReaderLine(owned, ownedParse, identityLineOfAgent, readerWalk{source: owned, admit: readerAdmit})
		readWithoutReaderData(owned, ownedParse, identityLineOfAgent)
		readAnotherIdentity(owned, ownedParse, identityLineOfAgent)
		if strings.ContainsRune(line, '\r') {
			// The raw parser reads a carriage return as bash does not, so
			// the contained parse refuses every such line.
			if parse.parsed() {
				t.Fatalf("%q: a line holding a carriage return parsed; bash keeps the character the parser drops", line)
			}
			return
		}
		if parse.parsed() != (rawErr == nil) {
			t.Fatalf("%q: the contained parse (parsed=%v) disagrees with the raw parser (error %v)", line, parse.parsed(), rawErr)
		}
		if !parse.parsed() {
			return
		}
		syntax.Walk(parse.file, func(node syntax.Node) bool {
			word, ok := node.(*syntax.Word)
			if !ok {
				return true
			}
			if _, err := readWord(word); err != nil {
				if _, isNotRead := errors.AsType[wordNotRead](err); !isNotRead {
					t.Fatalf("%q: readWord answered %v, not a wordNotRead", line, err)
				}
			}
			return true
		})
		// Phase 3: the predicate never answers reader line for a line that
		// holds, at any depth, a command outside the reader set.
		if _, outside := judgeReaderLine(line, parse, ""); outside != "" {
			return
		}
		syntax.Walk(parse.file, func(node syntax.Node) bool {
			call, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}
			value, err := readWord(call.Args[0])
			if err != nil {
				t.Fatalf("%q: a reader line whose command word is not read (%v)", line, err)
			}
			if value.expands {
				t.Fatalf("%q: a reader line whose command word the shell expands", line)
			}
			if _, reader := readerCommands[value.text]; !reader {
				t.Fatalf("%q: a reader line holding %q, which is outside the reader set", line, value.text)
			}
			return true
		})
	})
}

// BenchmarkHookParse measures A-4 of spec-agent-economy-6-shell-parser: the
// allocations of one parse of a typical line and of a reader loop. Only the
// allocation figures are read: a time figure from a host of unknown
// performance is not compared.
func BenchmarkHookParse(b *testing.B) {
	for _, bench := range []struct{ name, line string }{
		{"pipeline", `git show HEAD:x | grep -n '^##' | head -20`},
		{"reader-loop", `for f in a b; do grep -n "x" "$f" | head -5; done`},
	} {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if parse := parseCommandLine(bench.line); !parse.parsed() {
					b.Fatalf("%q did not parse: %s", bench.line, parse.failure)
				}
			}
		})
	}
}
