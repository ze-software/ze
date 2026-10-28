//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- capability probe for the raw OSPF integration tests

package transport

// rawSocketAvailable reports whether the caller can open the raw proto-89
// socket the wire tests need. Callers fail when it is refused inside userns.Enter.
func rawSocketAvailable() bool {
	return checkRawSocket() == nil
}
