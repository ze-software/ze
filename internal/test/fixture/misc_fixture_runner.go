package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/creack/pty"
)

func init() {
	Register("runner/commit-not-refused-for-a-red", commitNotRefusedForARedDriver)
	Register("runner/commit-push-refused-by-derived-debt", commitPushRefusedByDerivedDebtDriver)
	Register("runner/commit-push-script-rechecks", commitPushScriptRechecksDriver)
	Register("runner/verify-changed-record-clears-derived-debt", verifyChangedRecordClearsDerivedDebtDriver)
	Register("runner/verify-worktree-cut-sweep", verifyWorktreeCutSweepDriver)
	Register("runner/commit-attest-empty-commit", commitAttestEmptyCommitDriver)
	Register("runner/commit-review-owed-trailer", commitReviewOwedTrailerDriver)
	Register("runner/exec-quoted-argument", execQuotedArgumentDriver)
}

// execQuotedArgumentWant is the single argument test/runner/exec-quoted-argument.ci
// quotes on its exec= line. It carries the two characters that a whitespace
// split destroys: a space and a pipe.
const execQuotedArgumentWant = "one | two three"

// execQuotedArgumentDriver fails unless the runner handed the quoted exec=
// argument over as ONE argv element. A whitespace split delivers four elements
// (`"one`, `|`, `two`, `three"`), so this fixture is red against a runner that
// does not honor the quotes.
func execQuotedArgumentDriver(_ context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("argv has %d elements %q, want 1 element %q", len(args), args, execQuotedArgumentWant)
	}
	if args[0] != execQuotedArgumentWant {
		return fmt.Errorf("argv[0] = %q, want %q", args[0], execQuotedArgumentWant)
	}
	fmt.Println("quoted-argument-intact")
	return nil
}

// inheritedDropped is every variable a scratch-checkout fixture MUST NOT pass
// to the `le` it starts. One list, because every fixture here runs a real
// binary over a throwaway tree and each of these three points it somewhere
// else.
//
// ZE_REPO_ROOT is replaced rather than appended: the harness exports one naming
// this checkout, and a fixture that appends a second leaves the variable twice
// in the child's environment. Which copy the child reads is a property of the C
// library and of Go's own dedup rule, and a fixture must not depend on either.
//
// CLAUDE_PROJECT_DIR goes because the hook runtime resolves its root from it
// BEFORE it consults ZE_REPO_ROOT (hookruntime.newContext), so a harness that
// exports it points every check at the shared checkout.
//
// ZE_LE_BUILD_NAME goes because a fixture chooses bin/le, the shared build.
// That variable is the launcher's own channel for `./le --name <name>`, and the
// binary refuses to run when it names a build this process is not
// (cmd/ze/le_build_name.go). A session that started the suite through a named
// build would otherwise hand every child a name contradicting the binary the
// fixture picked.
//
// The wrong answer to any of the three points a real `le` at the real
// repository, where `le verify worktree` starts a native verification over the
// shared tree and writes the real green record.
var inheritedDropped = [...]string{envRepoRoot, "CLAUDE_PROJECT_DIR", "ZE_LE_BUILD_NAME"}

// envRootedAt answers this process's environment with every variable in
// inheritedDropped removed, and ZE_REPO_ROOT set to root.
func envRootedAt(root string) []string {
	inherited := os.Environ()
	kept := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			kept = append(kept, entry)
			continue
		}
		// env.Get matches case-insensitively and reads a dot as an underscore,
		// so both spellings of each key are dropped. environmentKeyReading is
		// that reading, declared once (fixture.go).
		if slices.ContainsFunc(inheritedDropped[:], func(key string) bool {
			return environmentKeyReading(name) == environmentKeyReading(key)
		}) {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, "ZE_REPO_ROOT="+root)
}

func rawCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, int, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = env
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return string(output), exit.ExitCode(), nil
	}
	return string(output), -1, err
}

// terminalCommand is rawCommand with a pseudo-terminal on the program's stdin,
// stdout and stderr. A whole-tree verify verb (`le verify worktree`,
// `le verify current`) refuses unless stdin and stdout are terminals, because
// it is a release check only Thomas starts at his own terminal; a fixture that
// drives one stands where Thomas stands, so it gives the program the terminal
// his shell would. The terminal writes CRLF, so the answer carries LF only, as
// rawCommand's does.
func terminalCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, int, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = env
	return runAtTerminal(command)
}

// terminalCommandStreams is terminalCommand with stderr kept apart: stdin and
// stdout are the pseudo-terminal, which is all the whole-tree gate asks of
// them (human.RequireTerminal), and stderr goes to its own buffer, as a shell
// at a terminal gives it with `2>file`. A fixture that parses stdout as JSON,
// or asserts which stream carried a message, needs the two kept apart.
func terminalCommandStreams(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, int, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = env
	var stderr bytes.Buffer
	// pty.Start fills only the streams left nil, so stderr stays this buffer.
	command.Stderr = &stderr
	stdout, code, err := runAtTerminal(command)
	return stdout, stderr.String(), code, err
}

// runAtTerminal starts command with a new pseudo-terminal on every stream it
// leaves nil, as the controlling terminal of a new session, and answers what
// the terminal carried with LF line ends, the exit code, and an error only when
// the program could not be started or waited for.
func runAtTerminal(command *exec.Cmd) (string, int, error) {
	terminal, err := pty.Start(command)
	if err != nil {
		return "", -1, err
	}
	defer terminal.Close() //nolint:errcheck // the program has exited; the terminal holds nothing more

	var output bytes.Buffer
	// The read ends when the program and every child holding the terminal have
	// closed it: Linux answers EIO there and macOS EOF, and neither is a failure.
	_, _ = output.ReadFrom(terminal) //nolint:errcheck // EIO or EOF is the end of the program's output
	answer := strings.ReplaceAll(output.String(), "\r\n", "\n")
	err = command.Wait()
	if err == nil {
		return answer, 0, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return answer, exit.ExitCode(), nil
	}
	return answer, -1, err
}

func rawCommandStreams(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, int, error) {
	command := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = dir
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return stdout.String(), stderr.String(), exit.ExitCode(), nil
	}
	return stdout.String(), stderr.String(), -1, err
}

func gitFixture(ctx context.Context, root string, files map[string]string) error {
	for path, content := range files {
		absolute := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(absolute), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(absolute, []byte(content), 0o600); err != nil {
			return err
		}
	}
	for _, args := range [][]string{{argInit, "-q", "."}, {argConfig, "user.email", "tester@example.com"}, {argConfig, "user.name", "Tester"}, {argConfig, "commit.gpgsign", valueFalse}, {argAdd, "-A"}, {argCommit, "-q", "-m", "base"}} {
		if output, code, err := rawCommand(ctx, root, os.Environ(), "git", args...); err != nil || code != 0 {
			return fmt.Errorf("git %s exit=%d: %w\n%s", strings.Join(args, " "), code, err, output)
		}
	}
	return nil
}

// commitNotRefusedForARedIndexes is each failure index the deleted structural
// red check refused a commit of mine.go for, keyed by the marker the driver
// prints once `le commit create` prepares the commit anyway.
var commitNotRefusedForARedIndexes = []struct{ marker, index string }{
	{"owned-red-not-refused", `{"stages":[{"stage":"doc wiring","exit-code":1,"groups":[` +
		`{"group-id":"files:mine","kind":"files","related":["mine.go"],"summary":"a Design reference does not resolve","rerun":"le doc wiring"}]}]}`},
	{"blind-red-not-refused", `{"stages":[{"stage":"doc wiring","exit-code":1,"groups":[` +
		`{"group-id":"subcheck:ci-sleep-ratchet","kind":"subcheck","related":[],"summary":"functional fixtures exceed the wait-call ceiling","rerun":"le doc wiring"}]}]}`},
	{"unreached-red-not-refused", `{"stopped-at":"rfc check","stages":[` +
		`{"stage":"rfc check","reached":true,"exit-code":1,"groups":[]},{"stage":"doc wiring","reached":false,"exit-code":0}]}`},
}

// commitNotRefusedForARedDriver proves, through the real `le commit create`,
// that a red prevents a release, never a commit (ai/META-DECISIONS.md, "Ze is
// pre-release"). In a scratch checkout it records each index in
// commitNotRefusedForARedIndexes and prepares a commit of mine.go with no
// admission: each must succeed. It then names each retired admission, which
// must be refused naming the keyword and its replacement, the body.
func commitNotRefusedForARedDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("commit-not-refused-for-a-red fixture takes no arguments")
	}
	repo, err := os.MkdirTemp("", "ze-commit-red-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup
	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod:                      "module fixture/red\n\ngo 1.24\n",
		fileFeatureGates:               contentFeatureGate,
		fileGitIgnore:                  contentGitIgnoreTmp,
		"docs/architecture/fixture.md": "# Fixture\n",
		"mine.go":                      "// Design: docs/architecture/fixture.md -- fixture\npackage mine\n",
	}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, "mine.go"), []byte("// Design: docs/architecture/fixture.md -- fixture\npackage mine\n\nfunc Mine() {}\n"), 0o600); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(repo, "tmp"), 0o750); err != nil {
		return err
	}
	leBinary, err := nativeLEBinary()
	if err != nil {
		return err
	}
	env := envRootedAt(repo)
	fmt.Fprintln(os.Stdout, "scratch-repo-ready") //nolint:errcheck // progress output
	runCreate := func(session string, extra ...string) (string, int, error) {
		line := append([]string{argCommit, "create", "session", session, "subject", "fixture change", "file", "mine.go",
			"no-test", "mine.go is a scratch-repo fixture for the commit gate, not product code"}, extra...)
		return rawCommand(ctx, repo, env, leBinary, line...)
	}
	for position, recorded := range commitNotRefusedForARedIndexes {
		if err := os.WriteFile(filepath.Join(repo, "tmp", "ze-verify-failures.json"), []byte(recorded.index), 0o600); err != nil {
			return err
		}
		output, code, err := runCreate("aaaa000" + strconv.Itoa(position+1))
		if err != nil || code != 0 {
			return fmt.Errorf("%s: a red in the index refused the commit: exit=%d %w %s", recorded.marker, code, err, output)
		}
		fmt.Fprintln(os.Stdout, recorded.marker) //nolint:errcheck // progress output
	}
	for position, keyword := range []string{"structural-red-ok", "broken-head-fix"} {
		output, code, err := runCreate("bbbb000"+strconv.Itoa(position+1), keyword, "a reason")
		if err != nil || code == 0 || !strings.Contains(output, strconv.Quote(keyword)) || !strings.Contains(output, "use body") {
			return fmt.Errorf("retired %s was not refused naming the body: exit=%d %w %s", keyword, code, err, output)
		}
	}
	fmt.Fprintln(os.Stdout, "retired-red-admissions-name-the-body") //nolint:errcheck // progress output
	return nil
}
