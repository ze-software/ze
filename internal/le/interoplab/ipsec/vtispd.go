// Design: docs/architecture/testing/interop.md -- vti-unmatched-discard-interface-spd and vti-ipv6-tunnel-source-no-cleartext-leak, the interface SPD and the tunnel-source guard
// Related: vtiipv6.go -- vtiIPv6PeerInner, vtiIPv6Carries, parseXFRMBlocks and the evidence reads these reuse
// Related: checkers.go -- scenarioCheckers names both checkers
// Related: helpers.go -- the ESP counter, ping and xfrm reads they compose

package ipsec

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// spdZeInner4 and spdZeInner6 are the addresses ze.conf puts on vti0 in
	// vti-unmatched-discard-interface-spd; spdSwanInner4 and spdSwanInner6 are the ones
	// the checker puts on strongSwan, inside the negotiated selectors.
	spdZeInner4   = "10.255.1.1"
	spdZeInner6   = "fd00:db8:1::1"
	spdSwanInner4 = "10.255.2.2"
	spdSwanInner6 = "fd00:db8:2::2"
	// spdZePrefix4 and spdZePrefix6 are Ze's side of the selectors, which strongSwan
	// routes toward its tunnel.
	spdZePrefix4 = "10.255.1.0/24"
	spdZePrefix6 = "fd00:db8:1::/64"
	// spdOutside4 and spdOutside6 are routed into vti0 by ze.conf and lie outside every
	// selector, so only vti0's own final entry decides them.
	spdOutside4 = "10.255.9.1"
	spdOutside6 = "fd00:db8:9::1"
	// spdOutsideCount is the echo requests sent to each outside address.
	spdOutsideCount = 3
	// spdCatchAllPriority is dataplane.PriorityUnmatched as iproute2 prints it: the
	// final entry of every SPD Ze holds.
	spdCatchAllPriority = "4294967295"
	// spdExemptionPriority is dataplane.PriorityUnmatchedXFRMInterface as iproute2
	// prints it: the if_id 0 selection entries, one above the catch-all.
	spdExemptionPriority = "4294967294"
	// xfrmActionBlock is iproute2's word for XFRM_POLICY_BLOCK after "action".
	xfrmActionBlock = "block"
	// xfrmStatOutPolBlock is the /proc/net/xfrm_stat counter xfrm_lookup_with_ifid
	// raises when a BLOCK policy decides an outbound packet.
	xfrmStatOutPolBlock = "XfrmOutPolBlock"
	// ipWordReplace and ipWordRoute are the iproute2 words the setup steps use.
	ipWordReplace = "replace"
	ipWordRoute   = "route"
	ipWordSrc     = "src"
)

// checkVTIUnmatchedDiscardInterfaceSPD proves the SPD selection function of RFC 4301
// Section 4.4.1 and the interface SPD's final entry against strongSwan, under
// `unmatched discard`:
//
//  1. IPv4 and IPv6 cross the route-based tunnel in both directions, each Ze-initiated
//     round trip counted by all four simplex SAs. Linux looks Ze's outbound packet up
//     at if_id 0 first, where the DISCARD catch-all would drop it; only the selection
//     entries (on vti0's ifindex, and on vti0's IPv6 address for IPv6 host output) hand
//     it to vti0's SPD, where the Child SA policy protects it. A Ze built without them
//     loses every Ze-initiated packet here.
//  2. vti0's SPD at if_id 0x7 holds its own final entry: a BLOCK at the last priority
//     in, out and fwd for both families.
//  3. A packet routed into vti0 outside every selector is dropped inside vti0's SPD:
//     every echo is lost, vti0's tx_errors rises (xfrmi_xmit counts a failed
//     xfrm_lookup_with_ifid, net/xfrm/xfrm_interface_core.c) and XfrmOutPolBlock rises
//     (the BLOCK decided it). A drop at if_id 0 never reaches vti0, so its counter
//     would not move.
//
// strongSwan is only the far end: charon's own policies are at if_id 0 and the replies
// are its kernel's.
func checkVTIUnmatchedDiscardInterfaceSPD(ctx context.Context, lab *scenarioLab) error {
	if err := lab.spdPeerInner(ctx); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	for _, target := range []string{spdSwanInner4, spdSwanInner6} {
		if err := lab.waitRouteThroughVTI(ctx, target); err != nil {
			return err
		}
	}
	for _, probe := range []pingProbe{
		{peer: zePeer, target: spdSwanInner4, source: spdZeInner4},
		{peer: zePeer, target: spdSwanInner6, source: spdZeInner6},
	} {
		message := "Ze to strongSwan " + probe.target + " did not cross vti0 under unmatched discard"
		if err := lab.verifyESPDirectionsToward(ctx, message, probe, espBothDirections); err != nil {
			return fmt.Errorf("%w\n%s", err, lab.vtiIPv6Evidence(ctx))
		}
	}
	for _, probe := range []pingProbe{
		{peer: swanPeer, target: spdZeInner4, source: spdSwanInner4},
		{peer: swanPeer, target: spdZeInner6, source: spdSwanInner6},
	} {
		if err := lab.requireLosslessPingFrom(ctx, probe, spdOutsideCount); err != nil {
			return fmt.Errorf("strongSwan to Ze %s did not cross vti0 under unmatched discard: %w\n%s",
				probe.target, err, lab.vtiIPv6Evidence(ctx))
		}
	}
	policies, err := lab.xfrmPolicy(ctx, zePeer)
	if err != nil {
		return err
	}
	if err := requireInterfaceSPDCatchAll(parseXFRMBlocks(policies), vtiIPv6IfID); err != nil {
		return fmt.Errorf("ze: %w; ip xfrm policy:\n%s", err, interoplab.OutputExcerpt(policies))
	}
	return lab.spdDropsOutsideSelectors(ctx)
}

// spdPeerInner gives strongSwan its inner addresses and the routes that send Ze's side
// of the selectors toward its tunnel, and enables IPv6 on its lab interface, where its
// policy-based tunnel decapsulates.
func (l *scenarioLab) spdPeerInner(ctx context.Context) error {
	for _, key := range vtiIPv6PeerSysctls {
		if _, err := l.exec(ctx, swanPeer, "sysctl", "-w", key+"="+vtiIPv6Enabled); err != nil {
			return fmt.Errorf("set strongSwan's %s: %w (the scenario's scenarioCheckers entry sets writesPeerSysctl)", key, err)
		}
	}
	steps := [][]string{
		{"ip", "-4", ipObjectAddress, ipWordReplace, spdSwanInner4 + "/32", ipArgDev, "lo"},
		{"ip", "-6", ipObjectAddress, ipWordReplace, spdSwanInner6 + "/128", ipArgDev, "lo"},
		{"ip", "-4", ipWordRoute, ipWordReplace, spdZePrefix4, ipArgDev, containerInterface, ipWordSrc, spdSwanInner4},
		{"ip", "-6", ipWordRoute, ipWordReplace, spdZePrefix6, ipArgDev, containerInterface, ipWordSrc, spdSwanInner6},
	}
	for _, step := range steps {
		if _, err := l.exec(ctx, swanPeer, step...); err != nil {
			return fmt.Errorf("strongSwan: %s: %w", strings.Join(step, " "), err)
		}
	}
	return nil
}

// waitRouteThroughVTI waits until Ze's kernel routes target into vti0, the static route
// ze.conf names.
func (l *scenarioLab) waitRouteThroughVTI(ctx context.Context, target string) error {
	var tb textbuf.Buffer
	viaVTI := tb.Str("dev ").Str(vtiIPv6Interface).Byte(' ').String()
	description := tb.Reset().Str("ze route to ").Str(target).Str(" through ").Str(vtiIPv6Interface).String()
	command := []string{"ip", ipWordRoute, "get", target}
	if _, err := l.waitOutput(ctx, zePeer, command, xfrmWaitTimeout, description, func(output string) bool {
		return strings.Contains(output+" ", viaVTI)
	}); err != nil {
		return fmt.Errorf("the static route to %s via %s never reached the kernel: %w", target, vtiIPv6Interface, err)
	}
	return nil
}

// spdDropsOutsideSelectors sends echoes routed into vti0 outside every selector and
// requires vti0's own final entry to drop them: every echo lost, and both vti0's
// tx_errors and XfrmOutPolBlock risen by at least the number sent.
func (l *scenarioLab) spdDropsOutsideSelectors(ctx context.Context) error {
	errorsBefore, err := l.vtiTXErrors(ctx)
	if err != nil {
		return err
	}
	blockedBefore, err := l.xfrmStat(ctx, zePeer, xfrmStatOutPolBlock)
	if err != nil {
		return err
	}
	targets := []string{spdOutside4, spdOutside6}
	for _, target := range targets {
		// No source: the kernel picks vti0's address, and a socket-chosen tunnel
		// address would be steered by the tunnel-source guard's rule instead.
		probe := pingProbe{peer: zePeer, target: target}
		output := l.execQuiet(ctx, zePeer, pingCommand(probe, spdOutsideCount)...)
		loss, err := pingLoss(output)
		if err != nil {
			return fmt.Errorf("ping %s through vti0 %w", target, err)
		}
		if loss != 100 {
			return fmt.Errorf("ping %s, outside every selector, lost %g%%, want every echo dropped by vti0's final entry: %s",
				target, loss, output)
		}
	}
	errorsAfter, err := l.vtiTXErrors(ctx)
	if err != nil {
		return err
	}
	blockedAfter, err := l.xfrmStat(ctx, zePeer, xfrmStatOutPolBlock)
	if err != nil {
		return err
	}
	sent := uint64(spdOutsideCount * len(targets))
	if errorsAfter < errorsBefore+sent {
		return fmt.Errorf("vti0's tx_errors rose from %d to %d for %d echoes outside the selectors, so they were not dropped inside vti0's SPD\n%s",
			errorsBefore, errorsAfter, sent, l.vtiIPv6Evidence(ctx))
	}
	if blockedAfter < blockedBefore+sent {
		return fmt.Errorf("%s rose from %d to %d for %d echoes outside the selectors, so no BLOCK entry decided them\n%s",
			xfrmStatOutPolBlock, blockedBefore, blockedAfter, sent, l.vtiIPv6Evidence(ctx))
	}
	return nil
}

// vtiTXErrors answers vti0's tx_errors counter from sysfs.
func (l *scenarioLab) vtiTXErrors(ctx context.Context) (uint64, error) {
	output, err := l.exec(ctx, zePeer, "cat", "/sys/class/net/"+vtiIPv6Interface+"/statistics/tx_errors")
	if err != nil {
		return 0, fmt.Errorf("read vti0's tx_errors: %w", err)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("vti0's tx_errors %q is not a counter: %w", output, err)
	}
	return value, nil
}

// xfrmStat answers one counter of the peer's /proc/net/xfrm_stat.
func (l *scenarioLab) xfrmStat(ctx context.Context, peer, name string) (uint64, error) {
	output, err := l.exec(ctx, peer, "cat", "/proc/net/xfrm_stat")
	if err != nil {
		return 0, fmt.Errorf("read %s's /proc/net/xfrm_stat: %w", peer, err)
	}
	return parseXFRMStat(output, name)
}

// parseXFRMStat answers the counter name from a /proc/net/xfrm_stat dump: one
// "<name> <value>" line per counter. A missing or unparsable counter is an error.
func parseXFRMStat(output, name string) (uint64, error) {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != name {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("xfrm_stat %s value %q: %w", name, fields[1], err)
		}
		return value, nil
	}
	return 0, fmt.Errorf("xfrm_stat holds no %s counter:\n%s", name, interoplab.OutputExcerpt(output))
}

// requireInterfaceSPDCatchAll refuses unless the SPD at ifID holds its final entry: a
// BLOCK at the last priority for each of in, out and fwd, from and to the wildcard of
// each family.
func requireInterfaceSPDCatchAll(blocks []xfrmBlock, ifID string) error {
	want := map[string]int{}
	for _, wildcard := range []string{"0.0.0.0/0", vtiIPv6Wildcard} {
		for _, direction := range []string{xfrmDirIn, xfrmDirOut, xfrmDirFwd} {
			want[wildcard+" "+direction] = 0
		}
	}
	for i := range blocks {
		block := &blocks[i]
		bound, ok := block.field("if_id")
		if !ok {
			continue
		}
		if bound != ifID {
			continue
		}
		priority, _ := block.field("priority")
		if priority != spdCatchAllPriority {
			continue
		}
		action, _ := block.field("action")
		direction, _ := block.field("dir")
		if action != xfrmActionBlock {
			return fmt.Errorf("the final %s entry of the SPD at if_id %s acts %q, want %s", direction, ifID, action, xfrmActionBlock)
		}
		if block.source != block.target {
			return fmt.Errorf("the final %s entry of the SPD at if_id %s selects src %s dst %s, want one wildcard", direction, ifID, block.source, block.target)
		}
		key := block.source + " " + direction
		if _, known := want[key]; !known {
			return fmt.Errorf("the SPD at if_id %s holds an unexpected final entry %s", ifID, key)
		}
		want[key]++
	}
	for key, count := range want {
		if count != 1 {
			return fmt.Errorf("the SPD at if_id %s holds %d final entries %s, want 1", ifID, count, key)
		}
	}
	return nil
}

const (
	// leakWANDestination is the WAN-only destination: an address strongSwan holds on
	// lo, reached from Ze out of eth0 and never through the tunnel.
	leakWANDestination = "2001:db8:99::1"
	// leakWANPrefix is Ze's route to it.
	leakWANPrefix = "2001:db8:99::/64"
	// leakZeWAN and leakSwanWAN are the IPv6 addresses of the two lab interfaces on
	// leakWANLink, so Ze has a next hop for leakWANPrefix. Docker numbers them at the
	// peers' host numbers (2 and 3); the checker asserts them again, with nodad.
	leakZeWAN   = "fd00:db8:a::2"
	leakSwanWAN = "fd00:db8:a::3"
	// leakProbeCount is the echoes each probe sends.
	leakProbeCount = 3
	// leakEthIPv6 is the sysctl that says whether IPv6 is on for Ze's lab interface,
	// which Docker disables on a network with no IPv6 and leaves on for this
	// scenario's network (networkIPv6, leakWANLink); the ip6_output drop a disabled
	// eth0 causes would hide a leak behind a dead interface.
	leakEthIPv6 = "net.ipv6.conf.eth0.disable_ipv6"
)

// leakWANLink is the IPv6 /64 the scenario's lab network carries (networkIPv6), so
// Docker leaves IPv6 on for both lab interfaces and numbers them leakZeWAN and
// leakSwanWAN.
var leakWANLink = netip.MustParsePrefix("fd00:db8:a::/64")

// checkVTIIPv6TunnelSourceNoCleartextLeak proves, from strongSwan's side of the WAN,
// that IPv6 from vti0's tunnel address toward a WAN-only destination never leaves Ze
// in the clear, under `unmatched discard`, for both ways the source is chosen:
//
//  1. The tunnel still carries IPv6 from the tunnel address (vtiIPv6Carries).
//  2. The leak path exists: Ze holds the if_id 0 source BYPASS for fd00:db8:1::1, the
//     guard's rule from that address is in place, and Ze's kernel routes the WAN-only
//     destination out of eth0 choosing fd00:db8:1::1 as the source.
//  3. Positive control: ICMPv6 from eth0's own address to the same destination, passed
//     by the `wan-control` bypass, reaches strongSwan's counter, so a clear packet
//     from Ze to that destination is seen when one is sent.
//  4. Echoes from the tunnel address, socket-chosen (ping -I, steered by the guard's
//     rule into a table that does not reach the destination) and kernel-chosen (no
//     source, dropped by the guard's postrouting term), never reach strongSwan's
//     counter.
//
// The WAN observer is an ip6tables INPUT rule in strongSwan's namespace; strongSwan's
// daemon sends and answers nothing new here.
func checkVTIIPv6TunnelSourceNoCleartextLeak(ctx context.Context, lab *scenarioLab) error {
	if err := lab.vtiIPv6PeerInner(ctx); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	if err := lab.leakWANPath(ctx); err != nil {
		return err
	}
	if err := lab.vtiIPv6Carries(ctx, "IPv6 from the tunnel address did not cross vti0 under the guard"); err != nil {
		return err
	}
	if err := lab.leakPathExists(ctx); err != nil {
		return err
	}

	controlProbe := pingProbe{peer: zePeer, target: leakWANDestination, source: leakZeWAN}
	lab.execQuiet(ctx, zePeer, pingCommand(controlProbe, leakProbeCount)...)
	control, err := lab.wanCounter(ctx, leakZeWAN)
	if err != nil {
		return err
	}
	if control < leakProbeCount {
		return fmt.Errorf("the WAN counter saw %d of %d clear echoes from eth0's own address %s, so it cannot show a leak\n%s",
			control, leakProbeCount, leakZeWAN, lab.vtiIPv6Evidence(ctx))
	}

	for _, probe := range []pingProbe{
		{peer: zePeer, target: leakWANDestination, source: vtiIPv6ZeInner},
		{peer: zePeer, target: leakWANDestination},
	} {
		lab.execQuiet(ctx, zePeer, pingCommand(probe, leakProbeCount)...)
	}
	leaked, err := lab.wanCounter(ctx, vtiIPv6ZeInner)
	if err != nil {
		return err
	}
	if leaked != 0 {
		return fmt.Errorf("strongSwan received %d clear IPv6 packets from the tunnel address %s on the WAN\n%s",
			leaked, vtiIPv6ZeInner, lab.vtiIPv6Evidence(ctx))
	}
	return nil
}

// leakWANPath builds the WAN path for the WAN-only destination: IPv6 on both lab
// interfaces, a static neighbor so Ze needs no neighbor discovery (which the DISCARD
// catch-all would drop), Ze's route to the destination with the tunnel address as its
// preferred source, and strongSwan's two INPUT counters.
func (l *scenarioLab) leakWANPath(ctx context.Context) error {
	// The lab network carries IPv6 (leakWANLink), so Docker left it on; read it back,
	// because a dead eth0 would make the zero count below prove nothing.
	enabled, err := l.exec(ctx, zePeer, "sysctl", "-n", leakEthIPv6)
	if err != nil {
		return fmt.Errorf("read Ze's %s: %w", leakEthIPv6, err)
	}
	if strings.TrimSpace(enabled) != vtiIPv6Enabled {
		return fmt.Errorf("ze's %s is %q, want %s: the scenario's lab network (networkIPv6 %s) did not leave IPv6 on for eth0, so no leak could be seen",
			leakEthIPv6, strings.TrimSpace(enabled), vtiIPv6Enabled, leakWANLink)
	}
	mac, err := l.exec(ctx, swanPeer, "cat", "/sys/class/net/"+containerInterface+"/address")
	if err != nil {
		return fmt.Errorf("read strongSwan's lab MAC address: %w", err)
	}
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return errors.New("strongSwan's lab interface reports no MAC address")
	}
	steps := []struct {
		peer    string
		command []string
	}{
		{zePeer, []string{"ip", "-6", ipObjectAddress, ipWordReplace, leakZeWAN + "/64", ipArgDev, containerInterface, "nodad"}},
		{swanPeer, []string{"ip", "-6", ipObjectAddress, ipWordReplace, leakSwanWAN + "/64", ipArgDev, containerInterface, "nodad"}},
		{swanPeer, []string{"ip", "-6", ipObjectAddress, ipWordReplace, leakWANDestination + "/128", ipArgDev, "lo"}},
		{zePeer, []string{"ip", "-6", "neigh", ipWordReplace, leakSwanWAN, "lladdr", mac, ipArgDev, containerInterface, "nud", "permanent"}},
		{zePeer, []string{"ip", "-6", ipWordRoute, ipWordReplace, leakWANPrefix, "via", leakSwanWAN, ipArgDev, containerInterface, ipWordSrc, vtiIPv6ZeInner}},
		{swanPeer, []string{"ip6tables", "-I", netfilterChainInput, "-s", vtiIPv6ZeInner + "/128", "-d", leakWANDestination + "/128", "-j", "ACCEPT"}},
		{swanPeer, []string{"ip6tables", "-I", netfilterChainInput, "-s", leakZeWAN + "/128", "-d", leakWANDestination + "/128", "-j", "ACCEPT"}},
	}
	for _, step := range steps {
		if _, err := l.exec(ctx, step.peer, step.command...); err != nil {
			return fmt.Errorf("%s: %s: %w", step.peer, strings.Join(step.command, " "), err)
		}
	}
	return nil
}

// leakPathExists refuses unless every part of the leak the guard closes is in place,
// so a zero count below cannot come from a missing path: the if_id 0 source BYPASS
// for the tunnel address, the guard's rule from it, and the kernel's choice of eth0
// and the tunnel address for the WAN-only destination.
func (l *scenarioLab) leakPathExists(ctx context.Context) error {
	var tb textbuf.Buffer
	ruleFrom := tb.Str("from ").Str(vtiIPv6ZeInner).Str(" lookup ").String()
	if _, err := l.waitOutput(ctx, zePeer, []string{"ip", "-6", "rule", ipWordShow}, xfrmWaitTimeout,
		"the tunnel-source guard's rule", func(output string) bool {
			return strings.Contains(output, ruleFrom)
		}); err != nil {
		return fmt.Errorf("ze installed no tunnel-source rule for %s: %w\n%s", vtiIPv6ZeInner, err, l.vtiIPv6Evidence(ctx))
	}
	policies, err := l.xfrmPolicy(ctx, zePeer)
	if err != nil {
		return err
	}
	if !holdsSourceExemption(parseXFRMBlocks(policies), vtiIPv6ZeInner+"/128") {
		return fmt.Errorf("ze holds no if_id 0 source BYPASS for %s, so this run cannot show the leak the guard closes; ip xfrm policy:\n%s",
			vtiIPv6ZeInner, interoplab.OutputExcerpt(policies))
	}
	route, err := l.exec(ctx, zePeer, "ip", "-6", ipWordRoute, "get", leakWANDestination)
	if err != nil {
		return fmt.Errorf("ze route to %s: %w", leakWANDestination, err)
	}
	if !strings.Contains(route+" ", "dev "+containerInterface+" ") {
		return fmt.Errorf("ze routes %s out of another device than %s: %s", leakWANDestination, containerInterface, route)
	}
	if !strings.Contains(route+" ", "src "+vtiIPv6ZeInner+" ") {
		return fmt.Errorf("ze does not choose the tunnel address %s as the source toward %s: %s", vtiIPv6ZeInner, leakWANDestination, route)
	}
	return nil
}

// holdsSourceExemption reports whether the dump holds the out entry at if_id 0, one
// place above the catch-all, whose source is source and whose destination is any.
func holdsSourceExemption(blocks []xfrmBlock, source string) bool {
	for i := range blocks {
		block := &blocks[i]
		if _, bound := block.field("if_id"); bound {
			continue
		}
		if block.source != source {
			continue
		}
		if block.target != vtiIPv6Wildcard {
			continue
		}
		direction, _ := block.field("dir")
		if direction != xfrmDirOut {
			continue
		}
		priority, _ := block.field("priority")
		if priority == spdExemptionPriority {
			return true
		}
	}
	return false
}

// wanCounter answers the packets strongSwan's INPUT counter for source has seen
// toward the WAN-only destination, read from `ip6tables -v -S INPUT`, which prints
// each rule with "-c <packets> <bytes>".
func (l *scenarioLab) wanCounter(ctx context.Context, source string) (uint64, error) {
	output, err := l.exec(ctx, swanPeer, "ip6tables", "-v", "-S", netfilterChainInput)
	if err != nil {
		return 0, fmt.Errorf("read strongSwan's WAN counters: %w", err)
	}
	return parseWANCounter(output, source+"/128", leakWANDestination+"/128")
}

// parseWANCounter answers the packet count of the rule matching source and target in
// an `ip6tables -v -S` dump. A missing rule or count is an error, never zero.
func parseWANCounter(output, source, target string) (uint64, error) {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if !hasFlagValue(fields, "-s", source) {
			continue
		}
		if !hasFlagValue(fields, "-d", target) {
			continue
		}
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] != "-c" {
				continue
			}
			value, err := strconv.ParseUint(fields[i+1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("WAN counter for %s: %q is not a count: %w", source, fields[i+1], err)
			}
			return value, nil
		}
		return 0, fmt.Errorf("the WAN rule for %s prints no counter: %s", source, line)
	}
	return 0, fmt.Errorf("strongSwan holds no WAN rule for %s -> %s:\n%s", source, target, interoplab.OutputExcerpt(output))
}

// hasFlagValue reports whether fields hold flag immediately followed by value.
func hasFlagValue(fields []string, flag, value string) bool {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != flag {
			continue
		}
		if fields[i+1] == value {
			return true
		}
	}
	return false
}
