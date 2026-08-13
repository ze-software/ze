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
	"fmt"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoeclient"
)

func init() {
	bngRegister = registerBNGSubsystems
}

// registerBNGSubsystems is the moved main.go construction block: it registers
// the L2TP subsystem when the operator asked for it (Enabled or at least one
// listener configured) and the PPPoE subsystem when it has access interfaces,
// and returns the web portal entries to advertise.
func registerBNGSubsystems(loadTree *zeconfig.Tree, configTree map[string]any, eng *engine.Engine) ([]webPortalService, error) {
	var portals []webPortalService

	// L2TP subsystem. ExtractParameters returns a zero-value struct when the
	// config tree has no l2tp block; we only register with the engine when
	// the operator actually asked for L2TP.
	l2tpParams, l2tpErr := l2tp.ExtractParameters(loadTree)
	if l2tpErr != nil {
		return nil, fmt.Errorf("parse l2tp config: %w", l2tpErr)
	}
	if l2tpParams.Enabled || len(l2tpParams.ListenAddrs) > 0 {
		if regErr := eng.RegisterSubsystem(l2tp.NewSubsystem(l2tpParams)); regErr != nil {
			return nil, fmt.Errorf("register l2tp subsystem: %w", regErr)
		}
		portals = append(portals, webPortalService{Key: "l2tp", Title: "L2TP Sessions", Path: "/l2tp"})
	}

	// PPPoE subsystem. ExtractParameters returns defaults when the config
	// tree has no `pppoe {}` block; we only register when the operator
	// configured at least one access interface.
	pppoeParams, pppoeErr := pppoe.ExtractParameters(configTree)
	if pppoeErr != nil {
		return nil, fmt.Errorf("parse pppoe config: %w", pppoeErr)
	}
	if pppoeParams.Enabled && len(pppoeParams.Interfaces) > 0 {
		if regErr := eng.RegisterSubsystem(pppoe.NewSubsystem(pppoeParams)); regErr != nil {
			return nil, fmt.Errorf("register pppoe subsystem: %w", regErr)
		}
	}
	return portals, nil
}
