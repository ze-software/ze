package authz

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// fakeBackend is a test Authenticator that returns configurable results.
type fakeBackend struct {
	result AuthResult
	err    error
	called bool
}

func (f *fakeBackend) Authenticate(AuthRequest) (AuthResult, error) {
	f.called = true
	return f.result, f.err
}

// VALIDATES: Authenticator interface exists and LocalAuthenticator implements it.
// PREVENTS: interface drift or missing method.
func TestAuthenticatorInterface(t *testing.T) {
	var _ Authenticator = (*LocalAuthenticator)(nil)
	var _ Authenticator = (*ChainAuthenticator)(nil)
}

// VALIDATES: AC-5 -- LocalAuthenticator wraps existing bcrypt logic unchanged.
// PREVENTS: regression in local auth after interface extraction.
func TestLocalAuthenticatorCompat(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)

	local := &LocalAuthenticator{
		Users: []UserConfig{
			{Name: "admin", Hash: string(hash), Profiles: []string{"admin"}},
		},
	}

	// Correct password succeeds.
	result, err := local.Authenticate(AuthRequest{Username: "admin", Password: "secret"})
	assert.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "local", result.Source)
	assert.Equal(t, []string{"admin"}, result.Profiles)

	// Wrong password returns ErrAuthRejected.
	result, err = local.Authenticate(AuthRequest{Username: "admin", Password: "wrong"})
	assert.ErrorIs(t, err, ErrAuthRejected)
	assert.False(t, result.Authenticated)

	// Unknown user returns ErrAuthRejected.
	result, err = local.Authenticate(AuthRequest{Username: "nobody", Password: "secret"})
	assert.ErrorIs(t, err, ErrAuthRejected)
	assert.False(t, result.Authenticated)

	// Empty username returns ErrAuthRejected.
	result, err = local.Authenticate(AuthRequest{Username: "", Password: "secret"})
	assert.ErrorIs(t, err, ErrAuthRejected)
	assert.False(t, result.Authenticated)
}

// VALIDATES: ChainAuthenticator tries backends in order, first success wins.
// PREVENTS: chain skipping backends or returning wrong result.
func TestChainAuthenticator(t *testing.T) {
	first := &fakeBackend{
		result: AuthResult{Authenticated: true, Source: "first", Profiles: []string{"admin"}},
	}
	second := &fakeBackend{
		result: AuthResult{Authenticated: true, Source: "second"},
	}

	chain := &ChainAuthenticator{Backends: []Authenticator{first, second}}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "first", result.Source)
	assert.True(t, first.called)
	assert.False(t, second.called, "second backend should not be called when first succeeds")
}

// VALIDATES: ChainAuthenticator falls through on connection error.
// PREVENTS: chain stopping on infrastructure failure instead of trying next.
func TestChainFallback(t *testing.T) {
	failing := &fakeBackend{
		err: fmt.Errorf("connection refused"),
	}
	local := &fakeBackend{
		result: AuthResult{Authenticated: true, Source: "local", Profiles: []string{"admin"}},
	}

	chain := &ChainAuthenticator{Backends: []Authenticator{failing, local}}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, "local", result.Source)
	assert.True(t, failing.called)
	assert.True(t, local.called)
}

// VALIDATES: ChainAuthenticator returns error when all backends fail.
// PREVENTS: silent auth success when no backend can authenticate.
func TestChainAllFail(t *testing.T) {
	first := &fakeBackend{err: fmt.Errorf("server down")}
	second := &fakeBackend{err: fmt.Errorf("server unreachable")}

	chain := &ChainAuthenticator{Backends: []Authenticator{first, second}}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.Error(t, err)
	assert.False(t, result.Authenticated)
	assert.True(t, first.called)
	assert.True(t, second.called)
}

// VALIDATES: AC-2 -- explicit rejection stops the chain immediately.
// PREVENTS: TACACS+ wrong-password falling through to local auth.
func TestChainRejectNoFallback(t *testing.T) {
	tacacs := &fakeBackend{
		result: AuthResult{Source: "tacacs"},
		err:    ErrAuthRejected,
	}
	local := &fakeBackend{
		result: AuthResult{Authenticated: true, Source: "local"},
	}

	chain := &ChainAuthenticator{Backends: []Authenticator{tacacs, local}}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "wrongpass"})

	assert.ErrorIs(t, err, ErrAuthRejected)
	assert.False(t, result.Authenticated)
	assert.Equal(t, "tacacs", result.Source)
	assert.True(t, tacacs.called)
	assert.False(t, local.called, "local MUST NOT be tried after explicit rejection")
}

// VALIDATES: ChainAuthenticator with no backends returns error.
// PREVENTS: nil pointer or silent pass with empty chain.
func TestChainNoBackends(t *testing.T) {
	chain := &ChainAuthenticator{}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.Error(t, err)
	assert.False(t, result.Authenticated)
}

// VALIDATES: ChainAuthenticator wraps last error.
// PREVENTS: losing error context from failing backends.
func TestChainWrapsLastError(t *testing.T) {
	connErr := fmt.Errorf("connection refused")
	first := &fakeBackend{err: connErr}

	chain := &ChainAuthenticator{Backends: []Authenticator{first}}
	_, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, connErr), "should wrap the connection error")
}

// VALIDATES: AC-3 — correct password passes bcrypt validation.
// PREVENTS: accepting wrong passwords or broken hash comparison.

func TestCheckPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	assert.True(t, CheckPassword(string(hash), "secret123"), "correct password should pass")
}

func TestCheckPasswordWrongPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	assert.False(t, CheckPassword(string(hash), "wrong"), "wrong password should fail")
}

func TestCheckPasswordEmptyHash(t *testing.T) {
	assert.False(t, CheckPassword("", "secret123"), "empty hash should fail")
}

func TestCheckPasswordEmptyPassword(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	assert.False(t, CheckPassword(string(hash), ""), "empty password should fail")
}

func TestAuthenticateUser(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin-pass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	users := []UserConfig{
		{Name: "admin", Hash: string(hash)},
		{Name: "operator", Hash: "invalid-not-bcrypt"},
	}

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"valid user and password", "admin", "admin-pass", true},
		{"valid user wrong password", "admin", "wrong", false},
		{"unknown user", "nobody", "admin-pass", false},
		{"user with invalid hash", "operator", "anything", false},
		{"empty username", "", "admin-pass", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuthenticateUser(users, tt.username, tt.password)
			assert.Equal(t, tt.want, got)
		})
	}
}

// VALIDATES: Bug 1 — reject all auth when no users configured.
// PREVENTS: unauthenticated SSH access when authentication block is omitted.
func TestAuthenticateUserNoUsersRejectsAll(t *testing.T) {
	var users []UserConfig // no users configured
	assert.False(t, AuthenticateUser(users, "admin", "password"), "should reject when no users configured")
	assert.False(t, AuthenticateUser(users, "root", "root"), "should reject any credentials")
	assert.False(t, AuthenticateUser(users, "", ""), "should reject empty credentials")
}

// VALIDATES: hash-as-token — sending the bcrypt hash itself authenticates.
// PREVENTS: ze cli unable to authenticate when zefs stores bcrypt hash.
func TestCheckPasswordHashAsToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	// Sending the hash itself should succeed (constant-time comparison).
	assert.True(t, CheckPassword(string(hash), string(hash)), "hash-as-token should pass")
}

// VALIDATES: duplicate user entries — auth tries all matching entries.
// PREVENTS: zefs user shadowed by config user with different hash.
func TestAuthenticateUserDuplicateEntries(t *testing.T) {
	hash1, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate hash1: %v", err)
	}
	hash2, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate hash2: %v", err)
	}
	// hash1 != hash2 (different salts) but both verify "pass".
	assert.NotEqual(t, string(hash1), string(hash2), "sanity: different salts produce different hashes")

	// User appears twice (zefs entry + config entry).
	users := []UserConfig{
		{Name: "admin", Hash: string(hash1)}, // zefs entry
		{Name: "admin", Hash: string(hash2)}, // config entry
	}

	// Sending hash1 as token should match the first entry.
	assert.True(t, AuthenticateUser(users, "admin", string(hash1)), "hash1 as token should match first entry")

	// Sending hash2 as token should match the second entry.
	assert.True(t, AuthenticateUser(users, "admin", string(hash2)), "hash2 as token should match second entry")

	// Plaintext should match via bcrypt on either entry.
	assert.True(t, AuthenticateUser(users, "admin", "pass"), "plaintext should match via bcrypt")
}

// VALIDATES: Bug 5 — timing-safe auth prevents username enumeration.
// PREVENTS: attackers distinguishing known from unknown usernames via response timing.
func TestAuthenticateUserTimingSafe(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate hash: %v", err)
	}

	users := []UserConfig{
		{Name: "admin", Hash: string(hash)},
	}

	// Time an unknown user auth attempt — should still invoke bcrypt (>10ms).
	start := time.Now()
	AuthenticateUser(users, "nonexistent", "anypassword")
	unknownDuration := time.Since(start)

	// Time a known user auth attempt with wrong password.
	start = time.Now()
	AuthenticateUser(users, "admin", "wrongpassword")
	knownDuration := time.Since(start)

	// Both should take a meaningful amount of time (bcrypt was invoked).
	// We use a generous threshold to avoid flaky tests, but bcrypt even at MinCost
	// takes >1ms, while a skip would be <1us.
	const minBcryptTime = 1 * time.Millisecond
	assert.Greater(t, unknownDuration, minBcryptTime,
		"unknown user should still invoke bcrypt (took %v)", unknownDuration)
	assert.Greater(t, knownDuration, minBcryptTime,
		"known user wrong password should invoke bcrypt (took %v)", knownDuration)
}
