// Design: docs/architecture/testing/interop.md -- the port-translated NAT scenarios and the rebind step.
// Related: nat.go -- the nat.conf port column and natPortRule.
// Related: checkers.go -- the scenario registry these checkers join.
// RFC: rfc/short/rfc3948.md -- RFC3948-2.1-2, the encapsulation ports follow IKE.
package ipsec

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The public ports the port-translated scenarios map strongSwan's UDP 4500 to: the one
// in nat.conf, then the one the rebind step moves it to. Neither is 4500, so a Child SA
// that encapsulates to the constant 4500 fails the assertion.
const (
	natFirstPort  uint16 = 14500
	natSecondPort uint16 = 14501
)

// The rebind log lines (internal/component/ike/engine/rebind.go). Row 1 is the only
// line the tunnel step accepts, row 2 with reason "transport mode" the only one the
// transport step accepts.
const (
	rebindLineMigrated = "ike: peer endpoint moved, Child SAs migrated to the new address and port"
	rebindLineRekeyed  = "ike: peer endpoint moved, Child SA cannot be migrated here, so it is rekeyed onto the new address and port"
	rebindLineRefused  = "ike: peer endpoint moved, migrating the Child SA failed and the old endpoints stay in place"
	rebindLineLost     = "ike: peer endpoint moved, the Child SA was lost while migrating it"
	rebindWait         = 90 * time.Second
)

// natPortTranslation is the strongSwan row of the port-translated scenarios' nat.conf.
// The checker needs it to replace the box's port rule, and the row in the file is its
// source: a mismatch fails the iptables delete, so the two cannot drift silently.
var natPortTranslation = natTranslation{
	real: netip.MustParseAddr(swanIP), public: netip.MustParseAddr(swanPublicIP), natPort: natFirstPort,
}

// checkRealNATPortTranslatedZeResponder proves RFC 3948 Section 2.1 across a NAT that
// changes strongSwan's port: "the Source Port and Destination Port MUST be the same as
// that used by IKE traffic." Ze's outbound state encapsulates to public port 14500,
// traffic crosses both ways, then the rebind step remaps the NAT to 14501 and Ze
// migrates the Child SA in place (row 1, same SPI) onto the new port.
func checkRealNATPortTranslatedZeResponder(ctx context.Context, lab *scenarioLab) error {
	if err := lab.natPortEstablish(ctx, true); err != nil {
		return err
	}
	spi, err := lab.waitZeOutboundEncap(ctx, natFirstPort)
	if err != nil {
		return err
	}
	if err := lab.natPortTunnelTraffic(ctx); err != nil {
		return err
	}
	if err := lab.natRebind(ctx); err != nil {
		return err
	}
	if err := lab.waitRebindLine(ctx, rebindLineMigrated, ""); err != nil {
		return err
	}
	moved, err := lab.waitZeOutboundEncap(ctx, natSecondPort)
	if err != nil {
		return err
	}
	if moved != spi {
		return fmt.Errorf("ze's outbound SPI changed from %s to %s, so the Child SA was rekeyed, not migrated", spi, moved)
	}
	return lab.natPortTunnelTraffic(ctx)
}

// checkRealNATPortTranslatedTransportZeResponder is the same NAT for a transport-mode
// Child SA, which no TunnelMigrator moves: after the remap Ze logs row 2 with reason
// "transport mode", rekeys the Child SA (a new SPI) and the new outbound state
// encapsulates to 14501.
func checkRealNATPortTranslatedTransportZeResponder(ctx context.Context, lab *scenarioLab) error {
	if err := lab.natPortEstablish(ctx, false); err != nil {
		return err
	}
	spi, err := lab.waitZeOutboundEncap(ctx, natFirstPort)
	if err != nil {
		return err
	}
	if err := lab.natPortTransportTraffic(ctx); err != nil {
		return err
	}
	if err := lab.natRebind(ctx); err != nil {
		return err
	}
	if err := lab.waitRebindLine(ctx, rebindLineRekeyed, `reason="transport mode"`); err != nil {
		return err
	}
	moved, err := lab.waitZeOutboundEncap(ctx, natSecondPort)
	if err != nil {
		return err
	}
	if moved == spi {
		return fmt.Errorf("ze's outbound SPI %s did not change, so the transport Child SA was not rekeyed", spi)
	}
	return lab.natPortTransportTraffic(ctx)
}

// checkRealNATPortTranslatedEAPZeResponder proves the EAP responder's first Child SA
// takes its ports from the IKE SA (AC-6): the endpoint is adopted once AUTH-from-MSK
// verifies, before the Child SA is built, so the state reads public port 14500.
func checkRealNATPortTranslatedEAPZeResponder(ctx context.Context, lab *scenarioLab) error {
	if err := lab.natPortEstablish(ctx, true); err != nil {
		return err
	}
	if _, err := lab.waitZeOutboundEncap(ctx, natFirstPort); err != nil {
		return err
	}
	return lab.natPortTunnelTraffic(ctx)
}

// responderEAPTLS12RefusalFacts are the parts of ze's refusal line
// (eapTLS12ExportRefused, internal/core/eap/eap_tls.go) the scenario requires: the
// refused export, the peer, the version, the missing extension and crypto/tls's own
// sentence, so a different failure cannot pass as this refusal.
var responderEAPTLS12RefusalFacts = [...]string{
	"cannot export the RFC 5216 Section 2.3 MSK",
	"for peer CN=swan-eap-client",
	"on TLS 1.2",
	"RFC 7627 extended master secret",
	"crypto/tls: ExportKeyingMaterial is unavailable",
}

// checkResponderEAPTLS12 drives ze's EAP-TLS server with a strongSwan client pinned to
// TLS 1.2, and asserts the TLS handshake completes and ze then refuses the MSK.
//
// No IKEv2 EAP-TLS initiator sends the RFC 7627 extended master secret on TLS 1.2:
// strongSwan through 6.1.0 defines the extension constant and never sends it,
// Libreswan has no EAP-TLS initiator, and OpenIKED has no EAP-TLS. Without it
// crypto/tls refuses the RFC 5705 export that RFC 5216 Section 2.3 defines the MSK
// as, so ze keeps that requirement and the scenario asserts the refusal (owner
// decision, 2026-10-21). It cannot go red on the session_id, because charon accepts
// an empty and a chosen one alike; the unit test reads the id off the ServerHello.
//
// Each assertion is numbered, so a red run names which one failed.
func checkResponderEAPTLS12(ctx context.Context, lab *scenarioLab) error {
	fail := func(assertion int, cause error) error {
		return fmt.Errorf("scenario responder-eap-tls12 assertion %d: %w", assertion, cause)
	}

	// Assertion 1. charon negotiated TLS 1.2, which is the condition under test.
	if err := lab.waitLog(ctx, swanPeer, "negotiated TLS 1.2", lab.timeout); err != nil {
		return fail(1, err)
	}
	// Assertion 2. charon received ze's Finished, so the handshake completed and the
	// refusal below is ze's and not a failed handshake.
	if err := lab.waitLog(ctx, swanPeer, "received TLS Finished handshake", lab.timeout); err != nil {
		return fail(2, err)
	}
	// Assertion 3. ze answered with EAP-Failure, so the IKE SA did not authenticate.
	if err := lab.waitLog(ctx, swanPeer, "received EAP_FAILURE, EAP authentication failed", lab.timeout); err != nil {
		return fail(3, err)
	}
	logs, err := lab.logs(ctx, zePeer)
	if err != nil {
		return fail(4, err)
	}
	// Assertion 4. The failure is ze's refusal to export the MSK without the extended
	// master secret, stated with every fact in responderEAPTLS12RefusalFacts.
	for _, fact := range responderEAPTLS12RefusalFacts {
		if !strings.Contains(logs, fact) {
			return fail(4, interoplab.WithSearchedOutput(fmt.Errorf("ze's EAP-TLS refusal does not state: %s", fact), logs))
		}
	}
	// Assertion 5. ze installed no XFRM state from the refused exchange.
	if err := lab.checkXFRMCount(ctx, zePeer, 0); err != nil {
		return fail(5, err)
	}
	// Assertion 6. strongSwan installed none either.
	if err := lab.checkXFRMCount(ctx, swanPeer, 0); err != nil {
		return fail(6, err)
	}
	return nil
}

// natPortEstablish routes strongSwan to Ze through the NAT box, then starts the
// connection. The box owns strongSwan's public address, so a host route via it is
// what puts the box on the path; Ze keeps its own address and is not behind a NAT,
// which is the precondition of the endpoint adoption the rebind step drives
// (RFC 7296 Section 2.23). strongSwan therefore starts with start_action = none.
func (l *scenarioLab) natPortEstablish(ctx context.Context, tunnel bool) error {
	var route textbuf.Buffer
	route.Str(zeIP).Str("/32")
	if _, err := l.exec(ctx, swanPeer, "ip", "route", "replace", route.String(), "via", swanPublicIP); err != nil {
		return fmt.Errorf("route strongSwan to ze through the NAT box: %w", err)
	}
	if _, err := l.exec(ctx, swanPeer, cmdSwanctl, "--initiate", "--child", "ze-child"); err != nil {
		return fmt.Errorf("strongSwan did not initiate: %w", err)
	}
	if err := establish(ctx, l); err != nil {
		return err
	}
	if !tunnel {
		return nil
	}
	if err := l.natInnerAddress(ctx, zePeer, natTunnelZeInner, natTunnelSwanInner); err != nil {
		return err
	}
	return l.natInnerAddress(ctx, swanPeer, natTunnelSwanInner, natTunnelZeInner)
}

// natRebind replaces the box's port rule so strongSwan's UDP 4500 leaves from the
// second public port, then flushes the box's UDP conntrack entries. NAT rules apply to
// a flow's first packet only, and strongSwan's keepalives and DPD keep the old entry
// alive, so without the flush the new rule never applies.
//
// The flush is checked twice, because a flush that silently did nothing leaves the
// checker waiting for a move that cannot come. conntrack MUST answer with its deleted
// count, and a listing afterwards MUST hold no flow from strongSwan's UDP 4500 still
// mapped to the first public port. A flow a keepalive creates after the flush is
// mapped by the new rule, so it never matches.
func (l *scenarioLab) natRebind(ctx context.Context) error {
	var script textbuf.Buffer
	script.Str(natPortRule("-D", natPortTranslation, natFirstPort)).Str(" && ").
		Str(natPortRule("-I", natPortTranslation, natSecondPort))
	if _, err := l.exec(ctx, natPeer, "sh", "-c", script.String()); err != nil {
		return fmt.Errorf("remap strongSwan's UDP 4500 on the NAT box: %w", err)
	}
	// The shell reports conntrack's exit status in each answer rather than as the exec
	// result: conntrack's exit status after a flush that matched nothing is not the
	// evidence, the count and the listing below are.
	flushed, err := l.exec(ctx, natPeer, "sh", "-c", "conntrack -D -p udp 2>&1; echo conntrack-exit=$?")
	if err != nil {
		return fmt.Errorf("flush the NAT box's UDP conntrack entries: %w", err)
	}
	if _, err := conntrackDeletedCount(flushed); err != nil {
		return fmt.Errorf("flush the NAT box's UDP conntrack entries: %w", err)
	}
	var list textbuf.Buffer
	list.Str("conntrack -L -p udp -s ").Str(swanIP).Str(" --sport 4500 2>&1; echo conntrack-exit=$?")
	listing, err := l.exec(ctx, natPeer, "sh", "-c", list.String())
	if err != nil {
		return fmt.Errorf("list the NAT box's UDP conntrack entries after the flush: %w", err)
	}
	stale, err := conntrackStaleMapping(listing)
	if err != nil {
		return fmt.Errorf("list the NAT box's UDP conntrack entries after the flush: %w", err)
	}
	if stale != "" {
		return fmt.Errorf("the NAT box still maps strongSwan's UDP 4500 to port %d after the flush, so the remap cannot take effect: %s", natFirstPort, stale)
	}
	return nil
}

// conntrackDeletedPattern reads the count conntrack prints after a delete.
var conntrackDeletedPattern = regexp.MustCompile(`(\d+) flow entries have been deleted`)

// conntrackDeletedCount answers the count from a `conntrack -D` answer, and an error
// carrying the whole answer when it holds none: the flush did not run.
func conntrackDeletedCount(answer string) (int, error) {
	match := conntrackDeletedPattern.FindStringSubmatch(answer)
	if match == nil {
		return 0, fmt.Errorf("conntrack printed no deleted count, so the flush did not run: %q", strings.TrimSpace(answer))
	}
	return strconv.Atoi(match[1])
}

// conntrackStaleMapping answers the first line of a `conntrack -L` listing whose
// reply tuple still targets natFirstPort, or "" when none does. A listing without
// conntrack's closing summary is an error: the listing did not run.
func conntrackStaleMapping(listing string) (string, error) {
	if !strings.Contains(listing, "flow entries have been shown") {
		return "", fmt.Errorf("conntrack printed no listing summary, so the listing did not run: %q", strings.TrimSpace(listing))
	}
	var tb textbuf.Buffer
	// String drains the buffer, so it is read once, here.
	want := tb.Str("dport=").Str(strconv.Itoa(int(natFirstPort))).Byte(' ').String()
	for line := range strings.SplitSeq(listing, "\n") {
		if strings.Contains(line+" ", want) {
			return strings.TrimSpace(line), nil
		}
	}
	return "", nil
}

// waitRebindLine waits for Ze's rebind log line want, carrying attribute when one is
// named, and fails when any other rebind line appears: the step proves one branch.
func (l *scenarioLab) waitRebindLine(ctx context.Context, want, attribute string) error {
	if err := l.waitLog(ctx, zePeer, want, rebindWait); err != nil {
		return err
	}
	logs, err := l.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	for _, other := range []string{rebindLineMigrated, rebindLineRekeyed, rebindLineRefused, rebindLineLost} {
		if other != want && strings.Contains(logs, other) {
			return interoplab.WithSearchedOutput(fmt.Errorf("ze took another rebind branch: %q", other), logs)
		}
	}
	if attribute == "" {
		return nil
	}
	for line := range strings.SplitSeq(logs, "\n") {
		if strings.Contains(line, want) && strings.Contains(line, attribute) {
			return nil
		}
	}
	return fmt.Errorf("ze's rebind line lacks %s", attribute)
}

// waitZeOutboundEncap waits until Ze's outbound state toward strongSwan's public
// address encapsulates from port 4500 to dport, and answers its SPI.
func (l *scenarioLab) waitZeOutboundEncap(ctx context.Context, dport uint16) (string, error) {
	var tb textbuf.Buffer
	// String drains the buffer, so it is read once, here. A second read inside the
	// poll answered "", which every encapsulating state contains, whatever its port.
	want := tb.Str("encap type espinudp sport 4500 dport ").Str(strconv.Itoa(int(dport))).String()
	var spi string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: rebindWait, Interval: 2 * time.Second, Description: "ze outbound " + want,
	}, func(probe context.Context) (string, error) {
		return l.xfrmState(probe, zePeer)
	}, func(state string) bool {
		spi = outboundEncapSPI(state, want)
		return spi != ""
	})
	return spi, err
}

// outboundEncapSPI finds the `ip xfrm state` entry from Ze to strongSwan's public
// address that carries encap, and answers its SPI, or "" when none does.
func outboundEncapSPI(state, encap string) string {
	head := "src " + zeIP + " dst " + swanPublicIP
	for entry := range strings.SplitSeq("\n"+state, "\nsrc ") {
		entry = "src " + entry
		if !strings.HasPrefix(entry, head) || !strings.Contains(entry, encap) {
			continue
		}
		for field := range strings.FieldsSeq(entry) {
			if strings.HasPrefix(field, "0x") {
				return field
			}
		}
	}
	return ""
}

// natPortTunnelTraffic pings across the tunnel both ways and requires strongSwan's ESP
// byte counters to rise, so the encrypted path, not a bypass, carried it.
func (l *scenarioLab) natPortTunnelTraffic(ctx context.Context) error {
	return l.natPortTraffic(ctx,
		pingProbe{peer: zePeer, target: natTunnelSwanInner, source: natTunnelZeInner},
		pingProbe{peer: swanPeer, target: natTunnelZeInner, source: natTunnelSwanInner})
}

// natPortTransportTraffic is natPortTunnelTraffic for the transport Child SA, whose
// traffic runs between Ze and the address Ze sees strongSwan at.
func (l *scenarioLab) natPortTransportTraffic(ctx context.Context) error {
	return l.natPortTraffic(ctx,
		pingProbe{peer: zePeer, target: swanPublicIP},
		pingProbe{peer: swanPeer, target: zeIP})
}

func (l *scenarioLab) natPortTraffic(ctx context.Context, there, back pingProbe) error {
	before, err := l.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := l.requireLosslessPingFrom(ctx, there, 3); err != nil {
		return err
	}
	if err := l.requireLosslessPingFrom(ctx, back, 3); err != nil {
		return err
	}
	after, err := l.xfrmCounters(ctx, swanPeer)
	if err != nil {
		return err
	}
	if counterSum(after) <= counterSum(before) {
		return fmt.Errorf("strongSwan's ESP byte counters did not rise, so the pings were not protected")
	}
	return nil
}

func counterSum(counters map[saKey]uint64) uint64 {
	var sum uint64
	for _, value := range counters {
		sum += value
	}
	return sum
}
