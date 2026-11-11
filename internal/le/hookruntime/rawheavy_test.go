// Design: docs/architecture/core-design.md -- native Bash hook policy
// Related: rawheavy.go -- rawAdvice, keptArgv and the packageRuns judgment

package hookruntime

import (
	"strings"
	"testing"
)

// init installs a package judgment that runs every package, standing in for
// testunit.PackageRuns, which this package cannot import (testunit reaches it
// through repo/changed). These tests judge the refusal's words; the wiring of
// the real judgment is TestRawGoTestAdvisesTheShortFormOnlyWhereTheActionRuns
// in internal/le/test/unit.
func init() {
	packageRuns = func(string, string) (bool, error) { return true, nil }
}

// TestRawGoTestAdviceKeepsAQuotedFilter is review I1 for the raw-job block.
//
// VALIDATES: a go test whose -run value holds a quoted bar, piped or not, is
// advised with that value and its package intact; a prefixed one keeps its
// assignment through env, with the value quoted again.
// PREVENTS: the raw-job block advising `go test -run TestA`, a different run
// with the package dropped, because its text split cut the quoted bar.
func TestRawGoTestAdviceKeepsAQuotedFilter(t *testing.T) {
	tests := []struct {
		name    string
		command string
		use     string
	}{
		{"piped", `go test -run 'TestA|TestB' ./internal/x/ | tail -5`,
			"./le test unit pkg path ./internal/x/ run 'TestA|TestB'"},
		{"unpiped", `go test -run 'A|B' ./x`, "./le test unit pkg path ./x run 'A|B'"},
		{"assignment", `GOFLAGS=-count=1 go test -run 'A|B' ./x`,
			"./le job run label unit-pkg command env GOFLAGS=-count=1 go test -run 'A|B' ./x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, message := rawRefusalOf(t, test.command)
			if code != 2 {
				t.Fatalf("%q: code %d, want 2: %s", test.command, code, message)
			}
			if !strings.Contains(message, "run raw, outside job admission") {
				t.Fatalf("%q: no raw-job block: %s", test.command, message)
			}
			if !strings.Contains(message, "  -- Use: "+test.use+"\n") {
				t.Errorf("%q: the raw-job block lacks %q: %s", test.command, test.use, message)
			}
			for _, banned := range []string{"-run TestA ", "-run A ", "run TestA\n", "run A\n"} {
				if strings.Contains(message, banned) {
					t.Errorf("%q: the raw-job block holds %q: %s", test.command, banned, message)
				}
			}
		})
	}
}

// TestRawGoTestMoveTheFormCannotCarry is the other half of the review ISSUE.
//
// VALIDATES: a go test after a directory change `env -C` cannot express (the
// previous directory, home, a pushd, a word the shell would expand) is
// advised as the long form with no move, and a sentence names the typed
// change and says to run the form from the directory it reaches.
// PREVENTS: advice wrapped in `bash -c '<move> && go test ...'`, which the
// hook's quote-blind split refuses again, and advice that silently drops the
// move with nothing saying so.
func TestRawGoTestMoveTheFormCannotCarry(t *testing.T) {
	for _, move := range []string{`cd -`, `cd`, `pushd internal`, `cd "$HOME/ze"`} {
		t.Run(move, func(t *testing.T) {
			command := move + " && go test ./x/"
			code, message := rawRefusalOf(t, command)
			if code != 2 {
				t.Fatalf("%q: code %d, want 2: %s", command, code, message)
			}
			if !strings.Contains(message, "  -- Use: ./le job run label unit-pkg command go test ./x/\n") {
				t.Errorf("%q: the refusal lacks the long form with no move: %s", command, message)
			}
			if !strings.Contains(message, "changes directory first ("+move+"), which this form cannot carry") {
				t.Errorf("%q: the refusal does not name the move it cannot carry: %s", command, message)
			}
			if strings.Contains(message, "bash -c") {
				t.Errorf("%q: the refusal advises bash -c: %s", command, message)
			}
		})
	}
}

// adviceCorpus is every command whose refusal the raw-job and piped-command
// tests pin, one per class those tests name, and a lint.
var adviceCorpus = []string{
	`go test ./internal/x/`,
	`go test ./internal/x/ ./internal/y/...`,
	`go test -run TestX ./internal/x/`,
	`go test -run '^TestX$' ./internal/x/`,
	`go test -race ./internal/x/`,
	`git status && go test ./internal/x/`,
	`go vet ./internal/x/`,
	`go vet -printf=false ./internal/x/`,
	`go test -v ./internal/x/`,
	`go test -run "it's" ./internal/x/`,
	`go test ./...`,
	`GOFLAGS=-count=1 go test ./internal/x/`,
	`timeout 60 go test ./internal/x/`,
	`cd internal && go test ./x/`,
	`cd internal && cd le; go test ./x/`,
	`cd internal && GOFLAGS=-count=1 go test ./x/`,
	`cd - && go test ./x/`,
	`pushd internal && go test ./x/`,
	`go test -run 'A|B' ./x`,
	`GOFLAGS=-count=1 go test -run 'A|B' ./x`,
	`golangci-lint run ./internal/x/`,
	`go test -run TestX ./internal/x/ | tail -5`,
	`go test -run 'TestA|TestB' ./internal/x/ | tail -5`,
	`go test ./a > /dev/null; go test ./b | tail -5`,
	`go test -run 'Test A$' ./internal/x/ | tail -5`,
	`GOFLAGS=-count=1 go test ./internal/x/ 2>&1 | grep FAIL`,
	`golangci-lint run ./internal/x/ | head -20`,
	`./le test functional gating | tail -5`,
}

// advisedCommands answers each command a refusal tells the caller to type:
// the raw-job block's Use line and the piped-command block's admitted line.
// A line naming a placeholder (`<package>`, `<label>`) is skipped: it is a
// shape the caller fills in, not a command to type.
func advisedCommands(message string) []string {
	var advised []string
	for line := range strings.SplitSeq(message, "\n") {
		advice, isUse := strings.CutPrefix(line, "  -- Use: ")
		if !isUse {
			advice = strings.TrimSpace(line)
		}
		typed := isUse || strings.HasPrefix(advice, "./le job run label ") || strings.HasPrefix(advice, "export ZE_AGENT_ID=")
		if !typed {
			continue
		}
		if strings.Contains(advice, "<") {
			continue
		}
		advised = append(advised, advice)
	}
	return advised
}

// TestRefusalAdviceIsAcceptedByTheHook is the review ISSUE and review N7.
//
// VALIDATES: every command the raw-job and piped-command refusals advise for
// the corpus, sent back through the whole pretool-bash hook, passes with exit
// 0, for the main thread and for a subagent; a subagent's advice starts with
// its own identity line, and the main thread's carries none.
// PREVENTS: advice whose literal command the same hook refuses: a `bash -c`
// string holding `&&`, which the quote-blind split cuts, and a subagent's
// advice without the identity line bashAgentIdentity requires.
func TestRefusalAdviceIsAcceptedByTheHook(t *testing.T) {
	for _, command := range adviceCorpus {
		t.Run("main/"+command, func(t *testing.T) {
			code, message := rawRefusalOf(t, command)
			if code != 2 {
				t.Fatalf("%q: code %d, want a refusal: %s", command, code, message)
			}
			advised := advisedCommands(message)
			if len(advised) == 0 {
				t.Fatalf("%q: the refusal advises no command: %s", command, message)
			}
			for _, advice := range advised {
				if strings.Contains(advice, "export ZE_AGENT_ID") {
					t.Errorf("%q: the main thread's advice carries an identity line: %q", command, advice)
				}
				if code, message := rawRefusalOf(t, advice); code != 0 {
					t.Errorf("%q: the advice %q is refused (code %d): %s", command, advice, code, message)
				}
			}
		})
		t.Run("subagent/"+command, func(t *testing.T) {
			code, _, message := runSubagentBash(t, subagentLine+command)
			if code != 2 {
				t.Fatalf("%q: code %d, want a refusal: %s", command, code, message)
			}
			advised := advisedCommands(message)
			if len(advised) == 0 {
				t.Fatalf("%q: the refusal advises no command: %s", command, message)
			}
			for _, advice := range advised {
				if !strings.HasPrefix(advice, subagentLine) {
					t.Errorf("%q: the advice %q does not start with the identity line", command, advice)
				}
				if code, _, message := runSubagentBash(t, advice); code != 0 {
					t.Errorf("%q: the advice %q is refused (code %d): %s", command, advice, code, message)
				}
			}
		})
	}
}

// TestRawGoVetShortLineSaysVet is review N6.
//
// VALIDATES: the short line of a go vet the action cannot express says the
// action vets, and names no test or race detector.
// PREVENTS: a vet refusal describing the test run.
func TestRawGoVetShortLineSaysVet(t *testing.T) {
	code, message := rawRefusalOf(t, `go vet -x ./internal/x/`)
	if code != 2 {
		t.Fatalf("code %d, want 2: %s", code, message)
	}
	if !strings.Contains(message, "The short form vets a package with its feature tags,") {
		t.Errorf("the vet short line does not say vet: %s", message)
	}
	if strings.Contains(message, "race detector") {
		t.Errorf("the vet short line names the race detector: %s", message)
	}
}
