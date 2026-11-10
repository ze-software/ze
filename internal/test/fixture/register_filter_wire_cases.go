// Design: docs/plugin-development/protocol.md -- The Route Fields and Sets of filter-update.
// Related: plugin_fixture_filter_wire_cases.go -- the post-filter fence drivers.

package fixture

func init() {
	Register("plugin/filter-wire-cases-fence", filterWireCasesFence, StopsDaemon)
	Register("plugin/filter-wire-cases-invalid-fence", filterWireCasesInvalidFence, StopsDaemon)
}
