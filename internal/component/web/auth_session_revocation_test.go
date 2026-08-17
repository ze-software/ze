package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// localGrant is the result the local backend returns for an authenticated user:
// it names itself on AuthResult.Source, and that name is what anchors a session
// to the local user list.
func localGrant() authz.AuthResult {
	return authz.AuthResult{Authenticated: true, Source: aaa.SourceLocal}
}

// reloadDuringLogin authenticates through inner and then rewrites the live user
// list, reproducing a reload that lands AFTER the authenticator has answered and
// BEFORE the login handler creates the session. The two are one statement apart
// on the same request, so the window is small and real.
type reloadDuringLogin struct {
	inner authz.Authenticator
	live  *liveUserList
	after []authz.UserConfig
}

func (r reloadDuringLogin) Authenticate(request authz.AuthRequest) (authz.AuthResult, error) {
	result, err := r.inner.Authenticate(request)
	r.live.set(r.after...)

	return result, err
}

// liveUserList is a user list a test can rewrite between requests, standing in
// for the running configuration a reload rewrites. It reports an error when
// failRead is set, which is what an unreadable configuration looks like to the
// session store.
type liveUserList struct {
	mu       sync.Mutex
	users    []authz.UserConfig
	failRead bool
	reads    int

	// beforeRead runs on every read, before the list is taken and outside this
	// list's own mutex, so a test can hold a reader inside validateToken while
	// something else reaches the store.
	beforeRead func()
}

func (l *liveUserList) read() ([]authz.UserConfig, error) {
	l.mu.Lock()
	l.reads++
	hook := l.beforeRead
	l.mu.Unlock()

	if hook != nil {
		hook()
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.failRead {
		return nil, errors.New("config unreadable")
	}

	return l.users, nil
}

func (l *liveUserList) setBeforeRead(hook func()) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.beforeRead = hook
}

// set replaces the whole list, the way an applied reload replaces the users the
// running configuration declares.
func (l *liveUserList) set(users ...authz.UserConfig) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.users = users
}

func (l *liveUserList) setFailRead(fail bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failRead = fail
}

// revocationUsers returns alice and bob, both with the bcrypt hash of
// "testpass".
func revocationUsers(t *testing.T) []authz.UserConfig {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	require.NoError(t, err)

	return []authz.UserConfig{
		{Name: "alice", Hash: string(hash)},
		{Name: "bob", Hash: string(hash)},
	}
}

// loginForCookie drives the real login handler and returns the ze-session
// cookie value it issued. It fails the test when no session was created, so a
// later 401 can only mean the cookie was refused rather than never issued.
func loginForCookie(t *testing.T, handler http.HandlerFunc, username, password string) string {
	t.Helper()

	form := url.Values{"username": {username}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close() //nolint:errcheck // cookies only

	for _, c := range resp.Cookies() {
		if c.Name == "ze-session" {
			return c.Value
		}
	}
	t.Fatalf("login for %q issued no ze-session cookie (status %d)", username, resp.StatusCode)

	return ""
}

// getWithCookie returns the status a protected route gives this session cookie.
func getWithCookie(handler http.Handler, token string) int {
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ze-session", Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec.Code
}

// TestSessionCookieFollowsTheRunningConfig drives the production shape: the real
// login handler issues a cookie, the real middleware serves it, the user list is
// rewritten the way an applied reload rewrites it, and the same cookie comes
// back on the next request.
// VALIDATES: AC-10 (a removed user's cookie is refused at once), AC-12 (a kept
// user's session survives with no churn).
// PREVENTS: the cookie branch of AuthMiddlewareWithAudit staying a bypass of the
// user list. It tested only the 24h TTL, so a deleted operator kept full
// config-edit rights in an open tab for the rest of the day.
func TestSessionCookieFollowsTheRunningConfig(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	authenticator := &authz.LocalAuthenticator{UsersFunc: live.read}
	store := NewSessionStore(live.read)

	login := loginHandler(store, authenticator, noopRenderer)
	protected := authMiddleware(store, authenticator, noopRenderer, okHandler())

	aliceToken := loginForCookie(t, login, "alice", "testpass")
	bobToken := loginForCookie(t, login, "bob", "testpass")

	require.Equal(t, http.StatusOK, getWithCookie(protected, aliceToken),
		"alice's cookie must work while the config declares her, or the refusal below proves nothing")
	require.Equal(t, http.StatusOK, getWithCookie(protected, bobToken),
		"bob's cookie must work while the config declares him")

	// The reload: alice is removed, bob is kept.
	live.set(revocationUsers(t)[1])

	assert.Equal(t, http.StatusUnauthorized, getWithCookie(protected, aliceToken),
		"a cookie whose user the running config no longer declares must be refused")
	assert.Equal(t, http.StatusOK, getWithCookie(protected, bobToken),
		"a user the reload keeps must not be logged out")

	// The refusal is a decision, not a coincidence: the store dropped the
	// session, so a replay of the same cookie stays refused.
	assert.Nil(t, store.validateToken(aliceToken),
		"the refused session must be invalidated, not merely denied once")
	assert.NotNil(t, store.validateToken(bobToken),
		"the kept user's session must survive intact")
}

// TestSessionOfRemoteBackendUserSurvivesLocalRemoval covers the regression the
// obvious fix would cause. A RADIUS or TACACS+ operator never appears in the
// config's user list, so re-checking every session against that list would log
// them all out on their next request.
// VALIDATES: AC-12 for a user the local list never declared.
// PREVENTS: turning "deleted config users lose their session" into "every
// remote-backend operator loses their session".
func TestSessionOfRemoteBackendUserSurvivesLocalRemoval(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	store := NewSessionStore(live.read)

	// radiususer is authenticated by a remote backend and is absent from the
	// local list, exactly as a RADIUS admin is.
	session, err := store.createSession("radiususer", authz.AuthResult{
		Authenticated: true,
		Profiles:      []string{"admin"},
		Source:        "radius",
	})
	require.NoError(t, err)
	assert.False(t, session.LocalAnchored,
		"a user the local list does not declare must not be anchored to it")

	live.set()

	got := store.validateToken(session.Token)
	require.NotNil(t, got, "the local list cannot revoke a session it never granted")
	assert.Equal(t, "radiususer", got.Username)
}

// TestSessionRefusedWhenLiveUserListUnreadable checks the guard fails closed.
// VALIDATES: AC-10's failure path -- an unreadable user list is not an empty
// one, and it is not a pass either.
// PREVENTS: a config read error reading as "no change", which would keep every
// session alive exactly when the daemon cannot tell who exists.
func TestSessionRefusedWhenLiveUserListUnreadable(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	store := NewSessionStore(live.read)

	session, err := store.createSession("alice", localGrant())
	require.NoError(t, err)
	require.True(t, session.LocalAnchored)
	require.NotNil(t, store.validateToken(session.Token))

	live.setFailRead(true)

	assert.Nil(t, store.validateToken(session.Token),
		"a session granted by the local user list must not be renewed against a list that cannot be read")
}

// TestSessionAnchoredWhenAReloadLandsDuringLogin is the case the old design
// could not pass. It recorded the anchor by RE-READING the live list inside
// createSession, which is a second question asked after the authenticator had
// already answered. A reload landing in that window answered "the config does
// not declare alice", so a session the local backend had just granted was
// recorded as un-revocable and survived the full 24h TTL -- the guard failing
// open on exactly the event it exists to catch.
// VALIDATES: AC-10 when the removal is concurrent with the login.
// PREVENTS: re-deriving the anchor instead of reporting the authenticator's own
// answer (AuthResult.Source).
func TestSessionAnchoredWhenAReloadLandsDuringLogin(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	local := &authz.LocalAuthenticator{UsersFunc: live.read}
	store := NewSessionStore(live.read)

	// The reload keeps bob and removes alice, and lands after her password has
	// been checked and before her session exists.
	racing := reloadDuringLogin{inner: local, live: live, after: revocationUsers(t)[1:]}

	login := loginHandler(store, racing, noopRenderer)
	protected := authMiddleware(store, local, noopRenderer, okHandler())

	token := loginForCookie(t, login, "alice", "testpass")

	assert.Equal(t, http.StatusUnauthorized, getWithCookie(protected, token),
		"a session the local backend granted must stay revocable when the removal lands during login")
	assert.Nil(t, store.validateToken(token),
		"the session must be invalidated, not merely denied once")
}

// TestSessionOfRemoteBackendUserSurvivesWhenTheLocalListDeclaresThemToo covers
// the other direction of the same fault. One name can exist in the local list
// AND in a RADIUS or TACACS+ directory, and the chain tries the remote backend
// first. Anchoring on list MEMBERSHIP made such a session revocable by a list
// that never authenticated it, so deleting the local entry logged out an
// operator the remote backend had admitted.
// VALIDATES: AC-12 for a name both backends know.
// PREVENTS: membership standing in for the grant.
func TestSessionOfRemoteBackendUserSurvivesWhenTheLocalListDeclaresThemToo(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	store := NewSessionStore(live.read)

	// The remote backend authenticated alice, who is also in the local list.
	remote := &recordingAuthenticator{
		result: authz.AuthResult{Authenticated: true, Source: "tacacs"},
	}

	login := loginHandler(store, remote, noopRenderer)
	protected := authMiddleware(store, remote, noopRenderer, okHandler())

	token := loginForCookie(t, login, "alice", "irrelevant")

	live.set()

	assert.Equal(t, http.StatusOK, getWithCookie(protected, token),
		"the local list must not revoke a session the remote backend granted, even to a name it also declares")
}

// TestSessionStoreWithoutLiveSourceRefusesALocalSession pins what a nil user
// source means: the store cannot say whether the user is still declared, so it
// refuses the sessions the local backend granted rather than serving what it
// cannot check.
// VALIDATES: the documented meaning of NewSessionStore(nil).
// PREVENTS: an unwired store reading as "nothing to check" and keeping every
// local session alive for the full TTL.
func TestSessionStoreWithoutLiveSourceRefusesALocalSession(t *testing.T) {
	store := NewSessionStore(nil)

	session, err := store.createSession("alice", localGrant())
	require.NoError(t, err)
	require.True(t, session.LocalAnchored,
		"the grant is the authenticator's answer and does not depend on the store having a list")

	assert.Nil(t, store.validateToken(session.Token),
		"a store with no live user list must refuse a session only that list could renew")
}

// TestValidateTokenRefusalLeavesANewerSessionAlone covers the replay window.
// A tab still polling with a revoked cookie reaches validateToken after the
// operator has re-added the user and that user has logged in again. Deleting by
// USERNAME then destroyed the new session on behalf of the old one.
// VALIDATES: invalidation is scoped to the token that failed.
// PREVENTS: a stale request logging out a live session.
func TestValidateTokenRefusalLeavesANewerSessionAlone(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	store := NewSessionStore(live.read)

	stale, err := store.createSession("alice", localGrant())
	require.NoError(t, err)

	// alice is removed, so the stale cookie is about to be refused.
	live.set(revocationUsers(t)[1])

	// Hold that refusal inside validateToken, after it has read the session and
	// before it invalidates anything. The list read runs without the store's
	// lock on purpose (it reaches out of this package), so this is the daemon's
	// own interleaving rather than an invented one.
	var once sync.Once
	reached := make(chan struct{})
	release := make(chan struct{})
	live.setBeforeRead(func() {
		once.Do(func() {
			close(reached)
			<-release
		})
	})

	refused := make(chan *webSession, 1)
	go func() { refused <- store.validateToken(stale.Token) }()

	<-reached

	// alice logs in again while the old request is in flight, through a remote
	// backend the local removal does not govern.
	fresh, err := store.createSession("alice", authz.AuthResult{Authenticated: true, Source: "tacacs"})
	require.NoError(t, err)
	close(release)

	assert.Nil(t, <-refused, "the stale cookie must still be refused")
	assert.NotNil(t, store.validateToken(fresh.Token),
		"a refused cookie must not invalidate the session created after it")
}

// TestValidateTokenReadsTheListPerRequest proves the check is not cached. A
// store that read the list once would answer from the configuration the daemon
// booted with, which is the whole defect.
// VALIDATES: AC-10's "no restart and no wait for the TTL".
func TestValidateTokenReadsTheListPerRequest(t *testing.T) {
	live := &liveUserList{users: revocationUsers(t)}
	store := NewSessionStore(live.read)

	session, err := store.createSession("alice", localGrant())
	require.NoError(t, err)

	before := live.reads
	for range 3 {
		require.NotNil(t, store.validateToken(session.Token))
	}
	assert.Equal(t, before+3, live.reads,
		"every request must ask the live list, so a reload lands without a restart")
}
