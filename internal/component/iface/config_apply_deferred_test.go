package iface

import (
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// errFakeNotReady is the answer ifacevpp.ensureChannel gives before the GoVPP
// handshake completes, wrapped the way the real backend wraps it.
var errFakeNotReady = fmt.Errorf("ifacevpp: VPP connector not ready: %w", ErrBackendNotReady)

// clearOwedApply drops an apply a test left owed, so the package-level state
// cannot reach the next test's vpp-ready handler.
func clearOwedApply(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		applyMu.Lock()
		owedApply = deferredApply{}
		applyMu.Unlock()
	})
}

// TestDummyCreatedBeforeVPPConnectsWaitsForTheConnection verifies that a
// dummy whose create the backend refuses as not ready is created once vpp
// connects, instead of failing the apply.
//
// VALIDATES: a not-ready create defers the whole apply without an error, and
// the vpp-ready handler performs it in full: the loopback, its LCP pair, its
// admin state and its addresses.
// PREVENTS: the measured defect. On the memory store the daemon reached the
// interface Config stage 1 ms after vpp began connecting, CreateDummy answered
// ErrBackendNotReady, applyConfig failed the stage, and the daemon exited
// (test/plugin vpp-loopback-reapply, vpp-loopback-reload-create).
func TestDummyCreatedBeforeVPPConnectsWaitsForTheConnection(t *testing.T) {
	clearOwedApply(t)
	fb := setupFakeBackendForTest(t)
	fb.createDummyErr = map[string]error{"dum0": errFakeNotReady}
	cfg := testConfigWithAddresses()

	errs := applyConfig(cfg, nil, fb)

	require.Empty(t, errs, "a backend that is not ready yet must not fail the apply")
	require.False(t, fb.created["dum0"], "nothing can be created before the backend connects")
	require.True(t, applyIsOwed(), "the refused apply must be owed to the connection")

	// The handshake completes: the backend now answers.
	fb.createDummyErr = nil
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)
	reconcileOnVPPConnected(&activeCfg)

	require.True(t, fb.created["dum0"], "the deferred dummy must be created when vpp connects")
	require.Equal(t, "dum0", fb.lcpPairs["dum0"], "the loopback must get its LCP pair")
	require.Equal(t, "up", fb.adminSet["dum0"], "the loopback must be brought up")
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
	require.False(t, applyIsOwed(), "a performed apply is no longer owed")
}

// TestDummyCreateStillNotReadyStaysOwed verifies that a vpp event whose
// backend still refuses the apply leaves it owed for the next event.
//
// VALIDATES: an owed apply survives a pass the backend refuses again.
// PREVENTS: an early or spurious ready signal consuming the only record of
// the apply, so the dummy is never created.
func TestDummyCreateStillNotReadyStaysOwed(t *testing.T) {
	clearOwedApply(t)
	fb := setupFakeBackendForTest(t)
	fb.createDummyErr = map[string]error{"dum0": errFakeNotReady}
	cfg := testConfigWithAddresses()

	require.Empty(t, applyConfig(cfg, nil, fb))
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)
	reconcileOnVPPConnected(&activeCfg)

	require.False(t, fb.created["dum0"])
	require.True(t, applyIsOwed(), "a pass the backend refused again must leave the apply owed")
}

// TestDummyCreateFailureStillFailsTheApply verifies that only the not-ready
// answer defers: any other create error fails the apply as before.
//
// VALIDATES: a real backend error is returned and nothing is owed.
// PREVENTS: the deferral swallowing a genuine failure, which would report a
// commit as applied when its dummy can never be made.
func TestDummyCreateFailureStillFailsTheApply(t *testing.T) {
	clearOwedApply(t)
	realErr := errors.New("netlink: rtnetlink receive: permission denied")
	fb := setupFakeBackendForTest(t)
	fb.createDummyErr = map[string]error{"dum0": realErr}

	errs := applyConfig(testConfigWithAddresses(), nil, fb)

	require.NotEmpty(t, errs)
	require.ErrorIs(t, errs[0], realErr)
	require.False(t, applyIsOwed(), "a failed apply is not owed")
}

// TestLaterApplySupersedesOwedApply verifies that an apply arriving before
// the connection replaces the owed one, and is measured against what the
// dataplane was last told rather than against the owed config.
//
// VALIDATES: the later commit's config is the one that lands, and the owed
// config is never applied on top of it.
// PREVENTS: the vpp-ready handler re-applying a config the operator has
// already replaced.
func TestLaterApplySupersedesOwedApply(t *testing.T) {
	clearOwedApply(t)
	fb := setupFakeBackendForTest(t)
	fb.createDummyErr = map[string]error{"dum0": errFakeNotReady}
	first := testConfigWithAddresses()
	require.Empty(t, applyConfig(first, nil, fb))

	fb.createDummyErr = nil
	second := &ifaceConfig{Backend: vppBackendName, Dummy: []ifaceEntry{{Name: "dum1"}}}
	require.Empty(t, applyConfig(second, first, fb))
	require.True(t, fb.created["dum1"])
	require.False(t, applyIsOwed(), "the later apply supersedes the owed one")
	require.Empty(t, second.previousManaged,
		"the owed config never reached the dataplane, so it owns nothing the later apply may prune")

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(second)
	reconcileOnVPPConnected(&activeCfg)
	require.False(t, fb.created["dum0"], "the superseded config must never be applied")
}

// TestReconcileOnVPPConnectedKeepsTheBackend verifies that the first
// connection of the process does not replace the backend, while a reconnect
// still does.
//
// VALIDATES: EventConnected runs against the backend it finds;
// reconcileOnVPPReady (EventReconnected) still reloads it.
// PREVENTS: the owed apply's loopbacks being forgotten. The vpp backend knows
// a loopback by its ze name only through its own name map, so a second
// connect-time pass that reloaded the backend (the post-subscription hand-off
// and EventConnected can both run) made the next apply create every loopback
// again.
func TestReconcileOnVPPConnectedKeepsTheBackend(t *testing.T) {
	clearOwedApply(t)
	var factoryCalls int
	err := RegisterBackend(vppBackendName, func() (Backend, error) {
		factoryCalls++
		return &fakeBackend{ifaces: map[string]fakeIface{}}, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, vppBackendName)
		backendsMu.Unlock()
	})
	require.NoError(t, LoadBackend(vppBackendName))
	first := GetBackend()

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(testConfigWithAddresses())

	reconcileOnVPPConnected(&activeCfg)
	reconcileOnVPPConnected(&activeCfg)
	require.Equal(t, 1, factoryCalls, "a connect must not reload the backend")
	require.Same(t, first, GetBackend())

	reconcileOnVPPReady(&activeCfg)
	require.Equal(t, 2, factoryCalls, "a reconnect must reload the backend")
}

// TestOwedApplyThatFailsStaysOwed verifies that an owed apply the backend
// fails with a real error, once connected, stays owed, so the next commit is
// measured against what the dataplane was last told.
//
// VALIDATES: after the failed owed apply, applyIsOwed still answers true, and
// a later apply that names the owed config as previous is measured against
// the owed apply's own previous (nil here), so it inherits nothing the owed
// config would have owned.
// PREVENTS: the failed owed apply being forgotten, after which the next
// commit treats a config that never reached the dataplane as applied and
// skips or prunes on its account.
func TestOwedApplyThatFailsStaysOwed(t *testing.T) {
	clearOwedApply(t)
	fb := setupFakeBackendForTest(t)
	fb.createDummyErr = map[string]error{"dum0": errFakeNotReady}
	first := testConfigWithAddresses()
	require.Empty(t, applyConfig(first, nil, fb))

	realErr := errors.New("vpp: create_loopback: VNET_API_ERROR_INVALID_VALUE")
	fb.createDummyErr = map[string]error{"dum0": realErr}
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(first)
	reconcileOnVPPConnected(&activeCfg)

	require.False(t, fb.created["dum0"])
	require.True(t, applyIsOwed(), "an owed apply that failed never reached the dataplane, so it is still owed")

	fb.createDummyErr = nil
	second := &ifaceConfig{Backend: vppBackendName, Dummy: []ifaceEntry{{Name: "dum1"}}}
	require.Empty(t, applyConfig(second, first, fb))
	require.True(t, fb.created["dum1"])
	require.False(t, applyIsOwed(), "the later apply supersedes the owed one")
	require.Empty(t, second.previousManaged,
		"the failed owed config owns nothing the later apply may prune")
}

// twoMirrorCfg is a config with two dummies, each mirroring its ingress, so a
// mirror teardown reports one error per source.
func twoMirrorCfg() *ifaceConfig {
	return &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{
			{Name: "mir0", Units: []unitEntry{{MirrorIngress: "cap0"}}},
			{Name: "mir1", Units: []unitEntry{{MirrorIngress: "cap0"}}},
		},
	}
}

// TestMirrorStepDefersOnlyWhenEveryErrorIsNotReady verifies recordAll, the
// answer to a step that reports several errors at once: it defers when every
// error is the not-ready answer, and fails the apply when any one is real.
//
// VALIDATES: two not-ready teardown errors leave the apply owed with no
// error; one not-ready and one real error fail the apply, return the real
// error, and leave nothing owed.
// PREVENTS: a real teardown failure being hidden behind a not-ready sibling,
// which would report the commit as waiting while the kernel keeps copying
// traffic the config no longer asks for.
func TestMirrorStepDefersOnlyWhenEveryErrorIsNotReady(t *testing.T) {
	realErr := errors.New("netlink refused")
	current := &ifaceConfig{
		Backend: "netlink",
		Dummy: []ifaceEntry{
			{Name: "mir0", Units: []unitEntry{{}}},
			{Name: "mir1", Units: []unitEntry{{}}},
		},
	}

	t.Run("all not ready", func(t *testing.T) {
		clearOwedApply(t)
		b := &fakeBackend{}
		previous := twoMirrorCfg()
		require.Empty(t, applyConfig(previous, nil, b))
		b.removeMirrorErr = map[string]error{"mir0": errFakeNotReady, "mir1": errFakeNotReady}

		require.Empty(t, applyConfig(current, previous, b))
		require.True(t, applyIsOwed())
	})

	t.Run("one real", func(t *testing.T) {
		clearOwedApply(t)
		b := &fakeBackend{}
		previous := twoMirrorCfg()
		require.Empty(t, applyConfig(previous, nil, b))
		b.removeMirrorErr = map[string]error{"mir0": errFakeNotReady, "mir1": realErr}

		errs := applyConfig(current, previous, b)
		require.NotEmpty(t, errs, "a real teardown failure must fail the apply")
		require.True(t, slices.ContainsFunc(errs, func(e error) bool { return errors.Is(e, realErr) }))
		require.False(t, applyIsOwed(), "a failed apply is not owed")
	})
}
