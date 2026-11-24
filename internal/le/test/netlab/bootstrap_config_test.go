//go:build ze_bgp

// Design: docs/guide/netlab.md -- activate interface events and peer-up route replay.
package testnetlab

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	// Register the actual schemas for every root the complete reference renders
	// carry and BGP's event vocabulary. No provider starts during registration.
	_ "github.com/ze-software/ze/internal/component/bfd/yang"
	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/config/redistribute/yang"
	_ "github.com/ze-software/ze/internal/component/iface/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
	_ "github.com/ze-software/ze/internal/component/ssh/yang"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	_ "github.com/ze-software/ze/internal/plugins/connected/yang"
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
	"github.com/ze-software/ze/internal/plugins/redistribute"
	_ "github.com/ze-software/ze/internal/plugins/static/yang"
)

func referenceNodeConfig(t *testing.T, node string) *config.Tree {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "contrib", "netlab", "golden", node+".conf")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.ParseTreeForValidation(string(body))
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// VALIDATES: every rendered peer authorizes exactly the orchestrator's peer-state
// input and UPDATE output, so establishment can trigger connected-route replay.
func TestReferencePeersAuthorizeRedistributionStateReplay(t *testing.T) {
	for node, count := range map[string]int{"r1": 2, "r2": 1, "r3": 1} {
		t.Run(node, func(t *testing.T) {
			resolved, err := bgpconfig.ResolveBGPTree(referenceNodeConfig(t, node))
			if err != nil {
				t.Fatal(err)
			}
			peers, err := reactor.PeersFromTree(resolved)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != count {
				t.Fatalf("parsed %d peers, want %d", len(peers), count)
			}
			srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{
				Plugins: []plugin.PluginConfig{{Name: redistribute.Name, Internal: true}},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			srv.UpdateDeliveryGraph(bgpevents.Namespace, reactor.DisseminationPeersFromSettings(peers))
			graph := srv.DeliveryGraph()
			byPeer := make(map[string][]pluginserver.ProcessEdges, len(peers))
			for _, pd := range graph.Inspect() {
				byPeer[pd.Peer] = pd.Processes
			}
			for _, peer := range peers {
				addr := peer.PeerKey().String()
				if !graph.ProcessReceivesState(addr, redistribute.Name) {
					t.Errorf("peer %s does not deliver bgp:state to %s", peer.Name, redistribute.Name)
				}
				if !graph.MaySend(addr, redistribute.Name, bgpevents.SendUpdate) {
					t.Errorf("peer %s does not permit bgp:update from %s", peer.Name, redistribute.Name)
				}
				if graph.MaySend(addr, redistribute.Name, bgpevents.SendRaw) {
					t.Errorf("peer %s must not permit bgp:raw from %s", peer.Name, redistribute.Name)
				}
				var edge *pluginserver.ProcessEdges
				for i := range byPeer[addr] {
					if byPeer[addr][i].Process == redistribute.Name {
						edge = &byPeer[addr][i]
						break
					}
				}
				if edge == nil {
					t.Fatalf("peer %s missing %s edge in DeliveryGraph.Inspect()", peer.Name, redistribute.Name)
				}
				if !slices.Equal(edge.Receive, []string{bgpevents.EventState}) {
					t.Errorf("peer %s %s receive = %v, want [%s]", peer.Name, redistribute.Name, edge.Receive, bgpevents.EventState)
				}
				if !slices.Equal(edge.Send, []string{bgpevents.SendUpdate}) {
					t.Errorf("peer %s %s send = %v, want [%s]", peer.Name, redistribute.Name, edge.Send, bgpevents.SendUpdate)
				}
			}
		})
	}
}

// VALIDATES: each reference node activates the interface provider without
// declaring managed addresses; netlab remains the address owner.
func TestReferenceConfigsActivateUnmanagedInterfaceProvider(t *testing.T) {
	for _, node := range []string{"r1", "r2", "r3"} {
		t.Run(node, func(t *testing.T) {
			interfaces := referenceNodeConfig(t, node).GetContainer("interface")
			if interfaces == nil {
				t.Fatal("interface provider is inactive: interface root is absent")
			}
			if managed := interfaces.ToPluginMap(); len(managed) != 0 {
				t.Errorf("interface root manages netlab state: %v", managed)
			}
		})
	}
}
