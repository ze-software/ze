// Design: docs/architecture/ike/ipsec-14-responder.md -- unauthenticated input during the responder handshake, against strongSwan.
// Related: checkers.go -- the scenario table that names checkResponderEAPSurvivesUnprotectedAuth.
// Related: suspicious.go -- injectOutOfSA, the raw datagram route this checker reuses.
// RFC: rfc/short/rfc7296.md -- RFC7296-2.4-3, RFC7296-2.4-12.
package ipsec

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// unprotForgeries is how many forged IKE_AUTH requests the checker sends while Ze waits
// mid-EAP. Ze's inbound token bucket admits 200 at once (dispatchInbound), so all of
// them reach the handshake, and the counter must rise by exactly this many: each forgery
// is dropped and counted once, and none ends the handshake.
const unprotForgeries = 20

// unprotForgedMessageID is the Message ID of the forgeries. Ze answered strongSwan's
// IKE_AUTH 1 with its first EAP request, so 2 is the request it waits for. The cached
// id 1 would be refused earlier, by replayCachedResponse, and prove nothing here.
const unprotForgedMessageID = 2

// unprotSKFillerOctets is the body of the forged Encrypted payload: an 8-octet IV,
// 40 octets of ciphertext and a 16-octet ICV for the scenario's aes256gcm16. Every
// octet is 0xa5, so the ICV check fails and nothing decrypts.
const unprotSKFillerOctets = 64

// unprotDropMetric is the Prometheus series discardUnauthenticatedHandshakeRequest
// (internal/component/ike/engine/responder_unauthenticated.go) raises for each drop.
const unprotDropMetric = `ze_ipsec_error_notify_suppressed_total{reason="handshake-unauthenticated"}`

// unprotSwanConnecting matches the IKE SA line `swanctl --list-sas` prints while the
// handshake runs: "ze: #2, CONNECTING, IKEv2, <spi-i>_i* <spi-r>_r". The responder SPI
// is Ze's, chosen in its IKE_SA_INIT response.
var unprotSwanConnecting = regexp.MustCompile(`(?m)^ze: #[0-9]+, CONNECTING, IKEv2, ([0-9a-fA-F]{16})_i\*? ([0-9a-fA-F]{16})_r`)

// unprotHoldRule is the iptables rule that holds Ze's answers back from strongSwan: every
// datagram from Ze's UDP 4500, which carries IKE_AUTH once both sides float there.
var unprotHoldRule = []string{"INPUT", "-s", zeIP, "-p", "udp", "--sport", "4500", "-j", "DROP"}

// checkResponderEAPSurvivesUnprotectedAuth proves RFC 7296 Section 2.4 against
// strongSwan as the EAP-MSCHAPv2 client and Ze as responder and authenticator: "an
// endpoint MUST NOT conclude that the other endpoint has failed based on any routing
// information (e.g., ICMP messages) or IKE messages that arrive without cryptographic
// protection".
//
// Method: the tunnel establishes once, unmolested, as the control. It is torn down and
// re-initiated while Ze's answers from port 4500 are dropped on strongSwan's side, so Ze
// receives IKE_AUTH 1, answers with its first EAP request, and waits in its EAP state;
// charon's retransmission of IKE_AUTH 1 shows the request went unanswered. Forged
// IKE_AUTH requests then go to Ze from strongSwan's address and port, under the live SPI
// pair at Message ID 2, each carrying an Encrypted payload that fails its integrity
// check. Ze's drop counter must rise by exactly the number sent, and once the hold is
// lifted charon's next retransmission must carry the EAP exchange to an established
// tunnel that passes traffic.
//
// Before the fix, the first forgery set StateDead: the counter never rose and the
// handshake never completed, so both assertions go red with the fix reverted.
func checkResponderEAPSurvivesUnprotectedAuth(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return fmt.Errorf("control exchange: %w", err)
	}
	if _, err := lab.exec(ctx, swanPeer, cmdSwanctl, "--terminate", "--ike", "ze"); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: "control IKE SA termination",
	}, func(probe context.Context) (string, error) { return lab.listSAs(probe) }, func(output string) bool {
		return !strings.Contains(output, "ESTABLISHED")
	})
	if err != nil {
		return err
	}
	dropsBefore, err := lab.unprotDropCount(ctx)
	if err != nil {
		return err
	}
	swanLogs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	retransmitsBefore := strings.Count(swanLogs, "retransmit 1 of request with message ID 1")

	if _, err := lab.exec(ctx, swanPeer, append([]string{"iptables", "-I"}, unprotHoldRule...)...); err != nil {
		return err
	}
	defer lab.execQuiet(context.Background(), swanPeer, append([]string{"iptables", "-D"}, unprotHoldRule...)...)
	if err := lab.check.Lab.ExecDetached(ctx, swanPeer, []string{cmdSwanctl, "--initiate", "--child", "ze-child"}, nil); err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second,
		Description: "charon retransmitting IKE_AUTH 1, which Ze answered into the hold",
	}, func(probe context.Context) (string, error) { return lab.logs(probe, swanPeer) }, func(current string) bool {
		return strings.Count(current, "retransmit 1 of request with message ID 1") > retransmitsBefore
	})
	if err != nil {
		return err
	}
	listing, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	spis := unprotSwanConnecting.FindStringSubmatch(listing)
	if spis == nil {
		return interoplab.WithSearchedOutput(fmt.Errorf("no CONNECTING IKE SA with both SPIs in swanctl --list-sas"), listing)
	}
	forged, err := unprotForgedAuthHex(spis[1], spis[2])
	if err != nil {
		return err
	}
	for range unprotForgeries {
		if err := lab.injectOutOfSA(ctx, forged); err != nil {
			return err
		}
	}
	var lastDrops int
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 15 * time.Second, Interval: time.Second,
		Description: "Ze counting each forged IKE_AUTH as an unauthenticated drop",
	}, func(probe context.Context) (int, error) { return lab.unprotDropCount(probe) }, func(current int) bool {
		lastDrops = current
		return current-dropsBefore == unprotForgeries
	})
	if err != nil {
		return fmt.Errorf("%s rose by %d, want %d: %w", unprotDropMetric, lastDrops-dropsBefore, unprotForgeries, err)
	}

	if _, err := lab.exec(ctx, swanPeer, append([]string{"iptables", "-D"}, unprotHoldRule...)...); err != nil {
		return err
	}
	if err := lab.waitSA(ctx, 60*time.Second); err != nil {
		return fmt.Errorf("the EAP exchange did not complete after the forgeries: %w", err)
	}
	if err := lab.waitChild(ctx, ""); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "no traffic after an EAP exchange that survived forged IKE_AUTH requests")
}

// unprotForgedAuthHex builds the forged IKE_AUTH request rawprobe sends, as the 0x hex
// injectOutOfSA takes. The four zero octets of the non-ESP marker (RFC 3948 Section
// 2.2) come first, because the exchange runs on UDP 4500. The RFC 7296 Section 3.1
// header follows: the two SPIs, Next Payload 46 (Encrypted), version 2.0 (0x20),
// Exchange Type IKE_AUTH (35), Flags Initiator (0x08), Message ID and Length. Then the
// Encrypted payload: Next Payload 48 (EAP), the critical octet, its length, and
// unprotSKFillerOctets of filler.
func unprotForgedAuthHex(initiatorSPI, responderSPI string) (string, error) {
	const markerOctets, headerOctets, genericOctets = 4, 28, 4
	skOctets := genericOctets + unprotSKFillerOctets
	total := headerOctets + skOctets
	message := make([]byte, 0, markerOctets+total)
	message = append(message, 0, 0, 0, 0)
	for _, spi := range []string{initiatorSPI, responderSPI} {
		raw, err := hex.DecodeString(spi)
		if err != nil {
			return "", fmt.Errorf("SPI %q from swanctl --list-sas is not hex: %w", spi, err)
		}
		message = append(message, raw...)
	}
	message = append(message, 46, 0x20, 35, 0x08,
		0, 0, 0, unprotForgedMessageID,
		byte(total>>24), byte(total>>16), byte(total>>8), byte(total),
		48, 0, byte(skOctets>>8), byte(skOctets))
	for range unprotSKFillerOctets {
		message = append(message, 0xa5)
	}
	return "0x" + hex.EncodeToString(message), nil
}

// unprotDropCount reads unprotDropMetric from Ze's Prometheus endpoint. A series not
// yet published reads zero, the true count before the first drop: the checker asserts
// a rise by an exact amount, never an absolute.
func (l *scenarioLab) unprotDropCount(ctx context.Context) (int, error) {
	answer, err := l.query(ctx, zePeer, "python3", "-c",
		"import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:9273/metrics', timeout=3).read().decode())")
	if err != nil {
		return 0, fmt.Errorf("read Ze's Prometheus endpoint: %w", err)
	}
	for line := range strings.SplitSeq(answer, "\n") {
		value, found := strings.CutPrefix(line, unprotDropMetric+" ")
		if !found {
			continue
		}
		count, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("%s value %q: %w", unprotDropMetric, value, err)
		}
		return int(count), nil
	}
	return 0, nil
}
