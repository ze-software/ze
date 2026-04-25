package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const editorTestConfig = `bgp {
	session {
		asn {
			local 65000;
		}
	}
	router-id 1.2.3.4;
}
`

func newEditorWithConfig(t *testing.T) *Editor {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte(editorTestConfig), 0o600))
	ed, err := NewEditor(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ed.Close() })
	return ed
}

// TestEditorDeactivateLeaf verifies the wrapper marks the right
// Tree node inactive and flips the dirty flag.
//
// VALIDATES: AC-1 -- Editor.DeactivateLeaf is the single mutation
// hook the CLI verb and the TUI both call.
func TestEditorDeactivateLeaf(t *testing.T) {
	ed := newEditorWithConfig(t)

	require.NoError(t, ed.DeactivateLeaf([]string{"bgp"}, "router-id"))
	bgp := ed.Tree().GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.True(t, bgp.IsLeafInactive("router-id"))
	assert.True(t, ed.Dirty())

	v, ok := bgp.Get("router-id")
	require.True(t, ok)
	assert.Equal(t, "1.2.3.4", v, "value preserved verbatim through deactivation")
}

// TestEditorActivateLeafSymmetric verifies activate undoes deactivate.
func TestEditorActivateLeafSymmetric(t *testing.T) {
	ed := newEditorWithConfig(t)
	require.NoError(t, ed.DeactivateLeaf([]string{"bgp"}, "router-id"))
	require.NoError(t, ed.ActivateLeaf([]string{"bgp"}, "router-id"))
	bgp := ed.Tree().GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.False(t, bgp.IsLeafInactive("router-id"))
}

// TestEditorDeactivateLeafIdempotentReject verifies a second
// deactivate on the same already-inactive leaf returns the
// ErrLeafAlreadyInactive sentinel so callers can use errors.Is for
// idempotent flows without inspecting error strings.
func TestEditorDeactivateLeafIdempotentReject(t *testing.T) {
	ed := newEditorWithConfig(t)
	require.NoError(t, ed.DeactivateLeaf([]string{"bgp"}, "router-id"))
	err := ed.DeactivateLeaf([]string{"bgp"}, "router-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLeafAlreadyInactive),
		"expected ErrLeafAlreadyInactive sentinel, got %v", err)
}

// TestEditorDeactivateLeafPermissive verifies the engine-level rule
// that pre-marking an absent leaf is allowed (Tree.SetLeafInactive
// contract). This is what lets a leaf with a YANG default be
// deactivated without a prior explicit set.
func TestEditorDeactivateLeafPermissive(t *testing.T) {
	ed := newEditorWithConfig(t)
	require.NoError(t, ed.DeactivateLeaf([]string{"bgp"}, "asn4"))
	bgp := ed.Tree().GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.True(t, bgp.IsLeafInactive("asn4"),
		"leaf marker must be set even when value is absent (default)")
	assert.True(t, ed.Dirty())
}

// TestEditorDeactivateLeafBadParent verifies a non-existent parent path
// is rejected -- path resolution must still gate mutation.
func TestEditorDeactivateLeafBadParent(t *testing.T) {
	ed := newEditorWithConfig(t)
	err := ed.DeactivateLeaf([]string{"does-not-exist"}, "router-id")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathNotFound),
		"expected ErrPathNotFound for missing parent, got %v", err)
	assert.False(t, ed.Dirty(), "no mutation when parent missing")
}

// TestEditorDeactivatePathRejectsMissing verifies the strict path
// helper rejects deactivation of a non-existent container/list-entry
// rather than silently materializing it.
func TestEditorDeactivatePathRejectsMissing(t *testing.T) {
	ed := newEditorWithConfig(t)
	err := ed.DeactivatePath([]string{"bgp", "peer", "never-existed"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathNotFound),
		"expected ErrPathNotFound, got %v", err)
	bgp := ed.Tree().GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.Empty(t, bgp.GetList("peer"),
		"no peer list entry should have been created")
	assert.False(t, ed.Dirty(), "no mutation when path missing")
}

// TestEditorActivatePathIdempotent verifies the activate counterpart
// surfaces ErrPathNotInactive when the path is already active.
func TestEditorActivatePathIdempotent(t *testing.T) {
	ed := newEditorWithConfig(t)
	err := ed.ActivatePath([]string{"bgp", "session"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathNotInactive),
		"expected ErrPathNotInactive, got %v", err)
}
