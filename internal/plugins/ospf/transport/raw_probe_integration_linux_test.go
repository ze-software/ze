//go:build integration && linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- skip guard for the raw OSPF integration tests

package transport

// rawSocketAvailable reports whether this host opens the raw proto-89 socket
// the integration tests need. It keeps the RFC-tagged units in
// rfc2328_transport_integration_linux_test.go byte for byte unchanged, so
// their discrimination records stay valid now that the probe returns a reason.
func rawSocketAvailable() bool {
	return checkRawSocket() == nil
}
