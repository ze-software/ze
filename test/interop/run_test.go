// Guard for the BGP interop runner's missing-lab polarity
// (plan/spec-rfcgate-2-evidence.md AC-1).
//
// test/interop/run.py used to print "Docker unavailable, skipping interop tests"
// and exit 0, while its sibling test/ipsec-interop/run.py exits 1 on the same
// condition. Two runners, opposite failure polarity, in one repo. The fail-open
// one is the dangerous half: once an interop scenario may carry an
// `RFC requirement:` tag (this spec's Phase 2), a green-but-unrun suite would
// satisfy `make ze-rfc-check` with evidence nothing executed
// (ai/rules/evidence.md).
//
// Go rather than Python because there is no Python test root under test/interop
// (scripts/dev/python_tests_test.go pins the three that exist), and this package
// follows test/hub's test-only-package precedent.

package interop_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInteropRunnerFailsClosedWithoutDocker
//
// VALIDATES: run.py exits non-zero and names Docker when the lab is unreachable.
// PREVENTS: the runner reporting success over an absence -- the same class of
// fail-open as a scanner silently skipping a file it does not recognize.
func TestInteropRunnerFailsClosedWithoutDocker(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	runner := filepath.Join(root, "test", "interop", "run.py")
	if _, err := os.Stat(runner); err != nil {
		t.Fatalf("stat %s: %v", runner, err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skipf("python3 not on PATH: %v", err)
	}

	// An empty directory as the entire PATH makes `docker` unreachable whether or
	// not this host has it installed, so the test is deterministic on a developer
	// laptop with Docker running and on a runner without it. python3 is invoked by
	// absolute path for the same reason.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	empty := t.TempDir()
	cmd := exec.CommandContext(ctx, python, runner)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+empty)

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("test/interop/run.py exited 0 with no Docker; it must fail closed.\noutput:\n%s", out)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run.py did not run: %v\noutput:\n%s", err, out)
	}
	text := strings.ToLower(string(out))
	if !strings.Contains(text, "docker") {
		t.Errorf("run.py's failure must name Docker as the missing prerequisite; got:\n%s", out)
	}
	if strings.Contains(text, "skipping") {
		t.Errorf("run.py must not describe a missing lab as skipping; got:\n%s", out)
	}
}
