// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: validate_topic_bfd.go, validate_topic_ospf.go, validate_topic_vrrp.go, validate_topic_traffic.go

package siteterminaldemo

// The ids of the network topics. A topic's observe helper names its own id
// to run the absence declaration registered beside it here.
const (
	demoBFDFailover    = "bfd-failover"
	demoOSPFAdjacency  = "ospf-adjacency"
	demoVRRPTracking   = "vrrp-tracking"
	demoTrafficAnomaly = "traffic-anomaly"
)

func init() {
	registerLiveTopic(demoBFDFailover, "topics/bfd/configure.tape", topicBFDAbsent, checkTopicBFD, awaitTopicBFD)
	registerLiveTopic(demoOSPFAdjacency, "topics/ospf/configure.tape", topicContainersAbsent("ospf"), checkTopicOSPF, awaitTopicOSPF)
	registerLiveTopic(demoVRRPTracking, "topics/vrrp/configure.tape", topicVRRPAbsent, checkTopicVRRPTracking, awaitTopicVRRPTracking)
	registerLiveTopic(demoTrafficAnomaly, "topics/traffic/configure.tape", topicContainersAbsent("traffic"), checkTopicTrafficUsage, awaitTopicTrafficUsage)
}
