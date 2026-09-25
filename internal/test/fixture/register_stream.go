// Design: docs/architecture/testing/ci-format.md -- compiled functional-test observers
// Related: fixture.go -- the registry and Run, which refuses the checkout root
//
// The two stream drivers write a scripted sequence of text commands to stdout
// at fixed pauses until they are stopped, for a test that spawns them as a
// command source. They were the le actions `test fixture dynamic` and
// `test fixture watchdog` until the harness fixture command became
// `le test fixture` (spec-le-subject-first-command-tree, D-8), and they
// write the same lines at the same pauses.

package fixture

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"
)

func init() {
	Register("dynamic", streamDriver(streamDynamic))
	Register("watchdog", streamDriver(streamWatchdog))
}

// dynamicFlow is the flow route the dynamic stream announces and withdraws.
const dynamicFlow = `flow route {\n match {\n source 10.0.0.1/32;\n destination 1.2.3.4/32;\n }\n then {\n discard;\n }\n }\n`

// stream writes one scripted sequence to out, pausing through pause, until
// pause reports that the run was stopped.
type stream func(context.Context, io.Writer, waiter) error

// waiter pauses for a duration and answers false when ctx ended first.
type waiter func(context.Context, time.Duration) bool

// streamDriver adapts a stream to a fixture driver that takes no arguments.
//
// SIGINT is ignored, deliberately: the parent protocol process then performs
// its normal SIGTERM shutdown sequence. signal.Ignore also removes the SIGINT
// half of the context Run built, so only SIGTERM ends the stream, and the
// driver then answers success.
func streamDriver(run stream) Driver {
	return func(ctx context.Context, args []string) error {
		if len(args) != 0 {
			return fmt.Errorf("a stream fixture takes no arguments, got %d", len(args))
		}
		signal.Ignore(os.Interrupt)
		return run(ctx, os.Stdout, wait)
	}
}

// wait is the waiter a running driver uses: a timer, or ctx ending.
func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// streamDynamic announces a flow route and a unicast route, then removes both,
// with ten seconds between lines. The loop ends only when pause reports a stop.
func streamDynamic(ctx context.Context, out io.Writer, pause waiter) error {
	for {
		if err := writeLine(out, "announce "+dynamicFlow); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "update text nhop set 10.0.0.1 nlri ipv4/unicast add 192.0.2.1/32"); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "update text nlri ipv4/unicast del 192.0.2.1/32"); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "withdraw "+dynamicFlow); err != nil {
			return err
		}
		if !pause(ctx, 10*time.Second) {
			return nil
		}
	}
}

// streamWatchdog withdraws and announces the watchdog groups in a fixed order.
// The loop ends only when pause reports a stop.
func streamWatchdog(ctx context.Context, out io.Writer, pause waiter) error {
	for {
		if !pause(ctx, 10*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog withdraw"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog withdraw watchdog-one"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce watchdog-one"); err != nil {
			return err
		}
		if !pause(ctx, 5*time.Second) {
			return nil
		}
		if err := writeLine(out, "bgp watchdog announce watchdog-two"); err != nil {
			return err
		}
		if err := writeLine(out, "bgp watchdog withdraw watchdog-two"); err != nil {
			return err
		}
	}
}

// writeLine writes one command line of a stream.
func writeLine(out io.Writer, line string) error {
	if _, err := fmt.Fprintln(out, line); err != nil {
		return fmt.Errorf("write stream fixture output: %w", err)
	}
	return nil
}
