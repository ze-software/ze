// Design: ai/rules/plugins.md -- ze_l2tp hub registration gating
//
// L2TP/PPPoE (BNG) subsystem construction for the hub composition root, gated
// on ze_l2tp: fills the bngRegister seam (bng_infra.go) with the construction
// logic that lived inline in main.go, and carries the pppoeclient blank
// import. The other roots are the generated all_ze_l2tp.go group file
// (schemas + BNG plugins) and the gated cmd/ze/dispatch_l2tp.go CLI root.

//go:build ze_l2tp

package hub

import (
	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoeclient"
)

const (
	bngBlockL2TP  = "l2tp"
	bngBlockPPPoE = "pppoe"
)

func init() {
	bngRegister = registerBNGSubsystems
}

// registerBNGSubsystems extracts the L2TP and PPPoE parameters and registers
// each subsystem independently with the engine: L2TP when the operator asked
// for it (Enabled or at least one listener configured) and PPPoE always (it
// starts idle when the config does not enable it). A parse or registration
// error in one subsystem records it in the returned not-started slice and never
// prevents the other from registering.
func registerBNGSubsystems(loadTree *zeconfig.Tree, configTree map[string]any, configPath string, eng *engine.Engine) ([]webPortalService, []serviceNotStarted) {
	var portals []webPortalService
	var down []serviceNotStarted

	portal, l2tpDown := registerL2TPSubsystem(loadTree, configPath, eng)
	if l2tpDown != nil {
		down = append(down, *l2tpDown)
	}
	if portal != nil {
		portals = append(portals, *portal)
	}

	if pppoeDown := registerPPPoESubsystem(configTree, configPath, eng); pppoeDown != nil {
		down = append(down, *pppoeDown)
	}

	return portals, down
}

// registerL2TPSubsystem extracts the l2tp block and registers the L2TP
// subsystem when the operator asked for it. ExtractParameters returns a
// zero-value struct when the config tree has no l2tp block, in which case both
// returned pointers are nil.
func registerL2TPSubsystem(loadTree *zeconfig.Tree, configPath string, eng *engine.Engine) (*webPortalService, *serviceNotStarted) {
	l2tpParams, l2tpErr := l2tp.ExtractParameters(loadTree)
	if l2tpErr != nil {
		d := notStarted(serviceNameL2TP, blockUnservable(serviceNameL2TP, bngBlockL2TP, configPath, l2tpErr))
		return nil, &d
	}
	if !l2tpParams.Enabled && len(l2tpParams.ListenAddrs) == 0 {
		return nil, nil
	}
	if regErr := eng.RegisterSubsystem(l2tp.NewSubsystem(&l2tpParams)); regErr != nil {
		d := notStarted(serviceNameL2TP, buildFailed(serviceNameL2TP, bngBlockL2TP, regErr))
		return nil, &d
	}
	return &webPortalService{Key: bngBlockL2TP, Title: "L2TP Sessions", Path: "/l2tp"}, nil
}

// registerPPPoESubsystem extracts the pppoe block and registers the PPPoE
// subsystem whatever the boot config says: a subsystem that is disabled, or
// names no access interface, starts idle, and its Reload starts it when a
// commit enables it. Registering it only for a boot config that already enabled
// it left a later `pppoe` commit with no Reload to reach, accepted and never
// applied.
func registerPPPoESubsystem(configTree map[string]any, configPath string, eng *engine.Engine) *serviceNotStarted {
	pppoeParams, pppoeErr := pppoe.ExtractParameters(configTree)
	if pppoeErr != nil {
		d := notStarted(serviceNamePPPoE, blockUnservable(serviceNamePPPoE, bngBlockPPPoE, configPath, pppoeErr))
		return &d
	}
	if regErr := eng.RegisterSubsystem(pppoe.NewSubsystem(pppoeParams)); regErr != nil {
		d := notStarted(serviceNamePPPoE, buildFailed(serviceNamePPPoE, bngBlockPPPoE, regErr))
		return &d
	}
	return nil
}
