// Design: ai/rules/plugins.md -- ze_l2tp BNG construction seam
//
// Always-on seam for the gated L2TP/PPPoE (BNG) subsystem construction. The
// gated register_l2tp.go sets bngRegister from its init(); main.go calls it
// when non-nil. This is the ssh_infra/gnmi_infra seam shape: the hub's
// construction site cannot go through the listener service registry (the BNG
// registers engine SUBSYSTEMS, not Reconfigurable listeners), so a nil-able
// hook var is the boundary. No l2tp type crosses it: inputs are config trees,
// the config path and the engine handle, outputs are plain webPortalService
// and serviceNotStarted values.

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
)

// bngRegister extracts the l2tp (from the load-result tree) and pppoe (from
// the resolved config map) parameters and registers each subsystem
// independently with the engine, returning the web portal entries to advertise
// and one serviceNotStarted record per subsystem that could not be parsed or
// registered (leaving only that subsystem down while the other and ze keep
// running). nil when ze_l2tp is off: the schema is gated with it, so an
// l2tp/pppoe config cannot load in that build and skipping the call drops
// nothing silently.
var bngRegister func(loadTree *zeconfig.Tree, configTree map[string]any, configPath string, eng *engine.Engine) ([]webPortalService, []serviceNotStarted)
