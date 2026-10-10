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

// newPromotingEditorManager builds the manager a daemon wires over the empty
// config file a daemon starts on: a peer validator that accepts, and a commit
// hook that promotes the candidate.
func newPromotingEditorManager(t *testing.T) (*EditorManager, *config.Schema) {
	t.Helper()
	installPeerValidator(t, func(*config.Tree) error { return nil })
	configPath := filepath.Join(t.TempDir(), "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("# ze config\n"), 0o600))
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	mgr := NewEditorManager(testConfigStore(t, configPath), configPath, schema,
		validatingEditorFactory(), testEditSessionFactory())
	mgr.SetCommitHook(func() error {
		return storage.PromoteCandidate(mgr.store, mgr.configPath)
	})
	return mgr, schema
}

// TestWebTerminalSetTakesATokenPath is AC-11 of
// spec-session-editor-file-mode-parity through the lines the operator types.
//
// GOAL: the web terminal reads `set <path> <leaf> <value>` the way the SSH
// editor does, so a peer set from the root lands, commits and can be copied.
// METHOD: type each line through executeTerminalNav at the root, commit, then
// copy the committed peer.
//
// VALIDATES: the leaf lands at its full path, the commit applies it, and the
// copy answers "Copied peer wbsrc to wbdst".
// PREVENTS: the terminal taking the first token as the leaf and the rest as its
// value, so `set bgp router-id 10.0.0.9` answered "set bgp ..." while it stored
// nothing a commit could apply, and the next copy answered "path not found".
func TestWebTerminalSetTakesATokenPath(t *testing.T) {
	mgr, schema := newPromotingEditorManager(t)
	for _, line := range [][]string{
		{"bgp", "router-id", "10.0.0.9"},
		{"bgp", "session", "asn", "local", "65000"},
		{"bgp", "peer", "wbsrc", "connection", "remote", "ip", "10.0.0.1"},
		{"bgp", "peer", "wbsrc", "connection", "local", "ip", "auto"},
		{"bgp", "peer", "wbsrc", "session", "asn", "remote", "65001"},
	} {
		_, output := executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verbSet, Args: line})
		require.NotContains(t, output, "error", "set %v", line)
	}
	bgp := mgr.Tree("alice").GetContainer("bgp")
	require.NotNil(t, bgp, "the set lines created no bgp container")
	routerID, ok := bgp.Get("router-id")
	require.True(t, ok, "the leaf lands at its full path")
	assert.Equal(t, "10.0.0.9", routerID)

	_, output := executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verbCommit, Args: []string{"now"}})
	require.Equal(t, terminalOutputCommitSuccessful, output)

	_, output = executeTerminalNav(schema, nil, mgr, "alice", nil,
		cliCommand{Verb: verbCopy, Args: []string{"bgp", "peer", "wbsrc", "to", "wbdst"}})
	assert.Equal(t, "Copied peer wbsrc to wbdst", output)
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

// TestWebTerminalDeactivateLeafAndEntry is AC-11 of
// spec-session-editor-file-mode-parity for deactivate and activate.
//
// GOAL: the web terminal deactivates a leaf and a list entry the way the SSH
// editor does, and answers in its words.
// METHOD: set a router-id and a peer, then deactivate and activate each one
// through executeTerminalNav at the root.
//
// VALIDATES: "Deactivated bgp router-id", "Deactivated bgp peer wbsrc", the
// "Activated" pair, and the leaf marked inactive in the working tree.
// PREVENTS: the terminal sending a leaf path to DeactivatePath, which only
// walks containers and list entries, so it answered "path not found".
func TestWebTerminalDeactivateLeafAndEntry(t *testing.T) {
	mgr, schema := newPromotingEditorManager(t)
	for _, line := range [][]string{
		{"bgp", "router-id", "10.0.0.9"},
		{"bgp", "peer", "wbsrc", "connection", "remote", "ip", "10.0.0.1"},
	} {
		_, output := executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verbSet, Args: line})
		require.NotContains(t, output, "error", "set %v", line)
	}
	run := func(verb string, args ...string) string {
		_, output := executeTerminalNav(schema, nil, mgr, "alice", nil, cliCommand{Verb: verb, Args: args})
		return output
	}

	assert.Equal(t, "Deactivated bgp router-id", run(verbDeactivate, "bgp", "router-id"))
	assert.True(t, mgr.Tree("alice").GetContainer("bgp").IsLeafInactive("router-id"), "the leaf is marked inactive")
	assert.Equal(t, "Deactivated bgp peer wbsrc", run(verbDeactivate, "bgp", "peer", "wbsrc"))
	assert.Equal(t, "Activated bgp router-id", run(verbActivate, "bgp", "router-id"))
	assert.Equal(t, "Activated bgp peer wbsrc", run(verbActivate, "bgp", "peer", "wbsrc"))
	assert.Equal(t, "bgp router-id already active", run(verbActivate, "bgp", "router-id"))
}
