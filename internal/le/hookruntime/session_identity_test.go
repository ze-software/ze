// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: session.go -- the identity check these tests drive through Run
package hookruntime

import (
	"slices"
	"strings"
	"testing"
)

const (
	// identityLineOfAgent is the line agent-1 of session parent-17 types.
	identityLineOfAgent = "export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-17; "
	// identityLineOfFork is the line a fork subagent of parent-17 types: it
	// has no agent id, so the session part is the whole line.
	identityLineOfFork = "export CLAUDE_CODE_SESSION_ID=parent-17; "
)

// runSubagentBash sends one Bash command through the pretool-bash entry as
// agent-1 of session parent-17.
func runSubagentBash(t *testing.T, command string) (code int, output, message string) {
	t.Helper()
	return runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "agent_id": "agent-1", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
}

// runForkBash sends one Bash command through the pretool-bash entry as a fork
// subagent of session parent-17, which carries no agent id.
func runForkBash(t *testing.T, command string) (code int, output, message string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "1")
	return runHook(t, t.TempDir(), "pretool-bash", map[string]any{
		"session_id": "parent-17", "tool_name": "Bash",
		"tool_input": map[string]any{"command": command},
	})
}

// VALIDATES: AC-1. A subagent's command made only of reader commands passes
// with exit 0 and an empty standard output, so the harness receives no changed
// input and the command runs exactly as typed. A loop whose body is readers
// (AC-5) and a here-document fed to a reader (AC-6) are reader lines too: the
// here-document's body is data the reader reads, so a `./le` written in it
// runs nothing.
// PREVENTS: the hook rewriting a command, which makes auto mode void the call
// and every other call of the same message.
func TestSubagentReaderCommandPassesUnchanged(t *testing.T) {
	for _, command := range []string{
		`for f in a b; do cat "$f"; done`,
		`cat <<EOF` + "\n./le spec current\nEOF",
		`grep -n "a\|b" internal/x.go | head -5`,
		`cd /srv/checkout && git log --oneline -3; sed -n 1,5p notes.md 2>&1 | wc -l`,
		`gopls symbols internal/le/hookruntime/session.go`,
		`cat docs/a.md docs/b.md | sort | uniq -c | tail -3`,
		`git diff --stat && git status --short`,
		`ls -la internal/le &> /dev/null`,
		"grep -c x a.md\ngrep -c y b.md",
		`grep -n 'echo $(date)' notes.md`,
		`[ -f notes.md ] && cat notes.md`,
		`test -d docs && ls docs`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 0 {
			t.Errorf("%q: code = %d, want 0; message: %s", command, code, message)
		}
		if output != "" {
			t.Errorf("%q: standard output = %q, want none: the hook returns no changed input", command, output)
		}
		if strings.Contains(message, "identity line") {
			t.Errorf("%q: a reader command was asked for the identity line: %s", command, message)
		}
	}
	code, output, message := runForkBash(t, `grep -n x notes.md | head -5`)
	if code != 0 || output != "" {
		t.Errorf("fork reader command: code=%d output=%q message=%s, want exit 0 and no output", code, output, message)
	}
}

// VALIDATES: AC-2. A subagent's command holding any command outside the reader
// set, and not starting with the agent's identity line, is refused with exit 2
// and a message that prints the full line for this agent.
// PREVENTS: a subagent's ./le call running as the main thread and sharing its
// commit namespace and spec claim.
func TestSubagentCommandOutsideTheReaderSetNeedsTheIdentityLine(t *testing.T) {
	for _, command := range []string{
		`./le spec current`,
		`grep -n x notes.md; ./le commit create`,
		`go version`,
		`echo $(./le spec current)`,
		"echo `./le spec current`",
		`echo "$(./le spec current)"`,
		`LC_ALL=C grep -n x notes.md`,
		`find . -name '*.md' -exec ./le doc check {} \;`,
		`git -C /srv/checkout log --oneline`,
		`git stash list`,
		`cat list.txt | xargs ./le doc check`,
		`for f in a b; do ./le doc check "$f"; done`,
		`rg --pre ./le x notes.md`,
		`sort --compress-program=./le notes.md`,
		`./le spec current <<EOF` + "\nx\nEOF",
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
	code, output, message := runForkBash(t, `./le spec current`)
	if code != 2 || output != "" {
		t.Errorf("fork: code=%d output=%q, want exit 2 and no output", code, output)
	}
	if !strings.Contains(message, identityLineOfFork) || strings.Contains(message, "ZE_AGENT_ID=") {
		t.Errorf("fork: message must print the session part alone, %q: %s", identityLineOfFork, message)
	}
}

// VALIDATES: AC-3. A command that starts with the agent's exact identity line
// passes, whatever follows, with no changed input.
// PREVENTS: an agent that typed the line it was given being refused again.
func TestSubagentCommandCarryingItsIdentityLinePasses(t *testing.T) {
	for _, command := range []string{
		`./le spec current`,
		`grep -n x notes.md; ./le spec state current`,
		`echo "$ZE_AGENT_ID" && ./le spec current`,
		`grep -rn "ZE_AGENT_ID=" docs/ ; ./le spec current`,
		`grep ZE_AGENT_ID= notes.md`,
		`bash tmp/commit-4f95a2b5-a-probe.sh`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+command)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
	}
	code, output, message := runForkBash(t, identityLineOfFork+`./le spec current`)
	if code != 0 || output != "" {
		t.Errorf("fork: code=%d output=%q message=%s, want exit 0 and no output", code, output, message)
	}
}

// VALIDATES: AC-4. A command whose identity line names another agent or
// session, or that takes another identity after its own line, is refused with
// the line this agent owns.
// PREVENTS: an agent acting under a sibling's commit namespace and spec claim,
// or under the main thread's.
func TestSubagentIdentityLineOfAnotherAgentIsRefused(t *testing.T) {
	for _, command := range []string{
		`export ZE_AGENT_ID=agent-2; export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		`export ZE_AGENT_ID=agent-1; export CLAUDE_CODE_SESSION_ID=parent-18; ./le spec current`,
		`export ZE_AGENT_ID=agent-10; export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		`export CLAUDE_CODE_SESSION_ID=parent-17; ./le spec current`,
		identityLineOfAgent + `export ZE_AGENT_ID=agent-2; ./le spec current`,
		identityLineOfAgent + `ZE_AGENT_ID=agent-2 ./le spec current`,
		identityLineOfAgent + `env CLAUDE_CODE_SESSION_ID=parent-18 ./le spec current`,
		identityLineOfAgent + `unset ZE_AGENT_ID; ./le spec current`,
		identityLineOfAgent + `eval "export ZE_AGENT_ID=agent-2"; ./le spec current`,
		identityLineOfAgent + `bash -c 'CLAUDE_CODE_SESSION_ID=parent-18 ./le spec current'`,
		identityLineOfAgent + `sh -c 'unset ZE_AGENT_ID; ./le spec current'`,
		identityLineOfAgent + `ZE_AGENT_ID=agent-2 ./le spec "unclosed`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", command, message)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line this agent owns %q: %s", command, identityLineOfAgent, message)
		}
	}
}

// VALIDATES: AC-5. A main-thread command, which carries no agent id and is no
// fork, is never asked for the identity line and receives no changed input.
// PREVENTS: the check binding the main thread, which is the identity le
// resolves by default.
func TestMainThreadCommandNeedsNoIdentityLine(t *testing.T) {
	t.Setenv("CLAUDE_CODE_FORK_SUBAGENT", "")
	for _, command := range []string{`./le spec current`, `go version`, `grep "unclosed notes.md`} {
		code, output, message := runHook(t, t.TempDir(), "pretool-bash", map[string]any{
			"session_id": "parent-17", "tool_name": "Bash",
			"tool_input": map[string]any{"command": command},
		})
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
		if strings.Contains(message, "identity line") {
			t.Errorf("%q: the main thread was asked for the identity line: %s", command, message)
		}
	}
}

// VALIDATES: R-2. A line the word splitter cannot parse (an unclosed quote, a
// trailing backslash) is never judged a reader line, even when the words it
// seems to hold are all readers.
// PREVENTS: a command the hook cannot see whole reaching ./le without the
// agent's identity.
func TestSubagentUnparsableCommandNeedsTheIdentityLine(t *testing.T) {
	for _, command := range []string{
		`grep -n "unclosed notes.md`,
		`grep -n 'unclosed notes.md | head`,
		`cat notes.md \`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
}

// VALIDATES: R2 of the phase 2 brief, through Run. A reader's judge is an
// allow-list, so a reader carrying an option it does not list (an abbreviated
// long option, an option that runs a program, writes a file or evaluates an
// array subscript), a second uniq operand, or a word the shell expands needs
// the identity line, while the same readers in the shapes their judges list
// run without it.
// PREVENTS: `printf -v 'a[$(./le spec current)]' v` passing as a reader line:
// bash evaluates the subscript, which runs ./le under the main thread's
// identity. The single quotes hide the substitution from the splitter.
func TestSubagentReaderOutsideItsJudgeNeedsTheIdentityLine(t *testing.T) {
	for _, command := range []string{
		`printf -v 'a[$(./le spec current)]' v`,
		`echo -v; printf $_ 'a[$(./le spec current)]' v`,
		`test -v 'a[$(./le spec current)]'`,
		`[ -v 'a[$(./le spec current)]' ]`,
		`[ -R 'a[$(./le spec current)]' ]`,
		`git grep --open=./le x`,
		`git log --ext x`,
		`git diff --output=out.txt`,
		`echo --ext-diff; git log -p $_`,
		`sort --compress=./le notes.md`,
		`sort --co=./le notes.md`,
		`sort -o out.txt notes.md`,
		`rg -z x notes.md`,
		`rg -z x`,
		`rg -iz x`,
		`rg --pre-glob '*' x notes.md`,
		`diff -l a b`,
		`diff --pag a b`,
		`find . -delete`,
		`find . -fprint out.txt`,
		`find . -name *.md`,
		`sed -n '1e ./le spec current' notes.md`,
		`sed -i 's/a/b/' notes.md`,
		`sed 's/a/b/w out.txt' notes.md`,
		`sed -f script.sed notes.md`,
		`sed --expr=1p notes.md`,
		`gopls execute x`,
		`gopls rename -w f.go:1:1 y`,
		`gopls -remote=auto symbols f.go`,
		`gopls codelens -exec f.go`,
		`uniq notes.md out.txt`,
		`cat a > f`,
		`echo x >> f`,
		`printf x >| f`,
		`cat a &> f`,
		`cat a 2> f`,
		`echo x >| f`,
		`echo x\>|sh`,
		`echo 'x'\>|sh`,
		`echo "x>"|sh`,
		`uniq -c ~/notes.md`,
		`sort -k1 X=~/notes.md`,
		// NOTE: bash brace-expands this to `find . -exec sh x`; the word
		// reading marks the brace expansion expanded (readWord), and find's
		// judge refuses a word the shell expands.
		`find . {-exec,sh} x`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
	for _, command := range []string{
		`sed -n '/go build/p;10,20p' notes.md`,
		`sed 's/[a-z]\/b/x/g' notes.md`,
		`git log --oneline -5 -- notes.md`,
		`git grep -n x -- '*.go'`,
		`sort -t, -k2 -n notes.md`,
		`rg -n -g '*.go' x`,
		`find . -name '*_test.go' -mtime -1`,
		`diff -u a b`,
		`printf '%s\n' x`,
		`[ -f notes.md ] && test -n x`,
		`gopls references -d f.go:1:2`,
		`uniq -c notes.md`,
		`cat a > /dev/null`,
		`grep x f 2>&1`,
		`cat a 2>/dev/null >&2 < notes.md`,
		// NOTE: `[` evaluates no arithmetic expression (bash does so only
		// under `[[`), and the single quotes hide `$(id)` from the shell.
		`[ 1 -eq 'x[$(id)]' ]`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
	}
}

// splitterFaultLines are lines the hand splitter read differently from bash,
// or that the byte allow-list refused: each one hid `le x` from the
// splitter, or holds a construct the allow-list left out. The guard tests
// drive the same constructs with a build in place of `le x`. Read from the
// bash parse, the lines splitterReaderLines names are reader lines, and every
// other one still holds a command or a construct outside the reader line.
var splitterFaultLines = []string{
	// A brace or a `!` inside a word ended the command, and the `#` after it
	// then read as a comment that hid `le x`.
	`cat x{#}; le x`,
	`cat x}#; le x`,
	`cat x!#; le x`,
	// ANSI-C quoting: bash reads `$'\''` as one quoted `'`, the splitter as
	// a quote that closes only at the last `'`.
	`cat $'\'' ; le x ; cat \'`,
	// A here-document body is text to bash and commands to the splitter.
	"cat <<cat\ngrep '\ncat\nle x\ncat <<cat\n'\ncat",
	`cat $'x'`,
	`cat $"x"`,
	"cat <<EOF\nx\nEOF",
	`cat x\ y`,
	`cat a{b}`,
	`cat a#b`,
	`cat '#'`,
	`echo "a\"b"`,
	"cat x\r",
	"cat caf\xc3\xa9",
	"cat x\x01",
	`cat a & cat b`,
	`cat a |& cat b`,
	`cat <(cat a)`,
	`cat ${x:-a}`,
	`cat "${x:-a}"`,
	`cat ${x}`,
	`[[ -f a ]]`,
	`cat a >& f`,
	`cat a <<< `,
	"cat 'a\nb'",
}

// splitterReaderLines are the lines of splitterFaultLines whose every command
// is a reader its judge admits, read from the bash parse
// (spec-agent-economy-6-shell-parser AC-6, AC-7).
var splitterReaderLines = []string{
	"cat <<EOF\nx\nEOF", `cat x\ y`, `cat a{b}`, `cat a#b`, `cat '#'`, `echo "a\"b"`,
	"cat caf\xc3\xa9", "cat x\x01", `cat a |& cat b`, `cat <(cat a)`, `cat ${x:-a}`,
	`cat "${x:-a}"`, `cat ${x}`, `[[ -f a ]]`, "cat 'a\nb'",
}

// VALIDATES: the identity exemption reads each line the splitter misread from
// the bash parse: every line of splitterFaultLines that holds a command or a
// construct outside the reader line needs the identity line, the ones
// splitterReaderLines names pass as typed, and the common reader shapes (a
// quoted parameter, a quoted `|`, a descriptor redirection, a quoted
// backslash bash keeps, a here-string) still run without it.
// PREVENTS: `cat x{#}; le x` passing as a reader line because a reader read
// `#}; le x` as a comment, and every other construct a hand-written splitter
// reads differently from bash.
func TestSubagentSplitterFaultLinesAreJudgedFromTheParse(t *testing.T) {
	for _, command := range splitterFaultLines {
		if slices.Contains(splitterReaderLines, command) {
			expectPasses(t, []string{command})
			continue
		}
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
	for _, command := range []string{
		`cat "$dir/x.log"`,
		`cat "${dir}/x.log"`,
		`grep -n 'a|b' f`,
		`git log --oneline -3`,
		`cat a 2>&1`,
		`sed -n '1,5p' f`,
		`grep -n "a\|b" f`,
		`grep -c x <<< y`,
		`cat a >> /dev/null`,
	} {
		code, output, message := runSubagentBash(t, command)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
	}
	// The splitter read `#}; ZE_AGENT_ID=...` as a comment, so the second
	// identity was not seen after the agent's own line; the parse sees it.
	for _, rest := range []string{
		`cat x{#}; ZE_AGENT_ID=agent-2 ./le spec current`,
		"cat <<cat\ngrep '\ncat\nCLAUDE_CODE_SESSION_ID=parent-18 ./le x\ncat <<cat\n'\ncat",
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
}

// VALIDATES: the reader-line predicate over the lines the allow-list judged
// by bytes: each line that holds a construct outside the reader line, or
// that does not parse, is outside it; each line the splitter read as readers
// that the parse reads as readers too is a reader line.
// PREVENTS: a construct the allow-list refused being admitted by the
// predicate without a reason the tree gives.
func TestReaderLineNamesEveryConstructOutsideIt(t *testing.T) {
	for _, line := range []string{
		"cat a;; cat b", "cat (a)", "cat a)", "echo `id`", `cat "$(id)"`, `cat $((1))`,
		`cat a<>b`, `cat a >>> b`, `cat a >`, `cat "${x"`, `cat "a` + "`" + `b"`, `cat 'a`, `cat "a`,
		`cat a > f`, `cat a >> f`, `cat a >| f`, `cat a &> f`, `cat a 2> f`, `cat a 2>> f`, `X=1 cat a`,
	} {
		if _, outside := judgeReaderLine(line, parseCommandLine(line), ""); outside == "" {
			t.Errorf("%q: admitted as a reader line, want it outside", line)
		}
	}
	for _, line := range []string{
		`cat a`, "cat a\tb\ncat c", `cat 'a b' "c d"`, `cat "$x" $y $_ $? $1 "$@"`,
		`cat "${NAME_1}"`, `cat a 2>&1 >&2`, `cat < a`, `cat <<< 'x'`, `ls *.go a?b [ab]c ~/x`,
		`[ -f a ] && test -n x`, `printf '%s\n' x`, `grep 'a(b){c}!d' f`, `grep "a\|b" f`,
		`cat $`, `cat "$"`, `cat $!`, `cat "a!b"`, `cat "a#b"`, "cat a\\\nb",
	} {
		if _, outside := judgeReaderLine(line, parseCommandLine(line), ""); outside != "" {
			t.Errorf("%q: outside the reader line (%s), want a reader line", line, outside)
		}
	}
}

// VALIDATES: inside double quotes the word reading keeps a backslash that
// bash keeps (before any character but `$`, a backtick, `"`, `\` and a
// newline), so the word a judge reads is the word bash passes on.
// PREVENTS: a judge admitting `\x` read as `x` while the command receives
// `\x`.
func TestReaderLineKeepsABackslashBashKeepsInDoubleQuotes(t *testing.T) {
	line, outside := judgeReaderLine(`grep "a\|b\$c" f`, parseCommandLine(`grep "a\|b\$c" f`), "")
	if outside != "" {
		t.Fatalf("the line is no reader line: %s", outside)
	}
	if got := strings.Join(line.commands[0].words, " "); got != `grep a\|b$c f` {
		t.Errorf("words = %q, want %q", got, `grep a\|b$c f`)
	}
}

// expectNeedsIdentityLine runs each command as a subagent with no identity
// line and requires the refusal that prints the line to type.
func expectNeedsIdentityLine(t *testing.T, commands []string) {
	t.Helper()
	for _, command := range commands {
		code, output, message := runSubagentBash(t, command)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", command, code, output)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: message lacks the line to type %q: %s", command, identityLineOfAgent, message)
		}
	}
}

// expectPasses runs each command as a subagent and requires it to pass as
// typed.
func expectPasses(t *testing.T, commands []string) {
	t.Helper()
	for _, command := range commands {
		code, output, message := runSubagentBash(t, command)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", command, code, output, message)
		}
	}
}

// VALIDATES: a redirection is a node of its statement wherever it is written
// (readerWalk.redirect, readerline.go), and its target is never a word of the
// command: a line whose command, read without the redirection, lies outside
// the reader set needs the identity line, and a reader with a redirection
// before, between or after its words passes.
// PREVENTS: `<<<cat ./le commit create x` passing as the reader cat, when its
// command is `./le`, and `sed -e <<<p 'w out' f` passing as `sed -e p`, when
// sed runs the script `w out`, which writes out.
func TestSubagentRedirectionBeforeAWordNeedsTheIdentityLine(t *testing.T) {
	expectNeedsIdentityLine(t, []string{
		`<<<cat ./le commit create x`,
		`<<<cat go build -o /tmp/x ./cmd/ze`,
		`sed -e <<<p 'w out' f`,
		`printf <<<x -v 'a[$(touch /tmp/p)]' y`,
	})
	// A redirection is a node of its statement wherever it is written, so
	// `<<<cat a` runs the command `a`, outside the reader set, while a
	// redirection before or between a reader's words leaves it a reader.
	expectNeedsIdentityLine(t, []string{`<<<cat a`})
	expectPasses(t, []string{`2>/dev/null cat a`, `cat <f a`, `cat a >/dev/null b`})
	expectPasses(t, []string{
		`cat "$dir/x.log"`,
		`grep -n 'a|b' f`,
		`cat a 2>&1`,
		`grep x f 2>/dev/null`,
		`cat a > /dev/null`,
		`sed -n '1,5p' f`,
	})
}

// VALIDATES: the parse keeps an empty quoted word as a word whose value is
// empty, so a reader's judge reads it as the value of the option before it:
// `diff -L ” -l a b` runs diff -l, which the judge refuses, and needs the
// identity line, while a reader that takes the empty word as an operand
// passes.
// PREVENTS: diff -L with an empty quoted value before -l passing as
// `diff -L -l a b`, which drops the empty word, while bash passes it as -L's
// value and runs diff -l, which pipes its output through pr.
func TestSubagentEmptyQuotedWordNeedsTheIdentityLine(t *testing.T) {
	expectNeedsIdentityLine(t, []string{
		`diff -L '' -l a b`,
		`diff -L "" -l a b`,
	})
	// The parse keeps an empty quoted word as a word whose value is empty,
	// so diff's judge sees -L take it and refuses the -l after it, and a
	// reader with no such option passes.
	if _, outside := judgeReaderLine(`diff -L '' -l a b`, parseCommandLine(`diff -L '' -l a b`), ""); !strings.Contains(outside, "`diff -L '' -l a b`") {
		t.Errorf("diff -L '' -l: outside = %q, want diff's judge to refuse the whole command", outside)
	}
	expectPasses(t, []string{`cat '' a`, `cat a''b ''""`, `cat a''b '' x`, `cat a''b "'"`})
}

// VALIDATES: a backslash bash keeps inside double quotes is kept by
// shellWords too, so a redirection's target is the file bash writes.
// PREVENTS: `cat x > "\/dev/null"` passing as a write to /dev/null, while
// bash writes the file `\/dev/null`.
func TestSubagentQuotedBackslashTargetIsAFile(t *testing.T) {
	expectNeedsIdentityLine(t, []string{`cat x > "\/dev/null"`})
	if got := strings.Join(shellWords(`go build -o "\bin/ze" "a\$b"`), " "); got != `go build -o \bin/ze a$b` {
		t.Errorf("shellWords = %q, want %q", got, `go build -o \bin/ze a$b`)
	}
}

// VALIDATES: after the agent's own identity line, a command that changes the
// environment by its form is refused whatever names it carries: export, env,
// exec, sudo, declare, unset and the rest, and an assignment in front of a
// command to a name that is not plain, on modeled and unmodeled lines alike,
// while the plain forms agents type still pass. The rows that name no
// identity variable prove each form by its form, and one row puts an
// environment tool behind each reserved word and each wrapper.
// PREVENTS: a name built from pieces (`ZE_AGENT_""ID`, `"${n}_ID"`, a
// backslash, a brace, `$'\x49'`) hiding from the literal-mention rule, and
// `env -i`, `exec -c` or `sudo` dropping the identity the line set.
func TestSubagentChangingTheEnvironmentAfterItsLineIsRefused(t *testing.T) {
	for _, rest := range []string{
		`export ZE_AGENT_""ID=x; ./le spec current`,
		`export Z''E_AGENT_ID=x; ./le spec current`,
		`unset CLAUDE_CODE_""SESSION_ID; ./le spec current`,
		`env ZE_AGENT_""ID=x le spec current`,
		`declare -n r=ZE_AGENT_""ID; r=x; ./le spec current`,
		`n=ZE_AGENT; export "${n}_ID=x"; ./le spec current`,
		`n=ZE_AGENT_; export "$n"ID=x; ./le spec current`,
		`ZE_AGENT_""ID=x ./le spec current`,
		`time Z""E_AGENT_ID=x ./le spec current`,
		`export ZE_AGENT_\ID=x; ./le spec current`,
		`ZE_AGENT_{ID,}=x ./le spec current`,
		`export $'ZE_AGENT_\x49D'=x; ./le spec current`,
		`env -i le x`,
		`exec -c le x`,
		`sudo le x`,
		`nice -n 5 env -u ZE_AGENT_""ID le x`,
		// AC-27 of spec-agent-economy-6-shell-parser: rows that name no
		// identity variable, so each is refused by its form and not by the
		// mention rule.
		`source x`,
		`. x`,
		`set -a`,
		`eval x`,
		`typeset -x A`,
		`readonly A`,
		`local A`,
		`command export A=1`,
		`if true; then export A=1; fi`,
		`timeout 5 env -i le x`,
		`time . x`,
		// An environment tool behind each reserved word and each wrapper.
		`if env -i le x; then true; fi`,
		`if false; then true; elif env -i le x; then true; fi`,
		`if false; then true; else env -i le x; fi`,
		`for f in a; do env -i le x; done`,
		`while env -i le x; do true; done`,
		`until env -i le x; do true; done`,
		`nohup env -i le x`,
		`timeout 1.5s sudo le x`,
		`xargs env -i le x`,
		`stdbuf -oL env -i le x`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
	for _, rest := range []string{
		`./le spec current`,
		`cd x && ./le spec current`,
		`FOO=bar ./le spec current`,
		`ZE_SPEC_WIP_CAP=15 ./le spec claim x.md`,
		`go test ./x`,
	} {
		// The identity check alone: a raw go test is the raw-job guard's to
		// refuse, not this one's.
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want the plain form to pass", rest)
		}
	}
}

// VALIDATES: after the agent's own identity line, any command that names an
// identity variable is refused as taking another identity, unless it is a
// reader its judge admits, on modeled and unmodeled lines alike.
// PREVENTS: each form of setting a variable that a list of forms missed:
// `+=`, a subscript, a keyword or `time` in front of the assignment,
// `command export`, `exec env`, `for`, `read`, `printf -v`, `let`, `mapfile`
// and `getopts`, and a redirection that moved the command word.
func TestSubagentNamingAnIdentityVariableAfterItsLineIsRefused(t *testing.T) {
	for _, rest := range []string{
		`ZE_AGENT_ID+=x ./le spec current`,
		`export ZE_AGENT_ID+=x; ./le spec current`,
		`ZE_AGENT_ID[0]=x ./le spec current`,
		`if true; then ZE_AGENT_ID=x ./le spec current; fi`,
		`time ZE_AGENT_ID=x ./le spec current`,
		`command export ZE_AGENT_ID=x; ./le spec current`,
		`builtin export CLAUDE_CODE_SESSION_ID=x; ./le spec current`,
		`exec env ZE_AGENT_ID=x ./le spec current`,
		`nice env ZE_AGENT_ID=x ./le spec current`,
		`for ZE_AGENT_ID in x; do ./le spec current; done`,
		`read ZE_AGENT_ID <<<x; ./le spec current`,
		`printf -v ZE_AGENT_ID x; ./le spec current`,
		`let ZE_AGENT_ID=1; ./le spec current`,
		`mapfile ZE_AGENT_ID <<<x; ./le spec current`,
		`getopts a ZE_AGENT_ID; ./le spec current`,
		`cat a > ZE_AGENT_ID; ./le spec current`,
		`<<<x ZE_AGENT_ID=y ./le spec current`,
		`>/dev/null export ZE_AGENT_ID=y; ./le spec current`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
	for _, rest := range []string{
		`grep ZE_AGENT_ID f`,
		`echo $ZE_AGENT_ID`,
		`grep -n CLAUDE_CODE_SESSION_ID f; ./le spec current`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", rest, code, output, message)
		}
	}
}

// VALIDATES: the journal row of 2026-11-11 in
// plan/journal/hook-existing-patterns-false-positive.md (`bashAgentIdentity`
// on a quoted reason). Through Run, after the agent's own identity line, a
// `./le` whose quoted argument holds `!=`, `->`, `^`, `~`, parentheses or a
// single quote passes, with or without a `cd` in front: the after-line walk
// reads the quoted argument as one word of the parse, so its characters
// change no verdict. The siblings hold the same quoted text and still change
// the environment: a declaration inside a substitution the quotes hold, an
// environment tool or a declaration in a later statement, arithmetic, and a
// mention of an identity variable outside a reader, which the mention rule
// reads in the source text by design.
// PREVENTS: the defect the row records. At 9f61ac6f47 a `!` inside the
// double-quoted reason put the line outside the splitter's modeled syntax
// (unmodeledSyntax, syntaxBang), so the remainder was judged by its text
// (textChangesEnvironment), which splits at `!` and `(` whatever the quotes:
// the piece after the `!` of `!=` began with the word `=`, read as an
// assignment to a name that is not plain, and the approval was refused as
// taking another identity.
func TestQuotedArgumentAfterTheLineChangesNoVerdict(t *testing.T) {
	for _, rest := range []string{
		`./le test weakened approve unit hookruntime.TestX reason "keeps a -> b ^ c ~ d"`,
		`cd internal && ./le test weakened approve unit x reason "a -> b"`,
		// The refused reason of the row, its characters kept.
		`./le test weakened approve unit static.TestX reason "Old expectation SwIfIndex==0 was wrong against VPP's fib_path_create, where 'else if (~0 != rpath->frp_sw_if_index)' makes index 0 (local0) an attached path; the new assertion ==NoInterface (^uint32(0)) is as strict."`,
		`cd /srv/checkout && ./le test weakened approve unit x reason "a (~0 != b) c"`,
		`./le x reason 'a != b; env -i ./le y'`,
		`./le x reason "set -a; export A=1 | env -i"`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", rest, code, output, message)
		}
	}
	for _, rest := range []string{
		`./le x reason "$(export ZE_AGENT_ID=y)"`,
		`./le x reason "a != b $(export A=1)"`,
		`./le x reason "a != b"; env -i ./le x`,
		`./le x reason "a -> b ^ c ~ d" && export A=1`,
		`cd internal && ./le x reason "a -> b"; unset A`,
		`./le x reason "a != b" "$(( n = 1 ))"`,
		`./le x reason "names ZE_AGENT_ID"`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
}

// TestEnvironmentToolIsFoundBehindEveryWrapper is AC-26 of
// spec-agent-economy-6-shell-parser.
//
// VALIDATES: through the hook's Run, after the agent's own identity line, a
// lone `.` behind `time`, `then`, `command` or `builtin` is the command word
// and is refused as taking another identity, on a line inside the modeled
// syntax and on one outside it (a brace group): a word the walk skips as a
// duration holds a digit. A wrapper's numeric argument that holds a digit is
// still skipped, so `timeout 5`, `timeout 1.5s` and `nice -n 5` in front of
// `./le` pass.
// PREVENTS: `time . ./env.sh` sourcing a file that sets either identity
// variable unseen, because the walk skipped `.` as a duration made only of
// the characters `0123456789.smhd`.
func TestEnvironmentToolIsFoundBehindEveryWrapper(t *testing.T) {
	for _, rest := range []string{
		`time . ./env.sh; ./le spec current`,
		`if true; then . ./env.sh; fi; ./le spec current`,
		`command . x; ./le spec current`,
		`builtin . x; ./le spec current`,
		`{ time . ./env.sh; }; ./le spec current`,
		// AC-29: a value option takes one value, and a tool is matched by
		// its base name.
		`timeout -s KILL 60 env -i ./le x`,
		`/usr/bin/env -i ./le x`,
		`nice -n 5 /usr/bin/sudo le x`,
		`stdbuf -o L env -i le x`,
		// One row per value option of each wrapper (R-9), and H-1: `time`
		// as the external program behind a wrapper.
		`timeout -k 5 60 env -i ./le x`,
		`timeout --signal KILL 60 env -i ./le x`,
		`nice --adjustment 5 env -i ./le x`,
		`stdbuf -i L env -i ./le x`,
		`stdbuf -e L env -i ./le x`,
		`xargs -I x env -i ./le x`,
		`xargs -n 1 env -i ./le x`,
		`xargs -d , env -i ./le x`,
		`command time env -i ./le x`,
		`nice time env -i ./le x`,
		`command time -o t env -i ./le x`,
		`/usr/bin/time -f x env -i ./le x`,
		// H-6: setsid, flock with its lock file and value options, strace
		// and ltrace with theirs, and the options that set the command's
		// environment or user.
		`setsid env -i ./le x`,
		`setsid -f env -i ./le x`,
		`flock l.lock env -i ./le x`,
		`flock -w 5 l.lock env -i ./le x`,
		`flock -E 3 l.lock env -i ./le x`,
		`strace -o out env -i ./le x`,
		`strace -s 64 env -i ./le x`,
		`strace -E A=1 ./le x`,
		`strace --env=A=1 ./le x`,
		`strace -u nobody ./le x`,
		`ltrace -o out env -i ./le x`,
		`ltrace -u nobody ./le x`,
		// The text route, which judges a remainder the parser refuses (the
		// unclosed quote): there a lone `.` after a wrapper holds no digit,
		// so it is the command word and not a skipped duration.
		`time . ./env.sh; ./le spec current 'oops`,
		`nice -n 5 . ./env.sh; ./le spec current 'oops`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" {
			t.Errorf("%q: code=%d output=%q, want exit 2 and no output", rest, code, output)
		}
		if !strings.Contains(message, "another identity") {
			t.Errorf("%q: message does not say the command takes another identity: %s", rest, message)
		}
	}
	for _, rest := range []string{
		`timeout 5 ./le spec current`,
		`timeout 1.5s ./le spec current`,
		`nice -n 5 ./le spec current`,
		// AC-29's survivor and the H-6 wrappers in front of `./le`.
		`timeout -s KILL 60 ./le x`,
		`setsid ./le x`,
		`flock l.lock ./le x`,
		`strace -o out ./le x`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", rest, code, output, message)
		}
	}
}
