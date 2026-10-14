// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: check_special.go -- specialCheckers registry entry, BFD failover nft precedent.
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ldpKeepaliveExpiryScenario proves that ze tells its LDP peer why a session
// ends when ze's KeepAlive Timer expires (RFC 5036 Sections 2.5.4 and
// 3.5.1.2.3): FRR must receive the KeepAlive Timer Expired Notification.
const ldpKeepaliveExpiryScenario = "ldp-keepalive-expiry-frr"

const (
	// ldpTransportAddress is ze's LDP transport address in the scenario. It is
	// higher than FRR's, so ze is the active LSR, the only role ze plays.
	ldpTransportAddress = "172.30.0.250"
	ldpPort             = "646"
	ldpLabTable         = "ldplab"
	ldpLabChain         = "input"
	// ldpLabChainSpec hooks the chain ahead of every other input filter.
	ldpLabChainSpec     = "{ type filter hook input priority -300; policy accept; }"
	ldpStateOperational = "OPERATIONAL"
	// ldpExpiryStatus is the Status name FRR logs for Status Code 0x00000014.
	ldpExpiryStatus = "KeepAlive Timer Expired"
	// ldpExpiryWait covers ze's 3 s KeepAlive Timer, the 5 s write bound and
	// FRR's handling of the Notification.
	ldpExpiryWait = 30 * time.Second
)

var frrShowLDPNeighbor = []string{cmdVtysh, "-c", "show mpls ldp neighbor"}

// checkLDPKeepaliveExpiry brings the LDP session up, drops FRR's TCP segments in
// ze's container so ze's KeepAlive Timer is the one that expires, and requires
// FRR to log a received Notification naming KeepAlive Timer Expired after the
// break, then to leave OPERATIONAL. Hellos are UDP and still pass, so the
// adjacency stays and the close is the timer's, not the adjacency expiry's.
func checkLDPKeepaliveExpiry(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	address := []string{"ip", "address", "add", ldpTransportAddress + "/32", "dev", "eth0"}
	if _, err := check.Lab.Exec(ctx, "ze", address, nil); err != nil {
		return fmt.Errorf("add ze's LDP transport address: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRR, frrShowLDPNeighbor, 120*time.Second, zeLabAddress, ldpStateOperational); err != nil {
		return fmt.Errorf("LDP session with ze never became operational: %w", err)
	}

	frrLog := []string{cmdCat, frrLogPath}
	logBefore, err := check.Lab.Query(ctx, peerFRR, frrLog, nil)
	if err != nil {
		return fmt.Errorf("read FRR's log before the break: %w", err)
	}
	if logBefore == "" {
		return errors.New("FRR's log is empty, so a received Notification could not be read")
	}
	before := expiryNotificationsLogged(logBefore)

	// The break: FRR's TCP segments toward ze, dropped in a table of ze's own
	// container. The FRR image ships no iptables, and the table is outside the
	// ze_ prefix, so ze's firewall never sweeps it.
	for _, argv := range [][]string{
		{cmdNft, nftActionAdd, nftObjectTable, nftFamilyInet, ldpLabTable},
		{cmdNft, nftActionAdd, nftObjectChain, nftFamilyInet, ldpLabTable, ldpLabChain, ldpLabChainSpec},
		{cmdNft, nftActionAdd, nftObjectRule, nftFamilyInet, ldpLabTable, ldpLabChain, "ip", "saddr", frrLabAddress, "tcp", "sport", ldpPort, "drop"},
	} {
		if _, err := check.Lab.Exec(ctx, "ze", argv, nil); err != nil {
			return fmt.Errorf("drop FRR's LDP TCP traffic: %w", err)
		}
	}
	defer func() {
		restore := []string{cmdNft, nftActionDelete, nftObjectTable, nftFamilyInet, ldpLabTable}
		_, restoreErr := check.Lab.Exec(context.WithoutCancel(ctx), "ze", restore, nil)
		if restoreErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("restore FRR's LDP TCP traffic: %w", restoreErr)
		}
	}()

	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpExpiryWait,
		Interval:    500 * time.Millisecond,
		Description: "FRR receiving ze's KeepAlive Timer Expired Notification",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrLog, nil)
	}, func(output string) bool {
		return expiryNotificationsLogged(output) > before
	})
	if err != nil {
		return fmt.Errorf("FRR logged no received %q Notification after the break: %w", ldpExpiryStatus, err)
	}

	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpExpiryWait,
		Interval:    500 * time.Millisecond,
		Description: "FRR's LDP neighbor leaving OPERATIONAL",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrShowLDPNeighbor, nil)
	}, func(output string) bool {
		return !strings.Contains(output, ldpStateOperational)
	})
	if err != nil {
		return fmt.Errorf("FRR's LDP neighbor stayed OPERATIONAL after the Notification: %w", err)
	}
	return nil
}

// expiryNotificationsLogged counts the received Notifications naming KeepAlive
// Timer Expired in FRR's log. `debug mpls ldp messages recv` logs each received
// message with its Status name; a line must also name a Notification, so a log
// line about FRR's own timer is not counted.
func expiryNotificationsLogged(frrLog string) int {
	count := 0
	for line := range strings.SplitSeq(frrLog, "\n") {
		if !strings.Contains(line, ldpExpiryStatus) {
			continue
		}
		if strings.Contains(strings.ToLower(line), "notification") {
			count++
		}
	}
	return count
}
