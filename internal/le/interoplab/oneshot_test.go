package interoplab

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: a one-shot the caller cancels, or its own bound ends, is removed
// by the name it was started under, and the error names both outcomes.
// PREVENTS: a stopped docker client leaving its container running the command
// to the end (AC-8: no labeled resource of a canceled run remains).
// Method: a fake runner answers the run with the context's error and records
// every argv; the test reads the removal that follows.
func TestRunOneShotRemovesACanceledContainerByName(t *testing.T) {
	cases := map[string]error{"canceled": context.Canceled, "timed out": context.DeadlineExceeded}
	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{}
			runner.run = func(command ProcessCommand) (ProcessResult, error) {
				if command.Arguments[1] == "run" {
					return ProcessResult{ExitCode: timeoutExitCode}, cause
				}
				return ProcessResult{}, nil
			}
			docker := newDocker(runner)
			_, err := docker.RunOneShot(t.Context(), OneShotContainer{Image: "alpine", Command: []string{"sleep", "60"}, Timeout: time.Second})
			if !errors.Is(err, cause) {
				t.Fatalf("RunOneShot error = %v, want it to wrap %v", err, cause)
			}
			if len(runner.commands) != 2 {
				t.Fatalf("recorded %d commands, want the run and its removal", len(runner.commands))
			}
			run := runner.commands[0].Arguments
			if !slices.Equal(run[:5], []string{DockerExecutable, "run", "--rm", "--name", fixtureOneShotName()}) {
				t.Errorf("run argv %v does not start with the named --rm run", run)
			}
			removal := runner.commands[1].Arguments
			if !slices.Equal(removal, []string{DockerExecutable, "rm", "-f", fixtureOneShotName()}) {
				t.Errorf("removal argv = %v, want rm -f of the one-shot's name", removal)
			}
		})
	}
}

// VALIDATES: a one-shot that ran to a non-zero exit is reported with its code
// and removes nothing, since --rm already did.
// PREVENTS: a failing gate read as success, or a needless removal call.
func TestRunOneShotReportsANonZeroExitWithoutRemoval(t *testing.T) {
	runner := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 3, Stderr: "refused\n"}, nil
	}}
	result, err := newDocker(runner).RunOneShot(t.Context(), OneShotContainer{Image: "alpine", Timeout: time.Second})
	if err == nil {
		t.Fatal("RunOneShot hid exit 3")
	}
	if result.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", result.ExitCode)
	}
	if OneShotDidNotExit(err) {
		t.Errorf("OneShotDidNotExit(%v) = true for a container that ran to an exit code", err)
	}
	if len(runner.commands) != 1 {
		t.Errorf("recorded %d commands, want only the run", len(runner.commands))
	}
}

// VALIDATES: the caller's streams reach the process runner unchanged.
// PREVENTS: an hour-long gate's output captured into memory instead of shown.
func TestRunOneShotPassesTheCallerStreams(t *testing.T) {
	runner := &recordingRunner{}
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("")
	_, err := newDocker(runner).RunOneShot(t.Context(), OneShotContainer{Image: "alpine", Timeout: time.Second, Stdin: stdin, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("RunOneShot: %v", err)
	}
	command := runner.commands[0]
	if command.Stdin != stdin {
		t.Error("Stdin did not reach the runner")
	}
	if command.Stdout != &stdout {
		t.Error("Stdout did not reach the runner")
	}
	if command.Stderr != &stderr {
		t.Error("Stderr did not reach the runner")
	}
}

// VALIDATES: a one-shot's network and bind mounts reach docker run before the
// image, each typed field as its own flag, and a mount with no target is
// refused before docker runs.
// PREVENTS: a client container that cannot reach a host listener, or reads
// no file the test wrote, because a typed field never reached the argv.
func TestRunOneShotRendersNetworkAndMounts(t *testing.T) {
	runner := &recordingRunner{}
	spec := OneShotContainer{Image: "alpine", Network: "host", Timeout: time.Second,
		Mounts:  []Mount{{Source: "/host/certs", Target: "/certs", ReadOnly: true}},
		Command: []string{"true"}}
	if _, err := newDocker(runner).RunOneShot(t.Context(), spec); err != nil {
		t.Fatalf("RunOneShot: %v", err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("recorded %d commands, want the run", len(runner.commands))
	}
	argv := runner.commands[0].Arguments
	image := slices.Index(argv, "alpine")
	network := slices.Index(argv, "--network")
	if network < 0 || network+1 >= image || argv[network+1] != "host" {
		t.Errorf("argv %v does not carry --network host before the image", argv)
	}
	mount := slices.Index(argv, "-v")
	if mount < 0 || mount+1 >= image || argv[mount+1] != "/host/certs:/certs:ro" {
		t.Errorf("argv %v does not carry -v /host/certs:/certs:ro before the image", argv)
	}

	refused := &recordingRunner{}
	spec.Mounts = []Mount{{Source: "/host/certs"}}
	if _, err := newDocker(refused).RunOneShot(t.Context(), spec); err == nil {
		t.Error("RunOneShot accepted a mount with no target")
	}
	if len(refused.commands) != 0 {
		t.Errorf("a refused one-shot still ran %d docker commands", len(refused.commands))
	}
}
