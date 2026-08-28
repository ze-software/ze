// Design: docs/architecture/core-design.md -- BGP redistribute source registration

package redistribute

import (
	"log/slog"
	"sync"

	"github.com/ze-software/ze/internal/component/config/redistribute"
)

// The redistribute source names this package registers. A source name is what
// an operator writes in `redistribute <source>`, so it is not the consumer name
// (bgpConsumerName) and not the protocol name (protocolName), even where the
// three spell the same word.
const (
	sourceNameBGP  = "bgp"
	sourceNameIBGP = "ibgp"
	sourceNameEBGP = "ebgp"
)

var bgpSourcesOnce sync.Once

func init() {
	RegisterBGPSources()
}

// RegisterBGPSources registers BGP-specific redistribute sources (bgp, ibgp, ebgp).
// Safe to call multiple times (uses sync.Once).
func RegisterBGPSources() {
	bgpSourcesOnce.Do(func() {
		mustRegister(redistribute.RouteSource{
			Name:        sourceNameBGP,
			Protocol:    protocolName,
			Description: "all BGP learned routes",
		})
		mustRegister(redistribute.RouteSource{
			Name:        sourceNameIBGP,
			Protocol:    protocolName,
			Description: "iBGP learned routes",
		})
		mustRegister(redistribute.RouteSource{
			Name:        sourceNameEBGP,
			Protocol:    protocolName,
			Description: "eBGP learned routes",
		})
	})
}

func mustRegister(src redistribute.RouteSource) {
	if err := redistribute.RegisterSource(src); err != nil {
		slog.Error("BUG: failed to register BGP redistribute source", "name", src.Name, "err", err)
	}
}
