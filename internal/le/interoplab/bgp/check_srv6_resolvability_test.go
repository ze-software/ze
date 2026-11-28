// Design: docs/architecture/testing/interop.md -- unit proofs for SRv6 resolvability interop predicates.
// Related: check_srv6_resolvability.go -- predicates tested here.
package bgp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSRv6ResolvabilityPredicates validates the pure JSON parsers used by
// checkSRv6ResolvabilityFRR against excluded, resolved, empty, and malformed
// Ze and FRR outputs.
// VALIDATES: srv6ResolvabilityRequireRowExcluded, srv6ResolvabilityRequireBestVPN,
//
//	srv6ResolvabilityRequireDoctor, and srv6ResolvabilityRequireFRRVPN.
//
// PREVENTS: vacuous absence checks or mis-parsed FRR/Ze JSON envelopes.
func TestSRv6ResolvabilityPredicates(t *testing.T) {
	excludedJSON := `{"routes":[{"peer":"172.30.0.3","family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:b892/152","srv6-sid-resolvable":false,"ineligible-reason":"srv6-sid-unresolvable"}]}`
	excludedHumanJSON := `{"routes":[{"peer":"172.30.0.3","family":"ipv6/mpls-vpn","prefix":"2001:db8:9252:1::/64","srv6-sid-resolvable":false,"ineligible-reason":"srv6-sid-unresolvable"}]}`
	resolvedJSON := `{"routes":[{"peer":"172.30.0.3","family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:b892/152"}]}`
	otherPeerJSON := `{"routes":[{"peer":"172.30.0.9","family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:b892/152","srv6-sid-resolvable":false,"ineligible-reason":"srv6-sid-unresolvable"}]}`
	wrongPrefixExcludedJSON := `{"routes":[{"peer":"172.30.0.3","family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:dead/152","srv6-sid-resolvable":false,"ineligible-reason":"srv6-sid-unresolvable"}]}`
	wrongPrefixResolvedJSON := `{"routes":[{"peer":"172.30.0.3","family":"ipv6/mpls-vpn","prefix":"fc00:0:1::/48"}]}`

	require.NoError(t, srv6ResolvabilityRequireRowExcluded(excludedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, true))
	require.NoError(t, srv6ResolvabilityRequireRowExcluded(excludedHumanJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, true))
	require.Error(t, srv6ResolvabilityRequireRowExcluded(excludedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, false))
	require.NoError(t, srv6ResolvabilityRequireRowExcluded(resolvedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, false))
	require.Error(t, srv6ResolvabilityRequireRowExcluded(resolvedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, true))
	require.Error(t, srv6ResolvabilityRequireRowExcluded(otherPeerJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, true))
	require.Error(t, srv6ResolvabilityRequireRowExcluded(wrongPrefixExcludedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, true))
	require.Error(t, srv6ResolvabilityRequireRowExcluded(wrongPrefixResolvedJSON, "172.30.0.3", srv6ResolvabilityVPNFamily, false))
	require.Error(t, srv6ResolvabilityRequireRowExcluded("not-json", "172.30.0.3", srv6ResolvabilityVPNFamily, true))

	bestWithVPN := `{"best-path":[{"family":"ipv4/unicast","prefix":"10.92.52.0/24","best-peer":"172.30.0.3"},{"family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:b892/152","best-peer":"172.30.0.3"}]}`
	bestArrayWithVPN := `[{"family":"ipv4/unicast","prefix":"10.92.52.0/24","best-peer":"172.30.0.3"},{"family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:b892/152","best-peer":"172.30.0.3"}]`
	bestWrongPrefixVPN := `[{"family":"ipv6/mpls-vpn","prefix":"1:100:fd:e900:0:120:10d:dead/152","best-peer":"172.30.0.3"}]`
	bestNoVPN := `[{"family":"ipv4/unicast","prefix":"10.92.52.0/24","best-peer":"172.30.0.3"}]`
	require.NoError(t, srv6ResolvabilityRequireBestVPN(bestWithVPN, "172.30.0.3", true))
	require.NoError(t, srv6ResolvabilityRequireBestVPN(bestArrayWithVPN, "172.30.0.3", true))
	require.Error(t, srv6ResolvabilityRequireBestVPN(bestArrayWithVPN, "172.30.0.3", false))
	require.NoError(t, srv6ResolvabilityRequireBestVPN(bestArrayWithVPN, "172.30.0.9", false))
	require.Error(t, srv6ResolvabilityRequireBestVPN(bestWrongPrefixVPN, "172.30.0.3", true))
	require.NoError(t, srv6ResolvabilityRequireBestVPN(bestWrongPrefixVPN, "172.30.0.3", false))
	require.NoError(t, srv6ResolvabilityRequireBestVPN(bestNoVPN, "172.30.0.3", false))
	require.Error(t, srv6ResolvabilityRequireBestVPN(bestNoVPN, "172.30.0.3", true))
	require.Error(t, srv6ResolvabilityRequireBestVPN("not-json", "172.30.0.3", false))

	docWithFinding := `{"diagnostics":[{"code":"doctor-bgp-srv6-sid-unresolvable","severity":"warning","message":"1 BGP route excluded","fix":"ze show bgp rib | match srv6-sid-unresolvable"}]}`
	docEmpty := `{"diagnostics":[]}`
	require.NoError(t, srv6ResolvabilityRequireDoctor(docWithFinding, true))
	require.Error(t, srv6ResolvabilityRequireDoctor(docWithFinding, false))
	require.NoError(t, srv6ResolvabilityRequireDoctor(docEmpty, false))
	require.Error(t, srv6ResolvabilityRequireDoctor(docEmpty, true))
	require.Error(t, srv6ResolvabilityRequireDoctor("not-json", false))

	frrEmpty := `{}`
	frrPresent := `{"totalRoutes":1,"totalPaths":1,"routes":{"routeDistinguishers":{"65001:1":{"2001:db8:9252:1::/64":[{"peerId":"172.30.0.2","path":"65001"}]}}}}`
	frrWrongPeer := `{"totalRoutes":1,"totalPaths":1,"routes":{"routeDistinguishers":{"65001:1":{"2001:db8:9252:1::/64":[{"peerId":"172.30.0.9","path":"65001"}]}}}}`

	require.NoError(t, srv6ResolvabilityRequireFRRVPN(frrEmpty, srv6ResolvabilityVPNRD, "172.30.0.2", ""))
	require.Error(t, srv6ResolvabilityRequireFRRVPN(frrPresent, srv6ResolvabilityVPNRD, "172.30.0.2", ""))
	require.Error(t, srv6ResolvabilityRequireFRRVPN("null", srv6ResolvabilityVPNRD, "172.30.0.2", ""))

	require.NoError(t, srv6ResolvabilityRequireFRRVPN(frrPresent, srv6ResolvabilityVPNRD, "172.30.0.2", srv6IneligiblePrefix))
	require.Error(t, srv6ResolvabilityRequireFRRVPN(frrEmpty, srv6ResolvabilityVPNRD, "172.30.0.2", srv6IneligiblePrefix))
	require.Error(t, srv6ResolvabilityRequireFRRVPN(frrWrongPeer, srv6ResolvabilityVPNRD, "172.30.0.2", srv6IneligiblePrefix))
}
