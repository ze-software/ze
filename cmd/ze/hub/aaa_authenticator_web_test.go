//go:build ze_web

package hub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
)

// fixedAuthn authenticates exactly one user/password pair, returning the given
// profiles and source. It stands in for a RADIUS/TACACS or local backend in the
// live-bundle authenticator tests without needing bcrypt fixtures.
type fixedAuthn struct {
	user, pass string
	profiles   []string
	source     string
}

func (f fixedAuthn) Authenticate(req aaa.AuthRequest) (aaa.AuthResult, error) {
	if req.Username == f.user && req.Password == f.pass {
		return aaa.AuthResult{Authenticated: true, Profiles: f.profiles, Source: f.source}, nil
	}
	return aaa.AuthResult{Source: f.source}, aaa.ErrAuthRejected
}

func webAuthenticatorUnderTest() liveAAABundleAuthenticator {
	return liveAAABundleAuthenticator{
		fallback: fixedAuthn{user: "localadmin", pass: "localpw", profiles: []string{"admin"}, source: "local"},
	}
}

// VALIDATES: AC-2 -- before infra setup installs the AAA bundle (web starts
// before config load in the BGP path), web login authenticates via the static
// local fallback.
func TestWebAuthFallsBackWhenNoBundle(t *testing.T) {
	resetAAABundleForTest(t)
	auth := webAuthenticatorUnderTest()

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "localadmin", Password: "localpw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, "local", res.Source)
	assert.Equal(t, []string{"admin"}, res.Profiles)
}

// VALIDATES: AC-2 -- once the live AAA bundle is installed, web authenticates via
// its chain, so RADIUS/TACACS admins (here a chain-only user) can log in on web.
func TestWebAuthUsesChainWhenInstalled(t *testing.T) {
	resetAAABundleForTest(t)
	auth := webAuthenticatorUnderTest()

	bundle := &aaa.Bundle{Authenticator: fixedAuthn{user: "radiususer", pass: "radiuspw", profiles: []string{"admin"}, source: "radius"}}
	swapAAABundle(bundle, nil)

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "radiususer", Password: "radiuspw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, "radius", res.Source, "authentication must come from the live chain")
}

// VALIDATES: AC-2/A-3 -- with a bundle installed, a user the chain rejects still
// authenticates via the local fallback, so local users are never locked out when
// RADIUS/TACACS is configured.
func TestWebAuthLocalPreservedWithBundle(t *testing.T) {
	resetAAABundleForTest(t)
	auth := webAuthenticatorUnderTest()

	bundle := &aaa.Bundle{Authenticator: fixedAuthn{user: "radiususer", pass: "radiuspw", source: "radius"}}
	swapAAABundle(bundle, nil)

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "localadmin", Password: "localpw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)
	assert.Equal(t, "local", res.Source, "local users must still authenticate when a chain is installed")
}

// VALIDATES: AC-2 -- bad credentials are rejected whether or not a bundle exists.
func TestWebAuthRejectsBadCredentials(t *testing.T) {
	resetAAABundleForTest(t)
	auth := webAuthenticatorUnderTest()

	_, err := auth.Authenticate(aaa.AuthRequest{Username: "localadmin", Password: "wrong"})
	require.Error(t, err)

	bundle := &aaa.Bundle{Authenticator: fixedAuthn{user: "radiususer", pass: "radiuspw", source: "radius"}}
	swapAAABundle(bundle, nil)
	_, err = auth.Authenticate(aaa.AuthRequest{Username: "nobody", Password: "nope"})
	require.Error(t, err)
}

// bcryptUser builds a config user whose stored hash matches password. MinCost
// keeps the tests fast; the cost factor is not what is under test here.
func bcryptUser(t *testing.T, name, password string, profiles ...string) authz.UserConfig {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return authz.UserConfig{Name: name, Hash: string(hash), Profiles: profiles}
}

// runningConfig stands in for the shared ConfigProvider: a user list a test can
// rewrite the way an applied reload rewrites it, and an error a test can arm the
// way a broken read would.
type runningConfig struct {
	users []authz.UserConfig
	err   error
}

func (c *runningConfig) source() func() ([]authz.UserConfig, error) {
	return func() ([]authz.UserConfig, error) { return c.users, c.err }
}

func webAuthOver(cfg *runningConfig, powerUsers ...authz.UserConfig) liveAAABundleAuthenticator {
	return liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{
		UsersFunc: liveLocalUsers(powerUsers, cfg.source(), nil),
	}}
}

// VALIDATES: a web user the operator deletes stops authenticating as soon as the
// reload applies, with no AAA chain installed (the pre-bundle window).
// PREVENTS: a deleted account logging in until the daemon restarts.
func TestWebAuthRejectsUserRemovedFromRunningConfig(t *testing.T) {
	resetAAABundleForTest(t)
	cfg := &runningConfig{users: []authz.UserConfig{bcryptUser(t, "alice", "alicepw", "admin")}}
	auth := webAuthOver(cfg)

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.NoError(t, err, "alice must authenticate while the config declares her")
	require.True(t, res.Authenticated)

	// The reload that deletes her from the configuration.
	cfg.users = nil

	_, err = auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.Error(t, err, "a user the running config no longer declares must not authenticate")
}

// VALIDATES: the same deletion holds once a RADIUS/TACACS chain is installed.
// This is the path the defect lived on: the chain rejects a user it never knew,
// and the fallback used to answer from the boot snapshot, which still said yes.
// PREVENTS: the chain's rejection being overturned by a stale local copy.
func TestWebAuthRejectsRemovedUserWhenChainInstalled(t *testing.T) {
	resetAAABundleForTest(t)
	cfg := &runningConfig{users: []authz.UserConfig{bcryptUser(t, "alice", "alicepw", "admin")}}
	auth := webAuthOver(cfg)
	swapAAABundle(&aaa.Bundle{Authenticator: fixedAuthn{user: "radiususer", pass: "radiuspw", source: "radius"}}, nil)

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.NoError(t, err, "a config user the chain does not know must still authenticate")
	require.True(t, res.Authenticated)
	assert.Equal(t, "local", res.Source)

	cfg.users = nil

	_, err = auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.Error(t, err, "the chain rejected her and the config no longer declares her: both say no")
}

// VALIDATES: a user the reload KEEPS still authenticates, and a user the reload
// ADDS authenticates without a restart.
// PREVENTS: reading the running config per login from locking out live users, or
// from being a snapshot under another name.
func TestWebAuthFollowsConfigInBothDirections(t *testing.T) {
	resetAAABundleForTest(t)
	alice := bcryptUser(t, "alice", "alicepw", "admin")
	cfg := &runningConfig{users: []authz.UserConfig{alice}}
	auth := webAuthOver(cfg)

	// A reload that keeps alice and adds bob.
	cfg.users = []authz.UserConfig{alice, bcryptUser(t, "bob", "bobpw", "read-only")}

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.NoError(t, err, "a user the reload kept must still authenticate")
	assert.True(t, res.Authenticated)

	res, err = auth.Authenticate(aaa.AuthRequest{Username: "bob", Password: "bobpw"})
	require.NoError(t, err, "a user the reload added must authenticate without a restart")
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"read-only"}, res.Profiles)
}

// VALIDATES: when the running config cannot be read, the login is refused and
// the error reaches the caller.
// PREVENTS: an unreadable config degrading into "whatever was true at boot",
// which is the permissive branch (ai/rules/evidence.md).
func TestWebAuthDeniesWhenRunningConfigUnreadable(t *testing.T) {
	resetAAABundleForTest(t)
	alice := bcryptUser(t, "alice", "alicepw", "admin")
	cfg := &runningConfig{users: []authz.UserConfig{alice}}
	auth := webAuthOver(cfg)

	require.NoError(t, func() error {
		_, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
		return err
	}(), "alice authenticates while the config is readable")

	cfg.err = errors.New("config store unavailable")

	_, err := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.Error(t, err, "a guard that cannot read the config must deny")

	// The fallback used alone surfaces the read error itself rather than a bare
	// rejection, so the cause is visible to the caller.
	direct := &authz.LocalAuthenticator{UsersFunc: liveLocalUsers(nil, cfg.source(), nil)}
	_, derr := direct.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.ErrorContains(t, derr, "config store unavailable")
}

// VALIDATES: a configUsersAuthenticator with no user source wired denies and
// names the missing wiring.
// PREVENTS: a forgotten seam reading as "this config declares nobody", which is
// the same answer a correct empty config gives.
func TestWebAuthDeniesWithNoUserSourceWired(t *testing.T) {
	resetAAABundleForTest(t)
	auth := &authz.LocalAuthenticator{
		UsersFunc: liveLocalUsers([]authz.UserConfig{bcryptUser(t, "zeadmin", "zepw")}, nil, nil),
	}

	_, err := auth.Authenticate(aaa.AuthRequest{Username: "zeadmin", Password: "zepw"})
	require.ErrorIs(t, err, errNoLiveConfigProvider,
		"an unwired source is a fault to report, not an empty config to accept")
}

// VALIDATES: the zefs power user authenticates even when the config declares no
// users at all, and keeps doing so across a reload that empties the config.
// PREVENTS: reading config users per login locking the operator out of the box
// through the one credential a config reload cannot touch.
func TestWebAuthPowerUserSurvivesEmptyConfig(t *testing.T) {
	resetAAABundleForTest(t)
	cfg := &runningConfig{users: []authz.UserConfig{bcryptUser(t, "alice", "alicepw")}}
	auth := webAuthOver(cfg, bcryptUser(t, "zeadmin", "zepw", "admin"))

	cfg.users = nil

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "zeadmin", Password: "zepw"})
	require.NoError(t, err, "the zefs power user lives in the blob store; no config reload removes it")
	assert.True(t, res.Authenticated)
	assert.Equal(t, []string{"admin"}, res.Profiles)
}

// VALIDATES: a config user overrides a same-named zefs power user, which is what
// mergeAuthUsers promises, and the override follows the running config.
// PREVENTS: a stale power-user password outliving the config that replaced it.
func TestWebAuthConfigUserOverridesPowerUser(t *testing.T) {
	resetAAABundleForTest(t)
	cfg := &runningConfig{users: []authz.UserConfig{bcryptUser(t, "zeadmin", "configpw", "admin")}}
	auth := webAuthOver(cfg, bcryptUser(t, "zeadmin", "zefspw", "admin"))

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "zeadmin", Password: "configpw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated, "the config entry takes precedence over the zefs one")

	_, err = auth.Authenticate(aaa.AuthRequest{Username: "zeadmin", Password: "zefspw"})
	require.Error(t, err, "the overridden zefs password must not still work")
}

// VALIDATES: noConfigUsers reports an empty configuration, so a surface with no
// config loaded authenticates the zefs power user and nobody else.
// PREVENTS: web-only mode either failing closed on every login or inheriting
// users from a configuration it never loaded.
func TestNoConfigUsersLeavesPowerUserOnly(t *testing.T) {
	resetAAABundleForTest(t)
	auth := liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{
		UsersFunc: liveLocalUsers([]authz.UserConfig{bcryptUser(t, "zeadmin", "zepw")}, noConfigUsers, nil),
	}}

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "zeadmin", Password: "zepw"})
	require.NoError(t, err)
	assert.True(t, res.Authenticated)

	_, err = auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.Error(t, err, "no configuration is loaded, so no config user exists")
}

// VALIDATES: the AAA chain's own local backend follows the running config, so a
// deleted user is refused by the chain itself rather than by a later fallback.
// PREVENTS: the stale half of this defect. buildAAABundle builds the chain once
// per reactor creation and no reload rebuilds it, so a snapshot there is
// consulted BEFORE any fallback and overrules it: fixing only the fallback left
// the deleted user logging in.
func TestAAABundleLocalBackendFollowsRunningConfig(t *testing.T) {
	resetAAABundleForTest(t)
	cfg := &runningConfig{users: []authz.UserConfig{bcryptUser(t, "alice", "alicepw", "admin")}}

	bundle, err := buildAAABundle(nil, nil, liveLocalUsers(nil, cfg.source(), nil), nil, nil)
	require.NoError(t, err)
	swapAAABundle(bundle, nil)

	// No fallback at all, so only the chain can answer.
	auth := liveAAABundleAuthenticator{}

	res, aerr := auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.NoError(t, aerr, "the chain must authenticate a user the running config declares")
	require.True(t, res.Authenticated)

	cfg.users = nil

	_, aerr = auth.Authenticate(aaa.AuthRequest{Username: "alice", Password: "alicepw"})
	require.Error(t, aerr, "the chain must refuse a user the running config no longer declares")
}
