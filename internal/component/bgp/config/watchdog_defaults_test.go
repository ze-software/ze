// Design: docs/architecture/rib-transition.md -- optional watchdog ownership of configured routes
// Related: peers.go -- defaults on resolved maps and routes from the original tree
// Related: bgp_routes.go -- hasWatchdogContainer excludes watchdog-owned routes

package bgpconfig

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestWatchdogDefaultPreservesOrdinaryStaticRoute runs the actual peer builder
// with an ordinary update and a watchdog-owned update. Schema defaults must not
// cause the ordinary route to disappear or the watchdog route to leak into sync.
func TestWatchdogDefaultPreservesOrdinaryStaticRoute(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()
	peer := buildMinimalPeer("192.0.2.1", "65001", "auto")
	ordinary := watchdogDefaultUpdate("198.51.100.0/24")
	peer.AddListEntry("update", "ordinary", ordinary)
	held := watchdogDefaultUpdate("203.0.113.0/24")
	watchdog := config.NewTree()
	watchdog.Set("name", "dnsr")
	held.SetContainer("watchdog", watchdog)
	peer.AddListEntry("update", "controlled", held)
	bgp.AddListEntry("peer", "upstream", peer)
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	if err != nil {
		t.Fatalf("build peers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("built %d peers, want one", len(peers))
	}
	if len(peers[0].StaticRoutes) != 1 {
		t.Fatalf("built %d static routes, want the ordinary update alone", len(peers[0].StaticRoutes))
	}
	if got := peers[0].StaticRoutes[0].Prefix.String(); got != "198.51.100.0/24" {
		t.Fatalf("static route = %s, want 198.51.100.0/24", got)
	}
	if ordinary.GetContainer("watchdog") != nil {
		t.Fatal("peer defaults changed the ordinary update's original config tree")
	}
}

// TestWatchdogSchemaDefaultIsFalse checks the loaded schema instead of copying
// the declared default into a parser fixture that would pass after schema drift.
func TestWatchdogSchemaDefaultIsFalse(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	if got := config.SchemaDefault(schema, "bgp/peer/update/watchdog/withdraw"); got != "false" {
		t.Fatalf("watchdog withdraw default = %q, want false", got)
	}
}

func watchdogDefaultUpdate(prefix string) *config.Tree {
	update := config.NewTree()
	attrs := config.NewTree()
	attrs.Set("origin", "igp")
	attrs.Set("next-hop", "1.2.3.4")
	update.SetContainer("attribute", attrs)
	nlri := config.NewTree()
	nlri.Set("content", "add "+prefix)
	update.AddListEntry("nlri", "ipv4/unicast", nlri)
	return update
}
