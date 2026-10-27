// Design: docs/architecture/testing/runner-architecture.md — cross-process exclusive groups.

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const exclusiveProcessMarker = "exclusive-runner-subprocess"

type exclusiveProcessEvent struct {
	Entered  bool
	Finished bool
	Passed   bool
	Canceled bool
	Error    string
}

// TestParallelRunnerExclusiveAcrossProcesses holds a real runner callback in one
// child while separate runner processes try the same, different, and empty groups.
// Each child has a different TMPDIR and working directory. A contender's own
// context ends while the first child still holds its callback: it MUST return a
// visible cancellation without ever entering that callback. No socket supplies
// accidental exclusion, and no source-text assertion substitutes for execution.
func TestParallelRunnerExclusiveAcrossProcesses(t *testing.T) {
	if len(os.Args) >= 5 && os.Args[len(os.Args)-3] == exclusiveProcessMarker {
		runExclusiveProcessChild(t, os.Args[len(os.Args)-2], os.Args[len(os.Args)-1])
		return
	}

	group := t.TempDir()
	first := startExclusiveProcess(t, group, "hold")
	first.requireEntered(t)

	t.Run("same-group-cancellation", func(t *testing.T) {
		second := startExclusiveProcess(t, group, "contend")
		event := second.next(t)
		if event.Entered {
			t.Fatal("separate runners entered the same exclusive group while its first holder was still running")
		}
		if !event.Finished || event.Passed || !event.Canceled || event.Error == "" {
			t.Fatalf("waiting runner did not report cancellation: %+v", event)
		}
		second.stop(t)
	})

	for _, tc := range []struct {
		name  string
		group string
	}{
		{name: "different-group", group: group + "-other"},
		{name: "ungrouped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := startExclusiveProcess(t, tc.group, "hold")
			other.requireEntered(t)
			other.stop(t)
		})
	}

	// Canceling an executing callback MUST release its group for a new process.
	first.stop(t)
	next := startExclusiveProcess(t, group, "hold")
	next.requireEntered(t)
	next.stop(t)
}

func runExclusiveProcessChild(t *testing.T, group, mode string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if mode == "contend" {
		var deadlineCancel context.CancelFunc
		ctx, deadlineCancel = context.WithTimeout(ctx, 250*time.Millisecond)
		defer deadlineCancel()
	}
	// Closing stdin is the parent's release barrier. The parent MUST close it
	// and reap this process before removing its temporary directories.
	go func() {
		var release [1]byte
		_, _ = os.Stdin.Read(release[:])
		cancel()
	}()

	r := NewParallelRunner[string](NewColorsWithOverride(false))
	r.SetQuiet(true)
	r.setNoSummary(true)
	r.SetConcurrency(1)
	record := r.addTestWithoutNick("exclusive-process", "", func(ctx context.Context, _ string) (bool, error) {
		if err := json.NewEncoder(os.Stdout).Encode(exclusiveProcessEvent{Entered: true}); err != nil {
			return false, err
		}
		<-ctx.Done()
		return false, ctx.Err()
	})
	record.ExclusiveGroup = group
	r.display.SetOutput(io.Discard)
	passed := r.Run(ctx)
	event := exclusiveProcessEvent{
		Finished: true,
		Passed:   passed,
		Canceled: errors.Is(record.Error, context.Canceled) || errors.Is(record.Error, context.DeadlineExceeded),
	}
	if record.Error != nil {
		event.Error = record.Error.Error()
	}
	if err := json.NewEncoder(os.Stdout).Encode(event); err != nil {
		t.Fatal(err)
	}
}

type exclusiveProcess struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	events  *json.Decoder
}

// startExclusiveProcess MUST register cleanup that cancels and reaps its child.
// The child MUST hold its callback until that cancellation reaches its context.
func startExclusiveProcess(t *testing.T, group, mode string) *exclusiveProcess {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, executable, "-test.run=^TestParallelRunnerExclusiveAcrossProcesses$", "--", exclusiveProcessMarker, group, mode)
	command.Dir = t.TempDir()
	command.Env = append(os.Environ(), "TMPDIR="+filepath.Join(command.Dir, "tmp"))
	command.Stderr = os.Stderr
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	child := &exclusiveProcess{command: command, stdin: stdin, events: json.NewDecoder(stdout)}
	t.Cleanup(func() { child.stop(t) })
	return child
}

func (p *exclusiveProcess) next(t *testing.T) exclusiveProcessEvent {
	t.Helper()
	var event exclusiveProcessEvent
	if err := p.events.Decode(&event); err != nil {
		t.Fatalf("read runner subprocess event: %v", err)
	}
	return event
}

func (p *exclusiveProcess) requireEntered(t *testing.T) {
	t.Helper()
	if event := p.next(t); !event.Entered {
		t.Fatalf("runner did not enter independent group: %+v", event)
	}
}

func (p *exclusiveProcess) stop(t *testing.T) {
	t.Helper()
	if p.command == nil {
		return
	}
	if err := p.stdin.Close(); err != nil {
		t.Errorf("cancel runner subprocess: %v", err)
	}
	if err := p.command.Wait(); err != nil {
		t.Errorf("runner subprocess: %v", err)
	}
	p.command = nil
}
