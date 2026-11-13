package reactor

import (
	"errors"
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy reaches each actual
// writer with an IGP link-local candidate. Auto chooses self for either peer
// class; configured overrides retain their existing explicit semantics.
// An unlicensed retained IPv6 address is refused beside a useful IPv4 survivor.
//
// RFC requirement: RFC8950-4-1 negative -- without the IPv4/unicast-to-IPv6
// pair, configured unchanged and explicit link-local overrides of a Default
// candidate are refused on direct and both queue drains, while the independent
// explicit IPv4 prefix is still advertised.
//
// MUTATION: forge both the exact Extended Next Hop pair and capability 77 in
// explicit resolution, before the resolver and final writer read those facts.
// The queued cases must then expose an extra forbidden link-local announcement
// beside the IPv4 survivor. A changed direct error class alone is not proof.
func TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy(t *testing.T) {
	for _, rail := range []string{"direct", "initial-drain", "closing-drain"} {
		t.Run(rail, func(t *testing.T) {
			for _, peerClass := range []string{"external", "internal"} {
				t.Run(peerClass, func(t *testing.T) {
					for _, tc := range []struct {
						name     string
						mode     uint8
						explicit string
						want     string
					}{
						{name: "auto", mode: NextHopAuto, want: "192.0.2.1"},
						{name: "self", mode: NextHopSelf, want: "192.0.2.1"},
						{name: "explicit", mode: NextHopExplicit, explicit: "192.0.2.9", want: "192.0.2.9"},
						{name: "unchanged", mode: NextHopUnchanged},
						{name: "explicit-link-local", mode: NextHopExplicit, explicit: "fe80::9"},
					} {
						t.Run(tc.name, func(t *testing.T) {
							peer, conn := newDefaultNextHopControlPeer(t, rail, "192.0.2.1")
							if peerClass == "internal" {
								peer.settings.PeerAS = peer.settings.LocalAS
							}
							peer.settings.NextHopMode = tc.mode
							if tc.explicit != "" {
								peer.settings.NextHopAddress = netip.MustParseAddr(tc.explicit)
							}
							adapter := defaultNextHopControlAdapter(peer)
							batch := defaultNextHopControlBatch("10.30.0.0/24", bgptypes.NewNextHopDefault(netip.MustParseAddr("fe80::1")))
							err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
							refused := tc.want == ""
							if rail == "direct" && refused {
								if !errors.Is(err, ErrNextHopIncompatible) {
									t.Fatalf("unlicensed retained link-local next hop: got %v, want ErrNextHopIncompatible", err)
								}
							} else if err != nil {
								t.Fatal(err)
							}
							canary := defaultNextHopControlBatch("10.31.0.0/24", bgptypes.NewNextHopExplicit(netip.MustParseAddr("198.51.100.1")))
							if err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), canary, plugin.OperatorSender()); err != nil {
								t.Fatal(err)
							}
							drainDefaultNextHopControl(t, peer, rail)
							bodies := defaultNextHopControlBodies(t, conn.written())
							wantCount := 1
							if !refused {
								wantCount = 2
							}
							if len(bodies) != wantCount {
								t.Fatalf("announcements = %d, want %d with the IPv4 survivor", len(bodies), wantCount)
							}
							if !refused {
								assertDefaultNextHopWire(t, bodies[0], netip.MustParseAddr(tc.want), []byte{24, 10, 30, 0})
							}
							assertDefaultNextHopWire(t, bodies[len(bodies)-1], netip.MustParseAddr("198.51.100.1"), []byte{24, 10, 31, 0})
						})
					}
				})
			}
		})
	}
}
