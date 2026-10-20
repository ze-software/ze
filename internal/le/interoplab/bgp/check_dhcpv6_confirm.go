// Design: docs/architecture/testing/interop.md -- ze's DHCPv6 Confirm against Kea.
// Related: check_dhcpv6_confirm_predicate.go -- the pure readings of Kea's log and ze's eth0.
// Related: check_dhcpv6.go -- the lease readings shared with the renewal scenario.
// Related: register_dhcp.go -- the registration of the checker.
package bgp

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	scenarioDHCPv6ConfirmKea = "dhcpv6-client-confirm-kea"

	// keaMovedConfigFile is the scenario file holding Kea's configuration
	// after the move; prepare mounts it read-only at keaMovedConfigTarget,
	// and the checker copies it over keaLiveConfig, the file Kea runs on
	// (test/interop/Dockerfile.kea).
	keaMovedConfigFile   = "kea-dhcp6-moved.conf"
	keaMovedConfigTarget = "/etc/kea/kea-dhcp6-moved.conf"
	keaLiveConfig        = "/run/kea/kea-dhcp6.conf"
	// kea6MovedAddress is the one pool address of kea-dhcp6-moved.conf's
	// subnet, which the lab does not render.
	kea6MovedAddress = "fd00:1e:0:1::50"

	// kea6ConfirmLifetime is the valid lifetime both configurations grant.
	kea6ConfirmLifetime = 600 * time.Second
	// kea6ReturnWait bounds ze's re-install of the leased address after the
	// link comes back up; the kernel removes it while the link is down.
	kea6ReturnWait = 10 * time.Second
	// kea6ReplugHold covers CNF_MAX_DELAY (1 s) and a Confirm exchange with
	// its retransmissions many times over.
	kea6ReplugHold = 20 * time.Second
	// kea6ReloadWait bounds Kea's configuration reload after SIGHUP.
	kea6ReloadWait = 30 * time.Second
	// kea6MoveWait covers the 30 s rate limit between two link-change
	// exchanges, CNF_MAX_DELAY, the Confirm, SOL_MAX_DELAY and the Solicit
	// and Request exchanges. A client that keeps the old address waits for
	// T1 (300 s) before Kea can take it back, far past this bound.
	kea6MoveWait = 60 * time.Second
)

// checkDHCPv6ConfirmKea proves the Confirm interop against Kea, in two
// phases. Phase one: ze leases the pool's one address, its eth0 goes down
// and up, and ze sends one CONFIRM that Kea answers Success; ze keeps the
// address on every sample and sends no SOLICIT. Phase two: Kea's
// configuration moves to another subnet and is reloaded, eth0 goes down and
// up again, Kea answers the CONFIRM NotOnLink, and ze removes the old address
// and leases the new subnet's.
func checkDHCPv6ConfirmKea(ctx context.Context, check *interoplab.CheckContext) error {
	fail := func(assertion int, cause error) error {
		return checkerFailure(ctx, check.Lab, scenarioDHCPv6ConfirmKea, assertion, cause)
	}
	address, err := kea6LeasedAddress(check.Network)
	if err != nil {
		return fail(1, err)
	}
	// Assertion 1. ze installed the leased address.
	if err := waitZeV6Lease(ctx, check.Lab, address, kea6ConfirmLifetime); err != nil {
		return fail(1, err)
	}
	// Assertion 2. After a replug on the same link ze keeps the address.
	if err := replugZe(ctx, check.Lab); err != nil {
		return fail(2, err)
	}
	if err := waitZeV6(ctx, check.Lab, kea6ReturnWait, func(output string) error {
		return zeV6LeaseVerdict(output, address, kea6ConfirmLifetime)
	}); err != nil {
		return fail(2, err)
	}
	if err := holdZeV6Lease(ctx, check.Lab, address, kea6ConfirmLifetime, time.Now().Add(kea6ReplugHold)); err != nil {
		return fail(2, err)
	}
	// Assertion 3. Kea answered one CONFIRM Success, and ze sent no SOLICIT.
	if err := readKea(ctx, check.Lab, func(log string) error { return kea6ReplugVerdict(log, address) }); err != nil {
		return fail(3, err)
	}
	// Assertion 4. Kea moves to another subnet on the same link.
	if err := reloadKeaMoved(ctx, check.Lab); err != nil {
		return fail(4, err)
	}
	// Assertion 5. After a replug ze drops the old address and leases one
	// in the new subnet.
	if err := replugZe(ctx, check.Lab); err != nil {
		return fail(5, err)
	}
	if err := waitZeV6(ctx, check.Lab, kea6MoveWait, func(output string) error {
		if err := zeV6AbsentVerdict(output, address); err != nil {
			return err
		}
		return zeV6LeaseVerdict(output, kea6MovedAddress, kea6ConfirmLifetime)
	}); err != nil {
		return fail(5, err)
	}
	// Assertion 6. Kea answered the CONFIRM NotOnLink, and the SOLICIT that
	// leased the new address followed it.
	if err := readKea(ctx, check.Lab, func(log string) error { return kea6MovedVerdict(log, address, kea6MovedAddress) }); err != nil {
		return fail(6, err)
	}
	return nil
}

// linkUp is the ip(8) link state that restores a link, the pair of linkDown.
const linkUp = "up"

// replugZe sets ze's eth0 down, then up: the link event ze answers with its
// move exchange.
func replugZe(ctx context.Context, lab interoplab.CheckerLab) error {
	for _, state := range []string{linkDown, linkUp} {
		if _, err := lab.Exec(ctx, "ze", []string{"ip", ipObjectLink, ipActionSet, ipOptionDevice, containerInterface, state}, nil); err != nil {
			return err
		}
	}
	return nil
}

// reloadKeaMoved copies the moved configuration over the one Kea runs on,
// sends SIGHUP, and waits for Kea to log the reload.
func reloadKeaMoved(ctx context.Context, lab interoplab.CheckerLab) error {
	if _, err := lab.Exec(ctx, peerKea, []string{"cp", keaMovedConfigTarget, keaLiveConfig}, nil); err != nil {
		return err
	}
	if err := lab.Signal(ctx, peerKea, "HUP"); err != nil {
		return err
	}
	var last error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: kea6ReloadWait, Interval: time.Second, Description: "Kea configuration reload",
	}, func(probeCtx context.Context) (string, error) {
		logs, err := lab.Logs(probeCtx, peerKea, keaLogLines)
		if err != nil {
			return "", err
		}
		return logs.Text, nil
	}, func(log string) bool {
		var reloaded bool
		reloaded, last = keaReloadVerdict(log)
		return reloaded || last != nil
	})
	if last != nil {
		return last
	}
	return err
}
