// Design: docs/architecture/config/apply-ordering.md -- the coarse node and its rollback
// Detail: ../../component/config/transaction/executor.go -- rollbackApplied, which skips a coarse node on purpose
// Related: register_config_apply_ordering_coarse_root.go -- the observer that proves a coarse node is applied at all
//
// The three observers behind test/reload/config-apply-ordering-mixed-rollback.ci.
//
// One refuses its section apply, which is how the test makes a transaction fail
// after a decomposed operation has already been applied. The applied observer
// prints the line the test asserts on: a coarse node's participant that applied
// is rolled back through the transaction-wide `config-rollback`, never through
// the per-operation rollback its SDK has no handler for. The unapplied observer
// sorts after the failer, so its apply never runs, and it prints the line the
// test rejects if the rollback still reaches its handler.
package fixture

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("reload/config-apply-ordering-mixed-rollback-failer", uiDriver(runMixedRollbackFailer))
	Register("reload/config-apply-ordering-mixed-rollback-applied", uiDriver(runMixedRollbackApplied))
	Register("reload/config-apply-ordering-mixed-rollback-unapplied", uiDriver(runMixedRollbackUnapplied))
}

// mixedRollbackRoot is the root both observers declare. No component
// decomposes it, so each observer is represented by one coarse node.
const mixedRollbackRoot = "rsvp-te"

// runMixedRollbackFailer refuses the section apply, which fails the
// transaction after the bgp decomposed operations have already run.
//
// It sorts after the applied observer and before the unapplied one, and coarse
// nodes are synthesized in sorted participant order
// (participantsWithoutOperations). The applied observer has therefore applied
// when this one fails, and owes a rollback: the case R-4 names, reached through
// the section rollback. The unapplied observer never reaches its own apply, so
// it owes none.
func runMixedRollbackFailer(ctx context.Context) error {
	plugin, err := newObserver("config-apply-ordering-mixed-rollback-failer")
	if err != nil {
		return fmt.Errorf("connect mixed rollback failer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		var tb textbuf.Buffer
		tb.Str("OK: mixed rollback: the failing participant refused its section apply\n").StdErr() //nolint:errcheck // fixture progress output
		return errors.New("config-apply-ordering-mixed-rollback: refusing the section apply on purpose")
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{mixedRollbackRoot}})
}

// runMixedRollbackApplied prints the line the test asserts on when the
// transaction-wide rollback reaches it. It registers no apply handler: the SDK
// accepts the apply for it, which is what makes the rollback owed.
func runMixedRollbackApplied(ctx context.Context) error {
	plugin, err := newObserver("config-apply-ordering-mixed-rollback-applied")
	if err != nil {
		return fmt.Errorf("connect mixed rollback applied observer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigRollback(func(_ string) error {
		var tb textbuf.Buffer
		tb.Str("OK: mixed rollback: the coarse node received the section rollback\n").StdErr() //nolint:errcheck // fixture progress output
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{mixedRollbackRoot}})
}

// runMixedRollbackUnapplied prints the line the test rejects if the
// transaction-wide rollback reaches its handler. Its apply never ran, so the
// SDK owes it no rollback (pkg/plugin/sdk/config_tx_gate.go); a handler run
// here would replay an undo from an earlier, committed apply.
func runMixedRollbackUnapplied(ctx context.Context) error {
	plugin, err := newObserver("config-apply-ordering-mixed-rollback-unapplied")
	if err != nil {
		return fmt.Errorf("connect mixed rollback unapplied observer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigRollback(func(_ string) error {
		var tb textbuf.Buffer
		tb.Str("BUG: mixed rollback: a participant that never applied ran its rollback handler\n").StdErr() //nolint:errcheck // fixture progress output
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{mixedRollbackRoot}})
}
