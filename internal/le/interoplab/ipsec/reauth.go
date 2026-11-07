// Design: docs/architecture/testing/interop.md -- the IPsec interop suite
// Related: checkers.go -- scenarioCheckers, which names this checker
//
// A gateway that reauthenticates a tunnel Ze started (spec-home-cpe-22).
package ipsec

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// reauthWait bounds the wait for strongSwan's reauthentication: the scenario sets
// reauth_time = 60s with rand_time = 0s, so it starts a minute after establishment.
const reauthWait = 3 * time.Minute

const (
	// swanReauthLine is what strongSwan 6.1.0 logs when it starts a reauthentication
	// (src/libcharon/sa/ike_sa.c, reauth).
	swanReauthLine = "reauthenticating IKE_SA"
	// zeAdoptLine is what Ze logs when an `initiate` session adopts the gateway's
	// successor (adoptPeerStartedSuccessor, reauth_receive.go).
	zeAdoptLine = "adopting the IKE SA the gateway created to reauthenticate"
	// zeReconnectLine is what Ze logs when a peer session ends and reconnects
	// (PeerSession.run, reconcile.go): the outage the adoption exists to avoid.
	zeReconnectLine = "ike: peer session ended, reconnecting"
)

// checkResponderReauthsZeInitiator proves AC-1 to AC-3 of spec-home-cpe-22 against
// strongSwan: Ze initiates, strongSwan as the original responder reauthenticates by
// creating a new IKE SA toward Ze, Ze adopts it with its new Child SA, strongSwan
// deletes the old IKE SA, and Ze never reconnects. The ESP SPIs change and traffic
// crosses the tunnel built on the successor.
//
// RFC 7296 Section 2.8.3: "While creation of a new IKE SA can be initiated by either
// party (initiator or responder in the original IKE SA), the use of EAP and/
// or Configuration payloads means in practice that reauthentication has
// to be initiated by the same party as the original IKE SA."
func checkResponderReauthsZeInitiator(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("empty ESP SPI snapshot before strongSwan's reauthentication")
	}
	if err := lab.waitLog(ctx, swanPeer, swanReauthLine, reauthWait); err != nil {
		return fmt.Errorf("strongSwan never started its reauthentication (assumption A-1): %w", err)
	}
	if err := lab.waitLog(ctx, zePeer, zeAdoptLine, lab.timeout); err != nil {
		return fmt.Errorf("ze did not adopt the IKE SA strongSwan created to reauthenticate: %w", err)
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: lab.timeout, Interval: 2 * time.Second, Description: "new ESP SAs after strongSwan's reauthentication",
	}, func(probe context.Context) (map[string]struct{}, error) { return lab.espSPIs(probe, swanPeer) }, func(after map[string]struct{}) bool {
		return newStrings(before, after)
	})
	if err != nil {
		return err
	}
	if err := lab.verifyTunnelTraffic(ctx, "traffic did not cross the tunnel Ze adopted from strongSwan's reauthentication"); err != nil {
		return err
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.Contains(zeLogs, zeReconnectLine) {
		return interoplab.WithSearchedOutput(fmt.Errorf("ze reconnected around strongSwan's reauthentication instead of adopting it"), zeLogs)
	}
	return nil
}

// The NAT reauthentication scenario's port mapping and background ping. Ze's UDP
// 4500 leaves the NAT box from zePublicIP port natFirstPort (nat.conf), so
// strongSwan's reauth IKE_SA_INIT reaches Ze only through that mapping (spec
// assumption A-3).
const (
	// reauthPingLog is where the background ping writes, inside the ze container.
	reauthPingLog = "/run/reauth-ping.log"
	// reauthPingCount and reauthPingInterval span strongSwan's reauth_time of 60s
	// with margin: 240 echoes at 0.5s run for two minutes from establishment.
	reauthPingCount    = "240"
	reauthPingInterval = "0.5"
	reauthPingDeadline = "150"
	// reauthPingLossMax is the loss AC-2 allows across the adoption.
	reauthPingLossMax = 1
)

// reauthPingSummary reads iputils ping's summary: "N packets transmitted, M received".
var reauthPingSummary = regexp.MustCompile(`(\d+) packets transmitted, (\d+) received`)

// checkRealNATPortTranslatedResponderReauthsZeInitiator proves AC-14 of
// spec-home-cpe-22: Ze initiates from behind a NAT that translates its UDP 4500 to
// 172.28.0.6:14500, strongSwan reauthenticates as the original responder, and its
// new IKE_SA_INIT reaches Ze through that mapping. strongSwan's reestablish()
// (src/libcharon/sa/ike_sa.c) copies the old SA's other_host, port included, into
// the new SA and resolves nothing when reauthenticating, so the request goes to
// 172.28.0.6[14500]. Ze adopts the successor, the new Child SA encapsulates both
// ways (Ze to strongSwan's 4500, strongSwan to the public 14500), a ping running
// across the reauthentication loses at most one echo, and Ze never reconnects.
//
// RFC 3948 Section 2.1: "the Source Port and Destination Port MUST be the same as
// that used by IKE traffic."
func checkRealNATPortTranslatedResponderReauthsZeInitiator(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if err := lab.assertNATVerdict(ctx); err != nil {
		return err
	}
	if _, err := lab.waitEncapSPI(ctx, swanPeer, swanIP, zePublicIP, natFirstPort, nil); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, zePeer, natTunnelZeInner, natTunnelSwanInner); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, swanPeer, natTunnelSwanInner, natTunnelZeInner); err != nil {
		return err
	}
	if err := lab.natPortTunnelTraffic(ctx); err != nil {
		return err
	}
	before, err := lab.espSPIs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if len(before) == 0 {
		return fmt.Errorf("empty ESP SPI snapshot before strongSwan's reauthentication")
	}
	if err := lab.startReauthPing(ctx); err != nil {
		return err
	}
	if err := lab.waitLog(ctx, swanPeer, swanReauthLine, reauthWait); err != nil {
		return fmt.Errorf("strongSwan never started its reauthentication (assumption A-1): %w", err)
	}
	if err := lab.waitLog(ctx, zePeer, zeAdoptLine, lab.timeout); err != nil {
		return fmt.Errorf("ze did not adopt the IKE SA strongSwan created through the NAT mapping to reauthenticate: %w", err)
	}
	if err := lab.requireReauthSentThroughMapping(ctx); err != nil {
		return err
	}
	// The successor's Child SA, read on both sides: strongSwan's outbound state
	// encapsulates to Ze's public port, Ze's to strongSwan's 4500, each with an SPI
	// the old Child SA did not hold.
	if _, err := lab.waitEncapSPI(ctx, swanPeer, swanIP, zePublicIP, natFirstPort, before); err != nil {
		return err
	}
	if _, err := lab.waitEncapSPI(ctx, zePeer, zeIP, swanPublicIP, 4500, before); err != nil {
		return err
	}
	if err := lab.requireReauthPingLoss(ctx); err != nil {
		return err
	}
	if err := lab.natPortTunnelTraffic(ctx); err != nil {
		return err
	}
	zeLogs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if strings.Contains(zeLogs, zeReconnectLine) {
		return interoplab.WithSearchedOutput(fmt.Errorf("ze reconnected around strongSwan's reauthentication instead of adopting it"), zeLogs)
	}
	return nil
}

// waitEncapSPI waits for an `ip xfrm state` entry on peer from source to target that
// encapsulates from UDP 4500 to dport, with an SPI outside old, and answers that SPI.
// A nil old accepts any SPI.
func (l *scenarioLab) waitEncapSPI(ctx context.Context, peer, source, target string, dport uint16, old map[string]struct{}) (string, error) {
	var tb textbuf.Buffer
	want := tb.Str("encap type espinudp sport 4500 dport ").Int(int64(dport)).String()
	var spi string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: l.timeout, Interval: 2 * time.Second, Description: peer + " outbound " + source + " to " + target + " " + want,
	}, func(probe context.Context) (string, error) {
		return l.xfrmState(probe, peer)
	}, func(state string) bool {
		spi = newEncapSPI(state, source, target, want, old)
		return spi != ""
	})
	return spi, err
}

// newEncapSPI finds the state entry from source to target carrying encap whose SPI
// is not in old, and answers its SPI, or "" when none does.
func newEncapSPI(state, source, target, encap string, old map[string]struct{}) string {
	head := "src " + source + " dst " + target
	for entry := range strings.SplitSeq("\n"+state, "\nsrc ") {
		entry = "src " + entry
		if !strings.HasPrefix(entry, head) {
			continue
		}
		if !strings.Contains(entry, encap) {
			continue
		}
		for field := range strings.FieldsSeq(entry) {
			if !strings.HasPrefix(field, "0x") {
				continue
			}
			if _, seen := old[field]; seen {
				break
			}
			return field
		}
	}
	return ""
}

// requireReauthSentThroughMapping reads charon's log after its reauthentication line
// for a datagram sent to Ze's public address and port. sender.c logs every datagram
// ("sending packet: from %#H to %#H (%zu bytes)"), and the reauth IKE_SA_INIT is the
// first one the new SA sends, so a reauth sent to Ze's real address or to port 500
// leaves no such line.
func (l *scenarioLab) requireReauthSentThroughMapping(ctx context.Context) error {
	swanLogs, err := l.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	start := strings.Index(swanLogs, swanReauthLine)
	if start < 0 {
		return interoplab.WithSearchedOutput(fmt.Errorf("strongSwan's log lost its reauthentication line"), swanLogs)
	}
	var tb textbuf.Buffer
	want := tb.Str("sending packet: from ").Str(swanIP).Str("[4500] to ").Str(zePublicIP).
		Byte('[').Int(int64(natFirstPort)).Byte(']').String()
	if !strings.Contains(swanLogs[start:], want) {
		return interoplab.WithSearchedOutput(fmt.Errorf("strongSwan sent nothing to Ze's NAT mapping after it started reauthenticating, want %q", want), swanLogs[start:])
	}
	return nil
}

// startReauthPing starts a ping from Ze's inner address that outlives the exec
// which starts it: its output goes to reauthPingLog, read by requireReauthPingLoss.
func (l *scenarioLab) startReauthPing(ctx context.Context) error {
	var script textbuf.Buffer
	script.Str(cmdPing).Str(" -c ").Str(reauthPingCount).Str(" -i ").Str(reauthPingInterval).
		Str(" -w ").Str(reauthPingDeadline).Str(" -W 2 -I ").Str(natTunnelZeInner).Byte(' ').Str(natTunnelSwanInner).
		Str(" </dev/null >").Str(reauthPingLog).Str(" 2>&1 &")
	if _, err := l.exec(ctx, zePeer, "sh", "-c", script.String()); err != nil {
		return fmt.Errorf("start the ping that spans strongSwan's reauthentication: %w", err)
	}
	return nil
}

// requireReauthPingLoss waits for the background ping's summary and refuses more
// than reauthPingLossMax lost echoes, or a summary it cannot read.
func (l *scenarioLab) requireReauthPingLoss(ctx context.Context) error {
	output, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: reauthWait, Interval: 2 * time.Second, Description: "the ping across strongSwan's reauthentication to finish",
	}, func(probe context.Context) (string, error) {
		return l.exec(probe, zePeer, "cat", reauthPingLog)
	}, func(output string) bool {
		return reauthPingSummary.MatchString(output)
	})
	if err != nil {
		return err
	}
	match := reauthPingSummary.FindStringSubmatch(output)
	sent, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("ping summary has an unreadable sent count %q: %s", match[1], output)
	}
	received, err := strconv.Atoi(match[2])
	if err != nil {
		return fmt.Errorf("ping summary has an unreadable received count %q: %s", match[2], output)
	}
	if sent-received > reauthPingLossMax {
		return fmt.Errorf("the ping across strongSwan's reauthentication lost %d of %d echoes, want at most %d: %s",
			sent-received, sent, reauthPingLossMax, output)
	}
	return nil
}
