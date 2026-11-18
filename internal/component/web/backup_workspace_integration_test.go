// Design: docs/architecture/web-interface.md -- independent backup browser editing.
//go:build integration

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

// TestBrowserBackupWorkspace follows the production route registry through a
// real file upload, correction, review, save and native artifact download. It
// reopens the downloaded artifact with the normal restore reader and compares
// every preexisting key except the selected configuration's publication keys.
// The sequence proves isolation from an existing live candidate as well as
// preservation of the uploaded artifact's separate saved draft and candidate.
// MUTATION: wiring a backup save to the live manager changes its candidate or
// invokes its publication trap; exporting config text fails the artifact reopen.
func TestBrowserBackupWorkspace(t *testing.T) {
	live := newTestEditorManager(t)
	require.NoError(t, live.SetValue("alice", []string{"bgp"}, "router-id", "192.0.2.210"))
	liveCandidate, err := live.Diff("alice")
	require.NoError(t, err)
	require.Contains(t, liveCandidate, "192.0.2.210", "the live candidate sentinel must exist before backup editing")
	liveKeys := backupBrowserKeys(t, live.store)
	var publications atomic.Int32
	live.SetCommitHook(func() error {
		publications.Add(1)
		return errors.New("backup editing reached live publication")
	})

	uploadPath, originalKeys := backupBrowserArtifact(t)
	originalBytes, err := os.ReadFile(uploadPath)
	require.NoError(t, err)
	browser := newBackupBrowser(t, live)
	browser.wait(`!!document.getElementById('backup-upload-file')`, "backup upload form")
	browser.command("upload", "#backup-upload-file", uploadPath)
	browser.command("click", "#backup-upload-submit")
	browser.wait(`!!document.getElementById('backup-config-select')`, "uploaded artifact configuration selection")
	require.Equal(t, "true", browser.evaluate(`Array.from(document.getElementById('backup-config-select').options).some(function(o){return o.value==='router.conf';})`))
	require.Equal(t, "true", browser.evaluate(`Array.from(document.getElementById('backup-config-select').options).some(function(o){return o.value==='spare.conf';})`))
	require.Equal(t, "false", browser.evaluate(`Array.from(document.getElementById('backup-config-select').options).some(function(o){return o.value==='router.conf.draft'||o.value==='router.conf.change.alice';})`),
		"saved editor records must not be offered as committed configurations")
	browser.command("select", "#backup-config-select", "router.conf")
	browser.command("click", "#backup-config-open")
	browser.wait(`!!document.getElementById('backup-content')`, "selected committed configuration")
	backupBrowserAssertContent(t, browser, validWebConfig,
		"opening the backup must not resume its saved draft or pending candidate")

	invalid := "bgp { router-id invalid-address; session { asn { local 65000; } } }\n"
	browser.command("fill", "#backup-content", invalid)
	browser.command("click", "#backup-validate")
	browser.wait(`!!document.getElementById('backup-error')&&document.getElementById('backup-error').textContent.trim().length>0`, "invalid edit refusal")
	backupBrowserAssertContent(t, browser, invalid, "the refused edit must remain available for correction")

	updated := "bgp {\n\trouter-id 203.0.113.45;\n\tsession { asn { local 65000; } }\n}\n"
	browser.command("fill", "#backup-content", updated)
	browser.command("click", "#backup-validate")
	browser.wait(`!!document.getElementById('backup-status')&&document.getElementById('backup-status').textContent.trim().length>0&&!document.getElementById('backup-error')`, "corrected configuration validation")
	browser.command("click", "#backup-review")
	browser.wait(`!!document.getElementById('backup-diff')&&document.getElementById('backup-diff').textContent.includes('203.0.113.45')`, "review of the corrected change")
	browser.command("click", "#backup-save")
	browser.wait(`!!document.getElementById('backup-status')&&document.getElementById('backup-status').textContent.toLowerCase().includes('saved')`, "artifact-only configuration save")

	downloadPath := filepath.Join(t.TempDir(), "updated.zefs")
	browser.command("download", "#backup-download", downloadPath)
	info, err := os.Lstat(downloadPath)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "the native browser download must be a regular artifact file")
	require.NoError(t, os.Chmod(downloadPath, 0o600), "apply the documented private-file permissions before opening the downloaded backup")
	restored, err := storage.ReadRestoreSource(downloadPath, "router.conf", "router.conf")
	require.NoError(t, err, "the browser must download a complete artifact accepted by the normal restore reader")
	restoredTree, err := config.NewParser(live.schema).Parse(string(restored.Data))
	require.NoError(t, err, "the saved artifact configuration must retain valid native syntax")
	bgp := restoredTree.GetContainer("bgp")
	require.NotNil(t, bgp)
	routerID, exists := bgp.Get("router-id")
	require.True(t, exists)
	assert.Equal(t, "203.0.113.45", routerID)
	asn := restoredTree.GetContainerPath("bgp/session/asn")
	require.NotNil(t, asn)
	localAS, exists := asn.Get("local")
	require.True(t, exists)
	assert.Equal(t, "65000", localAS, "saving the edited address must preserve the local AS")
	expectedTree, err := config.NewParser(live.schema).Parse(updated)
	require.NoError(t, err)
	assert.Equal(t, expectedTree.ToMap(), restoredTree.ToMap(),
		"shared publication may format or stamp the text but must preserve all configured values")
	downloaded, err := storage.OpenBlob(downloadPath, false)
	require.NoError(t, err)
	afterKeys := backupBrowserKeys(t, downloaded)
	activeStamp := strings.TrimSpace(string(afterKeys[zefs.KeyConfigActive.Key("router.conf")]))
	activeVersion, err := downloaded.ReadVersion("router.conf", activeStamp)
	require.NoError(t, err)
	assert.Equal(t, restored.Data, activeVersion, "the active version pointer must resolve to the downloaded configuration")
	assert.Equal(t, restored.Data, afterKeys[zefs.KeyFileActive.Key("router.conf")],
		"the active mirror and version pointer must contain the same published bytes")
	require.NoError(t, downloaded.Close())
	publicationKeys := map[string]bool{
		zefs.KeyConfigActive.Key("router.conf"):   true,
		zefs.KeyConfigRollback.Key("router.conf"): true,
		zefs.KeyFileActive.Key("router.conf"):     true,
	}
	for key, original := range originalKeys {
		if publicationKeys[key] {
			continue
		}
		preserved, exists := afterKeys[key]
		require.True(t, exists, "preexisting artifact key must survive: %s", key)
		assert.Equal(t, original, preserved, "preexisting artifact key must stay byte-identical: %s", key)
	}
	assert.Equal(t, originalKeys[zefs.KeyConfigActive.Key("router.conf")], afterKeys[zefs.KeyConfigRollback.Key("router.conf")],
		"rollback must still name the configuration the operator reviewed")
	assert.NotEqual(t, originalKeys[zefs.KeyConfigActive.Key("router.conf")], afterKeys[zefs.KeyConfigActive.Key("router.conf")])
	assert.Equal(t, liveKeys, backupBrowserKeys(t, live.store), "backup requests must preserve every live-store key")
	liveAfter, err := live.Diff("alice")
	require.NoError(t, err)
	assert.Equal(t, liveCandidate, liveAfter, "backup requests must preserve the existing live candidate")
	assert.Zero(t, publications.Load(), "no backup request may invoke a live publication callback")
	keptOriginal, err := os.ReadFile(uploadPath)
	require.NoError(t, err)
	assert.Equal(t, originalBytes, keptOriginal, "upload must leave the user's original file unchanged")

	browser.command("click", "#backup-close")
	browser.wait(`!!document.getElementById('backup-upload-file')&&!document.getElementById('backup-content')`, "closed workspace returns to upload")
}

// backupBrowserArtifact supplies both active configurations and actual pending
// artifact state so a browser save cannot pass by exporting its input unchanged
// or by discarding every key except the chosen configuration.
func backupBrowserArtifact(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "device.zefs")
	artifact, err := storage.CreateBlob(path, zefs.Spare(0))
	require.NoError(t, err)
	_, err = storage.RestoreConfig(artifact, "router.conf", []byte(validWebConfig))
	require.NoError(t, err)
	other := "bgp { router-id 192.0.2.99; session { asn { local 65001; } } }\n"
	_, err = storage.RestoreConfig(artifact, "spare.conf", []byte(other))
	require.NoError(t, err)
	draft := []byte("bgp { router-id 198.51.100.99; session { asn { local 65000; } } }\n")
	_, err = storage.WriteCandidateVersion(artifact, "router.conf", draft, time.Now().Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, artifact.WriteKey(zefs.KeyFileDraft.Key("router.conf"), draft))
	editor, err := cli.NewOfflineSessionEditor(artifact, "router.conf", "alice")
	require.NoError(t, err)
	require.NoError(t, editor.SetValue([]string{"bgp"}, "router-id", "198.51.100.99"))
	require.NoError(t, editor.SaveDraft())
	require.NoError(t, editor.SetValue([]string{"bgp"}, "router-id", "198.51.100.100"))
	require.True(t, artifact.Exists(cli.DraftPath("router.conf")))
	require.True(t, artifact.Exists(cli.ChangePath("router.conf", "alice")))
	require.NoError(t, artifact.WriteKey("test/backup/binary", []byte{0x00, 0xff, 0xc3, 0x28, 0x7f, 0x00}))
	require.NoError(t, artifact.WriteKey(zefs.KeyLocalAdminPassword.Key(), []byte("retained-private-credential")))
	keys := backupBrowserKeys(t, artifact)
	require.NoError(t, artifact.Close())
	return path, keys
}

// backupBrowserKeys reads through the raw-key API: configuration-only readers
// cannot prove that credentials, draft metadata and arbitrary binary values
// survived the upload and export.
func backupBrowserKeys(t *testing.T, store storage.Storage) map[string][]byte {
	t.Helper()
	keys, err := store.ListKeys("")
	require.NoError(t, err)
	values := make(map[string][]byte, len(keys))
	for _, key := range keys {
		value, err := store.ReadKey(key)
		require.NoError(t, err, key)
		values[key] = bytes.Clone(value)
	}
	return values
}

// backupBrowserAssertContent checks the DOM value inside the browser because
// the shared command adapter trims surrounding whitespace from CLI output.
// A native configuration's trailing newline must still be preserved.
func backupBrowserAssertContent(t *testing.T, browser *downloadBrowser, content, reason string) {
	t.Helper()
	encoded, err := json.Marshal(content)
	require.NoError(t, err)
	require.Equal(t, "true", browser.evaluate("document.getElementById('backup-content').value==="+string(encoded)), reason)
}

// newBackupBrowser serves the registered production subtree behind real cookie
// authentication. It closes the browser, then joins HTTP work, then closes the
// subtree owner; every artifact and native download belongs to this test.
func newBackupBrowser(t *testing.T, live *EditorManager) *downloadBrowser {
	t.Helper()
	_, err := exec.LookPath("agent-browser")
	require.NoError(t, err, "backup workspace acceptance requires agent-browser and Chromium")
	renderer, err := NewRenderer()
	require.NoError(t, err)
	route, ok := lookupWebRoute(RegisteredWebRoutes(), backupWorkspacePath)
	require.True(t, ok, "backup editing must be registered on the production surface")
	require.Equal(t, WrapAuth, route.Wrap)
	handler := route.Build(RouteDeps{Dispatch: backupCommandDispatcher(t), Renderer: renderer, Authorizer: fakeAuthorizer{allowEdit: true}, EditorManager: live})
	closer, ok := handler.(io.Closer)
	require.True(t, ok, "the registered subtree must expose its artifact cleanup obligation")
	t.Cleanup(func() { require.NoError(t, closer.Close()) })
	sessions := NewSessionStore(nil)
	session, err := sessions.createSession("alice", authz.AuthResult{Authorizer: fakeAuthorizer{allowEdit: true}})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.Handle(route.Pattern, handler)
	server := httptest.NewServer(authMiddleware(sessions, &recordingAuthenticator{}, noopRenderer, mux))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	browser := &downloadBrowser{t: t, ctx: ctx,
		session: fmt.Sprintf("ze-backup-%d-%x", os.Getpid(), downloadBrowserSequence.Add(1))}
	t.Cleanup(func() {
		cancel()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		browser.ctx = closeCtx
		browser.command("close")
	})
	browser.command("cookies", "set", "ze-session", session.Token, "--url", server.URL, "--http-only", "--same-site", "Strict")
	browser.command("open", server.URL+backupWorkspacePath)
	return browser
}
