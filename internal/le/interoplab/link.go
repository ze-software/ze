// Design: docs/architecture/testing/interop.md -- separate links between daemons.
// Related: lab.go -- runScenario validates the links and wraps each member's start.
// Related: docker.go -- PeerConfig.Prestart and the image start the wrapper execs.
package interoplab

import (
	"encoding/json"
	"errors"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Link is one layer-2 segment of a scenario that only its members join: a
// VLAN over the scenario's one network. Every container sits on the network's
// untagged eth0, so a protocol that finds its neighbors by multicast or
// broadcast (IS-IS, OSPF, PPPoE discovery) meets every container there. A
// daemon configured on a link meets only the other members, because the
// kernel of a container with no device on that VLAN never hands a tagged
// frame to a protocol socket. A packet socket on eth0 (tcpdump) still
// receives it: veth carries the tag out of band, so a filter on the inner
// ethertype matches, and only `tcpdump -e` prints the 802.1Q header.
//
// Each member reaches the link as eth<N>, where N counts the links that
// member joins in declaration order and reaches under no name of Devices (its
// first such link is eth1). A link with an
// IPv4 prefix gives each member its host number in the last octet; one with
// an IPv6 prefix gives each member the /64 address whose last octet is its
// host number. A link with neither prefix is a pure layer-2 segment (PPPoE, a
// LAN that DHCP numbers): its members hold the device up with no address.
// The scenario's files name those devices and addresses.
//
// One end of a link may be a planned peer that is not a member: External,
// which terminates the VLAN inside its own stack on no device the harness
// builds. freeRouter is one: rawInt.bin hands every frame of its container's
// eth0, tagged ones included, to the jar, whose own configuration holds the
// VLAN subinterface and its address.
type Link struct {
	// Name is how a refusal and a checker name the link.
	Name string `json:"name"`
	VLAN uint16 `json:"vlan"`
	// IPv4 is the link's network, at most a /24, or the zero Prefix for none.
	IPv4 netip.Prefix `json:"ipv4,omitzero"`
	// IPv6 is the link's /64, or the zero Prefix for none.
	IPv6 netip.Prefix `json:"ipv6,omitzero"`
	// Members are the names of the planned peers that join the link.
	Members []string `json:"members"`
	// External names the planned peer that terminates the link in its own
	// stack, or is empty when every end is a member. The harness builds
	// nothing for it and numbers nothing on it, so a link with an external
	// end needs only one member.
	External string `json:"external,omitempty"`
	// Devices names, by member, the device that member reaches the link as
	// in place of eth<N>, for a device a file the harness does not write
	// names already (a daemon configuration, a checker that takes it down).
	Devices map[string]string `json:"devices,omitempty"`
}

// LinkMembership is one member's place on one link: the device it reaches
// the link as and the addresses it holds there, each the zero Addr when the
// link carries no prefix of that family.
type LinkMembership struct {
	Link   string     `json:"link"`
	Device string     `json:"device"`
	IPv4   netip.Addr `json:"ipv4,omitzero"`
	IPv6   netip.Addr `json:"ipv6,omitzero"`
}

// linkVLANMax is the highest usable 802.1Q VLAN identifier; 0 and 4095 are
// reserved.
const linkVLANMax = 4094

// linkIPv4BitsMax keeps a member's host number inside the link's network.
const linkIPv4BitsMax = 24

// linkIPv6Bits is the one IPv6 prefix length a link takes: the member's host
// number fills the last octet of a /64, as Docker numbers the network itself.
const linkIPv6Bits = 64

// linkHostMax is the highest host number a member may hold on a link: 255 is
// the broadcast address of a /24.
const linkHostMax = 254

// linkDeviceOctetsMax is IFNAMSIZ less its terminating NUL: Linux refuses a
// longer interface name.
const linkDeviceOctetsMax = 15

// linkDevicePrefix is the name the harness numbers a member's devices under.
const linkDevicePrefix = "eth"

// ValidateLinks refuses a link the harness cannot build as declared: a VLAN
// outside 1 to 4094 or used twice, an IPv4 prefix that is not a network of at
// most /24, an IPv6 prefix that is not a /64 network, fewer than two ends, a
// member named twice, a member or external end the scenario does not plan, an
// external end that is also a member, a member whose host number cannot hold
// an address, or a device name a member cannot hold. Each such link would
// carry nothing, and the checker would read a silent lab as the daemon's
// fault, so the scenario fails before any container starts.
func ValidateLinks(scenario string, links []Link, peers []PeerConfig) error {
	hosts := make(map[string]uint8, len(peers))
	for index := range peers {
		hosts[peers[index].Name] = peers[index].Host
	}
	vlans := make(map[uint16]string, len(links))
	devices := make(map[linkDevice]string, len(links))
	for index := range links {
		link := &links[index]
		if err := validateLink(scenario, link, vlans, hosts); err != nil {
			return err
		}
		if err := validateLinkDevices(scenario, link, devices); err != nil {
			return err
		}
	}
	return nil
}

// linkDevice is one device a member holds, the key ValidateLinks refuses a
// second device name on.
type linkDevice struct {
	member string
	device string
}

// validateLink is ValidateLinks' refusal of one declared link.
func validateLink(scenario string, link *Link, vlans map[uint16]string, hosts map[string]uint8) error {
	var reason textbuf.Buffer
	reason.Str(scenario).Str("'s link ").Str(link.Name).Str(" on VLAN ").Int(int64(link.VLAN))
	if link.Name == "" {
		return errors.New(reason.Str(" has no name; name it so a refusal and a checker can say which link").String())
	}
	if link.VLAN == 0 {
		return errors.New(reason.Str(" is not a usable VLAN; declare one from 1 to 4094 in the scenario's Links").String())
	}
	if link.VLAN > linkVLANMax {
		return errors.New(reason.Str(" is not a usable VLAN; declare one from 1 to 4094 in the scenario's Links").String())
	}
	if other, seen := vlans[link.VLAN]; seen {
		return errors.New(reason.Str(" reuses the VLAN of link ").Str(other).Str("; give each link of the scenario its own VLAN").String())
	}
	vlans[link.VLAN] = link.Name
	if err := validateLinkIPv4(&reason, link.IPv4); err != nil {
		return err
	}
	if err := validateLinkIPv6(&reason, link.IPv6); err != nil {
		return err
	}
	if err := validateLinkExternal(&reason, link, hosts); err != nil {
		return err
	}
	named := make(map[string]bool, len(link.Members))
	for _, member := range link.Members {
		if named[member] {
			return errors.New(reason.Str(" names ").Str(member).Str(" twice; name each member once").String())
		}
		named[member] = true
		host, planned := hosts[member]
		if !planned {
			return errors.New(reason.Str(" names ").Str(member).
				Str(", and the scenario does not plan that peer; plan it, or name a planned peer").String())
		}
		if host == 0 {
			return errors.New(reason.Str(" names ").Str(member).Str(", whose host number 0 holds no address; give it a host from 1 to 254").String())
		}
		if host > linkHostMax {
			return errors.New(reason.Str(" names ").Str(member).Str(", whose host number ").Int(int64(host)).
				Str(" is the /24 broadcast; give it a host from 1 to 254").String())
		}
	}
	return nil
}

// validateLinkExternal refuses a link with fewer than two ends, counting its
// external end, and an external end the scenario does not plan or that is
// also a member, which would then get a device the far end never reads.
func validateLinkExternal(reason *textbuf.Buffer, link *Link, hosts map[string]uint8) error {
	if link.External == "" {
		if len(link.Members) < 2 {
			return errors.New(reason.Str(" has fewer than two members, so nothing can cross it; name both ends, " +
				"or name the planned peer that terminates it in its own stack as External").String())
		}
		return nil
	}
	if _, planned := hosts[link.External]; !planned {
		return errors.New(reason.Str(" names ").Str(link.External).
			Str(" as its external end, and the scenario does not plan that peer; plan it, or name a planned peer").String())
	}
	if linkHasMember(link, link.External) {
		return errors.New(reason.Str(" names ").Str(link.External).
			Str(" both as a member and as its external end; a member gets a device the harness builds, " +
				"an external end builds its own, so name it once").String())
	}
	if len(link.Members) == 0 {
		return errors.New(reason.Str(" has an external end and no member, so the harness builds nothing on it; " +
			"name the member that meets ").Str(link.External).String())
	}
	return nil
}

// validateLinkDevices refuses a device name link gives a member that the
// member cannot hold: one for a peer that is not a member, a name Linux or
// the shell would refuse, one the harness numbers or the container holds
// already, or a name the member holds on another link. devices carries the
// names already given across the scenario's links.
func validateLinkDevices(scenario string, link *Link, devices map[linkDevice]string) error {
	members := make([]string, 0, len(link.Devices))
	for member := range link.Devices {
		members = append(members, member)
	}
	slices.Sort(members)
	for _, member := range members {
		device := link.Devices[member]
		var reason textbuf.Buffer
		reason.Str(scenario).Str("'s link ").Str(link.Name).Str(" names device ").Str(device).Str(" for ").Str(member)
		if !linkHasMember(link, member) {
			return errors.New(reason.Str(", which is not a member; name a device only for a member of the link").String())
		}
		if !linkDeviceNameValid(device) {
			return errors.New(reason.Str("; a device name is 1 to ").Int(linkDeviceOctetsMax).
				Str(" lowercase letters and digits, so Linux takes it and the shell reads it as one word").String())
		}
		if linkDeviceNumbered(device) {
			return errors.New(reason.Str("; eth<N> is the name the harness numbers each member's links under, " +
				"so choose a name outside it").String())
		}
		if device == "lo" {
			return errors.New(reason.Str("; every container holds lo already, so choose another name").String())
		}
		key := linkDevice{member: member, device: device}
		if other, seen := devices[key]; seen {
			return errors.New(reason.Str(", which ").Str(member).Str(" holds on link ").Str(other).
				Str(" already; give each of its links its own device").String())
		}
		devices[key] = link.Name
	}
	return nil
}

// linkDeviceNameValid answers whether name is an interface name Linux takes
// that is also one shell word without quoting: 1 to 15 lowercase letters and
// digits.
func linkDeviceNameValid(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > linkDeviceOctetsMax {
		return false
	}
	for _, c := range name {
		if !linkDeviceOctet(c) {
			return false
		}
	}
	return true
}

func linkDeviceOctet(c rune) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	return c >= '0' && c <= '9'
}

// linkDeviceNumbered answers whether name is eth followed by digits only, the
// form of the names LinkMemberships gives (and of eth0, the scenario network).
func linkDeviceNumbered(name string) bool {
	digits, found := strings.CutPrefix(name, linkDevicePrefix)
	if !found {
		return false
	}
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validateLinkIPv4 refuses an IPv4 prefix a member's address cannot be built
// in. The zero Prefix is a link with no IPv4 address.
func validateLinkIPv4(reason *textbuf.Buffer, prefix netip.Prefix) error {
	if !prefix.IsValid() {
		return nil
	}
	if !prefix.Addr().Is4() {
		return errors.New(reason.Str(" carries ").Str(prefix.String()).Str(" as its IPv4 prefix; write an IPv4 network, as in 10.101.0.0/24").String())
	}
	if prefix.Masked() != prefix {
		return errors.New(reason.Str(" names the host ").Str(prefix.String()).Str(", not a network; write ").Str(prefix.Masked().String()).String())
	}
	if prefix.Bits() > linkIPv4BitsMax {
		return errors.New(reason.Str(" has an IPv4 prefix longer than a /24, so a member's host number may not fit; use a /24").String())
	}
	return nil
}

// validateLinkIPv6 refuses an IPv6 prefix that is not a /64 network. The zero
// Prefix is a link with no IPv6 address.
func validateLinkIPv6(reason *textbuf.Buffer, prefix netip.Prefix) error {
	if !prefix.IsValid() {
		return nil
	}
	if !prefix.Addr().Is6() {
		return errors.New(reason.Str(" carries ").Str(prefix.String()).Str(" as its IPv6 prefix; write an IPv6 /64, as in fd00:101::/64").String())
	}
	if prefix.Addr().Is4In6() {
		return errors.New(reason.Str(" carries ").Str(prefix.String()).Str(" as its IPv6 prefix; write an IPv6 /64, as in fd00:101::/64").String())
	}
	if prefix.Bits() != linkIPv6Bits {
		return errors.New(reason.Str(" has an IPv6 prefix that is not a /64; use a /64, as in fd00:101::/64").String())
	}
	if prefix.Masked() != prefix {
		return errors.New(reason.Str(" names the host ").Str(prefix.String()).Str(", not a network; write ").Str(prefix.Masked().String()).String())
	}
	return nil
}

// LinkMemberships answers, in declaration order, the links peer joins and
// what it holds on each, from links ValidateLinks accepted. A link whose
// Devices names peer's device gives that name and takes no eth<N>, so the
// numbered devices stay eth1, eth2 and on with no gap. An external end is not
// a member and holds nothing.
func LinkMemberships(links []Link, peer string, host uint8) []LinkMembership {
	var memberships []LinkMembership
	numbered := 0
	for index := range links {
		link := &links[index]
		if !linkHasMember(link, peer) {
			continue
		}
		device, named := link.Devices[peer]
		if !named {
			numbered++
			var numberedDevice textbuf.Buffer
			device = numberedDevice.Str(linkDevicePrefix).Int(int64(numbered)).String()
		}
		membership := LinkMembership{Link: link.Name, Device: device}
		if link.IPv4.IsValid() {
			octets := link.IPv4.Addr().As4()
			octets[3] = host
			membership.IPv4 = netip.AddrFrom4(octets)
		}
		if link.IPv6.IsValid() {
			octets := link.IPv6.Addr().As16()
			octets[15] = host
			membership.IPv6 = netip.AddrFrom16(octets)
		}
		memberships = append(memberships, membership)
	}
	return memberships
}

func linkHasMember(link *Link, peer string) bool {
	for _, member := range link.Members {
		if member == peer {
			return true
		}
	}
	return false
}

// linkScript renders the commands that put one member on each of its links:
// the VLAN subinterface of eth0, its addresses, then the device up. IPv6 is
// added nodad, so a daemon that binds the address at start never meets it
// tentative.
func linkScript(links []Link, memberships []LinkMembership) string {
	var script textbuf.Buffer
	for _, membership := range memberships {
		vlan := linkVLAN(links, membership.Link)
		script.Str("ip link add link eth0 name ").Str(membership.Device).Str(" type vlan id ").Int(int64(vlan)).Byte('\n')
		if membership.IPv4.IsValid() {
			bits := linkIPv4Bits(links, membership.Link)
			script.Str("ip address add ").Addr(membership.IPv4).Byte('/').Int(int64(bits)).
				Str(" dev ").Str(membership.Device).Byte('\n')
		}
		if membership.IPv6.IsValid() {
			script.Str("ip -6 address add ").Addr(membership.IPv6).Str("/64 dev ").Str(membership.Device).Str(" nodad\n")
		}
		script.Str("ip link set ").Str(membership.Device).Str(" up\n")
	}
	return script.String()
}

func linkVLAN(links []Link, name string) uint16 {
	for index := range links {
		if links[index].Name == name {
			return links[index].VLAN
		}
	}
	panic("BUG: link membership names a link that is not declared: " + name)
}

func linkIPv4Bits(links []Link, name string) int {
	for index := range links {
		if links[index].Name == name {
			return links[index].IPv4.Bits()
		}
	}
	panic("BUG: link membership names a link that is not declared: " + name)
}

// ImageStart is what an image runs when the container passes no override:
// its Config.Entrypoint and Config.Cmd as `docker image inspect` reports them,
// always in exec form (Docker stores a shell-form ENTRYPOINT as
// ["/bin/sh", "-c", "<line>"]).
type ImageStart struct {
	Entrypoint []string `json:"Entrypoint"`
	Command    []string `json:"Cmd"`
}

// parseImageStart reads the `{{json .Config}}` answer of an image inspect.
// An image that declares neither an entrypoint nor a command starts nothing
// a wrapper could exec, so it is refused rather than wrapped into a shell
// that exits at once.
func parseImageStart(image, answer string) (ImageStart, error) {
	var start ImageStart
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &start); err != nil {
		var reason textbuf.Buffer
		return ImageStart{}, errors.New(reason.Str("image ").Str(image).Str(" inspect answered no readable Config (").
			Err(err).Str("); run `docker image inspect ").Str(image).Str("` to see what the daemon holds").String())
	}
	if len(start.Entrypoint) == 0 && len(start.Command) == 0 {
		var reason textbuf.Buffer
		return ImageStart{}, errors.New(reason.Str("image ").Str(image).
			Str(" declares no ENTRYPOINT and no CMD, so a link member's wrapper has nothing to exec; add one to its Dockerfile").String())
	}
	return start, nil
}

// ComposeStart answers peer with its start wrapped for the links it joins and
// its Prestart (wrapStart), against the image start the container would run.
// It is the composition runScenario applies to every link member and every
// peer with a Prestart, so a suite's test reads the start a container gets.
func ComposeStart(peer PeerConfig, links []Link, start ImageStart) PeerConfig {
	wrapStart(&peer, linkScript(links, LinkMemberships(links, peer.Name, peer.Host)), start)
	return peer
}

// wrapStart replaces peer's start with one shell that builds its links, runs
// its Prestart, and then execs the start Docker would have run: the peer's
// own --entrypoint when its arguments pass one (Docker then drops the image's
// CMD), the image's entrypoint otherwise, followed by the peer's command or,
// when it passes none, the image's. The exec takes its argv from "$@", so no
// entrypoint or argument is ever re-quoted into the script. The script runs
// with errexit on, so a link that cannot be built stops the container before
// its daemon starts.
func wrapStart(peer *PeerConfig, links string, start ImageStart) {
	entrypoint := start.Entrypoint
	imageCommand := start.Command
	arguments := make([]string, 0, len(peer.Arguments)+2)
	for index := 0; index < len(peer.Arguments); index++ {
		argument := peer.Arguments[index]
		if argument == entrypointFlag && index+1 < len(peer.Arguments) {
			entrypoint = []string{peer.Arguments[index+1]}
			imageCommand = nil
			index++
			continue
		}
		if value, found := strings.CutPrefix(argument, entrypointFlag+"="); found {
			entrypoint = []string{value}
			imageCommand = nil
			continue
		}
		arguments = append(arguments, argument)
	}
	command := peer.Command
	if len(command) == 0 {
		command = imageCommand
	}
	var script textbuf.Buffer
	script.Str(links)
	if prestart := strings.TrimRight(peer.Prestart, "\n"); prestart != "" {
		script.Str(prestart).Byte('\n')
	}
	script.Str(`exec "$@"`)
	wrapped := make([]string, 0, 3+len(entrypoint)+len(command))
	wrapped = append(wrapped, wrapperErrexit, script.String(), wrapperArgumentsStart)
	wrapped = append(wrapped, entrypoint...)
	wrapped = append(wrapped, command...)
	peer.Arguments = append(arguments, entrypointFlag, wrapperShell)
	peer.Command = wrapped
	peer.Prestart = ""
}

const (
	// entrypointFlag is Docker's flag that replaces an image's entrypoint.
	entrypointFlag = "--entrypoint"
	// wrapperShell runs the wrapper; every link member's image carries it,
	// which the image probe proves before the scenario starts.
	wrapperShell = "/bin/sh"
	// wrapperErrexit runs the script with errexit on.
	wrapperErrexit = "-ec"
	// wrapperArgumentsStart becomes the script's $0, so every word after it
	// reaches the script as "$@".
	wrapperArgumentsStart = "--"
)

// linkToolProbe is the command a link member's image runs to prove it can
// build a link: the wrapper's shell, and ip from iproute2 (busybox's ip
// applet adds a VLAN subinterface the same way).
var linkToolProbe = []string{"-c", "command -v ip"}

// linkToolRefusal names the image that cannot build a link and its fix.
func linkToolRefusal(image, peer string, build ImageBuild, cause error) error {
	var reason textbuf.Buffer
	reason.Str("image ").Str(image).Str(" cannot build link member ").Str(peer).
		Str("'s VLAN devices: it has no ").Str(wrapperShell).Str(" or no ip (").Err(cause).Str("). Fix: ")
	if build.Dockerfile != "" {
		reason.Str("install iproute2 in ").Str(build.Dockerfile)
	} else {
		reason.Str("build ").Str(build.Tag).Str(" from a Dockerfile that installs iproute2")
	}
	return errors.New(reason.String())
}
