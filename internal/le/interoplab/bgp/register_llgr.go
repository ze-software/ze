// Design: docs/architecture/testing/interop.md -- preserve GR baseline, add LLGR lifecycle.
package bgp

func init() {
	specialCheckers[scenarioGracefulRestartFRR] = checkLLGRIndependentFRR
	speakerRunners[llgrSourceOracle] = runLLGRSource
}
