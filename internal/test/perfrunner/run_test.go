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

// TestPerfRunnerMountsLinuxLe pins AC-26 of
// plan/spec-le-subject-first-command-tree.md at the runner's command seam: the
// cross-build of le is GOOS=linux, CGO_ENABLED=0 and the container's GOARCH,
// its output file is named le, the sender container mounts it at
// /usr/local/bin/le, the sender runs `le perf send`, and no command names a
// bin/ze-perf path.
func TestPerfRunnerMountsLinuxLe(t *testing.T) {
	root := t.TempDir()
	runner := New(root, testReporter, io.Discard, io.Discard)
	runner.LinuxTags = "ze_le ze_bgp"
	var calls []fakeCall
	runner.Run = func(_ context.Context, _, _ io.Writer, _ string, env, argv []string) error {
		calls = append(calls, fakeCall{argv: argv, env: env})
		return nil
	}

	if err := runner.buildLinuxBinary(); err != nil {
		t.Fatalf("buildLinuxBinary: %v", err)
	}
	if !runner.runPerf(DUTs()[0], false) {
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
	for _, call := range calls[1:] {
		joined := strings.Join(call.argv, " ")
		if strings.Contains(joined, runner.LinuxBinary+":"+containerLe+":ro") {
			mounted = true
		}
		if strings.Contains(joined, containerLe+" perf send --dut-addr") {
			sent = true
		}
	}
	if !mounted {
		t.Errorf("no docker run mounts %s at %s: %q", runner.LinuxBinary, containerLe, calls)
	}
	if !sent {
		t.Errorf("the sender never runs `le perf send`: %q", calls)
	}
	for _, call := range calls {
		for _, word := range call.argv {
			if strings.Contains(word, filepath.Join("bin", "ze-perf")) {
				t.Errorf("a command names a bin/ze-perf path: %q", call.argv)
			}
		}
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
	if err := runner.buildLinuxBinary(); err == nil {
		t.Fatal("buildLinuxBinary accepted empty LinuxTags")
	}
}

// TestExecuteRefusesARunWithNoStep keeps an empty step selection from
// answering success over no work.
func TestExecuteRefusesARunWithNoStep(t *testing.T) {
	runner := New(t.TempDir(), testReporter, io.Discard, io.Discard)
	if code := runner.Execute(Steps{}, nil); code != 2 {
		t.Fatalf("Execute(no step) = %d, want 2", code)
	}
}
