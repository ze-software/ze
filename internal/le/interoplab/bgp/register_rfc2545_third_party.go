// Design: docs/architecture/testing/interop.md -- RFC 2545 Section 3 for a third-party next hop.
// Related: check_rfc2545_third_party.go -- checkRFC2545ThirdPartyLinkLocal, the checker this file registers.
package bgp

func init() {
	specialCheckers[rfc2545ThirdPartyScenario] = checkRFC2545ThirdPartyLinkLocal
}
