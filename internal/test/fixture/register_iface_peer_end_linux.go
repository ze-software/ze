//go:build linux

// Design: docs/functional-tests.md -- reading a link's peer end inside the namespace a daemon answer names

package fixture

// The reader a .ci places after an owned-link barrier, to assert on the peer
// end inside the namespace the daemon's answer names. Its body is
// iface_peer_end_linux.go.
func init() {
	Register("iface/peer-end", ifacePeerEnd)
}
