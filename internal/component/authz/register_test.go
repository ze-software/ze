package authz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
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

	result, err := contrib.Authenticator.Authenticate("admin", "secret")
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
