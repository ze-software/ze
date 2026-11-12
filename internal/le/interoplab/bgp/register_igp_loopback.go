// Design: docs/architecture/testing/interop.md -- reciprocal IGP loopback routes.
// Related: check_igp_loopback.go -- the checkers registered here.
package bgp

func init() {
	specialCheckers[isisDefaultLevelScenario] = checkISISDefaultLevelLoopbacks
	specialCheckers[ospfLoopbackScenario] = checkOSPFLoopbackPrefixes
}
