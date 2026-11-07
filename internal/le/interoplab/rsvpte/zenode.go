// Design: docs/architecture/testing/interop.md -- the freeRouter RSVP-TE suite
// Related: rsvpte.go -- zePeer and freeRtrPeer, which run the commands built here
//
// zenode.go builds the commands each lab container runs before its daemon
// starts. A Ze node writes the MPLS sysctls every Ze node needs, on eth0 and
// on each device of the links its scenario declares (labLinks, built by the
// core's start wrapper before this script runs), adds the routes its
// <role>-net.txt declares, then execs ze. A freeRouter node joins eth0 to the
// jar through rawInt.bin. The commands are built here, from typed values, so
// no scenario carries a shell script.

package rsvpte

import (
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// zeNodeInterface is the container's link on the lab segment.
	zeNodeInterface = "eth0"

	// The one line form of <role>-net.txt, by first word and full shape.
	netLineRoute = "route"
	routeUsage   = "want route <prefix> via <gateway>"
)

// zeNodeNet is what a Ze node's optional <role>-net.txt declares beyond the
// sysctls every Ze node writes: the routes it adds. A missing file declares
// none.
type zeNodeNet struct {
	routes []zeRoute
}

// zeRoute is one static route the node adds on eth0 before its daemon starts.
type zeRoute struct {
	prefix  netip.Prefix
	gateway netip.Addr
}

// readZeNodeNet reads a Ze role's <role>-net.txt. Each line is
//
//	route <prefix> via <gateway>
//
// with `#` comments and blank lines skipped. A missing file declares nothing;
// any other line refuses the plan, so a typo cannot run a scenario without the
// route it depends on. A VLAN is a link of the scenario (labLinks), never a
// line here.
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
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if err := network.add(strings.Fields(line)); err != nil {
			return zeNodeNet{}, fmt.Errorf("ze node network %s: line %q: %w", path, line, err)
		}
	}
	return network, nil
}

// add appends the route one line declares.
func (n *zeNodeNet) add(fields []string) error {
	if fields[0] != netLineRoute {
		return errors.New("the first word must be " + netLineRoute +
			"; declare a VLAN as one of the scenario's links in labLinks (internal/le/interoplab/rsvpte/rsvpte.go)")
	}
	route, err := parseZeRoute(fields[1:])
	if err != nil {
		return err
	}
	n.routes = append(n.routes, route)
	return nil
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

// zeNodeSysctls answers every key=value a Ze node writes: MPLS label space,
// MPLS input on eth0 and on each of its link devices, and IPv4 forwarding.
// The privilege test checks each key against the profile the node runs under.
func zeNodeSysctls(devices []string) []string {
	sysctls := make([]string, 0, 3+len(devices))
	sysctls = append(sysctls, "net.mpls.platform_labels=1048575", "net.mpls.conf."+zeNodeInterface+".input=1")
	for _, device := range devices {
		sysctls = append(sysctls, "net.mpls.conf."+device+".input=1")
	}
	return append(sysctls, "net.ipv4.ip_forward=1")
}

// zeNodeScript is the shell a Ze node's container runs: start the capture,
// write the sysctls, add the routes, then exec the daemon. devices are the
// node's link devices, which the core's start wrapper has built before this
// script runs, so the sysctl naming their MPLS input finds them. One command
// per line, so the privilege test reads each sysctl write on its own.
func zeNodeScript(network zeNodeNet, devices []string) string {
	var tb textbuf.Buffer
	tb.Str("mkdir -p /run/fr\n")
	tb.Str("tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > ").Str(captureFile).Str(" 2> /run/fr/tcpdump.err &\n")
	tb.Str("set -eu\n")
	tb.Str("sysctl -w ").Str(strings.Join(zeNodeSysctls(devices), " ")).Byte('\n')
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
