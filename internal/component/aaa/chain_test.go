package aaa

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeBackend is a test Authenticator that returns configurable results.
type fakeBackend struct {
	result  AuthResult
	err     error
	called  bool
	request AuthRequest
}

func (f *fakeBackend) Authenticate(request AuthRequest) (AuthResult, error) {
	f.called = true
	f.request = request
	return f.result, f.err
}

// VALIDATES: ChainAuthenticator implements Authenticator.
// PREVENTS: interface drift.
func TestAuthenticatorInterface(t *testing.T) {
	var _ Authenticator = (*ChainAuthenticator)(nil)
}

// VALIDATES: chain tries backends in order, first success wins.
// PREVENTS: chain skipping backends or returning wrong result.
func TestChainFirstSuccessWins(t *testing.T) {
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

// VALIDATES: chain falls through on connection error.
// PREVENTS: chain stopping on infrastructure failure instead of trying next.
func TestChainFallthroughOnError(t *testing.T) {
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

// VALIDATES: chain returns error when all backends fail.
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

// VALIDATES: explicit rejection stops the chain immediately (AC-7 semantics).
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

// VALIDATES: empty chain returns error.
// PREVENTS: nil pointer or silent pass.
func TestChainNoBackends(t *testing.T) {
	chain := &ChainAuthenticator{}
	result, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.Error(t, err)
	assert.False(t, result.Authenticated)
}

// VALIDATES: chain wraps last connection error.
// PREVENTS: losing error context from failing backends.
func TestChainWrapsLastError(t *testing.T) {
	connErr := fmt.Errorf("connection refused")
	first := &fakeBackend{err: connErr}

	chain := &ChainAuthenticator{Backends: []Authenticator{first}}
	_, err := chain.Authenticate(AuthRequest{Username: "user", Password: "pass"})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, connErr), "should wrap the connection error")
}

// VALIDATES: richer auth request metadata is forwarded unchanged through the chain.
// PREVENTS: SSH remote address or service being dropped before the TACACS backend sees them.
func TestChainAuthenticatorForwardsAuthRequest(t *testing.T) {
	backend := &fakeBackend{
		result: AuthResult{Authenticated: true, Source: "local"},
	}
	request := AuthRequest{
		Username:   "user",
		Password:   "pass",
		RemoteAddr: "203.0.113.5:2222",
		Service:    "ssh",
	}

	chain := &ChainAuthenticator{Backends: []Authenticator{backend}}
	result, err := chain.Authenticate(request)

	assert.NoError(t, err)
	assert.True(t, result.Authenticated)
	assert.Equal(t, request, backend.request)
}
