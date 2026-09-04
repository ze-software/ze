package hub

import (
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
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
	// No closed pointer ON THE AUTHORIZER, deliberately. One that denied once
	// its own bundle was retired would answer false after the swap below
	// whether ssh followed the slot or not, and the assertion could not tell a
	// live indirection from a captured field.
	oldAuthorizer := &infraBootAuthorizer{allow: true}
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
	// ssh receives the LIVE indirections over the atomic bundle slot, so what
	// has to be asserted is where they RESOLVE, not what they are. The boot
	// chain reports source "boot" and its authorizer allows; the candidate's
	// reports "candidate" and denies, so a candidate installed here is visible
	// in both answers. Comparing the values only re-stated the wiring line, and
	// it stopped being able to tell the two bundles apart at all.
	require.NotNil(t, sshInputs.Authenticator)
	sshResult, sshErr := sshInputs.Authenticator.Authenticate(aaa.AuthRequest{Username: "alice"})
	require.NoError(t, sshErr)
	assert.True(t, sshResult.Authenticated)
	assert.Equal(t, "boot", sshResult.Source, "ssh must authenticate through the boot bundle's chain")
	require.NotNil(t, sshInputs.Authorizer)
	assert.True(t, sshInputs.Authorizer.Authorize("alice", "", "show version", true),
		"ssh must authorize through the boot bundle's authorizer, which allows where the candidate's denies")
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

	// Everything above holds for a captured bundle.Authenticator too, which is
	// what ssh used to receive. Only a bundle the ssh values could not have
	// captured tells the two wirings apart: install one and ask again.
	replacementClosed := false
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "replacement",
		authenticator: &infraBootAuthenticator{source: "replacement"},
		authorizer:    &infraBootAuthorizer{allow: false, closed: &replacementClosed},
		closed:        &replacementClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, oldClosed, "installing a chain retires the one it replaces")

	reloadedResult, reloadedErr := sshInputs.Authenticator.Authenticate(aaa.AuthRequest{Username: "alice"})
	require.NoError(t, reloadedErr)
	assert.True(t, reloadedResult.Authenticated)
	assert.Equal(t, "replacement", reloadedResult.Source,
		"ssh must authenticate through the chain installed NOW, not the one boot built")
	assert.False(t, sshInputs.Authorizer.Authorize("alice", "", "show version", true),
		"ssh must authorize against the chain installed NOW, which denies where the boot one allowed")
}

// VALIDATES: the no-BGP daemon hands standalone SSH live indirections over the
// bundle slot, and hands nil when boot built no bundle.
// PREVENTS: the no-BGP path capturing bundle.Authenticator, which is the defect
// the BGP path carried: a server started at boot kept authenticating against the
// RADIUS shared secret the operator had rotated.
func TestNoBGPAAAWiringFollowsTheInstalledBundle(t *testing.T) {
	resetAAABundleForTest(t)

	absentAuthenticator, absentAuthorizer := noBGPAAAWiring(nil)
	assert.Nil(t, absentAuthenticator, "a failed boot build must leave ssh on local users")
	assert.Nil(t, absentAuthorizer)

	bootClosed := false
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		authorizer:    &infraBootAuthorizer{allow: true},
		closed:        &bootClosed,
	})
	swapAAABundle(boot, nil)
	t.Cleanup(func() { closeAAABundle(nil) })

	authenticator, authorizer := noBGPAAAWiring(boot)
	require.NotNil(t, authenticator)
	require.NotNil(t, authorizer)

	booted, err := authenticator.Authenticate(aaa.AuthRequest{Username: "alice"})
	require.NoError(t, err)
	assert.Equal(t, "boot", booted.Source)
	assert.True(t, authorizer.Authorize("alice", "", "show version", true))

	replacementClosed := false
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "replacement",
		authenticator: &infraBootAuthenticator{source: "replacement"},
		authorizer:    &infraBootAuthorizer{allow: false, closed: &replacementClosed},
		closed:        &replacementClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, bootClosed, "installing a chain retires the one it replaces")

	reloaded, err := authenticator.Authenticate(aaa.AuthRequest{Username: "alice"})
	require.NoError(t, err)
	assert.Equal(t, "replacement", reloaded.Source,
		"the value standalone ssh receives must resolve through the bundle slot")
	assert.False(t, authorizer.Authorize("alice", "", "show version", true))
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
	// REVERSED on 2026-09-04 by owner ruling: failover to the local user when
	// AAA fails MUST be the documented behavior. This row used to require the
	// opposite, that a nil boot-owned bundle leaves ssh unbuilt, and called it
	// failing closed.
	//
	// Skipping ssh is not failing closed. It removes the surface the operator
	// repairs the broken AAA config over, while the daemon keeps forwarding.
	// Authorization still denies on a nil bundle (liveAAABundleAuthorizer), and
	// the authenticator ssh receives falls back to the local accounts, which is
	// what the other two management surfaces already did.
	assert.True(t, sshBuilt, "a nil boot-owned bundle must still start ssh, which fails over to the local accounts")
}

// postStartDispatcherForTest installs a boot bundle, runs infraSetup and its
// post-start wiring, and returns the dispatcher that wiring configured.
func postStartDispatcherForTest(t *testing.T, boot *aaa.Bundle) *pluginserver.Dispatcher {
	t.Helper()
	swapAAABundle(boot, nil)
	t.Cleanup(func() { closeAAABundle(nil) })

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })
	sshBuild = func(*sshBuildInputs) sshServer { return authWiringSSHServer{} }

	dispatcher := pluginserver.NewDispatcher()
	r := &authWiringReactor{dispatcher: dispatcher}
	infraSetup(infra.HookParams{
		Reactor:   r,
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
		APIServer: func() *pluginserver.Server { return nil },
	}, nil, nil, nil)
	require.NotNil(t, r.postStart)
	r.postStart()
	return dispatcher
}

// registerPostStartCommand registers one always-succeeding command.
func registerPostStartCommand(d *pluginserver.Dispatcher, name string) {
	d.Register(name, func(*pluginserver.CommandContext, []string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, name)
}

// VALIDATES: the post-start dispatcher authorizes against the AAA bundle
// installed RIGHT NOW, so a reload that replaces the TACACS+ authorizer decides
// the next command.
// PREVENTS: the third absent call site of this defect class. SetPostStartFunc
// runs once and no reload re-enters it, so an authorizer captured there keeps
// deciding against a server the operator has decommissioned.
func TestPostStartDispatcherAuthorizesAgainstTheInstalledBundle(t *testing.T) {
	resetAAABundleForTest(t)

	bootClosed := false
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		// No closed pointer ON THE AUTHORIZER, deliberately. A fake that
		// observes its own bundle's close would deny after the swap whether
		// the dispatcher followed it or not, and the test could not tell the
		// two apart. This one keeps ALLOWING once retired, so a denial can
		// only come from the reloaded bundle.
		authorizer: &infraBootAuthorizer{allow: true},
		closed:     &bootClosed,
	})
	dispatcher := postStartDispatcherForTest(t, boot)

	const command = "test dispatcher authorization"
	registerPostStartCommand(dispatcher, command)
	ctx := &pluginserver.CommandContext{Username: "alice", RemoteAddr: "198.51.100.8:2200"}

	allowed, err := dispatcher.Dispatch(ctx, command)
	require.NoError(t, err)
	require.NotNil(t, allowed)
	assert.Equal(t, plugin.StatusDone, allowed.Status, "the installed bundle allows")

	replacementClosed := false
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "replacement",
		authenticator: &infraBootAuthenticator{source: "replacement"},
		authorizer:    &infraBootAuthorizer{allow: false, closed: &replacementClosed},
		closed:        &replacementClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, bootClosed, "the swap must close the bundle it replaces")

	refused, err := dispatcher.Dispatch(ctx, command)
	require.ErrorIs(t, err, pluginserver.ErrUnauthorized,
		"the dispatcher must decide against the reloaded bundle, which denies")
	require.NotNil(t, refused)
	assert.Equal(t, plugin.StatusError, refused.Status)
}

// VALIDATES: the post-start dispatcher accounts to the AAA bundle installed
// RIGHT NOW, so a reload that replaces the TACACS+ accountant receives the
// records for every later command.
// PREVENTS: accounting vanishing in SILENCE. The swap's Close stops the retired
// accountant's worker, and a captured hook then enqueues onto it: the send path
// drops the record and returns no error, so nothing anywhere reports the loss.
func TestPostStartDispatcherAccountsToTheInstalledBundle(t *testing.T) {
	resetAAABundleForTest(t)

	bootClosed := false
	bootAccountant := &infraBootAccountant{name: "boot", closed: &bootClosed}
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		// Allows even once retired, so this test measures ACCOUNTING alone. An
		// authorizer that denied after its bundle closed would stop the second
		// dispatch before it ever reached the accounting hook.
		authorizer: &infraBootAuthorizer{allow: true},
		accountant: bootAccountant,
		closed:     &bootClosed,
	})
	dispatcher := postStartDispatcherForTest(t, boot)

	const command = "test dispatcher accounting"
	registerPostStartCommand(dispatcher, command)
	ctx := &pluginserver.CommandContext{Username: "alice", RemoteAddr: "198.51.100.8:2200"}

	first, err := dispatcher.Dispatch(ctx, command)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, first.Status)
	require.Equal(t, []string{command}, bootAccountant.starts)
	require.Equal(t, []string{command}, bootAccountant.stops)

	replacementClosed := false
	replacementAccountant := &infraBootAccountant{name: "replacement", closed: &replacementClosed}
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "replacement",
		authenticator: &infraBootAuthenticator{source: "replacement"},
		authorizer:    &infraBootAuthorizer{allow: true},
		accountant:    replacementAccountant,
		closed:        &replacementClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, bootClosed, "the swap must close the bundle it replaces")

	second, err := dispatcher.Dispatch(ctx, command)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, second.Status)

	assert.Equal(t, []string{command}, replacementAccountant.starts,
		"the record must reach the installed accountant")
	assert.Equal(t, []string{command}, replacementAccountant.stops)
	assert.Equal(t, []string{command}, bootAccountant.starts,
		"the stopped accountant must receive nothing after the swap")
	assert.Equal(t, []string{command}, bootAccountant.stops)
	assert.False(t, bootAccountant.stoppedOnClosed,
		"no record may reach an accountant whose worker the swap has already stopped: it drops them and reports nothing")
	assert.False(t, replacementAccountant.stoppedOnClosed)
}

// VALIDATES: the post-start dispatcher consults the accepted LOCAL policy when
// boot could not build an AAA bundle, and refuses what that policy refuses.
// PREVENTS: authorization failing OPEN on the BGP path. Dispatcher.isAuthorized
// authorizes every command while its authorizer is nil
// (internal/component/plugin/server/command.go), so a wiring that installs no
// authorizer at all is silently allow-all. That is what this pins: an
// authorizer IS installed and IS consulted.
//
// REVISED on 2026-09-04 by owner ruling: a failed AAA build falls back to the
// local RBAC policy. This test used to declare no policy and require a denial,
// which is no longer the contract -- with no policy the fallback is the
// daemon's no-RBAC allow mode, the same answer an installed bundle with no
// authorizer gives. So the test now declares a policy that DENIES, which proves
// more than the old shape did: the authorizer is present, it is reached, and
// the operator's own rules decide.
func TestPostStartDispatcherDeniesWhenTheAAABootBuildFailed(t *testing.T) {
	resetAAABundleForTest(t)

	originalDefault := aaa.Default
	t.Cleanup(func() { aaa.Default = originalDefault })
	failing := aaa.NewBackendRegistryForTest()
	require.NoError(t, failing.Register(&infraBootBackend{
		name:     "failed-boot",
		buildErr: errors.New("AAA backend build failed"),
	}))
	aaa.Default = failing

	originalSSHBuild := sshBuild
	t.Cleanup(func() { sshBuild = originalSSHBuild })
	sshBuild = func(*sshBuildInputs) sshServer { return authWiringSSHServer{} }

	// The accepted LOCAL policy, which is what the fallback consults. It denies
	// every operational command for alice, so a refusal below can only have
	// come from this store being reached.
	denyAll := authz.NewStore()
	denyAll.AddProfile(authz.Profile{
		Name: "locked",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Deny},
	})
	denyAll.AssignProfiles("alice", []string{"locked"})
	publishAcceptedLocalIdentity(&acceptedLocalIdentityState{authorizer: denyAll})

	dispatcher := pluginserver.NewDispatcher()
	r := &authWiringReactor{dispatcher: dispatcher}
	_ = infraSetup(infra.HookParams{
		Reactor:   r,
		SSHConfig: infra.SSHExtractedConfig{HasConfig: true},
		APIServer: func() *pluginserver.Server { return nil },
	}, nil, nil, nil)
	require.Nil(t, aaaBundle.Load(), "the boot build must have failed")
	require.NotNil(t, r.postStart, "post-start wiring must be registered whatever boot produced")
	r.postStart()

	const command = "test denied without aaa"
	registerPostStartCommand(dispatcher, command)
	refused, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username:   "alice",
		RemoteAddr: "198.51.100.8:2200",
	}, command)
	require.ErrorIs(t, err, pluginserver.ErrUnauthorized,
		"the local policy denies alice, and a nil AAA bundle must consult it rather than allow by default")
	require.NotNil(t, refused)
	assert.Equal(t, plugin.StatusError, refused.Status)
}

// VALIDATES: a reload that ADDS accounting reaches the dispatcher on a daemon
// whose boot bundle carried no accountant.
// PREVENTS: the accounting hook being installed only when BOOT found one, which
// left every later addition of TACACS+ accounting unwired until a restart.
func TestPostStartDispatcherAccountsAfterAReloadAddsAnAccountant(t *testing.T) {
	resetAAABundleForTest(t)

	bootClosed := false
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		authorizer:    &infraBootAuthorizer{allow: true},
		closed:        &bootClosed,
	})
	require.Nil(t, boot.Accountant, "the boot bundle must carry no accountant")
	dispatcher := postStartDispatcherForTest(t, boot)

	const command = "test accounting added by reload"
	registerPostStartCommand(dispatcher, command)
	ctx := &pluginserver.CommandContext{Username: "alice", RemoteAddr: "198.51.100.8:2200"}

	first, err := dispatcher.Dispatch(ctx, command)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, first.Status)

	addedClosed := false
	added := &infraBootAccountant{name: "added", closed: &addedClosed}
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "added",
		authenticator: &infraBootAuthenticator{source: "added"},
		authorizer:    &infraBootAuthorizer{allow: true},
		accountant:    added,
		closed:        &addedClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, bootClosed, "installing a chain retires the one it replaces")

	second, err := dispatcher.Dispatch(ctx, command)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, second.Status)
	assert.Equal(t, []string{command}, added.starts,
		"accounting added by a reload must receive the START")
	assert.Equal(t, []string{command}, added.stops)
}

// VALIDATES: a command that starts before a reload and stops after it delivers
// its STOP record to the accountant installed NOW.
// PREVENTS: the record vanishing in silence. Installing a chain closes the one
// it replaces, which stops the retired TACACS+ accounting worker, and a send to
// a stopped worker drops the record and returns no error
// (internal/component/tacacs/accounting.go, enqueue).
func TestLiveAccountantDeliversAStopThatCrossesAnInstall(t *testing.T) {
	resetAAABundleForTest(t)

	bootClosed := false
	bootAccountant := &infraBootAccountant{name: "boot", closed: &bootClosed}
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name:          "boot",
		authenticator: &infraBootAuthenticator{source: "boot"},
		accountant:    bootAccountant,
		closed:        &bootClosed,
	})
	swapAAABundle(boot, nil)
	t.Cleanup(func() { closeAAABundle(nil) })

	accountant := newLiveAAABundleAccountant()
	taskID := accountant.CommandStart("alice", "198.51.100.8:2200", "show bgp")
	require.NotEmpty(t, taskID)
	require.Equal(t, []string{"show bgp"}, bootAccountant.starts)

	replacementClosed := false
	replacementAccountant := &infraBootAccountant{name: "replacement", closed: &replacementClosed}
	replacement := buildInfraBootBundle(t, &infraBootBackend{
		name:          "replacement",
		authenticator: &infraBootAuthenticator{source: "replacement"},
		accountant:    replacementAccountant,
		closed:        &replacementClosed,
	})
	swapAAABundle(replacement, nil)
	require.True(t, bootClosed, "installing a chain retires the one it replaces")

	accountant.CommandStop(taskID, "alice", "198.51.100.8:2200", "show bgp")

	assert.Equal(t, []string{"show bgp"}, replacementAccountant.stops,
		"the STOP must reach the installed accountant, whose worker is running")
	assert.Empty(t, bootAccountant.stops,
		"no record may reach an accountant whose worker the install has already stopped")
	assert.False(t, replacementAccountant.stoppedOnClosed)
}
