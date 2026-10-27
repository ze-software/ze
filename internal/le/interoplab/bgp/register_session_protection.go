// Design: docs/architecture/testing/interop.md -- GTSM and CoPP on ze's BGP listener, observed by a foreign peer.
package bgp

func init() {
	specialCheckers[sessionProtectionScenario] = checkSessionProtection
}
