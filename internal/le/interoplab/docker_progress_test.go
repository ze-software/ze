// Design: docs/architecture/testing/interop.md -- streamed builds retain immutable identity.
package interoplab

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

const progressImageID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// TestDockerBuildStreamsBeforeReturning proves real bytes reach the caller while
// the build is running, and a concurrent retag cannot change its selected image.
func TestDockerBuildStreamsBeforeReturning(t *testing.T) {
	var output bytes.Buffer
	images := make(map[string]string)
	var owned string
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		args := command.Arguments
		switch args[1] {
		case "build":
			if command.Timeout != dockerBuildTimeoutDefault {
				t.Fatalf("build budget = %v", command.Timeout)
			}
			for index, arg := range args {
				if arg == "-q" {
					t.Fatal("streamed build suppresses Docker's progress")
				}
				if arg == "-t" {
					images[args[index+1]] = progressImageID
					if args[index+1] != "shared:latest" {
						owned = args[index+1]
					}
				}
			}
			writeBuildProgress(t, command.Stdout, "compiling source\n")
			writeBuildProgress(t, command.Stderr, "exporting layers\n")
			if output.String() != "compiling source\nexporting layers\n" {
				t.Fatalf("progress is not visible before build exit: %q", output.String())
			}
			images["shared:latest"] = "sha256:" + strings.Repeat("b", 64)
			return ProcessResult{}, nil // Attached streams are not captured.
		case "image":
			name := args[len(args)-1]
			if args[2] == "inspect" {
				return ProcessResult{Stdout: images[name] + "\n"}, nil
			}
			delete(images, name)
			return ProcessResult{}, nil
		default:
			t.Fatalf("unexpected command: %v", args)
			return ProcessResult{}, nil
		}
	}}
	docker := newDocker(runner)
	image, err := docker.Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: &output})
	if err != nil {
		t.Fatal(err)
	}
	if !image.Built || image.Reference != progressImageID || owned == "" {
		t.Fatalf("built image = %+v; owned tag = %q", image, owned)
	}
	if err := docker.ReleaseImage(t.Context(), image); err != nil {
		t.Fatal(err)
	}
	if _, present := images[owned]; present {
		t.Fatal("released owned tag survives")
	}
	if images["shared:latest"] != "sha256:"+strings.Repeat("b", 64) {
		t.Fatal("release changed the shared cache tag")
	}
}

// TestDockerBuildProgressFailureKeepsBoundedDiagnostics proves a long streamed
// failure keeps its final stderr and exit status without retaining the build log.
func TestDockerBuildProgressFailureKeepsBoundedDiagnostics(t *testing.T) {
	var output bytes.Buffer
	removed := false
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if command.Arguments[1] != "build" {
			removed = true
			return ProcessResult{}, nil
		}
		writeBuildProgress(t, command.Stdout, "building\n")
		writeBuildProgress(t, command.Stderr, strings.Repeat("x", 65536)+"\ncompiler refused source\n")
		return ProcessResult{ExitCode: 7}, nil
	}}
	image, err := newDocker(runner).Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: &output})
	var failure *commandError
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want commandError", err)
	}
	if failure.ExitCode != 7 || !strings.HasSuffix(failure.Stderr, "compiler refused source") {
		t.Fatalf("failure lost the exit or diagnostic: %+v", failure)
	}
	if len(failure.Stderr) > outputExcerptBytesMax || output.Len() <= 65536 {
		t.Fatalf("capture/stream lengths = %d/%d", len(failure.Stderr), output.Len())
	}
	if image.Built || image.Reference != "" || !removed {
		t.Fatalf("failure retained a successful image or missed cleanup: %+v, removed=%v", image, removed)
	}
}

// TestDockerBuildProgressRefusesUnreadableIdentity proves inspect failure or a
// non-ID answer cannot become success, and the invocation's tag is still released.
func TestDockerBuildProgressRefusesUnreadableIdentity(t *testing.T) {
	for _, answer := range []ProcessResult{
		{},
		{Stdout: "shared:latest"},
		{Stdout: progressImageID + "\nextra"},
		{Stdout: "sha256:short"},
		{Stdout: "sha256:" + strings.Repeat("g", 64)},
		{ExitCode: 1, Stderr: "daemon unavailable"},
	} {
		t.Run(answer.Stdout+answer.Stderr, func(t *testing.T) {
			removed := false
			runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
				args := command.Arguments
				if args[1] == "build" {
					return ProcessResult{}, nil
				}
				if args[2] == "inspect" {
					return answer, nil
				}
				removed = true
				return ProcessResult{}, nil
			}}
			image, err := newDocker(runner).Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: io.Discard})
			if err == nil || image.Built || image.Reference != "" || !removed {
				t.Fatalf("unreadable identity: image=%+v error=%v removed=%v", image, err, removed)
			}
		})
	}
}

// TestDockerBuildProgressCancellationPreservesCause proves cancellation reaches
// the process boundary and cleanup gets an independent bounded context.
func TestDockerBuildProgressCancellationPreservesCause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runner := &cancelBuildRunner{t: t, cancel: cancel}
	image, err := newDocker(runner).Build(ctx, ImageBuild{Tag: "shared:latest", Progress: io.Discard})
	if !errors.Is(err, context.Canceled) || image.Built || !runner.removed {
		t.Fatalf("canceled build: image=%+v error=%v removed=%v", image, err, runner.removed)
	}
	if !strings.Contains(err.Error(), "last build message") {
		t.Fatalf("cancel lost stderr: %v", err)
	}
}

type cancelBuildRunner struct {
	t       *testing.T
	cancel  context.CancelFunc
	removed bool
}

func (r *cancelBuildRunner) Run(ctx context.Context, command ProcessCommand) (ProcessResult, error) {
	if command.Arguments[1] == "build" {
		writeBuildProgress(r.t, command.Stderr, "last build message\n")
		r.cancel()
		return ProcessResult{}, ctx.Err()
	}
	if ctx.Err() != nil || command.Timeout <= 0 || command.Timeout > time.Minute {
		r.t.Fatalf("cleanup inherited cancellation or lost its bound: %v, %v", ctx.Err(), command.Timeout)
	}
	r.removed = true
	return ProcessResult{}, nil
}

func writeBuildProgress(t *testing.T, output io.Writer, message string) {
	t.Helper()
	if output == nil {
		t.Fatal("build has no attached progress writer")
	}
	if _, err := io.WriteString(output, message); err != nil {
		t.Fatal(err)
	}
}

// TestDockerBuildProgressProcessCancellation cancels a real child only after
// its stdout reaches the caller, proving streaming neither waits for exit nor
// disconnects the process runner's cancellation path.
func TestDockerBuildProgressProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	progress := &cancelBuildProgress{cancel: cancel}
	runner := &buildProcessFixture{t: t}
	image, err := newDocker(runner).Build(ctx, ImageBuild{Tag: "shared:latest", Progress: progress})
	if !errors.Is(err, context.Canceled) || image.Built || !runner.removed {
		t.Fatalf("child cancellation: image=%+v error=%v removed=%v", image, err, runner.removed)
	}
	if !strings.Contains(progress.output.String(), "started") {
		t.Fatalf("no live child output reached the caller: %q", progress.output.String())
	}
}

// TestDockerBuildProgressPreservesWriterFailure keeps an output failure even
// when the real child also fails, which makes os/exec prefer its exit error.
func TestDockerBuildProgressPreservesWriterFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{name: "write error", err: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "short write", want: io.ErrShortWrite},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &buildProcessFixture{t: t, script: "printf 'build output\\n'; exit 7"}
			image, err := newDocker(runner).Build(t.Context(), ImageBuild{
				Tag: "shared:latest", Progress: &failedBuildProgress{err: test.err},
			})
			var failure *commandError
			if !errors.As(err, &failure) || failure.ExitCode != 7 {
				t.Fatalf("lost child exit: %v", err)
			}
			if !errors.Is(err, test.want) {
				t.Errorf("lost output failure %v: %v", test.want, err)
			}
			if image.Built || image.Reference != "" || !runner.removed {
				t.Fatalf("failed output retained an image: %+v, removed=%v", image, runner.removed)
			}
		})
	}
}

type failedBuildProgress struct {
	err error
}

func (w *failedBuildProgress) Write([]byte) (int, error) {
	return 0, w.err
}

type cancelBuildProgress struct {
	output bytes.Buffer
	cancel context.CancelFunc
}

func (p *cancelBuildProgress) Write(data []byte) (int, error) {
	count, err := p.output.Write(data)
	p.cancel()
	return count, err
}

type buildProcessFixture struct {
	t       *testing.T
	removed bool
	script  string
}

func (r *buildProcessFixture) Run(ctx context.Context, command ProcessCommand) (ProcessResult, error) {
	if command.Arguments[1] == "build" {
		script := r.script
		if script == "" {
			script = "printf 'started\\n'; exec sleep 30"
		}
		command.Arguments = []string{"sh", "-c", script}
		command.Timeout = 5 * time.Second
		return systemProcessRunner{}.Run(ctx, command)
	}
	if ctx.Err() != nil {
		r.t.Fatalf("cleanup context is canceled: %v", ctx.Err())
	}
	r.removed = true
	return ProcessResult{}, nil
}

// TestDockerBuildProgressPreservesCleanupFailure keeps both the original build
// failure and an owned-tag release refusal instead of hiding either cause.
func TestDockerBuildProgressPreservesCleanupFailure(t *testing.T) {
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if command.Arguments[1] == "build" {
			writeBuildProgress(t, command.Stderr, "compiler failure")
			return ProcessResult{ExitCode: 7}, nil
		}
		return ProcessResult{ExitCode: 1, Stderr: "daemon refused cleanup"}, nil
	}}
	_, err := newDocker(runner).Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: io.Discard})
	if err == nil {
		t.Fatal("failed build and cleanup answered success")
	}
	for _, diagnostic := range []string{"compiler failure", "daemon refused cleanup", "release failed build's owned tag"} {
		if !strings.Contains(err.Error(), diagnostic) {
			t.Fatalf("failure lost %q: %v", diagnostic, err)
		}
	}
}

// TestDockerBuildProgressKeepsEndpointRefusal proves opting into streaming does
// not start a command or attempt cleanup on an endpoint the wrapper refused.
func TestDockerBuildProgressKeepsEndpointRefusal(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	refusal := errors.New("no lab Docker endpoint")
	docker.endpointRefusal = refusal
	_, err := docker.Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: io.Discard})
	if !errors.Is(err, refusal) || len(runner.commands) != 0 {
		t.Fatalf("endpoint refusal: error=%v commands=%v", err, runner.commands)
	}
}

// TestDockerBuildProgressKeepsStdoutOnlyFailure proves legacy builders that put
// their diagnostic on stdout still leave an actionable, bounded command error.
func TestDockerBuildProgressKeepsStdoutOnlyFailure(t *testing.T) {
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if command.Arguments[1] != "build" {
			return ProcessResult{}, nil
		}
		writeBuildProgress(t, command.Stdout, strings.Repeat("x", 65536)+"\nlegacy build failure")
		return ProcessResult{ExitCode: 2}, nil
	}}
	_, err := newDocker(runner).Build(t.Context(), ImageBuild{Tag: "shared:latest", Progress: io.Discard})
	var failure *commandError
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want commandError", err)
	}
	if failure.Stderr != "" || !strings.HasSuffix(failure.Stdout, "legacy build failure") {
		t.Fatalf("stdout diagnostic was lost: %+v", failure)
	}
	if len(failure.Stdout) > outputExcerptBytesMax {
		t.Fatalf("stdout diagnostic is unbounded: %d bytes", len(failure.Stdout))
	}
}
