// Design: docs/architecture/web-interface.md -- browser recovery without file access.
//go:build integration

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/core/audit"
)

// TestBrowserConfigRecoveryFromBrokenFinder starts with a malformed stored
// change file, follows the real Finder error link, cancels once, and confirms
// recovery through the native form before editing again in the browser.
func TestBrowserConfigRecoveryFromBrokenFinder(t *testing.T) {
	_, err := exec.LookPath("agent-browser")
	require.NoError(t, err, "browser recovery acceptance requires agent-browser")
	mgr, schema, tree, _ := setupCLITerminalYANGTest(t)
	path := cli.ChangePath(mgr.configPath, "alice")
	require.NoError(t, mgr.store.WriteFile(path, []byte(corruptRecoveryContent), 0o600))
	committedBefore, err := mgr.committedConfig()
	require.NoError(t, err)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	sessions := NewSessionStore(nil)
	session, err := sessions.createSession("alice", authz.AuthResult{Authorizer: fakeAuthorizer{allowEdit: true}})
	require.NoError(t, err)
	recorder, err := audit.NewMemory(100)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", renderer.AssetHandler()))
	mux.Handle("GET /show/", HandleFragment(renderer, schema, tree, mgr, false, fakeAuthorizer{allowEdit: true}))
	mux.Handle("POST /config/set/", RequireSameOrigin(HandleConfigSetWithAuthorizer(mgr, schema, renderer, fakeAuthorizer{allowEdit: true})))
	mux.HandleFunc("GET /config/changes", HandleConfigChanges(mgr, renderer, fakeAuthorizer{allowEdit: true}))
	deps := RouteDeps{Renderer: renderer, Authorizer: fakeAuthorizer{allowEdit: true}, EditorManager: mgr, Recorder: recorder}
	for _, route := range RegisteredWebRoutes() {
		if !strings.HasSuffix(route.Pattern, " "+configRecoveryPath) {
			continue
		}
		handler := route.Build(deps)
		if route.Wrap == WrapMutation {
			handler = RequireSameOrigin(handler)
		}
		mux.Handle(route.Pattern, handler)
	}
	server := httptest.NewServer(authMiddleware(sessions, &recordingAuthenticator{}, noopRenderer, mux))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	browser := &downloadBrowser{t: t, ctx: ctx,
		session: fmt.Sprintf("ze-recovery-%d-%x", os.Getpid(), downloadBrowserSequence.Add(1))}
	t.Cleanup(func() {
		cancel()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		browser.ctx = closeCtx
		browser.command("close")
	})
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		browser.ctx = ctx
		t.Log("recovery browser failure state:", browser.evaluate(`JSON.stringify({path:location.pathname,text:document.body.innerText})`))
		t.Log("unreadable change file remains:", mgr.store.Exists(path))
	})
	// A native form's redirect needs a browser-owned cookie. Extra request
	// headers alone do not survive that navigation as an authenticated session.
	browser.command("cookies", "set", "ze-session", session.Token, "--url", server.URL, "--http-only", "--same-site", "Strict")
	browser.command("open", server.URL+"/show/bgp/")
	browser.wait(`!!document.getElementById('config-recovery-link')`, "clickable recovery from the real Finder failure")
	assert.NotContains(t, browser.evaluate(`document.body.textContent`), "private-unparseable-secret")
	browser.command("click", "#config-recovery-link")
	browser.wait(`!!document.getElementById('config-recovery-confirm')`, "independent recovery form")
	browser.command("click", "#config-recovery-cancel")
	browser.wait(`!!document.getElementById('config-recovery-link')`, "cancel preserves the broken candidate")
	kept, err := mgr.store.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, corruptRecoveryContent, string(kept))
	assert.Empty(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}))
	browser.command("click", "#config-recovery-link")
	browser.wait(`!!document.getElementById('config-recovery-confirm')`, "recovery can be reviewed again")
	browser.command("click", "#config-recovery-submit")
	require.Equal(t, "true", browser.evaluate(`location.pathname==='/config/recover'&&!document.getElementById('config-recovery-confirm').checked`),
		"native required checkbox prevents an unconfirmed submission")
	assert.True(t, mgr.store.Exists(path))
	browser.command("click", "#config-recovery-confirm")
	browser.command("click", "#config-recovery-submit")
	browser.wait(`!!document.getElementById('content-area')&&!document.getElementById('config-recovery')&&!document.getElementById('config-recovery-link')`,
		"confirmed recovery reopens the graphical editor")
	assert.False(t, mgr.store.Exists(path))
	committedAfter, err := mgr.committedConfig()
	require.NoError(t, err)
	assert.Equal(t, committedBefore, committedAfter)
	assert.Len(t, recorder.Query(audit.Filter{Action: audit.ActionConfigDiscard}), 1)
	browser.command("open", server.URL+"/show/bgp/")
	browser.wait(`!!document.getElementById('input-bgp-srouter--id')`, "recovered editor exposes its normal inputs")
	browser.command("fill", "#input-bgp-srouter--id", "192.0.2.210")
	browser.command("press", "Enter")
	browser.wait(`document.getElementById('commit-count').textContent.includes('1 pending change')`, "recovered editor accepts a new candidate edit")
	diff, err := mgr.Diff("alice")
	require.NoError(t, err)
	assert.Contains(t, diff, "192.0.2.210")
}
