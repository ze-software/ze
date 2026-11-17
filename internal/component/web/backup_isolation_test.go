// Design: docs/architecture/web-interface.md -- independent explicit workspace URLs.
package web

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestBackupHTTPSameAccountWorkspacesStayIndependent opens two forms before
// either save, edits different contained configurations under the same account,
// and reopens both downloads to prove that each explicit URL retains its own
// saved result and every unrelated original key.
func TestBackupHTTPSameAccountWorkspacesStayIndependent(t *testing.T) {
	owner, source := backupHTTPFixture(t)
	original := filepath.Join(t.TempDir(), "original.zefs")
	require.NoError(t, os.WriteFile(original, source, 0o600))
	before := backupSourceKeys(t, original)
	tabs := []struct {
		name    string
		content string
		path    string
		form    url.Values
	}{
		{name: "router.conf", content: strings.Replace(validWebConfig, "1.2.3.4", "203.0.113.40", 1)},
		{name: "other.conf", content: strings.Replace(validWebConfig, "1.2.3.4", "203.0.113.50", 1)},
	}
	for index := range tabs {
		tab := &tabs[index]
		upload := backupHTTPUpload(t, owner, source, "alice", "https://example.com")
		require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
		tab.path = upload.Header().Get("Location")
		opened := backupHTTPRequest(t, owner, tab.path+"?config="+url.QueryEscape(tab.name), "alice", nil)
		require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
		tab.form = url.Values{"config": {tab.name}, "base": {backupHTTPField(t, opened.Body.String(), "base")},
			"content": {tab.content}, "action": {"validate"}, "review": {""}}
		validated := backupHTTPRequest(t, owner, tab.path, "alice", tab.form)
		require.Equal(t, http.StatusOK, validated.Code, validated.Body.String())
		tab.form.Set("action", "review")
		reviewed := backupHTTPRequest(t, owner, tab.path, "alice", tab.form)
		require.Equal(t, http.StatusOK, reviewed.Code, reviewed.Body.String())
		tab.form.Set("review", backupHTTPField(t, reviewed.Body.String(), "review"))
		tab.form.Set("action", "save")
	}
	require.NotEqual(t, tabs[0].path, tabs[1].path)
	for _, tab := range tabs {
		saved := backupHTTPRequest(t, owner, tab.path, "alice", tab.form)
		require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
		require.Contains(t, saved.Body.String(), "saved into the backup")
	}
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	for _, tab := range tabs {
		download := backupHTTPRequest(t, owner, tab.path+"download", "alice", nil)
		require.Equal(t, http.StatusOK, download.Code, download.Body.String())
		output := filepath.Join(t.TempDir(), "download.zefs")
		require.NoError(t, os.WriteFile(output, download.Body.Bytes(), 0o600))
		restored, err := storage.ReadRestoreSource(output, tab.name, "")
		require.NoError(t, err)
		got, err := config.NewParser(schema).Parse(string(restored.Data))
		require.NoError(t, err)
		want, err := config.NewParser(schema).Parse(tab.content)
		require.NoError(t, err)
		assert.Equal(t, want.ToMap(), got.ToMap(), tab.path)
		keys := backupSourceKeys(t, output)
		for key, value := range before {
			if key == zefs.KeyFileActive.Key(tab.name) {
				continue
			}
			assert.Equal(t, value, keys[key], "workspace %s changed unrelated key %s", tab.path, key)
		}
	}
	retained, err := os.ReadFile(original)
	require.NoError(t, err)
	assert.Equal(t, source, retained, "neither workspace may change the original upload")
}
