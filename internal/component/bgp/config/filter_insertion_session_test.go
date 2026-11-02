// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: filter_insertion.go -- insertionHolds, the session condition at load
// Related: templated_chain.go -- templatedChain.keepHolding, the condition per dynamic peer

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// bilateralEBGP is the session condition of RFC 7999 Section 3.1's
// withholding (spec-filter-wire-6-stage-filters Q-5): every eBGP session that
// is not a route-server client.
var bilateralEBGP = []rpc.FilterSessionFact{
	{Fact: "ibgp", Values: []string{"false"}},
	{Fact: "rs-client", Values: []string{"false"}},
}

// TestSessionConditionInsertion checks spec-filter-wire-6-stage-filters AC-1
// for a session condition at the loader: the member lands on exactly the
// peers whose session facts meet every fact of the condition. Method: an
// export insertion on ibgp false and rs-client false; peer A is iBGP (remote
// AS 65000 = local), peer B is eBGP (65002) and no route-server client.
func TestSessionConditionInsertion(t *testing.T) {
	t.Run("every fact holds", func(t *testing.T) {
		registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterExport,
			Priority: 5, Session: bilateralEBGP})
		peers, err := insertionPeers(t, "")
		require.NoError(t, err)
		require.Contains(t, refNames(peers["B"].ExportPolicy.Filters()), "fake-inserter:late",
			"peer B is a bilateral eBGP session, so its export list carries the member")
		require.NotNil(t, peers["B"].ExportPolicy.Library(), "the inserted member runs in the peer's library chain")
		require.NotContains(t, refNames(peers["B"].ImportPolicy.Filters()), "fake-inserter:late",
			"an export insertion never reaches the import list")
		require.NotContains(t, refNames(peers["A"].ExportPolicy.Filters()), "fake-inserter:late",
			"peer A is iBGP, so ibgp false does not hold and nothing is inserted")
	})
	t.Run("an AS number in decimal", func(t *testing.T) {
		registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterExport,
			Priority: 5, Session: []rpc.FilterSessionFact{{Fact: "peer-as", Values: []string{"65002"}}}})
		peers, err := insertionPeers(t, "")
		require.NoError(t, err)
		require.Contains(t, refNames(peers["B"].ExportPolicy.Filters()), "fake-inserter:late",
			"peer B's AS is 65002")
		require.NotContains(t, refNames(peers["A"].ExportPolicy.Filters()), "fake-inserter:late",
			"peer A's AS is 65000")
	})
}

// TestSessionConditionInsertionPerDynamicPeer checks the same condition on a
// dynamic group, whose template states no remote AS: the member is placed in
// the template, which compiles per peer, and each peer keeps it only when its
// OPEN's AS makes the condition hold. Method: the group's import list gets the
// member on ibgp false; the template is resolved for an eBGP AS and for the
// local AS.
func TestSessionConditionInsertionPerDynamicPeer(t *testing.T) {
	registerInserter(t, rpc.FilterInsertion{Filter: "fake-inserter:late", Direction: rpc.FilterImport,
		Priority: 5, Session: bilateralEBGP[:1]})

	groups, err := loadTemplated(t, "ALLOW-65010", "")
	require.NoError(t, err)
	require.Len(t, groups, 1)
	template := groups[0].Settings.ImportPolicy
	require.Contains(t, refNames(template.Filters()), "fake-inserter:late",
		"the template carries the member in its place until a peer's facts are known")
	require.True(t, template.Templated(), "the template compiles per peer")

	external, err := template.CompileResolved(groups[0].Settings.SessionFacts(65010))
	require.NoError(t, err)
	require.Equal(t, []string{"fake-inserter:late", "ALLOW-65010"}, refNames(external.Filters()),
		"an eBGP peer of the group keeps the member ahead of the operator's")
	require.NotNil(t, external.Library())

	internal, err := template.CompileResolved(groups[0].Settings.SessionFacts(65000))
	require.NoError(t, err)
	require.Equal(t, []string{"ALLOW-65010"}, refNames(internal.Filters()),
		"an iBGP peer of the group drops the member")
}

// TestInsertionOnlyWhereItsPluginRuns checks that a compiled-in plugin's
// insertion, under a config or a session condition, applies only in a daemon
// that runs the plugin, configured or auto-loaded: the plugin that owns the
// procedure is its scope, and the condition then picks the peers. Method: the
// plugin server's seam (registry.SetRuntimeLoadedPlugins) answers a set
// without the inserting plugin, then one with it; peer B meets both
// conditions throughout.
func TestInsertionOnlyWhereItsPluginRuns(t *testing.T) {
	conditions := map[string]rpc.FilterInsertion{
		"session condition": {Filter: "fake-inserter:late", Direction: rpc.FilterExport, Priority: 5, Session: bilateralEBGP},
		"config condition":  {Filter: "fake-inserter:late", Direction: rpc.FilterImport, Priority: 5, Config: rrClient},
	}
	for name, ins := range conditions {
		t.Run(name, func(t *testing.T) {
			registerInserter(t, ins)
			t.Cleanup(func() { registry.SetRuntimeLoadedPlugins(nil) })
			peer := "B"
			if ins.Direction == rpc.FilterImport {
				peer = "A"
			}
			list := func(ps *reactor.PeerSettings) []string {
				if ins.Direction == rpc.FilterImport {
					return refNames(ps.ImportPolicy.Filters())
				}
				return refNames(ps.ExportPolicy.Filters())
			}

			registry.SetRuntimeLoadedPlugins(func() []string { return []string{"some-other-plugin"} })
			peers, err := insertionPeers(t, "")
			require.NoError(t, err)
			require.NotContains(t, list(peers[peer]), "fake-inserter:late",
				"the daemon does not run fake-inserter, so its member is placed nowhere")

			registry.SetRuntimeLoadedPlugins(func() []string { return []string{"fake-inserter"} })
			peers, err = insertionPeers(t, "")
			require.NoError(t, err)
			require.Contains(t, list(peers[peer]), "fake-inserter:late",
				"the daemon runs fake-inserter, so the peer meeting the condition carries its member")
		})
	}
}
