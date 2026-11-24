// Design: docs/guide/config-editor.md — path-qualified Editor.Diff (AC-30).
// Related: editor.go, editor_pending_diff.go, model_commands_commit.go.

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/audit"
)

// TestEditorDiffNamesChangedSiblingPath verifies that Editor.Diff renders
// path-qualified diff lines so identical leaf lines under distinct sibling
// blocks never collapse, in both session mode and non-session mode.
//
// VALIDATES: AC-30 — Editor.Diff names the full path of the changed sibling
// and keeps removed/added lines distinct from untouched siblings.
// PREVENTS: Path-less line-set diff (computeDiff) hiding removals or dropping
// path context when sibling blocks carry identical leaf lines.
func TestEditorDiffNamesChangedSiblingPath(t *testing.T) {
	t.Run("session editor uses shared path-qualified renderer", func(t *testing.T) {
		ed, _, _ := newLoadSessionEditor(t, pendingCompareSiblings)
		require.NoError(t, ed.SetValue([]string{"bgp", "peer", "alpha", "connection", "remote"}, "ip", "192.0.2.10"))

		diff := ed.Diff()
		assert.Contains(t, diff, "- bgp peer alpha connection remote ip 192.0.2.1\n")
		assert.Contains(t, diff, "+ bgp peer alpha connection remote ip 192.0.2.10\n")
		assert.NotContains(t, diff, "peer beta", "untouched sibling must not appear in diff")

		// Both siblings changed identically, then discard only alpha: DroppedDiff
		// must attribute only alpha and never mark beta's kept change as dropped.
		require.NoError(t, ed.SetValue([]string{"bgp", "peer", "beta", "connection", "remote"}, "ip", "192.0.2.10"))

		recorder, err := audit.NewMemory(100)
		require.NoError(t, err)
		m, err := NewModel(ed, FilesystemAuthorityOperatorLocal)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, m.Close()) })
		m.SetAuditRecorder(recorder, audit.SSH, "alice", "192.0.2.10:2222")

		_, err = m.cmdDiscardSession(strings.Fields("bgp peer alpha"))
		require.NoError(t, err)

		entries := recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard})
		require.Len(t, entries, 1)
		assert.Contains(t, entries[0].Detail, "bgp peer alpha connection remote ip")
		assert.NotContains(t, entries[0].Detail, "bgp peer beta", "kept sibling change must not be audited as dropped")
	})

	t.Run("non-session editor renders path-qualified diff", func(t *testing.T) {
		path := writeTestConfig(t, pendingCompareSiblings)
		ed, err := NewLooseFileEditor(nil, path)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, ed.Close()) })
		require.True(t, ed.treeValid)

		require.NoError(t, ed.SetValue([]string{"bgp", "peer", "alpha", "connection", "remote"}, "ip", "192.0.2.10"))

		diff := ed.Diff()
		assert.Contains(t, diff, "- bgp peer alpha connection remote ip 192.0.2.1\n")
		assert.Contains(t, diff, "+ bgp peer alpha connection remote ip 192.0.2.10\n")
		assert.NotContains(t, diff, "peer beta", "untouched sibling must not appear in diff")
	})
}
