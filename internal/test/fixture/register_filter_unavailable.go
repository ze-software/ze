// Design: docs/plugin-development/handlers.md -- route filters, "unavailable" on a stopped plugin

package fixture

// The AC-33 observer. Its body, and why the RIB is read only after the second
// peer's End-of-RIB, are plugin_fixture_filter_unavailable.go.
func init() {
	Register("plugin/filter-wire-plugin-unavailable", filterWirePluginUnavailable, StopsDaemon)
}
