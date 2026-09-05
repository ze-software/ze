package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// redsStageLog renders one stage log the way the verify engine writes it
// (internal/le/verify/engine/run.go, runMode): the header, the stage's own
// output, then the result line that closes it.
func redsStageLog(stage string, code int, body ...string) string {
	lines := append([]string{"### Stage: " + stage}, body...)
	lines = append(lines, "### Stage result: "+stage+" exit="+strconv.Itoa(code), "")

	return strings.Join(lines, "\n")
}

// verifyRedsInFlightDriver asks a verification run that has written 3 of its
// stage logs whether it has reddened one file, and asserts that the answer
// arrives, names the reddened file, states the unreported remainder, and never
// renders the unfinished run as a pass.
func verifyRedsInFlightDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("verify-reds fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-reds-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	empty, err := os.MkdirTemp("", "ze-verify-reds-empty-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(empty) //nolint:errcheck // fixture cleanup

	if err := redsScratchRun(repo); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "in-flight-run-ready") //nolint:errcheck // progress output

	run := func(root string, arguments ...string) (string, int, error) {
		return rawCommand(ctx, root, envRootedAt(root), le, arguments...)
	}

	out, code, err := run(repo, "verify", "reds", "file", "internal/mine/a.go")
	if err != nil || code == 0 {
		return fmt.Errorf("untouched path exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "undetermined") {
		return fmt.Errorf("untouched path verdict is not undetermined: %s", out)
	}
	if !strings.Contains(out, "not reported yet") || !strings.Contains(out, "3 of ") {
		return fmt.Errorf("the answer does not state the unreported remainder: %s", out)
	}
	fmt.Fprintln(os.Stdout, "reds-answers-before-the-run-ends") //nolint:errcheck // progress output

	out, code, err = run(repo, "verify", "reds", "file", "internal/theirs/b.go")
	if err != nil || code == 0 || !strings.Contains(out, "verdict: named") {
		return fmt.Errorf("reddened path exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "verify lint/run") {
		return fmt.Errorf("the answer does not name the stage that reddened the file: %s", out)
	}
	fmt.Fprintln(os.Stdout, "reds-names-the-reddened-file") //nolint:errcheck // progress output

	out, code, err = run(repo, "verify", "reds", "file", "internal/mine/a.go", "|", "json")
	if err != nil || code == 0 {
		return fmt.Errorf("json rendering exit=%d: %w %s", code, err, out)
	}
	for _, key := range []string{`"verdict"`, `"pending"`, `"reported"`, `"stages"`} {
		if !strings.Contains(out, key) {
			return fmt.Errorf("the json answer holds no %s: %s", key, out)
		}
	}
	fmt.Fprintln(os.Stdout, "reds-renders-as-json") //nolint:errcheck // progress output

	out, code, err = run(empty, "verify", "reds", "file", "internal/mine/a.go")
	if err != nil || code == 0 {
		return fmt.Errorf("no-run exit=%d: %w %s", code, err, out)
	}
	if !strings.Contains(out, "no-run") || strings.Contains(out, "not-named") {
		return fmt.Errorf("a checkout with no run did not say so: %s", out)
	}
	fmt.Fprintln(os.Stdout, "reds-refuses-to-answer-with-no-run") //nolint:errcheck // progress output

	return nil
}

// redsScratchRun writes a checkout holding two Go files and one full-mode run
// that has reached its third stage: lint red about theirs/b.go, tier green, and
// rfc broken before it could say which files it was about.
func redsScratchRun(repo string) error {
	files := map[string]string{
		fileGoMod:              "module fixture/reds\n\ngo 1.24\n",
		fileFeatureGates:       contentFeatureGate,
		"internal/mine/a.go":   "package mine\n",
		"internal/theirs/b.go": "package theirs\n",
		"tmp/verify/full-inflight/01-verify-lint-run.log": redsStageLog("verify lint/run", 1,
			`VERIFY FAILURE GROUP: {"group-id":"lint:theirs","kind":"lint",`+
				`"related":["internal/theirs/b.go"],"summary":"golangci-lint reported findings",`+
				`"rerun":"./le verify lint run"}`,
			"VERIFY FAILURE GROUPS COMPLETE: 1"),
		"tmp/verify/full-inflight/02-tier-check.log": redsStageLog("tier/check", 0, "OK"),
		"tmp/verify/full-inflight/03-rfc-check.log": redsStageLog("rfc/check", 2,
			"the stage broke before it could say which files it was about"),
	}
	for path, body := range files {
		full := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			return err
		}
	}

	return nil
}
