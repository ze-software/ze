package perfrunner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// testReporter is the report argv the tests hand New: the le that `le perf run`
// passes, spelled as a developer types it.
var testReporter = []string{"le", "perf", "report"}

func TestGenerateToFileNeverDestroysTheExistingReport(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(destination, []byte("OLD\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failed := func(_ context.Context, stdout, _ io.Writer, _ string, _ []string, _ []string) error {
		_, _ = io.WriteString(stdout, "PARTIAL\n")
		return errors.New("generator failed")
	}
	var stderr strings.Builder
	if generateToFile(context.Background(), failed, []string{"le", "perf", "report"}, destination, &stderr) {
		t.Fatal("failing generator reported success")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "OLD\n" {
		t.Fatalf("destination = %q, want original bytes", content)
	}
	if _, err := os.Stat(destination + ".new"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file remains: %v", err)
	}
}

func TestGenerateToFilePublishesOnlyACompleteResult(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "report.html")
	succeeded := func(_ context.Context, stdout, _ io.Writer, _ string, _ []string, _ []string) error {
		_, _ = io.WriteString(stdout, "HTML\n")
		return nil
	}
	if !generateToFile(context.Background(), succeeded, []string{"le", "perf", "report"}, destination, io.Discard) {
		t.Fatal("successful generator reported failure")
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "HTML\n" {
		t.Fatalf("destination = %q", content)
	}
}

func TestUnmeasuredDUTsReportsPartialFleetRuns(t *testing.T) {
	if got, want := unmeasuredDUTs([]string{"/tmp/ze.json", "/tmp/ze-propagation.json", "/tmp/bird.json"}), []string{"frr", "gobgp", "rustbgpd", "rustybgp", "freertr", "openbgpd"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing DUTs = %q, want %q", got, want)
	}
	all := make([]string, 0, len(DUTs()))
	for _, dut := range DUTs() {
		all = append(all, dut.Name+".json")
	}
	if got := unmeasuredDUTs(all); len(got) != 0 {
		t.Fatalf("whole fleet reports missing DUTs: %q", got)
	}
}

func TestConfigOverlayWinsPerFileWithoutReplacingDefaults(t *testing.T) {
	root := t.TempDir()
	runner := New(root, testReporter, io.Discard, io.Discard)
	overlay := filepath.Join(root, "overlay")
	if err := os.MkdirAll(overlay, 0o750); err != nil {
		t.Fatal(err)
	}
	runner.ConfigOverlay = overlay
	if err := os.WriteFile(filepath.Join(overlay, "ze.conf"), []byte("filter"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := runner.config("ze.conf"); got != filepath.Join(overlay, "ze.conf") {
		t.Fatalf("overlay config = %s", got)
	}
	if got := runner.config("bird.conf"); got != filepath.Join(root, "test", "perf", "configs", "bird.conf") {
		t.Fatalf("fallback config = %s", got)
	}
}

// fakeCall is one command the runner asked for, with the environment it set.
type fakeCall struct {
	argv []string
	env  []string
}

// dockerRecorder is the Docker process seam: it records every docker argv and
// answers through answer, or success. A command under an ended context fails
// the way the real client does when its context is canceled.
type dockerRecorder struct {
	commands [][]string
	answer   func([]string) (interoplab.ProcessResult, error)
}

func (r *dockerRecorder) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.commands = append(r.commands, command.Arguments)
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{ExitCode: -1}, err
	}
	if r.answer == nil {
		return interoplab.ProcessResult{}, nil
	}
	return r.answer(command.Arguments)
}

// recorded answers the commands recorded so far, one joined line each.
func (r *dockerRecorder) recorded() []string {
	lines := make([]string, 0, len(r.commands))
	for _, argv := range r.commands {
		lines = append(lines, strings.Join(argv, " "))
	}
	return lines
}

// fixtureRunner answers a Runner over a temporary checkout whose host
// commands succeed and whose Docker commands go to the returned recorder.
func fixtureRunner(t *testing.T, stderr io.Writer) (*Runner, *dockerRecorder) {
	t.Helper()
	runner := New(t.TempDir(), testReporter, io.Discard, stderr)
	runner.LinuxTags = "ze_le ze_bgp"
	runner.Run = func(context.Context, io.Writer, io.Writer, string, []string, []string) error { return nil }
	recorder := &dockerRecorder{}
	runner.Docker = interoplab.NewFixtureDocker(recorder)
	return runner, recorder
}

// TestPerfRunnerMountsLinuxLe pins AC-26 of
// spec-le-subject-first-command-tree at the runner's command seams: the
// cross-build of le is GOOS=linux, CGO_ENABLED=0 and the container's GOARCH,
// its output file is named le, the sender container mounts it at
// /usr/local/bin/le, the sender runs `le perf send`, and no command names a
// bin/ze-perf path. The Docker argv is read where the wrapper renders it.
func TestPerfRunnerMountsLinuxLe(t *testing.T) {
	runner, recorder := fixtureRunner(t, io.Discard)
	var calls []fakeCall
	runner.Run = func(_ context.Context, _, _ io.Writer, _ string, env, argv []string) error {
		calls = append(calls, fakeCall{argv: argv, env: env})
		return nil
	}

	if err := runner.buildLinuxBinary(t.Context()); err != nil {
		t.Fatalf("buildLinuxBinary: %v", err)
	}
	if !runner.runPerf(t.Context(), DUTs()[0], false) {
		t.Fatal("runPerf reported a failure over a fake that always succeeds")
	}

	build := calls[0]
	wantBuild := []string{"go", "build", "-tags", "ze_le ze_bgp", "-o", runner.LinuxBinary, "./cmd/ze"}
	if !reflect.DeepEqual(build.argv, wantBuild) {
		t.Fatalf("build argv = %q, want %q", build.argv, wantBuild)
	}
	for _, want := range []string{"GOOS=linux", "GOARCH=" + runtime.GOARCH, "CGO_ENABLED=0"} {
		if !slices.Contains(build.env, want) {
			t.Errorf("the cross-build environment lacks %s", want)
		}
	}
	if filepath.Base(runner.LinuxBinary) != "le" {
		t.Errorf("the linux binary is %s; the container selects the personality from the name le", runner.LinuxBinary)
	}

	var mounted, sent bool
	for _, joined := range recorder.recorded() {
		if strings.Contains(joined, " run -d ") && strings.Contains(joined, "-v "+runner.LinuxBinary+":"+containerLe+":ro") {
			mounted = true
		}
		if strings.Contains(joined, containerLe+" perf send --dut-addr") {
			sent = true
		}
	}
	if !mounted {
		t.Errorf("no docker run mounts %s at %s: %q", runner.LinuxBinary, containerLe, recorder.recorded())
	}
	if !sent {
		t.Errorf("the sender never runs `le perf send`: %q", recorder.recorded())
	}
	for _, argv := range append(recorder.commands, build.argv) {
		for _, word := range argv {
			if strings.Contains(word, filepath.Join("bin", "ze-perf")) {
				t.Errorf("a command names a bin/ze-perf path: %q", argv)
			}
		}
	}
}

// TestPerfContainersCarryTheirFlagsAndLabels pins the DUT and sender argv
// through the wrapper: the network, address, capabilities, mounts,
// environment and command the runner's own docker argv carried, plus the
// owner labels that let cleanup find a container a dead run left.
func TestPerfContainersCarryTheirFlagsAndLabels(t *testing.T) {
	runner, recorder := fixtureRunner(t, io.Discard)
	runner.GCTrace, runner.PProf, runner.PProfPort = true, true, 6060
	if err := runner.startDUT(t.Context(), DUTs()[0]); err != nil {
		t.Fatalf("startDUT: %v", err)
	}
	if !runner.runPerf(t.Context(), DUTs()[0], false) {
		t.Fatal("runPerf failed over a fake that always succeeds")
	}
	lines := recorder.recorded()
	for _, want := range []string{
		"docker run -d --name " + runner.containerName("ze") + " --label ze.lab",
		"--network " + runner.network + " --ip 172.31.0.2 --cap-add NET_ADMIN",
		"-v " + runner.config("ze.conf") + ":/etc/ze/bgp.conf:ro",
		"-e GODEBUG=gctrace=1",
		"ze-interop --pprof 127.0.0.1:6060 start /etc/ze/bgp.conf",
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("DUT argv %q lacks %q", lines[0], want)
		}
	}
	for _, want := range []string{
		"docker run -d --name " + runner.senderName() + " --label ze.lab",
		"--network " + runner.network + " --ip 172.31.0.10 --cap-add NET_ADMIN",
		"-v " + runner.resultsDir + ":/results ",
		"alpine:3.21 sleep 3600",
	} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("sender argv %q lacks %q", lines[1], want)
		}
	}
	if want := "docker exec " + runner.senderName() + " ip addr add 172.31.0.11/24 dev eth0"; lines[2] != want {
		t.Errorf("receiver address exec = %q, want %q", lines[2], want)
	}
	if want := "docker rm -f " + runner.senderName(); lines[len(lines)-1] != want {
		t.Errorf("the sender is not removed after the measurement: last command %q", lines[len(lines)-1])
	}
}

// TestPerfRemovalFailureIsReportedAndFailsTheRun pins AC-12: a container or
// network removal that fails is named on stderr with the cleanup command and
// makes Execute answer 1, rather than being discarded.
func TestPerfRemovalFailureIsReportedAndFailsTheRun(t *testing.T) {
	var stderr strings.Builder
	runner, recorder := fixtureRunner(t, &stderr)
	recorder.answer = func(argv []string) (interoplab.ProcessResult, error) {
		if slices.Equal(argv[1:3], []string{"image", "inspect"}) {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: No such image: " + argv[len(argv)-1]}, nil
		}
		if slices.Equal(argv[1:3], []string{"network", "rm"}) {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: network has active endpoints"}, nil
		}
		return interoplab.ProcessResult{}, nil
	}
	if code := runner.Execute(t.Context(), Steps{Test: true}, []string{"bird"}); code != 1 {
		t.Fatalf("Execute = %d over a network that could not be removed, want 1", code)
	}
	for _, want := range []string{"error: remove network " + runner.network, "network has active endpoints", cleanupCommand} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr %q lacks %q", stderr.String(), want)
		}
	}

	stderr.Reset()
	recorder.answer = func(argv []string) (interoplab.ProcessResult, error) {
		if argv[1] == "rm" {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: device or resource busy"}, nil
		}
		return interoplab.ProcessResult{}, nil
	}
	runner.removalFailed = false
	runner.runPerf(t.Context(), DUTs()[1], false)
	if !runner.removalFailed {
		t.Error("a sender removal that failed did not fail the run")
	}
	if want := "error: remove container " + runner.senderName(); !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q lacks %q", stderr.String(), want)
	}
}

// TestPerfInterruptedRunRemovesWhatItStarted pins AC-8 at the runner: when the
// root context ends while a DUT is up, the DUT and the network are still
// removed, on a context the cancellation does not reach, and the run fails.
func TestPerfInterruptedRunRemovesWhatItStarted(t *testing.T) {
	var stderr strings.Builder
	runner, recorder := fixtureRunner(t, &stderr)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	recorder.answer = func(argv []string) (interoplab.ProcessResult, error) {
		if argv[1] == "run" && slices.Contains(argv, runner.containerName("ze")) {
			cancel()
		}
		return interoplab.ProcessResult{}, nil
	}
	if code := runner.Execute(ctx, Steps{Test: true}, []string{"ze"}); code != 1 {
		t.Fatalf("Execute = %d after the root context ended, want 1", code)
	}
	lines := recorder.recorded()
	for _, want := range []string{"docker rm -f " + runner.containerName("ze"), "docker network rm " + runner.network} {
		if !slices.Contains(lines, want) {
			t.Errorf("an interrupted run did not run %q: %q", want, lines)
		}
	}
	if strings.Contains(stderr.String(), "error: remove") {
		t.Errorf("a removal failed under the canceled root: %q", stderr.String())
	}
}

// TestPerfRunnerRefusesALinuxBuildWithoutTags keeps a caller that forgot the
// tags from building a featureless le that answers "unknown command".
func TestPerfRunnerRefusesALinuxBuildWithoutTags(t *testing.T) {
	runner := New(t.TempDir(), testReporter, io.Discard, io.Discard)
	runner.Run = func(context.Context, io.Writer, io.Writer, string, []string, []string) error {
		t.Fatal("the runner built with no tags")
		return nil
	}
	if err := runner.buildLinuxBinary(t.Context()); err == nil {
		t.Fatal("buildLinuxBinary accepted empty LinuxTags")
	}
}

// TestExecuteRefusesARunWithNoStep keeps an empty step selection from
// answering success over no work.
func TestExecuteRefusesARunWithNoStep(t *testing.T) {
	runner := New(t.TempDir(), testReporter, io.Discard, io.Discard)
	if code := runner.Execute(t.Context(), Steps{}, nil); code != 2 {
		t.Fatalf("Execute(no step) = %d, want 2", code)
	}
}
