//go:build unix

// Related: proposed.go -- the git queries these cases observe through a recording git
package testweakened

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitCall is one invocation the recording git saw: its arguments and the git
// settings its environment carried.
type gitCall struct {
	args []string
	env  map[string]string
}

// fakeGit puts an executable named git first on PATH for the rest of the
// test. It appends each call's argv and git environment to a log, runs refuse
// (a shell fragment that may print a refusal and exit), and hands every other
// call to the real git. GIT_OPTIONAL_LOCKS and GIT_CONFIG_COUNT are cleared
// from the test's own environment, so a setting the log shows came from the
// code under test. It answers the log's path.
func fakeGit(t *testing.T, refuse string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	logPath := filepath.Join(directory, "calls.log")
	script := "#!/bin/sh\n" +
		"{ printf 'argv'; for argument in \"$@\"; do printf '\\037%s' \"$argument\"; done; printf '\\n'\n" +
		"  env | grep -E '^(GIT_OPTIONAL_LOCKS|GIT_CONFIG_COUNT|GIT_CONFIG_KEY_[0-9]+|GIT_CONFIG_VALUE_[0-9]+)=' || true\n" +
		"  printf 'end\\n'; } >> '" + logPath + "'\n" +
		refuse +
		"exec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o700); err != nil { //nolint:gosec // the test's own recording git must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, name := range []string{"GIT_OPTIONAL_LOCKS", "GIT_CONFIG_COUNT"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	return logPath
}

// readGitCalls parses the recording git's log. A log never written means git
// was never run.
func readGitCalls(t *testing.T, logPath string) []gitCall {
	t.Helper()
	content, err := os.ReadFile(logPath) //nolint:gosec // the test's own log
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls []gitCall
	for line := range strings.SplitSeq(string(content), "\n") {
		if arguments, found := strings.CutPrefix(line, "argv"); found {
			call := gitCall{env: make(map[string]string)}
			if arguments != "" {
				call.args = strings.Split(strings.TrimPrefix(arguments, "\x1f"), "\x1f")
			}
			calls = append(calls, call)
			continue
		}
		if line == "end" || line == "" || len(calls) == 0 {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		calls[len(calls)-1].env[key] = value
	}
	return calls
}

// verb answers the git subcommand of a call: its first argument that is not
// an option, skipping the value of -c and -C.
func (c gitCall) verb() string {
	for index := 0; index < len(c.args); index++ {
		argument := c.args[index]
		if argument == "-c" || argument == "-C" {
			index++
			continue
		}
		if strings.HasPrefix(argument, "-") {
			continue
		}
		return argument
	}
	return ""
}

// readOnlyQuery reports whether the call's environment carries every setting
// gitenv.ReadOnlyQuery adds: GIT_OPTIONAL_LOCKS=0, and maintenance.auto=false
// and gc.auto=0 as GIT_CONFIG_KEY_<n>/GIT_CONFIG_VALUE_<n> pairs.
func (c gitCall) readOnlyQuery() bool {
	if c.env["GIT_OPTIONAL_LOCKS"] != "0" {
		return false
	}
	count, err := strconv.Atoi(c.env["GIT_CONFIG_COUNT"])
	if err != nil {
		return false
	}
	pairs := make(map[string]string, count)
	for index := range count {
		suffix := strconv.Itoa(index)
		pairs[c.env["GIT_CONFIG_KEY_"+suffix]] = c.env["GIT_CONFIG_VALUE_"+suffix]
	}
	return pairs["maintenance.auto"] == "false" && pairs["gc.auto"] == "0"
}

// TestEveryHookGitQueryIsReadOnly proves every git call the edit hook's
// judgement and the approval writer make is a read that cannot touch the
// shared index: none is `git diff` or `git status` (each may refresh and
// rewrite the index), and each carries gitenv.ReadOnlyQuery's settings.
// Method: a recording git first on PATH logs each call's argv and git
// environment, then runs the real git; an approval, a whole-file Edit
// proposal (which reads the approval history), and a Write of a new test file
// run through it, and the log is read back.
// VALIDATES: gitCapture, approvalGit and approvalLanded each set
// gitenv.ReadOnlyQuery, and no hook-path query is diff or status. Dropping
// ReadOnlyQuery from any of them leaves GIT_OPTIONAL_LOCKS unset in that
// call's environment, because the test cleared it, and adding a diff or a
// status call is named by its verb.
// PREVENTS: a guard that runs on every session's edit locking or rewriting
// the index another session's commit script is using.
func TestEveryHookGitQueryIsReadOnly(t *testing.T) {
	root := headBaselineRepository(t)
	logPath := fakeGit(t, "")

	if _, err := Approve(root, fixtureSession, "pkg.TestKept", "verdict"); err != nil {
		t.Fatal(err)
	}
	if _, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/intent_test.go", Tool: "Edit", ToolInput: ProposedToolInput{OldString: "\trequire.NoError(t, err)\n", NewString: commentedOut},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/new_test.go", Tool: "Write", ToolInput: ProposedToolInput{Content: "package pkg\nfunc TestNew(t *testing.T) {}\n"},
	}); err != nil {
		t.Fatal(err)
	}

	calls := readGitCalls(t, logPath)
	verbs := make(map[string]bool)
	for _, call := range calls {
		verbs[call.verb()] = true
		if call.verb() == "diff" || call.verb() == "status" {
			t.Errorf("the hook path ran git %s, which may refresh the shared index: %q", call.verb(), call.args)
		}
		if !call.readOnlyQuery() {
			t.Errorf("git %q ran without gitenv.ReadOnlyQuery's settings: %v", call.args, call.env)
		}
	}
	for _, want := range []string{"ls-tree", "log"} {
		if !verbs[want] {
			t.Fatalf("the recording git saw no %s call (saw %v), so it did not observe the hook path", want, verbs)
		}
	}
}

// TestProposedRefusesWhenTheDirectoryListingFails proves the edit is refused
// when HEAD resolves and the edited file's HEAD text reads, but the listing of
// its committed directory fails: an unlisted directory would lose every moved
// or aliased sibling from the baseline. TestProposedRefusesWhenTheBaselineCannotBeRead
// cannot reach this branch, because an unborn HEAD fails the read before it.
// Method: a recording git refuses only the directory listing (ls-tree with
// --full-name) and runs every other call; a whole-file Edit of the committed
// file is proposed.
// VALIDATES: proposedBaselineSiblings answers errCommittedUnreadable when its
// ls-tree fails.
// PREVENTS: a failed listing read as a directory with no committed siblings.
func TestProposedRefusesWhenTheDirectoryListingFails(t *testing.T) {
	root := headBaselineRepository(t)
	logPath := fakeGit(t, "case \" $* \" in *\" ls-tree \"*\" --full-name \"*) echo 'fatal: the listing is refused by the test' >&2; exit 128;; esac\n")

	_, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/intent_test.go", Tool: "Edit", ToolInput: ProposedToolInput{OldString: "\trequire.NoError(t, err)\n", NewString: commentedOut},
	})
	if !errors.Is(err, errCommittedUnreadable) || !strings.Contains(err.Error(), "the listing is refused by the test") {
		t.Fatalf("an Edit whose directory listing fails = %v, want errCommittedUnreadable naming the listing", err)
	}
	readHead := false
	for _, call := range readGitCalls(t, logPath) {
		if call.verb() == "show" {
			readHead = true
		}
	}
	if !readHead {
		t.Fatal("the edited file's HEAD text was never read, so the failure is not the listing's")
	}
}

// TestProposedDecidesARootIsNoRepositoryWithoutGit proves a Write of a new
// test file in a root no repository holds is clean whatever git would print:
// the decision is made on what the directory is (rfc.InsideGitCheckout),
// never on the wording of a git refusal, which a localized git translates.
// Method: a fixture root with no .git, discovery bounded at its parent by
// GIT_CEILING_DIRECTORIES, and a recording git that refuses every call in
// another language; a Write of a new test file is proposed.
// VALIDATES: proposedRecreated answers no committed text for such a root
// without running git.
// PREVENTS: every new test Write refused on a machine whose git speaks
// another language (before the fix, the refusal's English prefix decided it).
func TestProposedDecidesARootIsNoRepositoryWithoutGit(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	// An explicit GIT_DIR names a repository whatever the directory holds.
	t.Setenv("GIT_DIR", "")
	if err := os.Unsetenv("GIT_DIR"); err != nil {
		t.Fatal(err)
	}
	logPath := fakeGit(t, "echo 'fatal: Kein Git-Repository (oder irgendeines der Elternverzeichnisse): .git' >&2; exit 128\n")

	report, err := proposedFixture(root, ProposedRequest{
		Path: "pkg/new_test.go", Tool: "Write", ToolInput: ProposedToolInput{Content: headBaselineCommitted},
	})
	if err != nil || report.ExitCode() != 0 || len(report.Weakened) != 0 {
		t.Fatalf("a new test Write in a root no repository holds = %#v, %v, want clean", report, err)
	}
	if calls := readGitCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("deciding the root holds no repository ran git %d times: %q", len(calls), calls[0].args)
	}
}
