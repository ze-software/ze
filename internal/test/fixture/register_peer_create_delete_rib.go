// Design: docs/architecture/api/commands.md -- create bgp peer, delete bgp peer

package fixture

// The runtime peer lifecycle driver. Its body, and why every effect is read
// back from the daemon rather than taken from an answer, are
// plugin_fixture_13_peer_lifecycle.go.
func init() {
	Register("plugin/api-peer-create-delete-rib", peerCreateDeleteRIB13)
}
