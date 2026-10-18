// Design: docs/architecture/testing/interop.md -- ze's DHCPv4 client against dnsmasq.
// Related: check_dhcp_predicate.go -- the pure readings of dnsmasq's log.
// Related: register_dhcp.go -- the registration of both checkers.
package bgp

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioDHCPRenewNak = "dhcpv4-renew-nak-dnsmasq"
	scenarioDHCPRebind   = "dhcpv4-rebind-two-servers-dnsmasq"

	// dnsmasqLogLines bounds each log read. A run of a few minutes at T1 20 s
	// logs a few dozen DHCP transactions of about a dozen lines each under
	// log-dhcp, far below this.
	dnsmasqLogLines = 4000
	// dhcpBindWait covers ze's start and its DORA; dhcpRenewWait seven T1
	// periods of 20 s (dhcpRenewalsWanted); dhcpNakWait one T1 period and a
	// DORA; dhcpRebindWait T2 (40 s) after the last ACK.
	dhcpBindWait   = 60 * time.Second
	dhcpRenewWait  = 200 * time.Second
	dhcpNakWait    = 90 * time.Second
	dhcpRebindWait = 90 * time.Second
	dhcpPoll       = 2 * time.Second
	// dhcpLeaseTime is the lease both scenario configs grant (dhcp-range, 120 s),
	// and dhcpExpiryMargin how long past its end the rebind checker looks.
	dhcpLeaseTime    = 120 * time.Second
	dhcpExpiryMargin = 10 * time.Second
)

// dhcpReservationCommand writes a reservation of dhcpReservedAddress for every
// hardware address into the file dnsmasq reads as dhcp-hostsfile
// (test/interop/Dockerfile.dnsmasq). The pool's one client is ze.
var dhcpReservationCommand = []string{"/bin/sh", "-c", "echo '*:*:*:*:*:*," + dhcpReservedAddress + "' > /run/dhcp-hosts"}

// checkDHCPRenewNak proves AC-24 against dnsmasq. ze binds the one-address
// pool, then holds it across seven RENEW/ACK exchanges, more than one lease,
// with no second DHCPDISCOVER. The checker then reserves another address for ze and has
// dnsmasq reload: dnsmasq NAKs the next RENEW, and ze tears the lease down,
// restarts with a DHCPDISCOVER and ends holding only the reserved address.
func checkDHCPRenewNak(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDHCPRenewNak, assertion, cause)
	}
	// Assertion 1. ze installed the leased address.
	if err := waitZeAddress(ctx, check, dhcpLeasedAddress); err != nil {
		return fail(1, err)
	}
	// Assertion 2. Seven renewals, one DHCPDISCOVER.
	if err := waitDnsmasq(ctx, check.Lab, peerDnsmasq, dhcpRenewWait, dhcpRenewalVerdict); err != nil {
		return fail(2, err)
	}
	// Assertion 3. The reservation moves, and dnsmasq re-reads it on SIGHUP.
	if _, err := check.Lab.Exec(ctx, peerDnsmasq, dhcpReservationCommand, nil); err != nil {
		return fail(3, err)
	}
	if err := check.Lab.Signal(ctx, peerDnsmasq, "HUP"); err != nil {
		return fail(3, err)
	}
	// Assertion 4. NAK, a second DHCPDISCOVER, an ACK of the reserved address.
	nak := func(log string) (bool, error) {
		err := dhcpNakVerdict(log)
		return err == nil, err
	}
	if err := waitDnsmasq(ctx, check.Lab, peerDnsmasq, dhcpNakWait, nak); err != nil {
		return fail(4, err)
	}
	// Assertion 5. ze holds the reserved address and no longer the old one.
	if err := waitZeAddress(ctx, check, dhcpReservedAddress); err != nil {
		return fail(5, err)
	}
	absent := operation{kind: opRequireAbsent, peer: "ze", command: zeAddressCommand(),
		absent: []string{dhcpLeasedAddress + "/"}, proof: []string{dhcpReservedAddress + "/"}}
	if err := runOperation(ctx, check.Network, check.Lab, &absent); err != nil {
		return fail(5, err)
	}
	return nil
}

// checkDHCPRebind proves AC-25 against two dnsmasq servers holding the same
// one-address pool. Once ze is bound, the checker stops the server whose log
// holds the lease ACK. ze's RENEW then goes unanswered, and at T2 its
// broadcast REBIND is acknowledged by the other server, and past the end of
// the first lease ze still holds the address and has sent no DHCPDISCOVER.
func checkDHCPRebind(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDHCPRebind, assertion, cause)
	}
	// Assertion 1. ze installed the leased address. The lease began no later
	// than the moment ze is seen holding it.
	if err := waitZeAddress(ctx, check, dhcpLeasedAddress); err != nil {
		return fail(1, err)
	}
	bound := time.Now()
	// Assertion 2. Exactly one server granted the lease: stop it.
	holder, other, err := dhcpLeaseHolder(ctx, check.Lab)
	if err != nil {
		return fail(2, err)
	}
	before, err := check.Lab.Logs(ctx, other, dnsmasqLogLines)
	if err != nil {
		return fail(2, err)
	}
	if !before.Available {
		return fail(2, errors.New("the second server's log was not read"))
	}
	if err := check.Lab.Stop(ctx, holder, 5); err != nil {
		return fail(2, err)
	}
	// Assertion 3. The other server acknowledges the REBIND, no DHCPDISCOVER.
	discovers := dnsmasqEvents(before.Text, dnsmasqDiscover, "")
	rebind := func(log string) (bool, error) {
		return dhcpRebindVerdict(log, discovers)
	}
	if err := waitDnsmasq(ctx, check.Lab, other, dhcpRebindWait, rebind); err != nil {
		return fail(3, err)
	}
	// Assertion 4. ze still holds the address once the lease the stopped
	// server granted has run out, and has sent no DHCPDISCOVER. The other
	// server's ACK alone proves nothing about ze: a client that ignored it
	// would keep the address until that expiry, then start over.
	held := operation{kind: opDelayRequireContains, peer: "ze", command: zeAddressCommand(),
		contains: []string{dhcpLeasedAddress + "/24"}, delay: max(time.Until(bound.Add(dhcpLeaseTime+dhcpExpiryMargin)), time.Second)}
	if err := runOperation(ctx, check.Network, check.Lab, &held); err != nil {
		return fail(4, err)
	}
	after, err := check.Lab.Logs(ctx, other, dnsmasqLogLines)
	if err != nil {
		return fail(4, err)
	}
	if !after.Available {
		return fail(4, errors.New("the second server's log was not read"))
	}
	if _, err := dhcpRebindVerdict(after.Text, discovers); err != nil {
		return fail(4, err)
	}
	return nil
}

// dhcpLeaseHolder answers which dnsmasq granted ze's lease: the one whose log
// holds a DHCPACK of the leased address. Both, or neither, is an error.
func dhcpLeaseHolder(ctx context.Context, lab interoplab.CheckerLab) (holder, other string, err error) {
	first, err := lab.Logs(ctx, peerDnsmasq, dnsmasqLogLines)
	if err != nil {
		return "", "", err
	}
	second, err := lab.Logs(ctx, peerDnsmasqB, dnsmasqLogLines)
	if err != nil {
		return "", "", err
	}
	if !first.Available {
		return "", "", errors.New("server A's log was not read")
	}
	if !second.Available {
		return "", "", errors.New("server B's log was not read")
	}
	firstAcked := dnsmasqEvents(first.Text, dnsmasqAck, dhcpLeasedAddress) > 0
	secondAcked := dnsmasqEvents(second.Text, dnsmasqAck, dhcpLeasedAddress) > 0
	if firstAcked == secondAcked {
		return "", "", errors.New("want exactly one server to have acknowledged " + dhcpLeasedAddress)
	}
	if firstAcked {
		return peerDnsmasq, peerDnsmasqB, nil
	}
	return peerDnsmasqB, peerDnsmasq, nil
}

// waitDnsmasq reads a dnsmasq log until verdict is done, and returns its
// error. A verdict still not done at timeout returns its last error.
func waitDnsmasq(ctx context.Context, lab interoplab.CheckerLab, peer string, timeout time.Duration, verdict func(string) (bool, error)) error {
	var last error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: dhcpPoll, Description: peer + " DHCP log evidence",
	}, func(probeCtx context.Context) (string, error) {
		logs, logErr := lab.Logs(probeCtx, peer, dnsmasqLogLines)
		if logErr != nil {
			return "", logErr
		}
		if !logs.Available {
			return "", errors.New("dnsmasq logs were not read")
		}
		return logs.Text, nil
	}, func(log string) bool {
		var done bool
		done, last = verdict(log)
		return done
	})
	if last != nil {
		return last
	}
	return err
}

// waitZeAddress waits for address, with its /24, on ze's eth0.
func waitZeAddress(ctx context.Context, check *interoplab.CheckContext, address string) error {
	present := operation{kind: opWaitContains, peer: "ze", command: zeAddressCommand(),
		contains: []string{address + "/24"}, timeout: dhcpBindWait}
	return runOperation(ctx, check.Network, check.Lab, &present)
}

func zeAddressCommand() []string {
	return []string{"ip", "-o", "-f", ipFamilyInet, ipObjectAddr, "show", "dev", containerInterface}
}
