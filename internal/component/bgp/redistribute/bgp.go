// Design: docs/architecture/core-design.md -- BGP redistribute source registration

package redistribute

import (
	"log/slog"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
)

var bgpSourcesOnce sync.Once

// RegisterBGPSources registers BGP-specific redistribute sources (bgp, ibgp, ebgp).
// Safe to call multiple times (uses sync.Once).
func RegisterBGPSources() {
	bgpSourcesOnce.Do(func() {
		mustRegister(redistribute.RouteSource{
			Name:        "bgp",
			Protocol:    "bgp",
			Description: "all BGP learned routes",
		})
		mustRegister(redistribute.RouteSource{
			Name:        "ibgp",
			Protocol:    "bgp",
			Description: "iBGP learned routes",
		})
		mustRegister(redistribute.RouteSource{
			Name:        "ebgp",
			Protocol:    "bgp",
			Description: "eBGP learned routes",
		})
	})
}

func mustRegister(src redistribute.RouteSource) {
	if err := redistribute.RegisterSource(src); err != nil {
		slog.Error("BUG: failed to register BGP redistribute source", "name", src.Name, "err", err)
	}
}
