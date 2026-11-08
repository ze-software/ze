package interoplab

import (
	"context"
	"strings"
	"testing"
)

// VALIDATES: a docker exec that outlives its bound reports the container, the
// command it ran inside it, and the bound, for both the attached and the detached
// form, and with environment flags in front of the container name.
// PREVENTS: the nightly "docker exec ze-ipsec-...-ze failed (exit 124)" that named
// the container and not which of a checker's dozens of commands hung.
// Method: a recording runner answers every exec with the timeout result the system
// runner produces, and the test reads the error text.
func TestDockerExecTimeoutNamesTheCommand(t *testing.T) {
	runner := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: timeoutExitCode}, context.DeadlineExceeded
	}}
	docker := newDocker(runner)

	_, err := docker.Exec(t.Context(), "lab-ze", []string{"ze", "show", "vpn", "ipsec", "sa"}, []EnvironmentVariable{{Name: "TOKEN", Value: "x"}})
	requireExecTimeoutText(t, err, "docker exec lab-ze ze show vpn ipsec sa")

	err = docker.ExecDetached(t.Context(), "lab-ze", []string{"sh", "-c", "first line\nsecond line"}, nil)
	requireExecTimeoutText(t, err, "docker exec -d lab-ze sh -c first line ...")
	if strings.Contains(err.Error(), "second line") {
		t.Fatalf("error carries the script past its first line: %v", err)
	}
}

// VALIDATES: a long command is cut to a bounded prefix in the error text.
// PREVENTS: a whole shell script copied into one error line.
func TestDockerExecOperationIsBounded(t *testing.T) {
	long := strings.Repeat("a", 4*execOperationCommandMax)
	operation := execOperation("docker exec", "lab-ze", []string{"sh", "-c", long})
	if len(operation) > len("docker exec lab-ze ")+execOperationCommandMax+len(" ...") {
		t.Fatalf("operation is %d bytes, want at most the bound: %q", len(operation), operation)
	}
	if !strings.HasSuffix(operation, " ...") {
		t.Fatalf("operation %q does not mark the cut", operation)
	}
}

func requireExecTimeoutText(t *testing.T, err error, operation string) {
	t.Helper()
	if err == nil {
		t.Fatal("a timed-out exec returned no error")
	}
	text := err.Error()
	if !strings.HasPrefix(text, operation+" did not finish within 30s") {
		t.Fatalf("error = %q, want it to open with %q and the 30s bound", text, operation)
	}
}

// VALIDATES: ExecExitCode answers the exit of a command Exec ran to a non-zero
// exit, and refuses an error whose command never ran to one: the client hit
// its bound, or the call succeeded.
// PREVENTS: a checker whose verdict is an expected non-zero exit (an
// unanswered connect's 124) reading every non-zero exit as a lab failure, or
// a bound the docker client hit as that verdict.
// Method: a recording runner answers each exec, and the test reads the
// classification of the error Exec returned.
func TestExecExitCodeAnswersOnlyARanCommand(t *testing.T) {
	exited := newDocker(&recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: 124}, nil
	}})
	_, err := exited.Exec(t.Context(), "lab-web", []string{"timeout", "5", "true"}, nil)
	if code, ok := ExecExitCode(err); !ok || code != 124 {
		t.Errorf("ExecExitCode(%v) = %d, %t; want 124, true", err, code, ok)
	}
	bounded := newDocker(&recordingRunner{run: func(ProcessCommand) (ProcessResult, error) {
		return ProcessResult{ExitCode: timeoutExitCode}, context.DeadlineExceeded
	}})
	_, err = bounded.Exec(t.Context(), "lab-web", []string{"sleep", "60"}, nil)
	if code, ok := ExecExitCode(err); ok {
		t.Errorf("ExecExitCode(%v) = %d, true for a docker client its bound stopped", err, code)
	}
	if _, ok := ExecExitCode(nil); ok {
		t.Error("ExecExitCode(nil) answered an exit")
	}
}
