// Design: docs/architecture/plugin/plugin-system.md -- runtime declaration precedence.
package registry

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// A current runtime declaration owns both true and false readiness answers;
// compiled metadata is used only when no runtime seam is installed.
// MUTATION: consult compiled registration before the runtime readiness seam.
func TestRuntimeSessionReadinessPrecedesCompiledMetadata(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	t.Cleanup(func() { SetRuntimeSessionReady(nil) })
	const silent = "compiled-readiness-false"
	const reporting = "compiled-readiness-true"
	for _, item := range []struct {
		name  string
		ready bool
	}{{silent, false}, {reporting, true}} {
		require.NoError(t, Register(Registration{Name: item.name, SignalsSessionReady: item.ready,
			RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 }}))
	}
	SetRuntimeSessionReady(func(name string) bool { return name == silent })
	require.True(t, SignalsSessionReady(silent), "a same-named static false must not hide current Stage-1 true")
	require.False(t, SignalsSessionReady(reporting), "a same-named static true must not invent current Stage-1 true")
	require.False(t, SignalsSessionReady("not-declared"))
	SetRuntimeSessionReady(nil)
	require.True(t, SignalsSessionReady(reporting), "static tools keep their compiled default without a server")
	require.False(t, SignalsSessionReady(silent))
}
