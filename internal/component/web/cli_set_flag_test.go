// Design: docs/guide/config-editor.md -- browser flags share SSH editor semantics.
package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebTerminalSetPresenceFlagCommits(t *testing.T) {
	const baseline = `bgp { router-id 10.0.0.9; session { asn local 65000; } }
firewall { backend nft; table flags { family inet; chain input {
    type filter; hook input; priority 0; policy drop;
    term allow { from { protocol tcp; destination-port 179; } }
} } }`
	mgr, schema := newPromotingEditorManagerOver(t, baseline)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	before, err := mgr.committedConfig()
	require.NoError(t, err)
	answer := terminalAsUser(t, mgr, "testuser", nil, "set firewall table flags chain input term allow then accept")
	assert.Contains(t, answer.Output, "set firewall table flags chain input term allow then accept")
	parent := []string{"firewall", "table", "flags", "chain", "input", "term", "allow", "then"}
	target := walkTree(mgr.Tree("testuser"), schema, parent)
	require.NotNil(t, target)
	value, found := target.Get("accept")
	assert.True(t, found)
	assert.Equal(t, "true", value)
	answer = terminalAsUser(t, mgr, "testuser", nil, "save")
	assert.NotContains(t, answer.Output, "error:")
	unchanged, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, before, unchanged)
	answer = terminalAsUser(t, mgr, "testuser", nil, "commit now")
	require.Contains(t, answer.Output, "commit successful")
	confirmed := terminalAsUser(t, mgr, "testuser", nil, "show confirmed firewall table flags chain input term allow then")
	assert.Contains(t, confirmed.Output, "accept")
	assert.Contains(t, terminalAsUser(t, mgr, "testuser", nil, "show confirmed bgp").Output, "10.0.0.9")
}

func TestWebTerminalSetEmptyFlagRefusesAnExtraValue(t *testing.T) {
	mgr, _ := newPromotingEditorManagerOver(t, `bgp { router-id 10.0.0.9; session { asn local 65000; } }`)
	_, err := mgr.GetOrCreate("testuser")
	require.NoError(t, err)
	answer := terminalAsUser(t, mgr, "testuser", nil, "set system conntrack accounting")
	assert.Contains(t, answer.Output, "set system conntrack accounting")
	system := mgr.Tree("testuser").GetContainer("system")
	require.NotNil(t, system)
	target := system.GetContainer("conntrack")
	require.NotNil(t, target)
	value, found := target.Get("accounting")
	assert.True(t, found)
	assert.Equal(t, "true", value)
	before := mgr.ContentAtPath("testuser", nil)
	answer = terminalAsUser(t, mgr, "testuser", nil, "set system conntrack timestamp true")
	assert.Contains(t, answer.Output, "error:")
	assert.Equal(t, before, mgr.ContentAtPath("testuser", nil))
}
