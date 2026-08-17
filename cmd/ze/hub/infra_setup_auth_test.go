package hub

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

type authWiringReactor struct {
	postStart  func()
	dispatcher *pluginserver.Dispatcher
}

func (r *authWiringReactor) SetPostStartFunc(fn func())           { r.postStart = fn }
func (r *authWiringReactor) Dispatcher() *pluginserver.Dispatcher { return r.dispatcher }
func (*authWiringReactor) Stop()                                  {}
func (*authWiringReactor) StopForRestart()                        {}

type authWiringSSHServer struct{}

func (authWiringSSHServer) Address() string { return "test" }

func authWiringBcryptUser(t *testing.T, name, password string, profiles ...string) aaa.UserCredential {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return aaa.UserCredential{Name: name, Hash: string(hash), Profiles: profiles}
}

type infraBootAuthenticator struct {
	source string
}

func (a *infraBootAuthenticator) Authenticate(aaa.AuthRequest) (aaa.AuthResult, error) {
	return aaa.AuthResult{Authenticated: true, Source: a.source}, nil
}

type infraBootAuthorizer struct {
	allow  bool
	closed *bool
}

func (a *infraBootAuthorizer) Authorize(_, _, _ string, _ bool) bool {
	return a.allow && (a.closed == nil || !*a.closed)
}

type infraBootAccountant struct {
	name            string
	closed          *bool
	starts          []string
	stops           []string
	stoppedOnClosed bool
}

func (a *infraBootAccountant) CommandStart(_, _, command string) string {
	a.starts = append(a.starts, command)
	return a.name + "-task"
}

func (a *infraBootAccountant) CommandStop(_, _, _, command string) {
	a.stops = append(a.stops, command)
	a.stoppedOnClosed = a.closed != nil && *a.closed
}

type infraBootBackend struct {
	name          string
	builds        *int
	buildErr      error
	authenticator aaa.Authenticator
	authorizer    aaa.Authorizer
	accountant    aaa.Accountant
	closed        *bool
}

func (b *infraBootBackend) Name() string { return b.name }
func (*infraBootBackend) Priority() int  { return 1 }
func (b *infraBootBackend) Build(aaa.BuildParams) (aaa.Contribution, error) {
	if b.builds != nil {
		(*b.builds)++
	}
	if b.buildErr != nil {
		return aaa.Contribution{}, b.buildErr
	}
	return aaa.Contribution{
		Authenticator: b.authenticator,
		Authorizer:    b.authorizer,
		Accountant:    b.accountant,
		Close: func() error {
			if b.closed != nil {
				*b.closed = true
			}
			return nil
		},
	}, nil
}

func buildInfraBootBundle(t *testing.T, backend aaa.Backend) *aaa.Bundle {
	t.Helper()
	registry := aaa.NewBackendRegistryForTest()
	require.NoError(t, registry.Register(backend))
	bundle, err := registry.Build(aaa.BuildParams{})
	require.NoError(t, err)
	return bundle
}

func TestInfraSetupUsesLiveUsersWithoutSSHSnapshot(t *testing.T) {
	resetAAABundleForTest(t)

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })

	operator := authWiringBcryptUser(t, "operator", "operator-secret", "readonly")
	liveCalls := 0
	liveUsers := func() ([]aaa.UserCredential, error) {
		liveCalls++
		return []aaa.UserCredential{operator}, nil
	}

	var builtUsers []aaa.UserCredential
	sshBuild = func(in *sshBuildInputs) sshServer {
		builtUsers = append([]aaa.UserCredential(nil), in.Users...)
		return authWiringSSHServer{}
	}

	server := infraSetup(infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	}, nil, nil, liveUsers)

	require.NotNil(t, server, "the seam stub proves infraSetup reached optional SSH construction")
	assert.Equal(t, 1, liveCalls, "infraSetup must resolve the shared live source exactly once for its boot snapshot")
	assert.Equal(t, []aaa.UserCredential{operator}, builtUsers,
		"optional SSH must receive the same shared list as AAA, not SSHExtractedConfig.Users")

	bundle := aaaBundle.Load()
	require.NotNil(t, bundle, "infraSetup must install AAA without an SSH-owned user snapshot")
	result, err := bundle.Authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "operator-secret"})
	require.NoError(t, err)
	assert.True(t, result.Authenticated)
}

func TestInfraSetupLiveUsersFailureFailsClosed(t *testing.T) {
	resetAAABundleForTest(t)

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })
	sshBuilt := false
	sshBuild = func(*sshBuildInputs) sshServer {
		sshBuilt = true
		return authWiringSSHServer{}
	}

	liveCalls := 0
	liveErr := errors.New("running config unavailable")
	liveUsers := func() ([]aaa.UserCredential, error) {
		liveCalls++
		return []aaa.UserCredential{{Name: "must-not-use"}}, liveErr
	}

	stderr := captureHubStderr(t, func() {
		_ = infraSetup(infra.HookParams{
			Reactor:   &authWiringReactor{},
			SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
		}, nil, nil, liveUsers)
	})

	assert.Contains(t, stderr, `msg="live local user source unavailable"`,
		"the infrastructure warning must preserve the operator-facing diagnostic")
	assert.Contains(t, stderr, `error="`+liveErr.Error()+`"`,
		"the infrastructure warning must preserve the original source error")
	assert.Equal(t, 1, liveCalls, "infraSetup must observe and report a failed shared-user read during construction")
	assert.False(t, sshBuilt, "optional SSH must not receive identities from a failed shared-user read")
	bundle := aaaBundle.Load()
	require.NotNil(t, bundle, "a failed local source must not remove other registered AAA backends")
	_, err := bundle.Authenticator.Authenticate(aaa.AuthRequest{Username: "operator", Password: "operator-secret"})
	require.Error(t, err, "an unreadable shared user source must not fall back to a stale local snapshot")
	assert.Equal(t, 1, liveCalls, "the failed source must not remain installed as a local authenticator callback")
}

// VALIDATES: the first infrastructure invocation builds and installs the
// daemon-owned AAA bundle exactly once.
// PREVENTS: an infrastructure hook reentry replacing boot authentication,
// authorization, or accounting backends.
func TestInfraSetupInitialBootBuildsAAABundleOnce(t *testing.T) {
	resetAAABundleForTest(t)

	originalDefault := aaa.Default
	t.Cleanup(func() { aaa.Default = originalDefault })

	builds := 0
	closed := false
	registry := aaa.NewBackendRegistryForTest()
	require.NoError(t, registry.Register(&infraBootBackend{
		name:          "initial",
		builds:        &builds,
		authenticator: &infraBootAuthenticator{source: "initial"},
		authorizer:    &infraBootAuthorizer{allow: true, closed: &closed},
		accountant:    &infraBootAccountant{name: "initial", closed: &closed},
		closed:        &closed,
	}))
	aaa.Default = registry

	params := infra.HookParams{Reactor: &authWiringReactor{}}
	_ = infraSetup(params, nil, nil, nil)
	first := aaaBundle.Load()
	require.NotNil(t, first)

	_ = infraSetup(params, nil, nil, nil)
	assert.Same(t, first, aaaBundle.Load())
	assert.Equal(t, 1, builds)
	assert.False(t, closed, "hook reentry must not close the boot-owned bundle")
}

// VALIDATES: no-BGP boot ownership crosses BGP auto-load without changing the
// installed bundle or closing any established backend state.
// PREVENTS: candidate bundle publication invalidating a stored TACACS
// authorizer or routing an in-flight STOP to a closed accountant.
func TestInfraSetupReentryReusesNoBGPBootBundle(t *testing.T) {
	resetAAABundleForTest(t)

	oldClosed := false
	oldAuthorizer := &infraBootAuthorizer{allow: true, closed: &oldClosed}
	oldAccountant := &infraBootAccountant{name: "boot", closed: &oldClosed}
	bootBundle := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		authorizer:    oldAuthorizer,
		accountant:    oldAccountant,
		closed:        &oldClosed,
	})
	swapAAABundle(bootBundle, nil)

	candidateBuilds := 0
	candidateClosed := false
	candidateAuthorizer := &infraBootAuthorizer{allow: false, closed: &candidateClosed}
	candidateAccountant := &infraBootAccountant{name: "candidate", closed: &candidateClosed}
	candidateRegistry := aaa.NewBackendRegistryForTest()
	require.NoError(t, candidateRegistry.Register(&infraBootBackend{
		name:          "candidate",
		builds:        &candidateBuilds,
		authenticator: &infraBootAuthenticator{source: "candidate"},
		authorizer:    candidateAuthorizer,
		accountant:    candidateAccountant,
		closed:        &candidateClosed,
	}))
	originalDefault := aaa.Default
	aaa.Default = candidateRegistry
	t.Cleanup(func() { aaa.Default = originalDefault })

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })
	var sshInputs *sshBuildInputs
	sshBuild = func(in *sshBuildInputs) sshServer {
		sshInputs = in
		return authWiringSSHServer{}
	}

	storedAuthorizer := aaa.BindProfiles(bootBundle.Authorizer, []string{"operator"})
	require.True(t, storedAuthorizer.Authorize("alice", "", "show version", true))
	liveAccountant := newLiveAAABundleAccountant()
	taskID := liveAccountant.CommandStart("alice", "198.51.100.8:2200", "show bgp")
	require.NotEmpty(t, taskID)

	dispatcher := pluginserver.NewDispatcher()
	r := &authWiringReactor{dispatcher: dispatcher}
	server := infraSetup(infra.HookParams{
		Reactor:   r,
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
		APIServer: func() *pluginserver.Server { return nil },
	}, nil, nil, nil)
	require.NotNil(t, server)
	require.NotNil(t, r.postStart)
	r.postStart()

	require.NotNil(t, sshInputs)
	assert.Equal(t, bootBundle.Authenticator, sshInputs.Authenticator)
	assert.Equal(t, bootBundle.Authorizer, sshInputs.Authorizer)
	assert.Same(t, bootBundle, aaaBundle.Load())
	assert.Zero(t, candidateBuilds, "BGP hook reentry must not build a candidate AAA bundle")
	assert.False(t, oldClosed, "the no-BGP boot bundle must remain live")
	assert.False(t, candidateClosed, "an unbuilt candidate must have no lifecycle side effects")
	assert.True(t, storedAuthorizer.Authorize("alice", "", "show version", true),
		"authorization captured before BGP auto-load must remain usable")

	liveAccountant.CommandStop(taskID, "alice", "198.51.100.8:2200", "show bgp")
	assert.Equal(t, []string{"show bgp"}, oldAccountant.starts)
	assert.Equal(t, []string{"show bgp"}, oldAccountant.stops)
	assert.False(t, oldAccountant.stoppedOnClosed,
		"an in-flight STOP must reach the still-open accountant that emitted START")

	const command = "test boot owned aaa"
	dispatcher.Register(command, func(*pluginserver.CommandContext, []string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, command)
	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username:   "alice",
		RemoteAddr: "198.51.100.8:2200",
	}, command)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, plugin.StatusDone, response.Status)
	assert.Equal(t, []string{"show bgp", command}, oldAccountant.starts)
	assert.Equal(t, []string{"show bgp", command}, oldAccountant.stops)
	assert.Empty(t, candidateAccountant.starts)
	assert.Empty(t, candidateAccountant.stops)
}

// VALIDATES: a failed no-BGP boot build still consumes AAA boot ownership.
// PREVENTS: later BGP auto-load retrying AAA construction with an unaccepted
// candidate configuration while the installed bundle pointer is nil.
func TestInfraSetupReentryDoesNotRetryFailedNoBGPAAABoot(t *testing.T) {
	resetAAABundleForTest(t)

	originalDefault := aaa.Default
	t.Cleanup(func() { aaa.Default = originalDefault })

	initialBuilds := 0
	initialErr := errors.New("initial AAA build failed")
	initialRegistry := aaa.NewBackendRegistryForTest()
	require.NoError(t, initialRegistry.Register(&infraBootBackend{
		name:          "failed-boot",
		builds:        &initialBuilds,
		buildErr:      initialErr,
		authenticator: &infraBootAuthenticator{source: "failed-boot"},
	}))
	aaa.Default = initialRegistry

	// Mirror the no-BGP boot path in runYANGConfig: attempt the build, register
	// accounting metrics, and record the nil result as the daemon's boot-owned
	// AAA state.
	bundle, err := buildAAABundle(nil, nil, nil, nil)
	require.ErrorIs(t, err, initialErr)
	require.Nil(t, bundle)
	registerAAAAccountingProvider(nil)
	swapAAABundle(nil, nil)

	candidateBuilds := 0
	candidateRegistry := aaa.NewBackendRegistryForTest()
	require.NoError(t, candidateRegistry.Register(&infraBootBackend{
		name:          "candidate",
		builds:        &candidateBuilds,
		authenticator: &infraBootAuthenticator{source: "candidate"},
		authorizer:    &infraBootAuthorizer{allow: true},
		accountant:    &infraBootAccountant{name: "candidate"},
	}))
	aaa.Default = candidateRegistry

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })
	sshBuilt := false
	sshBuild = func(*sshBuildInputs) sshServer {
		sshBuilt = true
		return authWiringSSHServer{}
	}

	_ = infraSetup(infra.HookParams{
		Reactor:   &authWiringReactor{},
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
	}, nil, nil, nil)

	assert.Equal(t, 1, initialBuilds)
	assert.Zero(t, candidateBuilds, "runtime hook reentry must not retry a failed boot build")
	assert.Nil(t, aaaBundle.Load())
	assert.False(t, sshBuilt, "a nil boot-owned bundle must keep SSH fail closed")
}
