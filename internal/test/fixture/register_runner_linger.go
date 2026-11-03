// Design: docs/architecture/testing/runner-architecture.md -- native lifecycle proof.

package fixture

func init() {
	Register("runner/owned-peer-linger", ownedPeerLinger)
	Register("runner/owned-peer-wire", ownedPeerWire)
	Register("runner/owned-peer-closed", ownedPeerClosed)
	Register("runner/owned-peer-fence", ownedPeerFence)
}
