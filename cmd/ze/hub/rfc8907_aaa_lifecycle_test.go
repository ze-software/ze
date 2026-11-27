package hub

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/loginservice"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// stubBackendForSwap is a Backend whose Build returns a contribution with a
// Close that flips a flag when invoked.
type stubBackendForSwap struct {
	name   string
	closed *bool
}

func (s *stubBackendForSwap) Name() string  { return s.name }
func (s *stubBackendForSwap) Priority() int { return 100 }
func (s *stubBackendForSwap) Build(_ aaa.BuildParams) (aaa.Contribution, error) {
	return aaa.Contribution{
		Authenticator: &stubAuthn{},
		Close: func() error {
			*s.closed = true
			return nil
		},
	}, nil
}

type stubAuthn struct{}

func (stubAuthn) Authenticate(aaa.AuthRequest) (aaa.AuthResult, error) {
	return aaa.AuthResult{}, errors.New("stub")
}

func buildStubBundle(t *testing.T, closedFlag *bool) *aaa.Bundle {
	t.Helper()
	// Throw-away registry per bundle so nothing leaks into aaa.Default and
	// each test's close-tracking flag stays isolated.
	r := aaa.NewBackendRegistryForTest()
	require.NoError(t, r.Register(&stubBackendForSwap{name: "stub", closed: closedFlag}))
	built, err := r.Build(aaa.BuildParams{})
	require.NoError(t, err)
	return built
}

// resetAAABundleForTest snapshots and clears the live AAA bundle, its boot
// ownership, and accepted identity for the test body. Cleanup closes the test
// bundle and restores every prior value so package-global state cannot leak.
func resetAAABundleForTest(t *testing.T) {
	t.Helper()
	pre := aaaBundle.Swap(nil)
	preBootClaimed := aaaBundleBootClaimed.Swap(false)
	preIdentity := acceptedLocalIdentity.Swap(nil)
	aaa.SetAcceptedLocalUsers(nil)
	aaa.SetAcceptedLoginSets(nil)
	// closeAAABundle latches the acceptance slot retired, because a daemon
	// closes its AAA bundle once, at exit. A test binary boots many daemons in
	// one process, so the latch is cleared here and again on the way out.
	resetAAAAcceptanceForTest()
	if pre != nil {
		t.Logf("aaa bundle leak: pre-test slot was non-nil; an earlier test did not clean up")
	}
	if preIdentity != nil {
		t.Logf("accepted local identity leak: pre-test slot was non-nil; an earlier test did not clean up")
	}
	t.Cleanup(func() {
		if testBundle := aaaBundle.Swap(pre); testBundle != nil {
			if err := testBundle.Close(); err != nil {
				t.Logf("aaa bundle close error during cleanup: %v", err)
			}
		}
		aaaBundleBootClaimed.Store(preBootClaimed)
		acceptedLocalIdentity.Store(preIdentity)
		resetAAAAcceptanceForTest()
		if preIdentity != nil {
			aaa.SetAcceptedLocalUsers(preIdentity.users)
			aaa.SetAcceptedLoginSets(preIdentity.loginSets)
		} else {
			aaa.SetAcceptedLocalUsers(nil)
			aaa.SetAcceptedLoginSets(nil)
		}
	})
}

// ensureLoginServicesForTest ensures each named login service is registered in
// loginservice for the duration of t and restores the prior snapshot on
// cleanup. Untagged hub test runs do not link gated surfaces such as ze_rest or
// ze_grpc, so tests that exercise login-service decisions register the needed
// names through this helper when absent.
func ensureLoginServicesForTest(t *testing.T, names ...string) {
	t.Helper()
	snap := loginservice.Snapshot()
	t.Cleanup(func() { loginservice.Restore(snap) })
	for _, name := range names {
		if !loginservice.Registered(name) {
			require.NoError(t, loginservice.Register(loginservice.Entry{
				Name:        name,
				Owner:       "cmd/ze/hub/test",
				Description: "test login service " + name,
			}))
		}
	}
}

// resetAAAAcceptanceForTest clears the reload acceptance state so the next test
// starts from a daemon that has accepted no configuration and retired nothing.
func resetAAAAcceptanceForTest() {
	aaaAcceptance.Lock()
	defer aaaAcceptance.Unlock()
	aaaAcceptedConfigOrder = 0
	aaaAcceptanceRetired = false
}

func localAuthzStoreForTest(action authz.Action) *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "operator",
		Run:  authz.Section{Default: action},
		Edit: authz.Section{Default: action},
	})
	store.AssignProfiles("alice", []string{"operator"})
	return store
}

type typedBundleAuthorizer struct {
	allow         bool
	legacyCalled  bool
	typedCalled   bool
	command       string
	args          []string
	peer          string
	localFallback aaa.Authorizer
}

func (a *typedBundleAuthorizer) Authorize(_, _, _ string, _ bool) bool {
	a.legacyCalled = true
	return a.allow
}

func (a *typedBundleAuthorizer) AuthorizeCommandArgs(_, _, command string, args []string, peer string, _ bool) bool {
	a.typedCalled = true
	a.command = command
	a.args = append([]string(nil), args...)
	a.peer = peer
	return a.allow
}

func (a *typedBundleAuthorizer) BindLocalFallback(local aaa.Authorizer) aaa.Authorizer {
	a.localFallback = local
	return a
}

type bundleAccountantProbe struct {
	mu         sync.Mutex
	name       string
	starts     []string
	stops      []string
	stopTaskID string
}

func (a *bundleAccountantProbe) CommandStart(_, _, command string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.starts = append(a.starts, command)
	return a.name + "-task"
}

func (a *bundleAccountantProbe) CommandStop(taskID, _, _, command string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopTaskID = taskID
	a.stops = append(a.stops, command)
}

// TestLiveAAABundleAuthorizerFailsClosedBeforeBundleInstall pins the
// authorization half of the failover, which FAILS CLOSED (owner ruling,
// 2026-09-04: "we should fail close - no user no login").
//
// A fallback to the local policy was tried and reverted the same day: it made a
// box with no system.authorization profile allow every command while its chain
// was broken. Authentication still fails over, so ssh starts and a local
// account logs in; authorization does not, so that session runs nothing.
func TestLiveAAABundleAuthorizerFailsClosedBeforeBundleInstall(t *testing.T) {
	resetAAABundleForTest(t)
	assert.False(t, (liveAAABundleAuthorizer{}).Authorize("alice", "", "show version", true),
		"no bundle means no policy was installed, so nothing may be authorized")
	assert.False(t, (liveAAABundleAuthorizer{}).AuthorizeCommandArgs("alice", "", "show version", nil, "", true),
		"both methods must answer alike, or a command is allowed by name and denied by its arguments")

	swapAAABundle(&aaa.Bundle{}, nil)
	assert.True(t, (liveAAABundleAuthorizer{}).Authorize("alice", "", "show version", true),
		"a non-nil bundle with no RBAC policy preserves the accepted no-RBAC allow mode")
}

// VALIDATES: no-BGP typed dispatch forwards exact command, args, and peer to
// the current bundle authorizer.
// PREVENTS: live bundle indirection flattening whitespace-containing cmd-args.
func TestLiveAAABundleAuthorizerPreservesTypedArgs(t *testing.T) {
	resetAAABundleForTest(t)
	remote := &typedBundleAuthorizer{allow: true}
	swapAAABundle(&aaa.Bundle{Authorizer: remote}, nil)

	args := []string{"neighbor description with spaces"}
	authorizer := liveAAABundleAuthorizer{}
	assert.True(t, authorizer.AuthorizeCommandArgs(
		aaa.ReservedInternalPrefix+"plugin:test",
		"127.0.0.1:1",
		"request bgp peer update",
		args,
		"192.0.2.7",
		false,
	))
	assert.False(t, remote.legacyCalled)
	assert.True(t, remote.typedCalled)
	assert.Equal(t, "request bgp peer update", remote.command)
	assert.Equal(t, args, remote.args)
	assert.Equal(t, "192.0.2.7", remote.peer)
}

// VALIDATES: accepted API authorization retains typed command boundaries while
// rebinding the selected external authorizer's local fallback generation.
// PREVENTS: TACACS+ receiving a flattened cmd-arg after API authentication.
func TestAcceptedLocalGenerationAuthorizerPreservesExternalTypedArgs(t *testing.T) {
	resetAAABundleForTest(t)
	remote := &typedBundleAuthorizer{allow: true}
	swapAAABundle(&aaa.Bundle{Authorizer: remote}, nil)

	args := []string{"community value with spaces"}
	authorizer := acceptedLocalGenerationAuthorizer{store: nil}
	assert.True(t, authorizer.AuthorizeCommandArgs(
		"api-user",
		"198.51.100.14:8443",
		"request bgp policy apply",
		args,
		"203.0.113.9",
		false,
	))
	assert.False(t, remote.legacyCalled)
	assert.True(t, remote.typedCalled)
	assert.Equal(t, args, remote.args)
	assert.Equal(t, "203.0.113.9", remote.peer)
	require.NotNil(t, remote.localFallback)
	assert.True(t, remote.localFallback.Authorize("api-user", "", "show version", true))
}

// RFC requirement: RFC8907-8.3-4 positive -- a command that replaces the AAA bundle keeps its original accountant alive through STOP without delaying the replacement inside its handler.
func TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap(t *testing.T) {
	resetAAABundleForTest(t)
	first := &bundleAccountantProbe{name: "first"}
	second := &bundleAccountantProbe{name: "second"}
	var firstClosed bool
	swapAAABundle(buildInfraBootBundle(t, &infraBootBackend{
		name: "first", authenticator: &stubAuthn{}, accountant: first, closed: &firstClosed,
	}), nil)

	dispatcher := pluginserver.NewDispatcher()
	installNoBGPAAADispatch(dispatcher)
	const commandName = "test live accounting"
	if err := dispatcher.Register(commandName, func(_ *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		swapAAABundle(&aaa.Bundle{Accountant: second}, nil)
		assert.False(t, firstClosed, "reload closed the accountant before the command's STOP")
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, commandName); err != nil {
		t.Fatal(err)
	}

	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username:   "alice",
		RemoteAddr: "198.51.100.8:2200",
	}, commandName)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, plugin.StatusDone, response.Status)
	assert.Equal(t, []string{commandName}, first.starts)
	assert.Equal(t, []string{commandName}, first.stops)
	assert.Empty(t, second.starts, "a swap must not mint a second START for one command")
	assert.Empty(t, second.stops, "the replacement server never received this command's START")
	assert.Equal(t, "first-task", first.stopTaskID)
	assert.True(t, firstClosed, "retired bundle survived its final accounting STOP")
}

// Concurrent in-flight commands keep unique accounting handles across reload;
// each STOP returns to its START's provider and later commands use the new one.
func TestLiveAAABundleAccountantConcurrentSwapKeepsPairs(t *testing.T) {
	resetAAABundleForTest(t)
	first := &bundleAccountantProbe{name: "first"}
	second := &bundleAccountantProbe{name: "second"}
	swapAAABundle(&aaa.Bundle{Accountant: first}, nil)
	accountant := newLiveAAABundleAccountant()

	const commands = 16
	started := make(chan string, commands)
	release := make(chan struct{})
	var wg sync.WaitGroup
	for range commands {
		wg.Go(func() {
			taskID := accountant.CommandStart("alice", "198.51.100.8:2200", "test concurrent accounting")
			started <- taskID
			<-release
			accountant.CommandStop(taskID, "alice", "198.51.100.8:2200", "test concurrent accounting")
		})
	}

	handles := make(map[string]struct{}, commands)
	for range commands {
		taskID := <-started
		require.NotEmpty(t, taskID)
		handles[taskID] = struct{}{}
	}
	require.Len(t, handles, commands)
	swapAAABundle(&aaa.Bundle{Accountant: second}, nil)
	close(release)
	wg.Wait()

	assert.Len(t, first.starts, commands)
	assert.Len(t, first.stops, commands)
	assert.Empty(t, second.starts, "a swap must not mint a START for a command already in flight")
	assert.Empty(t, second.stops)
	assert.Equal(t, "first-task", first.stopTaskID)

	taskID := accountant.CommandStart("bob", "203.0.113.4:2200", "test after swap")
	accountant.CommandStop(taskID, "bob", "203.0.113.4:2200", "test after swap")
	assert.Equal(t, []string{"test after swap"}, second.starts)
	assert.Len(t, second.stops, 1)
	assert.Equal(t, "second-task", second.stopTaskID)
}

type blockedStartAccountant struct {
	infraBootAccountant
	entered chan struct{}
	release chan struct{}
}

func (a *blockedStartAccountant) CommandStart(username, remoteAddr, command string) string {
	close(a.entered)
	<-a.release
	return a.infraBootAccountant.CommandStart(username, remoteAddr, command)
}

// RFC requirement: RFC8907-8.3-4 negative -- replacement cannot close an accountant while it is accepting START, even when the new configuration disables accounting.
func TestRFC8907BundleRetirementWaitsForAccountingPair(t *testing.T) {
	resetAAABundleForTest(t)
	var closed bool
	provider := &blockedStartAccountant{
		name:    "old",
		closed:  &closed,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	swapAAABundle(buildInfraBootBundle(t, &infraBootBackend{
		name: "old", authenticator: &stubAuthn{}, accountant: provider, closed: &closed,
	}), nil)
	accountant := newLiveAAABundleAccountant()
	started := make(chan string, 1)
	go func() {
		started <- accountant.CommandStart("alice", "192.0.2.1", "show version")
	}()
	<-provider.entered
	swapAAABundle(&aaa.Bundle{}, nil)
	assert.False(t, closed, "bundle closed while START was blocked")
	close(provider.release)
	taskID := <-started
	accountant.CommandStop(taskID, "alice", "192.0.2.1", "show version")
	assert.Equal(t, []string{"show version"}, provider.starts)
	assert.Equal(t, []string{"show version"}, provider.stops)
	assert.False(t, provider.stoppedOnClosed)
	assert.True(t, closed, "final STOP failed to release the retired bundle")
}

// VALIDATES: the local contribution in a newly built AAA bundle consults the
// boot authorization store installed by runYANGConfig.
func TestBuildAAABundleUsesInitialLiveLocalAuthorization(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, localAuthzStoreForTest(authz.Allow), nil, ""))

	bundle, err := buildAAABundle(nil, nil, nil, nil)
	require.NoError(t, err)
	swapAAABundle(bundle, nil)
	require.NotNil(t, bundle.Authorizer)
	assert.True(t, bundle.Authorizer.Authorize("alice", "", "show version", true))
	assert.False(t, bundle.Authorizer.Authorize("unassigned", "", "show version", true))
}

// VALIDATES: an already-installed local AAA authorizer dereferences the
// accepted identity on every decision instead of retaining its startup store.
func TestLiveLocalAuthorizerFollowsIdentityPublication(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, localAuthzStoreForTest(authz.Allow), nil, ""))

	bundle, err := buildAAABundle(nil, nil, nil, nil)
	require.NoError(t, err)
	swapAAABundle(bundle, nil)
	require.True(t, bundle.Authorizer.Authorize("alice", "", "show version", true))

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, localAuthzStoreForTest(authz.Deny), nil, ""))
	assert.False(t, bundle.Authorizer.Authorize("alice", "", "show version", true))
}

// VALIDATES: no system.authorization store retains the existing permissive
// post-authentication behavior.
func TestLiveLocalAuthorizerNilStoreAllows(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, ""))

	authorizer := liveLocalAuthorizer{}
	assert.True(t, authorizer.Authorize("alice", "", "show version", true))
	assert.True(t, authorizer.AuthorizeCommandArgs("alice", "", "show bgp rib", nil, "192.0.2.1", true))
}

// VALIDATES: a local authentication result keeps its resolved profiles instead
// of looking up a later username assignment during command authorization.
func TestLiveLocalAuthorizerBindsAuthenticationProfiles(t *testing.T) {
	resetAAABundleForTest(t)
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "recovery",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "assigned",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("alice", []string{"assigned"})
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))

	bound := aaa.BindProfiles(liveLocalAuthorizer{}, []string{"recovery"})
	assert.True(t, bound.Authorize("alice", "", "show version", true))
	assert.False(t, bound.Authorize("alice", "", "set system host-name router", false))
	typed, ok := bound.(aaa.CommandArgsAuthorizer)
	require.True(t, ok)
	assert.True(t, typed.AuthorizeCommandArgs("alice", "", "show", []string{"version"}, "", true))
}

// VALIDATES: the live BUNDLE authorizer keeps one authentication result's
// resolved profiles, instead of looking up a later username assignment during
// command authorization.
// PREVENTS: a privilege boundary silently moving. ssh binds this value for a
// public-key session (aaa.AuthorizerForResult over Config.Authorizer), and
// without BindProfiles aaa.BindProfiles returns it UNCHANGED, so a `ze init`
// break-glass recovery grant is authorized by whatever profile the store
// assigns the username. The two profiles below are opposites, so the unbound
// answer is the inverse of the bound one on both a run and an edit.
func TestLiveAAABundleAuthorizerBindsAuthenticationProfiles(t *testing.T) {
	resetAAABundleForTest(t)
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "recovery",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AddProfile(authz.Profile{
		Name: "assigned",
		Run:  authz.Section{Default: authz.Deny},
		Edit: authz.Section{Default: authz.Allow},
	})
	store.AssignProfiles("alice", []string{"assigned"})
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, store, nil, ""))

	// The bundle's authorizer is what buildAAABundle always installs: the local
	// backend contributes params.LocalAuthorizer, which is liveLocalAuthorizer.
	registry := aaa.NewBackendRegistryForTest()
	require.NoError(t, registry.Register(&infraBootBackend{
		name:          "local-only",
		authenticator: &infraBootAuthenticator{source: "local-only"},
		authorizer:    liveLocalAuthorizer{},
	}))
	bundle, err := registry.Build(aaa.BuildParams{})
	require.NoError(t, err)
	swapAAABundle(bundle, nil)

	bound := aaa.BindProfiles(liveAAABundleAuthorizer{}, []string{"recovery"})
	assert.True(t, bound.Authorize("alice", "", "show version", true))
	assert.False(t, bound.Authorize("alice", "", "set system host-name router", false))
	typed, ok := bound.(aaa.CommandArgsAuthorizer)
	require.True(t, ok)
	assert.True(t, typed.AuthorizeCommandArgs("alice", "", "show", []string{"version"}, "", true))
	assert.False(t, typed.AuthorizeCommandArgs("alice", "", "set", []string{"system", "host-name", "router"}, "", false))
}

// VALIDATES: the API generation authorizer binds recovery profiles against the
// store that authenticated the request, not the store's username assignment.
func TestAcceptedLocalGenerationAuthorizerBindsAuthenticationProfiles(t *testing.T) {
	resetAAABundleForTest(t)
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "recovery",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("alice", []string{"missing"})
	swapAAABundle(&aaa.Bundle{}, nil)

	bound := aaa.BindProfiles(acceptedLocalGenerationAuthorizer{store: store}, []string{"recovery"})
	assert.True(t, bound.Authorize("alice", "", "show version", true))
	assert.False(t, bound.Authorize("alice", "", "set system host-name router", false))
	typed, ok := bound.(aaa.CommandArgsAuthorizer)
	require.True(t, ok)
	assert.True(t, typed.AuthorizeCommandArgs("alice", "", "show", []string{"version"}, "", true))
}

// VALIDATES: daemon shutdown clears accepted credentials and policy along with
// the bundle, isolating the next daemon or test run.
func TestCloseAAABundleClearsAcceptedLocalIdentity(t *testing.T) {
	resetAAABundleForTest(t)
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, localAuthzStoreForTest(authz.Deny), nil, ""))
	require.False(t, (liveLocalAuthorizer{}).Authorize("alice", "", "show version", true))

	closeAAABundle(nil)
	assert.Nil(t, acceptedLocalIdentity.Load())
	assert.True(t, (liveLocalAuthorizer{}).Authorize("alice", "", "show version", true))
}

// VALIDATES: swapAAABundle closes the previously installed bundle.
// PREVENTS: TACACS+ accounting worker goroutine leaking across config reloads.
func TestSwapAAABundleClosesPrevious(t *testing.T) {
	resetAAABundleForTest(t)

	var firstClosed, secondClosed bool
	first := buildStubBundle(t, &firstClosed)
	second := buildStubBundle(t, &secondClosed)

	swapAAABundle(first, nil)
	assert.False(t, firstClosed, "first bundle must not be closed yet")

	swapAAABundle(second, nil)
	assert.True(t, firstClosed, "first bundle must be closed when second is installed")
	assert.False(t, secondClosed, "second bundle must not be closed yet")

	// Cleanup: close the still-installed bundle.
	closeAAABundle(nil)
	assert.True(t, secondClosed, "second bundle must be closed by closeAAABundle")
}

// VALIDATES: closeAAABundle is idempotent and safe with no installed bundle.
// PREVENTS: panic on exit paths that never ran infraSetup.
func TestCloseAAABundleNoBundle(t *testing.T) {
	// test-asserts-nothing: returning from closeAAABundle is the no-panic assertion.
	resetAAABundleForTest(t)
	// Must not panic.
	closeAAABundle(nil)
}

// VALIDATES: swapAAABundle with the same bundle twice does not double-close.
// PREVENTS: nil-pointer or accidental close when infraSetup runs twice with
// the same bundle (shouldn't happen, but the guard is cheap).
func TestSwapAAABundleSameBundleNoop(t *testing.T) {
	resetAAABundleForTest(t)

	var closed bool
	bundle := buildStubBundle(t, &closed)

	swapAAABundle(bundle, nil)
	swapAAABundle(bundle, nil)
	assert.False(t, closed, "swapping the same bundle must not close it")

	closeAAABundle(nil)
	assert.True(t, closed)
}

// VALIDATES: swapAAABundle is safe to call concurrently.
// PREVENTS: race conditions if config reload and shutdown overlap.
func TestSwapAAABundleConcurrent(t *testing.T) {
	resetAAABundleForTest(t)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			var closed bool
			bundle := buildStubBundle(t, &closed)
			swapAAABundle(bundle, nil)
		})
	}
	wg.Wait()

	closeAAABundle(nil)
}

// recoverySessionAuthorizer returns the authorizer a live break-glass session
// holds: the one aaa binds to a local login that resolved the reserved recovery
// profile, pinned to its user's generation that authenticated it.
func recoverySessionAuthorizer(t *testing.T, username string) aaa.Authorizer {
	t.Helper()
	return aaa.AuthorizerForResult(nil, aaa.AuthResult{
		Authenticated:  true,
		Source:         aaa.SourceLocal,
		Profiles:       []string{aaa.ReservedRecoveryProfile},
		UserGeneration: acceptedUserGeneration(t, username),
	})
}

// acceptedUserGeneration returns the per-user generation the accepted identity
// stamped on username's one record.
func acceptedUserGeneration(t *testing.T, username string) uint64 {
	t.Helper()
	accepted := acceptedLocalIdentity.Load()
	require.NotNil(t, accepted)

	for _, user := range accepted.users {
		if user.Name == username {
			return user.UserGeneration
		}
	}

	t.Fatalf("the accepted generation carries no user %q to log in as", username)
	return 0
}

// acceptedUserGenerations returns every per-user generation the accepted
// identity stamped, by user name. A name the accepted list repeats maps to one
// number per record.
func acceptedUserGenerations(t *testing.T) map[string][]uint64 {
	t.Helper()
	accepted := acceptedLocalIdentity.Load()
	require.NotNil(t, accepted)

	generations := make(map[string][]uint64, len(accepted.users))
	for _, user := range accepted.users {
		generations[user.Name] = append(generations[user.Name], user.UserGeneration)
	}
	return generations
}

// VALIDATES: republishing an identical local credential set reuses the accepted
// generation, so a live break-glass session keeps its authority across a config
// reload that changed no credential.
// PREVENTS: a web config commit revoking, inside its own request, the session
// that issued it -- the commit succeeded, then the commit bar it wrote back
// rendered read-only and every later edit answered 403.
func TestAcceptedLocalIdentityReusesGenerationForUnchangedCredentials(t *testing.T) {
	resetAAABundleForTest(t)

	users := []aaa.UserCredential{{
		Name:     "admin",
		Hash:     "$2a$10$accepted",
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	first := acceptedUserGeneration(t, "admin")
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	assert.Equal(t, first, acceptedUserGeneration(t, "admin"))
	assert.True(t, session.Authorize("admin", "", "config commit", false),
		"a reload that changed no credential must not revoke a live recovery session")
}

// VALIDATES: a changed local credential still advances the generation.
// PREVENTS: the reuse above becoming a permanent break-glass grant that a
// password change, a profile change, or a removed admin cannot revoke.
func TestAcceptedLocalIdentityAdvancesGenerationForChangedCredentials(t *testing.T) {
	resetAAABundleForTest(t)

	users := []aaa.UserCredential{{
		Name:     "admin",
		Hash:     "$2a$10$accepted",
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	first := acceptedUserGeneration(t, "admin")
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	rotated := []aaa.UserCredential{{
		Name:     "admin",
		Hash:     "$2a$10$rotated",
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(rotated, nil, nil, ""))
	assert.NotEqual(t, first, acceptedUserGeneration(t, "admin"))
	assert.False(t, session.Authorize("admin", "", "config commit", false),
		"a rotated password hash must revoke the session it no longer matches")

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(nil, nil, nil, ""))
	assert.False(t, session.Authorize("admin", "", "config commit", false),
		"removing the admin must revoke it too")
}

// VALIDATES: sameLocalCredential reads every field of aaa.UserCredential that
// authenticates or authorizes a user, in BOTH directions -- an equal pair
// compares equal, and a pair differing in one field compares different.
// PREVENTS: two ways of losing the revocation. A field the comparison forgets:
// Profiles and PublicKeys were exercised in neither direction, so deleting
// their lines left the suite green while a demoted profile and a revoked SSH
// key silently stopped revoking a live session. And a field a later change ADDS
// to aaa.UserCredential: the field count below fails the moment the struct
// grows, so whoever adds the field decides whether a change to it must revoke.
func TestSameLocalCredentialReadsEveryField(t *testing.T) {
	// The six fields this test covers: Name, Hash, Profiles, PublicKeys and
	// LoginSet below, and UserGeneration, the stamp, which is deliberately
	// excluded. A seventh field fails here rather than reaching production
	// unread. Counted by reflection because an unkeyed literal, which would
	// fail at compile time instead, is what `go vet` composites refuses for a
	// struct from another package.
	require.Equal(t, 6, reflect.TypeFor[aaa.UserCredential]().NumField(),
		"aaa.UserCredential grew a field: decide whether a change to it must revoke a live session, then extend sameLocalCredential and this test")

	accepted := aaa.UserCredential{
		Name:           "admin",
		Hash:           "$2a$10$accepted",
		Profiles:       []string{"ops"},
		PublicKeys:     []aaa.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: "AAAA"}},
		LoginSet:       "ssh-only",
		UserGeneration: 7,
	}

	assert.True(t, sameLocalCredential(accepted, accepted), "a record compares equal to itself")

	userRestamped := accepted
	userRestamped.UserGeneration = accepted.UserGeneration + 1
	assert.True(t, sameLocalCredential(accepted, userRestamped),
		"UserGeneration is the per-user stamp this comparison decides and must not be read")

	changes := []struct {
		field  string
		change func(*aaa.UserCredential)
	}{
		{"Name", func(u *aaa.UserCredential) { u.Name = "root" }},
		{"Hash", func(u *aaa.UserCredential) { u.Hash = "$2a$10$rotated" }},
		{"Profiles", func(u *aaa.UserCredential) { u.Profiles = []string{"read-only"} }},
		{"PublicKeys", func(u *aaa.UserCredential) {
			u.PublicKeys = []aaa.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: "BBBB"}}
		}},
		{"LoginSet", func(u *aaa.UserCredential) { u.LoginSet = "web-only" }},
		{"Profiles removed", func(u *aaa.UserCredential) { u.Profiles = nil }},
		{"PublicKeys revoked", func(u *aaa.UserCredential) { u.PublicKeys = nil }},
		{"LoginSet removed", func(u *aaa.UserCredential) { u.LoginSet = "" }},
	}
	for _, tc := range changes {
		t.Run(tc.field, func(t *testing.T) {
			candidate := accepted
			tc.change(&candidate)
			assert.False(t, sameLocalCredential(accepted, candidate),
				"a change to %s must revoke every session pinned to the accepted generation", tc.field)
		})
	}

	// Prove end-to-end that changing LoginSet advances UserGeneration while
	// republishing an identical LoginSet preserves it.
	resetAAABundleForTest(t)
	first := []aaa.UserCredential{{
		Name:     "alice",
		Hash:     "$2a$10$alice",
		Profiles: []string{"ops"},
		LoginSet: "ssh-only",
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(first, nil, nil, ""))
	gen1 := acceptedUserGeneration(t, "alice")

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(first, nil, nil, ""))
	assert.Equal(t, gen1, acceptedUserGeneration(t, "alice"),
		"unchanged LoginSet must preserve UserGeneration")

	changed := []aaa.UserCredential{{
		Name:     "alice",
		Hash:     "$2a$10$alice",
		Profiles: []string{"ops"},
		LoginSet: "web-only",
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(changed, nil, nil, ""))
	assert.NotEqual(t, gen1, acceptedUserGeneration(t, "alice"),
		"changing LoginSet must advance UserGeneration to revoke existing sessions")
}

// VALIDATES: publishAcceptedLocalIdentity pairs users by name when it stamps
// each user's generation, so a reordered set of identical credentials keeps
// every number, a renamed user gets a fresh one, a repeated name gets fresh
// numbers for each of its records, and a removed user is gone.
// PREVENTS: the pairing depending on infra.ExtractAuthUsers and
// mergeAuthUsers happening to produce the same order on both sides. An
// index-wise pairing stops revoking the day either one stops sorting, with
// every test still green.
func TestSameLocalCredentialsPairsUsersByName(t *testing.T) {
	resetAAABundleForTest(t)

	alice := aaa.UserCredential{Name: "alice", Hash: "$2a$10$alice", Profiles: []string{"ops"}}
	bob := aaa.UserCredential{Name: "bob", Hash: "$2a$10$bob"}
	publish := func(users ...aaa.UserCredential) {
		publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	}
	// acceptBase publishes alice and bob and returns the numbers each got, so
	// every case below starts from the same accepted pair.
	acceptBase := func() (uint64, uint64) {
		publish(alice, bob)
		return acceptedUserGeneration(t, "alice"), acceptedUserGeneration(t, "bob")
	}

	aliceBase, bobBase := acceptBase()
	publish(bob, alice)
	assert.Equal(t, aliceBase, acceptedUserGeneration(t, "alice"),
		"alice in the other order is the same record, so she keeps her number")
	assert.Equal(t, bobBase, acceptedUserGeneration(t, "bob"),
		"bob in the other order is the same record, so he keeps his number")

	_, bobBase = acceptBase()
	carol := aaa.UserCredential{Name: "carol", Hash: "$2a$10$bob"}
	publish(alice, carol)
	assert.NotEqual(t, bobBase, acceptedUserGeneration(t, "carol"),
		"renaming a user is a credential change, whatever its hash")
	assert.NotContains(t, acceptedUserGenerations(t), "bob", "the renamed user is gone")

	aliceBase, _ = acceptBase()
	publish(alice, alice)
	repeated := acceptedUserGenerations(t)["alice"]
	require.Len(t, repeated, 2, "both records of the repeated name are accepted")
	assert.NotEqual(t, aliceBase, repeated[0],
		"a repeated name leaves the pairing ambiguous, so its first record must fail closed")
	assert.NotEqual(t, aliceBase, repeated[1],
		"a repeated name leaves the pairing ambiguous, so its second record must fail closed")
	assert.NotEqual(t, repeated[0], repeated[1], "no two user records share a number")

	aliceBase, _ = acceptBase()
	publish(alice)
	assert.NotContains(t, acceptedUserGenerations(t), "bob", "a removed user is gone")
	assert.Equal(t, aliceBase, acceptedUserGeneration(t, "alice"),
		"removing bob is no change to alice's record, so she keeps her number")
}

// A password login's authorization follows the replacement bundle, including
// typed commands, rather than retaining its now-closed TACACS+ client.
func TestLivePasswordAuthorizationFollowsBundleReload(t *testing.T) {
	resetAAABundleForTest(t)
	first := &typedBundleAuthorizer{allow: true}
	boot := buildInfraBootBundle(t, &infraBootBackend{
		name: "remote", authenticator: &infraBootAuthenticator{source: "tacacs"}, authorizer: first,
	})
	swapAAABundle(boot, nil)
	result, err := (liveAAABundleAuthenticator{}).Authenticate(aaa.AuthRequest{Username: "alice"})
	require.NoError(t, err)
	require.NotNil(t, result.Authorizer)
	assert.True(t, result.Authorizer.Authorize("alice", "", "show version", true))

	replacement := &typedBundleAuthorizer{allow: false}
	swapAAABundle(&aaa.Bundle{Authorizer: replacement}, nil)
	assert.False(t, result.Authorizer.Authorize("alice", "", "show version", true))
	typed, ok := result.Authorizer.(aaa.CommandArgsAuthorizer)
	require.True(t, ok)
	args := []string{"two words", `slash\inside`}
	assert.False(t, typed.AuthorizeCommandArgs("alice", "", "show", args, "*", true))
	assert.Equal(t, args, replacement.args)
	assert.False(t, first.typedCalled)
}

// VALIDATES: AC-6, AC-8, AC-20 -- checkAcceptedLocalLoginService gates named
// principals (such as MCP bearer-list/oauth identities) against the accepted
// local identity generation, fails closed when uninitialized or retired, leaves
// anonymous and unmatched principals ungated per OD-3 option (a), and keeps the
// local recovery profile exempt.
// PREVENTS: MCP named principals bypassing a configured login-service set or
// succeeding after shutdown retires the accepted identity generation.
func TestCheckAcceptedLocalLoginService(t *testing.T) {
	resetAAABundleForTest(t)
	ensureLoginServicesForTest(t, "mcp", "ssh")

	// Anonymous callers (empty username) are ungated even before publication.
	assert.NoError(t, checkAcceptedLocalLoginService("", "mcp"))

	// Uninitialized accepted identity fails closed for named principals.
	assert.ErrorIs(t, checkAcceptedLocalLoginService("alice", "mcp"), aaa.ErrAuthRejected)

	users := []aaa.UserCredential{
		{Name: "sshonly", LoginSet: "ssh-set"},
		{Name: "mcponly", LoginSet: "mcp-set"},
		{Name: "unrestricted"},
		{Name: "recovery", Profiles: []string{aaa.ReservedRecoveryProfile}, LoginSet: "ssh-set"},
	}
	sets := map[string][]string{
		"ssh-set": {"ssh"},
		"mcp-set": {"mcp"},
	}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, "").withLoginSets(sets))

	assert.ErrorIs(t, checkAcceptedLocalLoginService("sshonly", "mcp"), aaa.ErrAuthRejected,
		"local user whose login set excludes mcp must be refused")
	assert.NoError(t, checkAcceptedLocalLoginService("mcponly", "mcp"),
		"local user whose login set includes mcp must be admitted")
	assert.NoError(t, checkAcceptedLocalLoginService("unrestricted", "mcp"),
		"local user with no login-service set configured must be admitted")
	assert.NoError(t, checkAcceptedLocalLoginService("external-oauth-sub", "mcp"),
		"named principal with no matching local user record has no set configured")
	assert.NoError(t, checkAcceptedLocalLoginService("recovery", "mcp"),
		"local recovery profile stays exempt from login-service sets")
	assert.ErrorIs(t, checkAcceptedLocalLoginService(aaa.ReservedInternalRPCIdentity, "mcp"), aaa.ErrAuthRejected,
		"reserved usernames must fail closed")

	// Closing the AAA bundle retires both the accepted identity and login sets.
	closeAAABundle(nil)
	assert.ErrorIs(t, checkAcceptedLocalLoginService("mcponly", "mcp"), aaa.ErrAuthRejected,
		"retired accepted identity must fail closed")
	assert.ErrorIs(t, aaa.CheckLoginSet("mcponly", "mcp-set", "mcp"), aaa.ErrAuthRejected,
		"retired login sets must fail closed")
}
