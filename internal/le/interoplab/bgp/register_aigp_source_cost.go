// Design: docs/architecture/testing/interop.md -- registered metric-control helper.
package bgp

func init() {
	registerProcessHelper(scenarioAIGPSourceCostFRR, runAIGPSourceCostProcess)
	specialCheckers[scenarioAIGPSourceCostFRR] = checkAIGPSourceCostFRR
}
