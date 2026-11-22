// Design: docs/architecture/testing/interop.md -- preserve GR baseline, add LLGR lifecycle.
package bgp

func init() {
	specialCheckers[scenarioGracefulRestartFRR] = checkLLGRIndependentFRR
	specialCheckers["bgp-llgr-per-family-frr"] = checkLLGRPerFamilyFRR
	speakerRunners[llgrSourceOracle] = runLLGRSource
}
