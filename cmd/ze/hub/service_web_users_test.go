//go:build ze_web

package hub

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/storage"
	zeweb "github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/pkg/zefs"
)

// VALIDATES: the web server derives BOTH the serve-or-not test and the live
// user view from the credentials its caller hands it, and reads the zefs
// database no second time.
// PREVENTS: the break-glass lockout. The hub reads zefs once and merges it into
// the closure the AAA chain answers from (liveLocalUsers, main.go). A web server
// that read zefs again could fail where that read succeeded: the power user
// authenticates through the chain, reports Source local, gets an anchored
// session, and then SessionStore.localUserDeclared does not find them, so every
// following request invalidates the session. That is a login loop on the one
// account that exists to recover a box.
func TestWebServerUsesTheCallersCredentialsWhenZefsIsUnreadable(t *testing.T) {
	// No database.zefs here, so any second read of the power user fails. This is
	// the divergence being removed, made total: the caller has the credentials
	// and the web server cannot get them for itself.
	setAPIConfigDir(t, t.TempDir())

	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "store.zefs"), dir)
	require.NoError(t, err, "blob storage")

	powerUsers := []authz.UserConfig{{Name: "admin", Hash: bcryptHash(t, "power-secret")}}
	live := func() ([]authz.UserConfig, error) { return powerUsers, nil }

	srv, broker := startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, false, "",
		nil, nil, nil, nil, nil, powerUsers, live, nil)
	require.NotNil(t, srv, "the web server must serve on the caller's power user; a second zefs read would report no authenticatable users and disable it")
	t.Cleanup(func() {
		if broker != nil {
			broker.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	require.NoError(t, srv.WaitReady(readyCtx))
	base := "https://" + srv.Address()
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // self-signed test certificate
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{"username": {"admin"}, "password": {"power-secret"}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/login", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "ze-session" {
			session = c
		}
	}
	require.NotNil(t, session, "the power user must be able to log in")

	// The second request is the one the lockout showed up on: the middleware
	// re-checks the anchored session against the live user list on EVERY
	// request, so a list that disagrees with the one that granted the session
	// refuses here and the operator loops on the login page forever.
	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/show/", http.NoBody)
	require.NoError(t, err)
	req.AddCookie(session)
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
		"the session the caller's credentials granted must survive its own next request")
}

// VALIDATES: the boot read of the zefs break-glass accounts reports its failure
// at a level the DEFAULT logger prints (slogutil.Logger defaults to WARN).
// PREVENTS: silence on the one account that recovers a box. The web factory used
// to print this on stderr when it read zefs for itself; that read is gone, so
// this site is the only producer left. At Debug a daemon with no api-server
// block and an unreadable database says nothing at all, and the operator meets
// the fault at the login prompt with no diagnostic anywhere.
func TestBootPowerUsersSaysSoWhenZefsIsUnreadable(t *testing.T) {
	// No database.zefs in this directory, so the read fails.
	setAPIConfigDir(t, t.TempDir())

	var out bytes.Buffer
	// slog.LevelWarn is what slogutil.Logger installs when no ze.log.* env var
	// selects otherwise, so this handler drops exactly what the daemon drops.
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))

	users := bootPowerUsers(log)

	assert.Empty(t, users, "an unreadable database declares no power user")
	assert.Contains(t, out.String(), "zefs power user unavailable",
		"an unreadable zefs database must be reported at a level the default logger prints")
}

// VALIDATES: an appliance built with `admin-disabled` boots SILENTLY. The flag
// declares no power user, so no read failed and nothing is broken.
// PREVENTS: a console instructed to repair the state the operator chose, on
// every boot, at the one level the default logger prints. The test above keeps
// a genuine read failure audible. The pair is why bootPowerUsers branches on
// the error instead of warning on all of them: with one Warn for both, the box
// that is working exactly as built is the noisiest one in the fleet.
func TestBootPowerUsersIsSilentWhenAdminIsDisabled(t *testing.T) {
	dir := t.TempDir()
	db, err := zefs.Create(filepath.Join(dir, "database.zefs"))
	require.NoError(t, err, "create zefs database")
	// Credentials present and readable: admin-disabled is the ONLY reason this
	// boot declares no power user, so a Warn here could come from nothing else.
	require.NoError(t, db.WriteFile(zefs.KeyLocalAdminUsername.Pattern, []byte("admin"), 0))
	require.NoError(t, db.WriteFile(zefs.KeyLocalAdminPassword.Pattern, []byte("$2y$10$hash"), 0))
	require.NoError(t, db.WriteFile(zefs.KeyInstanceAdminDisabled.Pattern, []byte("true"), 0))
	require.NoError(t, db.Close())
	setAPIConfigDir(t, dir)

	var out bytes.Buffer
	log := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}))

	users := bootPowerUsers(log)

	assert.Empty(t, users, "admin-disabled declares no power user")
	assert.Empty(t, out.String(),
		"a deliberate admin-disabled boot must not tell the operator to repair it")
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
//
// CAVEAT: the pipe is read only after fn RETURNS, so fn deadlocks if it writes
// more than the pipe buffer holds (64 KiB on Linux, 16 KiB on darwin). Both
// current callers write one short refusal line. A caller that captures a busy
// stderr needs a reader goroutine started before fn, not this helper.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err, "stderr pipe")
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	require.NoError(t, w.Close())
	data, readErr := io.ReadAll(r)
	require.NoError(t, readErr)
	require.NoError(t, r.Close())
	return string(data)
}

// VALIDATES: with no live user source wired, the web server refuses to serve and
// says why.
// PREVENTS: an unauthenticated admin UI reached by a nil seam. The power-user
// snapshot beside it NAMES accounts for the UI and answers nothing about who may
// log in, so treating it as the answer would serve a login the session check
// then refuses on every request.
func TestWebServerRefusesToServeWithNoUserSourceWired(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "store.zefs"), dir)
	require.NoError(t, err, "blob storage")

	powerUsers := []authz.UserConfig{{Name: "admin", Hash: bcryptHash(t, "power-secret")}}

	var srv *zeweb.WebServer
	var broker *zeweb.EventBroker
	stderr := captureStderr(t, func() {
		srv, broker = startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, false, "",
			nil, nil, nil, nil, nil, powerUsers, nil, nil)
	})
	if broker != nil {
		broker.Close()
	}

	assert.Nil(t, srv, "a nil user source must disable the server, not serve on the power-user snapshot beside it")
	assert.Contains(t, stderr, "no user source wired",
		"a guard that refuses must say why (ai/rules/evidence.md)")
}

// VALIDATES: when the live user source fails, the web server refuses to serve
// and reports the read error.
// PREVENTS: a read failure being read as "no users, so serve anyway" or as "some
// users, so serve". Neither is an answer: the source could not say who exists.
func TestWebServerRefusesToServeWhenTheUserSourceCannotBeRead(t *testing.T) {
	setAPIConfigDir(t, t.TempDir())
	dir := t.TempDir()
	store, err := storage.NewBlob(filepath.Join(dir, "store.zefs"), dir)
	require.NoError(t, err, "blob storage")

	powerUsers := []authz.UserConfig{{Name: "admin", Hash: bcryptHash(t, "power-secret")}}
	readErr := errors.New("running configuration is unreadable")
	//nolint:unparam // the shape is startWebServer's parameter; a read that fails names no users
	live := func() ([]authz.UserConfig, error) { return nil, readErr }

	var srv *zeweb.WebServer
	var broker *zeweb.EventBroker
	stderr := captureStderr(t, func() {
		srv, broker = startWebServer(store, "config.conf", []string{"127.0.0.1:0"}, false, "",
			nil, nil, nil, nil, nil, powerUsers, live, nil)
	})
	if broker != nil {
		broker.Close()
	}

	assert.Nil(t, srv, "a user source that cannot answer must disable the server")
	assert.Contains(t, stderr, "cannot read the running configuration users",
		"a guard that refuses must say why (ai/rules/evidence.md)")
	assert.Contains(t, stderr, readErr.Error(), "the refusal must carry the read failure it saw")
}
