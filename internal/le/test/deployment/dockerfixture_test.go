package testdeployment

import (
	"context"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// argvRecorder is the deployment tests' Docker process boundary: it records
// every argv the wrapper renders and answers it with answer, or with success.
type argvRecorder struct {
	commands [][]string
	// timeouts holds each command's bound, index for index with commands.
	timeouts []time.Duration
	answer   func([]string) (interoplab.ProcessResult, error)
}

func (r *argvRecorder) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.commands = append(r.commands, command.Arguments)
	r.timeouts = append(r.timeouts, command.Timeout)
	if err := ctx.Err(); err != nil {
		return interoplab.ProcessResult{ExitCode: 124}, err
	}
	if r.answer == nil {
		return interoplab.ProcessResult{}, nil
	}
	return r.answer(command.Arguments)
}

// detachedArgv answers the argv the wrapper renders for spec.
func detachedArgv(t *testing.T, spec interoplab.DetachedContainer) []string {
	t.Helper()
	recorder := &argvRecorder{}
	if err := interoplab.NewFixtureDocker(recorder).RunDetached(t.Context(), spec); err != nil {
		t.Fatalf("render %s: %v", spec.Name, err)
	}
	return recorder.commands[0]
}

// execArgv answers the argv the wrapper renders for spec.
func execArgv(t *testing.T, spec interoplab.ContainerExec) []string {
	t.Helper()
	recorder := &argvRecorder{}
	if spec.Timeout == 0 {
		spec.Timeout = watchedExecTimeout
	}
	if _, err := interoplab.NewFixtureDocker(recorder).ExecAttached(t.Context(), spec); err != nil {
		t.Fatalf("render exec in %s: %v", spec.Container, err)
	}
	return recorder.commands[0]
}

// VALIDATES: a proof whose peer image is absent pulls it under
// deploymentPullTimeout, the 15 minutes a multi-gigabyte VPP image needs.
// PREVENTS: the pull falling back to the wrapper's shorter default (D-3).
func TestEnsureImagePullsUnderTheDeploymentBound(t *testing.T) {
	const image = "ligato/vpp-base:24.02"
	recorder := &argvRecorder{answer: func(argv []string) (interoplab.ProcessResult, error) {
		if len(argv) > 2 && argv[1] == "image" {
			return interoplab.ProcessResult{ExitCode: 1, Stderr: "Error: No such image: " + image}, nil
		}
		return interoplab.ProcessResult{}, nil
	}}
	if err := ensureImage(t.Context(), interoplab.NewFixtureDocker(recorder), image, nil); err != nil {
		t.Fatalf("ensureImage: %v", err)
	}
	last := len(recorder.commands) - 1
	if recorder.commands[last][1] != "pull" {
		t.Fatalf("last command = %v, want the pull", recorder.commands[last])
	}
	if got := recorder.timeouts[last]; got != 15*time.Minute {
		t.Errorf("pull bound = %s, want 15m", got)
	}
}
