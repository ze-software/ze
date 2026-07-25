package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ze-software/ze/internal/component/aaa"
)

// VALIDATES: local backend Name + Priority.
// PREVENTS: priority drift that would put local before tacacs in the chain.
func TestLocalBackendIdentity(t *testing.T) {
	b := localBackend{}
	assert.Equal(t, "local", b.Name())
	assert.Equal(t, 200, b.Priority())
}

// VALIDATES: Build returns a LocalAuthenticator bound to params.LocalUsers.
// PREVENTS: factory ignoring configured users.
func TestLocalBackendBuildPropagatesUsers(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	params := aaa.BuildParams{
		LocalUsers: []aaa.UserCredential{
			{Name: "admin", Hash: string(hash), Profiles: []string{"admin"}},
		},
	}

	contrib, err := localBackend{}.Build(params)
	require.NoError(t, err)
	require.NotNil(t, contrib.Authenticator)

	result, err := contrib.Authenticator.Authenticate(aaa.AuthRequest{Username: "admin", Password: "secret"})
	require.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "local", result.Source)
	assert.Equal(t, []string{"admin"}, result.Profiles)
}

// VALIDATES: local backend is self-registered with aaa.Default by init().
// PREVENTS: blank-import wired up but init() not firing.
func TestLocalBackendSelfRegistered(t *testing.T) {
	// aaa.Default may have been built already by other tests in this binary;
	// registering again would fail. We just confirm a fresh registry accepts
	// the same factory without error (identity path).
	r := aaa.Default
	require.NotNil(t, r, "aaa.Default must exist after init")
	// Registering a duplicate should fail — proves init() already ran.
	err := r.Register(localBackend{})
	assert.Error(t, err, "init() already registered local; second Register must fail")
}

// VALIDATES: StoreAuthorizer preserves peer-scoped typed dispatch semantics.
// PREVENTS: typed inter-plugin authorization dropping the peer scope keyword.
func TestStoreAuthorizerAuthorizeCommandArgsPeerScoped(t *testing.T) {
	store := NewStore()
	store.AddProfile(Profile{
		Name: "ops",
		Run: Section{
			Default: Deny,
			Entries: []Entry{{Number: 10, Action: Allow, Match: "^peer show bgp rib$", Regex: true}},
		},
	})
	store.AssignProfiles("operator", []string{"ops"})

	auth := StoreAuthorizer{Store: store}
	assert.True(t, auth.AuthorizeCommandArgs("operator", "", "show bgp rib", nil, "10.0.0.1", true))
	assert.False(t, auth.AuthorizeCommandArgs("operator", "", "show bgp rib", nil, "", true))
}

// TestStoreAuthorizerNilStoreAllowsAll pins the "no authorization configured"
// default: extractAuthzConfig returns a NIL store when system.authorization is
// absent or defines no profiles, and a nil StoreAuthorizer must allow everything.
// This is the branch that keeps an un-configured box fully permissive AFTER the
// fail-closed change to Store.Authorize -- the permissive default lives here, one
// layer above Authorize, never in a fall-through inside it.
//
// VALIDATES: spec-fixit-authz-admin-fallthrough -- no-authz box stays allow-all.
// PREVENTS: the fail-closed Store.Authorize change bricking an un-configured box.
func TestStoreAuthorizerNilStoreAllowsAll(t *testing.T) {
	auth := StoreAuthorizer{Store: nil}
	assert.True(t, auth.Authorize("anyone", "", "restart", true))
	assert.True(t, auth.Authorize("", "", "router bgp", false))
	assert.True(t, auth.AuthorizeCommandArgs("anyone", "", "show bgp rib", nil, "10.0.0.1", true))
}
