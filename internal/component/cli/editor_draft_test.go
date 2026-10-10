package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// pendingKinds answers each pending change of the session as "kind path".
func pendingKinds(ed *Editor, sessionID string) []string {
	changes := ed.PendingChanges(sessionID)
	out := make([]string, 0, len(changes))
	for _, change := range changes {
		out = append(out, string(change.Kind)+" "+change.Path)
	}
	return out
}

// TestSessionCopyWritesThrough verifies AC-8 at the editor: a session copy is
// one copy-entry op in the change file, one pending copy change, the copy in
// the in-memory tree, and both entries in the committed config. An existing
// destination is refused and writes nothing (AC-30).
func TestSessionCopyWritesThrough(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)

	require.NoError(t, ed.CopyListEntry([]string{"bgp"}, "peer", "peer1", "peer2"))

	changeData, err := store.ReadFile(ChangePath(configPath, session.User))
	require.NoError(t, err)
	_, _, ops, err := config.ParseChangeFile(string(changeData), config.NewSetParser(ed.schema))
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, config.StructuralOpCopyEntry, ops[0].Type)
	assert.Equal(t, "thomas", ops[0].User)
	assert.Equal(t, []string{"copy bgp peer peer2"}, pendingKinds(ed, session.ID))
	require.NotNil(t, ed.tree.GetContainer("bgp").GetList("peer")["peer2"])

	err = ed.CopyListEntry([]string{"bgp"}, "peer", "peer1", "peer2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	after, err := store.ReadFile(ChangePath(configPath, session.User))
	require.NoError(t, err)
	assert.Equal(t, string(changeData), string(after), "a refused copy writes nothing")

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	committed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer committed.Close() //nolint:errcheck,gosec // Best effort cleanup
	peers := committed.tree.GetContainer("bgp").GetList("peer")
	require.NotNil(t, peers["peer1"])
	require.NotNil(t, peers["peer2"])
	ip, ok := peers["peer2"].GetContainer("connection").GetContainer("remote").Get("ip")
	assert.True(t, ok)
	assert.Equal(t, "1.1.1.1", ip)
}

// TestSessionCopyCarriesPendingSourceEdits verifies that a session copy of an
// entry this session has already edited commits the edits on the copy too. The
// commit applies the copy-entry op to the committed source before the leaf
// edits, so the edits reach the copy only when the change file carries them
// under the destination. The set replaces a committed value, so the copy's
// entry must not read as stale against a destination the committed file never
// held, and the delete must reach the copy as a delete.
func TestSessionCopyCarriesPendingSourceEdits(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)

	require.NoError(t, ed.SetValue([]string{"bgp", "peer", "peer1", "timer"}, "receive-hold-time", "180"))
	require.NoError(t, ed.DeleteValue([]string{"bgp", "peer", "peer1", "session", "asn"}, "remote"))
	require.NoError(t, ed.CopyListEntry([]string{"bgp"}, "peer", "peer1", "peer2"))
	assert.Contains(t, pendingKinds(ed, session.ID), "copy bgp peer peer2")

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	committed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer committed.Close() //nolint:errcheck,gosec // Best effort cleanup
	peers := committed.tree.GetContainer("bgp").GetList("peer")
	for _, key := range []string{"peer1", "peer2"} {
		require.NotNil(t, peers[key], key)
		peer := []string{"bgp", "peer", key}
		assert.Equal(t, "180", getValueAtPath(committed.tree, committed.schema, append(peer, "timer", "receive-hold-time")), key)
		assert.Empty(t, getValueAtPath(committed.tree, committed.schema, append(peer, "session", "asn", "remote")),
			"%s: the pending delete reaches the copy", key)
	}
}

// TestSessionDeactivateActivateLeafAndPath verifies AC-9 and AC-10 at the
// editor: leaf and path deactivation are structural ops shown as one pending
// change each and committed as inactive markers; a later session activates
// them back; activating an active node keeps the existing sentinel error.
func TestSessionDeactivateActivateLeafAndPath(t *testing.T) {
	configPath := writeTestConfig(t, validBGPConfig)
	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer ed.Close() //nolint:errcheck,gosec // Best effort cleanup
	session := NewEditSession("thomas", "ssh")
	ed.SetSession(session)

	peer := []string{"bgp", "peer", "peer1"}
	require.ErrorIs(t, ed.ActivatePath(peer), ErrPathNotInactive)
	require.ErrorIs(t, ed.ActivateLeaf([]string{"bgp"}, "router-id"), ErrLeafNotInactive)

	require.NoError(t, ed.DeactivateLeaf([]string{"bgp"}, "router-id"))
	require.NoError(t, ed.DeactivatePath(peer))
	require.ErrorIs(t, ed.DeactivatePath(peer), ErrPathAlreadyInactive)
	assert.ElementsMatch(t,
		[]string{"deactivate bgp router-id", "deactivate bgp peer peer1"},
		pendingKinds(ed, session.ID))
	assert.True(t, ed.tree.GetContainer("bgp").IsLeafInactive("router-id"))
	assert.True(t, ed.WalkPath(peer).IsInactive())

	result, err := ed.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	second, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer second.Close() //nolint:errcheck,gosec // Best effort cleanup
	require.True(t, second.tree.GetContainer("bgp").IsLeafInactive("router-id"), "committed leaf is inactive")
	require.True(t, second.WalkPath(peer).IsInactive(), "committed path is inactive")

	other := NewEditSession("thomas", "ssh")
	second.SetSession(other)
	require.NoError(t, second.ActivateLeaf([]string{"bgp"}, "router-id"))
	require.NoError(t, second.ActivatePath(peer))
	assert.ElementsMatch(t,
		[]string{"activate bgp router-id", "activate bgp peer peer1"},
		pendingKinds(second, other.ID))
	result, err = second.CommitSession()
	require.NoError(t, err)
	require.Empty(t, result.Conflicts)

	third, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	defer third.Close() //nolint:errcheck,gosec // Best effort cleanup
	assert.False(t, third.tree.GetContainer("bgp").IsLeafInactive("router-id"))
	assert.False(t, third.WalkPath(peer).IsInactive())
}
