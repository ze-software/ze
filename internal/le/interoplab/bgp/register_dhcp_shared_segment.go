// Design: docs/architecture/testing/interop.md -- ze's DHCPv4 server serving two subnets on one interface to dhcpcd.
package bgp

func init() {
	specialCheckers[scenarioDHCPSharedSegment] = checkDHCPSharedSegment
	registerPeerPreparation(scenarioDHCPSharedSegment, prepareDHCPSharedSegmentPeers)
}
