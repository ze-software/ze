// Design: docs/architecture/web-interface.md -- browser-created full system backups.
//go:build integration

package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestBrowserBackupSystemDownload follows the visible production form to a
// native browser download, then reopens the complete artifact and compares its
// raw keys against the unchanged synthetic daemon-owned store.
func TestBrowserBackupSystemDownload(t *testing.T) {
	store := systemBackupFixture(t)
	before := systemBackupKeys(t, store)
	browser := newBackupBrowser(t, nil)
	browser.wait(`!!document.getElementById('backup-system-download')`, "create system backup control")
	path := filepath.Join(t.TempDir(), "system-backup.zefs")
	browser.command("download", "#backup-system-download", path)
	require.NoError(t, os.Chmod(path, 0o600), "browser downloads need private permissions before the normal ZeFS opener")
	require.NoError(t, storage.CheckArtifact(path))
	require.Equal(t, before, backupSourceKeys(t, path))
	require.Equal(t, before, systemBackupKeys(t, store))
}
