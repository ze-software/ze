// Design: docs/architecture/api/commands.md — command-owned pipe filters, declared by a plugin
// Related: register_pipe_operation_declared.go — the name test/plugin/plugin-pipe-operation.ci forks

package fixture

import (
	"context"
	"errors"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// cmdShowFixturePipeOp is the one command the pipe-operation fixture declares,
// and pipeOpHistogram the operation it declares on it.
const (
	cmdShowFixturePipeOp = "show fixture pipe-op"
	pipeOpHistogram      = "histogram"
)

// pipeOperationProvider is a third-party plugin, run forked, that declares a
// pipe OPERATION at Stage 1: a Pipes entry stating an argument kind and no
// expansion, on its own command. Its handler answers what it received, so the
// .ci reads off the answer whether the operation crossed the process boundary
// in the execute-command pipe list rather than in the arguments:
//
//	{"pipe-operations": [<each operation's name, in order>], "args": [<arguments>]}
//
// It refuses, with an error and no answer, a call whose list is empty: the one
// line the .ci runs through the handler types the operation, so an empty list
// is the defect under test, and its red names the missing list.
func pipeOperationProvider(ctx context.Context, _ []string) error {
	plugin, err := newObserver("pipe-operation-declared")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	registration := sdk.Registration{
		Commands: []sdk.CommandDecl{{Name: cmdShowFixturePipeOp}},
		Pipes: []sdk.PipeDecl{{
			Command:     cmdShowFixturePipeOp,
			Name:        pipeOpHistogram,
			Argument:    "none",
			Description: "Count the rows of each length.",
		}},
	}
	plugin.OnExecuteCommand(func(_, command string, args []string, _ string, pipe []sdk.PipeOperation) (string, any, error) {
		if command != cmdShowFixturePipeOp {
			return statusError, map[string]any{fieldError: "unknown: " + command}, nil
		}
		if len(pipe) == 0 {
			return "", nil, errors.New("the execute-command pipe list arrived empty for a line that typed | " + pipeOpHistogram)
		}
		names := make([]string, 0, len(pipe))
		for _, operation := range pipe {
			names = append(names, operation.Name)
		}
		return statusDone, map[string]any{
			"pipe-operations": names,
			"args":            append([]string{}, args...),
		}, nil
	})
	return plugin.Run(ctx, &registration)
}
