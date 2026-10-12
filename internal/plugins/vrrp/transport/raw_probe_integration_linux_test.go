//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- skip guard for the raw VRRP integration tests

package transport

// rawSocketAvailable reports whether this host opens the raw proto-112 socket
// the integration tests need. It keeps the RFC-tagged units in
// transport_integration_linux_test.go byte for byte unchanged, so their
// discrimination records stay valid now that the probe returns a reason.
func rawSocketAvailable() bool {
	return checkRawSocket() == nil
}
