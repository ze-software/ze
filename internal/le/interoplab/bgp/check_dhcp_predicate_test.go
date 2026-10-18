package bgp

import (
	"strings"
	"testing"
)

// dnsmasqLog builds a dnsmasq log from message lines in dnsmasq's own shape.
func dnsmasqLog(lines ...string) string {
	var log strings.Builder
	for _, line := range lines {
		log.WriteString("dnsmasq-dhcp[7]: 3418273 " + line + "\n")
		log.WriteString("dnsmasq-dhcp[7]: 3418273 requested options: 1:netmask, 3:router\n")
	}
	return log.String()
}

const zeMAC = "02:42:ac:1e:00:02"

// TestDnsmasqEventsCountsKindAndAddress checks the line reader the checkers
// rest on: the kind must be followed by the interface, the address field must
// match exactly, and option-detail lines count for nothing.
func TestDnsmasqEventsCountsKindAndAddress(t *testing.T) {
	log := dnsmasqLog(
		"DHCPDISCOVER(eth0) "+zeMAC,
		"DHCPOFFER(eth0) 172.30.0.50 "+zeMAC,
		"DHCPREQUEST(eth0) 172.30.0.50 "+zeMAC,
		"DHCPACK(eth0) 172.30.0.50 "+zeMAC,
		"DHCPACK(eth0) 172.30.0.500 "+zeMAC,
		"DHCPACKX(eth0) 172.30.0.50 "+zeMAC,
	)
	cases := []struct {
		kind, address string
		want          int
	}{
		{dnsmasqDiscover, "", 1},
		{dnsmasqRequest, dhcpLeasedAddress, 1},
		{dnsmasqAck, dhcpLeasedAddress, 1},
		{dnsmasqAck, "", 2},
		{dnsmasqNak, "", 0},
	}
	for _, c := range cases {
		if got := dnsmasqEvents(log, c.kind, c.address); got != c.want {
			t.Errorf("dnsmasqEvents(%s, %q) = %d, want %d", c.kind, c.address, got, c.want)
		}
	}
}

// TestDHCPRenewalVerdict checks AC-24's first half on both sides of each
// bound: six renewals wait, seven pass, and a second DHCPDISCOVER fails at
// once whatever the renewal count.
func TestDHCPRenewalVerdict(t *testing.T) {
	bind := []string{"DHCPDISCOVER(eth0) " + zeMAC, "DHCPACK(eth0) 172.30.0.50 " + zeMAC}
	renew := "DHCPACK(eth0) 172.30.0.50 " + zeMAC
	cases := []struct {
		name     string
		lines    []string
		done, ok bool
	}{
		{"nothing yet", nil, false, false},
		{"six renewals", append(append([]string(nil), bind...), repeat(renew, 6)...), false, false},
		{"seven renewals", append(append([]string(nil), bind...), repeat(renew, 7)...), true, true},
		{"second discover", append(append([]string(nil), bind...), append(repeat(renew, 7), "DHCPDISCOVER(eth0) "+zeMAC)...), true, false},
	}
	for _, c := range cases {
		done, err := dhcpRenewalVerdict(dnsmasqLog(c.lines...))
		if done != c.done || (err == nil) != c.ok {
			t.Errorf("%s: done=%v err=%v, want done=%v ok=%v", c.name, done, err, c.done, c.ok)
		}
	}
}

// TestDHCPNakVerdict checks AC-24's second half: each of NAK, second
// DHCPDISCOVER and ACK of the reserved address is required.
func TestDHCPNakVerdict(t *testing.T) {
	nak := "DHCPNAK(eth0) 172.30.0.50 " + zeMAC + " static lease available"
	discover := "DHCPDISCOVER(eth0) " + zeMAC
	reserved := "DHCPACK(eth0) 172.30.0.51 " + zeMAC
	cases := []struct {
		name  string
		lines []string
		ok    bool
	}{
		{"complete", []string{discover, nak, discover, reserved}, true},
		{"no nak", []string{discover, discover, reserved}, false},
		{"no second discover", []string{discover, nak, reserved}, false},
		{"no ack of the reserved address", []string{discover, nak, discover}, false},
	}
	for _, c := range cases {
		if err := dhcpNakVerdict(dnsmasqLog(c.lines...)); (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

// TestDHCPRebindVerdict checks AC-25 on the second server's log: a
// REQUEST and ACK of the leased address pass, a DHCPDISCOVER after the stop
// fails at once, and a DISCOVER seen before the stop does not count.
func TestDHCPRebindVerdict(t *testing.T) {
	discover := "DHCPDISCOVER(eth0) " + zeMAC
	request := "DHCPREQUEST(eth0) 172.30.0.50 " + zeMAC
	ack := "DHCPACK(eth0) 172.30.0.50 " + zeMAC
	cases := []struct {
		name     string
		lines    []string
		before   int
		done, ok bool
	}{
		{"rebound", []string{discover, request, ack}, 1, true, true},
		{"no ack yet", []string{discover, request}, 1, false, false},
		{"discover after the stop", []string{discover, discover, request, ack}, 1, true, false},
	}
	for _, c := range cases {
		done, err := dhcpRebindVerdict(dnsmasqLog(c.lines...), c.before)
		if done != c.done || (err == nil) != c.ok {
			t.Errorf("%s: done=%v err=%v, want done=%v ok=%v", c.name, done, err, c.done, c.ok)
		}
	}
}

func repeat(line string, count int) []string {
	lines := make([]string, count)
	for index := range lines {
		lines[index] = line
	}
	return lines
}
