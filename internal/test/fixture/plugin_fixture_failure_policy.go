// Design: docs/architecture/api/process-protocol.md -- the Stage-1 failure-policy declaration
// Overview: register_failure_policy.go -- the three fixture names this file serves
// Related: fixture.go -- newObserver, the TLS connect-back every external fixture uses
// Related: ../../component/plugin/server/failure_policy.go -- the engine side these drive
//
// Three drivers for the three answers a plugin can give about its own failure.
// Each one is a REAL external process the daemon forks from a `run` line, so the
// exit these tests turn on is a process ending rather than a goroutine
// returning.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// failurePolicyStopGrace bounds the wait after a generation has said what it
// came to say. The process is about to exit and the daemon is about to act on
// that, so the wait exists only to let the stderr line reach the daemon's relay
// before the pipe closes.
const failurePolicyStopGrace = 250 * time.Millisecond

// failureRestartMarker counts the generations of the restart fixture, one byte
// for each. It is relative, so it lands in the working directory the daemon gave
// the plugin, which is the directory holding the configuration file, and it is a
// constant rather than a name built at run time so no caller can steer the read
// anywhere else.
const failureRestartMarker = "failure-restart.generation"

// failurePolicyGeneration answers how many times the restart fixture has been
// started.
//
// A file is what carries the count, because a restart is a NEW PROCESS: nothing
// in this process's memory survives to the generation the test waits for.
func failurePolicyGeneration() (int, error) {
	previous, err := os.ReadFile(failureRestartMarker)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("read %s: %w", failureRestartMarker, err)
	}
	generation := len(previous) + 1
	if err := os.WriteFile(failureRestartMarker, append(previous, 'x'), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", failureRestartMarker, err)
	}
	return generation, nil
}

// failurePolicyAct runs one scenario and reports its verdict. One goroutine for
// one act, ended by the send; the channel is buffered, so it never blocks.
func failurePolicyAct(ctx context.Context, plugin *sdk.Plugin, act ObserverScenario, verdict chan<- error) {
	verdict <- invokeScenario(ctx, plugin, act)
}

// failurePolicyServe runs the plugin transport and reports why it ended. One
// goroutine for one plugin lifecycle, ended by the send; the channel is
// buffered, so it never blocks.
func failurePolicyServe(ctx context.Context, plugin *sdk.Plugin, policy sdk.FailurePolicy, ended chan<- error) {
	ended <- plugin.Run(ctx, sdk.Registration{FailurePolicy: policy})
}

// failurePolicyRun connects as an external plugin, declares policy, completes
// the five stages, and calls act once every plugin is ready. It returns when act
// returns, which ENDS THE PROCESS: that exit is what the daemon reads.
func failurePolicyRun(ctx context.Context, name string, policy sdk.FailurePolicy, act ObserverScenario) error {
	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect %s: %w", name, err)
	}
	defer plugin.Close() //nolint:errcheck // the run result carries the useful transport error

	acted := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go failurePolicyAct(ctx, plugin, act, acted)
		return nil
	})

	ended := make(chan error, 1)
	go failurePolicyServe(ctx, plugin, policy, ended)

	select {
	case actErr := <-acted:
		ReportFailure(actErr)
		time.Sleep(failurePolicyStopGrace)
		return actErr
	case runErr := <-ended:
		// The daemon refused this plugin, which is the outcome the disagreement
		// driver is written to produce. The refusal itself is asserted on the
		// DAEMON's stderr, so this process reports nothing of its own.
		return runErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// failurePolicyRestartDriver declares restart and exits on its first generation.
// The daemon must start it again; the second generation says so and then asks
// the daemon to stop, which is what ends the test.
func failurePolicyRestartDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/failure-policy-restart takes no arguments")
	}
	return failurePolicyRun(ctx, "failure-restart", sdk.FailureRestart, func(_ context.Context, plugin *sdk.Plugin) error {
		generation, err := failurePolicyGeneration()
		if err != nil {
			return err
		}
		if generation == 1 {
			fmt.Fprintln(os.Stderr, "FIXTURE: failure-restart generation 1 is exiting")
			return nil
		}
		fmt.Fprintln(os.Stderr, "OK: the plugin process was started again after it exited")

		// A context of its own, because the caller's is about to end with this
		// process and the daemon owes an answer before that.
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := Dispatch(stopCtx, plugin, "request shutdown", nil); err != nil {
			return fmt.Errorf("ask the daemon to stop: %w", err)
		}
		return nil
	})
}

// failurePolicyFatalDriver declares fatal and exits. The daemon must stop.
func failurePolicyFatalDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/failure-policy-fatal takes no arguments")
	}
	return failurePolicyRun(ctx, "failure-fatal", sdk.FailureFatal,
		func(context.Context, *sdk.Plugin) error {
			fmt.Fprintln(os.Stderr, "FIXTURE: failure-fatal is exiting")
			return nil
		})
}

// failurePolicyDisagreeDriver declares ignore against a configuration block that
// asks for a respawn. Being REFUSED is the expected outcome here, so a transport
// that ends is a pass and reaching a ready state is the failure. The refusal
// itself is asserted on the daemon's stderr, which is where an operator reads
// it.
func failurePolicyDisagreeDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("plugin/failure-policy-disagree takes no arguments")
	}
	const name = "failure-disagree"

	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect %s: %w", name, err)
	}
	defer plugin.Close() //nolint:errcheck // the refusal is the daemon's to report, not this process's

	ready := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		ready <- errors.New("the daemon accepted a respawn its plugin declared it cannot meet")
		return nil
	})

	ended := make(chan error, 1)
	go failurePolicyServe(ctx, plugin, sdk.FailureIgnore, ended)

	select {
	case readyErr := <-ready:
		ReportFailure(readyErr)
		time.Sleep(failurePolicyStopGrace)
		return readyErr
	case <-ended:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
