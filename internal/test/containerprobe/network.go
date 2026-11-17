// Design: docs/architecture/container/runtime.md -- the test store and its fixture image
// Related: main.go -- the modes, the report and the waits this file adds to
// Related: ../../plugins/container/storeseam_zetest.go -- the instance-name prefix that selects run-networked

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// The networked entrypoint dials one TCP connection to its gateway, on a
// port nothing needs to listen on. A connection the gateway's stack accepts
// (a SYN-ACK came back) or refuses (a RST came back) proves a packet went to
// the gateway and an answer came back, so both mean reached. No answer
// within the bound, an unreachable host (no ARP answer) or an unreachable
// network (no route) means not reached. Nothing in the container's own
// namespace answers for the gateway: it holds no filter of its own.
const (
	gatewayPort      = 9
	gatewayDialBound = 5 * time.Second
)

// The report keys the networked entrypoint adds, and the link it is judged
// on: the container's first attachment is named eth0.
const (
	factDefaultRoute  = "default-route"
	factGatewayTarget = "gateway-target"
	factGatewayReach  = "gateway-reach"
	factLinks         = "links"
	attachedLink      = "eth0"
)

// The two prefixes of the gateway-reach fact: the host-side judge reads only
// the first, and the rest of the value says what came back or what failed.
const (
	reachedPrefix    = "reached: "
	notReachedPrefix = "not reached: "
)

// The route flags of /proc/net/route (RTF_UP, RTF_GATEWAY in linux/route.h).
const (
	routeFlagUp      = 0x1
	routeFlagGateway = 0x2
)

// sysClassNet is where the kernel lists the network devices of the reading
// process's network namespace.
const sysClassNet = "/sys/class/net"

// errNoDefaultRoute is the answer of defaultRoute for a table with no IPv4
// default route: an absent route is an answer of its own, never a zero
// address.
var errNoDefaultRoute = errors.New("no IPv4 default route")

// route is one IPv4 route through a gateway.
type route struct {
	gateway netip.Addr
	device  string
}

// observeNetwork reads, at the start of the entrypoint and with no wait, the
// facts of the container's network: each link with its MTU, flags and
// addresses, the IPv4 default route, and the outcome of one connection
// attempt to the route's gateway. A fact it cannot read is recorded with its
// error, so the host-side judge prints why rather than finding no report.
func observeNetwork() string {
	var report strings.Builder
	report.WriteString(observeLinks())
	routes, err := os.ReadFile("/proc/net/route")
	if err != nil {
		report.WriteString(factDefaultRoute + "=none: " + err.Error() + "\n")
		report.WriteString(factGatewayReach + "=" + notReachedPrefix + "no route table to find the gateway in\n")
		return report.String()
	}
	found, err := defaultRoute(string(routes))
	if err != nil {
		report.WriteString(factDefaultRoute + "=none: " + err.Error() + "\n")
		report.WriteString(factGatewayReach + "=" + notReachedPrefix + "no gateway to dial\n")
		return report.String()
	}
	report.WriteString(factDefaultRoute + "=" + routeFact(found) + "\n")
	target := netip.AddrPortFrom(found.gateway, gatewayPort)
	report.WriteString(factGatewayTarget + "=" + target.String() + "\n")
	report.WriteString(factGatewayReach + "=" + reachFact(dialGateway(target)) + "\n")
	return report.String()
}

// observeLinks answers one line of names and three lines per link: its MTU,
// its flags and its addresses, read through a netlink route socket (the
// default system-call filter admits AF_NETLINK).
func observeLinks() string {
	links, err := net.Interfaces()
	if err != nil {
		return factLinks + "=error: " + err.Error() + "\n"
	}
	var report strings.Builder
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.Name)
		key := linkKey(link.Name)
		report.WriteString(key + "-mtu=" + strconv.Itoa(link.MTU) + "\n")
		report.WriteString(key + "-flags=" + link.Flags.String() + "\n")
		addresses, err := link.Addrs()
		if err != nil {
			report.WriteString(key + "-addresses=error: " + err.Error() + "\n")
			continue
		}
		texts := make([]string, 0, len(addresses))
		for _, address := range addresses {
			texts = append(texts, address.String())
		}
		report.WriteString(key + "-addresses=" + strings.Join(texts, ",") + "\n")
	}
	slices.Sort(names)
	return factLinks + "=" + strings.Join(names, ",") + "\n" + report.String()
}

// linkKey is the report key prefix of the facts of link name.
func linkKey(name string) string {
	return "link-" + name
}

// routeFact is the report value of a default route.
func routeFact(found route) string {
	return "via " + found.gateway.String() + " dev " + found.device
}

// dialGateway makes the one connection attempt, bounded, and answers its
// error. A connection that opened is closed at once: its opening was the
// answer.
func dialGateway(target netip.AddrPort) error {
	ctx, cancel := context.WithTimeout(context.Background(), gatewayDialBound)
	defer cancel()
	dialer := net.Dialer{Timeout: gatewayDialBound}
	connection, err := dialer.DialContext(ctx, "tcp4", target.String())
	if err != nil {
		return err
	}
	return connection.Close()
}

// reachFact is the report value of the outcome of the connection attempt:
// reached when the gateway accepted or refused it, not reached otherwise.
// A refusal is ECONNREFUSED, which Linux answers for a RST, or for an ICMP
// port unreachable, and only the gateway can send either here.
func reachFact(err error) string {
	if err == nil {
		return reachedPrefix + "the gateway accepted the connection"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return reachedPrefix + "the gateway refused the connection"
	}
	return notReachedPrefix + err.Error()
}

// defaultRoute answers the IPv4 default route of routes, the text of
// /proc/net/route: the line whose destination and mask are both zero, that is
// up and goes through a gateway. A table without one answers
// errNoDefaultRoute, and a line it cannot read is an error that quotes it.
//
// The kernel prints each address as the 32-bit word it stores in network
// byte order, read in the host's byte order, so the word is written back in
// the host's order to recover the address bytes.
func defaultRoute(routes string) (route, error) {
	// The first line is the column header; an empty text is a header alone.
	_, body, _ := strings.Cut(strings.TrimSpace(routes), "\n")
	for line := range strings.SplitSeq(body, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return route{}, errors.New("a route line with fewer than 8 fields: " + strconv.Quote(line))
		}
		if fields[1] != "00000000" {
			continue
		}
		if fields[7] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 16)
		if err != nil {
			return route{}, errors.New("a route line with unreadable flags: " + strconv.Quote(line))
		}
		if flags&routeFlagUp == 0 {
			continue
		}
		if flags&routeFlagGateway == 0 {
			continue
		}
		word, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			return route{}, errors.New("a route line with an unreadable gateway: " + strconv.Quote(line))
		}
		var address [4]byte
		binary.NativeEndian.PutUint32(address[:], uint32(word)) //nolint:gosec // ParseUint bounded word to 32 bits
		return route{gateway: netip.AddrFrom4(address), device: fields[0]}, nil
	}
	return route{}, errNoDefaultRoute
}

// networkWant is what the networked judge requires of a report.
type networkWant struct {
	address netip.Prefix // held by eth0
	gateway netip.Addr   // the default route's, and the one dialed
	mtu     int          // of eth0
}

// check is one judged fact: what it states, whether it holds, and what the
// report held for it.
type check struct {
	what string
	ok   bool
	seen string
}

// judgeNetwork judges the facts of a networked entrypoint's report against
// want, one check per fact, in the order a reader debugs them: the link, its
// address, MTU and state, the route, then the gateway.
func judgeNetwork(facts map[string]string, want networkWant) []check {
	key := linkKey(attachedLink)
	links := seenFact(facts, factLinks)
	addresses := seenFact(facts, key+"-addresses")
	mtu := seenFact(facts, key+"-mtu")
	flags := seenFact(facts, key+"-flags")
	defaultVia := seenFact(facts, factDefaultRoute)
	target := seenFact(facts, factGatewayTarget)
	reach := seenFact(facts, factGatewayReach)
	wantRoute := routeFact(route{gateway: want.gateway, device: attachedLink})
	wantTarget := netip.AddrPortFrom(want.gateway, gatewayPort).String()
	return []check{
		{"at entrypoint start the container holds " + attachedLink + " and lo", links == attachedLink+",lo", links},
		{attachedLink + " holds " + want.address.String(), slices.Contains(strings.Split(addresses, ","), want.address.String()), addresses},
		{attachedLink + " has MTU " + strconv.Itoa(want.mtu), mtu == strconv.Itoa(want.mtu), mtu},
		{attachedLink + " is up", slices.Contains(strings.Split(flags, "|"), "up"), flags},
		{"the default route is " + wantRoute, defaultVia == wantRoute, defaultVia},
		{"the entrypoint dialed " + wantTarget, target == wantTarget, target},
		{"the gateway answered the connection attempt", strings.HasPrefix(reach, reachedPrefix), reach},
	}
}

// seenFact answers the report's value for key, or says the report holds
// none, so a missing fact never reads as an empty one.
func seenFact(facts map[string]string, key string) string {
	value, found := facts[key]
	if !found {
		return "no " + key + " in the report"
	}
	return value
}

// networked judges the report of container name, in the test store directory
// store, against want, and prints one line per check.
func networked(store, name string, want networkWant) bool {
	facts, err := awaitReport(filepath.Join(store, "layer", name, "upper", strings.TrimPrefix(reportPath, "/")))
	if err != nil {
		return judge("the container wrote its report", false, err.Error())
	}
	passed := true
	for _, judged := range judgeNetwork(facts, want) {
		passed = judge(judged.what, judged.ok, judged.seen) && passed
	}
	return passed
}

// hostEnd judges the host side of the container's link while it runs: bridge
// has exactly one member, and that member carries the mark ze:owned:OWNER.
// It writes the member's name to record, a file in the test's work
// directory, so unlinked can look for it after the stop, when the bridge may
// be gone too.
func hostEnd(bridge, owner, record string) bool {
	members, err := os.ReadDir(filepath.Join(sysClassNet, bridge, "brif"))
	if err != nil {
		return judge("the bridge "+bridge+" lists its members", false, err.Error())
	}
	names := make([]string, 0, len(members))
	for _, member := range members {
		names = append(names, member.Name())
	}
	if !judge("the bridge "+bridge+" has one member", len(names) == 1, strings.Join(names, ",")) {
		return false
	}
	alias, err := os.ReadFile(filepath.Join(sysClassNet, names[0], "ifalias")) //nolint:gosec // a device the bridge lists
	seen := strings.TrimSpace(string(alias))
	if err != nil {
		seen = err.Error()
	}
	marked := judge("the host end "+names[0]+" carries the mark ze:owned:"+owner, err == nil && seen == "ze:owned:"+owner, seen)
	if err := os.WriteFile(record, []byte(names[0]+"\n"), 0o644); err != nil { //nolint:gosec // a record the next step reads
		return judge("the host end's name is recorded in "+record, false, err.Error())
	}
	fmt.Println("probe: the host end " + names[0] + " is recorded in " + record)
	return marked
}

// unlinked waits, a bounded time, until no device named in record, the file
// hostEnd wrote, is left on the host, then judges each one.
func unlinked(record string) bool {
	names, err := readLines(record)
	if !judge("the recorded host ends are read from "+record, err == nil && len(names) > 0, strings.Join(names, ",")) {
		return false
	}
	for range recordWaitSteps {
		left := 0
		for _, name := range names {
			if _, err := os.Lstat(filepath.Join(sysClassNet, name)); err == nil {
				left++
			}
		}
		if left == 0 {
			break
		}
		time.Sleep(recordWaitStep)
	}
	passed := true
	for _, name := range names {
		_, err := os.Lstat(filepath.Join(sysClassNet, name))
		passed = judge("the host end "+name+" is gone", errors.Is(err, os.ErrNotExist), filepath.Join(sysClassNet, name)) && passed
	}
	return passed
}
