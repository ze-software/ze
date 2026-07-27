package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/config"
)

// fileModeEditorAdapter forces the file-mode (non write-through) editor path by
// dropping the session EditorManager.GetOrCreate assigns unconditionally
// (editor.go, GetOrCreate -> ed.SetSession). Without this seam the manager can
// only ever be exercised in zefs session mode, so the file-mode branch of
// Editor.DeleteListEntry would never be reached from the web entry point.
type fileModeEditorAdapter struct {
	contract.Editor
}

func (fileModeEditorAdapter) SetSession(contract.EditSession) {}

// newFileModeTestManager builds an EditorManager whose editors never enter
// session mode, so deletes take the Editor's direct-tree path.
func newFileModeTestManager(t *testing.T) *EditorManager {
	t.Helper()

	mgr, _ := newHandlerTestManager(t)
	inner := testEditorFactory()
	mgr.editorFactory = func(store any, configPath string) (contract.Editor, error) {
		ed, err := inner(store, configPath)
		if err != nil {
			return nil, err
		}
		return &fileModeEditorAdapter{Editor: ed}, nil
	}
	return mgr
}

// peerEntries returns the bgp/peer list of the user's working tree.
func peerEntries(t *testing.T, mgr *EditorManager, username string) map[string]*config.Tree { //nolint:unparam // username is explicit for test readability, matching postConfigRequest
	t.Helper()

	tree := mgr.Tree(username)
	require.NotNil(t, tree, "user working tree must exist")
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp, "bgp container must exist")
	return bgp.GetList("peer")
}

// TestHandleConfigDeleteRemovesListEntrySession verifies that the web delete
// endpoint removes a BGP peer (a YANG list entry), not just a leaf, and that in
// zefs session mode the removal is recorded in the per-user change file.
//
// VALIDATES: POST /config/delete/bgp/peer/ with leaf=<peer-key> removes the
// entry from the working tree and writes a delete-entry structural op to the
// change file (the session write-through added by 04ef5f079).
// PREVENTS: the web delete button silently doing nothing because the handler
// called DeleteValue, whose Tree.Delete only touches t.values/t.multiValues and
// early-returns for anything living in t.lists (setparser.go, Tree.Delete).
func TestHandleConfigDeleteRemovesListEntrySession(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "peer", "london"}))
	require.NotNil(t, peerEntries(t, mgr, "alice")["london"], "precondition: peer exists")

	handler := handleConfigDelete(mgr)
	req := postConfigRequest(t, "/config/delete/bgp/peer/", url.Values{"leaf": {"london"}}, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code,
		"successful delete must redirect with 303, body: %s", rec.Body.String())
	assert.Nil(t, peerEntries(t, mgr, "alice")["london"],
		"list entry must be gone from the working tree after delete")

	changeFile, err := os.ReadFile(cli.ChangePath(mgr.configPath, "alice"))
	require.NoError(t, err, "session mode must have written a per-user change file")
	assert.Contains(t, string(changeFile), "delete-entry bgp peer london",
		"session-mode delete must reach the change file as a structural op")
}

// TestHandleConfigDeleteRemovesListEntryFileMode is the file-mode half: the same
// endpoint against an editor with no session must remove the entry from the tree.
//
// VALIDATES: the schema-aware delete reaches Editor.DeleteListEntry's direct
// tree path when no zefs session is active.
// PREVENTS: fixing only the session-mode branch and leaving file-mode deletes inert.
func TestHandleConfigDeleteRemovesListEntryFileMode(t *testing.T) {
	mgr := newFileModeTestManager(t)
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "peer", "london"}))
	require.NotNil(t, peerEntries(t, mgr, "alice")["london"], "precondition: peer exists")

	handler := handleConfigDelete(mgr)
	req := postConfigRequest(t, "/config/delete/bgp/peer/", url.Values{"leaf": {"london"}}, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code,
		"successful delete must redirect with 303, body: %s", rec.Body.String())
	assert.Nil(t, peerEntries(t, mgr, "alice")["london"],
		"list entry must be gone from the working tree after delete")
}

// TestHandleConfigDeleteLeafStillRemovesValue guards the pre-existing leaf
// behavior of the same endpoint against the schema-aware rerouting.
//
// VALIDATES: POST /config/delete/bgp/ with leaf=router-id still clears the leaf.
// PREVENTS: the list-entry fix regressing plain leaf deletes.
func TestHandleConfigDeleteLeafStillRemovesValue(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "9.9.9.9"))

	handler := handleConfigDelete(mgr)
	req := postConfigRequest(t, "/config/delete/bgp/", url.Values{"leaf": {"router-id"}}, "alice")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusSeeOther, rec.Code)
	value, ok := mgr.Tree("alice").GetContainer("bgp").Get("router-id")
	assert.False(t, ok, "router-id must be cleared from the working tree, got %q", value)
}

// TestWebCLIBarDeleteRemovesListEntry covers the sibling call site: the web CLI
// bar's `delete` verb, which shares the manager and previously called DeleteValue.
//
// VALIDATES: `delete london` at context bgp/peer removes the peer entry.
// PREVENTS: fixing the delete button while the CLI bar keeps no-opping on lists.
func TestWebCLIBarDeleteRemovesListEntry(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "peer", "london"}))
	require.NotNil(t, peerEntries(t, mgr, "alice")["london"], "precondition: peer exists")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cli", http.NoBody)
	handleCLIDelete(rec, req, []string{"bgp", "peer"}, []string{"london"}, mgr, "alice")

	assert.Nil(t, peerEntries(t, mgr, "alice")["london"],
		"CLI bar delete must remove the list entry; response: %s", rec.Body.String())
}

// TestWebTerminalDeleteRemovesListEntry covers the second sibling call site: the
// web terminal-mode `delete` command.
//
// VALIDATES: terminal `delete london` at context bgp/peer removes the peer entry.
// PREVENTS: the terminal surface keeping the DeleteValue-only behavior.
func TestWebTerminalDeleteRemovesListEntry(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "peer", "london"}))
	require.NotNil(t, peerEntries(t, mgr, "alice")["london"], "precondition: peer exists")

	out := executeTerminalDelete(mgr, "alice", []string{"bgp", "peer"}, []string{"london"})

	assert.NotContains(t, out, "error:", "terminal delete must succeed, got %q", out)
	assert.Nil(t, peerEntries(t, mgr, "alice")["london"],
		"terminal delete must remove the list entry")
}

// recordingEditor reports a delete failure and records whether the manager fell
// back to the leaf-only DeleteValue after it.
type recordingEditor struct {
	contract.Editor
	deleteValueCalled bool
}

func (*recordingEditor) SetSession(contract.EditSession) {}
func (*recordingEditor) DeleteByPath([]string) error     { return errRecordingDeleteRefused }
func (e *recordingEditor) DeleteValue([]string, string) error {
	e.deleteValueCalled = true
	return nil
}

var errRecordingDeleteRefused = errors.New("schema not available")

// TestEditorManagerDeleteByPathFailsClosed proves the manager surfaces a refused
// delete instead of retrying through the leaf-only path that cannot see lists.
//
// VALIDATES: when the editor refuses (e.g. cli.Editor.DeleteByPath's
// errSchemaNotAvailable guard), the manager returns the error and never calls
// DeleteValue.
// PREVENTS: a silent fallback re-introducing the inert delete
// (ai/rules/fail-closed-guards.md).
func TestEditorManagerDeleteByPathFailsClosed(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	ed := &recordingEditor{}
	mgr.editorFactory = func(_ any, _ string) (contract.Editor, error) { return ed, nil }

	err := mgr.DeleteByPath("alice", []string{"bgp", "peer"}, "london")

	require.ErrorIs(t, err, errRecordingDeleteRefused, "refusal must reach the caller")
	assert.False(t, ed.deleteValueCalled, "manager must not fall back to the leaf-only delete")
}

// TestEditorManagerDeleteByPathDoesNotAliasCallerPath guards the slice handling:
// the manager must not write the leaf into the caller's path backing array.
//
// VALIDATES: DeleteByPath leaves the caller's path slice untouched.
// PREVENTS: a spare-capacity append clobbering the request path the handler
// reuses for its redirect.
func TestEditorManagerDeleteByPathDoesNotAliasCallerPath(t *testing.T) {
	mgr, _ := newHandlerTestManager(t)
	require.NoError(t, mgr.CreateEntry("alice", []string{"bgp", "peer", "london"}))

	path := make([]string, 2, 8) // spare capacity: a naive append would reuse it
	path[0], path[1] = "bgp", "peer"

	require.NoError(t, mgr.DeleteByPath("alice", path, "london"))

	assert.Equal(t, []string{"bgp", "peer"}, path, "caller path must be unchanged")
	assert.Nil(t, peerEntries(t, mgr, "alice")["london"])
}
