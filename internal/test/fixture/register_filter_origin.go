// Design: docs/architecture/api/process-protocol.md -- the filter text protocol

package fixture

// The AC-11 driver. Its body, and why the verdict is checked in compiled code
// rather than in the .ci file, are plugin_fixture_filter_origin.go.
func init() {
	Register("plugin/filter-match-on-origin", filterMatchOnOrigin)
}
