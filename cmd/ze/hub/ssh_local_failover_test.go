// Detail: infra_setup.go -- the ssh build condition and its authenticator
// Related: aaa_authenticator_web_test.go -- the same contract on the web surface

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// sshLocalUser builds a config user whose stored hash matches password. The web
// suite has an identical helper, and it sits behind //go:build ze_web: this
// contract is an ssh one and must hold in a build with no web server.
func sshLocalUser(t *testing.T, name, password string) authz.UserConfig {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return authz.UserConfig{Name: name, Hash: string(hash), Profiles: []string{"admin"}}
}

// sshChainAuthn stands in for a built AAA chain: it answers for exactly one
// user, so a result carrying its source proves the bundle answered rather than
// the fallback.
type sshChainAuthn struct {
	user string
	pass string
}

func (c sshChainAuthn) Authenticate(req aaa.AuthRequest) (aaa.AuthResult, error) {
	if req.Username == c.user && req.Password == c.pass {
		return aaa.AuthResult{Authenticated: true, Profiles: []string{"admin"}, Source: "tacacs"}, nil
	}
	return aaa.AuthResult{Source: "tacacs"}, aaa.ErrAuthRejected
}

// TestSSHAuthenticatorFallsBackToLocalWhenTheBundleIsAbsent pins the contract
// the owner ruled on 2026-09-04: when the AAA chain the config describes cannot
// be built, management login MUST fail over to the local accounts.
//
// The BGP boot path built ssh with a bare liveAAABundleAuthenticator and gated
// the whole server on a non-nil bundle, so one mistyped AAA block took ssh away
// entirely on the path a router running BGP takes. The fallback is what the
// live indirection answers from while the bundle slot holds nothing.
//
// VALIDATES: local login answers while no AAA bundle is installed.
// PREVENTS: an operator locked out of a running daemon by a config error in a
// backend they could have repaired over ssh.
func TestSSHAuthenticatorFallsBackToLocalWhenTheBundleIsAbsent(t *testing.T) {
	resetAAABundleForTest(t)

	auth := liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{
		Users: []authz.UserConfig{sshLocalUser(t, "opsadmin", "opspw")},
	}}

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "opsadmin", Password: "opspw"})
	require.NoError(t, err, "the local account must answer while the bundle slot is empty")
	assert.True(t, res.Authenticated)
}

// TestSSHAuthenticatorPrefersTheBundleOverTheLocalFallback is the other half.
// The fallback is not a second chain: a repaired config installs a bundle and
// that bundle answers, so a TACACS+ or RADIUS operator logs in without a
// restart and local login stops being the answer.
//
// PREVENTS: the fallback becoming a permanent bypass of the configured backend.
func TestSSHAuthenticatorPrefersTheBundleOverTheLocalFallback(t *testing.T) {
	resetAAABundleForTest(t)

	auth := liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{
		Users: []authz.UserConfig{sshLocalUser(t, "opsadmin", "opspw")},
	}}
	swapAAABundle(&aaa.Bundle{Authenticator: sshChainAuthn{user: "tacacsuser", pass: "tacacspw"}}, nil)

	res, err := auth.Authenticate(aaa.AuthRequest{Username: "tacacsuser", Password: "tacacspw"})
	require.NoError(t, err, "the installed chain must answer for its own user")
	assert.True(t, res.Authenticated)
	assert.Equal(t, "tacacs", res.Source, "the bundle answered, not the fallback")
}

// TestSSHAuthenticatorRejectsAnUnknownUserWithNoBundle checks the failover does
// not widen who may log in. A user neither the chain nor the local accounts
// declare is refused, so the fallback restores the local accounts and nothing
// else.
func TestSSHAuthenticatorRejectsAnUnknownUserWithNoBundle(t *testing.T) {
	resetAAABundleForTest(t)

	auth := liveAAABundleAuthenticator{fallback: &authz.LocalAuthenticator{
		Users: []authz.UserConfig{sshLocalUser(t, "opsadmin", "opspw")},
	}}

	_, err := auth.Authenticate(aaa.AuthRequest{Username: "stranger", Password: "opspw"})
	require.Error(t, err, "no bundle and no local account declares this user")
}

// TestInfraSetupGivesSSHAnAuthenticatorThatFailsOver drives the WIRING, not the
// authenticator. The three tests above prove liveAAABundleAuthenticator falls
// back correctly; none of them proves infraSetup hands ssh one that CAN. That
// gap is the defect itself: the call site passed a bare indirection, and gated
// the whole server on a non-nil bundle so it never ran with one missing.
//
// The bundle slot is emptied after infraSetup returns rather than by forcing a
// build failure, because the backend that fails on a keyless server (tacacs)
// is not linked into an untagged test binary. The slot is what the live
// indirection reads, so an empty one is the same state a failed build leaves.
//
// VALIDATES: ssh is built with no bundle, and the authenticator infraSetup gave
// it answers for a local account when the slot is empty.
// PREVENTS: the BGP boot path skipping ssh entirely, which took management
// access away from a daemon that was otherwise running.
func TestInfraSetupGivesSSHAnAuthenticatorThatFailsOver(t *testing.T) {
	resetAAABundleForTest(t)

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })

	var built *sshBuildInputs
	sshBuild = func(in *sshBuildInputs) sshServer {
		built = in
		return authWiringSSHServer{}
	}

	operator := sshLocalUser(t, "opsadmin", "opspw")
	liveUsers := func() ([]aaa.UserCredential, error) {
		return []aaa.UserCredential{{Name: operator.Name, Hash: operator.Hash, Profiles: operator.Profiles}}, nil
	}

	_ = infraSetup(infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	}, nil, nil, liveUsers)

	require.NotNil(t, built, "ssh must be built: the operator repairs a broken AAA config over it")

	// The state a failed AAA build leaves behind.
	swapAAABundle(nil, nil)
	require.Nil(t, aaaBundle.Load())

	res, err := built.Authenticator.Authenticate(aaa.AuthRequest{Username: "opsadmin", Password: "opspw"})
	require.NoError(t, err, "ssh must fail over to the local accounts when the bundle slot is empty")
	assert.True(t, res.Authenticated)
}

// TestInfraSetupStartsSSHWhenTheBundleIsNil is the other half of the wiring,
// and the one the fallback test cannot reach: whether ssh is BUILT AT ALL with
// no bundle installed. Gating it on `bundle != nil` is what took ssh away.
//
// The nil state is reached by consuming the boot claim first. infraSetup builds
// the bundle only when it wins that claim, so a consumed claim leaves the slot
// exactly as a failed build does, in a binary that links no failing backend.
//
// VALIDATES: ssh is constructed with no AAA bundle installed.
// PREVENTS: an operator with no management access on a daemon whose forwarding
// plane is running normally.
func TestInfraSetupStartsSSHWhenTheBundleIsNil(t *testing.T) {
	resetAAABundleForTest(t)

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })

	built := false
	sshBuild = func(*sshBuildInputs) sshServer {
		built = true
		return authWiringSSHServer{}
	}

	// Consume the single boot attempt, so infraSetup skips the build and sees
	// the nil slot a failed build would have left.
	require.True(t, claimAAABundleBoot(), "the test must own the boot claim before infraSetup asks for it")

	_ = infraSetup(infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	}, nil, nil, func() ([]aaa.UserCredential, error) {
		operator := sshLocalUser(t, "opsadmin", "opspw")
		return []aaa.UserCredential{{Name: operator.Name, Hash: operator.Hash, Profiles: operator.Profiles}}, nil
	})

	require.Nil(t, aaaBundle.Load(), "the boot claim was consumed, so no bundle is installed")
	assert.True(t, built, "ssh must be built with no AAA bundle: it is how the operator repairs the config")
}
