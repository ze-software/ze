// Design: docs/guide/graceful-restart.md -- retention and stale marking are family-scoped.
package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestGRReceivedFamilyRetentionAndLLGRScope(t *testing.T) {
	for _, structured := range []bool{false, true} {
		for _, scope := range []struct {
			name string
			families []string
			keepIPv6 bool
		}{
			{"legacy-peer-wide", nil, true},
			{"only-ipv4", []string{"ipv4/unicast"}, false},
			{"both-families", []string{"ipv4/unicast", "ipv6/unicast"}, true},
		} {
			t.Run(scope.name+map[bool]string{false:"/json", true:"/structured"}[structured], func(t *testing.T) {
				r := setupGRTestRIB(t)
				peer := netip.MustParseAddr("192.0.2.1")
				other := netip.MustParseAddr("192.0.2.2")
				destination := netip.MustParseAddr("192.0.2.3")
				r.ribOut[destination] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
					family.IPv4Unicast: {"10.0.0.0/24": {Prefix: "10.0.0.0/24", NextHop: "192.0.2.1", SourcePeer: peer.String()}},
					family.IPv6Unicast: {"2001:db8::/32": {Prefix: "2001:db8::/32", NextHop: "::1", SourcePeer: peer.String()}},
				})
				t.Cleanup(func() {
					for _, routes := range r.bgpPeers { routes.Release() }
					for _, families := range r.ribOut {
						for _, routes := range families {
							for _, entry := range routes { entry.release() }
						}
					}
				})
				r.peerUp[peer] = true
				args := append([]string{peer.String()}, scope.families...)
				if _, _, err := r.handleCommand("request bgp rib retain-routes", "*", args); err != nil { t.Fatal(err) }
				if _, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{peer.String(), "0"}); err != nil { t.Fatal(err) }
				if structured {
					r.handleStructuredState(&rpc.StructuredEvent{PeerAddress: peer.String(), State: rpc.SessionStateDown})
				} else {
					r.handleState(&Event{Peer: mustMarshal(t, map[string]any{"remote": map[string]any{"address": peer.String()}}), State: "down"})
				}
				if _, _, err := r.handleCommand("request bgp rib mark-stale", "*", []string{peer.String(), "0", "2", "ipv4/unicast"}); err != nil { t.Fatal(err) }
				received := r.bgpPeers[peer]
				if received == nil { t.Fatal("retained source inventory lost at DOWN") }
				ipv4, found := received.Lookup(family.IPv4Unicast, []byte{24, 10, 0, 0})
				if !found || ipv4.StaleLevel != 2 { t.Fatal("selected family did not enter LLGR") }
				ipv6, found := received.Lookup(family.IPv6Unicast, []byte{32, 0x20, 0x01, 0x0d, 0xb8})
				if found != scope.keepIPv6 { t.Fatal("received family retention differs from the supplied scope") }
				if found && ipv6.StaleLevel != 1 { t.Fatal("IPv4 LLGR changed another family's stale level") }
				if r.bgpPeers[other].StaleCount() != 0 { t.Fatal("another source was marked stale") }
				sent4 := r.ribOut[destination][family.IPv4Unicast][ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}]
				if sent4.StaleLevel != 2 { t.Fatal("selected sent family did not enter LLGR") }
				sent6, found := r.ribOut[destination][family.IPv6Unicast][ribOutKey{Prefix: netip.MustParsePrefix("2001:db8::/32")}]
				if found != scope.keepIPv6 { t.Fatal("sent ownership differs from retained family scope") }
				if found && sent6.StaleLevel != 1 { t.Fatal("IPv4 LLGR changed another sent family's stale level") }
			})
		}
	}
}
