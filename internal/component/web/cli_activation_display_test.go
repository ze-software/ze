// Design: docs/guide/config-editor.md -- browser pending views share activation display.

package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// TestWebTerminalChangesDisplaysInactiveCandidateUntilCommit distinguishes an
// actual inactive candidate from its HTTP-rendered pending and confirmed views.
func TestWebTerminalChangesDisplaysInactiveCandidateUntilCommit(t *testing.T) {
	const baseline = `bgp {
  router-id 10.0.0.9;
  session { asn local 65000; }
  peer view-peer {
    connection { local ip auto; remote ip 192.0.2.1; }
    session { asn remote 65001; router-id 10.0.0.9; }
  }
}`
	mgr, _ := newPromotingEditorManagerOver(t, baseline)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	before, err := mgr.committedConfig()
	require.NoError(t, err)
	for _, line := range []string{"deactivate bgp router-id", "deactivate bgp peer view-peer"} {
		answer := terminalAsUser(t, mgr, "testuser", nil, line)
		require.Contains(t, answer.Output, "Deactivated")
	}
	bgp := mgr.snapshotTree("testuser").GetContainer("bgp")
	require.True(t, bgp.IsLeafInactive("router-id"))
	require.True(t, bgp.GetList("peer")["view-peer"].IsInactive())
	require.Contains(t, config.Serialize(mgr.snapshotTree("testuser"), mgr.schema), "inactive: router-id 10.0.0.9")
	issues := terminalAsUser(t, mgr, "testuser", nil, "errors")
	assert.Equal(t, "No issues", issues.Output)
	changes := terminalAsUser(t, mgr, "testuser", nil, "show | changes")
	assert.Contains(t, changes.Output, "2 pending changes")
	assert.Contains(t, changes.Output, "inactive: router-id 10.0.0.9")
	assert.Contains(t, changes.Output, "inactive: peer view-peer")
	confirmed := terminalAsUser(t, mgr, "testuser", nil, "show confirmed")
	assert.NotContains(t, confirmed.Output, "inactive: router-id")
	assert.NotContains(t, confirmed.Output, "inactive: peer view-peer")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, before, unchanged)
	committed := terminalAsUser(t, mgr, "testuser", nil, "commit now")
	require.Contains(t, committed.Output, "commit successful")
	published, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.NotEqual(t, before, published)
	confirmed = terminalAsUser(t, mgr, "testuser", nil, "show confirmed")
	assert.Contains(t, confirmed.Output, "inactive: router-id 10.0.0.9")
	assert.Contains(t, confirmed.Output, "inactive: peer view-peer")
}
