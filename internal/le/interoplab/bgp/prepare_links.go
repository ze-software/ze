// Design: docs/architecture/testing/interop.md -- separate links between the lab's daemons.
// Related: prepare.go -- prepareScenario and wrapStart, which build these links before each daemon starts.
// Related: register_isis_import_policy.go -- the scenario that declares two links.
package bgp

import (
	"errors"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// labLink is one layer-2 segment of a scenario that only its members join: a
// VLAN over the lab bridge. Every container is on the bridge's untagged eth0,
// so a protocol that discovers its neighbors by multicast (IS-IS, OSPF) meets
// every daemon there. A daemon configured on a link meets only the other
// members, which is how a scenario puts Ze between two peers that never see
// each other (FRR-A -- Ze -- FRR-B).
//
// Each member reaches the link as eth<N>, where N counts the links that member
// joins in declaration order (its first link is eth1), and holds its host
// number on ipv4. The scenario's files name those devices and addresses.
type labLink struct {
	vlan uint16
	// ipv4 is the link's subnet. A member's address is the subnet's network
	// address with the member's host number in the last octet, so the subnet
	// is at most a /24.
	ipv4 netip.Prefix
	// members are indexes into linkLocalOwners: the containers whose start
	// wrapStart can wrap.
	members []int
}

// scenarioLinks holds the links of each scenario that has some, by scenario
// name. A scenario's register file adds its entry, so prepare.go names no
// scenario. A scenario with no entry runs every daemon on eth0 alone, as every
// scenario did before links existed.
var scenarioLinks = map[string][]labLink{}

// labLinkVLANMax is the highest usable 802.1Q VLAN identifier; 0 and 4095 are
// reserved.
const labLinkVLANMax = 4094

// labLinkPrefixBitsMax keeps a member's host number inside the link's subnet.
const labLinkPrefixBitsMax = 24

// renderLinks renders the shell that builds each owner's links, by index into
// linkLocalOwners, empty for an owner on no link. It refuses a link that
// cannot be built as declared: a VLAN outside 1 to 4094 or used twice, a
// subnet that is not an IPv4 network of at most /24, fewer than two members,
// a member twice, or a member whose container the scenario does not start
// (its config file is missing), because that link would carry nothing and the
// checker would read a silent lab as the daemon's fault.
func renderLinks(scenario string, links []labLink, peers []interoplab.PeerConfig) (ownerPrestarts, error) {
	var rendered ownerPrestarts
	var joined [len(linkLocalOwners)]int
	var scripts [len(linkLocalOwners)]textbuf.Buffer
	seen := make(map[uint16]bool, len(links))
	for index := range links {
		link := &links[index]
		if err := validateLink(scenario, link, seen, peers); err != nil {
			return rendered, err
		}
		for _, owner := range link.members {
			joined[owner]++
			writeLinkScript(&scripts[owner], link, joined[owner], linkLocalOwners[owner].host)
		}
	}
	for owner := range linkLocalOwners {
		rendered[owner] = scripts[owner].String()
	}
	return rendered, nil
}

// validateLink is renderLinks' refusal of one declared link.
func validateLink(scenario string, link *labLink, seen map[uint16]bool, peers []interoplab.PeerConfig) error {
	var reason textbuf.Buffer
	reason.Str(scenario).Str("'s link on VLAN ").Str(strconv.Itoa(int(link.vlan)))
	if link.vlan == 0 {
		return errors.New(reason.Str(" is not a usable VLAN; declare one from 1 to 4094 in scenarioLinks").String())
	}
	if link.vlan > labLinkVLANMax {
		return errors.New(reason.Str(" is not a usable VLAN; declare one from 1 to 4094 in scenarioLinks").String())
	}
	if seen[link.vlan] {
		return errors.New(reason.Str(" is declared twice; give each link of the scenario its own VLAN").String())
	}
	seen[link.vlan] = true
	if !link.ipv4.IsValid() {
		return errors.New(reason.Str(" has no subnet; declare its IPv4 network, as in 10.101.0.0/24").String())
	}
	if !link.ipv4.Addr().Is4() {
		return errors.New(reason.Str(" needs an IPv4 subnet, as in 10.101.0.0/24").String())
	}
	if link.ipv4.Masked() != link.ipv4 {
		return errors.New(reason.Str(" names a host, not a network; write ").Str(link.ipv4.Masked().String()).String())
	}
	if link.ipv4.Bits() > labLinkPrefixBitsMax {
		return errors.New(reason.Str(" is longer than a /24, so a member's host number may not fit; use a /24").String())
	}
	if len(link.members) < 2 {
		return errors.New(reason.Str(" has fewer than two members, so nothing can cross it; name both ends").String())
	}
	var member [len(linkLocalOwners)]bool
	for _, owner := range link.members {
		if owner < 0 {
			return errors.New(reason.Str(" names an owner outside linkLocalOwners; name one by its constant, as in frrLinkLocalOwner").String())
		}
		if owner >= len(linkLocalOwners) {
			return errors.New(reason.Str(" names an owner outside linkLocalOwners; name one by its constant, as in frrLinkLocalOwner").String())
		}
		if member[owner] {
			return errors.New(reason.Str(" names ").Str(linkLocalOwners[owner].peer).Str(" twice; name each member once").String())
		}
		member[owner] = true
		if !peerPrepared(peers, linkLocalOwners[owner].peer) {
			return errors.New(reason.Str(" names ").Str(linkLocalOwners[owner].peer).
				Str(", and the scenario does not start that container; add its config file to the scenario directory").String())
		}
	}
	return nil
}

// writeLinkScript appends the commands that put one member on link as its
// ordinal-th link device, at host on the link's subnet.
func writeLinkScript(script *textbuf.Buffer, link *labLink, ordinal int, host uint8) {
	var device textbuf.Buffer
	device.Str("eth").Str(strconv.Itoa(ordinal))
	name := device.String()
	octets := link.ipv4.Addr().As4()
	octets[3] = host
	script.Str("ip link add link eth0 name ").Str(name).Str(" type vlan id ").Str(strconv.Itoa(int(link.vlan))).Byte('\n')
	script.Str("ip address add ").Addr(netip.AddrFrom4(octets)).Byte('/').Str(strconv.Itoa(link.ipv4.Bits())).
		Str(" dev ").Str(name).Byte('\n')
	script.Str("ip link set ").Str(name).Str(" up\n")
}

// peerPrepared reports whether the scenario starts the container named peer.
func peerPrepared(peers []interoplab.PeerConfig, peer string) bool {
	for i := range peers {
		if peers[i].Name == peer {
			return true
		}
	}
	return false
}
