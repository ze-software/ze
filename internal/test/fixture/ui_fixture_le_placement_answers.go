package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	Register("ui/le-placement-answers", uiDriver(lePlacementAnswers))
}

var leANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type leCommandResult struct {
	code   int
	stdout string
	stderr string
}

func lePlacementAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("FAIL: ZE_REPO_ROOT is not set")
	}

	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("FAIL: resolving ZE_REPO_ROOT: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("FAIL: resolving the checkout: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("FAIL: stating the checkout: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("FAIL: ZE_REPO_ROOT is not a directory: %s", root)
	}

	work, err := os.MkdirTemp("", "le-placement-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: creating fixture directory: %w", err)
	}
	defer os.RemoveAll(work)

	work, err = filepath.EvalSymlinks(work)
	if err != nil {
		return fmt.Errorf("FAIL: resolving fixture directory: %w", err)
	}
	if lePathIsWithin(root, work) {
		return fmt.Errorf("FAIL: fixture directory %s is inside the checkout", work)
	}

	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	command := func(args ...string) (leCommandResult, error) {
		return leRun(ctx, root, binary, args...)
	}

	// The checkout is expected to satisfy every placement gate. Valid gate
	// pages are data on stdout; stderr remains reserved for diagnostics.
	if _, err := leExpectSuccessfulPage(command, "./le digest", "digest"); err != nil {
		return err
	}
	if _, err := leExpectSuccessfulPage(command, "./le repository tree-check", "repository", "tree-check"); err != nil {
		return err
	}
	if _, err := leExpectSuccessfulPage(command, "./le repository check", "repository", "check"); err != nil {
		return err
	}
	if _, err := leExpectSuccessfulPage(command, "./le tier check", "tier", "check"); err != nil {
		return err
	}
	if _, err := leExpectSuccessfulPage(command, "./le tier selftest", "tier", "selftest"); err != nil {
		return err
	}
	if _, err := leExpectSuccessfulPage(command, "tier report", "tier", "report"); err != nil {
		return err
	}

	// Run the report again for its coverage assertions. This also verifies that
	// a read-only gate is stable across consecutive invocations.
	audit, err := leExpectSuccessfulPage(command, "tier report coverage", "tier", "report")
	if err != nil {
		return err
	}
	areas := strings.Count(audit.stdout, "\nAREA: ")
	if areas != 3 {
		return fmt.Errorf("FAIL: the audit covered %d registries, want three", areas)
	}
	if strings.Count(audit.stdout, "\n  ") <= 100 {
		return errors.New("FAIL: the audit lists almost no subsystem, so the result is vacuous")
	}

	// Each payload must pass through the command engine's rendering operators.
	renders := []struct {
		args   []string
		marker string
	}{
		{[]string{"digest", "|", "json"}, `"anchors"`},
		{[]string{"digest", "|", "yaml"}, "digests:"},
		{[]string{"repository", "tree-check", "|", "json"}, `"findings"`},
		{[]string{"repository", "tree-check", "|", "yaml"}, "issues:"},
		{[]string{"tier", "check", "|", "json"}, `"checks"`},
		{[]string{"tier", "report", "|", "json"}, `"module"`},
		{[]string{"tier", "|", "table"}, "write-baseline"},
		{[]string{"repository", "|", "table"}, "tree-check"},
	}
	for _, render := range renders {
		got, err := command(render.args...)
		if err != nil {
			return err
		}
		if got.code != 0 && got.code != 1 {
			return fmt.Errorf("FAIL: %q exited %d: %s", render.args, got.code, got.stderr)
		}
		if !strings.Contains(got.stdout, render.marker) {
			return fmt.Errorf("FAIL: %q did not render %q:\n%s", render.args, render.marker, got.stdout)
		}
	}

	// count selects the only list in the payload.
	counted, err := command("tier", "check", "|", "count")
	if err != nil {
		return err
	}
	if strings.TrimSpace(counted.stdout) != "5" {
		return fmt.Errorf("FAIL: the check reports %q checks, want five", strings.TrimSpace(counted.stdout))
	}

	// Refusals retain the boundary-specific status assigned by the command
	// engine and identify the reason on stderr.
	refused, err := command("digest", "nope")
	if err != nil {
		return err
	}
	if refused.code != 1 {
		return fmt.Errorf("FAIL: a value after a single-gate command exited %d, want 1", refused.code)
	}
	if !strings.Contains(refused.stderr, "takes no arguments") {
		return fmt.Errorf("FAIL: the refusal does not say why: %s", refused.stderr)
	}

	refused, err = command("repository", "nope")
	if err != nil {
		return err
	}
	if refused.code != 2 {
		return fmt.Errorf("FAIL: an unknown action exited %d, want 2", refused.code)
	}
	if !strings.Contains(refused.stderr, "no such action in repository") {
		return fmt.Errorf("FAIL: the refusal does not name the area: %s", refused.stderr)
	}

	refused, err = command("tier", "check", "extra")
	if err != nil {
		return err
	}
	if refused.code != 2 {
		return fmt.Errorf("FAIL: a value after an action exited %d, want 2", refused.code)
	}

	// Help is intentionally allowed on either stream because the command engine
	// writes an unpiped page to stderr.
	listed, err := command("--help")
	if err != nil {
		return err
	}
	page := listed.stdout + listed.stderr
	for _, name := range []string{"digest", "repository", "tier"} {
		if !strings.Contains(page, "\n  "+name+" ") {
			return fmt.Errorf("FAIL: `le --help` does not list %s:\n%s", name, page)
		}
	}

	fmt.Println("PASS: the three placement gates satisfy their checkout contract")
	return nil
}

func leExpectSuccessfulPage(command func(...string) (leCommandResult, error), name string, args ...string) (leCommandResult, error) {
	got, err := command(args...)
	if err != nil {
		return leCommandResult{}, err
	}
	if got.code != 0 {
		return leCommandResult{}, fmt.Errorf("FAIL: %s exited %d\nstdout:\n%sstderr:\n%s", name, got.code, got.stdout, got.stderr)
	}
	if got.stderr != "" {
		return leCommandResult{}, fmt.Errorf("FAIL: %s wrote a diagnostic on success:\n%s", name, got.stderr)
	}
	if strings.TrimSpace(leANSI.ReplaceAllString(got.stdout, "")) == "" {
		return leCommandResult{}, fmt.Errorf("FAIL: %s rendered an empty page", name)
	}
	return got, nil
}

func leRun(ctx context.Context, dir, executable string, args ...string) (leCommandResult, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := leCommandResult{
		code:   0,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return leCommandResult{}, fmt.Errorf("FAIL: %s was interrupted: %w", executable, ctxErr)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return leCommandResult{}, fmt.Errorf("FAIL: executing %s: %w", executable, err)
}

func lePathIsWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
