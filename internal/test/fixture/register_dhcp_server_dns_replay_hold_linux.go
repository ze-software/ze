//go:build linux

// Design: docs/architecture/provisioning/dhcp-server.md -- Functional tests: the probe and its WAN lease
// Related: dhcp_server_dns_replay_hold_linux.go -- dhcpServerDNSReplayHold, the scenario this registers

package fixture

func init() {
	// The scripted access concentrator brings up the PPPoE lease, then a
	// reload starts the DHCP server after it (AC-13, D-21). The lease is PPP
	// and not DHCPv4 for the reason the resolver's replay scenario gives: a
	// DHCPv4 lease from Ze's own server needs the service block at boot.
	Register("pppoe/dhcp-server-dns-replay-hold",
		reloadingDriver("pppoe/dhcp-server-dns-replay-hold", "dhcp-server-dns-replay-hold-test",
			dhcpServerDNSReplayHold), StopsDaemon)
}
