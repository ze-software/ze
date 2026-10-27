// Design: docs/contributing/terminal-demos.md -- topics start from baseline plumbing
// Related: validate_showcase.go -- the guard used before every live topic

package siteterminaldemo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // Load the same schema registrations as the recorder binary.
)

// PREVENTS: rejecting the real lab's transit RIB while admitting a topic that
// was already enabled before its recording's live commit.
func TestShowcaseBaseTreeAcceptsActualBaseline(t *testing.T) {
	tree := readShowcaseBaseTree(t)
	if err := requireShowcaseBaseTree(tree); err != nil {
		t.Fatalf("actual showcase base rejected: %v", err)
	}
	// Subscription order is not part of the baseline contract.
	rib := tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"]
	rib.SetSlice("receive", []string{"refresh", "update", "state"})
	if err := requireShowcaseBaseTree(tree); err != nil {
		t.Fatalf("reordered baseline subscriptions rejected: %v", err)
	}
}

// PREVENTS: broadening the RIB exception to other peers, subscriptions or
// plugin implementations, and losing the topic/BFD/VRRP/XFRM absence guards.
func TestShowcaseBaseTreeRejectsPreconfiguredTopics(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*config.Tree)
		want   string
	}{
		{"VPN", func(tree *config.Tree) { tree.GetOrCreateContainer("vpn") }, "topic vpn"},
		{"BFD", func(tree *config.Tree) { tree.GetOrCreateContainer("bfd") }, "topic bfd"},
		{"OSPF", func(tree *config.Tree) { tree.GetOrCreateContainer("ospf") }, "topic ospf"},
		{"traffic", func(tree *config.Tree) { tree.GetOrCreateContainer("traffic") }, "topic traffic"},
		{"RPKI", func(tree *config.Tree) { tree.GetContainer("bgp").GetOrCreateContainer("rpki") }, "topic bgp/rpki"},
		{"IRR", func(tree *config.Tree) { tree.GetContainer("bgp").GetOrCreateContainer("irr") }, "topic bgp/irr"},
		{"missing BGP", func(tree *config.Tree) { tree.RemoveContainer("bgp") }, "no BGP"},
		{"peer BFD", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["frr"].GetContainer("connection").Set("bfd", "showcase")
		}, "peer frr already configures BFD"},
		{"topic binding", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").AddListEntry("process", "bgp-rpki", config.NewTree())
		}, "peer transit pre-binds bgp-rpki"},
		{"customer RIB", func(tree *config.Tree) {
			peers := tree.GetContainer("bgp").GetList("peer")
			peers["customer"].GetContainer("attach").AddListEntry("process", "bgp-rib", peers["transit"].GetContainer("attach").GetList("process")["bgp-rib"])
		}, "peer customer pre-binds bgp-rib"},
		{"FRR RIB", func(tree *config.Tree) {
			peers := tree.GetContainer("bgp").GetList("peer")
			peers["frr"].GetContainer("attach").AddListEntry("process", "bgp-rib", peers["transit"].GetContainer("attach").GetList("process")["bgp-rib"])
		}, "peer frr pre-binds bgp-rib"},
		{"RIB missing refresh", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("receive", []string{"update", "state"})
		}, "non-baseline bgp-rib subscriptions"},
		{"RIB extra receive", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("receive", []string{"update", "state", "refresh", "update-received"})
		}, "non-baseline bgp-rib subscriptions"},
		{"RIB wrong receive", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("receive", []string{"update", "state", "update-received"})
		}, "non-baseline bgp-rib subscriptions"},
		{"RIB no send", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("send", nil)
		}, "non-baseline bgp-rib subscriptions"},
		{"RIB extra send", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("send", []string{"update", "refresh"})
		}, "non-baseline bgp-rib subscriptions"},
		{"RIB wrong send", func(tree *config.Tree) {
			tree.GetContainer("bgp").GetList("peer")["transit"].GetContainer("attach").GetList("process")["bgp-rib"].SetSlice("send", []string{"refresh"})
		}, "non-baseline bgp-rib subscriptions"},
		{"topic plugin", func(tree *config.Tree) {
			plugin := config.NewTree()
			plugin.Set("use", "bgp-rpki")
			tree.GetContainer("plugin").AddListEntry("internal", "bgp-rpki", plugin)
		}, "preconfigures plugin bgp-rpki"},
		{"topic behind RIB name", func(tree *config.Tree) {
			tree.GetContainer("plugin").GetList("internal")["bgp-rib"].Set("use", "bgp-rpki")
		}, "bgp-rib uses"},
		{"XFRM", func(tree *config.Tree) {
			tree.GetContainer("interface").AddListEntry("xfrm", "vpn-xfrm", config.NewTree())
		}, "XFRM interface"},
		{"IPv4 VRRP", func(tree *config.Tree) {
			tree.GetContainer("interface").GetList("ethernet")["zeptrk0"].GetList("unit")["0"].GetContainer("ipv4").GetOrCreateContainer("vrrp")
		}, "interface zeptrk0 already configures VRRP"},
		{"IPv6 VRRP", func(tree *config.Tree) {
			tree.GetContainer("interface").GetList("ethernet")["zeptrk0"].GetList("unit")["0"].GetOrCreateContainer("ipv6").GetOrCreateContainer("vrrp")
		}, "interface zeptrk0 already configures VRRP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := readShowcaseBaseTree(t)
			tc.mutate(tree)
			err := requireShowcaseBaseTree(tree)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("base guard error = %v, want %q", err, tc.want)
			}
		})
	}
}

func readShowcaseBaseTree(t *testing.T) *config.Tree {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoDemoRoot, showcaseLabID, "ze.conf"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := config.ParseTreeForValidation(string(data))
	if err != nil {
		t.Fatalf("parse actual showcase base: %v", err)
	}
	return tree
}
