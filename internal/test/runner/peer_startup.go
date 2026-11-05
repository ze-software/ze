// Design: docs/architecture/testing/runner-architecture.md -- peer readiness within the case budget.
// Related: process_owner.go -- done closes after final output copying joins.

package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// awaitPeerListening waits for the post-bind token or the peer's actual exit.
// Callers MUST pass their existing bounded case context and retain the owner's
// deferred finish. Readiness consumes that context's remaining budget; a new
// startup deadline would reject a runnable case under host contention.
func awaitPeerListening(ctx context.Context, proc *ownedProcess, stdout *syncWriter, stderr *lockedBuilder) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			proc.cleanup()
			return fmt.Errorf("peer did not start listening before the test ended: %w "+
				"(stderr=%q, stdout=%q); check the peer startup output and the test's option=timeout budget",
				err, stderr.String(), stdout.String())
		}
		if len(stdout.missing()) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		case <-proc.done:
			// Context cancellation can kill the child before this select runs.
			// Preserve that cause instead of reporting its resulting signal exit.
			if ctx.Err() != nil {
				continue
			}
			// done joins output copying, so a last token written between the
			// check above and child exit remains a successful readiness fence.
			if len(stdout.missing()) == 0 {
				return nil
			}
			failure := fmt.Errorf("peer exited before listening (exit=%v, stderr=%q, stdout=%q); "+
				"fix the peer startup failure before running its consumer",
				proc.Wait(), stderr.String(), stdout.String())
			if strings.Contains(stderr.String(), peerNoTestDataMessage) {
				return errors.Join(failure, peerBindFailure(0, stderr.String(), stdout.String()))
			}
			return failure
		}
	}
}
