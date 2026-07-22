package infra

import (
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
)

// These are internal tests (package infra, not infra_test) on purpose: they
// drive the seam through its unregistered state, which no external caller can
// reach once the gated BGP engine has filled it from init(). The extractor
// tests in authz_test.go / ssh_test.go are external because they need
// plugin/all for the YANG schema.

// TestResolveBGPTreeWithoutEngineIsEmptyNotNil pins the no-engine contract for a
// config that carries no BGP at all -- every consumer of the seam then reads and
// ranges the result exactly as it would a real one.
//
// VALIDATES: with no resolver registered and no bgp{} in the tree,
// ResolveBGPTree returns an empty non-nil map and no error.
// PREVENTS: a BGP-less build failing `ze config dump` on a perfectly valid
// bgp-free config, or handing callers a nil they must special-case.
func TestResolveBGPTreeWithoutEngineIsEmptyNotNil(t *testing.T) {
	restore := swapResolver(nil)
	defer restore()

	got, err := ResolveBGPTree(config.NewTree())
	if err != nil {
		t.Fatalf("ResolveBGPTree with no engine and no bgp block: %v", err)
	}
	if got == nil {
		t.Fatal("ResolveBGPTree returned a nil map; callers must be able to range it")
	}
	if len(got) != 0 {
		t.Errorf("ResolveBGPTree = %v, want empty", got)
	}
}

// TestResolveBGPTreeWithoutEngineRejectsBGPConfig is the fail-closed half.
//
// VALIDATES: a tree carrying a bgp{} block with no resolver registered is an
// ERROR, not a silent empty resolution.
// PREVENTS: the drift case where the bgp{} YANG schema is linked but the engine
// is not -- `ze config dump` would print a config with its whole BGP section
// missing, and `ze config validate` would report it valid having checked none
// of it (ai/rules/fail-closed-guards.md).
func TestResolveBGPTreeWithoutEngineRejectsBGPConfig(t *testing.T) {
	restore := swapResolver(nil)
	defer restore()

	tree := config.NewTree()
	tree.SetContainer("bgp", config.NewTree())

	got, err := ResolveBGPTree(tree)
	if err == nil {
		t.Fatalf("ResolveBGPTree accepted a bgp block with no engine, returned %v", got)
	}
	if !strings.Contains(err.Error(), "without the BGP engine") {
		t.Errorf("error = %v, want it to name the missing engine", err)
	}
}

// TestResolveBGPTreeUsesRegisteredResolver proves the seam actually delegates,
// so the two tests above are describing a real fallback and not the only path.
//
// VALIDATES: with a resolver registered, ResolveBGPTree returns its result --
// including for a tree with a bgp{} block, which the no-engine path rejects.
// PREVENTS: a seam that always takes its fallback branch, which would make the
// fail-closed test above pass while BGP config was never resolved at all.
func TestResolveBGPTreeUsesRegisteredResolver(t *testing.T) {
	want := map[string]any{"peer": "sentinel"}
	var sawTree bool
	restore := swapResolver(func(tree *config.Tree) (map[string]any, error) {
		sawTree = tree != nil
		return want, nil
	})
	defer restore()

	tree := config.NewTree()
	tree.SetContainer("bgp", config.NewTree())

	got, err := ResolveBGPTree(tree)
	if err != nil {
		t.Fatalf("ResolveBGPTree with a registered resolver: %v", err)
	}
	if !sawTree {
		t.Error("resolver was not handed the tree")
	}
	if got["peer"] != "sentinel" {
		t.Errorf("ResolveBGPTree = %v, want the registered resolver's result %v", got, want)
	}
}

// TestValidateBGPPeersWithoutEngineIsNoOp pins the peer-validation seam: with no
// engine there is nothing to validate, and reporting a failure would block every
// config on a BGP-less build.
func TestValidateBGPPeersWithoutEngineIsNoOp(t *testing.T) {
	prev := bgpPeerValidator
	bgpPeerValidator = nil
	defer func() { bgpPeerValidator = prev }()

	if err := ValidateBGPPeers(config.NewTree()); err != nil {
		t.Errorf("ValidateBGPPeers with no engine = %v, want nil", err)
	}
}

// TestWriteGRMarkerSeam pins the graceful-restart seam in both states: the
// registered writer is called, and with no engine the reboot path is a silent
// no-op rather than a nil-hook panic (a BGP-less daemon has no session for a
// peer to treat as restarting).
//
// VALIDATES: WriteGRMarker delegates when a writer is registered and does
// nothing when it is not.
// PREVENTS: a reboot crashing a BGP-less daemon on the nil hook, and the
// opposite failure of a seam that never delegates -- which would make the
// no-engine case pass while the marker was never written on a real build.
func TestWriteGRMarkerSeam(t *testing.T) {
	prev := grMarkerWriter
	defer func() { grMarkerWriter = prev }()

	var called int
	grMarkerWriter = func([]plugin.InjectedCapability, storage.Storage) { called++ }
	WriteGRMarker(nil, nil)
	if called != 1 {
		t.Fatalf("registered GR-marker writer called %d times, want 1", called)
	}

	grMarkerWriter = nil
	WriteGRMarker(nil, nil)
	if called != 1 {
		t.Errorf("GR-marker writer called %d times after the seam was cleared, want 1", called)
	}
}

// swapResolver installs fn as the BGP tree resolver and returns a restore func,
// so a test can drive the unregistered state without leaking it to its siblings
// (the gated engine registers the real one from init() in a full-feature build).
func swapResolver(fn BGPTreeResolver) func() {
	prev := bgpTreeResolver
	bgpTreeResolver = fn
	return func() { bgpTreeResolver = prev }
}
