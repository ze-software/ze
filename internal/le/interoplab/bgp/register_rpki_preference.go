// Design: docs/architecture/testing/interop.md -- RTR cache preference, failover and fail-back observed at BIRD.
// Related: test/interop/scenarios/rtr-stayrtr-preference -- two StayRTR caches with different VRP sets.
package bgp

func init() {
	specialCheckers[rpkiPreferenceScenario] = checkRPKICachePreference
}
