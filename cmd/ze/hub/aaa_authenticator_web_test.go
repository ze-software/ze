//go:build ze_web

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
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
