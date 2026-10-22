// Design: docs/architecture/testing/interop.md -- the SR anycast next-hop scenario.
// Related: check_ospf_sr_anycast.go -- the checker it registers.
package bgp

func init() {
	specialCheckers[ospfSRAnycastScenario] = checkOSPFSRAnycastNextHop
}
