// Design: docs/architecture/api/commands.md — command-owned pipe filters, declared by a plugin
// Related: plugin_fixture_pipe_operation.go — the body of this provider

package fixture

// The provider test/plugin/plugin-pipe-operation.ci forks: a plugin that
// declares a pipe operation at Stage 1 and answers what its handler received.
func init() {
	Register("plugin/pipe-operation-declared", pipeOperationProvider)
}
