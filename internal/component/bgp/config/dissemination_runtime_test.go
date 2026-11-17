// Design: docs/architecture/api/process-protocol.md -- runtime declarations are authoritative.
package bgpconfig

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRuntimePeerBuilderDefersCompiledOwnership keeps offline validation strict
// while a runtime build leaves the authoritative declaration check to its reactor.
func TestRuntimePeerBuilderDefersCompiledOwnership(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	for _, name := range []string{"runtime-owner-one", "runtime-owner-two"} {
		require.NoError(t, registry.Register(registry.Registration{
			Name: name, Description: "compiled ownership differs from runtime",
			RunEngine: func(net.Conn) int { return 0 }, CLIHandler: func([]string) int { return 0 },
			Claims: []string{reactor.SelectedDisseminationRole},
			Dissemination: []rpc.DisseminationDecl{{Namespace: bgpevents.Namespace, Role: reactor.SelectedDisseminationRole,
				SourceReceive: []string{"update-received", "state"}, DestinationReceive: []string{"state", "refresh", "update-sent"}, DestinationSend: []string{"update"}}},
		}))
	}
	tree, err := config.ParseTreeForValidation(`
bgp {
 router-id 192.0.2.254;
 session { asn { local 65000; } }
 peer source {
  connection { remote { ip 192.0.2.1; } local { ip 192.0.2.254; } }
  session { asn { remote 65001; } }
  attach process runtime-owner-one { receive [ update state ]; }
  attach process runtime-owner-two { receive [ update state ]; }
 }
 peer destination {
  connection { remote { ip 192.0.2.2; } local { ip 192.0.2.254; } }
  session { asn { remote 65002; } }
  attach process runtime-owner-one { receive [ state refresh update-sent ]; send [ update ]; }
  attach process runtime-owner-two { receive [ state refresh update-sent ]; send [ update ]; }
 }
}`)
	require.NoError(t, err)
	_, err = PeersFromConfigTree(tree.Clone())
	require.Error(t, err, "offline validation must refuse competing compiled explicit owners")
	peers, _, _, err := reloadPeersFromTree(tree.Clone())
	require.NoError(t, err, "runtime declarations, not compiled metadata, decide ownership later")
	require.Len(t, peers, 2)
	for _, peer := range peers {
		require.False(t, peer.DisseminationOwner.IsSet(), "the pure builder must not publish compiled authority")
	}
}
