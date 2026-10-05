// Design: docs/architecture/testing/interop.md -- received FlowSpec interoperability.
package bgp

func init() {
	specialCheckers[scenarioFlowspecGoBGP] = checkFlowSpecGoBGP
}
