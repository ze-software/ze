//go:build ze_web

// Design: docs/guide/web-interface.md -- independent editor sessions and Session Limits.
// Related: editor_adapter.go -- production editor factory used after cache loss.

package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	zeconfigcmd "github.com/ze-software/ze/internal/component/config/cli"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

const webEditorRecoveryBase = "system { host recovery-original; }\n" +
	"bgp { router-id 192.0.2.1; session { asn { local 65000; } } " +
	"group original { description original-group; } }\n"

// TestWebEditorRecoveryReopensOwningCandidate proves a fresh manager restores
// old web intent through the production factory, authenticated GET and commit.
// A different user's saved draft and the same user's SSH intent are controls:
// neither enters this browser candidate or publication, and both survive it.
func TestWebEditorRecoveryReopensOwningCandidate(t *testing.T) {
	for _, mode := range []string{"finder", "workbench"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newWebEditorRecoveryFixture(t)
			manager := fixture.manager(t)
			fixture.checkCandidate(t, manager, mode)
			fixture.checkCommit(t, manager)
		})
	}
}

// TestWebEditorRecoverySavedIntentKeepsOtherOwners separates recovery of this
// browser's already-saved metadata from loading the entire shared draft. The
// real commit must publish the recovered web leaf and rename exactly once.
func TestWebEditorRecoverySavedIntentKeepsOtherOwners(t *testing.T) {
	fixture := newWebEditorRecoveryFixture(t)
	require.NoError(t, fixture.alice.SaveDraft())
	manager := fixture.manager(t)
	fixture.checkCandidate(t, manager, "finder")
	fixture.checkCommit(t, manager)
}

// TestWebEditorRecoveryRefusesUnreadableIntent exercises a real malformed
// durable file, rather than a factory stub. A refused first GET cannot cache
// a committed-only editor or overwrite intent; repairing the same file lets
// the same manager reopen the complete candidate on its next GET.
func TestWebEditorRecoveryRefusesUnreadableIntent(t *testing.T) {
	fixture := newWebEditorRecoveryFixture(t)
	path := cli.ChangePath("config.conf", "alice")
	good, err := fixture.store.ReadFile(path)
	require.NoError(t, err)
	bad := append([]byte(nil), good...)
	bad = append(bad, []byte("rename bgp group original to\n")...)
	require.NoError(t, fixture.store.WriteFile(path, bad, 0o600))
	manager := fixture.manager(t)
	response := fixture.get(t, manager, "finder", "/show/system/")
	assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), path)
	assert.Contains(t, response.Body.String(), "pending changes")
	assert.NotContains(t, response.Body.String(), "value=\"recovery-original\"")
	assert.Nil(t, manager.Tree("alice"), "a refused open must not publish an empty editor")
	unchanged, err := fixture.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, bad, unchanged)
	assert.ElementsMatch(t, fixture.bobEntries, fixture.entries(t, cli.DraftPath("config.conf"), fixture.bobSession))
	committed, err := fixture.store.ReadFile("config.conf")
	require.NoError(t, err)
	assert.Equal(t, []byte(webEditorRecoveryBase), committed)
	require.NoError(t, fixture.store.WriteFile(path, good, 0o600))
	fixture.checkCandidate(t, manager, "finder")
}

type webEditorRecoveryFixture struct {
	store      storage.Storage
	schema     *zeconfig.Schema
	startup    *zeconfig.Tree
	users      []authz.UserConfig
	alice      *cli.Editor
	bobSession string
	sshSession string
	bobEntries []zeconfig.SessionEntry
	sshEntries []zeconfig.SessionEntry
}

func newWebEditorRecoveryFixture(t *testing.T) *webEditorRecoveryFixture {
	t.Helper()
	setAPIConfigDir(t, t.TempDir())
	store, err := storage.Create(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.WriteFile("config.conf", []byte(webEditorRecoveryBase), 0o600))
	schema, err := zeconfig.YANGSchema()
	require.NoError(t, err)
	startup, err := zeconfig.NewParser(schema).Parse(webEditorRecoveryBase)
	require.NoError(t, err)
	fixture := &webEditorRecoveryFixture{
		store: store, schema: schema, startup: startup,
		users: []authz.UserConfig{bcryptUser(t, "alice", "synthetic")},
	}
	bob := fixture.owner(t, "bob", "web", 1)
	require.NoError(t, bob.SetValue([]string{"bgp", "group", "bob-saved"}, "description", "bob-only"))
	require.NoError(t, bob.SaveDraft())
	fixture.bobSession = bob.SessionID()
	fixture.alice = fixture.owner(t, "alice", "web", 2)
	require.NoError(t, fixture.alice.SetValue([]string{"system"}, "host", "alice-pending"))
	require.NoError(t, fixture.alice.RenameListEntry([]string{"bgp"}, "group", "original", []string{"alice-renamed"}))
	ssh := fixture.owner(t, "alice", "ssh", 3)
	require.NoError(t, ssh.SetValue([]string{"bgp", "group", "alice-ssh"}, "description", "ssh-only"))
	fixture.sshSession = ssh.SessionID()
	fixture.bobEntries = fixture.entries(t, cli.DraftPath("config.conf"), fixture.bobSession)
	fixture.sshEntries = fixture.entries(t, cli.ChangePath("config.conf", "alice"), fixture.sshSession)
	require.Len(t, fixture.bobEntries, 1)
	require.Len(t, fixture.sshEntries, 1)
	return fixture
}

func (f *webEditorRecoveryFixture) owner(t *testing.T, user, origin string, second int) *cli.Editor {
	t.Helper()
	created, err := newEditorFactory(zeconfigcmd.ValidateContent)(f.store, "config.conf")
	require.NoError(t, err)
	adapter, ok := created.(*editorAdapter)
	require.True(t, ok)
	t.Cleanup(func() { require.NoError(t, adapter.ed.Close()) })
	started := time.Date(2000, 1, 1, 0, 0, second, 0, time.UTC)
	adapter.ed.SetSession(&cli.EditSession{
		User: user, Origin: origin, StartTime: started,
		ID: user + "@" + origin + "%" + started.Format(time.RFC3339),
	})
	return adapter.ed
}

func (f *webEditorRecoveryFixture) manager(t *testing.T) *zeweb.EditorManager {
	t.Helper()
	manager := zeweb.NewEditorManager(f.store, "config.conf", f.schema,
		newEditorFactory(zeconfigcmd.ValidateContent), newEditSessionFactory())
	manager.SetCommitHook(func() error { return storage.PromoteCandidate(f.store, "config.conf") })
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	return manager
}

func (f *webEditorRecoveryFixture) checkCandidate(t *testing.T, manager *zeweb.EditorManager, mode string) {
	t.Helper()
	response := f.get(t, manager, mode, "/show/system/")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "value=\"alice-pending\"")
	response = f.get(t, manager, mode, "/show/bgp/group/alice-renamed/")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "original-group")
	assert.Equal(t, 2, manager.ChangeCount("alice"), "the recovered leaf and rename remain reviewable")
	diff := manager.Diff("alice")
	assert.Contains(t, diff, "alice-pending")
	assert.Contains(t, diff, "rename")
	assert.Contains(t, diff, "alice-renamed")
	assert.NotContains(t, diff, "bob-only")
	assert.NotContains(t, diff, "ssh-only")
	candidate := manager.ContentAtPath("alice", nil)
	assert.Contains(t, candidate, "alice-renamed")
	assert.NotContains(t, candidate, "bob-saved")
	assert.NotContains(t, candidate, "alice-ssh")
	committed, err := f.store.ReadFile("config.conf")
	require.NoError(t, err)
	assert.Equal(t, []byte(webEditorRecoveryBase), committed)
	f.checkOwners(t)
}

func (f *webEditorRecoveryFixture) checkCommit(t *testing.T, manager *zeweb.EditorManager) {
	t.Helper()
	form := url.Values{"action": {"now"}, "path": {""}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://ze.test/config/action", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := f.serve(t, manager, "finder", request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "commit successful")
	committed, err := f.store.ReadFile("config.conf")
	require.NoError(t, err)
	tree, _, err := zeconfig.NewSetParser(f.schema).ParseWithMeta(string(committed))
	require.NoError(t, err)
	system := tree.GetContainer("system")
	require.NotNil(t, system)
	host, exists := system.Get("host")
	require.True(t, exists)
	assert.Equal(t, "alice-pending", host)
	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp)
	assert.Equal(t, []string{"alice-renamed"}, bgp.ListKeys("group"))
	assert.Zero(t, manager.ChangeCount("alice"))
	assert.Empty(t, manager.Diff("alice"))
	f.checkOwners(t)
}

func (f *webEditorRecoveryFixture) checkOwners(t *testing.T) {
	t.Helper()
	assert.ElementsMatch(t, f.bobEntries, f.entries(t, cli.DraftPath("config.conf"), f.bobSession))
	assert.ElementsMatch(t, f.sshEntries, f.entries(t, cli.ChangePath("config.conf", "alice"), f.sshSession))
}

func (f *webEditorRecoveryFixture) entries(t *testing.T, path, session string) []zeconfig.SessionEntry {
	t.Helper()
	data, err := f.store.ReadFile(path)
	require.NoError(t, err)
	_, metadata, _, err := zeconfig.ParseCandidateChangeFile(string(data), f.schema)
	require.NoError(t, err)
	return metadata.SessionEntries(f.schema, session)
}

func (f *webEditorRecoveryFixture) get(t *testing.T, manager *zeweb.EditorManager, mode, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://ze.test"+path, nil)
	return f.serve(t, manager, mode, request)
}

func (f *webEditorRecoveryFixture) serve(t *testing.T, manager *zeweb.EditorManager, mode string, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	renderer, err := zeweb.NewRenderer()
	require.NoError(t, err)
	show := zeweb.HandleFragment(renderer, f.schema, f.startup, manager, false, nil)
	if mode == "workbench" {
		show = zeweb.HandleWorkbench(renderer, f.schema, f.startup, manager, false)
	}
	mux := http.NewServeMux()
	mux.Handle("/show/", show)
	mux.Handle("POST /config/action", zeweb.RequireSameOrigin(
		zeweb.HandleGraphicalAction(manager, f.schema, renderer, nil, nil, nil)))
	auth := &authz.LocalAuthenticator{Users: f.users}
	sessions := zeweb.NewSessionStore(func() ([]authz.UserConfig, error) { return f.users, nil })
	refuse := func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "login required", http.StatusUnauthorized) }
	handler := zeweb.AuthMiddlewareWithAudit(sessions, auth, refuse, mux, nil)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Origin", "https://ze.test")
	request.SetBasicAuth("alice", "synthetic")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
