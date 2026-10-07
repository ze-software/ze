// Design: docs/contributing/feature-maturity.md -- the `./le feature` verbs
//
// The unit tests in internal/le/feature drive Check over fixture trees. This
// fixture drives the BUILT le binary through argv over a committed tree of its
// own, which is the path a developer is on: the words they type, the exit code
// their shell reads, and the payload `| json` renders.
//
// Related: register_le_feature_answers.go -- the registration.

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

	"github.com/ze-software/ze/internal/le/rfc"
)

// leFeatureDeclaration is an experimental daemon feature whose real-path test
// exists and has no recorded green run.
const leFeatureDeclaration = "# Widget\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
	"| Name | Widgets |\n| Kind | daemon |\n| Scope | complete |\n" +
	"| Level | experimental |\n| Components | internal/widget |\n" +
	"| Real-path tests | test/plugin/widget.ci |\n| Docs | docs/widget.md |\n" +
	"| Doc review | 2026-10-07: the page and the row prose read against Send |\n" +
	"| Defect review | 2026-10-07: no journal row or immediate spec names internal/widget |\n\n" +
	"## Description\n\nWidgets are sent to every peer.\n"

type leFeatureAnswer struct {
	stdout string
	stderr string
	code   int
}

func leFeatureAnswers(ctx context.Context) error {
	binary, err := nativeLEBinary()
	if err != nil {
		return fmt.Errorf("le-feature-answers: %w", err)
	}
	tree, err := os.MkdirTemp("", "le-feature-answers-")
	if err != nil {
		return fmt.Errorf("le-feature-answers: %w", err)
	}
	defer os.RemoveAll(tree) //nolint:errcheck // a temporary tree; nothing reads it after the run

	if err := leFeatureTree(ctx, tree, leFeatureDeclaration); err != nil {
		return err
	}

	// The declaration stands at Experimental: check exits 0.
	check, err := leFeatureRun(ctx, tree, binary, "feature", "check")
	if err != nil {
		return err
	}
	if check.code != 0 {
		return fmt.Errorf("le-feature-answers: `le feature check` exited %d over an experimental declaration: %s%s",
			check.code, check.stdout, check.stderr)
	}

	// The report names the ceiling and the unmet Supported criterion (D-6).
	report, err := leFeatureRun(ctx, tree, binary, "feature", "report", "feature", "widget", "|", "json")
	if err != nil {
		return err
	}
	if report.code != 0 {
		return fmt.Errorf("le-feature-answers: `le feature report feature widget | json` exited %d: %s", report.code, report.stderr)
	}
	var entries []map[string]any
	if err := json.Unmarshal([]byte(report.stdout), &entries); err != nil {
		return fmt.Errorf("le-feature-answers: the report is not a JSON array: %w: %q", err, report.stdout)
	}
	if len(entries) != 1 {
		return fmt.Errorf("le-feature-answers: the report holds %d entries, want 1: %q", len(entries), report.stdout)
	}
	if entries[0]["ceiling"] != "experimental" {
		return fmt.Errorf("le-feature-answers: ceiling %v, want experimental", entries[0]["ceiling"])
	}
	if !strings.Contains(report.stdout, "exists, not run") {
		return fmt.Errorf("le-feature-answers: the report does not name the unrun test: %q", report.stdout)
	}

	// Declared Supported with no recorded green run: refused, exit 1.
	supported := strings.Replace(leFeatureDeclaration, "| Level | experimental |", "| Level | supported |", 1)
	if err := os.WriteFile(filepath.Join(tree, "features", "widget.md"), []byte(supported), 0o600); err != nil {
		return fmt.Errorf("le-feature-answers: %w", err)
	}
	refused, err := leFeatureRun(ctx, tree, binary, "feature", "check", "|", "json")
	if err != nil {
		return err
	}
	if refused.code != 1 {
		return fmt.Errorf("le-feature-answers: `le feature check` exited %d over a supported declaration with no run, want 1: %s%s",
			refused.code, refused.stdout, refused.stderr)
	}
	var verdict struct {
		Declarations int      `json:"declarations"`
		Refused      []string `json:"refused"`
	}
	if err := json.Unmarshal([]byte(refused.stdout), &verdict); err != nil {
		return fmt.Errorf("le-feature-answers: the check answer is not a JSON object: %w: %q", err, refused.stdout)
	}
	if verdict.Declarations != 1 {
		return fmt.Errorf("le-feature-answers: the check judged %d declarations, want 1", verdict.Declarations)
	}
	if !strings.Contains(strings.Join(verdict.Refused, "\n"), "exists, not run") {
		return fmt.Errorf("le-feature-answers: the refusal does not name the unrun test: %v", verdict.Refused)
	}

	fmt.Println("OK: check accepts the evidence level, report names the ceiling and the unrun test, and Supported without a recorded run is refused") //nolint:forbidigo // the fixture protocol reads this line from stdout
	return nil
}

// leFeatureTree writes and commits the tree the feature check judges. The
// commit is dated before the reviews, so the staleness criteria read it current.
func leFeatureTree(ctx context.Context, tree, declaration string) error {
	files := rfc.FixtureFiles()
	files["features/widget.md"] = declaration
	files["internal/widget/widget.go"] = "package widget\n\nfunc Send() {}\n"
	files["test/plugin/widget.ci"] = "cmd=foreground:seq=1:exec=ze\n"
	files["docs/widget.md"] = "# Widget\n"
	for rel, content := range files {
		full := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return fmt.Errorf("le-feature-answers: %w", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			return fmt.Errorf("le-feature-answers: %w", err)
		}
	}
	const date = "2026-10-01T12:00:00Z"
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{sourceConfig, "user.email", "test@example.com"},
		{sourceConfig, "user.name", "Ze Test"},
		{sourceConfig, "commit.gpgsign", valueFalse},
		{argAdd, "-A"},
		{"commit", "--quiet", "--message=seed"},
	} {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", tree}, args...)...) // #nosec G204 -- every argv word is written in this file
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("le-feature-answers: git %s: %w: %s", args[0], err, output)
		}
	}
	return nil
}

// leFeatureRun runs the le binary once against tree. A nonzero exit is DATA.
func leFeatureRun(ctx context.Context, tree, binary string, args ...string) (leFeatureAnswer, error) {
	command := exec.CommandContext(ctx, binary, args...) // #nosec G204 -- every argv word is written in this file
	command.Dir = tree
	command.Env = append(os.Environ(), "ZE_REPO_ROOT="+tree)
	var out, errOut bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errOut

	err := command.Run()
	answer := leFeatureAnswer{stdout: out.String(), stderr: errOut.String()}
	if err == nil {
		return answer, nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return leFeatureAnswer{}, fmt.Errorf("le-feature-answers: run le %s: %w", strings.Join(args, " "), err)
	}
	answer.code = exit.ExitCode()
	return answer, nil
}
