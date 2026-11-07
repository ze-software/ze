package fixture

// register_listener_migration.go names the scenarios that prove a SIGHUP
// reload moves the web, looking-glass and MCP listeners to new ports, and the
// SSH listener to a new address.
//
// Related: listener_migration_fixture.go -- the web, looking-glass and MCP scenario body
// Related: listener_migration_ssh_fixture.go -- the SSH scenario body
// Related: test/reload/listener-migration-web-lg-mcp.ci -- the web, looking-glass and MCP run
// Related: test/reload/listener-migration-ssh.ci -- the SSH run

func init() {
	Register("reload/listener-migration", listenerMigration)
	Register("reload/listener-migration-ssh", listenerMigrationSSH)
}
