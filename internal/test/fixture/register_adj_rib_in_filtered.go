// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7947 Section 2.1 pre-policy Adj-RIB-In retention fixtures
// RFC: rfc/short/rfc7947.md -- Section 2.1
package fixture

func init() {
	Register("plugin/adj-rib-in-filtered-visible", adjRIBInFilteredVisible, StopsDaemon)
	Register("plugin/adj-rib-in-filtered-status", adjRIBInFilteredStatus, StopsDaemon)
	Register("plugin/adj-rib-in-filtered-not-replayed", adjRIBInFilteredNotReplayed)
	Register("plugin/adj-rib-in-filtered-absent-for-plain-peer", adjRIBInFilteredAbsentForPlainPeer, StopsDaemon)
}
