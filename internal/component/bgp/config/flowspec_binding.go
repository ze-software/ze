// Design: docs/guide/flowspec-protected-router.md -- FlowSpec family detection
// Related: bus_requirements.go -- FlowSpec authorization checks against compiled bus filters

package bgpconfig

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/core/family"
)

func hasFlowSpecFamily(peer *reactor.PeerSettings) (bool, error) {
	flowSpec := false
	for name := range peer.PrefixMaximum {
		registered, known := family.LookupFamily(name)
		if !known {
			return false, fmt.Errorf("peer %s: configured family %q is not registered", peer.Name, name)
		}
		// Only the two FlowSpec SAFIs require the selecting-RIB replay boundary.
		//exhaustive:ignore
		switch registered.SAFI {
		case family.SAFIFlowSpec, family.SAFIFlowSpecVPN:
			flowSpec = true
		}
	}
	return flowSpec, nil
}
