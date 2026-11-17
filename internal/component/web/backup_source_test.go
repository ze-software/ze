// Design: docs/architecture/web-interface.md -- hosting-schema backup refusals.
package web

import (
	"bytes"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// TestBackupHTTPMalformedSourceKeepsArtifact permits raw inspection of an
// intact artifact whose committed text is malformed, but no editor action may
// mistake its missing parse tree for an empty, valid configuration.
func TestBackupHTTPMalformedSourceKeepsArtifact(t *testing.T) {
	backupHTTPRefusesInvalidSource(t, "# keep raw <&>\nbgp {\n  router-id 1.2.3.4;\n", "unexpected EOF in bgp block")
}

// TestBackupHTTPUnavailableSchemaKeepsArtifact preserves a configuration from
// an unavailable schema for inspection and download; validation, review and
// save refuse its unknown root instead of silently dropping it.
func TestBackupHTTPUnavailableSchemaKeepsArtifact(t *testing.T) {
	backupHTTPRefusesInvalidSource(t, validWebConfig+"\nbackup-unavailable-schema { option keep; }\n", "unknown top-level keyword: backup-unavailable-schema")
}

func backupHTTPRefusesInvalidSource(t *testing.T, content, diagnostic string) {
	t.Helper()
	owner, source := backupHTTPFixture(t)
	original := filepath.Join(t.TempDir(), "invalid-source.zefs")
	require.NoError(t, os.WriteFile(original, source, 0o600))
	artifact, err := storage.OpenBlob(original, true)
	require.NoError(t, err)
	require.NoError(t, artifact.WriteFile("router.conf", []byte(content), 0o600))
	require.NoError(t, artifact.Close())
	source, err = os.ReadFile(original)
	require.NoError(t, err)
	before := backupSourceKeys(t, original)
	upload := backupHTTPUpload(t, owner, source, "alice", "https://example.com")
	require.Equal(t, http.StatusSeeOther, upload.Code, upload.Body.String())
	path := upload.Header().Get("Location")
	id := strings.Trim(strings.TrimPrefix(path, backupWorkspacePath), "/")
	workspace := owner.items[id]
	require.NotNil(t, workspace)
	opened := backupHTTPRequest(t, owner, path+"?config=router.conf", "alice", nil)
	require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	assert.Equal(t, content, backupSourceTextarea(t, opened.Body.String()), "raw committed text must remain inspectable")
	base := backupHTTPField(t, opened.Body.String(), "base")
	for _, action := range []string{"validate", "review", "save"} {
		t.Run(action, func(t *testing.T) {
			form := url.Values{"config": {"router.conf"}, "base": {base}, "content": {content},
				"action": {action}, "review": {backupReviewToken(base, content)}}
			refused := backupHTTPRequest(t, owner, path, "alice", form)
			require.Equal(t, http.StatusBadRequest, refused.Code, refused.Body.String())
			assert.Contains(t, refused.Body.String(), diagnostic)
			assert.Contains(t, refused.Body.String(), "Correct the reported configuration problem and select Validate again")
			assert.Equal(t, content, backupSourceTextarea(t, refused.Body.String()), "refused text must remain available for correction")
			retained, err := os.ReadFile(workspace.path)
			require.NoError(t, err)
			assert.Equal(t, source, retained, "a refused action must leave the entire retained artifact byte-identical")
		})
	}
	download := backupHTTPRequest(t, owner, path+"download", "alice", nil)
	require.Equal(t, http.StatusOK, download.Code, download.Body.String())
	output := filepath.Join(t.TempDir(), "download.zefs")
	require.NoError(t, os.WriteFile(output, download.Body.Bytes(), 0o600))
	require.NoError(t, storage.CheckArtifact(output))
	assert.Equal(t, before, backupSourceKeys(t, output), "download must retain every raw key despite unavailable validation")
	retained, err := os.ReadFile(original)
	require.NoError(t, err)
	assert.Equal(t, source, retained, "the user's original artifact must remain unchanged")
}

func backupSourceTextarea(t *testing.T, content string) string {
	t.Helper()
	tokens := html.NewTokenizer(strings.NewReader(content))
	// The finite rendered document bounds this tokenizer walk.
	for tokens.Next() != html.ErrorToken {
		token := tokens.Token()
		if token.Data != "textarea" {
			continue
		}
		for _, attribute := range token.Attr {
			if attribute.Key != "id" {
				continue
			}
			if attribute.Val != "backup-content" {
				continue
			}
			require.Equal(t, html.TextToken, tokens.Next())
			// HTML textarea parsing removes one structural leading LF. The
			// template supplies it separately so the content's LF survives.
			return strings.TrimPrefix(tokens.Token().Data, "\n")
		}
	}
	t.Fatal("backup page lacks its configuration textarea")
	return ""
}

func backupSourceKeys(t *testing.T, path string) map[string][]byte {
	t.Helper()
	artifact, err := storage.OpenBlob(path, false)
	require.NoError(t, err)
	keys, err := artifact.ListKeys("")
	require.NoError(t, err)
	values := make(map[string][]byte, len(keys))
	// The test-owned artifact's fixed key census bounds this walk.
	for _, key := range keys {
		value, err := artifact.ReadKey(key)
		require.NoError(t, err, key)
		values[key] = bytes.Clone(value)
	}
	require.NoError(t, artifact.Close())
	return values
}
