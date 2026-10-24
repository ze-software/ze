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
	ldpPort     = "646"
	ldpLabTable = "ldplab"
	ldpLabChain = "output"
	// ldpLabChainSpec hooks the chain ahead of every other output filter.
	ldpLabChainSpec = "{ type filter hook output priority -300; policy accept; }"
	// ldpLabZeroWindow rewrites the receive window of every segment ze sends on
	// the LDP session (ze is the active LSR, so it dials FRR's port 646) to
	// zero. nft fixes the TCP checksum for the rewritten field; nft joins its
	// arguments into one rule.
	ldpLabZeroWindow    = "tcp dport " + ldpPort + " tcp window set 0"
	ldpStateOperational = "OPERATIONAL"
	// ldpExpiryStatus is the Status name FRR logs for Status Code 0x00000014.
	ldpExpiryStatus = "KeepAlive Timer Expired"
	// ldpReceivedNotification is the prefix FRR's message debug gives a received
	// Notification.
	ldpReceivedNotification = "msg[in]: notification:"
	// ldpExpiryWait covers ze's 3 s KeepAlive Timer, the 5 s write bound and
	// FRR's handling of the Notification.
	ldpExpiryWait = 30 * time.Second
)

var frrShowLDPNeighbor = []string{cmdVtysh, "-c", "show mpls ldp neighbor"}

// checkLDPKeepaliveExpiry brings the LDP session up, then closes ze's TCP
// receive window toward FRR, so FRR can send ze no LDP PDU and ze's KeepAlive
// Timer is the one that expires. It requires FRR to log a received
// Notification naming KeepAlive Timer Expired after the break, then to leave
// OPERATIONAL.
//
// Why a zero window and not a drop: a drop of FRR's segments toward ze also
// drops the ACKs FRR piggybacks on its KeepAlives, so ze's own stream stalls in
// retransmission, ze's Notification never reaches FRR, and FRR's timer fires
// first (spec-ldp-keepalive-expiry-notification A-4, seen 2026-10-20).
// A zero window advertised by ze leaves FRR's kernel holding its LDP data while
// it still acknowledges ze's, with pure ACKs and zero-window probes, so ze's
// KeepAlives keep FRR's timer from expiring and ze's Notification is delivered.
// FRR's KeepAlive Time cannot be raised instead: RFC 5036 Section 3.5.3 has both
// sides use the smaller proposal. Hellos are UDP and untouched, so the adjacency
// stays and the close is the timer's, not the adjacency expiry's.
func checkLDPKeepaliveExpiry(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the LDP KeepAlive expiry scenario has no selected IPv4 network")
	}
	frrAddress := networkHostAddress(check.Network, 3)
	zeAddress := networkHostAddress(check.Network, 2)
	if _, err := check.Lab.Exec(ctx, "ze", ldpTransportAddressAdd(check.Network), nil); err != nil {
		return fmt.Errorf("add ze's LDP transport address: %w", err)
	}
	if err := waitContains(ctx, check.Lab, peerFRR, frrShowLDPNeighbor, 120*time.Second, zeAddress, ldpStateOperational); err != nil {
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

	// The break: ze's advertised receive window, rewritten to zero in a table of
	// ze's own container. The FRR image ships no iptables, and the table is
	// outside the ze_ prefix, so ze's firewall never sweeps it.
	for _, argv := range [][]string{
		{cmdNft, nftActionAdd, nftObjectTable, nftFamilyInet, ldpLabTable},
		{cmdNft, nftActionAdd, nftObjectChain, nftFamilyInet, ldpLabTable, ldpLabChain, ldpLabChainSpec},
		{cmdNft, nftActionAdd, nftObjectRule, nftFamilyInet, ldpLabTable, ldpLabChain, "ip", "daddr", frrAddress, ldpLabZeroWindow},
	} {
		if _, err := check.Lab.Exec(ctx, "ze", argv, nil); err != nil {
			return fmt.Errorf("close ze's LDP receive window: %w", err)
		}
	}
	defer func() {
		restore := []string{cmdNft, nftActionDelete, nftObjectTable, nftFamilyInet, ldpLabTable}
		_, restoreErr := check.Lab.Exec(context.WithoutCancel(ctx), "ze", restore, nil)
		if restoreErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("reopen ze's LDP receive window: %w", restoreErr)
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
// message as "msg[in]: notification: lsr-id <id>, status <name> (fatal error)",
// so a line must carry that received-Notification prefix: a line about FRR's
// own timer (nbr_ktimeout), or a Notification FRR sent, is not counted.
func expiryNotificationsLogged(frrLog string) int {
	count := 0
	for line := range strings.SplitSeq(frrLog, "\n") {
		if !strings.Contains(line, ldpReceivedNotification) {
			continue
		}
		if strings.Contains(line, ldpExpiryStatus) {
			count++
		}
	}
	return count
}
