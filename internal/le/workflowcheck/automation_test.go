// Design: docs/architecture/testing/ci-workflows.md -- event selection and immutable actions

package workflowcheck

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// TestWorkflowActionsUseImmutableCommits reads real YAML steps and reusable
// jobs, so a pinned example in a comment cannot hide a mutable action.
func TestWorkflowActionsUseImmutableCommits(t *testing.T) {
	immutable := regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}$`)
	for _, name := range workflowNames(t) {
		var document struct {
			Jobs map[string]struct {
				Uses  string `yaml:"uses"`
				Steps []struct {
					Uses string `yaml:"uses"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(workflowSource(t, name)), &document); err != nil {
			t.Fatal(err)
		}
		for jobName, job := range document.Jobs {
			refs := []string{job.Uses}
			for _, step := range job.Steps {
				refs = append(refs, step.Uses)
			}
			for _, ref := range refs {
				if ref == "" {
					continue
				}
				if strings.HasPrefix(ref, "./") {
					continue
				}
				if !immutable.MatchString(ref) {
					t.Errorf("%s job %s uses mutable action %q", name, jobName, ref)
				}
			}
		}
	}
}

// TestVerifyCancelsOnlySupersededPullRequests pins the actual expression:
// unrelated events get a unique run group, including every main push/dispatch.
func TestVerifyCancelsOnlySupersededPullRequests(t *testing.T) {
	var document struct {
		Concurrency struct {
			Group  string `yaml:"group"`
			Cancel string `yaml:"cancel-in-progress"`
		} `yaml:"concurrency"`
		Jobs map[string]struct {
			Needs string `yaml:"needs"`
			If    string `yaml:"if"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflowSource(t, "verify.yml")), &document); err != nil {
		t.Fatal(err)
	}
	want := "${{ github.workflow }}-${{ github.event_name == 'pull_request' && format('pr-{0}', github.event.pull_request.number) || format('run-{0}', github.run_id) }}"
	if document.Concurrency.Group != want {
		t.Errorf("verify concurrency group = %q, want %q", document.Concurrency.Group, want)
	}
	if document.Concurrency.Cancel != "${{ github.event_name == 'pull_request' }}" {
		t.Errorf("verify cancels events other than superseded PRs: %q", document.Concurrency.Cancel)
	}
	job := document.Jobs["verify"]
	if job.Needs != "select" {
		t.Error("verify shards must wait for event selection")
	}
	if job.If != "${{ needs.select.outputs.run == 'yes' }}" {
		t.Errorf("verify shard selection = %q", job.If)
	}
}

// TestVerifyEventSelectionPreservesIndependentEvidence executes the workflow's
// selection script with a fake GitHub CLI. Failed or unreadable queries run the
// push; only a real verification for an open PR at this repository/SHA replaces it.
func TestVerifyEventSelectionPreservesIndependentEvidence(t *testing.T) {
	script := workflowStepRun(t, "verify.yml", "select", "Select the verification event")
	for _, required := range []string{
		`-f "head=$GITHUB_REPOSITORY_OWNER:$BRANCH"`,
		`.head.repo.full_name == env.GITHUB_REPOSITORY`,
		`.head.sha == env.GITHUB_SHA`,
		`--paginate --slurp`,
		`actions/workflows/verify.yml/runs`,
		`-f event=pull_request -f "head_sha=$GITHUB_SHA"`,
		`.head_repository.full_name == env.GITHUB_REPOSITORY`,
		`.head_sha == env.GITHUB_SHA`,
		`.status == "queued" or .status == "in_progress"`,
		`.conclusion == "success" or .conclusion == "failure" or .conclusion == "timed_out"`,
		`any(.pull_requests[]?; .number as $number | $open | index($number) != null)`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("event selection does not require %q", required)
		}
	}
	for _, test := range []struct {
		name      string
		event     string
		ref       string
		branch    string
		matches   string
		failure   string
		prFailure string
		want      string
		query     bool
	}{
		{name: "main evidence", event: "push", ref: "branch", branch: "main", matches: "1", want: "yes"},
		{name: "tag evidence", event: "push", ref: "tag", branch: "v1", matches: "1", want: "yes"},
		{name: "manual Dependabot evidence", event: "workflow_dispatch", ref: "branch", branch: "dependabot/go_modules/all", matches: "1", want: "yes"},
		{name: "pull request", event: "pull_request", ref: "branch", branch: "42/merge", matches: "1", want: "yes"},
		{name: "branch without PR", event: "push", ref: "branch", branch: "topic", matches: "0", want: "yes", query: true},
		{name: "matching PR", event: "push", ref: "branch", branch: "topic", matches: "1", want: "no", query: true},
		{name: "API failure", event: "push", ref: "branch", branch: "topic", failure: "1", want: "yes", query: true},
		{name: "PR lookup failure", event: "push", ref: "branch", branch: "topic", prFailure: "1", want: "yes", query: true},
		{name: "empty answer", event: "push", ref: "branch", branch: "topic", want: "yes", query: true},
		{name: "malformed answer", event: "push", ref: "branch", branch: "topic", matches: "unknown", want: "yes", query: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			stub := "#!/bin/bash\nprintf query > \"$QUERY_FILE\"\ncase \"$*\" in\n  *actions/workflows*) printf '%s\\n' \"$MATCHES\"; exit \"${FAILURE:-0}\" ;;\n  *) printf '[42]\\n'; exit \"${PR_FAILURE:-0}\" ;;\nesac\n"
			if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0o700); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "output")
			query := filepath.Join(dir, "query")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-euo", "pipefail", "-c", script) // #nosec G204 -- Run the repository workflow against task-owned CLI fixtures.
			command.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"),
				"EVENT="+test.event, "REF_TYPE="+test.ref, "BRANCH="+test.branch,
				"DEFAULT_BRANCH=main", "GITHUB_REPOSITORY=ze-software/ze", "GITHUB_REPOSITORY_OWNER=ze-software",
				"GITHUB_SHA=abc123", "GITHUB_OUTPUT="+output, "QUERY_FILE="+query,
				"MATCHES="+test.matches, "FAILURE="+test.failure, "PR_FAILURE="+test.prFailure)
			if result, err := command.CombinedOutput(); err != nil {
				t.Fatalf("event selection failed: %v\n%s", err, result)
			}
			if got := readFile(t, output); got != "run="+test.want+"\n" {
				t.Errorf("selection = %q, want run=%s", got, test.want)
			}
			_, err := os.Stat(query)
			if test.query {
				if err != nil {
					t.Errorf("branch push did not query matching PRs: %v", err)
				}
				return
			}
			if !os.IsNotExist(err) {
				t.Errorf("independent evidence queried PRs: %v", err)
			}
		})
	}
}

// TestPerfNightlyRefusesMissingEvidence runs the workflow shell in an empty
// directory, proving the nightly cannot turn an absent history into success.
func TestPerfNightlyRefusesMissingEvidence(t *testing.T) {
	script := workflowStepRun(t, "perf-nightly.yml", "perf-regression-check", "Perf regression check")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", "-euo", "pipefail", "-c", script) // #nosec G204 -- Run the repository workflow against an empty evidence directory.
	command.Dir = t.TempDir()
	result, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("nightly succeeded without any performance evidence")
	}
	if !strings.Contains(string(result), "::error::") {
		t.Fatalf("missing evidence has no actionable workflow error: %s", result)
	}
}

// TestPerfNightlyPropagatesTheComparisonVerdict supplies history and a fake le
// boundary, proving both the exact comparison argv and its success/failure exit.
func TestPerfNightlyPropagatesTheComparisonVerdict(t *testing.T) {
	script := workflowStepRun(t, "perf-nightly.yml", "perf-regression-check", "Perf regression check")
	for _, test := range []struct {
		name string
		code string
	}{
		{name: "comparison passed", code: "0"},
		{name: "comparison refused", code: "37"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			historyDir := filepath.Join(dir, "test", "perf", "history")
			if err := os.MkdirAll(historyDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(historyDir, "ze.ndjson"), []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			stub := "#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$ARGV_FILE\"\nexit \"$COMPARISON_CODE\"\n"
			if err := os.WriteFile(filepath.Join(dir, "le"), []byte(stub), 0o700); err != nil {
				t.Fatal(err)
			}
			argv := filepath.Join(dir, "argv")
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, "bash", "-euo", "pipefail", "-c", script) // #nosec G204 -- Run the repository workflow against a task-owned comparison fixture.
			command.Dir = dir
			command.Env = append(os.Environ(), "ARGV_FILE="+argv, "COMPARISON_CODE="+test.code)
			result, err := command.CombinedOutput()
			if test.code == "0" {
				if err != nil {
					t.Fatalf("comparison passed but nightly failed: %v\n%s", err, result)
				}
			} else {
				if err == nil {
					t.Fatal("nightly hid the comparison failure")
				}
				var failure *exec.ExitError
				if !errors.As(err, &failure) {
					t.Fatalf("comparison did not run: %v\n%s", err, result)
				}
				if failure.ExitCode() != 37 {
					t.Fatalf("nightly exit = %d, want 37: %s", failure.ExitCode(), result)
				}
			}
			if got := readFile(t, argv); got != "perf\ntrack\n--check\ntest/perf/history/ze.ndjson\n" {
				t.Errorf("comparison argv = %q", got)
			}
		})
	}
}

// workflowStepRun reads only a real run step; examples in comments and step
// names are never executed as the workflow under test.
func workflowStepRun(t *testing.T, name, jobName, stepName string) string {
	t.Helper()
	var document struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflowSource(t, name)), &document); err != nil {
		t.Fatal(err)
	}
	for _, step := range document.Jobs[jobName].Steps {
		if step.Name == stepName {
			if step.Run == "" {
				t.Fatalf("%s job %s step %s has no run script", name, jobName, stepName)
			}
			return step.Run
		}
	}
	t.Fatalf("%s job %s has no step %s", name, jobName, stepName)
	return ""
}
