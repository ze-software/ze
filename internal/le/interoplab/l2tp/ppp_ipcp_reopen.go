// Design: docs/architecture/testing/interop.md -- IPCP withdrawal and reopen after an injected Terminate-Request.
// Related: ppp_code_reject.go -- the ppp-code-reject-xl2tpd assertions these helpers end.
// Related: avprelay/pppinject.go -- the relay that injects the IPCP Terminate-Request.
package l2tp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// ipcpTerminateMarker is the relay marker for the one injected IPCP
	// Terminate-Request (avprelay/pppinject.go, terminateMarker).
	ipcpTerminateMarker = "/run/ipcp-terminate"

	// ipcpWithdrawalPoll is how often the checker reads Ze's PPP interface
	// while it waits for the IPv4 address to leave. Ze holds IPCP in Stopping
	// for one Restart time after it answers the Terminate-Request (3 s: the
	// scenario's ze.conf sets no lcp restart-timer), and answers no
	// Configure-Request there, so the address stays absent for one to two
	// Restart times. A read every 100 ms lands inside that window.
	ipcpWithdrawalPoll = 100 * time.Millisecond
)

// triggerIPCPTerminate creates the relay's marker, then sends one echo
// request from the LAC, detached, as the LAC datagram the relay injects the
// Terminate-Request behind. Detached, so the withdrawal poll that follows
// starts at once rather than after the echo's own timeout.
func triggerIPCPTerminate(ctx context.Context, lab labOperations) error {
	if _, err := lab.Exec(ctx, peerAVPRelay, []string{commandTouch, ipcpTerminateMarker}, nil); err != nil {
		return err
	}
	return lab.ExecDetached(ctx, peerLAC, []string{commandPing, "-c", "1", "-W", "1", localPPPAddress}, nil)
}

// waitIPv4Withdrawn polls Ze's PPP interface link until it holds no IPv4
// address while the interface itself stays: the IPCP TLD withdrawal, which
// leaves LCP and the PPP unit up.
func waitIPv4Withdrawn(ctx context.Context, lab labOperations, link string) error {
	var tb textbuf.Buffer
	description := tb.Str("ze ").Str(link).Str(" without its IPv4 address").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 15 * time.Second, Interval: ipcpWithdrawalPoll, Description: description,
	}, func(probeCtx context.Context) (string, error) {
		return ipv4AddressOutput(probeCtx, lab, link)
	}, func(output string) bool {
		_, _, held := pppIPv4Pair(output)
		return !held
	})
	return err
}

// waitIPv4Reinstalled polls Ze's PPP interface link until it holds the
// session's local address with the subscriber's as its peer again.
func waitIPv4Reinstalled(ctx context.Context, lab labOperations, link string) error {
	var tb textbuf.Buffer
	description := tb.Str("ze ").Str(link).Str(" with ").Str(localPPPAddress).Str(" peer ").Str(peerPPPAddress).Str(" again").String()
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second, Description: description,
	}, func(probeCtx context.Context) (string, error) {
		return ipv4AddressOutput(probeCtx, lab, link)
	}, func(output string) bool {
		local, peer, held := pppIPv4Pair(output)
		if !held {
			return false
		}
		if local != localPPPAddress {
			return false
		}
		return peer == peerPPPAddress
	})
	return err
}

// ipv4AddressOutput answers `ip -4 -o addr show dev <link>` in Ze's container.
// An interface with no IPv4 address prints nothing, which is an answer here,
// so the output is not refused for being empty; a missing interface fails the
// command and is an error.
func ipv4AddressOutput(ctx context.Context, lab labOperations, link string) (string, error) {
	result, err := lab.Exec(ctx, peerZe, []string{"ip", "-4", "-o", "addr", commandShow, ipOptionDevice, link}, nil)
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// pppIPv4Pair reads the first IPv4 address line of `ip -4 -o addr show`
// output, "inet <local> peer <peer>/<len> ...", and answers the local and
// peer addresses without their prefix lengths. held is false when no line
// carries an inet address.
func pppIPv4Pair(output string) (local, peer string, held bool) {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		for index := 0; index+1 < len(fields); index++ {
			switch fields[index] {
			case "inet":
				local, _, _ = strings.Cut(fields[index+1], "/")
				held = true
			case "peer":
				peer, _, _ = strings.Cut(fields[index+1], "/")
			}
		}
		if held {
			return local, peer, true
		}
	}
	return "", "", false
}

// ipcpReopened answers nil once the frames hold, after Ze's IPCP
// Terminate-Ack carrying terminateIdentifier, an IPCP Configure-Ack from each
// side: Ze acknowledging pppd's request and pppd acknowledging Ze's, which
// opens IPCP on both ends (RFC 1661 Section 4.1, Ack-Rcvd RCR+ and Ack-Sent
// RCA to Opened). It answers an error naming what is missing until then.
func ipcpReopened(frames []pppFrame) error {
	ack := -1
	for index, frame := range frames {
		if frame.direction != relayFromZe {
			continue
		}
		if frame.protocol != protocolIPCP {
			continue
		}
		if len(frame.packet) < 2 {
			continue
		}
		if frame.packet[0] != codeTerminateAck {
			continue
		}
		if frame.packet[1] == terminateIdentifier {
			ack = index
			break
		}
	}
	if ack < 0 {
		return errors.New("ze sent no IPCP Terminate-Ack to reopen after")
	}
	zeAcked, lacAcked := false, false
	for _, frame := range frames[ack+1:] {
		if frame.protocol != protocolIPCP {
			continue
		}
		if len(frame.packet) == 0 {
			continue
		}
		if frame.packet[0] != codeConfigureAck {
			continue
		}
		switch frame.direction {
		case relayFromZe:
			zeAcked = true
		case relayFromLAC:
			lacAcked = true
		}
	}
	if !zeAcked {
		return errors.New("ze sent no IPCP Configure-Ack after its Terminate-Ack: IPCP did not reopen")
	}
	if !lacAcked {
		return errors.New("pppd sent no IPCP Configure-Ack after ze's Terminate-Ack: IPCP did not reopen")
	}
	return nil
}
