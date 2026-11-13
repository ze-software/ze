// Design: docs/architecture/testing/interop.md -- OSPF redistribution to eBGP.
// Related: check_ospf_redistribute_default.go -- the checker registered here.
package bgp

func init() {
	specialCheckers[ospfRedistributeDefaultScenario] = checkOSPFRedistributeDefault
	registerPeerPreparation(ospfRedistributeDefaultScenario, prepareOSPFRedistributeDefaultPeers)
}
