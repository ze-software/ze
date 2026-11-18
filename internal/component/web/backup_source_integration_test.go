//go:build integration

// Design: docs/architecture/web-interface.md -- binary source repair through native browser forms.
package web

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestBrowserBackupSourceRepair proves a browser can upload a damaged artifact
// without first passing workspace admission, retain its failed check, and keep
// both the original bytes and an independently downloadable native salvage.
func TestBrowserBackupSourceRepair(t *testing.T) {
	live := newTestEditorManager(t)
	original, keys := backupBrowserArtifact(t)
	content, err := os.ReadFile(original)
	require.NoError(t, err)
	damaged := bytes.Clone(content)
	offset := bytes.Index(damaged, []byte("retained-private-credential"))
	require.GreaterOrEqual(t, offset, 0)
	damaged[offset] ^= 0xff
	input := filepath.Join(t.TempDir(), "damaged.zefs")
	require.NoError(t, os.WriteFile(input, damaged, 0o600))
	browser := newBackupBrowser(t, live)
	browser.wait(`!!document.getElementById('backup-source-file')`, "raw source file control")
	browser.command("upload", "#backup-source-file", input)
	browser.command("click", "#backup-source-upload")
	browser.wait(`!!document.getElementById('backup-source-check')`, "uploaded source controls")
	browser.command("click", "#backup-source-check")
	browser.wait(`!!document.getElementById('backup-source-report')&&document.getElementById('backup-source-report').textContent.toLowerCase().includes('crc')`, "native failed check document")
	kept := filepath.Join(t.TempDir(), "original-damaged.bin")
	browser.command("download", "#backup-source-download", kept)
	keptBytes, err := os.ReadFile(kept)
	require.NoError(t, err)
	require.Equal(t, damaged, keptBytes)
	browser.command("click", "#backup-source-repair")
	browser.wait(`!!document.getElementById('backup-source-salvage')&&document.getElementById('backup-source-status').textContent.toLowerCase().includes('warning')`, "salvage warning and download")
	salvage := filepath.Join(t.TempDir(), "salvaged.zefs")
	browser.command("download", "#backup-source-salvage", salvage)
	require.NoError(t, os.Chmod(salvage, 0o600))
	require.NoError(t, storage.CheckArtifact(salvage))
	store, err := storage.OpenBlob(salvage, false)
	require.NoError(t, err)
	actual, err := storage.ReadActiveConfig(store, "router.conf")
	require.NoError(t, err)
	require.Equal(t, keys["file/active/router.conf"], actual)
	binary, err := store.ReadKey("test/backup/binary")
	require.NoError(t, err)
	require.Equal(t, keys["test/backup/binary"], binary)
	require.NoError(t, store.Close())
	after := filepath.Join(t.TempDir(), "original-after-repair.bin")
	browser.command("download", "#backup-source-download", after)
	afterBytes, err := os.ReadFile(after)
	require.NoError(t, err)
	require.Equal(t, damaged, afterBytes)
	inputBytes, err := os.ReadFile(input)
	require.NoError(t, err)
	require.Equal(t, damaged, inputBytes, "browser requests never edit the selected local file")
	browser.command("click", "#backup-source-close")
	browser.wait(`!!document.getElementById('backup-source-file')&&!document.getElementById('backup-source-check')`, "closed source returns to upload")
}
