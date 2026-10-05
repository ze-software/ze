// Design: docs/architecture/testing/interop.md -- foreign collector epoch evidence.
package bgp

func init() {
	specialCheckers[medWholeSetScenario] = checkMEDWholeSet
}
