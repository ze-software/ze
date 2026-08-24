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
			aaa.SetAcceptedLocalProfileGeneration(preIdentity.generation)
		} else {
			aaa.SetAcceptedLocalProfileGeneration(0)
		}
	})
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
// for each new START, and delivers the STOP of a command that crossed a swap to
// the accountant installed NOW, carrying the task id its own START issued.
// PREVENTS: API, MCP, and standalone SSH dispatch losing the record. The swap
// closes the bundle it replaces, which stops that accountant's worker, and a
// send to a stopped worker drops the record and returns no error
// (internal/component/tacacs/accounting.go, enqueue). One command is still one
// START and one STOP: the replacement must mint no second START.
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
	assert.Empty(t, first.stops,
		"the retired accountant's worker is stopped, so a STOP sent there is lost in silence")
	assert.Empty(t, second.starts, "a swap must not mint a second START for one command")
	assert.Equal(t, []string{command}, second.stops)
	assert.Equal(t, "first-task", second.stopTaskID,
		"the STOP must carry the task id the START issued, so the two records still pair")
}

// VALIDATES: concurrent in-flight no-BGP commands keep unique accounting
// handles across a bundle swap, every STOP reaches the accountant installed
// NOW, and later commands use the replacement bundle for both records.
// PREVENTS: task-ID collisions, a swap minting extra STARTs, and concurrent
// STOP records landing on an accountant whose worker the swap has stopped.
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
	assert.Empty(t, first.stops,
		"the retired accountant's worker is stopped, so a STOP sent there is lost in silence")
	assert.Empty(t, second.starts, "a swap must not mint a START for a command already in flight")
	assert.Len(t, second.stops, commands, "every in-flight STOP must reach the installed accountant")
	assert.Equal(t, "first-task", second.stopTaskID,
		"each STOP carries the task id its own START issued, so the records still pair")

	taskID := accountant.CommandStart("bob", "203.0.113.4:2200", "test after swap")
	accountant.CommandStop(taskID, "bob", "203.0.113.4:2200", "test after swap")
	assert.Equal(t, []string{"test after swap"}, second.starts)
	assert.Len(t, second.stops, commands+1)
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
// profile, pinned to the generation that authenticated it.
func recoverySessionAuthorizer(t *testing.T, username string) aaa.Authorizer {
	t.Helper()
	accepted := acceptedLocalIdentity.Load()
	require.NotNil(t, accepted)

	for _, user := range accepted.users {
		if user.Name == username {
			return aaa.AuthorizerForResult(nil, aaa.AuthResult{
				Authenticated:   true,
				Source:          aaa.SourceLocal,
				Profiles:        []string{aaa.ReservedRecoveryProfile},
				LocalGeneration: user.LocalGeneration,
			})
		}
	}

	t.Fatalf("the accepted generation carries no user %q to log in as", username)
	return nil
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
	first := acceptedLocalIdentity.Load().generation
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(users, nil, nil, ""))
	assert.Equal(t, first, acceptedLocalIdentity.Load().generation)
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
	first := acceptedLocalIdentity.Load().generation
	session := recoverySessionAuthorizer(t, "admin")
	require.True(t, session.Authorize("admin", "", "config commit", false))

	rotated := []aaa.UserCredential{{
		Name:     "admin",
		Hash:     "$2a$10$rotated",
		Profiles: []string{aaa.ReservedRecoveryProfile},
	}}
	publishAcceptedLocalIdentity(newAcceptedLocalIdentity(rotated, nil, nil, ""))
	assert.NotEqual(t, first, acceptedLocalIdentity.Load().generation)
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
	// The five fields this test covers: Name, Hash, Profiles and PublicKeys
	// below, and LocalGeneration, which is deliberately excluded. A sixth field
	// fails here rather than reaching production unread. Counted by reflection
	// because an unkeyed literal, which would fail at compile time instead, is
	// what `go vet` composites refuses for a struct from another package.
	require.Equal(t, 5, reflect.TypeFor[aaa.UserCredential]().NumField(),
		"aaa.UserCredential grew a field: decide whether a change to it must revoke a live session, then extend sameLocalCredential and this test")

	accepted := aaa.UserCredential{
		Name:            "admin",
		Hash:            "$2a$10$accepted",
		Profiles:        []string{"ops"},
		PublicKeys:      []aaa.SSHPublicKey{{Name: "laptop", Type: "ssh-ed25519", Key: "AAAA"}},
		LocalGeneration: 7,
	}

	assert.True(t, sameLocalCredential(accepted, accepted), "a record compares equal to itself")

	restamped := accepted
	restamped.LocalGeneration = accepted.LocalGeneration + 1
	assert.True(t, sameLocalCredential(accepted, restamped),
		"LocalGeneration is the stamp this comparison decides and must not be read")

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
		{"Profiles removed", func(u *aaa.UserCredential) { u.Profiles = nil }},
		{"PublicKeys revoked", func(u *aaa.UserCredential) { u.PublicKeys = nil }},
	}
	for _, tc := range changes {
		t.Run(tc.field, func(t *testing.T) {
			candidate := accepted
			tc.change(&candidate)
			assert.False(t, sameLocalCredential(accepted, candidate),
				"a change to %s must revoke every session pinned to the accepted generation", tc.field)
		})
	}
}

// VALIDATES: sameLocalCredentials pairs users by name, so a reordered set of
// identical credentials still compares equal and a renamed user does not.
// PREVENTS: the comparison depending on infra.ExtractAuthUsers and
// mergeAuthUsers happening to produce the same order on both sides. An
// index-wise comparison stops revoking the day either one stops sorting, with
// every test still green.
func TestSameLocalCredentialsPairsUsersByName(t *testing.T) {
	alice := aaa.UserCredential{Name: "alice", Hash: "$2a$10$alice", Profiles: []string{"ops"}}
	bob := aaa.UserCredential{Name: "bob", Hash: "$2a$10$bob"}

	assert.True(t, sameLocalCredentials(
		[]aaa.UserCredential{alice, bob},
		[]aaa.UserCredential{bob, alice}),
		"the same two users in the other order are the same credential set")

	carol := aaa.UserCredential{Name: "carol", Hash: "$2a$10$bob"}
	assert.False(t, sameLocalCredentials(
		[]aaa.UserCredential{alice, bob},
		[]aaa.UserCredential{alice, carol}),
		"renaming a user is a credential change, whatever its hash")

	assert.False(t, sameLocalCredentials(
		[]aaa.UserCredential{alice, bob},
		[]aaa.UserCredential{alice, alice}),
		"a repeated name leaves the pairing ambiguous, so it must fail closed")

	assert.False(t, sameLocalCredentials(
		[]aaa.UserCredential{alice, bob},
		[]aaa.UserCredential{alice}),
		"a removed user is a credential change")
}
