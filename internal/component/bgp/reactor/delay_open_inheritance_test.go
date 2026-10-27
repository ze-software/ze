// Design: docs/architecture/behavior/peer-lifecycle.md — DelayOpen inheritance.
package reactor_test

import (
	"net/netip"
	"testing"
	"time"

	bgpconfig "github.com/ze-software/ze/internal/component/bgp/config"
	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// TestDelayOpenInheritanceToPeerSettings follows the actual tree resolver and
// peer-settings parser. Explicit zero must override an enabled inherited value.
func TestDelayOpenInheritanceToPeerSettings(t *testing.T) {
	tree, bgp := config.NewTree(), config.NewTree()
	timer := func(seconds string) *config.Tree {
		value := config.NewTree()
		value.Set("delay-open", seconds)
		return value
	}
	nextAddress := byte(1)
	newPeer := func() *config.Tree {
		value, connection := config.NewTree(), config.NewTree()
		local, remote := config.NewTree(), config.NewTree()
		local.Set("ip", "auto")
		remote.Set("ip", netip.AddrFrom4([4]byte{192, 0, 2, nextAddress}).String())
		nextAddress++
		connection.SetContainer("local", local)
		connection.SetContainer("remote", remote)
		value.SetContainer("connection", connection)
		session, asn := config.NewTree(), config.NewTree()
		asn.Set("local", "65000")
		asn.Set("remote", "65001")
		session.SetContainer("asn", asn)
		session.Set("router-id", "1.1.1.1")
		value.SetContainer("session", session)
		return value
	}
	bgp.SetContainer("timer", timer("5"))
	bgp.AddListEntry("peer", "global", newPeer())
	inherited := config.NewTree()
	inherited.AddListEntry("peer", "global-group", newPeer())
	bgp.AddListEntry("group", "inherited", inherited)
	group := config.NewTree()
	group.SetContainer("timer", timer("7"))
	group.AddListEntry("peer", "group", newPeer())
	off := newPeer()
	off.SetContainer("timer", timer("0"))
	group.AddListEntry("peer", "off", off)
	own := newPeer()
	own.SetContainer("timer", timer("240"))
	group.AddListEntry("peer", "own", own)
	bgp.AddListEntry("group", "override", group)
	tree.SetContainer("bgp", bgp)
	resolved, err := bgpconfig.ResolveBGPTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	peers, err := reactor.PeersFromTree(resolved)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]time.Duration{"global": 5, "global-group": 5, "group": 7, "off": 0, "own": 240}
	for _, settings := range peers {
		seconds, ok := want[settings.Name]
		if !ok {
			t.Fatalf("unexpected or duplicate peer %q", settings.Name)
		}
		if settings.DelayOpen != seconds*time.Second {
			t.Fatalf("%s inherited delay=%s, want %ds", settings.Name, settings.DelayOpen, seconds)
		}
		delete(want, settings.Name)
	}
	for name := range want {
		t.Errorf("missing peer %q", name)
	}
}
