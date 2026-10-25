package fixture

// Design: docs/contributing/running-commands.md -- what le's exit codes mean
// Related: register_le_exit_codes.go -- the registration.
// Related: ui_fixture_le_help_pages.go -- leHelpPagesBuild, the build this driver shares.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// leExitCodesLine is one le command line and the code D-Q8 gives it.
type leExitCodesLine struct {
	why   string
	dir   string // "" runs in the checkout
	stdin string
	args  []string
	code  int
}

// leExitCodes builds the le personality and runs one line per meaning of le's
// exit codes through the binary (spec-le-cli-usability AC-28 to AC-32): usage
// errors answer 2, a check that ran and found a problem 1, a gate with no tree
// to read 3, and the declared exemptions their own codes: the hook protocol's
// 2 for a block and a pass-through program's code as it is. `le help ai
// exit-codes` names each exemption.
func leExitCodes(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-exit-codes-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	lePath, _, err := leHelpPagesBuild(ctx, root, work)
	if err != nil {
		return err
	}
	empty := filepath.Join(work, "no-checkout")
	if err := os.MkdirAll(empty, 0o750); err != nil {
		return fmt.Errorf("create the directory with no checkout: %w", err)
	}

	for _, line := range []leExitCodesLine{
		{why: "an unknown command", args: []string{"no-such-command-exit-codes"}, code: 2},
		{why: "a word the ai namespace does not hold", args: []string{"ai", "no-such-member-exit-codes"}, code: 2},
		{why: "a stray word after a verb", args: []string{areaCLI, "grammar", actionCheck, "stray-exit-codes"}, code: 2},
		{why: "a value outside an enum", args: []string{"weekly", "channel", "no-such-channel"}, code: 2},
		{why: "a review-check over a spec with no review", args: []string{"commit", "review-check", "spec", "spec-no-review-exit-codes"}, code: 1},
		{why: "a grammar check with no checkout to read", dir: empty, args: []string{areaCLI, "grammar", actionCheck}, code: 3},
		{why: "an rfc check with no checkout to read", dir: empty, args: []string{"rfc", actionCheck}, code: 3},
		{why: "a hook that blocks (the hook protocol)", stdin: `{"tool_name":"Bash","tool_input":{"command":"git push --force"}}`,
			args: []string{"ai", "hooks", "pretool-bash"}, code: 2},
		{why: "a pass-through program's own failure", stdin: "not json",
			args: []string{leExitCodesAreaTest, "json-ip-bytes", "--ip", "192.0.2.1"}, code: 1},
	} {
		got, err := leExitCodesRun(ctx, lePath, root, line)
		if err != nil {
			return err
		}
		if got.exitCode != line.code {
			return fmt.Errorf("%s: le %s answered %d, want %d\nstdout:\n%s\nstderr:\n%s",
				line.why, strings.Join(line.args, " "), got.exitCode, line.code, got.stdout, got.stderr)
		}
	}

	table, err := leExitCodesRun(ctx, lePath, root, leExitCodesLine{args: []string{leHelpPagesHelpWord, "ai", "exit-codes"}})
	if err != nil {
		return err
	}
	if table.exitCode != 0 {
		return fmt.Errorf("le help ai exit-codes answered %d, stderr %q", table.exitCode, table.stderr)
	}
	for _, want := range []string{"le ai hooks", "test fixture", "mock", "pass-through", "usage error", "could not run"} {
		if !strings.Contains(table.stdout, want) {
			return fmt.Errorf("le help ai exit-codes does not name %q:\n%s", want, table.stdout)
		}
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// leExitCodesRun runs one line and answers its streams and exit code. The
// session identity is left out of the environment, so a hook run here records
// nothing against the session that started the test.
func leExitCodesRun(ctx context.Context, lePath, root string, line leExitCodesLine) (leHelpPagesResult, error) {
	cmd := exec.CommandContext(ctx, lePath, line.args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = root
	if line.dir != "" {
		cmd.Dir = line.dir
	}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "CLAUDE_") {
			continue
		}
		// A line run outside the checkout must not be handed one: le reads
		// ZE_REPO_ROOT (either spelling) before the working directory, and
		// the runner exports it.
		if line.dir != "" && leExitCodesNamesRoot(entry) {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Stdin = strings.NewReader(line.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := leHelpPagesResult{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("run le %s: %w", strings.Join(line.args, " "), err)
	}
	return result, nil
}

// leExitCodesNamesRoot reports whether an environment entry names the checkout
// root, in the underscore or the dotted spelling env.Get accepts.
func leExitCodesNamesRoot(entry string) bool {
	name, _, _ := strings.Cut(entry, "=")
	return strings.EqualFold(strings.ReplaceAll(name, ".", "_"), "ZE_REPO_ROOT")
}

// leExitCodesAreaTest is le's test area, whose pass-through children answer
// their own program's code.
const leExitCodesAreaTest = "test"
