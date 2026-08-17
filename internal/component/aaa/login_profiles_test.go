package aaa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: remote profiles are bound to the authentication result.
// PREVENTS: a later login for the same username changing this result's authority.
func TestProfileAuthorizerBindsRemoteProfiles(t *testing.T) {
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{result: AuthResult{
		Authenticated: true,
		Source:        "tacacs",
		Profiles:      []string{"read-only"},
	}}, probe)

	result, err := auth.Authenticate(AuthRequest{Username: "noc", Password: "pw"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"read-only"}, probe.bound)
	assert.NotNil(t, result.Authorizer)
}

// VALIDATES: a failed authentication carries no authorizer.
// PREVENTS: a rejected login receiving an authorization capability.
func TestProfileAuthorizerIgnoresFailure(t *testing.T) {
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{
		result: AuthResult{Authenticated: false, Profiles: []string{"admin"}},
		err:    ErrAuthRejected,
	}, probe)

	result, err := auth.Authenticate(AuthRequest{Username: "intruder", Password: "wrong"})
	assert.Error(t, err)
	assert.Nil(t, result.Authorizer)
	assert.Nil(t, probe.bound)
}

// VALIDATES: ordinary local users continue to use the live authorizer.
// PREVENTS: freezing a local config assignment at login.
func TestProfileAuthorizerKeepsOrdinaryLocalProfilesLive(t *testing.T) {
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{result: AuthResult{
		Authenticated: true,
		Source:        SourceLocal,
		Profiles:      []string{"read-only"},
	}}, probe)

	result, err := auth.Authenticate(AuthRequest{Username: "operator", Password: "pw"})
	assert.NoError(t, err)
	assert.Same(t, probe, result.Authorizer)
	assert.Nil(t, probe.bound)
}

// VALIDATES: a recovery grant is bound to the accepted credential generation.
// PREVENTS: an established recovery session retaining authority after reload.
func TestProfileAuthorizerExpiresLocalRecoveryGrant(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalProfileGeneration(0) })
	SetAcceptedLocalProfileGeneration(1)
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{result: AuthResult{
		Authenticated:   true,
		Source:          SourceLocal,
		Profiles:        []string{ReservedRecoveryProfile},
		LocalGeneration: 1,
	}}, probe)

	result, err := auth.Authenticate(AuthRequest{Username: "recovery", Password: "pw"})
	assert.NoError(t, err)
	assert.True(t, result.Authorizer.Authorize("recovery", "", "show version", true))
	typed, ok := result.Authorizer.(CommandArgsAuthorizer)
	require.True(t, ok)
	assert.True(t, typed.AuthorizeCommandArgs("recovery", "", "show", []string{"version"}, "", true))

	SetAcceptedLocalProfileGeneration(2)
	assert.False(t, result.Authorizer.Authorize("recovery", "", "show version", true))
	assert.False(t, typed.AuthorizeCommandArgs("recovery", "", "show", []string{"version"}, "", true))
}

// VALIDATES: a remote backend cannot mint the reserved recovery profile.
// PREVENTS: a TACACS+ or RADIUS reply granting local break-glass authority.
func TestProfileAuthorizerFiltersRemoteRecoveryProfile(t *testing.T) {
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{result: AuthResult{
		Authenticated: true,
		Source:        "tacacs",
		Profiles:      []string{"read-only", ReservedRecoveryProfile},
	}}, probe)

	result, err := auth.Authenticate(AuthRequest{Username: "remote-user", Password: "pw"})
	assert.NoError(t, err)
	assert.NotNil(t, result.Authorizer)
	assert.Equal(t, []string{"read-only"}, probe.bound)
}

// VALIDATES: the bound profile slice does not alias the backend's result.
// PREVENTS: backend slice reuse changing an established authorization view.
func TestProfileAuthorizerCopiesRemoteProfiles(t *testing.T) {
	profiles := []string{"read-only"}
	probe := &profileBindingProbe{}
	auth := WithProfileAuthorizer(&fakeBackend{result: AuthResult{
		Authenticated: true,
		Source:        "tacacs",
		Profiles:      profiles,
	}}, probe)

	_, err := auth.Authenticate(AuthRequest{Username: "noc", Password: "pw"})
	assert.NoError(t, err)
	profiles[0] = "admin"
	assert.Equal(t, []string{"read-only"}, probe.bound)
}

// VALIDATES: Build binds the composed authorizer to authentication results.
// PREVENTS: one transport bypassing result-scoped authorization.
func TestBuildWrapsAuthenticatorWithProfileAuthorizer(t *testing.T) {
	probe := &profileBindingProbe{}
	r := NewBackendRegistryForTest()
	err := r.Register(&fakeAuthBackend{
		name:       "probe",
		result:     AuthResult{Authenticated: true, Source: "probe", Profiles: []string{"read-only"}},
		authorizer: probe,
	})
	assert.NoError(t, err)

	bundle, err := r.Build(BuildParams{})
	assert.NoError(t, err)
	result, err := bundle.Authenticator.Authenticate(AuthRequest{Username: "built", Password: "pw"})
	assert.NoError(t, err)
	assert.NotNil(t, result.Authorizer)
	assert.Equal(t, []string{"read-only"}, probe.bound)
}

// VALIDATES: reserved-name usernames are rejected at AAA ingress.
// PREVENTS: a remote backend letting a client spoof a trusted identity.
func TestProfileAuthorizerRejectsReservedUsername(t *testing.T) {
	inner := &fakeBackend{result: AuthResult{Authenticated: true, Source: "fake", Profiles: []string{"admin"}}}
	auth := WithProfileAuthorizer(inner, &profileBindingProbe{})

	reserved := []string{
		ReservedInternalPrefix + "rpc",
		ReservedInternalPrefix + "plugin:evil",
		ReservedRecoveryProfile,
		ReservedSharedAPIUsername,
	}
	for _, name := range reserved {
		result, err := auth.Authenticate(AuthRequest{Username: name, Password: "x"})
		assert.ErrorIs(t, err, ErrAuthRejected, "reserved username %q must be rejected", name)
		assert.False(t, result.Authenticated, "reserved username %q must not authenticate", name)
	}
	assert.False(t, inner.called, "backend must not be consulted for a reserved username")

	result, err := auth.Authenticate(AuthRequest{Username: "alice", Password: "x"})
	assert.NoError(t, err)
	assert.True(t, result.Authenticated)
}

func TestLocalRecoveryAuthenticationRacingPublicationExpires(t *testing.T) {
	t.Cleanup(func() { SetAcceptedLocalProfileGeneration(0) })
	SetAcceptedLocalProfileGeneration(1)
	entered := make(chan struct{})
	release := make(chan struct{})
	auth := WithProfileAuthorizer(blockingAuthBackend{
		entered: entered,
		release: release,
		result: AuthResult{
			Authenticated:   true,
			Source:          SourceLocal,
			Profiles:        []string{ReservedRecoveryProfile},
			LocalGeneration: 1,
		},
	}, &profileBindingProbe{})
	done := make(chan AuthResult, 1)
	go func() {
		result, _ := auth.Authenticate(AuthRequest{Username: "racing-recovery", Password: "pw"})
		done <- result
	}()
	<-entered
	SetAcceptedLocalProfileGeneration(2)
	close(release)

	result := <-done
	assert.NotNil(t, result.Authorizer)
	assert.False(t, result.Authorizer.Authorize("racing-recovery", "", "show version", true))
}

type profileBindingProbe struct {
	bound []string
}

func (p *profileBindingProbe) Authorize(string, string, string, bool) bool {
	return true
}

func (p *profileBindingProbe) BindProfiles(profiles []string) Authorizer {
	p.bound = append([]string(nil), profiles...)
	return p
}

type blockingAuthBackend struct {
	entered chan<- struct{}
	release <-chan struct{}
	result  AuthResult
}

func (b blockingAuthBackend) Authenticate(AuthRequest) (AuthResult, error) {
	close(b.entered)
	<-b.release
	return b.result, nil
}

// fakeAuthBackend contributes an authenticator and optional authorizer.
type fakeAuthBackend struct {
	name       string
	result     AuthResult
	authorizer Authorizer
}

func (f *fakeAuthBackend) Name() string  { return f.name }
func (f *fakeAuthBackend) Priority() int { return 0 }
func (f *fakeAuthBackend) Build(BuildParams) (Contribution, error) {
	return Contribution{
		Authenticator: &fakeBackend{result: f.result},
		Authorizer:    f.authorizer,
	}, nil
}
