package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetaTreeSetGet verifies storing and retrieving metadata entries.
//
// VALIDATES: MetaTree stores/retrieves entries by YANG path segments.
//
// PREVENTS: Lost metadata for config values.
func TestMetaTreeSetGet(t *testing.T) {
	mt := NewMetaTree()

	entry := MetaEntry{
		User:    "thomas@local",
		Time:    time.Date(2026, 3, 12, 14, 30, 1, 0, time.UTC),
		Session: "thomas@local:1741783801",
	}

	mt.SetEntry("router-id", entry)

	got, ok := mt.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "thomas@local", got.User)
	assert.Equal(t, "thomas@local:1741783801", got.Session)
}

// TestMetaTreeNestedPath verifies metadata for nested paths.
//
// VALIDATES: MetaTree navigates containers and lists like Tree.
//
// PREVENTS: Lost metadata for nested config values.
func TestMetaTreeNestedPath(t *testing.T) {
	mt := NewMetaTree()

	entry := MetaEntry{
		User:    "alice@ssh",
		Time:    time.Date(2026, 3, 12, 14, 31, 0, 0, time.UTC),
		Session: "alice@ssh:1741783860",
	}

	// Set metadata for a nested path: neighbor -> 192.0.2.1 -> hold-time
	child := mt.GetOrCreateContainer("neighbor")
	listChild := child.GetOrCreateListEntry("192.0.2.1")
	listChild.SetEntry("hold-time", entry)

	// Retrieve it
	got, ok := listChild.GetEntry("hold-time")
	require.True(t, ok)
	assert.Equal(t, "alice@ssh", got.User)
}

// TestMetaTreeSessionFilter verifies filtering entries by session ID.
//
// VALIDATES: SessionEntries returns only entries matching a session.
//
// PREVENTS: Commit applying wrong session's changes.
func TestMetaTreeSessionFilter(t *testing.T) {
	mt := NewMetaTree()

	thomas := MetaEntry{
		User:    "thomas@local",
		Session: "thomas@local:1741783801",
	}
	alice := MetaEntry{
		User:    "alice@ssh",
		Session: "alice@ssh:1741783860",
	}

	mt.SetEntry("router-id", thomas)
	mt.SetEntry("local-as", alice)

	thomasEntries := mt.SessionEntries("thomas@local:1741783801")
	assert.Len(t, thomasEntries, 1)
	assert.Equal(t, "router-id", thomasEntries[0].Path)

	aliceEntries := mt.SessionEntries("alice@ssh:1741783860")
	assert.Len(t, aliceEntries, 1)
	assert.Equal(t, "local-as", aliceEntries[0].Path)
}

// TestMetaTreeRemoveSession verifies removing all entries for a session.
//
// VALIDATES: RemoveSession clears only the target session's entries.
//
// PREVENTS: Other sessions' changes being lost on discard.
func TestMetaTreeRemoveSession(t *testing.T) {
	mt := NewMetaTree()

	thomas := MetaEntry{
		User:    "thomas@local",
		Session: "thomas@local:1741783801",
	}
	alice := MetaEntry{
		User:    "alice@ssh",
		Session: "alice@ssh:1741783860",
	}

	mt.SetEntry("router-id", thomas)
	mt.SetEntry("local-as", alice)

	mt.RemoveSession("thomas@local:1741783801")

	// Thomas's entry should be gone
	_, ok := mt.GetEntry("router-id")
	assert.False(t, ok)

	// Alice's entry should remain
	got, ok := mt.GetEntry("local-as")
	assert.True(t, ok)
	assert.Equal(t, "alice@ssh", got.User)
}

// TestMetaTreeAllSessions verifies listing all unique session IDs.
//
// VALIDATES: AllSessions returns deduplicated session IDs.
//
// PREVENTS: Missing sessions in who/show changes.
func TestMetaTreeAllSessions(t *testing.T) {
	mt := NewMetaTree()

	mt.SetEntry("router-id", MetaEntry{Session: "thomas@local:1741783801"})
	mt.SetEntry("local-as", MetaEntry{Session: "thomas@local:1741783801"})
	mt.SetEntry("hold-time", MetaEntry{Session: "alice@ssh:1741783860"})

	sessions := mt.AllSessions()
	assert.Len(t, sessions, 2)
	assert.Contains(t, sessions, "thomas@local:1741783801")
	assert.Contains(t, sessions, "alice@ssh:1741783860")
}

// TestMetaTreeHasSession verifies checking if any entries exist for a session.
//
// VALIDATES: HasSession returns true only when entries exist.
//
// PREVENTS: Draft not cleaned up when all sessions done.
func TestMetaTreeHasSession(t *testing.T) {
	mt := NewMetaTree()
	mt.SetEntry("router-id", MetaEntry{Session: "thomas@local:1741783801"})

	assert.True(t, mt.HasSession("thomas@local:1741783801"))
	assert.False(t, mt.HasSession("alice@ssh:1741783860"))

	mt.RemoveSession("thomas@local:1741783801")
	assert.False(t, mt.HasSession("thomas@local:1741783801"))
}

// TestMetaTreeEmpty verifies empty MetaTree behavior.
//
// VALIDATES: Empty MetaTree returns no entries and no sessions.
//
// PREVENTS: Nil pointer panics on empty MetaTree.
func TestMetaTreeEmpty(t *testing.T) {
	mt := NewMetaTree()

	_, ok := mt.GetEntry("anything")
	assert.False(t, ok)

	assert.Empty(t, mt.AllSessions())
	assert.Empty(t, mt.SessionEntries("nobody"))
	assert.False(t, mt.HasSession("nobody"))
}

// TestMetaTreeContestedLeaf verifies multiple sessions editing the same leaf.
//
// VALIDATES: SetEntry from different sessions preserves both entries.
// GetAllEntries returns all entries; GetEntry returns the last.
//
// PREVENTS: Overwritten session entries (only same-session should replace).
func TestMetaTreeContestedLeaf(t *testing.T) {
	mt := NewMetaTree()

	alice := MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "10.0.0.1",
	}
	bob := MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "1.2.3.4",
	}

	mt.SetEntry("router-id", alice)
	mt.SetEntry("router-id", bob)

	// GetAllEntries should return both
	all := mt.GetAllEntries("router-id")
	require.Len(t, all, 2)
	assert.Equal(t, "alice", all[0].User)
	assert.Equal(t, "bob", all[1].User)

	// GetEntry returns last
	got, ok := mt.GetEntry("router-id")
	require.True(t, ok)
	assert.Equal(t, "bob", got.User)
}

// TestMetaTreeSetEntrySameSessionReplaces verifies that SetEntry replaces
// entries from the same session rather than appending duplicates.
//
// VALIDATES: Same-session SetEntry replaces, not appends.
//
// PREVENTS: Duplicate session entries accumulating on repeated edits.
func TestMetaTreeSetEntrySameSessionReplaces(t *testing.T) {
	mt := NewMetaTree()

	mt.SetEntry("router-id", MetaEntry{
		Session: "alice:100",
		Value:   "first",
	})
	mt.SetEntry("router-id", MetaEntry{
		Session: "alice:100",
		Value:   "second",
	})

	all := mt.GetAllEntries("router-id")
	require.Len(t, all, 1, "same-session should replace, not append")
	assert.Equal(t, "second", all[0].Value)
}

// TestMetaTreeRemoveSessionEntryContested verifies removing one session
// from a contested leaf preserves the other session's entry.
//
// VALIDATES: RemoveSessionEntry removes only target session entries.
//
// PREVENTS: Other session's metadata lost when one session discards.
func TestMetaTreeRemoveSessionEntryContested(t *testing.T) {
	mt := NewMetaTree()

	mt.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
		Value:   "10.0.0.1",
	})
	mt.SetEntry("router-id", MetaEntry{
		User:    "bob",
		Session: "bob:200",
		Value:   "1.2.3.4",
	})

	// Remove alice's entry only
	mt.RemoveSessionEntry("router-id", "alice:100")

	all := mt.GetAllEntries("router-id")
	require.Len(t, all, 1, "only bob's entry should remain")
	assert.Equal(t, "bob", all[0].User)
}

// TestMetaTreeRemoveSessionEntryNonExistent verifies removing a session
// that has no entries at a given leaf is a no-op.
//
// VALIDATES: RemoveSessionEntry is safe for non-existent session/leaf.
//
// PREVENTS: Panic or corruption on remove of missing entry.
func TestMetaTreeRemoveSessionEntryNonExistent(t *testing.T) {
	mt := NewMetaTree()

	mt.SetEntry("router-id", MetaEntry{
		User:    "alice",
		Session: "alice:100",
	})

	// Remove non-existent session -- should be a no-op
	mt.RemoveSessionEntry("router-id", "bob:200")
	all := mt.GetAllEntries("router-id")
	assert.Len(t, all, 1)

	// Remove non-existent leaf -- should be a no-op
	mt.RemoveSessionEntry("missing-leaf", "alice:100")
	all = mt.GetAllEntries("router-id")
	assert.Len(t, all, 1, "unrelated leaf removal should not affect existing entries")
}

// TestMetaTreeSessionEntriesNested verifies SessionEntries collects entries
// across nested containers and lists with correct YANG paths.
//
// VALIDATES: Recursive session collection builds correct space-separated paths.
//
// PREVENTS: Missing session entries in nested config structures.
func TestMetaTreeSessionEntriesNested(t *testing.T) {
	mt := NewMetaTree()

	sessionID := "alice:100"

	// Top-level entry
	mt.SetEntry("router-id", MetaEntry{Session: sessionID, Value: "1.2.3.4"})

	// Nested in container
	neighbor := mt.GetOrCreateContainer("neighbor")
	peer := neighbor.GetOrCreateListEntry("192.0.2.1")
	peer.SetEntry("peer-as", MetaEntry{Session: sessionID, Value: "65001"})

	// Different session at a different leaf (should not be collected)
	peer.SetEntry("local-as", MetaEntry{Session: "bob:200", Value: "65000"})

	entries := mt.SessionEntries(sessionID)
	require.Len(t, entries, 2)

	// Build map of path -> value for easy assertion
	pathValues := make(map[string]string, len(entries))
	for _, e := range entries {
		pathValues[e.Path] = e.Entry.Value
	}

	assert.Equal(t, "1.2.3.4", pathValues["router-id"])
	assert.Equal(t, "65001", pathValues["neighbor 192.0.2.1 peer-as"])
}

// TestMetaTreeHasSessionNested verifies HasSession finds entries
// deep in the nested MetaTree structure.
//
// VALIDATES: HasSession traverses containers and lists recursively.
//
// PREVENTS: False negative when session entry is in a deeply nested subtree.
func TestMetaTreeHasSessionNested(t *testing.T) {
	mt := NewMetaTree()

	// No top-level entries for the session -- only nested
	child := mt.GetOrCreateContainer("neighbor")
	entry := child.GetOrCreateListEntry("192.0.2.1")
	entry.SetEntry("peer-as", MetaEntry{Session: "alice:100"})

	assert.True(t, mt.HasSession("alice:100"), "should find session in nested structure")
	assert.False(t, mt.HasSession("bob:200"), "should not find absent session")
}

// TestMetaTreeAllSessionsNested verifies AllSessions collects from nested structures.
//
// VALIDATES: AllSessions traverses containers and lists recursively.
//
// PREVENTS: Missing sessions that exist only in nested subtrees.
func TestMetaTreeAllSessionsNested(t *testing.T) {
	mt := NewMetaTree()

	// Session at top level
	mt.SetEntry("router-id", MetaEntry{Session: "alice:100"})

	// Different session only at nested level
	child := mt.GetOrCreateContainer("neighbor")
	entry := child.GetOrCreateListEntry("192.0.2.1")
	entry.SetEntry("peer-as", MetaEntry{Session: "bob:200"})

	sessions := mt.AllSessions()
	assert.Len(t, sessions, 2)
	assert.Contains(t, sessions, "alice:100")
	assert.Contains(t, sessions, "bob:200")
}

// TestMetaTreeGetContainerNil verifies read-only container navigation returns nil for missing.
//
// VALIDATES: GetContainer returns nil without creating the container.
//
// PREVENTS: Read-only navigation accidentally creating MetaTree nodes.
func TestMetaTreeGetContainerNil(t *testing.T) {
	mt := NewMetaTree()

	got := mt.GetContainer("missing")
	assert.Nil(t, got)

	// Verify the container was NOT created by the read
	assert.Empty(t, mt.Containers())
}

// TestMetaTreeGetListEntryNil verifies read-only list entry navigation returns nil for missing.
//
// VALIDATES: GetListEntry returns nil without creating the entry.
//
// PREVENTS: Read-only navigation accidentally creating MetaTree nodes.
func TestMetaTreeGetListEntryNil(t *testing.T) {
	mt := NewMetaTree()

	got := mt.GetListEntry("missing-key")
	assert.Nil(t, got)

	// Verify the entry was NOT created by the read
	assert.Empty(t, mt.Lists())
}

// TestMetaTreeRemoveSessionNested verifies RemoveSession clears entries
// across nested containers and lists.
//
// VALIDATES: RemoveSession is recursive across all MetaTree levels.
//
// PREVENTS: Orphan session entries surviving in nested subtrees after removal.
func TestMetaTreeRemoveSessionNested(t *testing.T) {
	mt := NewMetaTree()

	sessionID := "alice:100"
	mt.SetEntry("router-id", MetaEntry{Session: sessionID})
	child := mt.GetOrCreateContainer("neighbor")
	entry := child.GetOrCreateListEntry("192.0.2.1")
	entry.SetEntry("peer-as", MetaEntry{Session: sessionID})
	entry.SetEntry("local-as", MetaEntry{Session: "bob:200"})

	mt.RemoveSession(sessionID)

	// Alice's entries should be gone everywhere
	assert.False(t, mt.HasSession(sessionID))
	_, ok := mt.GetEntry("router-id")
	assert.False(t, ok)

	// Bob's entry should survive
	assert.True(t, mt.HasSession("bob:200"))
	got, ok := entry.GetEntry("local-as")
	assert.True(t, ok)
	assert.Equal(t, "bob:200", got.Session)
}
