package hub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

// validConfig is minimal hierarchical config the YANG parser accepts.
var validConfig = []byte("system {\n}\n")

// VALIDATES: readConfigWithStorage falls back to ReadConfigFromPath when store is nil.
// PREVENTS: nil-store panic in non-blob deployments.
func TestReadConfigWithStorage_NilStore(t *testing.T) {
	readFn := readConfigWithStorage(nil, filepath.Join(t.TempDir(), "missing.conf"))
	_, _, err := readFn()
	assert.Error(t, err, "nil store with missing file should error")
}

// VALIDATES: readConfigWithStorage reads from blob storage active version.
// PREVENTS: archive scheduler failing on gokrazy where config is blob-backed.
func TestReadConfigWithStorage_BlobStorage(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "prod.conf")

	stamp := "20260617-120000.000"
	parsed, err := storage.ParseVersionStamp(stamp)
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, validConfig, parsed))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, stamp))

	readFn := readConfigWithStorage(store, configPath)
	data, tree, err := readFn()
	require.NoError(t, err)
	assert.Equal(t, validConfig, data)
	assert.NotNil(t, tree)
}

// VALIDATES: readConfigWithStorage falls back to os.ReadFile when blob read fails.
// PREVENTS: regression on filesystem-only deployments using blob-aware store.
func TestReadConfigWithStorage_FallbackToFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "router.conf")

	require.NoError(t, os.WriteFile(configPath, validConfig, 0o600))

	readFn := readConfigWithStorage(store, configPath)
	data, tree, err := readFn()
	require.NoError(t, err)
	assert.Equal(t, validConfig, data)
	assert.NotNil(t, tree)
}

// VALIDATES: readConfigWithStorage returns error when both blob and filesystem fail.
// PREVENTS: silent nil tree on missing config.
func TestReadConfigWithStorage_BothFail(t *testing.T) {
	store := storage.NewFilesystem()
	configPath := filepath.Join(t.TempDir(), "nonexistent.conf")

	readFn := readConfigWithStorage(store, configPath)
	_, _, err := readFn()
	assert.Error(t, err)
}

// VALIDATES: startArchiveScheduler is a no-op when tree is nil.
// PREVENTS: panic on nil tree during reload when load() returns nil parsedTree.
func TestStartArchiveScheduler_NilTree(t *testing.T) {
	assert.NotPanics(t, func() {
		startArchiveScheduler(nil, "test.conf", nil, nil)
	})
}

// VALIDATES: readConfigWithStorage reads the promoted active version, not stale data.
// PREVENTS: archive scheduler running with pre-commit config after candidate promotion.
func TestReadConfigWithStorage_ReadsPromotedVersion(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewFilesystem()
	configPath := filepath.Join(dir, "prod.conf")

	oldStamp := "20260617-100000.000"
	oldParsed, err := storage.ParseVersionStamp(oldStamp)
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, validConfig, oldParsed))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, oldStamp))

	// An always-on config root, not bgp: this test also runs in the bare ze_core
	// pass, where BGP is compiled out (//go:build ze_bgp) and a bgp{} block is
	// correctly rejected as an unknown keyword. What matters here is only that
	// the content differs from validConfig and parses.
	newContent := []byte("system {\n}\ninterface {\n}\n")
	newStamp := "20260617-110000.000"
	newParsed, err := storage.ParseVersionStamp(newStamp)
	require.NoError(t, err)
	require.NoError(t, store.WriteVersion(configPath, newContent, newParsed))
	require.NoError(t, storage.WritePointer(store, configPath, storage.PointerActive, newStamp))

	readFn := readConfigWithStorage(store, configPath)
	data, _, err := readFn()
	require.NoError(t, err)
	assert.NotEqual(t, validConfig, data, "should not read the old active version")
	assert.Equal(t, newContent, data, "should read the latest promoted active version")
}
