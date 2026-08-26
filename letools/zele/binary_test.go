// Other package tests exercise registered handlers.
// This test compiles ze with ze_le and runs the resulting binary.
// It covers the build, composition root, and argv handling between registration and a working command.
// The test compiles once and invokes that binary four times.

package zele

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/lepath"
)

// crossingTags is the build the crossing is meant for: a ze that dispatches
// root commands (ze_core) with le's tools linked (ze_le). No shipped build
// sets the second one.
const crossingTags = "ze_core ze_le"

// compileTimeout bounds one cold cmd/ze compile.
// A cold compile took 34s on this hardware on 2026-08-26.
// A run beyond this limit indicates a stuck toolchain.
const compileTimeout = 10 * time.Minute

// runTimeout bounds one invocation of the compiled binary. Every le command
// this case runs answers from the checkout, and the slowest measured 0.3 s.
const runTimeout = 2 * time.Minute

// compileCrossing compiles ze with the crossing tag and answers the binary's
// path.
func compileCrossing(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}

	binary := filepath.Join(t.TempDir(), "ze-le")
	ctx, cancel := context.WithTimeout(t.Context(), compileTimeout)
	defer cancel()

	compile := exec.CommandContext(ctx, "go", "build", "-tags", crossingTags, "-o", binary, "./cmd/ze")
	compile.Dir = root
	compile.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compiling ze with -tags %q: %v\n%s", crossingTags, compileErr, out)
	}
	return binary
}

// invoke runs the compiled binary and returns its output and exit code.
// A nonzero code is a valid answer in this helper.
// CombinedOutput returns an error for that code, but ProcessState distinguishes it from a process that never started.
func invoke(t *testing.T, binary string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	out, err := cmd.CombinedOutput()
	if cmd.ProcessState == nil {
		t.Fatalf("run %s %v: %v\n%s", binary, args, err, out)
	}
	return string(out), cmd.ProcessState.ExitCode()
}

// TestZeWithTheLeTagRunsLesCommands verifies the crossing from build to process output.
// It proves the shared-engine claim instead of restating it.
// A ze binary lists le commands, runs one, and renders its answer through shared pipe operators.
func TestZeWithTheLeTagRunsLesCommands(t *testing.T) {
	binary := compileCrossing(t)

	page, code := invoke(t, binary, "le", "--help")
	if code != 0 {
		t.Fatalf("`ze le --help` answered %d, want 0:\n%s", code, page)
	}
	if !strings.Contains(page, "ze le <command>") {
		t.Errorf("the page does not name the command a reader would type:\n%s", page)
	}
	// Two tools that have been le's since the port began, so this asserts the
	// command SET arrived rather than one lucky registration.
	for _, want := range []string{"working-tree", "parity"} {
		if !strings.Contains(page, want) {
			t.Errorf("`ze le --help` does not list %q:\n%s", want, page)
		}
	}

	// working-tree is the cheapest tool that answers from the checkout alone
	// and exits 0 whatever it finds there.
	report, code := invoke(t, binary, "le", "working-tree")
	if code != 0 {
		t.Fatalf("`ze le working-tree` answered %d, want 0:\n%s", code, report)
	}
	if !strings.Contains(report, "working tree:") {
		t.Errorf("`ze le working-tree` printed no report:\n%s", report)
	}

	rendered, code := invoke(t, binary, "le", "working-tree", "|", "json")
	if code != 0 {
		t.Fatalf("`ze le working-tree | json` answered %d, want 0:\n%s", code, rendered)
	}
	if !json.Valid([]byte(rendered)) {
		t.Errorf("`| json` rendered something that is not JSON:\n%s", rendered)
	}

	refusal, code := invoke(t, binary, "le", "no-such-tool")
	if code != 1 {
		t.Errorf("`ze le no-such-tool` answered %d, want 1:\n%s", code, refusal)
	}
}
