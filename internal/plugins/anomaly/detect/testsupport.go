// Design: docs/architecture/anomaly/anomaly-4-interop-harness.md -- in-process test composition seam.
package detect

import (
	"github.com/ze-software/ze/internal/component/trafficfeature"
	"github.com/ze-software/ze/internal/core/observation"
	"github.com/ze-software/ze/pkg/ze"
)

// ChainForTest builds a real detector on bus, fed by a real trafficfeature service
// over the global observation feed. It mirrors the detector wiring in RunEngine
// (register.go) so an in-process integration test can drive facts->judgment with no
// plugin lifecycle and no config file. Test-only.
//
// tick feeds the detector one trafficfeature snapshot, which is what the plugin's
// one-second ticker does in production. The caller drives it rather than a real
// ticker so a test controls the window boundaries: an unsynchronized tick can
// sample a window mid-publish and read a partial feature vector.
//
// The caller MUST call stop, which detaches from the feature service. It is the
// companion of this call and there is no other release path.
func ChainForTest(cfg *Config, bus ze.EventBus) (tick, stop func()) {
	svc := trafficfeature.NewService(observation.Global())
	id := svc.Attach() // starts the service and subscribes it to observation.Global()
	d := newDetector(cfg, bus)

	tick = func() { d.onTick(svc.Snapshot()) }
	stop = func() { svc.Detach(id) }
	return tick, stop
}
