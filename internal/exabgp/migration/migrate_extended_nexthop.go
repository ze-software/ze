// Design: docs/exabgp/exabgp-migration.md -- Extended next hop for an IPv4 route
// Related: migrate.go -- migrateCapability writes the declared nexthop entries
// Related: migrate_routes.go -- convertRouteToUpdate writes the update entries read here
// RFC: rfc/short/rfc8950.md

package migration

import (
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// extendedNextHopFamilies maps each ze family whose IPv4 routes may carry an
// IPv6 next hop to the ExaBGP spelling a warning quotes. They are the pinned
// ExaBGP's EXTENDED_NEXTHOP_FAMILIES (configuration/grammar/tree/neighbor.py):
// ipv4 unicast, multicast, nlri-mpls and mpls-vpn.
var extendedNextHopFamilies = map[string]string{
	familyIPv4Unicast: "ipv4 unicast",
	"ipv4/multicast":  "ipv4 multicast",
	"ipv4/mpls-label": "ipv4 nlri-mpls",
	"ipv4/mpls-vpn":   "ipv4 mpls-vpn",
}

// capabilityDisable is the ExaBGP word that turns a capability off.
const capabilityDisable = "disable"

// addExtendedNextHopEntries asks for the Extended Next Hop Encoding each
// migrated IPv4 route with an IPv6 next hop needs, as the pinned ExaBGP's
// _extended_next_hop does at load. ExaBGP 4.2 and 5.0 sent such a route without
// asking, so a config with no `nexthop` entry for the family gains
// `<family> ipv6` in session capability nexthop, with one warning per family
// naming the first route that needed it. `capability { nexthop disable; }`
// refuses the config with the pin's reason.
//
// RFC 8950 Section 4: "A BGP speaker MUST only advertise the IPv4 or VPN-IPv4
// NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first
// ascertained via the BGP Capability Advertisement that the BGP peer supports
// the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair."
// ze advertises the capability only for a configured entry, so without one the
// route a 4.2 or 5.0 config sent would never leave.
func addExtendedNextHopEntries(src, peer *config.Tree, peerName string, result *MigrateResult) error {
	for _, update := range peer.GetListOrdered("update") {
		attribute := update.Value.GetContainer("attribute")
		if attribute == nil {
			continue
		}
		value, ok := attribute.Get(routeAttributeNextHop)
		if !ok {
			continue
		}
		// `self` and every other word that is no address names no IPv6 next hop.
		nextHop, err := netip.ParseAddr(value)
		if err != nil {
			continue
		}
		if !nextHop.Is6() {
			continue
		}
		for _, nlri := range update.Value.GetListOrdered("nlri") {
			family := config.StripListKeySuffix(nlri.Key)
			exabgpFamily, covered := extendedNextHopFamilies[family]
			if !covered {
				continue
			}
			if err := requireExtendedNextHop(src, peer, family, exabgpFamily, nextHop, nlri.Value, peerName, result); err != nil {
				return err
			}
		}
	}
	return nil
}

// requireExtendedNextHop adds the `<family> ipv6` entry for one route, or
// refuses when the source config turned the capability off.
func requireExtendedNextHop(src, peer *config.Tree, family, exabgpFamily string, nextHop netip.Addr, nlri *config.Tree, peerName string, result *MigrateResult) error {
	entry := family + " ipv6"
	capability := peerCapability(peer)
	nexthops := capability.GetContainer("nexthop")
	if nexthops != nil {
		if _, declared := nexthops.Get(entry); declared {
			return nil
		}
	}
	if extendedNextHopDisabled(src) {
		return fmt.Errorf("neighbor %s: next-hop %s is IPv6, which an %s route carries only with the Extended Next Hop Encoding, and \"capability { nexthop disable; }\" turns it off (RFC 8950)",
			peerName, nextHop, exabgpFamily)
	}
	if nexthops == nil {
		nexthops = config.NewTree()
		capability.SetContainer("nexthop", nexthops)
	}
	nexthops.Set(entry, "")

	route, _ := nlri.Get("content")
	var tb textbuf.Buffer
	tb.Str("neighbor ").Str(peerName).Str(": route ").Str(route).Str(" has next-hop ").Str(nextHop.String()).
		Str(", which is IPv6, so \"nexthop { ").Str(entry).Str("; }\" is added to session capability (RFC 8950 Extended Next Hop Encoding); the ").
		Str(family).Str(" routes with an IPv6 next-hop are only sent if the peer agrees")
	result.Warnings = append(result.Warnings, tb.String())
	return nil
}

// peerCapability answers the peer's session capability container, creating the
// session and capability containers when the migration wrote neither.
func peerCapability(peer *config.Tree) *config.Tree {
	session := peer.GetContainer("session")
	if session == nil {
		session = config.NewTree()
		peer.SetContainer("session", session)
	}
	capability := session.GetContainer("capability")
	if capability == nil {
		capability = config.NewTree()
		session.SetContainer("capability", capability)
	}
	return capability
}

// extendedNextHopDisabled reports whether the ExaBGP config wrote
// `capability { nexthop disable; }` (or false), the one form the pin lets refuse
// the entry.
func extendedNextHopDisabled(src *config.Tree) bool {
	capability := src.GetContainer("capability")
	if capability == nil {
		return false
	}
	value, ok := capability.GetFlex("nexthop")
	if !ok {
		return false
	}
	return value == capabilityDisable || value == "false"
}
