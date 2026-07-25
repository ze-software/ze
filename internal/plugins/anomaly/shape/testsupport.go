// Design: plan/learned/1054-anomaly-4-interop-harness.md -- in-process test composition seam.
package shape

import (
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	"github.com/ze-software/ze/pkg/ze"
)

// SubscribeForTest constructs an armed responder, mocks the firewall backend so
// arming needs no kernel, subscribes it to the anomaly detect events on bus, and
// returns an accessor for the currently-armed source prefixes plus a
// stop/unsubscribe. It mirrors the responder wiring in RunEngine (register.go)
// so an in-process integration test can exercise judgment->response without the
// plugin lifecycle. Test-only.
func SubscribeForTest(bus ze.EventBus) (armedList func() []string, stop func()) {
	registerTables = func(string, []firewall.Table) {}
	applyAll = func() error { return nil }

	cfg := DefaultConfig()
	cfg.Mode = ModeArmed
	r := newResponder(cfg)

	unsubD := anomalyevent.Detected.Subscribe(bus, r.onDetected)
	unsubO := anomalyevent.Ongoing.Subscribe(bus, r.onOngoing)
	unsubC := anomalyevent.Cleared.Subscribe(bus, r.onCleared)

	armedList = func() []string {
		r.mu.Lock()
		defer r.mu.Unlock()
		out := make([]string, 0, len(r.armed))
		for p := range r.armed {
			out = append(out, p.String())
		}
		return out
	}
	stop = func() {
		unsubD()
		unsubO()
		unsubC()
		r.Stop()
	}
	return armedList, stop
}
