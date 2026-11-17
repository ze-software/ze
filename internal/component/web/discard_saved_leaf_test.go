// Design: docs/guide/config-editor.md -- discard all drops pending changes.
// Related: editor.go -- explicit discard ends the cached web editor.

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
)

// TestWebDiscardAllDropsSavedLeafAfterReopen saves a change to a leaf that
// already exists in committed configuration, then discards and reopens. The
// candidate and shared materialization must lose that change, while another
// account's saved metadata and unsaved file remain intact and nothing commits.
func TestWebDiscardAllDropsSavedLeafAfterReopen(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	// This probe uses the private candidate constructor explicitly. The
	// promoting helper's separate validation fixture uses a shared-view editor.
	mgr.editorFactory = testEditorFactory()
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "group", "saved-foreign"}, "description", "bob-saved"))
	bob, err := mgr.GetOrCreate("bob")
	require.NoError(t, err)
	bobSession := bob.editor.SessionID()
	require.NoError(t, mgr.SaveDraft("bob"))
	require.NoError(t, mgr.SetValue("bob", []string{"bgp", "group", "unsaved-foreign"}, "description", "bob-unsaved"))
	foreignPath := cli.ChangePath(mgr.configPath, "bob")
	foreignBefore, err := mgr.store.ReadFile(foreignPath)
	require.NoError(t, err)

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.99"))
	require.NoError(t, mgr.SaveDraft("alice"))
	require.Equal(t, 1, mgr.ChangeCount("alice"), "saved intent remains pending before discard")
	beforeTree := mgr.Tree("alice")
	require.NotNil(t, beforeTree)
	beforeBGP := beforeTree.GetContainer("bgp")
	require.NotNil(t, beforeBGP)
	beforeID, found := beforeBGP.Get("router-id")
	require.True(t, found)
	require.Equal(t, "10.0.0.99", beforeID)
	committedBefore, err := mgr.committedConfig()
	require.NoError(t, err)
	draftBefore, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	_, metaBefore, err := config.NewSetParser(schema).ParseWithMeta(string(draftBefore))
	require.NoError(t, err)
	foreignSaved := metaBefore.SessionEntries(schema, bobSession)
	require.Len(t, foreignSaved, 1)

	require.NoError(t, mgr.Discard("alice"))
	mgr.mu.RLock()
	_, cached := mgr.sessions["alice"]
	mgr.mu.RUnlock()
	require.False(t, cached, "the explicit discard must end the old editor")
	_, err = mgr.GetOrCreate("alice")
	require.NoError(t, err)
	assert.Zero(t, mgr.ChangeCount("alice"), "reopening must not recover discarded saved ownership")
	afterTree := mgr.Tree("alice")
	require.NotNil(t, afterTree)
	afterBGP := afterTree.GetContainer("bgp")
	require.NotNil(t, afterBGP)
	afterID, found := afterBGP.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "10.0.0.250", afterID, "the candidate returns to its committed leaf")
	committedAfter, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committedBefore, committedAfter, "discard applies no configuration")
	foreignAfter, err := mgr.store.ReadFile(foreignPath)
	require.NoError(t, err)
	assert.Equal(t, foreignBefore, foreignAfter, "the other account's unsaved intent stays byte-identical")
	draftAfter, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	draftTree, metaAfter, err := config.NewSetParser(schema).ParseWithMeta(string(draftAfter))
	require.NoError(t, err)
	assert.ElementsMatch(t, foreignSaved, metaAfter.SessionEntries(schema, bobSession), "the other account's saved ownership survives")
	for _, session := range metaAfter.AllSessions() {
		for _, entry := range metaAfter.SessionEntries(schema, session) {
			assert.NotEqual(t, "alice", entry.Entry.User, "the saved draft must not retain discarded ownership")
		}
	}
	draftBGP := draftTree.GetContainer("bgp")
	require.NotNil(t, draftBGP)
	draftID, found := draftBGP.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "10.0.0.250", draftID, "unowned discarded values must not survive in the shared materialization")
}

// TestWebDiscardSavedLeafPreservesContestedForeignValue saves two owners at
// the same leaf. Discard restores the committed private candidate and keeps
// the other account's saved value and exact metadata in the shared draft.
func TestWebDiscardSavedLeafPreservesContestedForeignValue(t *testing.T) {
	mgr, schema := newPromotingEditorManagerOver(t, windowCommittedConfig)
	mgr.editorFactory = testEditorFactory()
	require.NoError(t, mgr.SetValue("bob", []string{"bgp"}, "router-id", "10.0.0.88"))
	bob, err := mgr.GetOrCreate("bob")
	require.NoError(t, err)
	bobSession := bob.editor.SessionID()
	require.NoError(t, mgr.SaveDraft("bob"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.99"))
	require.NoError(t, mgr.SaveDraft("alice"))
	committedBefore, err := mgr.committedConfig()
	require.NoError(t, err)
	draftBefore, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	_, metaBefore, err := config.NewSetParser(schema).ParseWithMeta(string(draftBefore))
	require.NoError(t, err)
	foreignSaved := metaBefore.SessionEntries(schema, bobSession)
	require.Len(t, foreignSaved, 1)
	require.NoError(t, mgr.Discard("alice"))
	_, err = mgr.GetOrCreate("alice")
	require.NoError(t, err)
	assert.Zero(t, mgr.ChangeCount("alice"))
	candidate := mgr.Tree("alice")
	require.NotNil(t, candidate)
	bgp := candidate.GetContainer("bgp")
	require.NotNil(t, bgp)
	candidateID, found := bgp.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "10.0.0.250", candidateID)
	draftAfter, err := mgr.store.ReadFile(cli.DraftPath(mgr.configPath))
	require.NoError(t, err)
	shared, metaAfter, err := config.NewSetParser(schema).ParseWithMeta(string(draftAfter))
	require.NoError(t, err)
	assert.ElementsMatch(t, foreignSaved, metaAfter.SessionEntries(schema, bobSession))
	sharedBGP := shared.GetContainer("bgp")
	require.NotNil(t, sharedBGP)
	sharedID, found := sharedBGP.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "10.0.0.88", sharedID, "surviving pending ownership overrides only the shared materialization")
	committedAfter, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committedBefore, committedAfter)
}

// TestWebDiscardSavedPathKeepsOtherSavedLeaf discards one saved path, then
// reopens the candidate and checks that its other saved change still resumes.
func TestWebDiscardSavedPathKeepsOtherSavedLeaf(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, windowCommittedConfig)
	mgr.editorFactory = testEditorFactory()
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.99"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "group", "kept"}, "description", "saved-outside-path"))
	require.NoError(t, mgr.SaveDraft("alice"))
	require.NoError(t, mgr.DiscardPath("alice", []string{"bgp", "router-id"}))
	mgr.mu.Lock()
	delete(mgr.sessions, "alice")
	mgr.mu.Unlock()
	_, err := mgr.GetOrCreate("alice")
	require.NoError(t, err)
	assert.Equal(t, 1, mgr.ChangeCount("alice"))
	candidate := mgr.Tree("alice")
	require.NotNil(t, candidate)
	bgp := candidate.GetContainer("bgp")
	require.NotNil(t, bgp)
	value, found := bgp.Get("router-id")
	require.True(t, found)
	assert.Equal(t, "10.0.0.250", value)
	kept := bgp.GetList("group")["kept"]
	require.NotNil(t, kept)
	description, found := kept.Get("description")
	require.True(t, found)
	assert.Equal(t, "saved-outside-path", description)
}
