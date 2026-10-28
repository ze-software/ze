//go:build linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- capability probe for the raw VRRP integration tests

package transport

// rawSocketAvailable reports whether the caller can open the raw proto-112
// socket the wire tests need. Callers fail when it is refused inside userns.Enter.
func rawSocketAvailable() bool {
	return checkRawSocket() == nil
}
