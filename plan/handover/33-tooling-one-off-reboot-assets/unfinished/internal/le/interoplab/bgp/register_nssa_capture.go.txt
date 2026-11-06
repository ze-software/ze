// Design: docs/architecture/testing/interop.md -- first-epoch NSSA observation.
package bgp

func init() {
	registerPeerPreparation("ospf-stub-nssa-frr", prepareNSSAPeers)
}
