// Design: docs/guide/command-reference.md -- request plugin restart, the command this fixture is restarted by
// Related: plugin_fixture_restart_generation.go -- the drivers registered here

package fixture

func init() {
	Register("plugin/restart-generation", restartGenerationDriver)
	Register("plugin/restart-generation-gone", restartGenerationGone)
}
