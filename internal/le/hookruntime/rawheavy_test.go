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
			"./le job run label unit-pkg quiet command env GOFLAGS=-count=1 go test -run 'A|B' ./x"},
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
			if !strings.Contains(message, "  -- Use: ./le job run label unit-pkg quiet command go test ./x/\n") {
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

// TestLauncherLoopsCompileNoPattern is AC-21 of
// spec-agent-economy-6-shell-parser.
//
// VALIDATES: rawHeavy and programWords skip a launcher's numeric argument
// (`timeout 60`, `nice 10`, `timeout 1.5m`) as before, and each launcher word
// added in front of a segment costs at most two allocations: the
// numeric-argument pattern is compiled once, at package load. The test
// compares a segment behind k launcher pairs with one behind k+8, so the
// allocations every judgment makes whatever its launchers (the word slice,
// the kept argv, the refusal) cancel out, and only the cost of each added
// word is measured.
// PREVENTS: a regular expression compiled on every pass of the launcher
// loops, which costs every Bash call of every session dozens of allocations
// per launcher word.
func TestLauncherLoopsCompileNoPattern(t *testing.T) {
	if got := programWords("nice 10 timeout 1.5m go test ./x"); strings.Join(got, " ") != "go test ./x" {
		t.Errorf("programWords skipped to %q, want go test ./x", got)
	}
	if refusal := rawHeavy("timeout 60 go test ./x", nil, t.TempDir()); refusal.what != "`go test`" {
		t.Errorf("rawHeavy behind timeout 60 answered %q, want the go test refusal", refusal.what)
	}
	// The per-word budget: an added word costs shellWords one string, and
	// the word slice and the prefix slice an amortized share of a doubling,
	// so two allocations per word is a ceiling no correct loop reaches. One
	// compile of the pattern costs dozens of allocations, and the loops would
	// compile at least once for each launcher word, so a compile inside
	// either loop exceeds the ceiling many times over.
	const (
		fewPairs              = 2
		addedPairs            = 8
		wordsPerPair          = 2
		allocationsPerWordMax = 2
	)
	few := strings.Repeat("timeout 5 ", fewPairs) + "cat x"
	many := strings.Repeat("timeout 5 ", fewPairs+addedPairs) + "cat x"
	budget := float64(addedPairs * wordsPerPair * allocationsPerWordMax)
	if got := programWords(many); strings.Join(got, " ") != "cat x" {
		t.Fatalf("programWords behind %d launchers skipped to %q, want cat x", fewPairs+addedPairs, got)
	}
	short := testing.AllocsPerRun(20, func() { programWords(few) })
	long := testing.AllocsPerRun(20, func() { programWords(many) })
	if long-short > budget {
		t.Errorf("programWords: %v allocations behind %d launcher pairs, %v behind %d: more than %d per added word, the launcher loop compiles a pattern",
			long, fewPairs+addedPairs, short, fewPairs, allocationsPerWordMax)
	}
	short = testing.AllocsPerRun(20, func() { rawHeavy(few, nil, "") })
	long = testing.AllocsPerRun(20, func() { rawHeavy(many, nil, "") })
	if long-short > budget {
		t.Errorf("rawHeavy: %v allocations behind %d launcher pairs, %v behind %d: more than %d per added word, the launcher loop compiles a pattern",
			long, fewPairs+addedPairs, short, fewPairs, allocationsPerWordMax)
	}
}

// TestRawRefusalAdvisesTheQuietForm is AC-20 of
// spec-agent-economy-6-shell-parser.
//
// VALIDATES: through the hook's Run, a raw go test or go vet the short action
// cannot express, and a raw golangci-lint run, are refused with a Use line in
// the quiet admitted form `./le job run label <label> quiet command <argv>`,
// and the generic-form line carries `quiet` too. A go test the short action
// expresses still advises the short action, which is quiet by itself.
// PREVENTS: the refusal teaching the long form without `quiet`, whose
// streamed output fills the caller's context (ai/rules/commands.md), while
// the piped-command refusal already teaches the quiet form.
func TestRawRefusalAdvisesTheQuietForm(t *testing.T) {
	tests := []struct {
		name    string
		command string
		use     string
	}{
		{"go-test-flag", `go test -v ./internal/x/`, "./le job run label unit-pkg quiet command go test -v ./internal/x/"},
		{"go-vet-flag", `go vet -printf=false ./internal/x/`,
			"./le job run label vet-pkg quiet command go vet -printf=false ./internal/x/"},
		{"lint", `golangci-lint run ./internal/x/`, "./le job run label lint quiet command golangci-lint run ./internal/x/"},
		{"go-test-short", `go test ./internal/x/`, "./le test unit pkg path ./internal/x/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, message := rawRefusalOf(t, test.command)
			if code != 2 {
				t.Fatalf("%q: code %d, want 2: %s", test.command, code, message)
			}
			if !strings.Contains(message, "  -- Use: "+test.use+"\n") {
				t.Errorf("%q: the refusal lacks the Use line %q: %s", test.command, test.use, message)
			}
			if !strings.Contains(message, "  -- Generic form:\n     ./le job run label <label> quiet command <argv...>\n") {
				t.Errorf("%q: the generic form lacks quiet: %s", test.command, message)
			}
			for _, label := range []string{"unit-pkg", "vet-pkg", "lint", "<label>"} {
				if strings.Contains(message, "./le job run label "+label+" command") {
					t.Errorf("%q: the refusal teaches the %s form without quiet: %s", test.command, label, message)
				}
			}
		})
	}
}

// TestLauncherValueOptionsAreSkippedWithTheirValue drives the raw-job and
// piped-command guards with a launcher option that takes a value, beside the
// same line without the option and its value.
//
// VALIDATES: AC-33 of spec-agent-economy-6-shell-parser. Both launcher loops
// (rawHeavy, programWords) skip a value-taking option together with its one
// value, read from launcherValueOptions, so each line gets the verdict of the
// line without the option; an option that takes no value is skipped alone,
// as before. The `-c` rows of the criterion keep the verdict of their line
// without the option, a pass: the text judgment reads the `-c` string as one
// word whether or not an option comes first, and reading inside it is the
// tree judgment's work (AC-9, AC-10).
// PREVENTS: `timeout -s KILL 60 go test ./x` and `sudo -u u go test ./x`
// passing because `KILL` and `u` were read as the program, and `time -o t.log
// ./le test unit all | tail -5` passing because `t.log` was.
func TestLauncherValueOptionsAreSkippedWithTheirValue(t *testing.T) {
	root := t.TempDir()
	for _, row := range []struct {
		guard   string
		check   hookCheck
		with    string
		without string
		refused bool
	}{
		{"raw", bashRawHeavy, `timeout -s KILL 60 go test ./x`, `timeout 60 go test ./x`, true},
		{"raw", bashRawHeavy, `timeout --signal KILL 60 go test ./x`, `timeout 60 go test ./x`, true},
		{"raw", bashRawHeavy, `timeout -k 5s 60 go test ./x`, `timeout 60 go test ./x`, true},
		{"raw", bashRawHeavy, `nice -n 5 go test ./x`, `nice go test ./x`, true},
		{"raw", bashRawHeavy, `sudo -u u go test ./x`, `sudo go test ./x`, true},
		{"raw", bashRawHeavy, `sudo --user u go test ./x`, `sudo go test ./x`, true},
		{"raw", bashRawHeavy, `env -u HOME go test ./x`, `env go test ./x`, true},
		{"raw", bashRawHeavy, `time -o t.log go vet ./x`, `time go vet ./x`, true},
		{"raw", bashRawHeavy, `sh -o errexit go test ./x`, `sh go test ./x`, true},
		{"raw", bashRawHeavy, `bash +O extglob go test ./x`, `bash go test ./x`, true},
		{"raw", bashRawHeavy, `bash -O extglob -c 'go test ./x'`, `bash -c 'go test ./x'`, false},
		{"pipe", bashLossyPipe, `timeout -s KILL 600 ./le test unit all | tail -5`, `timeout 600 ./le test unit all | tail -5`, true},
		{"pipe", bashLossyPipe, `sudo -u u ./le test unit all | tail -5`, `sudo ./le test unit all | tail -5`, true},
		{"pipe", bashLossyPipe, `time -o t.log ./le test unit all | tail -5`, `time ./le test unit all | tail -5`, true},
		{"pipe", bashLossyPipe, `bash -o pipefail ./le test unit all | tail -5`, `bash ./le test unit all | tail -5`, true},
		{"pipe", bashLossyPipe, `nice -n 5 go test ./x | tail -5`, `nice go test ./x | tail -5`, true},
		{"pipe", bashLossyPipe, `bash -o pipefail -c './le test unit all | tail -5'`, `bash -c './le test unit all | tail -5'`, false},
		// An option that takes no value is skipped alone, so the word after
		// it is still read as the program.
		{"raw", bashRawHeavy, `sudo -E go test ./x`, `sudo go test ./x`, true},
		{"raw", bashRawHeavy, `env -i go test ./x`, `env go test ./x`, true},
		{"raw", bashRawHeavy, `timeout -v 60 go test ./x`, `timeout 60 go test ./x`, true},
		{"pipe", bashLossyPipe, `sudo -E ./le test unit all | tail -5`, `sudo ./le test unit all | tail -5`, true},
	} {
		with := guardVerdict(row.check, root, row.with)
		without := guardVerdict(row.check, root, row.without)
		if (without != nil) != row.refused {
			t.Errorf("%s: %q without the option refused = %t, want %t", row.guard, row.without, without != nil, row.refused)
		}
		if (with != nil) != (without != nil) {
			t.Errorf("%s: %q refused = %t, but %q refused = %t: the option and its value must not change the verdict",
				row.guard, row.with, with != nil, row.without, without != nil)
		}
	}
}
