// Detail: types.go -- backendRegistry.Build, which composes the chain
// Related: chain.go -- ChainAuthenticator, the reject-versus-error rule

package aaa

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chainProbe answers for exactly one credential pair and rejects everything
// else, so a result's Source says which backend answered.
type chainProbe struct {
	source string
	user   string
	pass   string
}

func (p chainProbe) Authenticate(req AuthRequest) (AuthResult, error) {
	if req.Username == p.user && req.Password == p.pass {
		return AuthResult{Authenticated: true, Source: p.source}, nil
	}
	return AuthResult{Source: p.source}, ErrAuthRejected
}

// unreachableProbe stands in for a server that is configured and not answering.
// Its error is NOT ErrAuthRejected, so the chain must try the next backend.
type unreachableProbe struct{ source string }

func (p unreachableProbe) Authenticate(AuthRequest) (AuthResult, error) {
	return AuthResult{Source: p.source}, errors.New("dial tcp: connection refused")
}

// TestChainSurvivesABackendThatWillNotBuild is the whole point of this file.
//
// A backend whose Build fails is DROPPED and the rest of the chain composes
// without it. Build used to return on the first error, which took the LOCAL
// backend down with the broken one: local sits at priority 200, so a TACACS+
// server declared with no shared secret left the daemon with no authenticator
// at all rather than with the local accounts the chain exists to fall back to.
//
// VALIDATES: a local user still authenticates when a higher-priority backend
// cannot be built, and the error naming the dropped backend is still returned.
// PREVENTS: one mistyped AAA block taking every account on the box with it.
func TestChainSurvivesABackendThatWillNotBuild(t *testing.T) {
	registry := NewBackendRegistryForTest()
	require.NoError(t, registry.Register(&stubBackend{
		name:     "tacacs",
		priority: 100,
		err:      errors.New("no shared secret configured for this TACACS+ server"),
	}))
	require.NoError(t, registry.Register(&stubBackend{
		name:     SourceLocal,
		priority: 200,
		contrib:  Contribution{Authenticator: chainProbe{source: SourceLocal, user: "opsadmin", pass: "opspw"}},
	}))

	bundle, err := registry.Build(BuildParams{})

	require.Error(t, err, "the dropped backend must still be reported, so a reload can refuse on it")
	assert.Contains(t, err.Error(), "tacacs", "the error must name the backend that was dropped")
	require.NotNil(t, bundle, "the chain that composed must be returned, not discarded with the broken backend")

	result, authErr := bundle.Authenticator.Authenticate(AuthRequest{Username: "opsadmin", Password: "opspw"})
	require.NoError(t, authErr, "the local account must authenticate with the tacacs backend dropped")
	assert.True(t, result.Authenticated)
	assert.Equal(t, SourceLocal, result.Source)
}

// TestChainWithNoBackendLeftBuildsNothing is the other edge, and it is what
// "no user, no login" rests on. When every backend was dropped there is no
// chain to fall back within, so the caller gets no bundle. A nil bundle
// authenticates nobody and ssh is not started (cmd/ze/hub/infra_setup.go).
func TestChainWithNoBackendLeftBuildsNothing(t *testing.T) {
	registry := NewBackendRegistryForTest()
	require.NoError(t, registry.Register(&stubBackend{
		name: "tacacs", priority: 100, err: errors.New("no shared secret"),
	}))
	require.NoError(t, registry.Register(&stubBackend{
		name: SourceLocal, priority: 200, err: errors.New("unreadable user list"),
	}))

	bundle, err := registry.Build(BuildParams{})

	require.Error(t, err)
	assert.Nil(t, bundle, "nothing composed, so nothing may authenticate")
	assert.Contains(t, err.Error(), "tacacs", "every dropped backend is named")
	assert.Contains(t, err.Error(), SourceLocal)
	assert.Contains(t, err.Error(), "no authentication backend configured")
}

// TestTheLocalBackendAnswersOnlyAfterTheRemoteOneFails states the rule the
// owner asked for, at the layer that owns it: with a user in BOTH places, the
// remote backend is asked first and the local one answers only when the remote
// could not.
//
// The distinction is reject versus unreachable, and it is the whole security
// property. A wrong password at a REACHABLE server must not fall through to the
// local hash, because the server said no and the box would be saying yes.
func TestTheLocalBackendAnswersOnlyAfterTheRemoteOneFails(t *testing.T) {
	build := func(remote Authenticator) *Bundle {
		t.Helper()
		registry := NewBackendRegistryForTest()
		require.NoError(t, registry.Register(&stubBackend{
			name: "tacacs", priority: 100, contrib: Contribution{Authenticator: remote},
		}))
		require.NoError(t, registry.Register(&stubBackend{
			name:     SourceLocal,
			priority: 200,
			contrib:  Contribution{Authenticator: chainProbe{source: SourceLocal, user: "dual", pass: "localpw"}},
		}))
		bundle, err := registry.Build(BuildParams{})
		require.NoError(t, err)
		require.NotNil(t, bundle)
		return bundle
	}

	t.Run("the remote server answers, so the local hash is never consulted", func(t *testing.T) {
		bundle := build(chainProbe{source: "tacacs", user: "dual", pass: "remotepw"})

		result, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "dual", Password: "remotepw"})
		require.NoError(t, err)
		assert.Equal(t, "tacacs", result.Source, "the reachable server owns this login")
	})

	t.Run("the remote server REJECTS, so the chain stops and the local hash is not tried", func(t *testing.T) {
		bundle := build(chainProbe{source: "tacacs", user: "dual", pass: "remotepw"})

		_, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "dual", Password: "localpw"})
		require.Error(t, err, "the local password must NOT open a session the central server refused")
	})

	t.Run("the remote server is unreachable, so the local hash answers", func(t *testing.T) {
		bundle := build(unreachableProbe{source: "tacacs"})

		result, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "dual", Password: "localpw"})
		require.NoError(t, err, "an unreachable server must fall through to the local account")
		assert.Equal(t, SourceLocal, result.Source)
	})

	t.Run("the remote server is unreachable and the local password is wrong, so nobody logs in", func(t *testing.T) {
		bundle := build(unreachableProbe{source: "tacacs"})

		_, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "dual", Password: "guessed"})
		require.Error(t, err, "the fallback is the local ACCOUNT, never an open door")
	})
}
