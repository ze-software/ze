package hub

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
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

// resetAAABundleForTest snapshots and clears both live AAA slots for the test
// body. Cleanup closes the test bundle and restores the prior bundle and local
// authorization store so package-global state cannot leak between tests.
func resetAAABundleForTest(t *testing.T) {
	t.Helper()
	pre := aaaBundle.Swap(nil)
	preStore := liveLocalAuthzStore.Swap(nil)
	if pre != nil {
		t.Logf("aaa bundle leak: pre-test slot was non-nil; an earlier test did not clean up")
	}
	if preStore != nil {
		t.Logf("local authz store leak: pre-test slot was non-nil; an earlier test did not clean up")
	}
	t.Cleanup(func() {
		if testBundle := aaaBundle.Swap(pre); testBundle != nil {
			if err := testBundle.Close(); err != nil {
				t.Logf("aaa bundle close error during cleanup: %v", err)
			}
		}
		liveLocalAuthzStore.Store(preStore)
	})
}

func localAuthzStoreForTest(username string, action authz.Action) *authz.Store {
	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "operator",
		Run:  authz.Section{Default: action},
		Edit: authz.Section{Default: action},
	})
	store.AssignProfiles(username, []string{"operator"})
	return store
}

// VALIDATES: the local contribution in a newly built AAA bundle consults the
// boot authorization store installed by runYANGConfig.
func TestBuildAAABundleUsesInitialLiveLocalAuthorization(t *testing.T) {
	resetAAABundleForTest(t)
	swapLocalAuthzStore(localAuthzStoreForTest("alice", authz.Allow))

	bundle, err := buildAAABundle(nil, nil, nil, nil)
	require.NoError(t, err)
	swapAAABundle(bundle, nil)
	require.NotNil(t, bundle.Authorizer)
	assert.True(t, bundle.Authorizer.Authorize("alice", "", "show version", true))
	assert.False(t, bundle.Authorizer.Authorize("unassigned", "", "show version", true))
}

// VALIDATES: an already-installed local AAA authorizer dereferences the live
// store on every decision instead of retaining its startup *authz.Store.
func TestLiveLocalAuthorizerFollowsStoreSwap(t *testing.T) {
	resetAAABundleForTest(t)
	swapLocalAuthzStore(localAuthzStoreForTest("alice", authz.Allow))

	bundle, err := buildAAABundle(nil, nil, nil, nil)
	require.NoError(t, err)
	swapAAABundle(bundle, nil)
	require.True(t, bundle.Authorizer.Authorize("alice", "", "show version", true))

	swapLocalAuthzStore(localAuthzStoreForTest("alice", authz.Deny))
	assert.False(t, bundle.Authorizer.Authorize("alice", "", "show version", true))
}

// VALIDATES: no system.authorization store retains the existing permissive
// post-authentication behavior.
func TestLiveLocalAuthorizerNilStoreAllows(t *testing.T) {
	resetAAABundleForTest(t)
	swapLocalAuthzStore(nil)

	authorizer := liveLocalAuthorizer{}
	assert.True(t, authorizer.Authorize("alice", "", "show version", true))
	assert.True(t, authorizer.AuthorizeCommandArgs("alice", "", "show bgp rib", nil, "192.0.2.1", true))
}

// VALIDATES: daemon shutdown clears local policy state along with the bundle,
// isolating the next daemon or test run.
func TestCloseAAABundleClearsLiveLocalAuthorization(t *testing.T) {
	resetAAABundleForTest(t)
	swapLocalAuthzStore(localAuthzStoreForTest("alice", authz.Deny))
	require.False(t, (liveLocalAuthorizer{}).Authorize("alice", "", "show version", true))

	closeAAABundle(nil)
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
