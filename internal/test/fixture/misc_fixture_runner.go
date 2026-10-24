package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func init() {
	Register("runner/verify-scope-wiring-attribution", verifyScopeWiringDriver)
	Register("runner/commit-push-refused-by-derived-debt", commitPushRefusedByDerivedDebtDriver)
	Register("runner/commit-push-script-rechecks", commitPushScriptRechecksDriver)
	Register("runner/verify-green-record-clears-derived-debt", verifyGreenRecordClearsDerivedDebtDriver)
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

func verifyScopeWiringDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("wiring attribution fixture takes no arguments")
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-wiring-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup
	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod:                      "module fixture/wiring\n\ngo 1.24\n",
		fileFeatureGates:               contentFeatureGate,
		fileGitIgnore:                  contentGitIgnoreTmp,
		"docs/architecture/fixture.md": "# Fixture\n",
		"mine.go":                      "// Design: docs/architecture/fixture.md -- fixture\npackage mine\n",
		"theirs.go":                    "// Design: docs/architecture/fixture.md -- fixture\npackage theirs\n",
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
	if output, code, err := rawCommand(ctx, repo, env, leBinary, "verify status", "write", "exit-code", "1", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write verify status exit=%d: %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "scratch-repo-ready") //nolint:errcheck // progress output
	writeIndex := func(extra map[string]any) error {
		groups := []any{map[string]any{
			fieldStage: checkDocWiring, fieldGroupID: "files:wiring", fieldKind: fieldFiles, fieldRelated: []string{"theirs.go"},
			fieldSummary: "an exported symbol added by this change has no non-test reference", fieldRerun: "le doc wiring", fieldDetailLog: pathDocWiringLog, fieldParallel: columnGroup,
		}}
		if extra != nil {
			groups = append(groups, extra)
		}
		value := map[string]any{"stages": []any{map[string]any{fieldStage: checkDocWiring, "exit-code": 1, "groups": groups}}}
		data, _ := json.Marshal(value)
		return os.WriteFile(filepath.Join(repo, "tmp", "ze-verify-failures.json"), data, 0o600)
	}
	runCreate := func(session string) (string, int, error) {
		return rawCommand(ctx, repo, env, leBinary, argCommit, "create",
			"session", session, "subject", "fixture change", "file", "mine.go",
			"no-test", "mine.go is a scratch-repo fixture for gate attribution, not product code")
	}
	if err := writeIndex(nil); err != nil {
		return err
	}
	// No structural-red-ok is named, so a red charged to this commit would
	// refuse it: the commit landing is the proof the foreign red was not charged.
	output, code, err := runCreate("aaaa1111")
	if err != nil || code != 0 {
		return fmt.Errorf("foreign wiring red refused commit: exit=%d %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "wiring-red-of-another-session-not-charged") //nolint:errcheck // progress output
	if err := writeIndex(map[string]any{
		fieldStage: checkDocWiring, fieldGroupID: "files:design-refs", fieldKind: fieldFiles, fieldRelated: []string{"mine.go"},
		fieldSummary: "a Design reference does not resolve", fieldRerun: "le doc check links", fieldDetailLog: pathDocWiringLog, fieldParallel: columnGroup,
	}); err != nil {
		return err
	}
	output, code, err = runCreate("bbbb2222")
	if err != nil || code == 0 || !strings.Contains(output, "deterministic structural gate(s)") || !strings.Contains(output, "doc wiring") {
		return fmt.Errorf("own wiring red did not refuse: exit=%d %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "wiring-red-of-my-own-file-still-refuses") //nolint:errcheck // progress output
	if err := writeIndex(map[string]any{
		fieldStage: checkDocWiring, fieldGroupID: "subcheck:ci-sleep-ratchet", fieldKind: "subcheck", fieldRelated: []string{},
		fieldSummary: "functional fixtures exceed the wait-call ceiling", fieldRerun: "le doc wiring", fieldDetailLog: pathDocWiringLog, fieldParallel: columnGroup,
	}); err != nil {
		return err
	}
	output, code, err = runCreate("cccc3333")
	if err != nil || code == 0 || !strings.Contains(output, "charged for want of path attribution") || !strings.Contains(output, "subcheck:ci-sleep-ratchet") {
		return fmt.Errorf("blind wiring red did not refuse: exit=%d %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "unattributable-wiring-red-is-charged") //nolint:errcheck // progress output
	return nil
}
