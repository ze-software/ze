// Design: docs/guide/command-reference.md -- request plugin restart, the command this fixture is restarted by
// Related: register_restart_generation.go -- the fixture name this driver serves
// Related: plugin_fixture_failure_policy.go -- failurePolicyGeneration, the same marker-file count
//
// restart-generation is an external plugin that stays running until the daemon
// closes its connection, and answers one command with the generation it is.
// A restart replaces the PROCESS, so nothing in this process's memory survives
// to the next one: the count lives in a marker file in the working directory
// the daemon gave the plugin, which is the directory holding the configuration.
// A .ci tells the process that answers after a restart from the one that
// answered before it by that count, with no pid to compare across steps.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// restartGenerationCommand is the plugin's one command. Its answer carries
// restartGenerationKey, whose value is restartGenerationToken followed by the
// generation number, one token with no space so a .ci needle can name it.
const (
	restartGenerationCommand = "show restart-generation status"
	restartGenerationKey     = "generation"
	restartGenerationToken   = "restart-generation-"
)

// restartGenerationMarker counts the generations that answered, one byte for
// each. It is relative, so it lands in the working directory the daemon gave
// the plugin, and a constant, so no caller can steer the write anywhere else.
const restartGenerationMarker = "restart-generation.generation"

// restartGenerationDriver connects as an external plugin, declares its one
// command, and serves until the daemon closes the connection. The SDK's event
// loop answers a closed connection with a nil error (sdk_dispatch.go), so a
// restart that stops this process is a clean exit, not a fixture failure.
func restartGenerationDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/restart-generation takes no arguments")
	}
	plugin, err := newObserver("restart-generation")
	if err != nil {
		return fmt.Errorf("connect restart-generation: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // the run result carries the useful transport error

	// The generation is counted once, at the first answer, rather than at
	// process start: only a process that completed the handshake and serves
	// the command counts, so a fork that never got that far (a declaration
	// probe, a refused connect-back) cannot shift the number.
	generation := sync.OnceValues(restartGenerationCount)
	plugin.OnExecuteCommand(func(_ string, command string, _ []string, _ string) (string, any, error) {
		if command != restartGenerationCommand {
			return statusError, nil, fmt.Errorf("restart-generation serves %q, not %q", restartGenerationCommand, command)
		}
		count, countErr := generation()
		if countErr != nil {
			return statusError, nil, countErr
		}
		var token textbuf.Buffer
		token.Str(restartGenerationToken).Int(int64(count))
		return statusDone, map[string]any{restartGenerationKey: token.String()}, nil
	})
	return plugin.Run(ctx, &sdk.Registration{
		Commands: []sdk.CommandDecl{{Name: restartGenerationCommand}},
	})
}

// restartGenerationCount appends one byte to the marker file and answers how
// many generations have counted, this one included.
func restartGenerationCount() (int, error) {
	previous, err := os.ReadFile(restartGenerationMarker)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", restartGenerationMarker, err)
	}
	if err := os.WriteFile(restartGenerationMarker, append(previous, 'x'), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", restartGenerationMarker, err)
	}
	return len(previous) + 1, nil
}
