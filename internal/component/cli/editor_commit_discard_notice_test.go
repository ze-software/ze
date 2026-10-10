package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDiscardNoticeReachesEverySessionOfTheUser is review round 2 ISSUE 4
// (NOTE 10 ruling) of spec-session-editor-file-mode-parity: the notice was one
// file per user, taken by whichever session read it first, so the user's
// other session was never told and kept showing the discarded value.
//
// GOAL: every session of the overridden user is told once, and each one's
// view drops the discarded value.
// METHOD: alice has two sessions sharing her change file; bob forces over
// her router-id; each alice session runs its draft poll (checkDraftChanged)
// twice.
//
// VALIDATES: both sessions report the discard on their first poll, neither
// on its second, and neither still shows 10.0.0.1.
// PREVENTS: the second session silently showing a value already discarded.
func TestDiscardNoticeReachesEverySessionOfTheUser(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := newTestTreeStore(t, configPath)

	first, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer first.Close() //nolint:errcheck,gosec // test cleanup
	first.SetSession(NewEditSession("alice", "ssh"))
	require.NoError(t, first.SetValue([]string{"bgp"}, "router-id", "10.0.0.1"))

	second, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer second.Close() //nolint:errcheck,gosec // test cleanup
	second.SetSession(NewEditSession("alice", "web"))

	bob, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer bob.Close() //nolint:errcheck,gosec // test cleanup
	bob.SetSession(NewEditSession("bob", "ssh"))
	require.NoError(t, bob.SetValue([]string{"bgp"}, "router-id", "10.0.0.2"))
	forced, err := bob.CommitSessionForce()
	require.NoError(t, err)
	require.Equal(t, 1, forced.Applied)

	for name, ed := range map[string]*Editor{"first": first, "second": second} {
		changed, notice := ed.checkDraftChanged()
		assert.True(t, changed, name)
		assert.Contains(t, notice, "Your change at bgp router-id was discarded by bob", name)
		assert.NotContains(t, ed.WorkingContent(), "10.0.0.1", name+": the view dropped the discarded value")
		_, again := ed.checkDraftChanged()
		assert.NotContains(t, again, "discarded", name+": the notice is shown once per session")
	}
}
