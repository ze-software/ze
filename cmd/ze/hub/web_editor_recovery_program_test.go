//go:build ze_web

// Design: docs/architecture/config/change-file-structural-ops.md -- replay order.
// Related: web_editor_recovery_test.go -- production recovery fixture.

package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// TestWebEditorRecoveryReplaysSavedThenUnsavedProgram proves metadata follows
// its saved rename, the unsaved value wins, and a later copy/rename retains its
// snapshot and list position through a fresh production manager and commit.
func TestWebEditorRecoveryReplaysSavedThenUnsavedProgram(t *testing.T) {
	f := newWebEditorRecoveryFixture(t)
	path := []string{"bgp", "group", "alice-renamed"}
	require.NoError(t, f.alice.SetValue(path, "description", "saved-own"))
	require.NoError(t, f.alice.SaveDraft())
	require.NoError(t, f.alice.SetValue(path, "description", "unsaved-own"))
	for _, key := range []string{"zulu", "alpha"} {
		require.NoError(t, f.alice.SetValue([]string{"bgp", "group", key}, "description", key))
	}
	require.NoError(t, f.alice.CopyListEntry([]string{"bgp"}, "group", "alice-renamed", []string{"copied"}))
	require.NoError(t, f.alice.RenameListEntry([]string{"bgp"}, "group", "copied", []string{"final"}))
	want := []string{"alice-renamed", "final", "zulu", "alpha"}
	require.Equal(t, want, recoveryGroupOrder(f.alice.Tree()), "the live CLI candidate establishes the expected order")
	require.Len(t, f.alice.PendingChanges(f.alice.SessionID()), 8, "the live CLI session establishes five assignments and three operations")
	manager := f.manager(t)
	response := f.get(t, manager, "finder", "/show/bgp/group/final/")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "unsaved-own")
	require.NotNil(t, manager.Tree("alice"))
	assert.Equal(t, want, recoveryGroupOrder(manager.Tree("alice")))
	assert.Equal(t, 8, manager.ChangeCount("alice"), "five owned assignments and three address operations")
	diff := manager.Diff("alice")
	assert.Contains(t, diff, "unsaved-own")
	assert.NotContains(t, diff, "description saved-own\n")
	assert.Contains(t, diff, "copy")
	assert.Contains(t, diff, "rename")
	assert.NotContains(t, manager.ContentAtPath("alice", nil), "bob-only")
	assert.NotContains(t, manager.ContentAtPath("alice", nil), "ssh-only")
	f.checkOwners(t)
	form := url.Values{"action": {"now"}, "path": {""}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://ze.test/config/action", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = f.serve(t, manager, "finder", request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "commit successful")
	committed, err := f.store.ReadFile("config.conf")
	require.NoError(t, err)
	tree, _, err := zeconfig.NewSetParser(f.schema).ParseWithMeta(string(committed))
	require.NoError(t, err)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.Equal(t, want, recoveryGroupOrder(tree))
	for _, key := range []string{"alice-renamed", "final"} {
		entry := bgp.GetList("group")[key]
		require.NotNil(t, entry)
		value, exists := entry.Get("description")
		assert.True(t, exists)
		assert.Equal(t, "unsaved-own", value)
	}
	assert.Zero(t, manager.ChangeCount("alice"))
	assert.Empty(t, manager.Diff("alice"))
	f.checkOwners(t)
	change, err := f.store.ReadFile(cli.ChangePath("config.conf", "alice"))
	require.NoError(t, err)
	_, _, ops, err := zeconfig.ParseCandidateChangeFile(string(change), f.schema)
	require.NoError(t, err)
	assert.Empty(t, ops, "every recovered web operation was consumed exactly once")
}

func recoveryGroupOrder(tree *zeconfig.Tree) []string {
	entries := tree.GetContainer("bgp").GetListOrdered("group")
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}
