//go:build linux

// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server these fixtures exchange with
// Related: dhcp_fixture_probe_linux.go -- dhcpLinkDriver and dhcpProbeDriver, the drivers this registers

package fixture

func init() {
	Register("dhcp/link", dhcpLinkDriver)
	Register("dhcp/probe", dhcpProbeDriver)
}
