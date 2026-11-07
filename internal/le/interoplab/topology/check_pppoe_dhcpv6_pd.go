// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_pppoe_dhcpv6_pd.go -- the scenario this checker judges.
package topology

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// pdWait bounds each IPv6 state poll: IPv6CP opens with IPCP, the first RA
// arrives within accel-ppp's initial advertisement interval, and DHCPv6
// runs Solicit, Advertise, Request and Reply after the device is up.
const pdWait = 90 * time.Second

// checkPPPoEDHCPv6PD proves spec-home-cpe-8 Phases 1 to 3 against accel-ppp:
// IPv6CP opens and pppoe0 holds a link-local address from a non-zero
// interface identifier while no pppN device holds one (AC-1); pppoe0 runs
// accept_ra 2 with forwarding and autoconf on, routes ::/0 via accel-ppp's
// link-local and holds a SLAAC address from the RA's prefix (AC-5); and the
// DHCPv6 client on pppoe0 asked for IA_PD with a /56 hint and requested the
// /60 accel-ppp offered, which accel-ppp then delegated (AC-6, WPD-3).
func checkPPPoEDHCPv6PD(ctx context.Context, check *interoplab.CheckContext) error {
	if err := pollUntil(ctx, sessionUpTimeout, cpeDevice+" holds a link-local address", func() error {
		answer, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "-6", ipWordAddress, wordShow}, nil)
		if err != nil {
			return err
		}
		return linkLocalVerdict(answer, cpeDevice)
	}); err != nil {
		return err
	}
	if err := checkWANSysctls(ctx, check); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, "::/0 via "+pdAccelLinkLocal+" on "+cpeDevice, func() error {
		answer, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-6", ipWordRoute, wordShow, "default"}, nil)
		if err != nil {
			return err
		}
		return defaultRouterVerdict(answer, pdAccelLinkLocal, cpeDevice)
	}); err != nil {
		return err
	}
	if err := pollUntil(ctx, pdWait, cpeDevice+" holds a SLAAC address in "+pdSLAACPool, func() error {
		answer, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "-6", ipWordAddress, wordShow, ipWordDev, cpeDevice, "scope", "global"}, nil)
		if err != nil {
			return err
		}
		return slaacVerdict(answer, netip.MustParsePrefix(pdSLAACPool))
	}); err != nil {
		return err
	}
	return pollUntil(ctx, pdWait, "accel-ppp delegated a /"+strconv.Itoa(pdDelegatedLength)+" to the CPE", func() error {
		log, err := check.Lab.Query(ctx, nodeAccel, []string{commandCat, pdAccelLog}, nil)
		if err != nil {
			return err
		}
		return delegationVerdict(log, netip.MustParsePrefix(pdDelegatePool))
	})
}

// checkWANSysctls waits for the three per-device sysctls the ipv6 container
// writes to pppoe0: an RA is accepted with forwarding on only at accept_ra 2.
// The client hands each write to the sysctl component over the EventBus when
// the session device appears, so the write lands after the device exists and
// a single read can precede it: each value is polled, never read once.
func checkWANSysctls(ctx context.Context, check *interoplab.CheckContext) error {
	for _, want := range []struct{ name, value string }{{"accept_ra", "2"}, {"forwarding", "1"}, {"autoconf", "1"}} {
		if err := pollUntil(ctx, sessionUpTimeout, "ze-cpe's "+cpeDevice+" "+want.name+" is "+want.value, func() error {
			answer, err := check.Lab.Query(ctx, nodeCPE, []string{commandCat, "/proc/sys/net/ipv6/conf/" + cpeDevice + "/" + want.name}, nil)
			if err != nil {
				return fmt.Errorf("read ze-cpe's %s %s: %w", cpeDevice, want.name, err)
			}
			if strings.TrimSpace(answer) != want.value {
				return fmt.Errorf("ze-cpe's %s %s is %q, want %s", cpeDevice, want.name, strings.TrimSpace(answer), want.value)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// linkLocalVerdict reads `ip -o -6 address show` and accepts only
// one fe80::/64 address on device, from a non-zero interface identifier, and
// none on a kernel-named pppN device (the client's device carries its
// configured name, spec-home-cpe-1).
func linkLocalVerdict(answer, device string) error {
	var held []netip.Prefix
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		name := fields[1]
		prefix, err := netip.ParsePrefix(fields[3])
		if err != nil {
			continue
		}
		if !prefix.Addr().IsLinkLocalUnicast() {
			continue
		}
		if kernelPPPName(name) {
			return fmt.Errorf("kernel device %s holds link-local %s, so the session device does not carry the configured name %s", name, prefix, device)
		}
		if name == device {
			held = append(held, prefix)
		}
	}
	if len(held) != 1 {
		return fmt.Errorf("%s holds %d link-local addresses, want 1 from IPv6CP:\n%s", device, len(held), answer)
	}
	if held[0].Bits() != 64 {
		return fmt.Errorf("%s's link-local %s is not a /64", device, held[0])
	}
	identifier := held[0].Addr().As16()
	if [8]byte(identifier[8:]) == [8]byte{} {
		return fmt.Errorf("%s's link-local %s has a zero interface identifier, which RFC 5072 Section 4.1 forbids", device, held[0])
	}
	return nil
}

// kernelPPPName says whether name is a kernel-named PPP device: ppp then
// digits only.
func kernelPPPName(name string) bool {
	digits, found := strings.CutPrefix(name, "ppp")
	if !found {
		return false
	}
	if digits == "" {
		return false
	}
	_, err := strconv.ParseUint(digits, 10, 32)
	return err == nil
}

// defaultRouterVerdict reads `ip -6 route show default` and accepts a default
// route via router on device, the one Router Discovery installs.
func defaultRouterVerdict(answer, router, device string) error {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if fields[0] != "default" {
			continue
		}
		if strings.Join(fields[1:5], " ") == "via "+router+" "+ipWordDev+" "+device {
			return nil
		}
	}
	return fmt.Errorf("no IPv6 default route via %s dev %s:\n%s", router, device, answer)
}

// slaacVerdict reads `ip -o -6 address show dev <device> scope global` and
// accepts an address inside pool, which only the RA's prefix can give it.
func slaacVerdict(answer string, pool netip.Prefix) error {
	for line := range strings.SplitSeq(answer, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		prefix, err := netip.ParsePrefix(fields[3])
		if err != nil {
			continue
		}
		if pool.Contains(prefix.Addr()) && prefix.Bits() == 64 {
			return nil
		}
	}
	return fmt.Errorf("no global /64 address in %s:\n%s", pool, answer)
}

// errNoSolicitHint says the CPE's Solicit carried no IA_PD with the length hint.
var errNoSolicitHint = errors.New("accel-ppp received no DHCPv6 Solicit carrying IA-PD with the hint IA-Prefix ::/" + strconv.Itoa(pdHintLength))

// delegationVerdict reads accel-ppp's DHCPv6 log, which dhcpv6_packet_print
// writes one message to a line ("recv [DHCPv6 Solicit XID=... <IA-PD ...
// <IA-Prefix ::/56 ...>>]"). It accepts a Solicit carrying IA-PD with the
// /56 hint (RFC 9915 Section 21.22, the unspecified prefix with a length), a
// Request carrying an IA-Prefix of the delegated length from pool, which a
// client sends only for the prefix an Advertise offered, so the different
// length was taken (RFC 7084 WPD-3), and a Reply delegating that prefix.
func delegationVerdict(log string, pool netip.Prefix) error {
	hint := false
	var requested, replied netip.Prefix
	for line := range strings.SplitSeq(log, "\n") {
		switch {
		case strings.Contains(line, "recv [DHCPv6 Solicit"):
			if strings.Contains(line, "IA-PD") && strings.Contains(line, "IA-Prefix ::/"+strconv.Itoa(pdHintLength)+" ") {
				hint = true
			}
		case strings.Contains(line, "recv [DHCPv6 Request"):
			if prefix, ok := iaPrefixIn(line, pool); ok {
				requested = prefix
			}
		case strings.Contains(line, "send [DHCPv6 Reply"):
			if prefix, ok := iaPrefixIn(line, pool); ok {
				replied = prefix
			}
		}
	}
	if !hint {
		return fmt.Errorf("%w:\n%s", errNoSolicitHint, interoplab.OutputExcerpt(log))
	}
	if !requested.IsValid() {
		return fmt.Errorf("the CPE sent no DHCPv6 Request for a prefix in %s, so it did not take the /%d offered:\n%s", pool, pdDelegatedLength, interoplab.OutputExcerpt(log))
	}
	if requested.Bits() != pdDelegatedLength {
		return fmt.Errorf("the CPE requested %s, want a /%d", requested, pdDelegatedLength)
	}
	if replied != requested {
		return fmt.Errorf("accel-ppp's Reply delegated %v, want the requested %s:\n%s", replied, requested, interoplab.OutputExcerpt(log))
	}
	return nil
}

// iaPrefixIn answers the first "IA-Prefix <prefix>/<length>" of line that
// lies in pool.
func iaPrefixIn(line string, pool netip.Prefix) (netip.Prefix, bool) {
	rest := line
	for {
		_, after, found := strings.Cut(rest, "IA-Prefix ")
		if !found {
			return netip.Prefix{}, false
		}
		rest = after
		text, _, _ := strings.Cut(after, " ")
		prefix, err := netip.ParsePrefix(text)
		if err != nil {
			continue
		}
		if pool.Contains(prefix.Addr()) {
			return prefix, true
		}
	}
}
