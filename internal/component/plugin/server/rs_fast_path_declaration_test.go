// Design: docs/architecture/bgp/structural-forwarding.md -- the fast path's send permission
// Related: events_session_ready_test.go -- the same alias seam for FencesLiveForwards
//
// VALIDATES: registry.RelaysOnRSFastPath answers for a route server attached
// under an alias (`internal relay { use bgp-rs }`), through the plugin server's
// runtimeRegistryNames seam.
// PREVENTS: the reactor fast path refusing every destination of an aliased
// route server, because the binding's process name is not a registry key.
package server

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestRelaysOnRSFastPathResolvesTheProcessAlias registers one declaring and
// one silent implementation, runs each under an alias, and asks the registry
// about the alias, the implementation name and an unknown process.
func TestRelaysOnRSFastPathResolvesTheProcessAlias(t *testing.T) {
	const relaying, silent = "test-rs-relay-implementation", "test-rs-silent-implementation"
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	for _, reg := range []registry.Registration{
		{Name: relaying, RelaysOnRSFastPath: true},
		{Name: silent},
	} {
		reg.Description = "route-server fast path declaration test fixture"
		reg.RunEngine = func(net.Conn) int { return 0 }
		reg.CLIHandler = func([]string) int { return 0 }
		require.NoError(t, registry.Register(reg))
	}

	s := sessionReadyTestServer()
	runningProcess(s, "aliased-relay", relaying)
	runningProcess(s, "aliased-silent", silent)
	registry.SetRuntimeRegistryNames(s.processRegistryNames)
	t.Cleanup(func() { registry.SetRuntimeRegistryNames(nil) })

	assert.True(t, registry.RelaysOnRSFastPath("aliased-relay"),
		"the alias must be answered from the registration its use spelling names")
	assert.True(t, registry.RelaysOnRSFastPath(relaying),
		"a process attached under the implementation name answers the same way")
	assert.False(t, registry.RelaysOnRSFastPath("aliased-silent"),
		"a plugin that declares nothing is no route server for the fast path")
	assert.False(t, registry.RelaysOnRSFastPath("never-started"),
		"a process the server cannot see grants the fast path nothing")
}
