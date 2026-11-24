// Design: docs/architecture/testing/interop.md -- spec-fib-depth BGP/FIB interop registration.
// Related: check_fib_depth.go -- bespoke checkers and pure output parsers for ECMP, recursive NH, and IGP cost.
package bgp

import "time"

const (
	scenarioECMPFRR        = "bgp-ecmp-frr"
	scenarioRecursiveNHFRR = "bgp-recursive-nh-frr"
	scenarioIGPCostBIRD    = "bgp-igp-cost-bird"

	ecmpEqualPrefix   = "10.100.0.0/24"
	ecmpUnequalPrefix = "10.100.2.0/24"

	recursiveSubjectPrefix      = "10.200.0.0/24"
	recursiveUnresolvablePrefix = "10.200.1.0/24"
	recursiveDirectPrefix       = "10.200.2.0/24"
	recursiveStaticCoverPrefix  = "10.250.0.0/24"
	recursiveResolvableNH       = "10.250.0.1"
	recursiveUnresolvableNH     = "10.252.0.1"

	igpCostSubjectPrefix     = "10.210.0.0/24"
	igpCostFRRNextHop        = "10.221.0.1"
	igpCostGoBGPNextHop      = "10.222.0.1"
	igpCostStepName          = "igp-cost"
	igpCostBIRDPeerA         = "peer_a"
	igpCostBIRDPeerB         = "peer_b"
	igpCostBIRDCheapStatic   = "igp_cost_b_cheap"
	igpCostBIRDExpensiveStat = "igp_cost_b_expensive"

	fibDepthZeProtocol        = "250"
	fibDepthFRRProtocolBGP    = "bgp"
	fibDepthCmdIP             = "ip"
	fibDepthFlagIPv4          = "-4"
	fibDepthFlagJSON          = "-j"
	fibDepthFlagNumeric       = "-N"
	fibDepthGoBGPDel          = "del"
	fibDepthGoBGPOriginIGP    = "igp"
	fibDepthGoBGPLocalPref    = "local-pref"
	fibDepthGoBGPMED          = "med"
	fibDepthZeShowECMPGroups  = "show ecmp-groups"
	fibDepthZeShowNextHopTbl  = "show nexthop-table"
	fibDepthZeShowRIBReceived = "show bgp rib | received"
	fibDepthWaitBudget        = 90 * time.Second
)

func init() {
	specialCheckers[scenarioECMPFRR] = checkECMPFRR
	specialCheckers[scenarioRecursiveNHFRR] = checkRecursiveNHFRR
	specialCheckers[scenarioIGPCostBIRD] = checkIGPCostBIRD
}
