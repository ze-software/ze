package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/asn"
)

// configureNotation records one notation for the rest of a test.
func configureNotation(t *testing.T, notation asn.Notation) {
	t.Helper()
	if err := asn.Configure(notation.String()); err != nil {
		t.Fatalf("asn.Configure(%s): %v", notation, err)
	}
}

// candidateTree returns a full config tree whose bgp block selects a notation.
func candidateTree(token string) map[string]any {
	bgp := makeBGPTree(map[string]testPeer{
		"peer1": {remoteIP: "10.0.0.1", remoteAS: "65001", localAS: "65000"},
	})
	bgp[asn.LeafName] = token
	return map[string]any{"bgp": bgp}
}

// TestVerifyingACandidateOnTheTreeBranchLeavesTheRenderedNotation proves the
// verify phase changes no rendering when the reactor parses the tree it was
// handed. The method verifies a candidate selecting asdot+ on a process
// rendering asplain, and reads the notation back.
//
// This covers ONE of the two branches loadPeersFullOrTree takes. A reactor
// with no config path and no reload function parses the tree directly
// (PeersFromTree, config.go), which reaches neither ResolveBGPTree nor the
// reload closure. Those two are where this write lived in rounds 1 and 2, so
// this test cannot see that defect return, and it does not claim to.
// TestReloadVerifyLeavesTheRenderedNotation (../config) covers the file
// branch, and its forced red is what proves the guard.
//
// VALIDATES: the tree branch of loadPeersFullOrTree records nothing.
// PREVENTS: a future write inside PeersFromTree or its callers.
func TestVerifyingACandidateOnTheTreeBranchLeavesTheRenderedNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationPlain)

	cfg := &Config{ListenAddr: "127.0.0.1:0", Standalone: true}
	r := New(cfg)
	if err := r.Start(); err != nil {
		t.Fatalf("start reactor: %v", err)
	}
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	candidate, ok := candidateTree(asn.TokenDotPlus)["bgp"].(map[string]any)
	if !ok {
		t.Fatal("candidateTree built no bgp block")
	}

	if err := adapter.VerifyConfig(candidate); err != nil {
		t.Fatalf("verifying the candidate failed for an unrelated reason: %v", err)
	}
	// Nothing calls SetConfigTree, so the commit is refused. The notation an
	// operator reads must still be the one the running configuration selects.
	if got := asn.Configured(); got != asn.NotationPlain {
		t.Errorf("the verify phase changed the rendered notation to %v, want %v unchanged", got, asn.NotationPlain)
	}
}

// TestTheRunningConfigRecordsTheNotation proves the value DOES change when a
// candidate becomes the running configuration, so the fix above did not
// disconnect the leaf. The method calls the last step of a successful reload.
//
// VALIDATES: SetConfigTree records the notation the running config selects.
// PREVENTS: a test that passes because nothing writes the value at all.
func TestTheRunningConfigRecordsTheNotation(t *testing.T) {
	restore := asn.Configured()
	t.Cleanup(func() { configureNotation(t, restore) })
	configureNotation(t, asn.NotationPlain)

	cfg := &Config{ListenAddr: "127.0.0.1:0", Standalone: true}
	r := New(cfg)
	if err := r.Start(); err != nil {
		t.Fatalf("start reactor: %v", err)
	}
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}

	adapter.SetConfigTree(candidateTree(asn.TokenDotPlus))
	if got := asn.Configured(); got != asn.NotationDotPlus {
		t.Errorf("the running config recorded %v, want %v", got, asn.NotationDotPlus)
	}

	adapter.SetConfigTree(candidateTree(asn.TokenPlain))
	if got := asn.Configured(); got != asn.NotationPlain {
		t.Errorf("the running config recorded %v, want %v", got, asn.NotationPlain)
	}

	// A tree with no bgp block leaves the previous value in place rather than
	// resetting it. Such a tree never reached the loader, so the notation
	// already in force is the better answer.
	configureNotation(t, asn.NotationDot)
	adapter.SetConfigTree(map[string]any{})
	if got := asn.Configured(); got != asn.NotationDot {
		t.Errorf("a tree with no bgp block changed the notation to %v, want %v", got, asn.NotationDot)
	}
}
