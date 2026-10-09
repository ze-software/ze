// Design: docs/architecture/testing/interop.md -- received-route next-hop identity proof.
// Related: helper_srv6_identity.go -- public SDK export filter, not a reactor seam.
package bgp

const (
	srv6IdentityScenario          = "bgp-srv6-next-hop-identity-frr"
	srv6IdentityUnchangedScenario = "bgp-srv6-next-hop-identity-unchanged-frr"
	srv6IdentityRelease           = "request interop srv6-release"
	srv6IdentityState             = "show interop srv6-policy"
)

func init() {
	registerProcessHelper(srv6IdentityScenario, runSRv6IdentityPolicy)
	specialCheckers[srv6IdentityScenario] = checkSRv6Identity
	specialCheckers[srv6IdentityUnchangedScenario] = checkSRv6Identity
}
