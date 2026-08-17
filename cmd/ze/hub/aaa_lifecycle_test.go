package hub

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
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
	aaa.SetAcceptedLocalProfileGeneration(0)
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
		if preIdentity != nil {
			aaa.SetAcceptedLocalProfileGeneration(preIdentity.generation)
		} else {
			aaa.SetAcceptedLocalProfileGeneration(0)
		}
	})
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

func TestLiveAAABundleAuthorizerFailsClosedBeforeBundleInstall(t *testing.T) {
	resetAAABundleForTest(t)
	assert.False(t, (liveAAABundleAuthorizer{}).Authorize("alice", "", "show version", true))

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

// VALIDATES: no-BGP startup installs accounting that resolves the live bundle
// for each new START while routing STOP to the accountant that produced it.
// PREVENTS: API, MCP, and standalone SSH dispatch leaking a task across a swap.
func TestInstallNoBGPAAADispatchPairsAccountingAcrossSwap(t *testing.T) {
	resetAAABundleForTest(t)
	first := &bundleAccountantProbe{name: "first"}
	second := &bundleAccountantProbe{name: "second"}
	swapAAABundle(&aaa.Bundle{Accountant: first}, nil)

	dispatcher := pluginserver.NewDispatcher()
	installNoBGPAAADispatch(dispatcher)
	const command = "test live accounting"
	dispatcher.Register(command, func(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
		swapAAABundle(&aaa.Bundle{Accountant: second}, nil)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	}, command)

	response, err := dispatcher.Dispatch(&pluginserver.CommandContext{
		Username:   "alice",
		RemoteAddr: "198.51.100.8:2200",
	}, command)
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, plugin.StatusDone, response.Status)
	assert.Equal(t, []string{command}, first.starts)
	assert.Equal(t, []string{command}, first.stops)
	assert.Equal(t, "first-task", first.stopTaskID)
	assert.Empty(t, second.starts)
	assert.Empty(t, second.stops)
}

// VALIDATES: concurrent in-flight no-BGP commands keep unique accounting
// handles across a bundle swap, and later commands use the replacement bundle.
// PREVENTS: task-ID collisions or swaps misrouting concurrent STOP records.
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
	assert.Empty(t, second.starts)
	assert.Empty(t, second.stops)

	taskID := accountant.CommandStart("bob", "203.0.113.4:2200", "test after swap")
	accountant.CommandStop(taskID, "bob", "203.0.113.4:2200", "test after swap")
	assert.Equal(t, []string{"test after swap"}, second.starts)
	assert.Equal(t, []string{"test after swap"}, second.stops)
	assert.Equal(t, "second-task", second.stopTaskID)
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
