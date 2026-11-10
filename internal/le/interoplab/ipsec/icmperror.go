// Design: docs/architecture/testing/interop.md -- an ICMP error on a Child SA, delivered or dropped by its payload header.
// Related: helpers.go -- inner addresses, /proc/net/snmp counters and directed XFRM byte counters.
package ipsec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The icmp-error-payload-check scenario's addresses. Ze's inner address and the
// consistent target sit inside the Child SA's selectors (10.2.0.0/24 and
// 10.1.0.0/24); the error source and the inconsistent target sit inside none.
const (
	icmpZeInner            = "10.2.0.1"
	icmpZePrefix           = "10.2.0.0/24"
	icmpConsistentTarget   = "10.1.0.77"
	icmpInconsistentTarget = "10.7.0.5"
	icmpErrorSource        = "10.9.9.9"
	// icmpProbeCount echo requests with TTL 1 each draw one Time Exceeded from
	// strongSwan, inside its per-destination ICMP rate limit (icmpv4_xrlim_allow,
	// one a second after a burst of six).
	icmpProbeCount = 3
	// The /proc/net/snmp counters each step reads. icmp_rcv (net/ipv4/icmp.c)
	// counts a received message by type only after its XFRM policy checks pass,
	// so InTimeExcds on Ze moves only for a delivered error, while OutTimeExcds on
	// strongSwan shows the error was generated at all.
	icmpDelivered = "Icmp.InTimeExcds"
	icmpGenerated = "Icmp.OutTimeExcds"
	// icmpSecondRouteTable is a second table the route back to Ze's inner prefix goes
	// in, so the step does not depend on what that table holds when it runs.
	icmpSecondRouteTable = "220"
	// ipWordTable is the ip-route(8) keyword that names a routing table, and
	// ipWordVia the one that names a gateway.
	ipWordTable = "table"
	ipWordVia   = "via"
	// wordAdd is the verb ip-xfrm(8) and nft(8) both take to add an object.
	wordAdd = "add"
)

var reqidPattern = regexp.MustCompile(`\breqid (\d+)\b`)

// checkICMPErrorPayloadCheck proves the inbound ICMP error processing of RFC 4301
// Section 6.2 for an error addressed to Ze itself.
//
// RFC 4301 Section 6.2: "If an IPsec implementation receives an inbound ICMP error
// message on an SA, and the IP and ICMP headers of the message do not match the
// traffic selectors for the SA, the receiver MUST process the received message in a
// special fashion."
//
// strongSwan's host is the router that generates both errors: with forwarding on and
// Ze's inner prefix routed from 10.9.9.9, an echo request from Ze with TTL 1 draws a
// Time Exceeded from 10.9.9.9, an address no selector covers. Both errors ride the
// Child SA into Ze, and Ze's inbound ESP byte counter shows they did.
//
//   - The consistent error embeds Ze's echo 10.2.0.1 -> 10.1.0.77, which crossed the
//     SA; reversed, it matches the SA's selectors. Ze must deliver it: Icmp.InTimeExcds
//     moves and ping prints the error from 10.9.9.9.
//   - The inconsistent error embeds Ze's echo 10.2.0.1 -> 10.7.0.5, which left in the
//     clear by ze.conf's outbound bypass. No policy charon negotiates carries it, so the
//     checker adds a strongSwan out policy for 10.9.9.9 bound to the SA's reqid. Ze
//     must drop it: the reversed header meets the catch-all discard, and
//     Icmp.InTimeExcds does not move.
//
// Red proof, consistent half: remove `state.ICMP = true` in xfrmStateFromParams
// (internal/component/ike/dataplane/xfrm_linux.go) and rebuild the Ze image (the
// harness does it). icmp_rcv then drops the error at the failed outer policy check,
// the ESP octets still arrive, and InTimeExcds stays still. Inconsistent half: widen
// the inbound protect policy's source selector to 0.0.0.0/0 in xfrmPolicyFromParams;
// the outer check then passes and the error is delivered.
//
// A forwarded error (Ze as a security gateway in front of 10.2.0.0/24) is out of
// scope: Linux performs no payload check on the forward path (LL-XFRM-ICMP-TRANSIT,
// docs/architecture/linux-limitations.md).
func checkICMPErrorPayloadCheck(ctx context.Context, lab *scenarioLab) (err error) {
	if err := establish(ctx, lab); err != nil {
		return err
	}
	for _, peer := range []string{zePeer, swanPeer} {
		if _, err := lab.waitXFRM(ctx, peer); err != nil {
			return err
		}
	}
	if err := lab.icmpErrorRoutes(ctx); err != nil {
		return err
	}

	consistent, err := lab.icmpErrorStep(ctx, icmpConsistentTarget)
	if err != nil {
		return err
	}
	if err := consistent.judge(true); err != nil {
		return fmt.Errorf("the error embedding %s -> %s, inside the SA's selectors: %w", icmpZeInner, icmpConsistentTarget, err)
	}

	removePolicy, err := lab.carryErrorSourceOverSA(ctx)
	if err != nil {
		return err
	}
	// The policy is removed whatever the step answers, and a failed removal joins
	// the step's own error rather than replacing it.
	defer func() {
		err = errors.Join(err, removePolicy(ctx))
	}()
	inconsistent, err := lab.icmpErrorStep(ctx, icmpInconsistentTarget)
	if err != nil {
		return err
	}
	if err := inconsistent.judge(false); err != nil {
		return fmt.Errorf("the error embedding %s -> %s, outside the SA's selectors: %w", icmpZeInner, icmpInconsistentTarget, err)
	}
	return nil
}

// icmpErrorRoutes gives Ze its inner address and a route to each target through
// strongSwan, and makes strongSwan the router that answers a TTL-1 echo with a Time
// Exceeded from icmpErrorSource. Every route names its gateway or source explicitly,
// so the step does not depend on the lab network carrying a default route.
func (l *scenarioLab) icmpErrorRoutes(ctx context.Context) error {
	steps := []struct {
		peer    string
		command []string
	}{
		{zePeer, []string{"ip", ipObjectAddress, ipWordReplace, icmpZeInner + "/32", ipArgDev, "lo"}},
		{zePeer, []string{"ip", ipWordRoute, ipWordReplace, icmpConsistentTarget + "/32", ipWordVia, swanIP, ipArgDev, containerInterface, ipWordSrc, icmpZeInner}},
		{zePeer, []string{"ip", ipWordRoute, ipWordReplace, icmpInconsistentTarget + "/32", ipWordVia, swanIP, ipArgDev, containerInterface, ipWordSrc, icmpZeInner}},
		// The return route to the error source keeps a reverse-path filter on Ze
		// from dropping the decapsulated error before icmp_rcv sees it.
		{zePeer, []string{"ip", ipWordRoute, ipWordReplace, icmpErrorSource + "/32", ipWordVia, swanIP, ipArgDev, containerInterface}},
		{swanPeer, []string{"ip", ipObjectAddress, ipWordReplace, icmpErrorSource + "/32", ipArgDev, "lo"}},
		// __icmp_send answers a forwarded packet from source 0, so the route back
		// to Ze's inner prefix supplies the source: 10.9.9.9.
		{swanPeer, []string{"ip", ipWordRoute, ipWordReplace, icmpZePrefix, ipArgDev, containerInterface, ipWordSrc, icmpErrorSource}},
		// The same route in table 220 too (icmpSecondRouteTable).
		{swanPeer, []string{"ip", ipWordRoute, ipWordReplace, icmpZePrefix, ipArgDev, containerInterface, ipWordSrc, icmpErrorSource, ipWordTable, icmpSecondRouteTable}},
		// A forward route for each target, so ip_forward reaches its TTL check.
		{swanPeer, []string{"ip", ipWordRoute, ipWordReplace, icmpConsistentTarget + "/32", ipArgDev, containerInterface}},
		{swanPeer, []string{"ip", ipWordRoute, ipWordReplace, icmpInconsistentTarget + "/32", ipArgDev, containerInterface}},
	}
	// strongSwan forwards Ze's TTL-1 echoes, so it routes. The write is a direct exec
	// call, not a row of the table below, so the derivation in
	// TestOnlyCheckersThatWritePeerSysctlsAreFlagged sees it and holds this scenario's
	// writesPeerSysctl to the truth.
	if output, err := l.exec(ctx, swanPeer, "sysctl", "-w", "net.ipv4.ip_forward=1"); err != nil {
		return fmt.Errorf("on %s, sysctl -w net.ipv4.ip_forward=1: %w: %s", swanPeer, err, output)
	}
	for _, step := range steps {
		if output, err := l.exec(ctx, step.peer, step.command...); err != nil {
			return fmt.Errorf("on %s, %s: %w: %s", step.peer, strings.Join(step.command, " "), err, output)
		}
	}
	return nil
}

// carryErrorSourceOverSA adds strongSwan's out policy for icmpErrorSource toward Ze's
// inner prefix, bound to the reqid of the SA charon installed toward Ze, and answers
// the function that removes it. A tunnel-mode SA carries no state selector
// (kernel_netlink_ipsec.c add_sa installs one for transport and BEET only), so the
// policy resolves to charon's own outbound SA.
func (l *scenarioLab) carryErrorSourceOverSA(ctx context.Context) (func(context.Context) error, error) {
	// strongSwan's OUTBOUND state: the one it encrypts toward Ze with.
	state, err := l.exec(ctx, swanPeer, "ip", xfrmCommand, ipWordState, "list", ipWordSrc, swanIP, xfrmHeaderTarget, zeIP)
	if err != nil {
		return nil, fmt.Errorf("read strongSwan's outbound state %s -> %s: %w: %s", swanIP, zeIP, err, state)
	}
	reqid, err := reqidOf(state)
	if err != nil {
		return nil, fmt.Errorf("strongSwan's outbound state toward ze: %w", err)
	}
	selector := []string{ipWordSrc, icmpErrorSource + "/32", xfrmHeaderTarget, icmpZePrefix, "dir", "out"}
	add := append([]string{"ip", xfrmCommand, ipWordPolicy, wordAdd}, selector...)
	add = append(add, "tmpl", ipWordSrc, swanIP, xfrmHeaderTarget, zeIP, "proto", "esp", "reqid", reqid, "mode", "tunnel")
	if output, err := l.exec(ctx, swanPeer, add...); err != nil {
		return nil, fmt.Errorf("add strongSwan's out policy for %s: %w: %s", icmpErrorSource, err, output)
	}
	remove := func(ctx context.Context) error {
		command := append([]string{"ip", xfrmCommand, ipWordPolicy, "delete"}, selector...)
		if output, err := l.exec(ctx, swanPeer, command...); err != nil {
			return fmt.Errorf("remove strongSwan's out policy for %s: %w: %s", icmpErrorSource, err, output)
		}
		return nil
	}
	return remove, nil
}

// reqidOf answers the reqid an `ip xfrm state` listing of ONE SA prints, as the
// decimal text ip(8) takes back. A listing with none, or with two different ones, is
// an error: a policy bound to the wrong reqid resolves to no SA, and the error would
// leave in the clear.
func reqidOf(state string) (string, error) {
	matches := reqidPattern.FindAllStringSubmatch(state, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("the state prints no reqid: %s", state)
	}
	reqid := matches[0][1]
	if _, err := strconv.ParseUint(reqid, 10, 32); err != nil {
		return "", fmt.Errorf("reqid %q is not a 32-bit number: %w", reqid, err)
	}
	for _, match := range matches[1:] {
		if match[1] != reqid {
			return "", fmt.Errorf("the listing names reqid %s and %s: %s", reqid, match[1], state)
		}
	}
	return reqid, nil
}

// icmpErrorObservation is what one step saw: the errors strongSwan generated, the
// ESP octets Ze decrypted from strongSwan, the errors Ze delivered, and whether ping
// printed an error from icmpErrorSource.
type icmpErrorObservation struct {
	generated uint64
	carried   uint64
	delivered uint64
	printed   bool
	output    string
}

// icmpErrorStep sends icmpProbeCount TTL-1 echo requests from Ze's inner address to
// target and reads the counters on both sides around them.
func (l *scenarioLab) icmpErrorStep(ctx context.Context, target string) (icmpErrorObservation, error) {
	zeBefore, err := l.snmpCounters(ctx, zePeer)
	if err != nil {
		return icmpErrorObservation{}, err
	}
	swanBefore, err := l.snmpCounters(ctx, swanPeer)
	if err != nil {
		return icmpErrorObservation{}, err
	}
	carriedBefore, err := l.zeInboundESPBytes(ctx)
	if err != nil {
		return icmpErrorObservation{}, err
	}

	// ping exits non-zero when no echo is answered, which is every TTL-1 probe, so
	// its error is not a failure; it is kept for the refusal when ping printed
	// nothing at all.
	output, pingErr := l.exec(ctx, zePeer, icmpPingCommand(target)...)
	if strings.TrimSpace(output) == "" {
		if pingErr != nil {
			return icmpErrorObservation{}, fmt.Errorf("ping from %s to %s printed nothing: %w", icmpZeInner, target, pingErr)
		}
		return icmpErrorObservation{}, fmt.Errorf("ping from %s to %s printed nothing", icmpZeInner, target)
	}
	if pingErr != nil {
		var tb textbuf.Buffer
		output = tb.Str(output).Str("\n(exec: ").Str(pingErr.Error()).Byte(')').String()
	}

	zeAfter, err := l.snmpCounters(ctx, zePeer)
	if err != nil {
		return icmpErrorObservation{}, err
	}
	swanAfter, err := l.snmpCounters(ctx, swanPeer)
	if err != nil {
		return icmpErrorObservation{}, err
	}
	carriedAfter, err := l.zeInboundESPBytes(ctx)
	if err != nil {
		return icmpErrorObservation{}, err
	}
	delivered, ok := counterDelta(zeBefore, zeAfter, icmpDelivered)
	if !ok {
		return icmpErrorObservation{}, fmt.Errorf("ze's /proc/net/snmp carries no %s", icmpDelivered)
	}
	generated, ok := counterDelta(swanBefore, swanAfter, icmpGenerated)
	if !ok {
		return icmpErrorObservation{}, fmt.Errorf("strongSwan's /proc/net/snmp carries no %s", icmpGenerated)
	}
	if carriedAfter < carriedBefore {
		return icmpErrorObservation{}, fmt.Errorf("ze's inbound ESP byte counter went back from %d to %d", carriedBefore, carriedAfter)
	}
	return icmpErrorObservation{
		generated: generated,
		carried:   carriedAfter - carriedBefore,
		delivered: delivered,
		printed:   pingPrintedErrorFrom(output, icmpErrorSource),
		output:    output,
	}, nil
}

// judge refuses a step whose error was not generated or did not ride the SA, since
// then nothing about Ze's processing was observed, then requires the delivery the
// caller names.
func (o icmpErrorObservation) judge(wantDelivered bool) error {
	if o.generated == 0 {
		return fmt.Errorf("strongSwan sent no Time Exceeded, so ze was never offered an error: %s", o.output)
	}
	if o.carried == 0 {
		return fmt.Errorf("strongSwan sent %d Time Exceeded but ze decrypted no ESP from it, so the errors did not ride the Child SA: %s", o.generated, o.output)
	}
	if wantDelivered {
		if o.delivered == 0 {
			return fmt.Errorf("ze decrypted %d ESP octets from strongSwan but delivered no Time Exceeded (%s did not move): %s", o.carried, icmpDelivered, o.output)
		}
		if !o.printed {
			return fmt.Errorf("ze delivered %d Time Exceeded but ping printed none from %s: %s", o.delivered, icmpErrorSource, o.output)
		}
		return nil
	}
	if o.delivered != 0 {
		return fmt.Errorf("ze delivered %d Time Exceeded (%s), want the error dropped: %s", o.delivered, icmpDelivered, o.output)
	}
	if o.printed {
		return fmt.Errorf("ping printed an error from %s, want the error dropped: %s", icmpErrorSource, o.output)
	}
	return nil
}

// icmpPingCommand is the argv of one step: TTL 1, so the first router, strongSwan,
// answers each request with a Time Exceeded.
func icmpPingCommand(target string) []string {
	return []string{cmdPing, "-c", strconv.Itoa(icmpProbeCount), "-W", "2", "-t", "1", "-I", icmpZeInner, target}
}

// pingPrintedErrorFrom reports whether iputils ping printed an error line from
// source: "From <source> icmp_seq=<n> Time to live exceeded".
func pingPrintedErrorFrom(output, source string) bool {
	var tb textbuf.Buffer
	prefix := tb.Str("From ").Str(source).Str(" icmp_seq=").String()
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}

// zeInboundESPBytes answers the octets Ze's inbound SAs from strongSwan decrypted.
func (l *scenarioLab) zeInboundESPBytes(ctx context.Context) (uint64, error) {
	counters, err := l.xfrmCounters(ctx, zePeer)
	if err != nil {
		return 0, err
	}
	return directedBytes(counters, swanIP, zeIP)
}

// directedBytes sums the byte counters of every SA from source to target. No such
// SA is an error, never zero, because zero is what the "nothing arrived" refusal
// reads.
func directedBytes(counters map[saKey]uint64, source, target string) (uint64, error) {
	found := false
	var total uint64
	for key, value := range counters {
		if key.source != source {
			continue
		}
		if key.target != target {
			continue
		}
		found = true
		total += value
	}
	if !found {
		return 0, fmt.Errorf("no SA from %s to %s in the XFRM counters", source, target)
	}
	return total, nil
}
