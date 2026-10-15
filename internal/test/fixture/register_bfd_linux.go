// Design: docs/architecture/bfd.md -- the BFD surface the test/bfd suite drives
// Overview: bfd_fixture_link_local_linux.go -- the scenarios this file names

//go:build linux

package fixture

// The link-local fixtures build veth pairs and write raw frames with AF_PACKET,
// which only Linux has. test/bfd/bfd-first-packet-link-local.ci declares
// needs-linux, so no other host names them.
func init() {
	Register("bfd/bfd-first-packet-link-local-setup", bfdFirstPacketLinkLocalSetup)
	Register("bfd/bfd-first-packet-link-local-teardown", bfdFirstPacketLinkLocalTeardown)
	Register("bfd/bfd-first-packet-link-local",
		bfdObserver("bfd-first-packet-link-local-test", bfdFirstPacketLinkLocalScenario))
}
