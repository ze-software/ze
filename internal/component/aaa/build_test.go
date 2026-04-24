package aaa

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubAuthorizer records calls and returns a fixed verdict.
type stubAuthorizer struct {
	allow  bool
	called int
}

func (s *stubAuthorizer) Authorize(_, _, _ string, _ bool) bool {
	s.called++
	return s.allow
}

// stubAccountant records calls.
type stubAccountant struct {
	starts []string
	stops  []string
}

func (s *stubAccountant) CommandStart(_, _, command string) string {
	s.starts = append(s.starts, command)
	return "task-1"
}

func (s *stubAccountant) CommandStop(_, _, _, command string) {
	s.stops = append(s.stops, command)
}

// VALIDATES: AC-5 — Build with no backends registered returns an error.
// PREVENTS: silent pass when the hub forgot to register any backend.
func TestBundleComposeEmpty(t *testing.T) {
	r := NewBackendRegistry()
	_, err := r.Build(BuildParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication backend")
}

// VALIDATES: AC-6 — single backend producing an Authenticator yields a chain of length 1.
// PREVENTS: chain rejecting a lone backend.
func TestBundleComposeLocalOnly(t *testing.T) {
	r := NewBackendRegistry()
	local := &fakeBackend{result: AuthResult{Authenticated: true, Source: "local"}}
	require.NoError(t, r.Register(&stubBackend{
		name:    "local",
		contrib: Contribution{Authenticator: local},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)

	require.NotNil(t, bundle.Authenticator)
	result, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "u", Password: "p"})
	require.NoError(t, err)
	assert.Equal(t, "local", result.Source)
	assert.Nil(t, bundle.Authorizer, "no backend contributed an authorizer")
	assert.Nil(t, bundle.Accountant, "no backend contributed an accountant")
}

// VALIDATES: AC-7 — chain order follows Priority (tacacs before local).
// PREVENTS: reversed fallback (local first would swallow every auth).
func TestBundleComposeChainOrder(t *testing.T) {
	r := NewBackendRegistry()
	tacacs := &fakeBackend{err: ErrAuthRejected, result: AuthResult{Source: "tacacs"}}
	local := &fakeBackend{result: AuthResult{Authenticated: true, Source: "local"}}

	// Register in reverse priority on purpose — orderedBackends must sort.
	require.NoError(t, r.Register(&stubBackend{
		name: "local", priority: 200,
		contrib: Contribution{Authenticator: local},
	}))
	require.NoError(t, r.Register(&stubBackend{
		name: "tacacs", priority: 100,
		contrib: Contribution{Authenticator: tacacs},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)

	// tacacs rejects; chain MUST stop immediately per AC-2 semantics.
	_, err = bundle.Authenticator.Authenticate(AuthRequest{Username: "u", Password: "p"})
	assert.ErrorIs(t, err, ErrAuthRejected)
	assert.True(t, tacacs.called)
	assert.False(t, local.called, "local must not be tried after tacacs rejects")
}

// VALIDATES: AC-8 — the first non-nil Authorizer (lowest priority) is selected.
// PREVENTS: silently picking the wrong backend's authorizer.
func TestBundleAuthorizerSelection(t *testing.T) {
	r := NewBackendRegistry()
	tacacsAuthz := &stubAuthorizer{allow: false}
	localAuthz := &stubAuthorizer{allow: true}

	require.NoError(t, r.Register(&stubBackend{
		name: "tacacs", priority: 100,
		contrib: Contribution{Authenticator: &fakeBackend{}, Authorizer: tacacsAuthz},
	}))
	require.NoError(t, r.Register(&stubBackend{
		name: "local", priority: 200,
		contrib: Contribution{Authenticator: &fakeBackend{}, Authorizer: localAuthz},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)
	require.NotNil(t, bundle.Authorizer)

	allowed := bundle.Authorizer.Authorize("u", "", "c", true)
	assert.False(t, allowed, "tacacs authorizer should be picked")
	assert.Equal(t, 1, tacacsAuthz.called)
	assert.Equal(t, 0, localAuthz.called)
}

// VALIDATES: AC-9 — the first non-nil Accountant is selected.
// PREVENTS: two backends fighting over accounting.
func TestBundleAccountantSelection(t *testing.T) {
	r := NewBackendRegistry()
	tacacsAcct := &stubAccountant{}

	require.NoError(t, r.Register(&stubBackend{
		name: "tacacs", priority: 100,
		contrib: Contribution{Authenticator: &fakeBackend{}, Accountant: tacacsAcct},
	}))
	require.NoError(t, r.Register(&stubBackend{
		name: "local", priority: 200,
		contrib: Contribution{Authenticator: &fakeBackend{}},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)
	require.NotNil(t, bundle.Accountant)

	id := bundle.Accountant.CommandStart("u", "r", "show version")
	bundle.Accountant.CommandStop(id, "u", "r", "show version")
	assert.Equal(t, []string{"show version"}, tacacsAcct.starts)
	assert.Equal(t, []string{"show version"}, tacacsAcct.stops)
}

// VALIDATES: AC-13 — Bundle.Close() fans out to every contributed Close hook.
// PREVENTS: TacacsAccountant worker goroutine leaking on shutdown.
func TestBundleCloseFanOut(t *testing.T) {
	r := NewBackendRegistry()
	var stopped1, stopped2 bool

	require.NoError(t, r.Register(&stubBackend{
		name: "a", priority: 100,
		contrib: Contribution{
			Authenticator: &fakeBackend{},
			Close:         func() error { stopped1 = true; return nil },
		},
	}))
	require.NoError(t, r.Register(&stubBackend{
		name: "b", priority: 200,
		contrib: Contribution{
			Authenticator: &fakeBackend{},
			Close:         func() error { stopped2 = true; return nil },
		},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)

	require.NoError(t, bundle.Close())
	assert.True(t, stopped1, "backend a close should fire")
	assert.True(t, stopped2, "backend b close should fire")
}

// VALIDATES: Build surfaces factory errors rather than skipping the backend silently.
// PREVENTS: misconfigured tacacs being silently dropped, leaving only local.
func TestBundleBuildSurfacesFactoryError(t *testing.T) {
	r := NewBackendRegistry()
	require.NoError(t, r.Register(&stubBackend{
		name: "bad", err: errors.New("bad config"),
	}))

	_, err := r.Build(BuildParams{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bad")
	assert.Contains(t, err.Error(), "bad config")
}

// VALIDATES: a Backend that returns an empty Contribution is skipped, not rejected.
// PREVENTS: hub forced to keep unused factories happy with no-op configs.
func TestBundleEmptyContributionSkipped(t *testing.T) {
	r := NewBackendRegistry()
	require.NoError(t, r.Register(&stubBackend{
		name: "quiet", priority: 50,
		contrib: Contribution{}, // contributes nothing
	}))
	require.NoError(t, r.Register(&stubBackend{
		name: "loud", priority: 100,
		contrib: Contribution{Authenticator: &fakeBackend{}},
	}))

	bundle, err := r.Build(BuildParams{})
	require.NoError(t, err)
	require.NotNil(t, bundle.Authenticator)
}
