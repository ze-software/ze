// Design: docs/architecture/api/commands.md -- "Discovery: what each catalog reader can see"
// Related: plugin_fixture_command_catalog.go -- the driver registered here

package fixture

// The name is the path a `.ci` file spells after `ze-test fixture`.
func init() {
	Register("plugin/command-catalog-plugin-shape", commandCatalogPluginShape)
}
