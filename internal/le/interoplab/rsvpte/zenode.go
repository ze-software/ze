// Design: docs/architecture/testing/interop.md -- the freeRouter RSVP-TE suite
// Related: rsvpte.go -- zePeer and freeRtrPeer, which run the commands built here
//
// zenode.go builds the commands each lab container runs before its daemon
// starts. A Ze node writes the MPLS sysctls every Ze node needs, creates the
// VLANs and adds the routes its scenario declares in <role>-net.txt, then execs
// ze. A freeRouter node joins eth0 to the jar through rawInt.bin. The commands
// are built here, from typed values, so no scenario carries a shell script.

package rsvpte

import (
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// zeNodeInterface is the container's link on the lab segment.
	zeNodeInterface = "eth0"
	// vlanNameOctetsMax is IFNAMSIZ less its terminating NUL: Linux refuses a
	// longer interface name.
	vlanNameOctetsMax = 15
	// vlanIDMax is the largest VLAN ID 802.1Q leaves usable; 0 and 4095 are reserved.
	vlanIDMax = 4094

	// The two line forms of <role>-net.txt, by first word and full shape.
	netLineVLAN  = "vlan"
	netLineRoute = "route"
	vlanUsage    = "want vlan <name> id <1-4094> address <address/length>"
	routeUsage   = "want route <prefix> via <gateway>"
)

// zeNodeNet is what a Ze node's optional <role>-net.txt declares beyond the
// sysctls every Ze node writes: the VLANs it creates on eth0 and the routes it
// adds. A missing file declares neither.
type zeNodeNet struct {
	vlans  []zeVLAN
	routes []zeRoute
}

// zeVLAN is one 802.1Q link on eth0 with the one address the node holds on it.
type zeVLAN struct {
	name    string
	id      uint16
	address netip.Prefix
}

// zeRoute is one static route the node adds on eth0 before its daemon starts.
type zeRoute struct {
	prefix  netip.Prefix
	gateway netip.Addr
}

// readZeNodeNet reads a Ze role's <role>-net.txt. Each line is one of
//
//	vlan <name> id <1-4094> address <address/length>
//	route <prefix> via <gateway>
//
// with `#` comments and blank lines skipped. A missing file declares nothing;
// any other line refuses the plan, so a typo cannot run a scenario without the
// route or VLAN it depends on.
func readZeNodeNet(path string) (zeNodeNet, error) {
	content, err := os.ReadFile(path) //nolint:gosec // a scenario file in the checkout under test
	if errors.Is(err, fs.ErrNotExist) {
		return zeNodeNet{}, nil
	}
	if err != nil {
		return zeNodeNet{}, fmt.Errorf("read Ze node network %s: %w", path, err)
	}
	var network zeNodeNet
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := network.add(strings.Fields(line)); err != nil {
			return zeNodeNet{}, fmt.Errorf("Ze node network %s: line %q: %w", path, line, err)
		}
	}
	return network, nil
}

// add appends the VLAN or route one line declares.
func (n *zeNodeNet) add(fields []string) error {
	switch fields[0] {
	case netLineVLAN:
		vlan, err := parseZeVLAN(fields[1:])
		if err != nil {
			return err
		}
		n.vlans = append(n.vlans, vlan)
		return nil
	case netLineRoute:
		route, err := parseZeRoute(fields[1:])
		if err != nil {
			return err
		}
		n.routes = append(n.routes, route)
		return nil
	}
	return errors.New("the first word must be " + netLineVLAN + " or " + netLineRoute)
}

// parseZeVLAN reads `<name> id <1-4094> address <address/length>`.
func parseZeVLAN(fields []string) (zeVLAN, error) {
	if len(fields) != 5 {
		return zeVLAN{}, errors.New(vlanUsage)
	}
	if fields[1] != "id" {
		return zeVLAN{}, errors.New(vlanUsage)
	}
	if fields[3] != "address" {
		return zeVLAN{}, errors.New(vlanUsage)
	}
	name := fields[0]
	if !validLinkName(name) {
		return zeVLAN{}, fmt.Errorf("VLAN name %q must be 1 to %d lowercase letters and digits", name, vlanNameOctetsMax)
	}
	id, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return zeVLAN{}, fmt.Errorf("VLAN id %q: %w", fields[2], err)
	}
	if id == 0 {
		return zeVLAN{}, fmt.Errorf("VLAN id %d is reserved: use 1 to %d", id, vlanIDMax)
	}
	if id > vlanIDMax {
		return zeVLAN{}, fmt.Errorf("VLAN id %d is above %d", id, vlanIDMax)
	}
	address, err := netip.ParsePrefix(fields[4])
	if err != nil {
		return zeVLAN{}, fmt.Errorf("VLAN address: %w", err)
	}
	return zeVLAN{name: name, id: uint16(id), address: address}, nil
}

// parseZeRoute reads `<prefix> via <gateway>`.
func parseZeRoute(fields []string) (zeRoute, error) {
	if len(fields) != 3 {
		return zeRoute{}, errors.New(routeUsage)
	}
	if fields[1] != "via" {
		return zeRoute{}, errors.New(routeUsage)
	}
	prefix, err := netip.ParsePrefix(fields[0])
	if err != nil {
		return zeRoute{}, fmt.Errorf("route prefix: %w", err)
	}
	if prefix != prefix.Masked() {
		return zeRoute{}, fmt.Errorf("route prefix %s has host bits set: write %s", prefix, prefix.Masked())
	}
	gateway, err := netip.ParseAddr(fields[2])
	if err != nil {
		return zeRoute{}, fmt.Errorf("route gateway: %w", err)
	}
	return zeRoute{prefix: prefix, gateway: gateway}, nil
}

// validLinkName answers whether name is a usable interface name made of
// lowercase letters and digits, so it is one shell word without quoting.
func validLinkName(name string) bool {
	if name == "" {
		return false
	}
	if len(name) > vlanNameOctetsMax {
		return false
	}
	for _, c := range name {
		if !isLowerAlnum(c) {
			return false
		}
	}
	return true
}

func isLowerAlnum(c rune) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	return c >= '0' && c <= '9'
}

// zeNodeSysctls answers every key=value a Ze node writes: MPLS label space,
// MPLS input on eth0 and on each VLAN, and IPv4 forwarding. The privilege test
// checks each key against the profile the node runs under.
func zeNodeSysctls(vlans []zeVLAN) []string {
	sysctls := make([]string, 0, 3+len(vlans))
	sysctls = append(sysctls, "net.mpls.platform_labels=1048575", "net.mpls.conf."+zeNodeInterface+".input=1")
	for _, vlan := range vlans {
		sysctls = append(sysctls, "net.mpls.conf."+vlan.name+".input=1")
	}
	return append(sysctls, "net.ipv4.ip_forward=1")
}

// zeNodeScript is the shell a Ze node's container runs: start the capture,
// create each VLAN, write the sysctls (after the VLANs, whose MPLS input they
// name), add the routes, then exec the daemon. One command per line, so the
// privilege test reads each sysctl write on its own.
func zeNodeScript(network zeNodeNet) string {
	var tb textbuf.Buffer
	tb.Str("mkdir -p /run/fr\n")
	tb.Str("tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > ").Str(captureFile).Str(" 2> /run/fr/tcpdump.err &\n")
	tb.Str("set -eu\n")
	for _, vlan := range network.vlans {
		tb.Str("ip link add link ").Str(zeNodeInterface).Str(" name ").Str(vlan.name).Str(" type vlan id ").Int(int64(vlan.id)).Byte('\n')
		tb.Str("ip addr add ").Str(vlan.address.String()).Str(" dev ").Str(vlan.name).Byte('\n')
		tb.Str("ip link set ").Str(vlan.name).Str(" up\n")
	}
	tb.Str("sysctl -w ").Str(strings.Join(zeNodeSysctls(network.vlans), " ")).Byte('\n')
	for _, route := range network.routes {
		tb.Str("ip route add ").Str(route.prefix.String()).Str(" via ").Str(route.gateway.String()).Str(" dev ").Str(zeNodeInterface).Byte('\n')
	}
	tb.Str("exec ze start /etc/ze/ze.conf\n")
	return tb.String()
}

// freeRtrScript is the shell a freeRouter node's container runs. freeRouter
// owns its own IPv4 stack and MAC on the shared segment, so eth0 is promiscuous
// and rawInt.bin carries every frame between eth0 and the jar over loopback
// UDP. tcpdump records the RSVP this peer sends and receives, with MPLS frames
// too, because a PATH carried through a bypass LSP is labeled. The scenario
// mounts rtr-hw.txt and rtr-sw.txt under /etc/freertr.
//
// rawInt's connected UDP reader exits on ICMP port-unreachable, so the jar's
// socket on 127.0.0.1:26001 (0x6591) MUST be bound before rawInt starts.
const freeRtrScript = "set -eu\n" +
	"mkdir -p /run/fr\n" +
	"ip link set eth0 promisc on\n" +
	"tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > " + captureFile + " 2> /run/fr/tcpdump.err &\n" +
	"java -Xms32m -Xmx256m -jar /peer/rtr.jar routers /etc/freertr/rtr-hw.txt /etc/freertr/rtr-sw.txt > /run/fr/console.txt 2>&1 &\n" +
	"timeout 60 sh -c \"until grep -q '0100007F:6591' /proc/net/udp; do sleep 1; done\"\n" +
	"exec /peer/rawInt.bin eth0 26002 127.0.0.1 26001 127.0.0.1\n"
