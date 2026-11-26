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

// VALIDATES: StoreAuthorizer.AuthorizeLocalScope evaluates mixed device rules
// locally and fails closed when Store is nil.
// PREVENTS: a nil StoreAuthorizer silently acting as wildcard fleet scope.
func TestStoreAuthorizerAuthorizeLocalScope(t *testing.T) {
	store := NewStore()
	store.AddProfile(Profile{
		Name: "fleet-west",
		Run: Section{
			Default: Deny,
			Entries: []Entry{
				{Number: 10, Action: Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("operator", []string{"fleet-west"})

	edgeCmd, edgeArgs := aaa.FleetDeviceReadTokens("edge-01")
	coreCmd, coreArgs := aaa.FleetDeviceReadTokens("core-01")

	auth := StoreAuthorizer{Store: store}
	assert.True(t, auth.AuthorizeLocalScope("operator", edgeCmd, edgeArgs, true))
	assert.False(t, auth.AuthorizeLocalScope("operator", coreCmd, coreArgs, true))
	assert.False(t, auth.AuthorizeLocalScope("unassigned", edgeCmd, edgeArgs, true))

	nilAuth := StoreAuthorizer{Store: nil}
	assert.False(t, nilAuth.AuthorizeLocalScope("operator", edgeCmd, edgeArgs, true))
}

// VALIDATES: boundStoreAuthorizer.AuthorizeLocalScope uses only the bound
// profile list and fails closed on nil store, empty profiles, or unknown names.
// PREVENTS: a remote session with an unmapped or unknown profile gaining scope.
func TestBoundStoreAuthorizerAuthorizeLocalScope(t *testing.T) {
	store := NewStore()
	store.AddProfile(Profile{
		Name: "admin",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Allow},
	})
	store.AddProfile(Profile{
		Name: "fleet-west",
		Run: Section{
			Default: Deny,
			Entries: []Entry{
				{Number: 10, Action: Allow, Match: "^show fleet device name edge-.*$", Regex: true},
			},
		},
	})
	store.AssignProfiles("alice", []string{"admin"})

	edgeCmd, edgeArgs := aaa.FleetDeviceReadTokens("edge-01")
	coreCmd, coreArgs := aaa.FleetDeviceReadTokens("core-01")

	base := StoreAuthorizer{Store: store}
	bound := base.BindProfiles([]string{"fleet-west"})
	allowed, supported := aaa.AuthorizeLocalScope(bound, "alice", edgeCmd, edgeArgs, true)
	assert.True(t, supported)
	assert.True(t, allowed)

	allowed, supported = aaa.AuthorizeLocalScope(bound, "alice", coreCmd, coreArgs, true)
	assert.True(t, supported)
	assert.False(t, allowed, "bound profile must override static username admin assignment")

	unknownBound := base.BindProfiles([]string{"does-not-exist"})
	allowed, supported = aaa.AuthorizeLocalScope(unknownBound, "alice", edgeCmd, edgeArgs, true)
	assert.True(t, supported)
	assert.False(t, allowed, "unknown remote profile name must fail closed")

	emptyBound := base.BindProfiles(nil)
	allowed, supported = aaa.AuthorizeLocalScope(emptyBound, "alice", edgeCmd, edgeArgs, true)
	assert.True(t, supported)
	assert.False(t, allowed, "empty bound profiles must fail closed")

	nilStoreBound := (StoreAuthorizer{Store: nil}).BindProfiles([]string{"fleet-west"})
	allowed, supported = aaa.AuthorizeLocalScope(nilStoreBound, "alice", edgeCmd, edgeArgs, true)
	assert.True(t, supported)
	assert.False(t, allowed, "missing local store on bound profile must fail closed")
}
