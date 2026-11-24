// Design: docs/architecture/api/process-protocol.md -- candidate peer grants.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestDisseminationCandidateUsesResolvedBindings proves the registered producer
// reaches inherited group bus blocks with their resolved target sets while
// leaving the caller's candidate map intact.
//
// VALIDATES: disseminationPeersFromTree inherits group > bus rules onto group
// members and preserves peer-specific bus blocks without mutating input.
// PREVENTS: Stage 1 dissemination checks missing group-inherited bus narrowing.
func TestDisseminationCandidateUsesResolvedBindings(t *testing.T) {
	tree, err := config.ParseTreeForValidation(`
bus {
 endpoint external-table {
  external { run "/bin/true"; }
  receive { allow [ bgp:update bgp:state bgp:refresh ]; }
  send { allow [ bgp:update ]; }
 }
}
bgp {
 router-id 192.0.2.254;
 session { asn { local 65000; } }
 group sources {
  bus { send { to [ /bus/endpoint/external-table ]; } }
  peer source {
   connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
   session { asn { remote 65001; } }
  }
 }
 peer destination {
  connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
  session { asn { remote 65002; } }
  bus { receive { from [ /bus/endpoint/external-table ]; } }
 }
}`)
	require.NoError(t, err)
	input := tree.ToRunningMap()
	before := tree.ToRunningMap()
	peers, err := disseminationPeersFromTree(input)
	require.NoError(t, err)
	require.Len(t, peers, 2)
	require.Equal(t, before, input)
	for _, peer := range peers {
		if peer.Name == "destination" {
			require.Equal(t, "/bgp/peer/destination", peer.Path)
			require.True(t, peer.Bus.Receive.TargetsPresent)
			require.Equal(t, []string{"/bus/endpoint/external-table"}, peer.Bus.Receive.Targets)
			continue
		}
		require.Equal(t, "source", peer.Name)
		require.Equal(t, "/bgp/group/sources/peer/source", peer.Path)
		require.Equal(t, "/bgp/group/sources", peer.ParentPath)
		require.Equal(t, pluginserver.BusOriginInherited, peer.BusOrigin)
		require.True(t, peer.Bus.Send.TargetsPresent)
		require.Equal(t, []string{"/bus/endpoint/external-table"}, peer.Bus.Send.Targets)
	}
}
