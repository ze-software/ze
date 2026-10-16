package fixture

// register_pool_stats.go names the scenario that proves `show metrics pool`
// reaches the rib-owned pool-stats handler and names every attribute pool.
//
// Related: plugin_fixture_pool_stats.go -- the scenario body
// Related: test/plugin/bgp-pool-stats.ci -- the run

func init() {
	Register("plugin/bgp-pool-stats", observe04(poolStats))
}
