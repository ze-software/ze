// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: fixture.go — fixture registry and observer helpers

package fixture

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// drainFixtureHold is how long the drain responder holds its drain before it
// answers, so a .ci can see the daemon wait for the answer.
const drainFixtureHold = time.Second

// drainFixtureBudget is the responder's declared DrainBudget, above its hold.
const drainFixtureBudget = 5

func init() {
	Register("plugin/drain-responder", drainResponder)
	Register("plugin/drain-budget-over-ceiling", drainBudgetOverCeiling)
}

// drainResponder registers a drain handler and a budget, logs each drain it
// receives with its kind, holds it for drainFixtureHold, then answers. When
// its connection closes it checks that the drain came first: a stop that
// closed the connection with no drain answered fails the run, so the order
// "drain, then teardown" is asserted by the plugin that lives it.
func drainResponder(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-responder")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	var answered atomic.Bool
	plugin.OnDrain(func(drainCtx context.Context, input rpc.DrainInput) error {
		deadlineSeconds := int(time.Until(input.Deadline).Round(time.Second) / time.Second)
		drainFixtureLog("drain-fixture: drain received kind=" + input.Kind.String() +
			" deadline-in=" + strconv.Itoa(deadlineSeconds) + "s")
		select {
		case <-time.After(drainFixtureHold):
		case <-drainCtx.Done():
			drainFixtureLog("drain-fixture: FAIL connection closed while the drain was held")
			return drainCtx.Err()
		}
		answered.Store(true)
		drainFixtureLog("drain-fixture: drain answered")
		return nil
	})

	runErr := plugin.Run(ctx, sdk.Registration{DrainBudget: drainFixtureBudget})
	if !answered.Load() {
		drainFixtureLog("drain-fixture: FAIL connection closed before the drain was answered")
		return errors.Join(errors.New("drain fixture: stopped with no drain answered"), runErr)
	}
	drainFixtureLog("drain-fixture: connection closed after the drain")
	return runErr
}

// drainBudgetOverCeiling declares a DrainBudget one second above the daemon
// drain ceiling, which Stage 1 refuses.
func drainBudgetOverCeiling(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-over-ceiling")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	plugin.OnDrain(func(context.Context, rpc.DrainInput) error { return nil })
	return plugin.Run(ctx, sdk.Registration{DrainBudget: rpc.DrainCeilingSeconds + 1})
}

// drainFixtureLog writes one line to stderr, which the daemon relays for the
// .ci expectations while the plugin runs.
func drainFixtureLog(line string) {
	os.Stderr.WriteString(line + "\n") //nolint:errcheck // a lost line fails the .ci expectation that reads it
}
