//go:build linux

// Design: docs/architecture/provisioning/dhcp-server.md -- Functional tests: the probe and its WAN lease
// Related: dhcp_fixture_probe_learned_linux.go -- dhcpProbeLearnedDriver, the driver this registers

package fixture

func init() {
	Register("dhcp/probe-learned", dhcpProbeLearnedDriver)
}
