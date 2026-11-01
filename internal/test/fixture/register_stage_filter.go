// Design: docs/plugin-development/handlers.md -- route filters, auto-inserted members

package fixture

// The stage-filter observer. Its body is plugin_fixture_stage_filter.go.
func init() {
	Register("plugin/stage-filter-inserted", stageFilterInserted)
}
