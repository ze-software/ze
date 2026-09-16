package fixture

// register_show_mtu.go names the scenarios that prove `show mtu` measures a
// clamped path, sizes the tunnels that ride it, and reports what it could not
// size.
//
// Related: plugin_fixture_show_mtu.go -- the scenario bodies
// Related: test/plugin/show-mtu-host.ci, show-mtu-exhaustive.ci,
//          show-mtu-json.ci, show-mtu-no-ipsec-component.ci,
//          show-mtu-oversized-tunnels.ci, show-mtu-ike-probe.ci -- the runs

func init() {
	Register("plugin/show-mtu-host", showMTUHost)
	Register("plugin/show-mtu-exhaustive", showMTUExhaustive)
	Register("plugin/show-mtu-json", showMTUJSON)
	Register("plugin/show-mtu-no-tunnels", showMTUNoTunnels)
	Register("plugin/show-mtu-oversized", showMTUOversized)
	Register("plugin/show-mtu-ike-probe", showMTUIKEProbe)
}
