// Design: docs/plugin-development/protocol.md -- The filter-update batch.
// Related: test/plugin/filter-wire-batch.ci -- the concurrent sources.

package filterwirecases

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The external evaluator gives each accepted route a unique MED derived from
// its source and generation. Reading it here, through the daemon's later
// chain member, catches a verdict or set assigned to the wrong batch position.
func filterWireCasesCheckBatch(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	prefix, med, _, err := filterWireCasesNumbers(route)
	if err != nil {
		return nil, err
	}
	network, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("batch prefix: %w", err)
	}
	if !network.Addr().Is4() {
		return nil, fmt.Errorf("batch prefix %s is not IPv4", prefix)
	}
	octets := network.Addr().As4()
	want := uint32(octets[1])*100 + uint32(octets[2])
	if med != want {
		return nil, fmt.Errorf("batch %s MED is %d, want %d: verdict position or per-route sets changed", prefix, med, want)
	}
	filterWireCasesSaid("OK: batch ordered sets "+prefix+" numeric field checked", nil)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}
