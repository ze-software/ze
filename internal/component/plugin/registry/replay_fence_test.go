// Design: docs/architecture/plugin/plugin-system.md -- owned runtime fence lookup.
package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplayFenceLookupReleasePreservesSuccessor(t *testing.T) {
	first := SetRuntimeReplayFence(func(string) bool { return false })
	second := SetRuntimeReplayFence(func(name string) bool { return name == "current-external" })
	t.Cleanup(second)
	first()
	require.True(t, FencesLiveForwards("current-external"), "old server release cannot erase its successor")
	require.False(t, FencesLiveForwards("not-declared"))
	second()
	require.False(t, FencesLiveForwards("current-external"), "released external declaration must not survive")
}
