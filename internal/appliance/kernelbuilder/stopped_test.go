package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// stopCase is one way a build's context ends: the operator's Ctrl-C, a
// deadline its caller set, or Ze's own limit (WithBuildLimit).
type stopCase struct {
	name    string
	context func() (context.Context, context.CancelFunc)
	want    error
	say     string
}

// stopCases answers the three ways a build stops, each about 300 ms in.
func stopCases() []stopCase {
	const after = 300 * time.Millisecond
	return []stopCase{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			stop := time.AfterFunc(after, cancel)
			return ctx, func() { stop.Stop(); cancel() }
		}, context.Canceled, "the kernel build was canceled"},
		{"caller deadline", func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), after)
		}, context.DeadlineExceeded, "the kernel build ran out of the time its caller gave it"},
		{"build limit", func() (context.Context, context.CancelFunc) {
			return WithBuildLimit(context.Background(), after)
		}, context.DeadlineExceeded, "the kernel build did not finish within the 300ms limit Ze gives one kernel build"},
	}
}

// checkStopped fails t unless err names the stop as tc says, at stage, and
// ends a deadline with next's last line, never with the step's own
// "signal: killed".
func checkStopped(t *testing.T, err error, tc stopCase, stage, next string) {
	t.Helper()
	if err == nil {
		t.Fatal("a stopped build was reported as a build")
	}
	if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), stage+": "+tc.say) {
		t.Errorf("a stopped build does not name the stop, want %q (%v):\n%v", stage+": "+tc.say, tc.want, err)
	}
	if strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("a stopped build is reported as a failed step:\n%v", err)
	}
	if !errors.Is(tc.want, context.DeadlineExceeded) {
		return
	}
	lines := strings.Split(err.Error(), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, next) {
		t.Errorf("a build out of time does not end with %q on its own line:\n%v", next, err)
	}
}

// TestDockerStopNamesTheStep proves a build stopped while Docker builds the
// builder image, or while the kernel container compiles, is named as that
// step stopping: canceled, out of its caller's time, or past Ze's limit, which
// is named. runCommand plays docker with a sleep under the build's context,
// the way exec.CommandContext kills the real Docker CLI.
//
// VALIDATES: round-8 ISSUE 1 (Docker).
// PREVENTS: a Ctrl-C or a deadline on the Docker backend reported as
// "kernel container failed: signal: killed", with no step and no next step.
func TestDockerStopNamesTheStep(t *testing.T) {
	steps := []struct {
		name  string
		call  int
		stage string
		next  string
		left  string
	}{
		{"builder image", 1, "Docker was stopped while it built the kernel builder image", "docker 'build' ", ""},
		{"kernel container", 2, "the kernel container was stopped while it built the kernel", "docker ps --filter ancestor=fixture", "docker ps --filter ancestor=fixture"},
	}
	for _, step := range steps {
		for _, tc := range stopCases() {
			t.Run(step.name+"/"+tc.name, func(t *testing.T) {
				req := Request{Root: t.TempDir(), Version: "7.1.1", Arch: "amd64", Profile: "qemu", Target: "installer", SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder", CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config"}, Image: "fixture", Stdout: os.Stderr, Stderr: os.Stderr}
				pinHostWithoutAContainerStore(t)
				old := runCommand
				t.Cleanup(func() { runCommand = old })
				calls := 0
				runCommand = func(ctx context.Context, _ Request, _ string, _ ...string) error {
					calls++
					if calls == step.call {
						return exec.CommandContext(ctx, "sleep", "60").Run()
					}
					return nil
				}
				ctx, cancel := tc.context()
				defer cancel()
				err := runDocker(ctx, req)
				checkStopped(t, err, tc, step.stage, step.next)
				// The container outlives the Docker CLI whatever stopped the
				// build, so the line that finds it ends every case, a Ctrl-C
				// included (round-9 ISSUE 2).
				if step.left == "" || err == nil {
					return
				}
				lines := strings.Split(err.Error(), "\n")
				if last := lines[len(lines)-1]; !strings.HasPrefix(last, step.left) {
					t.Errorf("a stopped kernel container does not end with %q, so a container still compiling goes unnamed:\n%v", step.left, err)
				}
			})
		}
	}
}

// TestWorkerBuildStopNamesTheStep proves a build stopped while Go compiles
// the QEMU kernel worker is named as that step stopping, and a deadline ends
// with the command to compile it by hand.
//
// VALIDATES: round-8 ISSUE 1 (QEMU worker).
// PREVENTS: a stop reported as "build QEMU kernel worker: signal: killed".
func TestWorkerBuildStopNamesTheStep(t *testing.T) {
	for _, tc := range stopCases() {
		t.Run(tc.name, func(t *testing.T) {
			old := runWorkerBuild
			t.Cleanup(func() { runWorkerBuild = old })
			runWorkerBuild = func(ctx context.Context, _ Request, _ []string, _ ...string) error {
				return exec.CommandContext(ctx, "sleep", "60").Run()
			}
			ctx, cancel := tc.context()
			defer cancel()
			_, err := buildGuestWorker(ctx, Request{Root: t.TempDir(), Arch: "amd64"})
			checkStopped(t, err, tc, "the Go compile of the QEMU kernel worker was stopped", "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go ")
		})
	}
}

// TestBuildLimitNamesHours proves the limit an operator reads is written in
// hours, "the 2-hour limit", as the CLI's kernelBuildTimeout is.
func TestBuildLimitNamesHours(t *testing.T) {
	if got, want := (buildLimit{limit: 120 * time.Minute}).Error(), "the 2-hour limit Ze gives one kernel build"; got != want {
		t.Errorf("buildLimit(120m) = %q, want %q", got, want)
	}
}
