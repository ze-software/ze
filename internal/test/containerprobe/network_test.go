package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"testing"
)

// routeWord answers the column text the kernel prints in /proc/net/route for
// address: the 32-bit word stored in network byte order, read in the host's
// byte order and printed as eight hex digits.
func routeWord(address string) string {
	octets := netip.MustParseAddr(address).As4()
	return fmt.Sprintf("%08X", binary.NativeEndian.Uint32(octets[:]))
}

// routeTable builds a /proc/net/route text from its route lines.
func routeTable(lines ...string) string {
	header := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT"
	return header + "\n" + strings.Join(lines, "\n") + "\n"
}

// routeLine builds one route line of eth0 in the kernel's column order.
func routeLine(destination, gateway, flags, mask string) string {
	return "eth0\t" + destination + "\t" + gateway + "\t" + flags + "\t0\t0\t0\t" + mask + "\t0\t0\t0"
}

// TestDefaultRouteReadsGatewayAndDevice proves the IPv4 default route is read
// with its gateway in the right byte order and its device, beside the
// network's own direct route. Method: the table a container on 10.88.0.0/24
// holds, built with the host's byte order the way the kernel prints it.
func TestDefaultRouteReadsGatewayAndDevice(t *testing.T) {
	table := routeTable(
		routeLine("00000000", routeWord("10.88.0.1"), "0003", "00000000"),
		routeLine(routeWord("10.88.0.0"), "00000000", "0001", routeWord("255.255.255.0")),
	)
	found, err := defaultRoute(table)
	if err != nil {
		t.Fatalf("defaultRoute refused a table with a default route: %v", err)
	}
	if found.gateway != netip.MustParseAddr("10.88.0.1") {
		t.Errorf("gateway: got %s, want 10.88.0.1", found.gateway)
	}
	if found.device != "eth0" {
		t.Errorf("device: got %q, want eth0", found.device)
	}
	if got := routeFact(found); got != "via 10.88.0.1 dev eth0" {
		t.Errorf("routeFact: got %q", got)
	}
}

// TestDefaultRouteAbsentIsNamed proves a table without a default route, or a
// default entry that is down or has no gateway, answers errNoDefaultRoute and
// never a zero address. Method: three tables, each one step short of a
// default route through a gateway.
func TestDefaultRouteAbsentIsNamed(t *testing.T) {
	tables := map[string]string{
		"direct route only": routeTable(routeLine(routeWord("10.88.0.0"), "00000000", "0001", routeWord("255.255.255.0"))),
		"no gateway flag":   routeTable(routeLine("00000000", routeWord("10.88.0.1"), "0001", "00000000")),
		"not up":            routeTable(routeLine("00000000", routeWord("10.88.0.1"), "0002", "00000000")),
		"header alone":      routeTable(),
	}
	for name, table := range tables {
		_, err := defaultRoute(table)
		if !errors.Is(err, errNoDefaultRoute) {
			t.Errorf("%s: got %v, want errNoDefaultRoute", name, err)
		}
	}
}

// TestDefaultRouteRefusesAnUnreadableLine proves a line the probe cannot read
// is an error quoting it, never skipped as if it held no default route.
func TestDefaultRouteRefusesAnUnreadableLine(t *testing.T) {
	_, err := defaultRoute(routeTable("eth0\t00000000\t0100580A"))
	if err == nil {
		t.Fatal("defaultRoute accepted a line with three fields")
	}
	if errors.Is(err, errNoDefaultRoute) {
		t.Fatalf("an unreadable line answered as no default route: %v", err)
	}
}

// TestReachFactJudgesTheAnswer proves which outcomes of the connection
// attempt mean the gateway was reached: an accepted connection and a refused
// one, both of which need an answer from the gateway, and nothing else.
// Method: the errors the dialer returns, wrapped as net.Dialer wraps them.
func TestReachFactJudgesTheAnswer(t *testing.T) {
	dialError := func(errno syscall.Errno) error {
		return &net.OpError{Op: "dial", Net: "tcp4", Err: os.NewSyscallError("connect", errno)}
	}
	cases := []struct {
		name    string
		err     error
		reached bool
	}{
		{"accepted", nil, true},
		{"refused", dialError(syscall.ECONNREFUSED), true},
		{"host unreachable", dialError(syscall.EHOSTUNREACH), false},
		{"network unreachable", dialError(syscall.ENETUNREACH), false},
		{"timed out", dialError(syscall.ETIMEDOUT), false},
		{"bound passed", &net.OpError{Op: "dial", Net: "tcp4", Err: context.DeadlineExceeded}, false},
	}
	for _, c := range cases {
		fact := reachFact(c.err)
		if got := strings.HasPrefix(fact, reachedPrefix); got != c.reached {
			t.Errorf("%s: reached %v, want %v (fact %q)", c.name, got, c.reached, fact)
		}
		if !c.reached && !strings.HasPrefix(fact, notReachedPrefix) {
			t.Errorf("%s: fact %q does not say not reached", c.name, fact)
		}
	}
}

// wiredFacts is the report of a container attached as the handoff
// describes: eth0 with 10.88.0.2/24, MTU 1500, up, the default route via
// 10.88.0.1, and the gateway's refusal of the attempt.
func wiredFacts() map[string]string {
	return map[string]string{
		factLinks:              "eth0,lo",
		"link-eth0-addresses":  "10.88.0.2/24,fe80::1/64",
		"link-eth0-mtu":        "1500",
		"link-eth0-flags":      "up|broadcast|multicast|running",
		factDefaultRoute:       "via 10.88.0.1 dev eth0",
		factGatewayTarget:      "10.88.0.1:9",
		factGatewayReach:       reachedPrefix + "the gateway refused the connection",
		"link-lo-addresses":    "127.0.0.1/8",
		"link-lo-mtu":          "65536",
		"link-lo-flags":        "up|loopback|running",
		"some-unrelated-entry": "kept",
	}
}

// wiredWant is what the .ci asks of that container.
var wiredWant = networkWant{
	address: netip.MustParsePrefix("10.88.0.2/24"),
	gateway: netip.MustParseAddr("10.88.0.1"),
	mtu:     1500,
}

// TestJudgeNetworkAcceptsAWiredContainer proves every check holds for the
// report of a correctly wired container. Method: judge wiredFacts.
func TestJudgeNetworkAcceptsAWiredContainer(t *testing.T) {
	for _, judged := range judgeNetwork(wiredFacts(), wiredWant) {
		if !judged.ok {
			t.Errorf("check %q failed on a wired container, saw %q", judged.what, judged.seen)
		}
	}
}

// TestJudgeNetworkFailsTheOneCheckAFactBreaks proves each fact is judged by
// its own check, and that a missing fact fails it with a reason, never as an
// empty value. Method: change one fact of wiredFacts at a time and require
// exactly the named check to fail.
func TestJudgeNetworkFailsTheOneCheckAFactBreaks(t *testing.T) {
	cases := []struct {
		name   string
		key    string
		value  string
		remove bool
		failed string
	}{
		{"no link at entrypoint start", factLinks, "lo", false, "at entrypoint start the container holds eth0 and lo"},
		{"links unreadable", factLinks, "error: socket: operation not permitted", false, "at entrypoint start the container holds eth0 and lo"},
		{"another address", "link-eth0-addresses", "10.88.0.3/24", false, "eth0 holds 10.88.0.2/24"},
		{"the address with another length", "link-eth0-addresses", "10.88.0.2/16", false, "eth0 holds 10.88.0.2/24"},
		{"no address fact", "link-eth0-addresses", "", true, "eth0 holds 10.88.0.2/24"},
		{"another MTU", "link-eth0-mtu", "1450", false, "eth0 has MTU 1500"},
		{"down", "link-eth0-flags", "broadcast|multicast", false, "eth0 is up"},
		{"no default route", factDefaultRoute, "none: no IPv4 default route", false, "the default route is via 10.88.0.1 dev eth0"},
		{"another gateway", factDefaultRoute, "via 10.88.0.254 dev eth0", false, "the default route is via 10.88.0.1 dev eth0"},
		{"another target", factGatewayTarget, "10.88.0.254:9", false, "the entrypoint dialed 10.88.0.1:9"},
		{"no answer", factGatewayReach, notReachedPrefix + "dial tcp4 10.88.0.1:9: i/o timeout", false, "the gateway answered the connection attempt"},
		{"no reach fact", factGatewayReach, "", true, "the gateway answered the connection attempt"},
	}
	for _, c := range cases {
		facts := wiredFacts()
		if c.remove {
			delete(facts, c.key)
		} else {
			facts[c.key] = c.value
		}
		matched := false
		for _, judged := range judgeNetwork(facts, wiredWant) {
			if judged.what != c.failed {
				if !judged.ok {
					t.Errorf("%s: check %q failed too, saw %q", c.name, judged.what, judged.seen)
				}
				continue
			}
			matched = true
			if judged.ok {
				t.Errorf("%s: check %q held", c.name, judged.what)
			}
			if c.remove && judged.seen != "no "+c.key+" in the report" {
				t.Errorf("%s: a missing fact reads %q", c.name, judged.seen)
			}
		}
		if !matched {
			t.Errorf("%s: no check is named %q", c.name, c.failed)
		}
	}
}
