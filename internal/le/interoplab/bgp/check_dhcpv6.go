// Design: docs/architecture/testing/interop.md -- ze's DHCPv6 client against Kea.
// Related: check_dhcpv6_predicate.go -- the pure readings of Kea's log and ze's eth0.
// Related: register_dhcp.go -- the registration of the checker.
package bgp

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioDHCPv6RenewKea = "dhcpv6-client-renew-kea"

	// keaLogLines bounds each log read. At INFO Kea logs about five lines per
	// exchange, and a run of a few minutes at T1 10 s holds a few dozen
	// exchanges plus two start-ups, far below this.
	keaLogLines = 4000
	// kea6BindWait covers ze's start, the first Solicit's random delay and
	// the Solicit and Request exchanges; kea6Sample is how often the hold
	// phases read ze's eth0.
	kea6BindWait = 60 * time.Second
	kea6Sample   = 2 * time.Second
	// kea6StopSeconds is the grace docker gives Kea before SIGKILL.
	kea6StopSeconds = 5
)

// checkDHCPv6RenewKea proves the spec's lease-maintenance interop against
// Kea. ze binds the pool's one address, then holds it for three lease
// periods, every sample a /128 with a finite lifetime no longer than the
// lease, while Kea logs one Solicit, one Request and a RENEW exchange per T1,
// each extending the lease. The checker then restarts Kea, which comes back
// under a new server DUID with the lease still in its database: ze's next
// RENEW names a server that is gone, its REBIND is answered, and ze keeps the
// address past the end of the lease granted before the restart, then renews
// with the new server.
func checkDHCPv6RenewKea(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDHCPv6RenewKea, assertion, cause)
	}
	address, err := kea6LeasedAddress(check.Network)
	if err != nil {
		return fail(1, err)
	}
	// Assertion 1. ze installed the leased address.
	if err := waitZeV6Lease(ctx, check.Lab, address, kea6ValidLifetime); err != nil {
		return fail(1, err)
	}
	// Assertion 2. ze holds it, refreshed, for three lease periods.
	holdEnd := time.Now().Add(kea6HoldPeriods*kea6ValidLifetime + kea6HoldMargin)
	if err := holdZeV6Lease(ctx, check.Lab, address, kea6ValidLifetime, holdEnd); err != nil {
		return fail(2, err)
	}
	// Assertion 3. Kea saw one Solicit and Request, then a Renew per T1.
	if err := readKea(ctx, check.Lab, func(log string) error { return kea6RenewalVerdict(log, address) }); err != nil {
		return fail(3, err)
	}
	// Assertion 4. Kea restarts under a new server DUID.
	stopped := time.Now()
	if err := check.Lab.Stop(ctx, peerKea, kea6StopSeconds); err != nil {
		return fail(4, err)
	}
	if err := check.Lab.Start(ctx, peerKea); err != nil {
		return fail(4, err)
	}
	// Assertion 5. ze keeps the address for two lease periods after the
	// stop: the lease granted before it runs out within the first, so only
	// an answered REBIND keeps the address past it.
	if err := holdZeV6Lease(ctx, check.Lab, address, kea6ValidLifetime, stopped.Add(2*kea6ValidLifetime)); err != nil {
		return fail(5, err)
	}
	// Assertion 6. A REBIND, not a SOLICIT, kept it, and ze then renewed
	// with the server that answered.
	rebound := func(log string) error {
		done, verdict := kea6RebindVerdict(log, address)
		if verdict != nil {
			return verdict
		}
		if !done {
			return errors.New("the rebind phase is not complete in Kea's log")
		}
		return kea6RenewAfterRebindVerdict(log, address)
	}
	if err := readKea(ctx, check.Lab, rebound); err != nil {
		return fail(6, err)
	}
	return nil
}

// kea6LeasedAddress is the pool's one address on the lab's IPv6 /64: host
// keaLeasedHostNumber, as kea-dhcp6.conf renders it.
func kea6LeasedAddress(network interoplab.Network) (string, error) {
	if !network.IPv6.IsValid() {
		return "", errors.New("the DHCPv6 scenario ran without an IPv6 lab network; kea-dhcp6.conf names fd00:1e:0:: to select one")
	}
	octets := network.IPv6.Masked().Addr().As16()
	octets[15] = keaLeasedHostNumber
	return netip.AddrFrom16(octets).String(), nil
}

// readKea reads Kea's log once and returns verdict's answer.
func readKea(ctx context.Context, lab interoplab.CheckerLab, verdict func(string) error) error {
	logs, err := lab.Logs(ctx, peerKea, keaLogLines)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("the Kea log was not read")
	}
	return verdict(logs.Text)
}

// waitZeV6Lease waits for ze's eth0 to hold address as a /128 with a lease
// lifetime of at most lifetime.
func waitZeV6Lease(ctx context.Context, lab interoplab.CheckerLab, address string, lifetime time.Duration) error {
	return waitZeV6(ctx, lab, kea6BindWait, func(output string) error {
		return zeV6LeaseVerdict(output, address, lifetime)
	})
}

// waitZeV6 reads ze's eth0 every kea6Sample until verdict accepts a reading,
// and returns the last reading's error at timeout.
func waitZeV6(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration, verdict func(string) error) error {
	var last error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: timeout, Interval: kea6Sample, Description: "ze DHCPv6 addresses on eth0",
	}, func(probeCtx context.Context) (string, error) {
		return zeV6Addresses(probeCtx, lab)
	}, func(output string) bool {
		last = verdict(output)
		return last == nil
	})
	if last != nil {
		return last
	}
	return err
}

// holdZeV6Lease reads ze's eth0 every kea6Sample until end, and fails on the
// first reading that does not hold address with a lease lifetime of at most
// lifetime. A sample that fails to run is an error: a gap in the readings is
// not a hold.
func holdZeV6Lease(ctx context.Context, lab interoplab.CheckerLab, address string, lifetime time.Duration, end time.Time) error {
	// The bound: end, a fixed instant; each pass waits kea6Sample.
	for {
		output, err := zeV6Addresses(ctx, lab)
		if err != nil {
			return err
		}
		if err := zeV6LeaseVerdict(output, address, lifetime); err != nil {
			return err
		}
		remaining := time.Until(end)
		if remaining <= 0 {
			return nil
		}
		timer := time.NewTimer(min(remaining, kea6Sample))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func zeV6Addresses(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	result, err := lab.Exec(ctx, "ze", []string{"ip", "-o", "-6", ipObjectAddr, ipActionShow, ipOptionDevice, containerInterface}, nil)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}
