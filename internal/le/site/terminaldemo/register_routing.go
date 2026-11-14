// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios_routing.go -- live RPKI and IRR checks.

package siteterminaldemo

func init() {
	registerLiveTopic(demoRPKI, "topics/rpki/configure.tape", topicContainersAbsent("bgp/rpki"), checkTopicRPKI, awaitTopicRPKI)
	registerLiveTopic("irr-filter", "topics/irr/configure.tape", topicContainersAbsent("bgp/irr"), checkTopicIRR, awaitTopicIRR)
}
