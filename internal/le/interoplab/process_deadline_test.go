package interoplab

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// VALIDATES: a command its caller's context deadline ended is named for that
// deadline, while a command its own bound ended keeps the bound's wording.
// Both still answer context.DeadlineExceeded and exit 124, so no caller's
// verdict changes; only the text does.
// METHOD: run `sleep 10` once under a caller deadline shorter than the
// command's 30s bound, and once under a bound shorter than the caller's
// deadline, and read the commandError text the Docker wrapper builds from
// each result.
// PREVENTS: "docker exec ze-iop-frr-<sfx> cat /var/run/frr/bgpd.pid did not
// finish within 30s (exit 124)" in no-family-peer-eor-frr, where the 60s
// Wait window closed while the probe's first exec ran, so the exec was killed
// after well under 30s and reported as a 30s hang.
// MUTATION: in systemProcessRunner.Run, return runCtx.Err() for every
// deadline (drop the context.Cause test). The caller-deadline case then reads
// "did not finish within 30s" and this test goes red; the bound case stays
// green, which shows the two cases are told apart, not renamed together.
func TestProcessRunnerNamesTheDeadlineThatEndedTheCommand(t *testing.T) {
	const operation = "docker exec lab-ze sleep 10"

	const bound = 30 * time.Second
	caller, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	result, err := systemProcessRunner{}.Run(caller, ProcessCommand{Arguments: []string{"sleep", "10"}, Timeout: bound})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller-deadline run error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	cut, ok := errors.AsType[*callerDeadlineError](err)
	if !ok {
		t.Fatalf("caller-deadline run error = %v (%T), want *callerDeadlineError", err, err)
	}
	if cut.Elapsed >= bound {
		t.Errorf("caller-deadline run elapsed = %s, want less than the %s bound", cut.Elapsed, bound)
	}
	if result.ExitCode != timeoutExitCode {
		t.Errorf("caller-deadline run exit code = %d, want %d", result.ExitCode, timeoutExitCode)
	}
	text := (&commandError{Operation: operation, ExitCode: result.ExitCode, Cause: err, Timeout: bound}).Error()
	if !strings.HasPrefix(text, operation+" was cut short by its caller's deadline after ") {
		t.Errorf("caller-deadline run text = %q, want it to name the caller's deadline", text)
	}
	if !strings.Contains(text, "before its own 30s bound") {
		t.Errorf("caller-deadline run text = %q, want it to name the bound it did not reach", text)
	}
	if strings.Contains(text, "did not finish within") {
		t.Errorf("caller-deadline run reads as a hang of its own bound: %q", text)
	}

	const short = 100 * time.Millisecond
	roomy, cancelRoomy := context.WithTimeout(t.Context(), bound)
	defer cancelRoomy()
	result, err = systemProcessRunner{}.Run(roomy, ProcessCommand{Arguments: []string{"sleep", "10"}, Timeout: short})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded run error = %v, want context.DeadlineExceeded", err)
	}
	if _, ok := errors.AsType[*callerDeadlineError](err); ok {
		t.Fatalf("bounded run error = %v, named the caller's deadline for the command's own bound", err)
	}
	if result.ExitCode != timeoutExitCode {
		t.Errorf("bounded run exit code = %d, want %d", result.ExitCode, timeoutExitCode)
	}
	text = (&commandError{Operation: operation, ExitCode: result.ExitCode, Cause: err, Timeout: short}).Error()
	if !strings.HasPrefix(text, operation+" did not finish within 100ms") {
		t.Errorf("bounded run text = %q, want it to name the bound", text)
	}
	if strings.Contains(text, "cut short") {
		t.Errorf("bounded run text names the caller's deadline: %q", text)
	}
}

// VALIDATES: a cross-compile its caller's deadline ended is not refused as a
// slow build: the refusal names the caller's deadline and leaves out the
// cold-cache reasons that only a compile reaching labBuildTimeout earns.
// METHOD: the build runner answers the compile with the callerDeadlineError
// systemProcessRunner returns for that case, and the test reads the refusal
// stageBinary builds.
// MUTATION: delete the callerDeadlineError branch in stageBinary. The error
// still wraps context.DeadlineExceeded, so the refusal reads "did not finish
// within 15m0s" with the cold-cache Why, and this test goes red.
func TestStageBinaryCutShortNamesTheCallersDeadline(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	runner := newBuildRunner("arm64")
	runner.run = func(command ProcessCommand) (ProcessResult, error) {
		if len(command.Arguments) > 1 && command.Arguments[1] == "version" {
			return ProcessResult{Stdout: "arm64\n"}, nil
		}
		return ProcessResult{ExitCode: timeoutExitCode}, &callerDeadlineError{Elapsed: 2 * time.Second}
	}
	binary := LabBinary{Name: "le", Base: "ze_le", Output: "test/interop/le-linux"}
	err = StageBinaries(root, false, binary)(context.Background(), newDocker(runner))
	if err == nil {
		t.Fatal("a cross-compile its caller's deadline ended was accepted")
	}
	message := err.Error()
	want := "cross-compile le into test/interop/le-linux was cut short by its caller's deadline after 2s, before its own " +
		labBuildTimeout.String() + " bound"
	if !strings.Contains(message, want) {
		t.Errorf("the refusal does not say %q:\n%s", want, message)
	}
	for _, banned := range []string{"did not finish within", "Why: a cold Go build cache"} {
		if strings.Contains(message, banned) {
			t.Errorf("the refusal reads as a slow build (%q):\n%s", banned, message)
		}
	}
}
