// Design: docs/architecture/testing/interop.md -- the port-translated NAT scenarios and the rebind step.
// Related: nat.go -- the nat.conf port column and natPortRule.
// Related: checkers.go -- the scenario registry these checkers join.
// RFC: rfc/short/rfc3948.md -- RFC3948-2.1-2, the encapsulation ports follow IKE.
package ipsec

import (
	"context"
	"fmt"
	"net/netip"
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

// checkResponderEAPTLS12 is the TLS 1.2 regression proof for the EAP-TLS server: a
// strongSwan client capped at TLS 1.2 authenticates and traffic crosses. It cannot go
// red on the session_id, because charon accepts an empty and a chosen one alike.
func checkResponderEAPTLS12(ctx context.Context, lab *scenarioLab) error {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if !strings.Contains(logs, "negotiated TLS 1.2") {
		return interoplab.WithSearchedOutput(fmt.Errorf("strongSwan never logged negotiated TLS 1.2"), logs)
	}
	if _, err := lab.waitXFRM(ctx, swanPeer); err != nil {
		return err
	}
	if _, err := lab.waitXFRM(ctx, zePeer); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not cross the EAP-TLS 1.2 Child SA")
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
func (l *scenarioLab) natRebind(ctx context.Context) error {
	var script textbuf.Buffer
	script.Str(natPortRule("-D", natPortTranslation, natFirstPort)).Str(" && ").
		Str(natPortRule("-I", natPortTranslation, natSecondPort)).
		Str(" && { conntrack -D -p udp || true; }")
	if _, err := l.exec(ctx, natPeer, "sh", "-c", script.String()); err != nil {
		return fmt.Errorf("remap strongSwan's UDP 4500 on the NAT box: %w", err)
	}
	return nil
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
	var want textbuf.Buffer
	want.Str("encap type espinudp sport 4500 dport ").Str(strconv.Itoa(int(dport)))
	var spi string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: rebindWait, Interval: 2 * time.Second, Description: "ze outbound " + want.String(),
	}, func(probe context.Context) (string, error) {
		return l.xfrmState(probe, zePeer)
	}, func(state string) bool {
		spi = outboundEncapSPI(state, want.String())
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
