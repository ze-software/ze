package fixture

// register_listener_migration.go names the scenario that proves a SIGHUP
// reload moves the web, looking-glass and MCP listeners to new ports.
//
// Related: listener_migration_fixture.go -- the scenario body
// Related: test/reload/listener-migration-web-lg-mcp.ci -- the run

func init() {
	Register("reload/listener-migration", listenerMigration)
}
