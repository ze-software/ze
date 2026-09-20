// VALIDATES: the cascade door onto a promotion. A prefix programmed over its
// winner's own gateway, whose covering route then goes, is re-programmed over
// the equal-cost member that still resolves, carrying the MEMBER's forwarding
// path.
// PREVENTS: the promotion being proved through the live door alone. The entry
// is built in one place for all four emitters, so a defect in the split shows
// wherever a promotion happens, and the cascade is the emitter that performs
// it after the FIB already holds the winner's own path.

package sysrib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// TestCascadePromotesAMemberWithItsOwnLabels drives the promotion through the
// resolver rather than through a change of winner: both gateways resolve, the
// winner's cover goes, and the member takes the prefix.
func TestCascadePromotesAMemberWithItsOwnLabels(t *testing.T) {
	state := newPromotionState(t, netip.MustParsePrefix("10.63.0.0/24"))
	state.member.labels = []uint32{200}
	state.winner.labels = []uint32{100}

	connectedID := redistevents.RegisterProtocol("connected")
	loc := getLocRIB()
	winnerCover := netip.MustParsePrefix("192.0.2.0/24")
	loc.Insert(family.IPv4Unicast, winnerCover, locrib.Path{Source: connectedID})

	addProducerRoute(state.sysrib, state.prefix, state.winner)
	addProducerRoute(state.sysrib, state.prefix, state.member)

	installed := publishedChanges(t, state.bus)
	if len(installed) == 0 || installed[len(installed)-1].NextHop != state.winner.nextHop {
		t.Fatalf("setup: published %+v, want %s programmed over the winner's own gateway %s",
			installed, state.prefix, state.winner.nextHop)
	}

	loc.Remove(family.IPv4Unicast, winnerCover, connectedID, 0)
	state.sysrib.processCascade([]netip.Addr{state.winner.nextHop})

	promoted := publishedChanges(t, state.bus)[len(installed):]
	if len(promoted) != 1 || promoted[0].Action != routeaction.Update {
		t.Fatalf("the cascade published %+v, want one update for %s: the winner's gateway lost its "+
			"cover and the member still resolves", promoted, state.prefix)
	}
	if promoted[0].NextHop != state.member.nextHop || promoted[0].Interface != state.member.iface {
		t.Errorf("the cascade programmed %s over %s out %q, want the member's %s out %q",
			state.prefix, promoted[0].NextHop, promoted[0].Interface,
			state.member.nextHop, state.member.iface)
	}
	if !labelsEqual(promoted[0].Labels, state.member.labels) {
		t.Errorf("the cascade imposed %v on the member's path, want its own stack %v",
			promoted[0].Labels, state.member.labels)
	}
}
