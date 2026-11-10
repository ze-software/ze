// Design: docs/architecture/ospf/ospf-12-auth.md -- durable runtime initialization.
// Related: gr_nvs.go -- restart facts use the same daemon state client.
package ospf

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// initializeState MUST complete before subscribing to interface events or opening
// packet transports. Constructors and configure callbacks MUST only attach the
// client; the initial call belongs to OnStarted, and reload calls it for each new
// engine. sync.Once retains a failure so a later call cannot bypass the gate.
func (e *engine) initializeState() error {
	if e.state == nil {
		return nil
	}
	e.stateOnce.Do(func() {
		boot, err := e.incrementBootCount()
		if err != nil {
			e.stateErr = fmt.Errorf("ospf: initialize durable boot count: %w", err)
			return
		}
		e.auth.setBootCount(boot, e.incrementBootCount)
		if err := e.gr.resumeFromNVS(); err != nil {
			e.stateErr = fmt.Errorf("ospf: read graceful restart state: %w", err)
		}
	})
	return e.stateErr
}

// incrementBootCount is used at startup and at sequence wrap. The caller MUST
// withhold packets until this durable increment succeeds.
func (e *engine) incrementBootCount() (uint32, error) {
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()
	return loadOSPFBootCount(ctx, e.state)
}

// runningStateFix is the operator's first step when an apply could not put
// the engines back to the running config. The only error applyEngines answers
// is initializeState's, through instanceManager.reconcile and
// v6EngineSet.apply: an engine the undo recreates must read its boot count and
// restart state from the daemon's store again. The daemon's store refuses with
// statestore.ErrUnavailable, whose text names `ze init`, when it has none, and
// `ze doctor` reports a store it cannot read, a corrupt one, or a full disk.
const runningStateFix = "OSPF starts an engine only after it reads its boot count and graceful restart state " +
	"from ze's runtime state store: run `ze doctor` and fix the store problem it reports, " +
	"or run `ze init` if the error above says ze has no persistent store"

// runningEnginesLost is the error of an apply that failed with applyErr and
// whose undo, the apply of the running config, failed with undoErr. It wraps
// sdk.ErrRunningConfigLost, so the SDK opens the rollback gate and
// OnConfigRollback tries the repair again, and it carries the problem, the
// SDK's why and the fix, whose last step is the daemon restart.
func runningEnginesLost(applyErr, undoErr error) error {
	return sdk.RunningConfigLost(fmt.Errorf("%w; putting the running config back: %w", applyErr, undoErr),
		"OSPF could not apply the new config, and could not put its engines back to the running config either",
		runningStateFix)
}
