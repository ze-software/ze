// Design: docs/architecture/api/process-protocol.md — the stop drain RPC
// Overview: fixture.go — fixture registry and observer helpers
// Related: ../plugins/fakedrain/fakedrain.go — the drain responder both modes share

package fixture

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/test/plugins/fakedrain"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/drain-responder", drainResponder)
	Register("plugin/drain-budget-over-ceiling", drainBudgetOverCeiling)
}

// drainResponder is fakedrain's responder over an external connection: it
// answers the stop drain exactly as the in-process plugin does, and fails the
// run when its connection closes with no drain answered.
func drainResponder(ctx context.Context, _ []string) error {
	plugin, err := newObserver("drain-responder")
	if err != nil {
		return err
	}
	defer plugin.Close() //nolint:errcheck // fixture teardown, so a close failure changes no assertion

	answered := fakedrain.Responder(plugin)
	runErr := plugin.Run(ctx, sdk.Registration{DrainBudget: fakedrain.Budget})
	if !answered() {
		fakedrain.Log("drain-fixture: FAIL connection closed before the drain was answered")
		return errors.Join(errors.New("drain fixture: stopped with no drain answered"), runErr)
	}
	fakedrain.Log("drain-fixture: connection closed after the drain")
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
