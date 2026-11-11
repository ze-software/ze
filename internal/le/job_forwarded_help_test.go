// Design: docs/contributing/running-commands.md -- every word after command belongs to the child.
package le

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

const forwardedHelpOutput = "JOB_FORWARDED_HELP_OUTPUT"
const forwardedHelpDispatchChild = "JOB_FORWARDED_HELP_DISPATCH_CHILD"

// TestJobDispatchRunsChildHelpArguments runs the actual job dispatcher and a
// child process. Its final help word must retain its exact argument boundary,
// execute the child, preserve its output in the quiet log and answer its code.
func TestJobDispatchRunsChildHelpArguments(t *testing.T) {
	for _, word := range []string{"--help", "-h", "-help", "help"} {
		t.Run(word, func(t *testing.T) {
			root, output := forwardedHelpFixture(t)
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			argv := []string{binary, "-test.run=^TestJobForwardedHelpChild$", "--", "argument with spaces", word}
			line := append([]string{"job", "run", "label", "unit-pkg", "quiet", "command"}, argv...)
			code, answer := forwardedHelpDispatch(t, root, output, line)
			if code != 23 {
				t.Fatalf("job dispatch answered %d, want the executed child's 23; answer:\n%s", code, answer)
			}
			body, err := os.ReadFile(output)
			if err != nil {
				t.Fatalf("child did not publish its executed argv: %v", err)
			}
			var got []string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode executed child argv: %v", err)
			}
			if !slices.Equal(got, argv) {
				t.Fatalf("executed argv = %q, want %q", got, argv)
			}
			logs, err := filepath.Glob(filepath.Join(root, "tmp", "session", "*", "scratch", "job-unit-pkg-*.log"))
			if err != nil {
				t.Fatal(err)
			}
			if len(logs) != 1 {
				t.Fatalf("job created %d quiet logs, want the child's one durable log: %v", len(logs), logs)
			}
			logged, err := os.ReadFile(logs[0])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(logged), "forwarded-help-child-ran\n"+string(body)+"\n") {
				t.Fatalf("quiet log lost the executed child's sentinel or exact argv:\n%s", logged)
			}
			if !strings.Contains(answer, "job unit-pkg: exit 23, log ") {
				t.Fatalf("dispatch lost the real job verdict and durable log path:\n%s", answer)
			}
		})
	}
}

// TestJobDispatchOwnHelpDoesNotRunAChild pairs forwarded help with the job
// action's own question. Its usage must answer before admission or execution.
func TestJobDispatchOwnHelpDoesNotRunAChild(t *testing.T) {
	root, output := forwardedHelpFixture(t)
	code, answer := forwardedHelpDispatch(t, root, output, []string{"job", "run", "--help"})
	if code != 0 || !strings.Contains(answer, "le job run") || !strings.Contains(answer, "command") {
		t.Fatalf("job's own help = %d, %q; want its declared usage with 0", code, answer)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("job's own help executed a child or could not establish its absence: %v", err)
	}
	logs, err := filepath.Glob(filepath.Join(root, "tmp", "session", "*", "scratch", "job-*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 {
		t.Fatalf("job's own help admitted work and wrote quiet logs: %v", logs)
	}
}

func forwardedHelpFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("tmp/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"add", ".gitignore"},
		{"-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "fixture"},
	} {
		command := exec.CommandContext(t.Context(), "git", arguments...)
		command.Dir = root
		command.Env = env.Without(os.Environ(), "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("prepare disposable Git fixture %q: %v\n%s", arguments, err, output)
		}
	}
	output := filepath.Join(t.TempDir(), "executed-argv.json")
	return root, output
}

// forwardedHelpDispatch enters the complete maintained composition in its own
// process. Child-only environment overrides cannot redirect another test's root
// or make this fixture run inside the real admission that launched the package.
func forwardedHelpDispatch(t *testing.T, root, output string, line []string) (int, string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	arguments := append([]string{"-test.run=^TestJobHelpDispatchChild$", "--"}, line...)
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = root
	command.Env = append(env.Without(os.Environ(), job.ParentKey, lepath.RootKey, "ze.agent.id"),
		lepath.RootKey+"="+root, "CLAUDE_CODE_SESSION_ID=forwarded-help-fixture", "ze.agent.id=",
		forwardedHelpOutput+"="+output, forwardedHelpDispatchChild+"=1")
	answer, err := command.CombinedOutput()
	if err == nil {
		return 0, string(answer)
	}
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatalf("start composed dispatcher fixture: %v\n%s", err, answer)
	}
	return exited.ExitCode(), string(answer)
}

// TestJobHelpDispatchChild supplies the full registration tree from register.go
// without replacing its namespace declarations or disabling their validation.
func TestJobHelpDispatchChild(t *testing.T) {
	if os.Getenv(forwardedHelpDispatchChild) == "" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("composed dispatcher fixture received no argument separator")
	}
	env.ResetCache()
	os.Exit(leroot.Dispatch("le", os.Args[separator+1:]))
}

// TestJobForwardedHelpChild is the executed process, not a parser stand-in.
// Without the fixture's output path it has no work during an ordinary run.
func TestJobForwardedHelpChild(t *testing.T) {
	path := os.Getenv(forwardedHelpOutput)
	if path == "" {
		return
	}
	argv, err := json.Marshal(os.Args)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, argv, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stdout.WriteString("forwarded-help-child-ran\n" + string(argv) + "\n"); err != nil {
		t.Fatal(err)
	}
	os.Exit(23)
}
