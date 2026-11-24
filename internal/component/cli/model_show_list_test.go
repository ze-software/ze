// Design: docs/guide/config-editor.md — bare keyed-list show rendering.
// Related: editor.go, editor_mask.go, model_commands_show.go.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

const testBGPWithTwoPeers = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
	peer alpha {
		connection {
			remote {
				ip 192.0.2.1
			}
		}
		session {
			asn {
				local 65000
				remote 65001
			}
		}
	}
	peer beta {
		connection {
			remote {
				ip 192.0.2.2
			}
		}
		session {
			asn {
				local 65000
				remote 65002
			}
		}
	}
}
`

const testBGPWithNoPeers = `bgp {
	router-id 1.2.3.4
	session {
		asn {
			local 65000
		}
	}
}
`

func newBareListShowModel(t *testing.T, content string) *Model {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	store := newTestTreeStore(t, configPath)
	ed, err := NewEditorWithStorage(store, configPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ed.Close()) })
	ed.SetSession(NewEditSession("alice", "ssh"))

	m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	return &m
}

// TestShowBareKeyedListPopulated verifies that showing a bare keyed list path
// renders only that list's entries across candidate, confirmed, saved, and
// set-format views rather than dumping the entire root configuration or empty.
//
// VALIDATES: show bgp peer, show confirmed bgp peer, show saved bgp peer, and
// show bgp peer | format config render only the peer list entries.
// PREVENTS: silent fall-through dumping the full root config on candidate or
// returning empty on confirmed/saved for bare keyed list paths.
func TestShowBareKeyedListPopulated(t *testing.T) {
	m := newBareListShowModel(t, testBGPWithTwoPeers)

	// Candidate view: renders only peer entries, not sibling router-id.
	res, err := m.cmdShow([]string{"bgp", "peer"})
	require.NoError(t, err)
	require.NotNil(t, res.configView, "populated bare list should return configView")
	assert.Contains(t, res.configView.content, "peer alpha")
	assert.Contains(t, res.configView.content, "peer beta")
	assert.Contains(t, res.configView.content, "192.0.2.1")
	assert.Contains(t, res.configView.content, "192.0.2.2")
	assert.NotContains(t, res.configView.content, "router-id 1.2.3.4", "must not dump root config")

	// Confirmed view: renders committed list entries, not empty or root.
	res, err = m.cmdShow([]string{srcConfirmed, "bgp", "peer"})
	require.NoError(t, err)
	require.NotNil(t, res.configView, "show confirmed bgp peer should return configView")
	assert.Contains(t, res.configView.content, "peer alpha")
	assert.Contains(t, res.configView.content, "peer beta")
	assert.NotContains(t, res.configView.content, "router-id 1.2.3.4", "must not dump root config")

	// Saved view: add a third peer, save draft, and inspect saved bare list.
	require.NoError(t, m.editor.SetValue([]string{"bgp", "peer", "gamma", "connection", "remote"}, "ip", "192.0.2.3"))
	require.NoError(t, m.editor.SaveDraft())

	res, err = m.cmdShow([]string{srcSaved, "bgp", "peer"})
	require.NoError(t, err)
	require.NotNil(t, res.configView, "show saved bgp peer should return configView")
	assert.Contains(t, res.configView.content, "peer gamma")
	assert.Contains(t, res.configView.content, "192.0.2.3")
	assert.NotContains(t, res.configView.content, "router-id 1.2.3.4", "must not dump root config")

	// Set format pipe: keeps only bgp peer lines.
	res, err = m.cmdShowPipe([]string{"bgp", "peer"}, []PipeFilter{{Type: cmdFormat, Arg: fmtConfig}})
	require.NoError(t, err)
	assert.Contains(t, res.output, "set bgp peer alpha")
	assert.Contains(t, res.output, "set bgp peer beta")
	assert.NotContains(t, res.output, "router-id")
}

// TestShowBareKeyedListEmpty verifies that showing an empty keyed list returns
// an explicit empty answer instead of silently dumping the root configuration.
//
// VALIDATES: show bgp peer, show confirmed bgp peer, and show saved bgp peer
// return emptyConfiguration when no entries exist, and ContentAtPath/DisplayContentAtPath
// return empty string instead of the root tree.
// PREVENTS: wholeOnMiss fallback dumping the entire root configuration for an
// empty keyed list.
func TestShowBareKeyedListEmpty(t *testing.T) {
	m := newBareListShowModel(t, testBGPWithNoPeers)

	assert.Empty(t, m.editor.ContentAtPath([]string{"bgp", "peer"}))
	assert.Empty(t, m.editor.DisplayContentAtPath([]string{"bgp", "peer"}))

	// Candidate view: explicit empty answer.
	res, err := m.cmdShow([]string{"bgp", "peer"})
	require.NoError(t, err)
	assert.Nil(t, res.configView)
	assert.Equal(t, emptyConfiguration, res.output)
	assert.NotContains(t, res.output, "router-id")

	// Confirmed view: explicit empty answer.
	res, err = m.cmdShow([]string{srcConfirmed, "bgp", "peer"})
	require.NoError(t, err)
	assert.Nil(t, res.configView)
	assert.Equal(t, emptyConfiguration, res.output)
	assert.NotContains(t, res.output, "router-id")

	// Saved view: save a non-peer edit to draft, then show saved bgp peer.
	require.NoError(t, m.editor.SetValue([]string{"bgp"}, "router-id", "5.6.7.8"))
	require.NoError(t, m.editor.SaveDraft())

	res, err = m.cmdShow([]string{srcSaved, "bgp", "peer"})
	require.NoError(t, err)
	assert.Nil(t, res.configView)
	assert.Equal(t, emptyConfiguration, res.output)
	assert.NotContains(t, res.output, "5.6.7.8")
}

// TestShowBareKeyedListMasksSecrets verifies that rendering a bare keyed list
// masks every ze:sensitive leaf inside list entries.
//
// VALIDATES: DisplayContentAtPath and renderTreeAtPath mask secrets in bare list output.
// PREVENTS: Secret disclosure when showing a keyed list without a key.
func TestShowBareKeyedListMasksSecrets(t *testing.T) {
	schema, tree := secretCLISchemaAndTree(true)
	ed := secretCLIEditor(schema, tree)
	m := &Model{editor: ed}

	listPath := []string{"environment", "api-server", "client"}

	display := ed.DisplayContentAtPath(listPath)
	assert.Contains(t, display, "fleet")
	assert.Contains(t, display, config.SecretDataPlaceholder)
	assert.NotContains(t, display, storedCLISecret)
	assert.NotContains(t, display, "enabled", "bare list must not include sibling container leaves")

	rendered := m.renderTreeAtPath(tree, listPath, fmtTree)
	assert.Contains(t, rendered, "fleet")
	assert.Contains(t, rendered, config.SecretDataPlaceholder)
	assert.NotContains(t, rendered, storedCLISecret)
	assert.NotContains(t, rendered, "enabled", "bare list must not include sibling container leaves")
}
