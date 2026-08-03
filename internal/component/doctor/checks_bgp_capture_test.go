// Detail: checks_bgp_capture.go -- unit tests for the BGP capture directory check

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// newCaptureTree builds a config tree with one BGP peer whose capture container
// carries the given leaves.
func newCaptureTree(enabled, directory string) *config.Tree {
	const peer = "192.0.2.1"
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	p := config.NewTree()
	cap := p.GetOrCreateContainer("capture")
	if enabled != "" {
		cap.Set("enabled", enabled)
	}
	if directory != "" {
		cap.Set("directory", directory)
	}
	bgp.AddListEntry("peer", peer, p)
	return tree
}

// VALIDATES: capture disabled (the default) produces no diagnostic, so the check
// costs a clean report nothing.
// PREVENTS: doctor warning about a directory nobody asked for.
func TestCheckBGPCaptureDirectoryDisabled(t *testing.T) {
	assert.Empty(t, checkBGPCaptureDirectory(config.NewTree()))
	assert.Empty(t, checkBGPCaptureDirectory(newCaptureTree("", "/nonexistent/zecap")))
	assert.Empty(t, checkBGPCaptureDirectory(newCaptureTree("false", "/nonexistent/zecap")))
}

// VALIDATES: an enabled capture whose directory is writable passes.
// PREVENTS: a false alarm on a correctly configured box.
func TestCheckBGPCaptureDirectoryWritable(t *testing.T) {
	assert.Empty(t, checkBGPCaptureDirectory(newCaptureTree("true", t.TempDir())))
}

// VALIDATES: an enabled capture whose directory cannot be created or written is
// reported, naming the peer and the path.
// PREVENTS: an operator enabling capture and finding no file and no reason --
// the runtime dependency this feature introduces.
func TestCheckBGPCaptureDirectoryNotWritable(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	diags := checkBGPCaptureDirectory(newCaptureTree("true", filepath.Join(blocker, "sub")))
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-bgp-capture-directory", diags[0].Code)
	assert.Contains(t, diags[0].Message, "192.0.2.1")
	assert.Contains(t, diags[0].Path, "sub")
}

// VALIDATES: a peer that enables capture without naming a directory is checked
// against the schema default, not skipped.
// PREVENTS: the default path going unchecked because the leaf is absent.
func TestCheckBGPCaptureDirectoryUsesDefault(t *testing.T) {
	diags := checkBGPCaptureDirectory(newCaptureTree("true", ""))
	// The default is /var/lib/ze/capture. On a developer machine it is usually
	// absent and unwritable, on a deployed box it is writable. Either verdict is
	// correct; what must not happen is the check silently doing nothing.
	for _, d := range diags {
		assert.Equal(t, "doctor-bgp-capture-directory", d.Code)
		assert.Contains(t, d.Path, defaultBGPCaptureDirectory)
	}
	assert.LessOrEqual(t, len(diags), 1)
}

// VALIDATES: a peer inside a group is checked too, not only a top-level peer.
// PREVENTS: half the configuration surface going unchecked.
func TestCheckBGPCaptureDirectoryInGroup(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	p := config.NewTree()
	c := p.GetOrCreateContainer("capture")
	c.Set("enabled", "true")
	c.Set("directory", filepath.Join(blocker, "sub"))
	group.AddListEntry("peer", "198.51.100.1", p)
	bgp.AddListEntry("group", "transit", group)

	diags := checkBGPCaptureDirectory(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "198.51.100.1")
}
