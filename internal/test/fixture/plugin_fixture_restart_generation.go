// Design: docs/guide/command-reference.md -- request plugin restart, the command this fixture is restarted by
// Related: register_restart_generation.go -- the fixture names these drivers serve
// Related: plugin_fixture_failure_policy.go -- failurePolicyGeneration, the same marker-file count
// Related: daemon_await_exit_fixture.go -- daemonProcessState, the exit test restartGenerationGone reuses
//
// restart-generation is an external plugin that stays running until the daemon
// closes its connection, and answers one command with the generation it is.
// A restart replaces the PROCESS, so nothing in this process's memory survives
// to the next one: the count lives in a marker file in the working directory
// the daemon gave the plugin, which is the directory holding the configuration.
// A .ci tells the process that answers after a restart from the one that
// answered before it by that count, with no pid to compare across steps.
//
// With the argument ignore-first-stop, generation 1 is the plugin that ignores
// its stop: it records its pid, ignores SIGTERM and SIGINT, and goes on running
// after the daemon closes its connection, so only a kill ends it. Later
// generations behave, so the test still ends cleanly. The step driver
// plugin/restart-generation-gone then asks whether that process is gone.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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
// The two markers below are relative for the same reason.
const restartGenerationMarker = "restart-generation.generation"

// restartGenerationIgnoreStop is the argument that makes generation 1 ignore
// its stop.
const restartGenerationIgnoreStop = "ignore-first-stop"

// restartGenerationPIDMarker holds the pid of the generation that ignores its
// stop, written when it counts itself. restartGenerationHeldMarker is written
// once that generation's connection has closed and it is still running: its
// presence is what proves the stop was ignored, so a later "gone" can only be
// a kill and never the process exiting on its own.
const (
	restartGenerationPIDMarker  = "restart-generation.ignored-stop.pid"
	restartGenerationHeldMarker = "restart-generation.ignored-stop.held"
)

// restartGenerationHoldMax bounds how long the generation that ignores its
// stop runs on after its connection closed. It is the bound in a run where
// the daemon never kills it, the defect the test exists to catch: it is long
// enough to outlast restartGenerationGone's whole poll, so that defect reads
// as red, and the process still ends without the runner. Where the daemon
// works, its group kill ends the process about a second after the stop.
const restartGenerationHoldMax = 60 * time.Second

// restartGenerationGoneAttempts and restartGenerationGoneDelay bound the wait
// for the ignoring generation to be gone: 10 seconds, far under
// restartGenerationHoldMax, so a process that is still running at the end can
// only be one nothing killed.
const (
	restartGenerationGoneAttempts = 40
	restartGenerationGoneDelay    = 250 * time.Millisecond
)

// restartGenerationDriver connects as an external plugin, declares its one
// command, and serves until the daemon closes the connection. The SDK's event
// loop answers a closed connection with a nil error (sdk_dispatch.go), so a
// restart that stops this process is a clean exit, not a fixture failure,
// except for the generation ignore-first-stop tells to ignore it.
func restartGenerationDriver(ctx context.Context, args []string) error {
	ignoreFirstStop, err := restartGenerationMode(args)
	if err != nil {
		return err
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
	// ignoring is set by the handler goroutine and read after Run returns.
	var ignoring atomic.Bool
	plugin.OnExecuteCommand(func(_ string, command string, _ []string, _ string) (string, any, error) {
		if command != restartGenerationCommand {
			return statusError, nil, fmt.Errorf("restart-generation serves %q, not %q", restartGenerationCommand, command)
		}
		count, countErr := generation()
		if countErr != nil {
			return statusError, nil, countErr
		}
		if ignoreFirstStop && count == 1 && !ignoring.Load() {
			if armErr := restartGenerationArmIgnore(); armErr != nil {
				return statusError, nil, armErr
			}
			ignoring.Store(true)
		}
		var token textbuf.Buffer
		token.Str(restartGenerationToken).Int(int64(count))
		return statusDone, map[string]any{restartGenerationKey: token.String()}, nil
	})
	runErr := plugin.Run(ctx, &sdk.Registration{
		Commands: []sdk.CommandDecl{{Name: restartGenerationCommand}},
	})
	if !ignoring.Load() {
		return runErr
	}
	return errors.Join(runErr, restartGenerationHold())
}

// restartGenerationMode answers whether the arguments ask generation 1 to
// ignore its stop. Anything but no argument or that one word is refused.
func restartGenerationMode(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == restartGenerationIgnoreStop {
		return true, nil
	}
	return false, fmt.Errorf("plugin/restart-generation takes no argument or %q, not %q",
		restartGenerationIgnoreStop, strings.Join(args, " "))
}

// restartGenerationArmIgnore makes this process the one that ignores its
// stop: it records its pid for restartGenerationGone, and ignores the two
// signals that would end it through the fixture's NotifyContext (Run, in
// fixture.go), so that a stop could end it only by a kill. Ignore undoes that
// Notify for these signals.
func restartGenerationArmIgnore() error {
	var pid textbuf.Buffer
	pid.Int(int64(os.Getpid())).Str("\n")
	if err := os.WriteFile(restartGenerationPIDMarker, []byte(pid.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", restartGenerationPIDMarker, err)
	}
	signal.Ignore(syscall.SIGTERM, os.Interrupt)
	return nil
}

// restartGenerationHold is the ignored stop: the connection has closed and
// this process goes on running. It records that it outlived the close, then
// waits out restartGenerationHoldMax. Only a kill ends it before that, and
// running to the end of the hold is the defect, so it answers an error.
func restartGenerationHold() error {
	if err := os.WriteFile(restartGenerationHeldMarker, []byte("held\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", restartGenerationHeldMarker, err)
	}
	hold := time.NewTimer(restartGenerationHoldMax)
	defer hold.Stop()
	<-hold.C
	return fmt.Errorf("plugin/restart-generation ignored its stop and was still running %s after its connection closed: nothing killed it",
		restartGenerationHoldMax)
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

// restartGenerationGone is a .ci step, run in the test's directory, which is
// the plugin's working directory too. It answers once the generation that
// ignored its stop is gone, and refuses when that generation never outlived
// its closed connection (the test then proved no kill) or is still running
// after restartGenerationGoneAttempts. No process, or state Z, is gone: the
// process has exited and holds nothing, whoever reaps it
// (daemonProcessState).
func restartGenerationGone(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/restart-generation-gone takes no arguments")
	}
	if _, err := os.Stat(restartGenerationHeldMarker); err != nil {
		return fmt.Errorf("the generation told to ignore its stop never ran on past its closed connection, so no kill is proven: %w", err)
	}
	pid, err := readPID(restartGenerationPIDMarker)
	if err != nil {
		return fmt.Errorf("read the ignoring generation's pid: %w", err)
	}
	var state string
	var stateErr error
	if !Poll(ctx, restartGenerationGoneAttempts, restartGenerationGoneDelay, func() bool {
		state, stateErr = daemonProcessState(ctx, pid)
		if stateErr != nil {
			return false
		}
		if state == "" {
			return true
		}
		return strings.HasPrefix(state, "Z")
	}) {
		return fmt.Errorf("the generation that ignored its stop (pid %d) is still running: state %q: %w", pid, state, stateErr)
	}
	var report textbuf.Buffer
	report.Str("restart-generation: the generation that ignored its stop (pid ").Int(int64(pid)).
		Str(") ran on past its closed connection and is gone\n")
	_, err = os.Stdout.WriteString(report.String())
	return err
}
