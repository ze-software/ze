// Design: docs/architecture/ospf/ospf-12-auth.md -- durable runtime initialization.
// Related: gr_nvs.go -- restart facts use the same daemon state client.
package ospf

import (
	"context"
	"fmt"
	"time"
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
		ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
		defer cancel()
		boot, err := loadOSPFBootCount(ctx, e.state)
		if err != nil {
			e.stateErr = fmt.Errorf("ospf: initialize durable boot count: %w", err)
			return
		}
		e.auth.setBootCount(boot)
		if err := e.gr.resumeFromNVS(); err != nil {
			e.stateErr = fmt.Errorf("ospf: read graceful restart state: %w", err)
		}
	})
	return e.stateErr
}
