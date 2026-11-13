// Design: docs/guide/redistribution.md -- live connected removal.
// Related: eventbus.go -- the SDK invokes retirement before acknowledging Bye.

package connected

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/redistevents"
)

// retire drains admitted address/replay publications while the SDK can still
// emit withdrawals. A refusal keeps the observer live and its inventory intact
// for configuration recovery or another Bye. Local paths are released only
// after SDK Run joins event delivery; the engine owns forked path cleanup.
func (o *routeObserver) retire() error {
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	if o.retired {
		return nil
	}
	for prefix := range o.prefixes {
		if err := o.emitID(redistevents.ActionRemove, prefix, 0); err != nil {
			failure := fmt.Errorf("withdraw connected redistribution %s: %w", prefix, err)
			// A failed response can follow a successful remote withdrawal.
			// Restore the complete current set before admitting events again.
			for current := range o.prefixes {
				if restoreErr := o.emitID(redistevents.ActionAdd, current, 0); restoreErr != nil {
					failure = errors.Join(failure, fmt.Errorf("restore connected redistribution %s: %w", current, restoreErr))
				}
			}
			return failure
		}
	}
	o.retired = true
	return nil
}
