// Detail: infra_setup.go -- the ssh build condition
// Detail: aaa_lifecycle.go -- liveAAABundleAuthorizer
// Related: internal/component/aaa/chain_survives_a_failed_backend_test.go --
// the chain behavior these tests rest on, proven at the layer that owns it

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config/infra"
)

// sshChainAuthn stands in for a composed chain: it answers for exactly one
// credential pair, so a result proves the bundle was consulted.
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

// sshBuiltBy runs infraSetup with the ssh seam stubbed and answers whether ssh
// was constructed, plus the inputs it received.
func sshBuiltBy(t *testing.T, params infra.HookParams) (bool, *sshBuildInputs) {
	t.Helper()
	original := sshBuild
	t.Cleanup(func() { sshBuild = original })

	var got *sshBuildInputs
	sshBuild = func(in *sshBuildInputs) sshServer {
		got = in
		return authWiringSSHServer{}
	}
	_ = infraSetup(params, nil, nil, nil)
	return got != nil, got
}

// TestSSHIsNotStartedWhenNothingComposed pins the "no user, no login" half of
// the owner's ruling of 2026-09-04 at the wiring.
//
// A nil bundle means NO backend built, so no account of any kind can
// authenticate. A listener that can authenticate nobody is a port and not a
// service, so ssh is not started.
//
// That state is rare and means what it says. A backend that will not build is
// dropped and the chain composes without it, which
// internal/component/aaa/chain_survives_a_failed_backend_test.go proves.
// Reaching a nil bundle takes EVERY backend failing.
func TestSSHIsNotStartedWhenNothingComposed(t *testing.T) {
	resetAAABundleForTest(t)

	// Consume the single boot attempt, so infraSetup skips the build and sees
	// the empty slot a total build failure leaves behind.
	require.True(t, claimAAABundleBoot(), "the test must own the boot claim before infraSetup asks for it")

	built, _ := sshBuiltBy(t, infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	})

	require.Nil(t, aaaBundle.Load(), "the boot claim was consumed, so no bundle is installed")
	assert.False(t, built, "no backend composed, so ssh must not accept connections it can never authenticate")
}

// TestSSHIsStartedWhenTheChainComposed is the other half, and it makes the test
// above a statement about the CHAIN rather than about ssh being fragile. With a
// bundle installed, ssh is built and its authenticator reads the live slot.
func TestSSHIsStartedWhenTheChainComposed(t *testing.T) {
	resetAAABundleForTest(t)
	swapAAABundle(&aaa.Bundle{Authenticator: sshChainAuthn{user: "opsadmin", pass: "opspw"}}, nil)

	built, inputs := sshBuiltBy(t, infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	})

	require.True(t, built, "a composed chain must get an ssh listener")
	require.NotNil(t, inputs)

	res, err := inputs.Authenticator.Authenticate(aaa.AuthRequest{Username: "opsadmin", Password: "opspw"})
	require.NoError(t, err, "the authenticator ssh receives must read the installed chain")
	assert.True(t, res.Authenticated)
}

// TestAuthorizationFailsClosedWhileTheBundleIsAbsent is the authorization half.
//
// Owner ruling, 2026-09-04: "we should fail close - no user no login". A
// fallback to the local RBAC policy was tried and reverted the same day,
// because a box that declares no system.authorization profile would then allow
// EVERY command while its chain was broken: falling back to a policy means
// falling back to what it says, and an absent one says allow.
//
// PREVENTS: a daemon that cannot build the chain its config describes becoming
// the daemon that authorizes most freely.
func TestAuthorizationFailsClosedWhileTheBundleIsAbsent(t *testing.T) {
	resetAAABundleForTest(t)

	// A local policy that ALLOWS. Even so, no bundle means no answer: this is
	// what tells fail-closed from a fallback that happened to refuse.
	policy := authz.NewStore()
	policy.AddProfile(authz.Profile{
		Name: "operator",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Allow},
	})
	policy.AssignProfiles("opsadmin", []string{"operator"})
	publishAcceptedLocalIdentity(&acceptedLocalIdentityState{authorizer: policy})

	authorizer := liveAAABundleAuthorizer{}
	require.Nil(t, aaaBundle.Load(), "the slot must be empty for the guard to be under test")

	assert.False(t, authorizer.Authorize("opsadmin", "", "show bgp", true),
		"the local policy allows this command, and a nil bundle must still refuse it")
	assert.False(t, authorizer.AuthorizeCommandArgs("opsadmin", "", "show bgp", nil, "", true),
		"both methods must answer alike")

	// An installed bundle restores the policy's own answer, so the guard is
	// about the missing chain and not about this operator.
	swapAAABundle(&aaa.Bundle{Authorizer: liveLocalAuthorizer{}}, nil)
	assert.True(t, authorizer.Authorize("opsadmin", "", "show bgp", true),
		"with a bundle installed the local policy decides again")
}
