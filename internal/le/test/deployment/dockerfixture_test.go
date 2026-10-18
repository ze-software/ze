package testdeployment

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// argvRecorder is the deployment tests' Docker process boundary: it records
// every argv the wrapper renders and answers it with answer, or with success.
type argvRecorder struct {
	commands [][]string
	answer   func([]string) (interoplab.ProcessResult, error)
}

func (r *argvRecorder) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	r.commands = append(r.commands, command.Arguments)
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
