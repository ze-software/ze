// Design: docs/plugin-development/schema.md -- an external plugin brings its own YANG module
// Overview: plugin_fixture_external_yang.go -- the driver this file names

package fixture

func init() {
	Register("plugin/external-yang", externalYANGDriver)
}
