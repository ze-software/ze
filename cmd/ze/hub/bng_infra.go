// Design: ai/rules/feature-gate-registration.md -- ze_l2tp BNG construction seam
//
// Always-on seam for the gated L2TP/PPPoE (BNG) subsystem construction. The
// gated register_l2tp.go sets bngRegister from its init(); main.go calls it
// when non-nil. This is the ssh_infra/gnmi_infra seam shape: the hub's
// construction site cannot go through the listener service registry (the BNG
// registers engine SUBSYSTEMS, not Reconfigurable listeners), so a nil-able
// hook var is the boundary. No l2tp type crosses it: inputs are config trees
// and the engine handle, outputs are plain webPortalService values.

package hub

import (
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/engine"
)

// bngRegister extracts the l2tp (from the load-result tree) and pppoe (from
// the resolved config map) parameters and registers the enabled subsystems
// with the engine, returning the web portal entries to advertise. nil when
// ze_l2tp is off: the schema is gated with it, so an l2tp/pppoe config cannot
// load in that build and skipping the call drops nothing silently.
var bngRegister func(loadTree *zeconfig.Tree, configTree map[string]any, eng *engine.Engine) ([]webPortalService, error)
