// Design: docs/architecture/core-design.md -- BGP redistribute source registration

package redistribute

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

var bgpSourcesOnce sync.Once

// RegisterBGPSources registers BGP-specific redistribute sources (ibgp, ebgp)
// and wires the validator callbacks. Safe to call multiple times (uses sync.Once).
func RegisterBGPSources() {
	bgpSourcesOnce.Do(func() {
		RegisterSource(RouteSource{
			Name:        "ibgp",
			Protocol:    "bgp",
			Description: "iBGP learned routes",
		})
		RegisterSource(RouteSource{
			Name:        "ebgp",
			Protocol:    "bgp",
			Description: "eBGP learned routes",
		})
		config.SetRedistributeSourceCallbacks(
			func(name string) bool {
				_, ok := LookupSource(name)
				return ok
			},
			SourceNames,
		)
	})
}
