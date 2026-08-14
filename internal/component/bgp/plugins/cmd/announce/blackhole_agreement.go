// Design: docs/architecture/bgp/on-demand-origination.md -- the verbs this gate sits in front of
// RFC: rfc/short/rfc7999.md -- Section 3.1, the agreement owed before BLACKHOLE is advertised

package announce

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/blackholecfg"
	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/selector"
)

// errNoAgreedPeer refuses an origination no selected session consented to.
var errNoAgreedPeer = errors.New("no selected peer agreed to the BLACKHOLE community")

// agreedSelector narrows sel to the peers that recorded an agreement to carry
// the BLACKHOLE community, and refuses when none did.
//
// RFC 7999 Section 3.1: "In a bilateral peering relationship, use of the
// BLACKHOLE community MUST be agreed upon by the two networks before advertising
// it." The obligation is about the COMMUNITY, not the prefix, and Ze's half of
// that agreement is the community named on the peer -- the same leaf-list the
// honoring path reads, because one session is what both directions are about.
//
// A peer that agreed is announced to; a peer that did not is left out of the
// fan-out entirely. It is NOT sent the prefix untagged: the operator asked for
// traffic toward that address to be discarded, and an ordinary announcement of a
// host route under attack ATTRACTS that traffic instead. Withholding leaves the
// peer's view unchanged, which is the only outcome the RFC's own compatibility
// case describes for a network that does not use the community.
//
// When no selected peer agreed, the command errors rather than succeeding with
// an empty fan-out. Under duress a blackhole that silently reached nobody is the
// failure this refusal exists to make visible.
//
// The running config is read on each invocation rather than cached. This is a
// CLI verb, so the cost is one parse per operator command, and nothing has to be
// invalidated when the operator commits a new agreement.
func agreedSelector(ctx *pluginserver.CommandContext, sel *selector.Selector, verb string) (*selector.Selector, error) {
	reactor := ctx.Reactor()
	if reactor == nil {
		return nil, errReactorNotAvailable
	}
	tree := reactor.GetConfigTree()
	bgpCfg, _ := tree["bgp"].(map[string]any)
	rules, err := blackholecfg.Parse(bgpCfg)
	if err != nil {
		// Refused rather than read as "nobody agreed". A value nobody could parse
		// is an agreement the operator sees in the running config, and answering
		// it with silence is what the parse refuses everywhere else.
		return nil, fmt.Errorf("%s: %w", verb, err)
	}

	peers := pluginserver.PeersMatching(ctx, sel)
	agreed := make([]netip.Addr, 0, len(peers))
	var refused []string
	for i := range peers {
		// The peer's own address first, then the group it belongs to. A session
		// the reactor created from a dynamic group has no address in the config
		// document, so its agreement is the one its group stated and the group
		// name is what reaches it. A peer that stated its own agreement keeps it,
		// because the address is consulted first.
		//
		// A miss is a session that agreed to nothing, which is the closed answer
		// this gate exists to give.
		rule, ok := configjson.LookupPeerConfig(rules, peers[i].AddrStr(), peers[i].Name, peers[i].GroupName)
		if ok && rule.Agreed(attribute.CommunityBlackhole) {
			agreed = append(agreed, peers[i].Address)
			continue
		}
		refused = append(refused, peerLabel(&peers[i]))
	}

	if len(agreed) == 0 {
		return nil, fmt.Errorf(
			"%s: %w (65535:666): RFC 7999 Section 3.1 requires the two networks to agree before it is advertised; "+
				"record the agreement with `blackhole { communities blackhole; }` on the peer or its group, "+
				"or announce the prefix without the community using `announce unicast` -- not agreed: %s",
			verb, errNoAgreedPeer, strings.Join(refusedOrAll(refused), ", "))
	}
	return selector.Addrs(agreed), nil
}

// peerLabel names a peer the way an operator configured it, falling back to the
// address when the peer has no name.
func peerLabel(p *plugin.PeerInfo) string {
	if p.Name != "" {
		return p.Name
	}
	return p.AddressStr
}

// refusedOrAll keeps the message honest when the selector matched no peer at
// all: "not agreed: " followed by nothing reads as a formatting bug rather than
// as the empty peer table it is.
func refusedOrAll(refused []string) []string {
	if len(refused) == 0 {
		return []string{"no peer matched the selector"}
	}
	return refused
}
