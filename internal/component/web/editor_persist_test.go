package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func TestEditorManagerListEntryPersistence(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("# ze config\n"), 0o600))

	schema, err := config.YANGSchema()
	require.NoError(t, err)

	store := storage.NewFilesystem()
	mgr := NewEditorManager(store, configPath, schema)

	// Simulate CLI set from web handler
	err = mgr.SetValue("insecure", []string{"bgp", "peer", "test-peer", "remote"}, "as", "65001")
	require.NoError(t, err, "SetValue must succeed")

	// Check the tree
	tree := mgr.Tree("insecure")
	require.NotNil(t, tree, "Tree must not be nil after SetValue")

	t.Logf("Root containers: %v", tree.ContainerNames())

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp, "bgp container must exist after SetValue")

	peerList := bgp.GetList("peer")
	require.NotNil(t, peerList, "peer list must exist")
	require.Contains(t, peerList, "test-peer", "test-peer entry must exist")
}
