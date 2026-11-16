// Design: docs/labs/l2tp-interop.md -- several LACs with many sessions in each tunnel.
// Related: scenarios.go -- scenario discovery and dispatch.
// Related: scale_plan.go -- topology and cleanup derived from the declared peers.
package l2tp

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioScale = "l2tp-many-sessions-kl2tpd"
	imageZeScale  = "ze-l2tp-scale"
	imageKL2TP    = "ze-l2tp-kl2tpd"
	scaleLACCount = 3
)

func init() {
	ze := l2tpImage(imageZeScale, "test/interop-l2tp", "Dockerfile.ze-scale", false)
	ze.Binaries = []interoplab.LabBinary{{Name: "ze-scale", Base: imageBaseZe, Output: "test/interop-l2tp/ze-scale-linux", PrepareOverlay: prepareScaleAllocatorOverlay}}
	interoplab.RegisterImage(ze)
	interoplab.RegisterImage(l2tpImage(imageKL2TP, "test/interop-l2tp", "Dockerfile.kl2tpd", true))
	templates := scalePeerTemplates("", "", "")
	peers := make([]string, 0, len(templates))
	for index := range templates {
		peers = append(peers, templates[index].Name)
	}
	registerScenario(scenarioRegistration{
		name: scenarioScale, check: checkScaleSessions, failurePeers: peers,
		needsKernel: true, plan: scaleScenarioPlan, images: scaleImageBuilds,
	})
}

// checkScaleSessions is the wiring-stage failure. The AC-2 implementation
// replaces this body after the registered plan has been exercised by its test.
func checkScaleSessions(context.Context, labOperations, time.Duration) error {
	return errors.New("scenario l2tp-many-sessions-kl2tpd assertion 1: scale session proof is not implemented")
}
