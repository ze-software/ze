// Design: docs/architecture/config/syntax.md -- BGP is an optional subsystem.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// TestRuntimePeerBuildersAcceptNoBGPSection keeps a non-BGP daemon's real tree
// intact through boot, the initial rebuild and candidate attachment resolution.
// ResolveBGPTree itself remains strict for callers explicitly requesting BGP.
func TestRuntimePeerBuildersAcceptNoBGPSection(t *testing.T) {
	tree, err := config.ParseTreeForValidation(`
l2tp {
 enabled false;
 pool { ipv4 { gateway 10.100.0.1; start 10.100.0.10; end 10.100.0.10; } }
}
bus {
 endpoint pool-source { internal fakel2tp; }
 endpoint pool-observer { external { run "le test fixture reload/l2tp-pool-range"; } }
}`)
	require.NoError(t, err)
	peers, groups, err := peerPopulationsFromTree(tree.Clone())
	require.NoError(t, err)
	require.Empty(t, peers)
	require.Empty(t, groups)
	peers, globals, groups, err := reloadPeersFromTree(tree.Clone())
	require.NoError(t, err)
	require.Empty(t, peers)
	require.Empty(t, groups)
	require.Equal(t, reactor.Globals{}, globals)
	grants, err := disseminationPeersFromTree(tree.ToRunningMap())
	require.NoError(t, err)
	require.Empty(t, grants)
	require.Nil(t, tree.GetContainer("bgp"), "absence must not invent configuration")
	require.NotNil(t, tree.GetContainer("l2tp"), "the unrelated subsystem stays configured")
	_, err = ResolveBGPTree(tree)
	require.ErrorIs(t, err, errMissingRequiredBgpBlock)
}
