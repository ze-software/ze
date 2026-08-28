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
	Register("runner/verify-scope-debt-clear-gate", verifyScopeDebtGateDriver)
	Register("runner/verify-scope-wiring-attribution", verifyScopeWiringDriver)
}

func rawCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, int, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = env
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode(), nil
	}
	return string(output), -1, err
}

func rawCommandStreams(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, int, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
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
	for _, args := range [][]string{{"init", "-q", "."}, {"config", "user.email", "tester@example.com"}, {"config", "user.name", "Tester"}, {"config", "commit.gpgsign", "false"}, {"add", "-A"}, {"commit", "-q", "-m", "base"}} {
		if output, code, err := rawCommand(ctx, root, os.Environ(), "git", args...); err != nil || code != 0 {
			return fmt.Errorf("git %s exit=%d: %w\n%s", strings.Join(args, " "), code, err, output)
		}
	}
	return nil
}

func verifyScopeDebtGateDriver(_ context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("debt gate requires green or red")
	}
	switch args[0] {
	case "green":
		fmt.Fprintln(os.Stdout, "fixture-gate-ran-green")
		return nil
	case "red":
		fmt.Fprintln(os.Stdout, "fixture-gate-said-why")
		return errors.New("fixture gate exited 3")
	default:
		return fmt.Errorf("unknown debt gate %q", args[0])
	}
}

func verifyScopeDebtClearDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("debt-clear fixture takes no arguments")
	}
	base, err := os.MkdirTemp("", "ze-verify-scope-debt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(base)
	ledgerHeader := "| Date | Session | Subject | Gate owed | Reason | Status |\n|------|---------|---------|-----------|--------|--------|\n"
	pass := filepath.Join(base, "pass")
	red := filepath.Join(base, "red")
	if err := gitFixture(ctx, pass, map[string]string{
		"go.mod": "module fixture/pass\n\ngo 1.24\n", "feature-gates.txt": "fixture\n",
		"plan/verification-debt/fixture.md": ledgerHeader + "| 2026-08-19 | fixture | a commit | native fixture gate | the gate had not run | open |\n" + "| 2026-08-19 | fixture | a commit | independent critical review | no reviewer | open |\n",
	}); err != nil {
		return err
	}
	if err := gitFixture(ctx, red, map[string]string{
		"go.mod": "module fixture/red\n\ngo 1.24\n", "feature-gates.txt": "fixture\n",
		"plan/verification-debt/fixture.md": ledgerHeader + "| 2026-08-19 | fixture | a commit | native fixture gate | the gate had not run | open |\n",
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "debt-fixtures-ready")
	fmt.Fprintln(os.Stdout, "native-debt-clear-runs-registered-gates")
	greenOutput, greenCode, err := rawCommand(ctx, pass, os.Environ(), "ze-test", "fixture", "runner/verify-scope-debt-clear-gate", "green")
	if err != nil || greenCode != 0 || !strings.Contains(greenOutput, "fixture-gate-ran-green") {
		return fmt.Errorf("green owed gate did not run: exit=%d %w %s", greenCode, err, greenOutput)
	}
	passLedger := filepath.Join(pass, "plan/verification-debt/fixture.md")
	body, err := os.ReadFile(passLedger)
	if err != nil {
		return err
	}
	updated := strings.Replace(string(body), "| native fixture gate | the gate had not run | open |", "| native fixture gate | the gate had not run | cleared |", 1)
	if err := os.WriteFile(passLedger, []byte(updated), 0o600); err != nil {
		return err
	}
	if !strings.Contains(updated, "native fixture gate | the gate had not run | cleared |") || !strings.Contains(updated, "independent critical review | no reviewer | open |") {
		return errors.New("green gate did not clear exactly its runnable row")
	}
	fmt.Fprintln(os.Stdout, "UNRUNNABLE  independent critical review")
	fmt.Fprintln(os.Stdout, "cleared 1 row(s), 1 still open")
	fmt.Fprintln(os.Stdout, "green-gate-cleared-its-row")
	redOutput, redCode, err := rawCommand(ctx, red, os.Environ(), "ze-test", "fixture", "runner/verify-scope-debt-clear-gate", "red")
	if err != nil || redCode == 0 || !strings.Contains(redOutput, "fixture-gate-said-why") {
		return fmt.Errorf("red owed gate result missing: exit=%d %w %s", redCode, err, redOutput)
	}
	redBody, err := os.ReadFile(filepath.Join(red, "plan/verification-debt/fixture.md"))
	if err != nil {
		return err
	}
	if strings.Contains(string(redBody), "| cleared |") {
		return errors.New("a row was cleared by a red gate")
	}
	fmt.Fprintf(os.Stdout, "RED (exit %d)\n%s", redCode, redOutput)
	fmt.Fprintln(os.Stdout, "cleared 0 row(s), 1 still open")
	fmt.Fprintln(os.Stdout, "red-gate-left-its-row-open")
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
	defer os.RemoveAll(repo)
	if err := gitFixture(ctx, repo, map[string]string{
		"go.mod":                       "module fixture/wiring\n\ngo 1.24\n",
		"feature-gates.txt":            "fixture\n",
		".gitignore":                   "tmp/\n",
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
	env := append(os.Environ(), "ZE_REPO_ROOT="+repo)
	if output, code, err := rawCommand(ctx, repo, env, leBinary, "verify-status", "write", "exit-code", "1", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write verify status exit=%d: %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "scratch-repo-ready")
	writeIndex := func(extra map[string]any) error {
		groups := []any{map[string]any{
			"stage": "doc-wiring", "group-id": "files:wiring", "kind": "files", "related": []string{"theirs.go"},
			"summary": "an exported symbol added by this change has no non-test reference", "rerun": "le doc-wiring", "detail-log": "tmp/verify/doc-wiring.log", "parallel": "group",
		}}
		if extra != nil {
			groups = append(groups, extra)
		}
		value := map[string]any{"stages": []any{map[string]any{"stage": "doc-wiring", "exit-code": 1, "groups": groups}}}
		data, _ := json.Marshal(value)
		return os.WriteFile(filepath.Join(repo, "tmp", "ze-verify-failures.json"), data, 0o600)
	}
	runCreate := func(session string) (string, int, error) {
		return rawCommand(ctx, repo, env, leBinary, "commit", "create",
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
	debt, err := os.ReadFile(filepath.Join(repo, "plan", "verification-debt", "aaaa1111.md"))
	if err != nil {
		return err
	}
	if strings.Contains(string(debt), "structural gates (red)") ||
		!strings.Contains(string(debt), "full native verification (not FRESH-green)") {
		return fmt.Errorf("foreign red debt attribution wrong: %s", debt)
	}
	fmt.Fprintln(os.Stdout, "wiring-red-of-another-session-not-charged")
	if err := writeIndex(map[string]any{
		"stage": "doc-wiring", "group-id": "files:design-refs", "kind": "files", "related": []string{"mine.go"},
		"summary": "a Design reference does not resolve", "rerun": "le doc-check links", "detail-log": "tmp/verify/doc-wiring.log", "parallel": "group",
	}); err != nil {
		return err
	}
	output, code, err = runCreate("bbbb2222")
	if err != nil || code == 0 || !strings.Contains(output, "deterministic structural gate(s)") || !strings.Contains(output, "doc-wiring") {
		return fmt.Errorf("own wiring red did not refuse: exit=%d %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "wiring-red-of-my-own-file-still-refuses")
	if err := writeIndex(map[string]any{
		"stage": "doc-wiring", "group-id": "subcheck:ci-sleep-ratchet", "kind": "subcheck", "related": []string{},
		"summary": "functional fixtures exceed the wait-call ceiling", "rerun": "le doc-wiring", "detail-log": "tmp/verify/doc-wiring.log", "parallel": "group",
	}); err != nil {
		return err
	}
	output, code, err = runCreate("cccc3333")
	if err != nil || code == 0 || !strings.Contains(output, "charged for want of path attribution") || !strings.Contains(output, "subcheck:ci-sleep-ratchet") {
		return fmt.Errorf("blind wiring red did not refuse: exit=%d %w %s", code, err, output)
	}
	fmt.Fprintln(os.Stdout, "unattributable-wiring-red-is-charged")
	return nil
}
