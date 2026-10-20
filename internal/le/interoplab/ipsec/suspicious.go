// Design: docs/architecture/testing/interop.md -- RFC 7296 Section 2.21.4 notice over an existing IKE SA, against strongSwan.
// Related: checkers.go -- the scenario table that names checkOutOfSARequestNotifiedOverSA.
package ipsec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The two header-only IKE datagrams strongSwan injects toward Ze, from its own address
// and IKE port, under an SPI pair no SA holds. RFC 7296 Section 3.1 layout: initiator
// SPI, responder SPI, Next Payload 0, version 2.0 (0x20), Exchange Type INFORMATIONAL
// (37, 0x25), Flags, Message ID 7, Length 28.
//
// The first carries the Initiator flag alone, so it is a request. The second also sets
// the Response flag (0x20), which Section 2.21.4 forbids answering.
const (
	outOfSARequestHex  = "0xdeadbeef01020304cafef00d0506070800202508000000070000001c"
	outOfSAResponseHex = "0xdeadbeef01020304cafef00d0506070800202528000000070000001c"
)

// outOfSANoticeSettle is how long the checker waits to see that a datagram Ze must not
// act on drew nothing. Ze's owner loop answers within one scheduling round, and the
// positive leg below shows the notice arriving well inside this window.
const outOfSANoticeSettle = 5 * time.Second

// checkOutOfSARequestNotifiedOverSA proves RFC 7296 Section 2.21.4 against strongSwan:
// "A node receiving a suspicious message from an IP address (and port, if NAT traversal
// is used) with which it has an IKE SA SHOULD send an IKE Notify payload in an IKE
// INFORMATIONAL exchange over that SA."
//
// Method: establish the PSK tunnel, then have strongSwan send, from its own address and
// port 500, an IKE datagram naming an IKE SA Ze does not hold. charon decrypts what Ze
// sends over the live SA and logs the payloads it parsed, so a "parsed INFORMATIONAL
// request" line carrying N(INVAL_IKE_SPI) is another implementation confirming that the
// notice arrived protected under the SA's keys. A response-marked copy is sent first,
// and must draw no such line. The SA and its tunnel must survive both.
func checkOutOfSARequestNotifiedOverSA(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	before, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	baseline := countProtectedInvalidIKESPI(before)

	// Control: a datagram marked as a response is audited and never answered.
	if err := lab.injectOutOfSA(ctx, outOfSAResponseHex); err != nil {
		return err
	}
	if err := waitDuration(ctx, outOfSANoticeSettle); err != nil {
		return err
	}
	after, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if countProtectedInvalidIKESPI(after) != baseline {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("ze sent INVALID_IKE_SPI over the SA for an out-of-SA message marked as a response"), after)
	}

	if err := lab.injectOutOfSA(ctx, outOfSARequestHex); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 15 * time.Second, Interval: time.Second,
		Description: "strongSwan parsing a protected INFORMATIONAL request carrying INVALID_IKE_SPI",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(current string) bool {
		return countProtectedInvalidIKESPI(current) > baseline
	})
	if err != nil {
		return fmt.Errorf("ze did not notify the peer over its IKE SA of a request naming an unknown IKE SA: %w", err)
	}
	if err := lab.waitSA(ctx, lab.timeout); err != nil {
		return fmt.Errorf("the IKE SA did not survive the notice: %w", err)
	}
	return lab.verifyTunnelTraffic(ctx, "no traffic after the out-of-SA notice")
}

// countProtectedInvalidIKESPI counts the INFORMATIONAL requests charon parsed that
// carried INVALID_IKE_SPI. charon logs a request only after it decrypted and verified
// it under an IKE SA, so an unprotected INVALID_IKE_SPI response can never match.
func countProtectedInvalidIKESPI(logs string) int {
	count := 0
	for line := range strings.SplitSeq(logs, "\n") {
		if !strings.Contains(line, "parsed INFORMATIONAL request") {
			continue
		}
		if strings.Contains(line, "N(INVAL_IKE_SPI)") {
			count++
		}
	}
	return count
}

// injectOutOfSA sends one raw IKE datagram from strongSwan's address and port 500 to
// Ze's port 500. --send-ip keeps nping on the IP layer, as craftedProbe explains.
func (l *scenarioLab) injectOutOfSA(ctx context.Context, payloadHex string) error {
	output, err := l.exec(ctx, swanPeer, "nping", "--send-ip", "--no-capture", "--count", "1",
		"--udp", "--source-port", "500", "--dest-port", "500", "--data", payloadHex, zeIP)
	if err != nil {
		return fmt.Errorf("nping out-of-SA IKE datagram to %s on strongSwan: %w: %s", zeIP, err, output)
	}
	return nil
}
