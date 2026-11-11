// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
// Related: readerline.go -- the reader-line predicate these tests judge
// Related: afterline.go -- the after-line walk these tests judge
// Related: session_identity_test.go -- runSubagentBash and identityLineOfAgent
package hookruntime

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// readerLineCommands answers the command words the predicate admitted, in
// sorted order, or the construct it named.
func readerLineCommands(command, identity string) ([]string, string) {
	line, outside := judgeReaderLine(command, parseCommandLine(command), identity)
	if outside != "" {
		return nil, outside
	}
	words := make([]string, 0, len(line.commands))
	for _, admitted := range line.commands {
		words = append(words, admitted.words[0])
	}
	slices.Sort(words)
	return words, ""
}

// VALIDATES: AC-5, AC-6 and AC-7 of spec-agent-economy-6-shell-parser. Each
// row is a reader line through one admitted construct, or a reader line the
// byte allow-list or the hand splitter refused. The predicate admits it and
// names exactly the commands it admitted, so a construct whose body the walk
// skipped shows as a missing command, and a subagent runs it through Run
// without the identity line, with empty standard output. The rows hold the
// spec's construct lines verbatim, so a recorded row this spec loosens can
// name this test as its proof.
// PREVENTS: a reader loop or conditional paying the identity line and every
// guard's text verdict (Task, findings 1 and 2), and a construct being
// admitted without its commands being walked.
func TestReaderLineReachesThroughConstructs(t *testing.T) {
	for _, row := range []struct {
		command string
		// admitted is the sorted command words the predicate must admit.
		admitted []string
	}{
		// AC-5, the parity rows, and the journal row of 2026-11-11.
		{`for f in a b; do grep -n "x\|go vet ./y" "$f"; done`, []string{"grep"}},
		{`while read -r f; do grep -c 'x;go test ./y' "$f"; done < list`, []string{"grep", "read"}},
		{`for f in a b; do grep -c 'x; go test ./y | tail -1' "$f"; done`, []string{"grep"}},
		{`for f in a b; do grep -n 'go build ./cmd/ze' "$f"; done`, []string{"grep"}},
		{`for f in a b; do grep -n 'cat /tmp/x' "$f"; done`, []string{"grep"}},
		{`for p in a b; do grep -c x a | tail -3; done`, []string{"grep", "tail"}},
		// AC-6, one row per admitted construct.
		{`if [ -f a ]; then cat a; fi`, []string{"[", "cat"}},
		{`if [ -f x ]; then cat x; elif [ -d x ]; then ls x; else echo none; fi`, []string{"[", "[", "cat", "echo", "ls"}},
		{`while read -r f; do cat "$f"; done < list`, []string{"cat", "read"}},
		{`until true; do cat a; done`, []string{"cat", "true"}},
		{`case x in a) cat a;; esac`, []string{"cat"}},
		{`case "$x" in a|b) echo a;; *.go) echo b;; esac`, []string{"echo", "echo"}},
		{`[[ -f "$f" ]] && cat "$f"`, []string{"cat"}},
		{`[[ -f "$f" && "$f" == *.go ]] && cat "$f"`, []string{"cat"}},
		{`[[ $x =~ ^a ]] || [[ a < b ]]`, []string{}},
		{`{ cat a; cat b; }`, []string{"cat", "cat"}},
		{`( cat a )`, []string{"cat"}},
		{`( cd x && ls )`, []string{"cd", "ls"}},
		{`! grep -q x a`, []string{"grep"}},
		{`time grep -c x a`, []string{"grep"}},
		{`echo "$(cat a)"`, []string{"cat", "echo"}},
		{`echo "$(cat f)" ` + "`pwd`", []string{"cat", "echo", "pwd"}},
		{"cat <<EOF\nx\nEOF", []string{"cat"}},
		{"cat <<EOF\nx $(echo y)\nEOF", []string{"cat", "echo"}},
		{"cat <<'EOF'\n$(go test ./x)\nEOF", []string{"cat"}},
		{`diff <(sort a) <(sort b)`, []string{"diff", "sort", "sort"}},
		// GNU cmp, every option of cmp(1) 3.10 (journal row of 2026-11-11,
		// `bashTestDeletion` line-wide words).
		{`cmp a b`, []string{"cmp"}},
		{`cmp -s a b && cmp -bl -i 10:20 -n 1K a b`, []string{"cmp", "cmp"}},
		{`cmp --quiet --bytes 5 --ignore-initial=4 - b 3 4`, []string{"cmp"}},
		{`n=3; echo "$n" "${n:-0}" "${n#x}" "${n,,}" "${#n}" "${n/a/b}"`, []string{"echo"}},
		// AC-7: what the byte allow-list or the splitter refused.
		{`grep -n '^##' f`, []string{"grep"}},
		{`git show HEAD | grep -n '^##'`, []string{"git", "grep"}},
		{`grep -n '{x}' f`, []string{"grep"}},
		{`grep -n x{ f`, []string{"grep"}},
		{`grep -n '!x' f`, []string{"grep"}},
		{`grep -n x\ y f`, []string{"grep"}},
		{`grep -n 'é' f`, []string{"grep"}},
		{`grep -n '' f`, []string{"grep"}},
		{`grep -e '' 'go build ./cmd/z'`, []string{"grep"}},
		{`grep -n "foo$" f`, []string{"grep"}},
		{`< f grep -n x`, []string{"grep"}},
		{`>/dev/null grep x f`, []string{"grep"}},
		{`[ -f x ] 2>/dev/null && cat x`, []string{"[", "cat"}},
		{`[ ! -f x ] && cat y`, []string{"[", "cat"}},
		// Red walk: a judge handed a redirection's target as an argument.
		{`sort f | uniq -c 2>/dev/null`, []string{"sort", "uniq"}},
		{`cat x 2>&1 >&2 <&0 >&-`, []string{"cat"}},
		{`cat x &>/dev/null; cat y >>/dev/null`, []string{"cat", "cat"}},
		{`grep x <<< "$y"`, []string{"grep"}},
		// The splitter-fault lines that are reader lines read from the tree.
		{`cat x\ y`, []string{"cat"}},
		{`cat a{b}`, []string{"cat"}},
		{`cat a#b`, []string{"cat"}},
		{`cat '#'`, []string{"cat"}},
		{`echo "a\"b"`, []string{"echo"}},
		{"cat caf\xc3\xa9", []string{"cat"}},
		{`cat a |& cat b`, []string{"cat", "cat"}},
		{`cat <(cat a)`, []string{"cat", "cat"}},
		{`cat ${x:-a}`, []string{"cat"}},
		{`cat "${x:-a}"`, []string{"cat"}},
		{`cat ${x}`, []string{"cat"}},
		{`[[ -f a ]]`, []string{}},
		{"cat 'a\nb'", []string{"cat"}},
		{`<<<cat a`, []string{"a"}},
		{`2>/dev/null cat a`, []string{"cat"}},
		{`cat <f a`, []string{"cat"}},
		{`cat a >/dev/null b`, []string{"cat"}},
		{`cat '' a`, []string{"cat"}},
		{"cat x\x01", []string{"cat"}},
		// Recorded rows the splitter refused for a subagent: a reader loop,
		// a here-document whose body names `./le`, which is data to cat and
		// runs nothing, and loops over a list read from a file.
		{`for f in a b; do cat "$f"; done`, []string{"cat"}},
		{"cat <<EOF\n./le spec current\nEOF", []string{"cat"}},
		{`while read -r f; do echo $f; done < tmp/list`, []string{"echo", "read"}},
		{`while read -r line; do echo no-sleep-here; done < tmp/list`, []string{"echo", "read"}},
	} {
		if row.command == `<<<cat a` {
			// The here-string's word is `cat`; the command is `a`, which is no
			// reader, so this row is refused: it stands here to show the
			// redirection is a node and its target never a command word.
			if _, outside := readerLineCommands(row.command, ""); !strings.Contains(outside, "`a`, which is outside the reader set") {
				t.Errorf("%q: refused for %q, want the command `a` named", row.command, outside)
			}
			continue
		}
		admitted, outside := readerLineCommands(row.command, "")
		if outside != "" {
			t.Errorf("%q: not a reader line: %s", row.command, outside)
			continue
		}
		if !slices.Equal(admitted, row.admitted) {
			t.Errorf("%q: admitted %q, want %q: the walk skipped or invented a command", row.command, admitted, row.admitted)
		}
		code, output, message := runSubagentBash(t, row.command)
		if code != 0 {
			t.Errorf("%q: a subagent is refused with code %d: %s", row.command, code, message)
		}
		if output != "" {
			t.Errorf("%q: standard output = %q, want none: the hook never changes a command", row.command, output)
		}
	}
	// AC-14: the agent's own exact line in front leaves the line a reader
	// line for the guards, and its two exports are not judged as commands.
	command := identityLineOfAgent + `for f in a; do grep -n 'go vet ./y' "$f"; done`
	admitted, outside := readerLineCommands(command, identityLineOfAgent)
	if outside != "" {
		t.Errorf("%q: not a reader line after the agent's own line: %s", command, outside)
	}
	if !slices.Equal(admitted, []string{"grep"}) {
		t.Errorf("%q: admitted %q, want only grep", command, admitted)
	}
}

// VALIDATES: AC-8, AC-30, AC-7's ANSI-C survivor and AC-12 at the predicate.
// Each row holds one node outside the table, alone or inside an admitted
// construct. The predicate names it in words that name what the line holds,
// and a subagent is refused through Run with that name and the identity
// line to type. The heavy rows put a raw job where a walk that skipped a
// body, a substitution or a here-document would miss it.
// PREVENTS: R-1, a construct admitted by mistake, which lets a subagent's
// `le` call act as the main thread with no identity line, and a refusal
// naming a construct the line does not hold.
func TestReaderLineRefusesWhatCanRunAnythingElse(t *testing.T) {
	t.Setenv("zehooklowervar", "1")
	for _, row := range []struct {
		command string
		// named is a substring the refusal must hold.
		named string
	}{
		{`f() { cat x; }; f`, "a function definition"},
		{`export x=1; cat x`, "`export`"},
		{`declare a=1`, "`declare`"},
		{`local a=1`, "`local`"},
		{`readonly a=1`, "`readonly`"},
		{`typeset a=1`, "`typeset`"},
		{`let a=1`, "arithmetic"},
		{`echo $((1+2))`, "arithmetic"},
		{`(( a = 1 ))`, "arithmetic"},
		{`for ((i=0; i<1; i++)); do cat x; done`, "arithmetic `for (( ))`"},
		{`select f in a; do cat x; done`, "`select`"},
		{`echo "${a[0]}"`, "an array index"},
		{`echo "${a:1:2}"`, "a slice"},
		{`echo "${!a}"`, "an indirect expansion"},
		{`echo "${!a*}"`, "variable names"},
		{`echo "${a:=x}"`, "assigns"},
		{`echo "${a?x}"`, "assigns, stops the shell"},
		{`echo "${a@P}"`, "prompt string"},
		{`cat x &`, "a background `&`"},
		{`coproc cat x`, "a coprocess"},
		{`LC_ALL=C sort f`, "in front of a command"},
		{`x=1 cat f`, "in front of a command"},
		{`PATH=.`, "not lower case"},
		{`IFS=x`, "not lower case"},
		{`zehooklowervar=2`, "the environment already holds"},
		{`a+=x`, "an appending assignment"},
		{`a[1]=x`, "an array element"},
		{`a=(x y)`, "an array assignment"},
		{`for PATH in a; do cat x; done`, "the loop variable `PATH`"},
		{`[[ a -eq b ]]`, "`-eq`"},
		{`[[ -v a ]]`, "`-v`"},
		{`[[ -o posix ]]`, "`-o`"},
		{`read -a arr`, "`read -a arr`"},
		{`read -n 1 x`, "`read -n 1 x`"},
		{`read -r PATH`, "`read -r PATH`"},
		{`read -r`, "`read -r`"},
		{`cat x > out`, outsideFileWrite},
		{`cat x >> out`, outsideFileWrite},
		{`cat x &> out`, outsideFileWrite},
		{`cat x >| out`, outsideFileWrite},
		{`cat x 2> out`, outsideFileWrite},
		{`cat x >& out`, outsideFileWrite},
		{`cat x <> f`, outsideFileWrite},
		{`cat x > "$f"`, outsideFileWrite},
		{`while true; do cat x; done > out`, outsideFileWrite},
		{`cat x {fd}>/dev/null`, "assigns a descriptor to a variable"},
		{`echo >(cat)`, "an output process substitution"},
		{`cat @(a|b)`, "an extended glob"},
		// AC-7's survivor: ANSI-C and locale quoting, in those words.
		{`grep x $'a\tb'`, "its value is not read"},
		{`grep x $"a"`, "its value is not read"},
		// A byte the parser drops and bash keeps, and a cut-short line.
		{"cat x\r\ncat y", "a carriage return"},
		{"cat x\rcat y", "a carriage return"},
		{`cat notes.md \`, "a backslash that ends the line"},
		// AC-12 at the predicate.
		{`cat 'x`, "does not parse as bash"},
		// Red walk: a body, a substitution, a here-document or a test word
		// the walk skips lets the raw job pass.
		{`for f in a; do go test ./x; done`, "`go`, which is outside the reader set"},
		{`if true; then go vet ./x; fi`, "`go`, which is outside the reader set"},
		{`while go test ./x; do echo x; done`, "`go`, which is outside the reader set"},
		{`case x in a) go test ./x;; esac`, "`go`, which is outside the reader set"},
		{`{ go vet ./x; }`, "`go`, which is outside the reader set"},
		{`( go test ./x )`, "`go`, which is outside the reader set"},
		{`! golangci-lint run`, "`golangci-lint`, which is outside the reader set"},
		{`time go test ./x`, "`go`, which is outside the reader set"},
		{`echo "$(go test ./x)"`, "`go`, which is outside the reader set"},
		{"echo `go test ./x`", "`go`, which is outside the reader set"},
		{`x=$(go test ./x)`, "`go`, which is outside the reader set"},
		{`diff <(go test ./x) a`, "`go`, which is outside the reader set"},
		{`echo "${a:-$(go test ./x)}"`, "`go`, which is outside the reader set"},
		{`[[ -n $(go test ./x) ]]`, "`go`, which is outside the reader set"},
		{`cat < "$(go test ./x)"`, "`go`, which is outside the reader set"},
		{"cat <<EOF\n$(go test ./x)\nEOF", "`go`, which is outside the reader set"},
		{`for f in $(go test ./x); do cat f; done`, "`go`, which is outside the reader set"},
		{`$(echo go) test ./x`, "outside the reader set"},
		{`eval cat x`, "`eval`, which is outside the reader set"},
		{`bash -c 'cat x'`, "`bash`, which is outside the reader set"},
		// cmp's judge is an allow-list: an option cmp(1) does not give, an
		// abbreviation getopt would expand, and a word the shell expands.
		{`cmp -x a b`, "only with the options its judge lists"},
		{`cmp --frobnicate a b`, "only with the options its judge lists"},
		{`cmp --print a b`, "only with the options its judge lists"},
		{`cmp $f b`, "only with the options its judge lists"},
		{strings.Repeat("{ ", 70) + "cat x" + strings.Repeat("; }", 70), "nested deeper"},
	} {
		_, outside := readerLineCommands(row.command, "")
		if outside == "" {
			t.Errorf("%q: admitted as a reader line, want refused for %q", row.command, row.named)
			continue
		}
		if !strings.Contains(outside, row.named) {
			t.Errorf("%q: refused for %q, want a refusal naming %q", row.command, outside, row.named)
		}
		code, _, message := runSubagentBash(t, row.command)
		if code != 2 {
			t.Errorf("%q: a subagent's code = %d, want 2", row.command, code)
		}
		if !strings.Contains(message, row.named) {
			t.Errorf("%q: the refusal does not name %q: %s", row.command, row.named, message)
		}
		if !strings.Contains(message, identityLineOfAgent) {
			t.Errorf("%q: the refusal does not print the identity line to type: %s", row.command, message)
		}
	}
}

// VALIDATES: the identity-line prefix is read only when the parse agrees it
// is the agent's exports. A line that starts with the agent's own text but
// whose statements inside it are not its exports, and a line with another
// agent's line, are not lifted for the guards.
// PREVENTS: the prefix strip reading a reader line after text the shell
// runs as something else.
func TestReaderLineAfterIdentityLineNeedsTheAgentsExports(t *testing.T) {
	for _, command := range []string{
		"export ZE_AGENT_ID=agent-2; export CLAUDE_CODE_SESSION_ID=parent-17; grep -n 'go vet' f",
		identityLineOfAgent + "export x=1; grep -n x f",
	} {
		if _, outside := readerLineCommands(command, identityLineOfAgent); outside == "" {
			t.Errorf("%q: admitted as a reader line after the identity line", command)
		}
	}
}

// VALIDATES: AC-5 through Run, the spec's wiring row. A subagent's reader
// loop whose grep pattern holds a quoted `go vet` after a bar passes with no
// identity line, exit 0 and empty standard output, and so does the same loop
// for the main thread; the loop that runs the vet is refused.
// PREVENTS: the journal row of 2026-11-11: a loop of readers refused as a raw
// job because the text split cut its pattern at the quoted bar.
func TestSubagentReaderLoopPassesUnchanged(t *testing.T) {
	loop := `for f in a b; do grep -n "x\|go vet ./y" "$f"; done`
	code, output, message := runSubagentBash(t, loop)
	if code != 0 || output != "" {
		t.Errorf("subagent %q: code=%d output=%q message=%s, want exit 0 and no output", loop, code, output, message)
	}
	if code, message := runMainBash(t, loop); code != 0 {
		t.Errorf("main %q: code=%d message=%s, want exit 0", loop, code, message)
	}
	code, _, message = runSubagentBash(t, `for f in a b; do go vet ./y; done`)
	if code != 2 || !strings.Contains(message, identityLineOfAgent) {
		t.Errorf("a loop that runs go vet: code=%d, want 2 with the identity line: %s", code, message)
	}
}

// VALIDATES: AC-14 through Run. A subagent's command that starts with its
// exact identity line, followed by a reader line holding a quoted `go vet`, a
// quoted `go build` or a /tmp path, passes every guard, as the same line does
// without the identity line in front; another agent's line in front, and an
// export after the line, are refused.
// PREVENTS: the identity line, which the agent is asked to type, turning a
// reader line into one judged by every guard's text.
func TestIdentityLineBeforeAReaderLineStillLifts(t *testing.T) {
	for _, rest := range []string{
		`for f in a b; do grep -n "x\|go vet ./y" "$f"; done`,
		`grep -n 'go build ./cmd/ze' f`,
		`grep -n 'cat /tmp/x' f`,
		`grep -c 'x;go test ./y | tail -1' f`,
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 0 || output != "" {
			t.Errorf("%q: code=%d output=%q message=%s, want exit 0 and no output", rest, code, output, message)
		}
	}
	for _, command := range []string{
		"export ZE_AGENT_ID=agent-2; export CLAUDE_CODE_SESSION_ID=parent-17; grep -n 'go build ./cmd/ze' f",
		identityLineOfAgent + "export X=1; grep -n 'go build ./cmd/ze' f",
	} {
		if code, _, _ := runSubagentBash(t, command); code != 2 {
			t.Errorf("%q: code=%d, want 2", command, code)
		}
	}
}

// VALIDATES: AC-15. A refused piped command on a line that parses is advised
// from its own parsed words, each written back as one shell word: after a
// quoted `#`, before a redirection, which is never one of its words, and
// inside a group, which only the tree judgment refuses (AC-9, AC-10): the
// text split cuts the group at the quoted bar into pieces that start with
// `{` and with `TestB'`, and neither guard's text judgment refuses either.
// PREVENTS: the advice being cut at a quoted bar, or carrying `2>&1` as an
// argument of the advised command.
func TestRefusalAdvisesTheParsedCommand(t *testing.T) {
	advised := "quiet command go test -run 'TestA|TestB' ./x"
	for _, command := range []string{
		`grep -n '#' f; go test -run 'TestA|TestB' ./x | tail -5`,
		`go test -run 'TestA|TestB' ./x 2>&1 | tail -5`,
		`{ go test -run 'TestA|TestB' ./x | tail -5; }`,
	} {
		code, message := runMainBash(t, command)
		if code != 2 {
			t.Errorf("%q: code=%d, want 2", command, code)
			continue
		}
		if !strings.Contains(message, advised) {
			t.Errorf("%q: the advice does not hold the parsed command %q: %s", command, advised, message)
		}
		if strings.Contains(message, advised+" 2>&1") {
			t.Errorf("%q: the advice carries the redirection as a word: %s", command, message)
		}
	}
}

// VALIDATES: AC-16. A build on a reader+build line whose other commands are
// reader constructs, or whose redirection is written before its words, is
// judged by its words; a build inside a construct keeps the text judgment,
// which refuses a build with no -o.
// PREVENTS: a reader loop beside a build sending the build to the text
// judgment, and a redirection before the words hiding the build.
func TestRootBuildReadsTheBuildFromTheTree(t *testing.T) {
	root := goBuildTree(t)
	judgeGuardRows(t, root, []guardRow{
		{"build", bashRootBuild, `cat a; go build -o bin/z ./cmd/z`, false},
		{"build", bashRootBuild, `2>/dev/null go build -o bin/z ./cmd/z`, false},
		{"build", bashRootBuild, `for f in a; do cat "$f"; done; go build -o bin/z ./cmd/z`, false},
		{"build", bashRootBuild, `go build ./cmd/z > /dev/null`, true},
		{"build", bashRootBuild, `for f in a; do go build ./cmd/z; done`, true},
		{"build", bashRootBuild, `go build -o bin/$X ./cmd/z; cat a > f`, true},
	})
}

// VALIDATES: AC-17 and fail-open hole H-2. On a reader line the /tmp guard
// judges every word of the tree: a loop list, a case word, an assignment
// value, a redirection target and a `[[` operand, and a word the shell
// expands also by its typed text; a here-document body, a here-string and a
// /tmp under a literal parent pass. The lines that name /tmp through a loop
// list, a here-string or an expanded parent (AC-35) are reader lines, so the
// refusal they keep is the /tmp guard's own, never the identity line's.
// PREVENTS: `cat foo$D/tmp/x` passing because the value drops `$D`.
func TestSystemTmpJudgesEveryWordOfAReaderLine(t *testing.T) {
	for _, line := range []string{
		`for f in /tmp/a; do cat "$f"; done`,
		`cat <<< /tmp/a`,
		`cat ${v}/tmp`,
		`cat ${v}/tmp/x`,
		`ls ${HOME}/tmp`,
	} {
		if _, outside := readerLineCommands(line, ""); outside != "" {
			t.Errorf("%q: not a reader line: %s", line, outside)
		}
	}
	judgeGuardRows(t, t.TempDir(), []guardRow{
		{"tmp", bashSystemTmp, `cat ${v}/tmp`, true},
		{"tmp", bashSystemTmp, `cat ${v}/tmp/x`, true},
		{"tmp", bashSystemTmp, `ls ${HOME}/tmp`, true},
		{"tmp", bashSystemTmp, `for f in /tmp/a; do cat "$f"; done`, true},
		{"tmp", bashSystemTmp, `case /tmp/a in x) cat x;; esac`, true},
		{"tmp", bashSystemTmp, `x=/tmp/a; cat "$x"`, true},
		{"tmp", bashSystemTmp, `cat < /tmp/a`, true},
		{"tmp", bashSystemTmp, `[[ -f /tmp/a ]]`, true},
		{"tmp", bashSystemTmp, `cat foo$D/tmp/x`, true},
		{"tmp", bashSystemTmp, `cat <<< /tmp/a`, false},
		{"tmp", bashSystemTmp, "cat <<EOF\n/tmp/a\nEOF", false},
		{"tmp", bashSystemTmp, `grep -n 'cat /tmp/x' f`, false},
		{"tmp", bashSystemTmp, `cat x/tmp/y`, false},
	})
}

// VALIDATES: AC-18 and fail-open hole H-3. The deletion guard reads a plain
// rm line from the tree, judging only the rm arguments; an rm inside a
// construct, with an expanded word or beside a cd keeps the line-wide
// judgment; a here-document body's words reach the line-wide list; a line
// that does not parse is judged by its text.
// PREVENTS: `bash <<EOF` with `rm a_test.go` in its body passing because the
// body is one word of the tree.
func TestTestDeletionReadsThePlainRmFromTheTree(t *testing.T) {
	judgeGuardRows(t, t.TempDir(), []guardRow{
		{"deletion", bashTestDeletion, `rm -f scratch && grep x internal/a_test.go`, false},
		{"deletion", bashTestDeletion, `rm -f scratch; cat internal/a_test.go 2>/dev/null`, false},
		{"deletion", bashTestDeletion, `( rm -f scratch ); grep x internal/a_test.go`, true},
		{"deletion", bashTestDeletion, `rm -f $f && grep x internal/a_test.go`, true},
		{"deletion", bashTestDeletion, `cd x && rm -f scratch && grep x internal/a_test.go`, true},
		{"deletion", bashTestDeletion, "bash <<EOF\nrm a_test.go\nEOF", true},
		// Review B5: a body is split as shell text, so a quote, a `;` or a
		// bracket around the rm does not hide it.
		{"deletion", bashTestDeletion, "bash <<EOF\nrm \"internal/a_test.go\"\nEOF", true},
		{"deletion", bashTestDeletion, "bash <<EOF\nrm internal/a_test.go;\nEOF", true},
		{"deletion", bashTestDeletion, "bash <<EOF\n(rm internal/a_test.go)\nEOF", true},
		{"deletion", bashTestDeletion, `rm a_test.go 'oops`, true},
		{"deletion", bashTestDeletion, `find . -name a_test.go -delete; fi`, true},
		// The journal row of 2026-11-11: cmp is a reader, so a test path it
		// compares beside an rm of a scratch file is no rm argument, and an rm
		// of the test path it compared is still refused.
		{"deletion", bashTestDeletion, `cmp tmp/x internal/a_test.go; rm tmp/x`, false},
		{"deletion", bashTestDeletion, `cmp -s tmp/x internal/a_test.go && rm -f tmp/x`, false},
		{"deletion", bashTestDeletion, `cmp x internal/a_test.go; rm internal/a_test.go`, true},
		{"deletion", bashTestDeletion, `cmp --frobnicate tmp/x internal/a_test.go; rm tmp/x`, true},
	})
}

// VALIDATES: AC-19 and fail-open holes H-4, H-5 and H-6. After the agent's
// own identity line, a declaration, an environment tool, a builtin that
// assigns a name a value can build, arithmetic, a command word the shell
// expands, a substitution that runs an environment tool, and a startup
// variable in front of a command are each refused as taking another
// identity, by the node's kind and the command word; the plain forms pass.
// PREVENTS: `n=ZE_AGENT_""ID; read -r "$n" <<< x; ./le x` and the other
// forms that build an identity variable's name from pieces.
func TestAnotherIdentityIsJudgedByTheNode(t *testing.T) {
	for _, rest := range []string{
		`export X=1`,
		`declare -x X`,
		`env X=1 ./le x`,
		`n=x; read -r "$n" <<< y; ./le x`,
		`printf -v "$n" x; ./le x`,
		`mapfile a <<< x; ./le x`,
		`readarray a <<< x; ./le x`,
		`getopts a b; ./le x`,
		`trap 'x' EXIT; ./le x`,
		`enable -f lib.so x; ./le x`,
		`let a=1; ./le x`,
		`(( a = 1 )); ./le x`,
		`echo $(( a = 1 )); ./le x`,
		`c=env; $c -i ./le x`,
		`x=$(env -i ./le x)`,
		`./le x "$(env -i ./le y)"`,
		`echo "${a:=x}"; ./le x`,
		`BASH_ENV=./x.sh ./le x`,
		`ENV=./x.sh ./le x`,
		`SHELLOPTS=x ./le x`,
		`BASHOPTS=x ./le x`,
		`PROMPT_COMMAND=x ./le x`,
		`BASH_FUNC_f=x ./le x`,
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
		`FOO=bar ./le x`,
		`cd x && ./le x`,
		`grep ZE_AGENT_ID f`,
		`read -r line <<< x; ./le x`,
	} {
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want the plain form to pass", rest)
		}
	}
}

// VALIDATES: AC-31, both directions. After the agent's own identity line a
// `set` whose options change no variable passes the identity check, and every
// other `set` is refused.
// PREVENTS: `set -euo pipefail` paying a refusal, and `set -a` or `set -k`
// passing, which move later assignments into the environment.
func TestSetOptionsThatChangeNoVariablePass(t *testing.T) {
	for _, rest := range []string{
		`set -o pipefail; ./le x`, `set -e; ./le x`, `set -eu; ./le x`,
		`set -x; ./le x`, `set +e; ./le x`, `set -euo pipefail; ./le x`,
	} {
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: refused, want it to pass: the options change no variable", rest)
		}
	}
	for _, rest := range []string{
		`set -a; ./le x`, `set -k; ./le x`, `set -- a b; ./le x`, `set -o allexport; ./le x`,
		`set; ./le x`, `set -o; ./le x`, `set -e "$x"; ./le x`, `set -ea; ./le x`,
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || !strings.Contains(message, "another identity") {
			t.Errorf("%q: code=%d, want it refused as taking another identity: %s", rest, code, message)
		}
	}
}

// deletedSplitterNames are the declarations phase 3 of
// spec-agent-economy-6-shell-parser deletes with the hand splitter and the
// byte allow-list.
var deletedSplitterNames = []string{
	"lineCommands", "lineWords", "shellLine", "modeledSyntax", "unmodeledSyntax", "syntaxCommand",
	"rawWords", "transparentWords", "remainderChangesEnvironment", "writesOnlyDevNull", "cdFollowed",
	"pipelineExpensive", "plainReader", "separatorAt", "expandsUnquoted", "tildeExpands",
}

// VALIDATES: AC-3 and AC-4. No file of the package, its tests included,
// declares a name of the hand splitter or the byte allow-list; the files
// that held them hold no declaration; and one function calls the parser.
// PREVENTS: the splitter kept beside the parser (ai/rules/no-layering.md).
func TestHandSplitterIsGone(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	parsers := 0
	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		if oneOf(name, "shellline.go", "shellsyntax.go") && len(file.Decls) != 0 {
			t.Errorf("%s still declares %d names: the commit deletes it", name, len(file.Decls))
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncDecl:
				if slices.Contains(deletedSplitterNames, node.Name.Name) {
					t.Errorf("%s declares %s, which the parser replaced", name, node.Name.Name)
				}
			case *ast.TypeSpec:
				if slices.Contains(deletedSplitterNames, node.Name.Name) {
					t.Errorf("%s declares the type %s, which the parser replaced", name, node.Name.Name)
				}
			case *ast.ValueSpec:
				for _, ident := range node.Names {
					if slices.Contains(deletedSplitterNames, ident.Name) {
						t.Errorf("%s declares %s, which the parser replaced", name, ident.Name)
					}
				}
			case *ast.SelectorExpr:
				if node.Sel.Name == "NewParser" && !strings.HasSuffix(name, "_test.go") {
					parsers++
				}
			}
			return true
		})
	}
	if parsers != 1 {
		t.Errorf("the package calls syntax.NewParser %d times outside its tests, want once (parseBash)", parsers)
	}
	for _, name := range []string{"shellline.go", "shellsyntax.go"} {
		if _, err := os.Stat(name); err == nil {
			t.Logf("%s still exists as an empty stub; the commit's remove deletes it", name)
		}
	}
}

// VALIDATES: review B1 of spec-agent-economy-6-shell-parser phase 3. A
// wrapper that ends the line on an option that takes a value (`xargs -n`,
// `timeout -s`) leaves no command word: the after-line walk answers that
// without reading past the words, and a command after it is still judged, so
// an environment tool after it is refused and the git guard still refuses a
// destructive verb on the same line.
// PREVENTS: the walk indexing one word past the line, a panic Run's recover
// answers with exit 0, which skips every check after the identity check.
func TestWrapperEndingOnAValueOptionIsJudged(t *testing.T) {
	for _, wrapper := range []string{
		`xargs -n`, `timeout -s`, `nice -n`, `stdbuf -o`, `strace -o`, `ltrace -o`, `flock -w`, `command time -o`,
	} {
		command := identityLineOfAgent + wrapper
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want a wrapper with no command after it to pass", wrapper)
		}
		refused := identityLineOfAgent + wrapper + "; env -i ./le x"
		if code, _, message := runSubagentBash(t, refused); code != 2 || !strings.Contains(message, "another identity") {
			t.Errorf("%q: code=%d, want the env after it refused as taking another identity: %s", refused, code, message)
		}
	}
	if code, message := runMainBash(t, "git reset --hard; xargs -n"); code != 2 {
		t.Errorf("main `git reset --hard; xargs -n`: code=%d, want the destructive git guard to refuse it: %s", code, message)
	}
	if code, _, message := runSubagentBash(t, identityLineOfAgent+"git reset --hard; xargs -n"); code != 2 {
		t.Errorf("subagent `git reset --hard; xargs -n` after its line: code=%d, want the destructive git guard to refuse it: %s", code, message)
	}
}

// linesBashReadsOtherwise are lines whose tree, as the parser builds it,
// names fewer commands than bash runs: a comment ending in a backslash, whose
// newline the lexer joins and bash does not (review B2); a `$` followed by a
// backslash-newline, which bash joins into a substitution and the parser
// reads as text, in double quotes, in a here-document body and in a default
// (B3); and a here-document whose delimiter line bash joins, so the body ends
// earlier (I1). identityForm is the same shape with an identity assignment as
// the hidden command, and textRoute marks a row the guards read by its text.
var linesBashReadsOtherwise = []struct {
	line, identityForm string
	textRoute          bool
}{
	{"echo a # note \\\ncat x", "echo a # note \\\nZE_AGENT_ID=x cat x", true},
	{"echo \"$\\\n(cat x)\"", "echo \"$\\\n(ZE_AGENT_ID=x cat x)\"", true},
	{"cat <<EOF\n$\\\n(head x)\nEOF", "cat <<EOF\n$\\\n(ZE_AGENT_ID=x head x)\nEOF", true},
	{"echo ${x:-$\\\n(cat y)}", "echo ${x:-$\\\n(ZE_AGENT_ID=x cat y)}", true},
	{"cat <<EOF\nEO\\\nF\nhead x\nEOF", "cat <<EOF\nEO\\\nF\nZE_AGENT_ID=x head x\nEOF", false},
}

// VALIDATES: review B2, B3 and I1 of phase 3. For each line of
// linesBashReadsOtherwise, a real bash runs a command the tree does not name
// (so the row shows a real disagreement), and the hook does not read the line
// from its tree: it is no reader line, a text-route row is not read by the
// deletion and build guards' parse either, and after the agent's identity
// line the hidden identity assignment is refused.
// METHOD: runBashLine, as TestParsedCommandsMatchBash runs the corpus, then
// the predicate, deletionLineReadable and takesAnotherIdentity.
// PREVENTS: `echo a # note \` then `./le spec release` on the next line, or
// `echo "$\` then `(./le spec release)"`, passing as one echo.
func TestLinesBashReadsOtherwiseAreNotRead(t *testing.T) {
	bash := lookBash(t)
	for _, row := range linesBashReadsOtherwise {
		parse := parseCommandLine(row.line)
		if !parse.parsed() {
			t.Errorf("%q did not parse (%s); the row needs a line the parser reads otherwise than bash", row.line, parse.failure)
			continue
		}
		shape := analyseBashLine(parse.file)
		if shape.skip != "" {
			t.Errorf("%q: the comparison cannot run it (%s)", row.line, shape.skip)
			continue
		}
		view, reason := runBashLine(t, bash, row.line, shape)
		if reason != "" {
			t.Errorf("%q: bash did not run it (%s)", row.line, reason)
			continue
		}
		hidden := len(view.notFound) > 0
		for _, argv := range view.calls {
			if !slices.Contains(shape.names, argv[0]) {
				hidden = true
			}
		}
		if !hidden {
			t.Errorf("%q: bash ran only commands the tree names (%q), so the row shows no disagreement", row.line, shape.names)
		}
		if _, outside := judgeReaderLine(row.line, parse, ""); outside == "" {
			t.Errorf("%q: a reader line, want it outside: bash runs a command its tree does not name", row.line)
		}
		if row.textRoute && deletionLineReadable(row.line, parse) {
			t.Errorf("%q: the deletion and build guards read it from its tree, want the text route", row.line)
		}
		command := identityLineOfAgent + row.identityForm
		if !takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: passes after the identity line, want the hidden identity assignment refused", row.identityForm)
		}
	}
}

// VALIDATES: review I1 of phase 3. A here-document stays on a reader line only
// when its delimiter is plain text and, for a body the shell expands, the body
// holds no backslash-newline; and no body line ends with the delimiter after
// other text. The plain forms stay reader lines.
// PREVENTS: `cat <<$x`, a joined delimiter line and a `${x}EOF` body line
// ending the body where bash and the parser disagree.
func TestHereDocumentOnAReaderLineHasAPlainDelimiter(t *testing.T) {
	for _, line := range []string{
		"cat <<$x\nhi\n$x",
		"cat <<EOF\nEO\\\nF\n./le spec release\nEOF",
		"cat <<EOF\n${x}EOF\n./le spec release\nEOF",
		"cat <<EOF\nxEOF\nEOF",
	} {
		if _, outside := judgeReaderLine(line, parseCommandLine(line), ""); outside == "" {
			t.Errorf("%q: a reader line, want the here-document outside", line)
		}
	}
	for _, line := range []string{
		"cat <<EOF\nx\nEOF", "cat <<'EOF'\n$x\nEOF", "cat <<\"EOF\"\nx\nEOF", "cat <<-EOF\n\tx\n\tEOF",
	} {
		if _, outside := judgeReaderLine(line, parseCommandLine(line), ""); outside != "" {
			t.Errorf("%q: outside (%s), want a reader line", line, outside)
		}
	}
}

// VALIDATES: review B4 of phase 3. After the agent's identity line, an
// assignment to either identity variable in front of a command is refused,
// and a reader's mentions are counted from its own words and here-document
// body, never from the span the parser gives a statement with a here-document,
// which runs past the commands typed after it on the same line.
// PREVENTS: `cat <<EOF && ZE_AGENT_ID=other ./le spec claim x` passing because
// the cat's span reached the assignment.
func TestReaderMentionsAreCountedFromItsOwnWords(t *testing.T) {
	for _, rest := range []string{
		"cat <<EOF && ZE_AGENT_ID=other ./le spec claim x\nhi\nEOF",
		"cat <<EOF && CLAUDE_CODE_SESSION_ID=other ./le spec claim x\nhi\nEOF",
		"cat <<EOF; ZE_AGENT_ID=other ./le spec claim x\nhi\nEOF",
		"ZE_AGENT_ID=other ./le spec claim x",
	} {
		code, output, message := runSubagentBash(t, identityLineOfAgent+rest)
		if code != 2 || output != "" || !strings.Contains(message, "another identity") {
			t.Errorf("%q: code=%d output=%q, want it refused as taking another identity: %s", rest, code, output, message)
		}
	}
	for _, rest := range []string{
		"cat <<EOF\nZE_AGENT_ID\nEOF",
		"cat <<< ZE_AGENT_ID",
		"grep -n ZE_AGENT_ID f | head -3",
		`echo "$(grep ZE_AGENT_ID f)"`,
	} {
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want a reader naming the variable to pass", rest)
		}
	}
}

// VALIDATES: review I2 of phase 3. After the agent's identity line, an
// expansion or a test that evaluates an array subscript or an arithmetic
// operand (`${a[...]}`, a slice, a `[[` arithmetic comparison, `test -v` or
// `-R`, and a test whose operator the shell expands) is refused, because it
// can assign a variable whose name a value builds; the plain tests pass.
// PREVENTS: `n=ZE_AGENT; n=${n}_ID; echo ${a[$n=7]}; ./le x` passing.
func TestArithmeticAfterTheLineIsRefused(t *testing.T) {
	const built = "n=ZE_AGENT; n=${n}_ID; "
	for _, rest := range []string{
		`echo ${a[$n=7]}; ./le x`,
		`[[ "$n=7" -eq 7 ]]; ./le x`,
		`echo ${x:$n=7}; ./le x`,
		`test -v "a[$n=7]"; ./le x`,
		`[ -v "a[$n=7]" ]; ./le x`,
		`test -R x; ./le x`,
		`o=-v; test $o "a[$n=7]"; ./le x`,
	} {
		code, _, message := runSubagentBash(t, identityLineOfAgent+built+rest)
		if code != 2 || !strings.Contains(message, "another identity") {
			t.Errorf("%q: code=%d, want it refused as taking another identity: %s", rest, code, message)
		}
	}
	for _, rest := range []string{
		`[[ -f a ]] && ./le x`, `test -f a && ./le x`, `[ "$a" = b ] && ./le x`, `echo "${a:-x}"; ./le x`,
	} {
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want the plain test to pass", rest)
		}
	}
}

// VALIDATES: review I3 of phase 3 (R-9). The wrappers' options are read as
// their manuals give them, on the tree route and on the text route: xargs
// `--max-lines` takes no separate value, a short cluster ending in a value
// letter (`-0n 1`, `-fo out`) takes the next word, strace's and ltrace's long
// value options take one, and timeout takes one duration operand, whatever it
// is spelled, before the command.
// PREVENTS: an option read with the wrong width, which makes a value the
// command word or the command word a value, so `env -i` behind it passes.
func TestWrapperOptionsAreReadAsTheirManualsGiveThem(t *testing.T) {
	for _, rest := range []string{
		`xargs --max-lines env -i ./le x`,
		`xargs -0n 1 env -i ./le x`,
		`strace -fo out env -i ./le x`,
		`strace --output out env -i ./le x`,
		`strace --trace open env -i ./le x`,
		`strace -fu root ./le x`,
		`ltrace --config f env -i ./le x`,
		`timeout inf env -i ./le x`,
		`timeout 1e3 env -i ./le x`,
	} {
		for _, command := range []string{identityLineOfAgent + rest, identityLineOfAgent + rest + " 'oops"} {
			if !takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
				t.Errorf("%q: passes, want the environment tool behind the wrapper refused", command)
			}
		}
	}
	for _, rest := range []string{
		`timeout 5 ./le x`, `timeout 1.5s ./le x`, `nice -n 5 ./le x`, `xargs -0n 1 ./le x`, `strace -fo out ./le x`,
	} {
		command := identityLineOfAgent + rest
		if takesAnotherIdentity(command, parseCommandLine(command), identityLineOfAgent) {
			t.Errorf("%q: read as taking another identity, want it to pass", rest)
		}
	}
}
