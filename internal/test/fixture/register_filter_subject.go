// Design: docs/plugin-development/protocol.md -- The Route Fields of filter-update

package fixture

// The filter-subject driver. Its body, and why the fields are checked in
// compiled code rather than in the .ci file, are plugin_fixture_filter_subject.go.
func init() {
	Register("plugin/filter-subject-names-every-attribute", filterSubjectObserver, StopsDaemon)
}
