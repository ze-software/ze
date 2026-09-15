package fixture

// register_ping_df.go names the scenario that proves `show ping` with the
// Don't Fragment mode reports the MTU a router answers with.
//
// Related: plugin_fixture_ping_df.go -- the scenario body
// Related: test/plugin/ping-do-not-fragment-reports-mtu.ci -- the privileged run
// Related: test/plugin/ping-do-not-fragment-unprivileged.ci -- the run without CAP_NET_RAW

func init() {
	Register("plugin/ping-do-not-fragment", pingDoNotFragment)
}
