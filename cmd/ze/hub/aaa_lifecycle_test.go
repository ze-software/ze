package hub

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/aaa"
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

func (stubAuthn) Authenticate(_, _ string) (aaa.AuthResult, error) {
	return aaa.AuthResult{}, errors.New("stub")
}

func buildStubBundle(t *testing.T, closedFlag *bool) *aaa.Bundle {
	t.Helper()
	// Throw-away registry per bundle so nothing leaks into aaa.Default and
	// each test's close-tracking flag stays isolated.
	r := aaa.NewBackendRegistry()
	require.NoError(t, r.Register(&stubBackendForSwap{name: "stub", closed: closedFlag}))
	built, err := r.Build(aaa.BuildParams{})
	require.NoError(t, err)
	return built
}

// resetAAABundleForTest snapshots the pre-test aaaBundle, clears the slot
// for the test body, and on cleanup: (1) closes whatever bundle the test
// installed, surfacing any Close error to t.Log so failures are visible;
// (2) restores the pre-test bundle so later tests see the prior state
// instead of a cleared slot.
//
// If a pre-existing bundle is found, that means an earlier test installed
// a bundle and never cleaned up (likely crashed or skipped this helper).
// We log a warning so the leak is visible -- the bad state would otherwise
// propagate silently to every subsequent test in the binary.
func resetAAABundleForTest(t *testing.T) {
	t.Helper()
	pre := aaaBundle.Swap(nil)
	if pre != nil {
		t.Logf("aaa bundle leak: pre-test slot was non-nil; an earlier test did not clean up")
	}
	t.Cleanup(func() {
		if testBundle := aaaBundle.Swap(pre); testBundle != nil {
			if err := testBundle.Close(); err != nil {
				t.Logf("aaa bundle close error during cleanup: %v", err)
			}
		}
	})
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
