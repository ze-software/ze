// Design: docs/architecture/testing/interop.md -- concurrent state validation.
// Related: check_ldp_keepalive_expiry.go -- the LDP scenario whose transport address, nft table and FRR log reading this one shares.
// Related: check_special.go -- specialCheckers registry entry.
// RFC: rfc/short/rfc5036.md
package bgp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// ldpInitUnknownTLVScenario proves RFC 5036 Section 3.3 on ze's receive side
// against FRR: "Upon receipt of an unknown TLV, if U is clear (=0), a
// notification MUST be returned to the message originator and the entire
// message MUST be ignored".
const ldpInitUnknownTLVScenario = "ldp-init-unknown-tlv-frr"

const (
	// ldpInitChain hooks ze's container's prerouting ahead of conntrack and
	// every other filter, so the rewrite happens before ze's socket reads.
	ldpInitChain     = "prerouting"
	ldpInitChainSpec = "{ type filter hook prerouting priority -300; policy accept; }"
	// ldpInitRewrite matches the first data segment FRR sends on the session
	// and rewrites the type of the first Capability TLV in FRR's
	// Initialization. ldpd's send_init writes the LDP PDU header (10 octets),
	// the message header (8), the Common Session Parameters TLV (18), then the
	// Dynamic Capability Announcement TLV, type 0x8506 with the U bit set
	// (RFC 5561 Section 9). With the 32-octet TCP header a Linux peer sends
	// when timestamps are on (tcp doff 8), the message type sits at bit 336 of
	// the transport header and that TLV's type at bit 544. The rewrite writes
	// 0x3f00: the U bit and the F bit clear and a type no RFC registers, the
	// type TestLDPInitUnknownTLVUClearNotifies uses. The TLV keeps its length
	// and its value, so the message differs from FRR's only by that type.
	// nft recomputes the TCP checksum over the rewritten field. The counter
	// is the proof the rewrite ran, read before any verdict on ze.
	ldpInitRewrite = "tcp sport " + ldpPort + " tcp doff 8 @th,336,16 0x0200 @th,544,16 0x8506 counter @th,544,16 set 0x3f00"
	// ldpUnknownTLVStatus is the Status name FRR logs for Status Code
	// 0x00000006 (ldpd logmsg.c, status_code_name).
	ldpUnknownTLVStatus = "status Unknown TLV"
	// ldpInitIgnoredWindow is how long FRR must stay short of OPERATIONAL
	// after ze's Notification. ze answering the Initialization with a
	// KeepAlive would make FRR OPERATIONAL within one round trip.
	ldpInitIgnoredWindow = 10 * time.Second
	// ldpInitWait covers discovery, ze's dial and FRR's handling of the
	// Notification.
	ldpInitWait = 120 * time.Second
	// ldpTransportHost is the host part of ze's transport address, 250 in
	// the scenario config's 172.30.0.250.
	ldpTransportHost = 250
)

// ldpTransportAddressAdd puts ze's LDP transport address, host .250 of the
// selected lab subnet, on eth0. Until it exists ze has no address to dial
// from, so a checker adds it once the lab state the session must meet is in
// place. The subnet is the one the run selected: a concurrent run moves the
// lab off 172.30.0.0/24, and the scenario config is rewritten to match.
func ldpTransportAddressAdd(network interoplab.Network) []string {
	return []string{"ip", "address", "add", networkHostAddress(network, ldpTransportHost) + "/32", "dev", "eth0"}
}

// checkLDPInitUnknownTLV rewrites one TLV type in FRR's real Initialization on
// its way into ze's container, then requires what Section 3.3 asks of ze:
// FRR, the message originator, logs a received Unknown TLV Notification, and
// the session does not become OPERATIONAL, because ze ignored the whole
// Initialization and so never answered it with a KeepAlive. Removing the
// rewrite is the positive control: the same peers then reach OPERATIONAL, so
// the refusal was the rewritten TLV's and not a broken lab's.
//
// FRR cannot be told to send an unknown TLV, so FRR proves the receiving side
// of ze's answer: that the Notification reached the originator and decodes
// as Unknown TLV. The bytes ze judged are FRR's own, one field changed.
func checkLDPInitUnknownTLV(ctx context.Context, check *interoplab.CheckContext) (resultErr error) {
	if !check.Network.IPv4.IsValid() {
		return errors.New("the LDP unknown-TLV scenario has no selected IPv4 network")
	}
	frrAddress := networkHostAddress(check.Network, 3)
	zeAddress := networkHostAddress(check.Network, 2)
	frrLog := []string{cmdCat, frrLogPath}
	// FRR creates its log file some time after the container is ready, so
	// the baseline waits for a readable log rather than reading absence.
	logBefore, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpInitWait,
		Interval:    time.Second,
		Description: "FRR's log becoming readable",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrLog, nil)
	}, func(string) bool { return true })
	if err != nil {
		return fmt.Errorf("read FRR's log before the session: %w", err)
	}
	before := unknownTLVNotificationsLogged(logBefore)

	// The rewrite goes in before ze's transport address exists, so the first
	// session ze dials already meets it.
	for _, argv := range [][]string{
		{cmdNft, nftActionAdd, nftObjectTable, nftFamilyInet, ldpLabTable},
		{cmdNft, nftActionAdd, nftObjectChain, nftFamilyInet, ldpLabTable, ldpInitChain, ldpInitChainSpec},
		{cmdNft, nftActionAdd, nftObjectRule, nftFamilyInet, ldpLabTable, ldpInitChain, "ip", "saddr", frrAddress, ldpInitRewrite},
	} {
		if _, err := check.Lab.Exec(ctx, "ze", argv, nil); err != nil {
			return fmt.Errorf("install the Initialization rewrite: %w", err)
		}
	}
	rewriteInstalled := true
	defer func() {
		if !rewriteInstalled {
			return
		}
		remove := []string{cmdNft, nftActionDelete, nftObjectTable, nftFamilyInet, ldpLabTable}
		if _, removeErr := check.Lab.Exec(context.WithoutCancel(ctx), "ze", remove, nil); removeErr != nil && resultErr == nil {
			resultErr = fmt.Errorf("remove the Initialization rewrite: %w", removeErr)
		}
	}()

	if _, err := check.Lab.Exec(ctx, "ze", ldpTransportAddressAdd(check.Network), nil); err != nil {
		return fmt.Errorf("add ze's LDP transport address: %w", err)
	}

	listRewrite := []string{cmdNft, nftActionList, nftObjectChain, nftFamilyInet, ldpLabTable, ldpInitChain}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpInitWait,
		Interval:    time.Second,
		Description: "the rewrite matching FRR's Initialization",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, "ze", listRewrite, nil)
	}, func(output string) bool {
		packets, counted := rewriteMatched(output)
		return counted && packets > 0
	})
	if err != nil {
		return fmt.Errorf("the rewrite never matched FRR's Initialization, so ze was never shown an unknown TLV: %w", err)
	}

	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     ldpInitWait,
		Interval:    500 * time.Millisecond,
		Description: "FRR receiving ze's Unknown TLV Notification",
	}, func(probeCtx context.Context) (string, error) {
		return check.Lab.Query(probeCtx, peerFRR, frrLog, nil)
	}, func(output string) bool {
		return unknownTLVNotificationsLogged(output) > before
	})
	if err != nil {
		return fmt.Errorf("FRR logged no received %q Notification for its rewritten Initialization: %w", ldpUnknownTLVStatus, err)
	}

	// The whole message ignored: ze sent no KeepAlive for it, so FRR stays
	// short of OPERATIONAL for the window.
	deadline := time.Now().Add(ldpInitIgnoredWindow)
	for time.Now().Before(deadline) {
		output, err := check.Lab.Query(ctx, peerFRR, frrShowLDPNeighbor, nil)
		if err != nil {
			return fmt.Errorf("read FRR's LDP neighbors while the rewrite runs: %w", err)
		}
		if strings.Contains(output, ldpStateOperational) {
			return fmt.Errorf("FRR reached OPERATIONAL with ze while every Initialization carried an unknown TLV with the U bit clear, so ze processed a message RFC 5036 Section 3.3 says to ignore: %s", output)
		}
		time.Sleep(time.Second)
	}

	// The positive control: the same peers, FRR's Initialization untouched.
	remove := []string{cmdNft, nftActionDelete, nftObjectTable, nftFamilyInet, ldpLabTable}
	if _, err := check.Lab.Exec(ctx, "ze", remove, nil); err != nil {
		return fmt.Errorf("remove the Initialization rewrite: %w", err)
	}
	rewriteInstalled = false
	if err := waitContains(ctx, check.Lab, peerFRR, frrShowLDPNeighbor, ldpInitWait, zeAddress, ldpStateOperational); err != nil {
		return fmt.Errorf("with FRR's Initialization untouched the session never became operational, so the refusal above proves nothing about the TLV: %w", err)
	}
	return nil
}

// unknownTLVNotificationsLogged counts the received Notifications naming
// Unknown TLV in FRR's log. `debug mpls ldp messages recv` logs each received
// message as "msg[in]: notification: lsr-id <id>, status <name>", so a line
// must carry the received-Notification prefix: a Notification FRR sent is not
// counted.
func unknownTLVNotificationsLogged(frrLog string) int {
	count := 0
	for line := range strings.SplitSeq(frrLog, "\n") {
		if !strings.Contains(line, ldpReceivedNotification) {
			continue
		}
		if strings.Contains(line, ldpUnknownTLVStatus) {
			count++
		}
	}
	return count
}

// rewriteMatched reads the packet count of the rewrite rule's counter out of
// `nft list chain`. A listing with no readable counter answers false, so the
// caller's wait keeps polling rather than reading it as a count.
func rewriteMatched(listing string) (int, bool) {
	_, rest, found := strings.Cut(listing, "counter packets ")
	if !found {
		return 0, false
	}
	digits, _, _ := strings.Cut(rest, " ")
	packets, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return packets, true
}
