// Design: docs/architecture/testing/interop.md -- established L2TP scenario declarations.
// Related: scenarios.go -- immutable init-time scenario registry.
// Related: l2tp.go -- the established peer preparation and image selection.
package l2tp

import "github.com/ze-software/ze/internal/le/interoplab"

func init() {
	registerStandardScenario(scenarioPPPIPv4, checkPPPIPv4, true, peerZe, peerLAC)
	registerStandardScenario(scenarioBGPRedistribute, checkBGPRedistribute, true, peerZe, peerLAC, peerFRR)
	registerStandardScenario(scenarioInitiator, checkInitiator, false, peerZe, peerLAC)
	registerStandardScenario(scenarioRadiusAttrs, checkRadiusAttributes, true, peerZe, peerLAC, peerRadius)
	registerStandardScenario(scenarioUnknownMandatoryAVP, checkUnknownMandatoryAVP, true, peerZe, peerLAC, peerAVPRelay)
	registerStandardScenario(scenarioDNSResolver, checkDNSResolverSubscriber, true, peerZe, peerLAC, peerKnot)
	registerStandardScenario(scenarioPPPCodeReject, checkPPPCodeReject, true, peerZe, peerLAC, peerAVPRelay)
	registerStandardScenario(scenarioLNSMRU, checkLNSMRU, true, peerZe, peerLAC, peerRadius)
	registerStandardScenario(scenarioLNSOmittedMRU, checkLNSOmittedMRU, true, peerZe, peerLAC, peerRadius)
	registerStandardScenario(scenarioLNSNegotiatedMRU, checkLNSNegotiatedMRU, true, peerZe, peerLAC, peerRadius)
	registerStandardScenario(scenarioLNSCHAPOnly, checkLNSCHAPOnly, true, peerZe, peerLAC, peerRadius)
	registerStandardScenario(scenarioProxyMRU, checkProxyMRU, true, peerZe)
	registerStandardScenario(scenarioRadiusFixture, checkRadiusAccountingFixture, true, peerZe)
	registerStandardScenario(scenarioLCPRestart, checkLCPRestart, true, peerZe, peerLAC, peerAVPRelay)
	registerStandardScenario(scenarioIPv6CPRejectedByLNS, checkIPv6CPRejectedByLNS, true, peerZe, peerLAC, peerAVPRelay)
	registerStandardScenario(scenarioNoIPRejectsIPCP, checkNoIPRejectsIPCP, true, peerZe, peerLAC, peerAVPRelay)
	registerStandardScenario(scenarioIPCPTerminateRedist, checkIPCPTerminateRedistribute, true,
		peerZe, peerLAC, peerAVPRelay, peerFRR)
}

func registerStandardScenario(name string, check scenarioCheck, needsKernel bool, peers ...string) {
	registerScenario(scenarioRegistration{
		name: name, check: check, failurePeers: peers, needsKernel: needsKernel,
		plan: standardScenarioPlan, images: standardScenarioImages,
	})
}

func standardScenarioImages(root string, source interoplab.ScenarioSource, frrImage string) []interoplab.ImageBuild {
	return standardImageBuilds(root, []interoplab.ScenarioPlan{{Source: source}}, frrImage)
}
