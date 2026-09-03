package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withPeerValidator installs a peer validator on the infra seam for the duration
// of a test and restores it to nil. Restoring nil rather than the previous value
// is correct HERE and only here: the doctor test binary does not link
// internal/component/bgp/config (confirmed with `go list -deps`), so its init()
// never runs and the seam starts nil. A test that wants the rejecting branch
// must therefore supply its own.
func withPeerValidator(t *testing.T, fn func(*config.Tree) error) {
	t.Helper()
	infra.SetBGPPeerValidator(fn)
	t.Cleanup(func() { infra.SetBGPPeerValidator(nil) })
}

// bgpTreeWithPeer builds the smallest tree that carries a bgp{} block with one
// peer, which is what gates checkBGPPeerConfig.
//
// `family` is a LIST keyed by the family name, so it is built with AddListEntry.
// Building it as a container would re-enshrine the exact shape this work found
// broken in containerPeersLabeled (checks_linux.go) -- harmless here, because
// the stub validator ignores the tree, which is precisely why it would rot
// unnoticed.
func bgpTreeWithPeer() *config.Tree {
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	session := peer.GetOrCreateContainer("session")
	session.AddListEntry("family", "ipv4/mpls-label", config.NewTree())
	bgp.AddListEntry("peer", "peer1", peer)
	return tree
}

// VALIDATES: checkBGPPeerConfig does not mutate the caller's tree.
// PREVENTS: the defect this check shipped with. ValidateBGPPeers reaches
// PeersFromConfigTree, which calls config.PruneInactive -- an in-place mutation
// (internal/component/config/prune.go:168). Doctor's tree is shared by ~30 later
// checks in runChecks, so validating in place deleted every `inactive:` node from
// underneath them: an inactive peer with an unbindable local ip silently stopped
// producing doctor-bgp-listen, and only when a bgp{} block was present, making
// the whole report order- and content-dependent.
func TestCheckBGPPeerConfigDoesNotMutateTree(t *testing.T) {
	var seen *config.Tree
	withPeerValidator(t, func(tr *config.Tree) error {
		seen = tr
		// Stand in for what PruneInactive does to whatever it is handed.
		tr.RemoveContainer("bgp")
		return nil
	})

	tree := bgpTreeWithPeer()
	checkBGPPeerConfig(tree)

	require.NotNil(t, seen, "the validator must actually have been called")
	assert.Nil(t, seen.GetContainer("bgp"), "the stub must really have mutated what it was given")
	assert.NotNil(t, tree.GetContainer("bgp"),
		"checkBGPPeerConfig must hand the validator a CLONE: the caller's tree is "+
			"shared by every later check in runChecks")
}

// VALIDATES: `ze doctor` reports an error diagnostic when the BGP engine's own
// peer resolution rejects the configuration, instead of answering ready.
// PREVENTS: the defect test/plugin/mpls-doctor.ci ran into on CI -- a config
// naming a family that does not exist (`ipv4/mpls-unicast`) was rejected by
// `ze config validate` with "unknown address family" while `ze doctor --json`
// reported "ready": true and exited 0. An operator checking readiness before
// starting the daemon got a clean bill of health for a config the daemon
// refuses to load.
func TestCheckBGPPeerConfigReportsRejection(t *testing.T) {
	withPeerValidator(t, func(*config.Tree) error {
		return errors.New(`peer peer1: unknown address family "ipv4/mpls-unicast"`)
	})

	diags := checkBGPPeerConfig(bgpTreeWithPeer())

	require.Len(t, diags, 1, "a rejected peer config must produce exactly one diagnostic")
	assert.Equal(t, "doctor-config-bgp-peer", diags[0].Code)
	// ERROR: a config the engine refuses means the daemon will not start, so the
	// report must not be ready and `ze doctor` must exit non-zero. Anything
	// weaker lets a readiness check bless a config that cannot run.
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "unknown address family",
		"the diagnostic must carry the engine's own reason, not just a generic failure")
}

// VALIDATES: an accepted peer config produces no diagnostic, and a tree with no
// bgp{} block never consults the validator at all.
// PREVENTS: a false error on every non-BGP config, and a spurious readiness
// failure once the check is in the default doctor sequence.
func TestCheckBGPPeerConfigSilentWhenValid(t *testing.T) {
	t.Run("valid-config", func(t *testing.T) {
		withPeerValidator(t, func(*config.Tree) error { return nil })
		assert.Empty(t, checkBGPPeerConfig(bgpTreeWithPeer()))
	})

	t.Run("no-bgp-block", func(t *testing.T) {
		called := false
		withPeerValidator(t, func(*config.Tree) error {
			called = true
			return errors.New("must not be consulted")
		})
		assert.Empty(t, checkBGPPeerConfig(config.NewTree()))
		assert.False(t, called, "a config with no bgp{} block must not reach the peer validator")
	})

	t.Run("nil-tree", func(t *testing.T) {
		withPeerValidator(t, func(*config.Tree) error { return errors.New("must not be consulted") })
		assert.Empty(t, checkBGPPeerConfig(nil))
	})
}

// VALIDATES: with the BGP engine compiled out the infra seam is nil and the
// check stays silent rather than erroring.
// PREVENTS: a `ze_bgp`-off build reporting a BGP diagnostic it cannot evaluate.
// This is the fail-OPEN case that is correct: no engine means no peers to
// reject, which is different from an engine that could not be asked.
func TestCheckBGPPeerConfigSilentWithoutEngine(t *testing.T) {
	withPeerValidator(t, nil)
	assert.Empty(t, checkBGPPeerConfig(bgpTreeWithPeer()))
}

// withRolelessPeerReporter installs a roleless-peer reporter on the infra seam
// for one test and restores it to nil, for the reason withPeerValidator gives:
// this test binary does not link internal/component/bgp/config, so the seam
// starts nil and a test that wants an answer supplies it.
func withRolelessPeerReporter(t *testing.T, fn func(*config.Tree) []string) {
	t.Helper()
	infra.SetBGPRolelessPeerReporter(fn)
	t.Cleanup(func() { infra.SetBGPRolelessPeerReporter(nil) })
}

// VALIDATES: AC-35. `ze doctor` enumerates every peer and dynamic group that
// declares no RFC 9234 role, under its own diagnostic code, as a warning.
// PREVENTS: the role gap staying invisible on the operator-facing readiness
// path. A peer with no role is accepted and owes no transit-leak filter, so
// nothing else in the report would mention it.
func TestCheckBGPPeersWithoutRoleReportsEveryPeer(t *testing.T) {
	withRolelessPeerReporter(t, func(*config.Tree) []string {
		return []string{"peer1", "peer2"}
	})

	diags := checkBGPPeersWithoutRole(bgpTreeWithPeer())
	require.Len(t, diags, 1, "one aggregated diagnostic, never one per peer")
	assert.Equal(t, diagnosticBGPPeerNoRole, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity,
		"the config loads and the daemon starts on it, so this is not an error")
	assert.Contains(t, diags[0].Message, "peer1")
	assert.Contains(t, diags[0].Message, "peer2")
}

// VALIDATES: a config whose peers all declare a role produces no diagnostic,
// and a tree with no bgp block is not even asked about.
// PREVENTS: a permanent warning that operators learn to ignore.
func TestCheckBGPPeersWithoutRoleSilentWhenEveryPeerDeclaresOne(t *testing.T) {
	asked := false
	withRolelessPeerReporter(t, func(*config.Tree) []string {
		asked = true
		return nil
	})

	assert.Empty(t, checkBGPPeersWithoutRole(bgpTreeWithPeer()))
	assert.True(t, asked, "the seam is consulted for a tree that carries a bgp block")

	asked = false
	assert.Empty(t, checkBGPPeersWithoutRole(config.NewTree()))
	assert.False(t, asked, "a config with no bgp block has no peer to report")
}

// VALIDATES: checkBGPPeersWithoutRole does not mutate the caller's tree.
// PREVENTS: the defect checkBGPPeerConfig shipped with. The reporter prunes
// inactive nodes in place, and doctor's tree is shared by ~30 later checks.
func TestCheckBGPPeersWithoutRoleDoesNotMutateTree(t *testing.T) {
	withRolelessPeerReporter(t, func(tr *config.Tree) []string {
		// Stand in for what PruneInactive does to whatever it is handed.
		tr.RemoveContainer("bgp")
		return nil
	})

	tree := bgpTreeWithPeer()
	checkBGPPeersWithoutRole(tree)
	assert.NotNil(t, tree.GetContainer("bgp"), "the caller's tree must survive the check")
}
