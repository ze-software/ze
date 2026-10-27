// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: validate_topic_bfd.go, validate_topic_ospf.go, validate_topic_vrrp.go, validate_topic_traffic.go

package siteterminaldemo

func init() {
	registerLiveTopic("bfd-failover", "topics/bfd/configure.tape", checkTopicBFD, awaitTopicBFD)
	registerLiveTopic("ospf-adjacency", "topics/ospf/configure.tape", checkTopicOSPF, awaitTopicOSPF)
	registerLiveTopic("vrrp-tracking", "topics/vrrp/configure.tape", checkTopicVRRPTracking, awaitTopicVRRPTracking)
	registerLiveTopic("traffic-anomaly", "topics/traffic/configure.tape", checkTopicTrafficUsage, awaitTopicTrafficUsage)
}
