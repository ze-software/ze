// Design: docs/architecture/config/apply-ordering.md -- the coarse node and its rollback
// Detail: ../../component/config/transaction/executor.go -- rollbackApplied, which skips a coarse node on purpose
// Related: register_config_apply_ordering_coarse_root.go -- the observer that proves a coarse node is applied at all
//
// The two observers behind test/reload/config-apply-ordering-mixed-rollback.ci.
//
// One refuses its section apply, which is how the test makes a transaction fail
// after a decomposed operation has already been applied. The other prints the
// line the test asserts on: a coarse node's participant is rolled back through
// the transaction-wide `config-rollback`, never through the per-operation
// rollback its SDK has no handler for.
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
	Register("reload/config-apply-ordering-mixed-rollback-observer", uiDriver(runMixedRollbackObserver))
}

// mixedRollbackRoot is the root both observers declare. No component
// decomposes it, so each observer is represented by one coarse node.
const mixedRollbackRoot = "rsvp-te"

// runMixedRollbackFailer refuses the section apply, which fails the
// transaction after the bgp decomposed operations have already run.
//
// It sorts before the observer, and coarse nodes are synthesized in sorted
// participant order (participantsWithoutOperations), so the observer never
// reaches its own apply. That is the case R-4 names: the participant still owes
// a rollback, and the rollback that reaches it is the section one.
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

// runMixedRollbackObserver prints the line the test asserts on when the
// transaction-wide rollback reaches it.
func runMixedRollbackObserver(ctx context.Context) error {
	plugin, err := newObserver("config-apply-ordering-mixed-rollback-observer")
	if err != nil {
		return fmt.Errorf("connect mixed rollback observer: %w", err)
	}
	defer plugin.Close() //nolint:errcheck // Run returns the useful transport error
	plugin.OnConfigRollback(func(_ string) error {
		var tb textbuf.Buffer
		tb.Str("OK: mixed rollback: the coarse node received the section rollback\n").StdErr() //nolint:errcheck // fixture progress output
		return nil
	})
	return plugin.Run(ctx, sdk.Registration{WantsConfig: []string{mixedRollbackRoot}})
}
