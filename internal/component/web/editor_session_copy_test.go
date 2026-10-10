package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestWebSessionCopyAfterCommit is AC-11 of spec-session-editor-file-mode-parity
// at the manager the web terminal drives.
//
// GOAL: a list entry the operator committed through the web can be copied in
// the same session straight after the commit.
// METHOD: over a base that holds bgp and over the empty file a daemon creates,
// link a peer validator that accepts, as a daemon with the BGP engine
// does, wire the commit hook the daemon wires (promote the candidate), set a
// peer, commit it, then copy it.
//
// VALIDATES: the copy succeeds and the working tree holds both entries.
// PREVENTS: the web editor losing the committed tree at commit, so the next
// copy answers "path not found" for the entry it just committed.
func TestWebSessionCopyAfterCommit(t *testing.T) {
	for _, tc := range []struct{ name, base string }{
		{name: "base holds bgp", base: commitTestConfig},
		{name: "base is the daemon's empty file", base: "# ze config\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { checkWebSessionCopyAfterCommit(t, tc.base) })
	}
}

func checkWebSessionCopyAfterCommit(t *testing.T, base string) {
	t.Helper()
	installPeerValidator(t, func(*config.Tree) error { return nil })
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(base), 0o600))
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	mgr := NewEditorManager(testConfigStore(t, configPath), configPath, schema,
		validatingEditorFactory(), testEditSessionFactory())
	mgr.SetCommitHook(func() error {
		return storage.PromoteCandidate(mgr.store, mgr.configPath)
	})

	require.NoError(t, mgr.SetValue("alice", []string{"bgp"}, "router-id", "10.0.0.9"))
	require.NoError(t, mgr.SetValue("alice", []string{"bgp", "session", "asn"}, "local", "65000"))
	peer := []string{"bgp", "peer", "wbsrc"}
	require.NoError(t, mgr.SetValue("alice", append(peer, "connection", "remote"), "ip", "10.0.0.1"))
	require.NoError(t, mgr.SetValue("alice", append(peer, "connection", "local"), "ip", "auto"))
	require.NoError(t, mgr.SetValue("alice", append(peer, "session", "asn"), "remote", "65001"))
	result, err := mgr.Commit("alice")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Conflicts)
	require.Positive(t, result.Applied)

	require.NoError(t, mgr.CopyListEntry("alice", []string{"bgp"}, "peer", "wbsrc", "wbdst"))

	bgp := mgr.Tree("alice").GetContainer("bgp")
	require.NotNil(t, bgp)
	peers := bgp.GetList("peer")
	assert.Contains(t, peers, "wbsrc")
	assert.Contains(t, peers, "wbdst")
}

// TestWebTerminalListEntryOpWording pins the answer the web terminal prints for
// copy and rename.
//
// GOAL: the web terminal answers in the words the SSH editor uses.
// METHOD: run execListEntryOp with an op that succeeds, for each verb.
//
// VALIDATES: "Copied peer a to b" and "Renamed peer a to b".
// PREVENTS: the verb and a bare "d" glued together ("copyd").
func TestWebTerminalListEntryOpWording(t *testing.T) {
	ok := func([]string, string, string, string) error { return nil }
	args := []string{"bgp", "peer", "a", "to", "b"}

	_, copied := execListEntryOp(args, nil, listEntryCopy, ok)
	assert.Equal(t, "Copied peer a to b", copied)

	_, renamed := execListEntryOp(args, nil, listEntryRename, ok)
	assert.Equal(t, "Renamed peer a to b", renamed)
}
