// Design: docs/plugin-development/handlers.md -- route filters, the one evaluator both transports call

package fixture

// The AC-35 observer. Its body, and why the RIB is read only after both
// peers' End-of-RIB, are plugin_fixture_filter_thirdparty.go.
func init() {
	Register("plugin/filter-wire-thirdparty-compiled-in", filterWireThirdpartyCompiledIn, StopsDaemon)
}
