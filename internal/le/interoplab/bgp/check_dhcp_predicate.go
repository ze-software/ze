// Design: docs/architecture/testing/interop.md -- DHCPv4 client evidence read from dnsmasq's log.
// Related: check_dhcp.go -- the lab I/O that feeds these readings.
package bgp

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dnsmasq logs one line per DHCP message it handles, the message type with the
// interface in parentheses, then the address when the message carries one:
//
//	dnsmasq-dhcp[7]: 3418273 DHCPACK(eth0) 172.30.0.50 02:42:ac:1e:00:02
//
// A DHCPDISCOVER line carries the address only when the client asked for one.
const (
	dnsmasqDiscover = "DHCPDISCOVER"
	dnsmasqRequest  = "DHCPREQUEST"
	dnsmasqAck      = "DHCPACK"
	dnsmasqNak      = "DHCPNAK"
)

// dhcpRenewalsWanted is how many RENEW/ACK exchanges after the one DORA the
// renew-NAK scenario requires. AC-24 asks for at least three, but three fit
// inside one 120 s lease: a client that sends RENEW at T1 and REBIND at T2 yet
// never extends its deadlines draws three ACKs before its first lease expires.
// Seven T1 periods of 20 s outlast the lease, so only a client that extends
// it reaches seven without a second DHCPDISCOVER.
const dhcpRenewalsWanted = 7

// dnsmasqEvents counts the lines of a dnsmasq log that record a message of
// kind ("DHCPACK"). With address set, only the lines whose address field is
// that address count; with it empty, every line of that kind counts.
func dnsmasqEvents(log, kind, address string) int {
	count := 0
	for line := range strings.Lines(log) {
		fields := strings.Fields(line)
		for index, field := range fields {
			if !dnsmasqKind(field, kind) {
				continue
			}
			if address == "" {
				count++
				break
			}
			if index+1 < len(fields) && fields[index+1] == address {
				count++
			}
			break
		}
	}
	return count
}

// dnsmasqKind reports whether field is kind followed by the interface in
// parentheses, so "DHCPACK(eth0)" is a DHCPACK and "DHCPACKX(eth0)" is not.
func dnsmasqKind(field, kind string) bool {
	rest, found := strings.CutPrefix(field, kind)
	if !found {
		return false
	}
	return strings.HasPrefix(rest, "(")
}

// dhcpRenewalVerdict reads AC-24's first half from dnsmasq's log: one
// DHCPDISCOVER, then at least dhcpRenewalsWanted renewals of the leased
// address. Every ACK of that address after the first is a renewal, and a
// second DHCPDISCOVER means ze dropped the lease and started over. done is
// false while the log is still on its way to an answer.
func dhcpRenewalVerdict(log string) (done bool, err error) {
	discovers := dnsmasqEvents(log, dnsmasqDiscover, "")
	if discovers > 1 {
		return true, dhcpCountError("ze sent DHCPDISCOVER", discovers, "times while its lease was held, want 1")
	}
	renewals := dnsmasqEvents(log, dnsmasqAck, dhcpLeasedAddress) - 1
	if renewals < dhcpRenewalsWanted {
		return false, dhcpCountError("dnsmasq acknowledged", max(renewals, 0), "renewals of the lease, want at least 7")
	}
	return true, nil
}

// dhcpNakVerdict reads AC-24's second half: after the reservation moved,
// dnsmasq NAKed a request for the old address, ze sent a new DHCPDISCOVER
// (its second), and dnsmasq acknowledged the reserved address.
func dhcpNakVerdict(log string) error {
	if dnsmasqEvents(log, dnsmasqNak, dhcpLeasedAddress) == 0 {
		return errors.New("dnsmasq logged no DHCPNAK for " + dhcpLeasedAddress)
	}
	if discovers := dnsmasqEvents(log, dnsmasqDiscover, ""); discovers < 2 {
		return dhcpCountError("ze sent DHCPDISCOVER", discovers, "times, want a second one after the NAK")
	}
	if dnsmasqEvents(log, dnsmasqAck, dhcpReservedAddress) == 0 {
		return errors.New("dnsmasq logged no DHCPACK for the reserved " + dhcpReservedAddress)
	}
	return nil
}

// dhcpRebindVerdict reads AC-25 from the log of the server that did NOT grant
// the lease: it acknowledged a DHCPREQUEST for the leased address, and it saw
// no DHCPDISCOVER beyond the discoversBefore it had logged when the leasing
// server stopped.
func dhcpRebindVerdict(log string, discoversBefore int) (done bool, err error) {
	if discovers := dnsmasqEvents(log, dnsmasqDiscover, ""); discovers > discoversBefore {
		return true, dhcpCountError("ze sent DHCPDISCOVER", discovers-discoversBefore, "times after the leasing server stopped, want 0")
	}
	if dnsmasqEvents(log, dnsmasqRequest, dhcpLeasedAddress) == 0 {
		return false, errors.New("the second server logged no DHCPREQUEST for " + dhcpLeasedAddress)
	}
	if dnsmasqEvents(log, dnsmasqAck, dhcpLeasedAddress) == 0 {
		return false, errors.New("the second server logged no DHCPACK for " + dhcpLeasedAddress)
	}
	return true, nil
}

func dhcpCountError(prefix string, count int, suffix string) error {
	var text textbuf.Buffer
	return errors.New(text.Str(prefix).Byte(' ').Int(int64(count)).Byte(' ').Str(suffix).String())
}
