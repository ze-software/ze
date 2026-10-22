// Design: docs/architecture/exabgp-bridge.md -- the family an `announce eor` or `announce route-refresh` line names
// Related: bridge_route_forms.go -- convertEOR and convertRouteRefresh, the callers
// Related: bridge_family.go -- canonicalExabgpSAFI, the SAFI vocabulary

package bridge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/family"
)

// pairFamily answers the ze family an `<afi> <safi>` pair names on a control
// line, `announce eor <afi> <safi>` or `announce route-refresh <afi> <safi>`.
// Both lines carry no route, so any family ze carries is a valid answer,
// l2vpn vpls and bgp-ls bgp-ls included.
//
// The answer is read from the family registry, the one declaration of what ze
// carries. It was a hand-written AFI set and the route SAFI map until
// 2026-10-22, which named neither l2vpn evpn, l2vpn vpls nor bgp-ls bgp-ls,
// so ExaBGP 4.2 and 5.0 scripts sending those were refused a line ze can send.
// A family no linked plugin registers is refused here, as ze's own handler
// would refuse it, with the families ze does carry.
//
// needs is the caller's sentinel, which every refusal wraps.
func pairFamily(needs error, afi, safi string) (string, error) {
	name := safiFamily(afi, canonicalExabgpSAFI(safi))
	if _, ok := family.LookupFamily(name); ok {
		return name, nil
	}
	names := family.RegisteredFamilyNames()
	slices.Sort(names)
	var withAFI []string
	afis := map[string]bool{}
	for _, registered := range names {
		registeredAFI, _, _ := strings.Cut(registered, "/")
		afis[registeredAFI] = true
		if registeredAFI == afi {
			withAFI = append(withAFI, exabgpFamilyName(registered))
		}
	}
	if !afis[afi] {
		known := make([]string, 0, len(afis))
		for each := range afis {
			known = append(known, each)
		}
		slices.Sort(known)
		return "", fmt.Errorf("%w: %q names no address family ze carries; the address families are %s",
			needs, afi, strings.Join(known, ", "))
	}
	return "", fmt.Errorf("%w: %q names no subsequent address family ze carries for %s; the %s families are %s",
		needs, safi, afi, afi, strings.Join(withAFI, ", "))
}
