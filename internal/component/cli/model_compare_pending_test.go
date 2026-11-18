// Design: docs/guide/config-editor.md -- default comparison reviews session intent.
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
)

const pendingCompareSiblings = `bgp {
    router-id 1.2.3.4
    session { asn { local 65000; } }
    peer alpha { connection { remote { ip 192.0.2.1; } } }
    peer beta { connection { remote { ip 192.0.2.1; } } }
}`

// TestSSHComparePendingSiblingPath types the actual comparison command into
// the SSH model. Equal leaf text in another peer cannot hide the removal or
// leave the replacement without its complete configuration address.
func TestSSHComparePendingSiblingPath(t *testing.T) {
	ed, session, _ := newLoadSessionEditor(t, pendingCompareSiblings)
	require.True(t, ed.treeValid, "the sibling fixture must parse")
	require.NoError(t, ed.SetValue([]string{"bgp", "peer", "alpha", "connection", "remote"}, "ip", "192.0.2.10"))
	before := ed.PendingChanges(session.ID)
	require.Len(t, before, 1)

	m, err := NewModel(ed, FilesystemAuthorityUnknown)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	m = enterLine(t, m, "show | compare")
	require.NoError(t, m.err)
	out := m.ViewportContent()
	assert.Contains(t, out, "- bgp peer alpha connection remote ip 192.0.2.1\n")
	assert.Contains(t, out, "+ bgp peer alpha connection remote ip 192.0.2.10")
	assert.NotContains(t, out, "peer beta", "an untouched sibling is not session intent")
	assert.Equal(t, before, ed.PendingChanges(session.ID), "review must not consume or rebase ownership")
}

// TestSSHComparePendingUsesCurrentCommitted publishes a new confirmed value
// after the session edit. Review reads that value while preserving the edit's
// original Previous value, which commit still needs for conflict detection.
func TestSSHComparePendingUsesCurrentCommitted(t *testing.T) {
	ed, session, path := newLoadSessionEditor(t, pendingCompareSiblings)
	require.True(t, ed.treeValid)
	require.NoError(t, ed.SetValue([]string{"bgp", "peer", "alpha", "connection", "remote"}, "ip", "192.0.2.10"))
	before := ed.PendingChanges(session.ID)
	require.Len(t, before, 1)
	require.Equal(t, "192.0.2.1", before[0].Previous)
	committed := ed.tree.Clone()
	remote := walkPath(committed, ed.schema, []string{"bgp", "peer", "alpha", "connection", "remote"})
	require.NotNil(t, remote)
	remote.Set("ip", "192.0.2.20")
	_, err := storage.WriteCandidateVersion(ed.store, path, []byte(config.Serialize(committed, ed.schema)), time.Now())
	require.NoError(t, err)
	require.NoError(t, storage.PromoteCandidate(ed.store, path))

	m, err := NewModel(ed, FilesystemAuthorityUnknown)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	m = enterLine(t, m, "show | compare")
	require.NoError(t, m.err)
	out := m.ViewportContent()
	assert.Contains(t, out, "- bgp peer alpha connection remote ip 192.0.2.20\n")
	assert.Contains(t, out, "+ bgp peer alpha connection remote ip 192.0.2.10")
	assert.NotContains(t, out, "- bgp peer alpha connection remote ip 192.0.2.1\n")
	assert.Equal(t, before, ed.PendingChanges(session.ID), "comparison must retain the original conflict baseline")
}

// TestSSHComparePendingEmpty keeps a real unchanged session's answer empty;
// a comparison cannot satisfy the changed-path case by printing every entry.
func TestSSHComparePendingEmpty(t *testing.T) {
	ed, _, _ := newLoadSessionEditor(t, pendingCompareSiblings)
	require.True(t, ed.treeValid)
	m, err := NewModel(ed, FilesystemAuthorityUnknown)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.Close()) })
	m = enterLine(t, m, "show | compare")
	require.NoError(t, m.err)
	assert.Empty(t, strings.TrimSpace(m.ViewportContent()))
}
