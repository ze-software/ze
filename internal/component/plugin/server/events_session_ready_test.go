package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// registerSessionReadyPlugin registers a throwaway in-tree plugin that declares
// the session-ready report, and restores the registry when the test ends.
func registerSessionReadyPlugin(t *testing.T, name string, declares bool) {
	t.Helper()
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:                name,
		Description:         "session-ready declaration test fixture",
		SignalsSessionReady: declares,
		RunEngine:           func(net.Conn) int { return 0 },
		CLIHandler:          func([]string) int { return 0 },
	}))
}

// runningProcess puts one process under the name the operator gave it, with the
// run/use spelling the config carried.
func runningProcess(s *Server, name, run string) *process.Process {
	pm := s.procManager.Load()
	proc := process.NewProcess(plugin.PluginConfig{Name: name, Internal: true, Run: run})
	pm.AddProcess(name, proc)
	return proc
}

func sessionReadyTestServer() *Server {
	s := &Server{
		config:        &ServerConfig{},
		registry:      plugin.NewPluginRegistry(),
		loadedPlugins: make(map[string]bool),
	}
	s.procManager.Store(process.NewProcessManager(nil))
	return s
}

// TestDeclaresSessionReadyResolvesTheProcessAlias drives the seam the reactor's
// initial-sync barrier reaches through registry.SignalsSessionReady.
//
// VALIDATES: a process an operator attached under an alias is answered from the
// registration its `use` spelling names, and an alias whose implementation
// declares nothing is still answered false.
// PREVENTS: the barrier silently dropping every aliased binding of a declaring
// plugin. `plugin { internal rs { use bgp-rs } }` runs the process as "rs" while
// the registration is filed under "bgp-rs", and the repository's own scenarios
// bind the alias form (82 `internal rs`, 86 `internal adj-rib-in`, 28
// `internal rib`). Answering false for those names sends the peer's End-of-RIB
// while the plugin is still replaying, so the marker claims an initial routing
// update that has not completed (RFC 4724 Section 4).
func TestDeclaresSessionReadyResolvesTheProcessAlias(t *testing.T) {
	const (
		declaring = "test-session-ready-implementation"
		silent    = "test-session-ready-silent-implementation"
	)
	registerSessionReadyPlugin(t, declaring, true)
	registerSessionReadyPlugin(t, silent, false)

	s := sessionReadyTestServer()
	runningProcess(s, "aliased-declarer", declaring)
	runningProcess(s, "aliased-silent", silent)
	runningProcess(s, declaring, declaring)

	assert.True(t, s.declaresSessionReady("aliased-declarer"),
		"the alias must be answered from the registration its use spelling names")
	assert.True(t, s.declaresSessionReady(declaring),
		"a process attached under the implementation name answers the same way")
	assert.False(t, s.declaresSessionReady("aliased-silent"),
		"resolving the alias must not turn a plugin that declares nothing into a reporter")
	assert.False(t, s.declaresSessionReady("never-started"),
		"a process the server cannot see sends no report")
}

// TestDeclaresSessionReadyReadsTheStageOneDeclaration keeps the external route
// open beside the alias resolution above.
//
// VALIDATES: a running process that declared signals-session-ready at Stage 1 is
// answered true even though no compile-time registration exists for it.
// PREVENTS: the alias resolution replacing the only route an EXTERNAL plugin has
// to the declaration (test/plugin/initial-sync-barrier-raw.ci).
func TestDeclaresSessionReadyReadsTheStageOneDeclaration(t *testing.T) {
	s := sessionReadyTestServer()

	external := runningProcess(s, "external-reporter", "ze-test fixture plugin/initial-sync-barrier-raw")
	assert.False(t, s.declaresSessionReady("external-reporter"),
		"an external process that declared nothing is not waited for")

	external.SetRegistration(&plugin.PluginRegistration{SignalsSessionReady: true})
	assert.True(t, s.declaresSessionReady("external-reporter"),
		"the Stage-1 declaration is the external plugin's route to the barrier")
}
