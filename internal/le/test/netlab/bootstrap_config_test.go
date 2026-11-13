//go:build ze_bgp

// Design: docs/guide/netlab.md -- activate interface events and peer-up route replay.
package testnetlab

import (
	"os"
	"path/filepath"
	"reflect"
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
	_ "github.com/ze-software/ze/internal/component/plugin/yang"
	_ "github.com/ze-software/ze/internal/component/ssh/yang"
	"github.com/ze-software/ze/internal/core/events"
	_ "github.com/ze-software/ze/internal/plugins/connected/yang"
	_ "github.com/ze-software/ze/internal/plugins/isis/yang"
	_ "github.com/ze-software/ze/internal/plugins/ospf/yang"
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
			for _, peer := range peers {
				var bindings int
				for _, binding := range peer.ProcessBindings {
					if binding.PluginName != "redistribute-orchestrator" {
						continue
					}
					bindings++
					wantReceive := map[string]events.Direction{"state": events.DirBoth}
					if binding.ReceiveAll {
						t.Errorf("peer %s grants every receive type", peer.Name)
					}
					if !reflect.DeepEqual(binding.Receive, wantReceive) {
						t.Errorf("peer %s receives %v, want state delivery only", peer.Name, binding.Receive)
					}
					if binding.SendAll {
						t.Errorf("peer %s grants every send type", peer.Name)
					}
					if !reflect.DeepEqual(binding.Send, map[string]bool{"update": true}) {
						t.Errorf("peer %s sends %v, want UPDATE permission only", peer.Name, binding.Send)
					}
				}
				if bindings != 1 {
					t.Errorf("peer %s has %d orchestrator bindings, want exactly one", peer.Name, bindings)
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
