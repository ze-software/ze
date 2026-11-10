package interoplab

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// startPanic is the first stderr line every le process wrote at start in
// commit c77a8dd2f3, which is what a probe of that build would have read.
const startPanic = `panic: BUG: le value "plugin" of "test qemu netns suites" has a ValueHelp line of 65 characters`

// newProbeRunner answers a staging preflight as newBuildRunner does, and
// answers every `docker run` (the start probe's one-shot) with answer.
func newProbeRunner(architecture string, answer func(ProcessCommand) (ProcessResult, error)) *buildRunner {
	runner := newBuildRunner(architecture)
	build := runner.run
	runner.run = func(command ProcessCommand) (ProcessResult, error) {
		if isOneShotRun(command) {
			return answer(command)
		}
		return build(command)
	}
	return runner
}

// isOneShotRun reports whether command is a `docker run`.
func isOneShotRun(command ProcessCommand) bool {
	return len(command.Arguments) > 1 && command.Arguments[0] == DockerExecutable && command.Arguments[1] == "run"
}

// probes answers every `docker run` the runner recorded.
func (r *buildRunner) probes() []ProcessCommand {
	var out []ProcessCommand
	for _, command := range r.commands {
		if isOneShotRun(command) {
			out = append(out, command)
		}
	}
	return out
}

// VALIDATES: AC-3. A staged le that panics at start is refused by the staging
// preflight, before any image build, and the refusal names the binary, its
// output path, the probe argv, the exit status, the first stderr lines and
// the fix. Method: the start probe's `docker run` is answered with exit 2
// and the panic c77a8dd2f3 caused; a binary declared after le is never built.
// PREVENTS: a le that cannot start passing preflight, so every helper
// container fails its readiness wait one scenario after another.
func TestStageBinariesRefusesABinaryThatCannotStart(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	runner := newProbeRunner("amd64", func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 2, Stderr: startPanic + "\n\ngoroutine 1 [running]:\nmain.main()\n"}, nil
	})
	declared := []LabBinary{
		{Name: "le", Base: "ze_le", Output: "test/interop/le-linux", StartProbe: []string{"help"}},
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: "test/interop/ze-linux"},
	}

	err = StageBinaries(root, false, declared...)(context.Background(), newDocker(runner))
	if err == nil {
		t.Fatal("StageBinaries accepted a le whose start probe exited 2 with a panic")
	}
	message := err.Error()
	for _, want := range []string{
		"lab binary le",
		"test/interop/le-linux",
		"`le help`",
		"exited 2",
		startPanic,
		"goroutine 1 [running]:",
		"Why:",
		"Fix: correct the start failure the stderr names",
		"./le test integration interop again",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("the refusal lacks %q:\n%s", want, message)
		}
	}

	builds := runner.builds()
	if len(builds) != 1 {
		t.Fatalf("the preflight ran %d builds, want 1: it MUST stop at the refused probe", len(builds))
	}
	probes := runner.probes()
	if len(probes) != 1 {
		t.Fatalf("the preflight ran %d probes, want 1", len(probes))
	}
	argv := probes[0].Arguments
	joined := strings.Join(argv, " ")
	if !slices.Contains(argv, StaticBinaryImage) {
		t.Errorf("the probe does not run in %s: %s", StaticBinaryImage, joined)
	}
	if want := outputOf(t, builds[0]) + ":/lab/le:ro"; !slices.Contains(argv, want) {
		t.Errorf("the probe does not mount the staged le read-only as %q: %s", want, joined)
	}
	if !strings.HasSuffix(joined, StaticBinaryImage+" /lab/le help") {
		t.Errorf("the probe does not run `/lab/le help` as its command: %s", joined)
	}
}

// VALIDATES: AC-3 survivor and AC-4. Only a binary that declares a start probe
// is run; a healthy probe passes and the preflight proceeds; the probe's
// elapsed time is on its progress line. Method: ze declares no probe, le
// declares `help`, and the one-shot answers exit 0.
// PREVENTS: a staged daemon started with no config (which cannot start) and
// read as a broken build, and a probe whose cost is invisible in the run.
func TestStageBinariesProbesOnlyDeclaredBinaries(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	runner := newProbeRunner("amd64", func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: "le help page\n"}, nil
	})
	declared := []LabBinary{
		{Name: "ze", Base: repofeaturetags.DaemonBase, Output: "test/interop/ze-linux"},
		{Name: "le", Base: "ze_le", Output: "test/interop/le-linux", StartProbe: []string{"help"}},
	}

	var progress bytes.Buffer
	if err := stageBinaries(context.Background(), newDocker(runner), root, "amd64", declared, &progress); err != nil {
		t.Fatalf("a healthy probe was refused: %v", err)
	}
	if builds := runner.builds(); len(builds) != len(declared) {
		t.Fatalf("the preflight ran %d builds, want %d", len(builds), len(declared))
	}
	probes := runner.probes()
	if len(probes) != 1 {
		t.Fatalf("the preflight ran %d probes, want 1 (le only)", len(probes))
	}
	if joined := strings.Join(probes[0].Arguments, " "); strings.Contains(joined, "ze-linux") {
		t.Errorf("the probe mounted ze, which declares no probe: %s", joined)
	}

	lines := strings.Split(strings.TrimSpace(progress.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("the probe wrote %d progress lines, want 1: %q", len(lines), progress.String())
	}
	line := lines[0]
	if !strings.HasPrefix(line, "interop: start probe le (`le help`): pass (") {
		t.Errorf("the progress line does not report le's pass: %q", line)
	}
	if !strings.HasSuffix(line, "s)") {
		t.Errorf("the progress line carries no elapsed time: %q", line)
	}
}

// VALIDATES: a probe Docker could not run to an exit refuses too, naming the
// binary and that its start is unproven, rather than passing.
// PREVENTS: a docker client failure read as a binary that started.
func TestStageBinariesRefusesAProbeThatDidNotRun(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the checkout root: %v", err)
	}
	runner := newProbeRunner("amd64", func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{}, errors.New("the docker client was stopped")
	})
	declared := []LabBinary{{Name: "le", Base: "ze_le", Output: "test/interop/le-linux", StartProbe: []string{"help"}}}

	var progress bytes.Buffer
	err = stageBinaries(context.Background(), newDocker(runner), root, "amd64", declared, &progress)
	if err == nil {
		t.Fatal("a probe the docker client never ran to an exit was accepted")
	}
	for _, want := range []string{"lab binary le", "did not run to an exit", "unproven", "./le test integration interop again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal lacks %q:\n%s", want, err)
		}
	}
	if !strings.Contains(progress.String(), "interop: start probe le (`le help`): FAIL (") {
		t.Errorf("the progress line does not report the failure: %q", progress.String())
	}
}

// VALIDATES: a probe's stderr is quoted up to startProbeStderrLinesMax lines,
// and the rest is counted rather than dropped in silence.
// PREVENTS: a full goroutine dump burying the panic line in the refusal.
func TestStartProbeStderrQuotesTheLeadingLines(t *testing.T) {
	lines := make([]string, startProbeStderrLinesMax+3)
	for index := range lines {
		lines[index] = "line"
	}
	lines[0] = startPanic
	quoted := startProbeStderr(CommandResult{Stderr: strings.Join(lines, "\n")})
	if !strings.HasPrefix(quoted, "  "+startPanic) {
		t.Errorf("the quote does not open with the panic line: %q", quoted)
	}
	if got := strings.Count(quoted, "\n") + 1; got != startProbeStderrLinesMax+1 {
		t.Errorf("the quote holds %d lines, want %d quoted and one count", got, startProbeStderrLinesMax+1)
	}
	if !strings.HasSuffix(quoted, "... 3 more line(s)") {
		t.Errorf("the quote does not count the dropped lines: %q", quoted)
	}
	if empty := startProbeStderr(CommandResult{}); !strings.Contains(empty, "printed nothing") {
		t.Errorf("an empty answer is not named: %q", empty)
	}
}
