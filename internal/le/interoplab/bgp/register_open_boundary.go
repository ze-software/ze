// Design: docs/architecture/testing/interop.md -- the bgp-open-extended-params-boundary-frr scenario.
package bgp

func init() {
	specialCheckers[scenarioOpenExtendedParamsBoundaryFRR] = checkOpenExtendedParamsBoundaryFRR
}
