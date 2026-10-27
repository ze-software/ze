// Design: docs/architecture/testing/interop.md -- native DelayOpen scenario.
package bgp

func init() {
	specialCheckers[delayOpenScenario] = checkDelayOpenFRR
}
