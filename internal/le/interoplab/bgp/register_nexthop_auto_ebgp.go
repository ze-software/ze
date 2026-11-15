// Design: docs/architecture/bgp/structural-forwarding.md -- default eBGP next-hop rewriting.
// Related: check_nexthop_auto_ebgp.go -- independent receiver assertions.
package bgp

func init() {
	specialCheckers[nextHopAutoEBGPScenario] = checkNextHopAutoEBGP
}
