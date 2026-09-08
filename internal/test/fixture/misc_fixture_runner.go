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
	"strings"
)

func init() {
	Register("runner/verify-scope-debt-clear", verifyScopeDebtClearDriver)
	Register("runner/verify-scope-debt-discharge", verifyScopeDebtDischargeDriver)
	Register("runner/verify-scope-wiring-attribution", verifyScopeWiringDriver)
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

// envRootedAt answers this process's environment with ZE_REPO_ROOT REPLACED
// rather than appended.
//
// The harness exports ZE_REPO_ROOT naming this checkout, and a fixture that
// appends a second one leaves the variable twice in the child's environment.
// Which copy the child reads is a property of the C library and of Go's own
// dedup rule, and a fixture must not depend on either: the wrong answer points
// a real `le` at the real repository, where `le commit debt-clear` starts a
// native verification over the shared tree and re-judges the real ledger.
func envRootedAt(root string) []string {
	inherited := os.Environ()
	kept := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		name, _, found := strings.Cut(entry, "=")
		// env.Get matches case-insensitively and reads a dot as an underscore,
		// so both spellings of the key are dropped.
		if found && strings.EqualFold(strings.ReplaceAll(name, ".", "_"), "ZE_REPO_ROOT") {
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

// verifyScopeDebtClearDriver drives the REAL `le commit debt-clear` against a
// scratch checkout and judges the LEDGER it leaves behind.
//
// The ledger is the assertion because it is the product's own artifact. An
// earlier version of this driver edited the ledger itself and then asserted its
// own edit, which is a scenario that passes against a stub: it printed
// "green-gate-cleared-its-row" without any product code having cleared
// anything.
//
// What a functional scenario CAN reach is the half that runs no gate: a row no
// gate can re-run is named, nothing is run for it, and it stays open. Both
// halves of the gate-exit decision, green and red, are proved by
// TestDebtClearingHonorsTheGateExit (internal/le/commit/commit_test.go), which
// names its own action dispatcher. Through the real dispatcher each of them is
// a native verification run, which claims this machine's job slots and takes
// the better part of an hour, so no scenario may start one.
func verifyScopeDebtClearDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("debt-clear fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-debt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	ledgerHeader := "| Date | Session | Subject | Gate owed | Reason | Status |\n|------|---------|---------|-----------|--------|--------|\n"
	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod: "module fixture/debt\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate,
		fileGitIgnore:                       contentGitIgnoreTmp,
		"plan/verification-debt/fixture.md": ledgerHeader + "| 2026-08-19 | fixture | a commit | independent critical review | no reviewer | open |\n",
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "debt-fixtures-ready") //nolint:errcheck // progress output

	// envRootedAt, never os.Environ(): the harness exports ZE_REPO_ROOT naming
	// THIS checkout, and a le that reads it re-judges the real ledger and starts
	// a native verification over the shared tree.
	out, code, err := rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-clear")
	if err != nil || code != 0 {
		return fmt.Errorf("debt-clear over an unrunnable row exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "independent critical review") {
		return fmt.Errorf("debt-clear did not name the row it cannot run: %s", out)
	}
	fmt.Fprintln(os.Stdout, "native-debt-clear-runs-registered-gates") //nolint:errcheck // progress output

	body, err := os.ReadFile(filepath.Join(repo, "plan", "verification-debt", "fixture.md")) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	if strings.Contains(string(body), "| cleared |") {
		return fmt.Errorf("a row was cleared by a gate that never ran:\n%s", body)
	}
	if !strings.Contains(string(body), "| open |") {
		return fmt.Errorf("the unrunnable row left the ledger:\n%s", body)
	}
	fmt.Fprintln(os.Stdout, "unrunnable-row-named-and-left-open") //nolint:errcheck // progress output

	// The cut grammar, through the real binary. `part` without `of` cannot say
	// how many pieces the stages were dealt into, so it is refused before any
	// row is read and before any gate starts.
	out, code, err = rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-clear", "part", "1")
	if err != nil {
		return fmt.Errorf("debt-clear part 1: %w %s", err, out)
	}
	if code == 0 {
		return fmt.Errorf("debt-clear accepted a cut that names no piece count: %s", out)
	}
	fmt.Fprintln(os.Stdout, "half-named-cut-refused") //nolint:errcheck // progress output

	// The other half of the claim: nothing was RUN for that row. A native
	// verification leaves both of these behind in the checkout it judged, so
	// their absence is what says the command answered from the ledger alone.
	for _, artifact := range []string{filepath.Join("tmp", "verify"), filepath.Join("tmp", "ze-verify.status")} {
		if _, statErr := os.Stat(filepath.Join(repo, artifact)); statErr == nil {
			return fmt.Errorf("debt-clear started a gate for a row no gate can run: %s exists", artifact)
		}
	}
	fmt.Fprintln(os.Stdout, "debt-clear-started-no-gate") //nolint:errcheck // progress output
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
	repoRoot := os.Getenv("ZE_REPO_ROOT")
	if repoRoot == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	leBinary := filepath.Join(repoRoot, "bin", "le")
	if info, statErr := os.Stat(leBinary); statErr != nil || info.Mode()&0o111 == 0 {
		return fmt.Errorf("native le binary is not executable: %s", leBinary)
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
			"unverified", "another session edited the tree",
			"stale-index-ok", "scratch checkout intentionally has no generated discovery index",
			"no-test", "mine.go is a scratch-repo fixture for gate attribution, not product code")
	}
	if err := writeIndex(nil); err != nil {
		return err
	}
	output, code, err := runCreate("aaaa1111")
	if err != nil || code != 0 {
		return fmt.Errorf("foreign wiring red refused commit: exit=%d %w %s", code, err, output)
	}
	debt, err := os.ReadFile(filepath.Join(repo, "plan", "verification-debt", "aaaa1111.md")) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	if strings.Contains(string(debt), "structural gates (red)") ||
		!strings.Contains(string(debt), "full native verification (not FRESH-green)") {
		return fmt.Errorf("foreign red debt attribution wrong: %s", debt)
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

// verifyScopeDebtDischargeDriver drives the REAL `le commit debt-discharge`
// against a scratch checkout and judges the LEDGER it leaves behind.
//
// The ledger is the assertion because it is the product's own artifact. The
// scenario is the whole loop an operator runs: a row no gate can re-run is
// discharged from the commit it names, the ledger stops counting it, and then
// the record is tampered with and the row counts open again. That last step is
// what a stub cannot fake: nothing is deleted, and the row returns to open
// purely because the derivation is re-run on every read.
func verifyScopeDebtDischargeDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("debt-discharge fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-discharge-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	const subject = "a commit that closes no spec"
	const shard = "plan/verification-debt/fixture.md"
	ledgerHead := "| Date | Session | Subject | Gate owed | Reason | Status |\n" +
		"|------|---------|---------|-----------|--------|--------|\n"
	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod: "module fixture/discharge\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate,
		fileGitIgnore: contentGitIgnoreTmp,
		shard:         ledgerHead + "| 2026-09-08 | fixture | " + subject + " | independent critical review | no reviewer | open |\n",
	}); err != nil {
		return err
	}
	// The commit the discharge names: it carries a file and removes no spec, so
	// today's closure producer answers that no review was ever owed for it.
	if err := os.WriteFile(filepath.Join(repo, "note.md"), []byte("# note\n"), 0o600); err != nil {
		return err
	}
	for _, command := range [][]string{{argAdd, "--", "note.md"}, {argCommit, "-q", "-m", subject}} {
		out, code, err := rawCommand(ctx, repo, os.Environ(), "git", command...)
		if err != nil || code != 0 {
			return fmt.Errorf("git %s in the fixture checkout: %w %s", command[0], err, out)
		}
	}
	head, code, err := rawCommand(ctx, repo, os.Environ(), "git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return fmt.Errorf("git rev-parse HEAD: %w %s", err, head)
	}
	sha := strings.TrimSpace(head)
	fmt.Fprintln(os.Stdout, "discharge-fixtures-ready") //nolint:errcheck // progress output

	out, code, err := rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-status")
	if err != nil || code != 0 {
		return fmt.Errorf("debt-status before the discharge exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "1 open") {
		return fmt.Errorf("debt-status before the discharge did not count the row open: %s", out)
	}

	out, code, err = rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-discharge",
		"shard", "fixture.md", "line", "3", "kind", "not-applicable", argCommit, sha)
	if err != nil || code != 0 {
		return fmt.Errorf("debt-discharge exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "discharged 1 row(s)") {
		return fmt.Errorf("debt-discharge did not discharge the row: %s", out)
	}
	record := filepath.Join(repo, "plan", "verification-debt", "discharged")
	entries, err := os.ReadDir(record)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("no discharge record under %s: %w", record, err)
	}
	fmt.Fprintln(os.Stdout, "native-debt-discharge-wrote-a-record") //nolint:errcheck // progress output

	out, code, err = rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-status")
	if err != nil || code != 0 {
		return fmt.Errorf("debt-status after the discharge exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "0 open") || !strings.Contains(out, "1 discharged") {
		return fmt.Errorf("the ledger still counts the discharged row open: %s", out)
	}
	if !strings.Contains(out, "not-applicable 1") {
		return fmt.Errorf("debt-status does not split the discharged count by kind: %s", out)
	}
	fmt.Fprintln(os.Stdout, "discharged-row-no-longer-counted-open") //nolint:errcheck // progress output

	// The record still says what it said. Only the ROW moved, so its digest no
	// longer matches, and the derivation refuses to apply a record that answers
	// bytes the ledger no longer holds.
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(shard)),
		[]byte(ledgerHead+"| 2026-09-08 | fixture | "+subject+
			" | independent critical review | a reason somebody edited | open |\n"), 0o600); err != nil {
		return err
	}
	out, code, err = rawCommand(ctx, repo, envRootedAt(repo), le, argCommit, "debt-status")
	if err != nil || code != 0 {
		return fmt.Errorf("debt-status after the tamper exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "1 open") || !strings.Contains(out, "INVALID DISCHARGE") {
		return fmt.Errorf("a tampered row stayed discharged: %s", out)
	}
	fmt.Fprintln(os.Stdout, "tampered-row-counted-open-again") //nolint:errcheck // progress output
	return nil
}
