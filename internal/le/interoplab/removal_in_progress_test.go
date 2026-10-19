package interoplab

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// autoRemovingDaemon models D-f (2026-10-20): a one-shot started with --rm
// hits its bound, so the daemon is already removing the container when the
// teardown's `docker rm -f` arrives, and refuses it with "removal of container
// <name> is already in progress". The inspect then finds the container for
// presentFor more looks, and absent after; a negative presentFor keeps it
// present forever. inspectFails makes every inspect a daemon failure. Not safe
// for concurrent use.
type autoRemovingDaemon struct {
	presentFor   int
	inspectFails bool
	inspects     int
}

func (a *autoRemovingDaemon) run(command ProcessCommand) (ProcessResult, error) {
	argv := command.Arguments
	name := argv[len(argv)-1]
	switch {
	case argv[1] == "run":
		return ProcessResult{ExitCode: timeoutExitCode}, context.DeadlineExceeded
	case argv[1] == "rm":
		return ProcessResult{ExitCode: 1, Stderr: "Error response from daemon: removal of container " + name + " is already in progress"}, nil
	case argv[1] == dockerContainerObject && argv[2] == dockerInspectVerb:
		a.inspects++
		if a.inspectFails {
			return ProcessResult{ExitCode: 1, Stderr: "Cannot connect to the Docker daemon"}, nil
		}
		if a.presentFor < 0 || a.inspects <= a.presentFor {
			return ProcessResult{Stdout: "abc\n"}, nil
		}
		return ProcessResult{ExitCode: 1, Stderr: "Error: No such object: " + name}, nil
	}
	return ProcessResult{}, nil
}

// VALIDATES: D-f. A one-shot whose bound ended, and whose --rm removal the
// daemon already runs, is reported removed once the container is gone, even
// when the first looks after the refused `rm -f` still find it.
// PREVENTS: a false "docker rm -f failed ... is already in progress" report
// for a container the daemon destroyed a moment later.
// METHOD: autoRemovingDaemon refuses the rm -f as in progress and keeps the
// container present for three inspects; the test reads the one-shot's error.
func TestOneShotRemovalAlreadyInProgressIsNotAFailure(t *testing.T) {
	daemon := &autoRemovingDaemon{presentFor: 3}
	docker := newDocker(&recordingRunner{run: daemon.run})
	_, err := docker.RunOneShot(t.Context(), OneShotContainer{Image: "alpine:3", Timeout: dockerCommandTimeout, Command: []string{"sleep", "60"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("one-shot error = %v, want the run's own bound", err)
	}
	if strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("a removal the daemon finished was reported failed: %v", err)
	}
	if daemon.inspects != 4 {
		t.Fatalf("looked for the container %d times, want 4 (present 3 times, then gone)", daemon.inspects)
	}
}

// VALIDATES: D-f. A container whose removal the daemon says is in progress,
// and that is still present when the settle window ends, is reported by name
// as still present, with the command that removes it.
// PREVENTS: an endless wait, or a bare docker refusal with no next step.
func TestRemovalAlreadyInProgressThatNeverEndsNamesTheCleanup(t *testing.T) {
	daemon := &autoRemovingDaemon{presentFor: -1}
	docker := newDocker(&recordingRunner{run: daemon.run})
	err := docker.RemoveContainer(t.Context(), "ze-oneshot-stuck")
	if err == nil {
		t.Fatal("a container still present after the window was reported removed")
	}
	for _, want := range []string{"ze-oneshot-stuck", "still present", cleanupCommand} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("removal error %q does not name %q", err, want)
		}
	}
	if daemon.inspects < 2 {
		t.Fatalf("looked for the container %d times, want a wait over the window", daemon.inspects)
	}
}

// VALIDATES: D-f. A removal in progress whose check fails is unknown, with
// both errors, the same answer confirmRemoval gives for any failed removal.
// PREVENTS: an unanswered inspect read as "gone" during the wait.
func TestRemovalAlreadyInProgressWithAFailedCheckIsUnknown(t *testing.T) {
	daemon := &autoRemovingDaemon{inspectFails: true}
	docker := newDocker(&recordingRunner{run: daemon.run})
	err := docker.RemoveContainer(t.Context(), "ze-oneshot-1")
	if _, unknown := errors.AsType[*removalUnknownError](err); !unknown {
		t.Fatalf("removal error = %v, want a removalUnknownError", err)
	}
}
