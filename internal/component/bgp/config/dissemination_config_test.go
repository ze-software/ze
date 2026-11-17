// Design: docs/architecture/api/process-protocol.md -- candidate peer grants.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDisseminationCandidateUsesResolvedBindings proves the registered producer
// reaches inherited explicit attachments with their original directions, while
// leaving the caller's candidate tree intact. Runtime metadata is judged later.
func TestDisseminationCandidateUsesResolvedBindings(t *testing.T) {
	tree, err := config.ParseTreeForValidation(`
bgp {
 router-id 192.0.2.254;
 session { asn { local 65000; } }
 group sources {
  attach process external-table { receive [ update state ]; }
  peer source {
   connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
   session { asn { remote 65001; } }
  }
 }
 peer destination {
  connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
  session { asn { remote 65002; } }
  attach process external-table { receive [ state refresh update-sent ]; send [ update ]; }
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
		var attachment *plugin.PeerProcessBinding
		for i := range peer.Bindings {
			if peer.Bindings[i].PluginName == "external-table" {
				attachment = &peer.Bindings[i]
			}
		}
		require.NotNil(t, attachment, "default capability bindings must not hide the explicit attachment")
		require.True(t, attachment.Explicit)
		if peer.Name == "destination" {
			require.True(t, attachment.Send["update"])
			continue
		}
		require.Equal(t, "source", peer.Name)
		require.False(t, attachment.Send["update"])
	}
}
