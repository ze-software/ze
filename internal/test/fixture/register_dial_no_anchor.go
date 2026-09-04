// Design: docs/architecture/api/process-protocol.md -- the plugin trust anchor

package fixture

// The AC-4 driver. Its body, and why the assertion is on the returned plugin
// rather than on a log line, are plugin_fixture_dial_no_anchor.go.
func init() {
	Register("plugin/dial-no-anchor", pluginDialNoAnchor)
}
