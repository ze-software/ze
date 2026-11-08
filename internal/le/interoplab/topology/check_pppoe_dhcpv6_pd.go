// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_pppoe_dhcpv6_pd.go -- the scenario this checker judges.
// Related: check_pppoe_dhcpv6_pd_lan.go -- its LAN half (Phases 4 and 5).
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
// runs Solicit, Advertise, Request and Reply after the device is up, or the
// Rapid Commit Solicit and Reply followed by the CPE's Renew at T1, which
// accel-ppp.conf's pref-lifetime of 60 puts 30 seconds after the Reply.
const pdWait = 90 * time.Second

// checkPPPoEDHCPv6PD proves spec-home-cpe-8 against accel-ppp and dhcpcd.
// Phases 1 to 3 come first:
// IPv6CP opens and pppoe0 holds a link-local address from a non-zero
// interface identifier while no pppN device holds one (AC-1); pppoe0 runs
// accept_ra 2 with forwarding and autoconf on, routes ::/0 via accel-ppp's
// link-local and holds a SLAAC address from the RA's prefix (AC-5); and the
// DHCPv6 client on pppoe0 asked for IA_PD with a /56 hint and took the /60
// accel-ppp delegated (AC-6, WPD-3): by requesting it after an Advertise, or,
// after a Rapid Commit Reply, by renewing it (delegationVerdict). Phases 4
// and 5 then judge the delegated prefix's use on lan0 (checkDelegatedLAN).
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
	pool := netip.MustParsePrefix(pdDelegatePool)
	var delegated netip.Prefix
	if err := pollUntil(ctx, pdWait, "accel-ppp delegated a /"+strconv.Itoa(pdDelegatedLength)+" to the CPE", func() error {
		log, err := check.Lab.Query(ctx, nodeAccel, []string{commandCat, pdAccelLog}, nil)
		if err != nil {
			return err
		}
		if err := delegationVerdict(log, pool); err != nil {
			return err
		}
		delegated = delegatedPrefix(readDelegationLog(log, pool))
		return nil
	}); err != nil {
		return err
	}
	if err := checkDelegatedLAN(ctx, check, delegated); err != nil {
		return fmt.Errorf("%w\n%s", err, pdLANEvidence(ctx, check))
	}
	return nil
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

// The tokens accel-ppp's dhcpv6_packet_print writes for the messages and the
// option the verdict reads (dhcpv6_packet.c, its message and option names).
const (
	accelRecvSolicit  = "recv [DHCPv6 Solicit"
	accelRecvRequest  = "recv [DHCPv6 Request"
	accelRecvRenew    = "recv [DHCPv6 Renew"
	accelRecvRebind   = "recv [DHCPv6 Rebind"
	accelSendReply    = "send [DHCPv6 Reply"
	accelRapidCommit  = "<Rapid-Commit>"
	accelIAPDOption   = "IA-PD"
	accelIAPrefixWord = "IA-Prefix "
)

// delegationLog is what accel-ppp's DHCPv6 log says about the delegation:
// each prefix is the last one in the delegate pool a message of that kind
// carried, and the zero prefix where none did.
type delegationLog struct {
	// hint: a Solicit carried IA-PD with the hint ::/pdHintLength.
	hint bool
	// rapidSolicit: that Solicit also carried the Rapid Commit option.
	rapidSolicit bool
	// requested: the CPE's Request (the four-message path).
	requested netip.Prefix
	// replied: accel-ppp's latest Reply of any kind.
	replied netip.Prefix
	// rapidReplied: accel-ppp's Reply carrying Rapid Commit, the answer to
	// a Solicit (the two-message path).
	rapidReplied netip.Prefix
	// extended: the CPE's Renew or Rebind after the Rapid Commit Reply.
	extended netip.Prefix
}

// readDelegationLog reads accel-ppp's DHCPv6 log, which dhcpv6_packet_print
// writes one message to a line ("recv [DHCPv6 Solicit XID=... <IA-PD ...
// {IA-Prefix ::/56 ...}>]").
func readDelegationLog(log string, pool netip.Prefix) delegationLog {
	var seen delegationLog
	for line := range strings.SplitSeq(log, "\n") {
		prefix, inPool := iaPrefixIn(line, pool)
		switch {
		case strings.Contains(line, accelRecvSolicit):
			if !strings.Contains(line, accelIAPDOption) {
				continue
			}
			if !strings.Contains(line, accelIAPrefixWord+"::/"+strconv.Itoa(pdHintLength)+" ") {
				continue
			}
			seen.hint = true
			seen.rapidSolicit = seen.rapidSolicit || strings.Contains(line, accelRapidCommit)
		case strings.Contains(line, accelRecvRequest):
			if inPool {
				seen.requested = prefix
			}
		case strings.Contains(line, accelSendReply):
			if !inPool {
				continue
			}
			seen.replied = prefix
			if strings.Contains(line, accelRapidCommit) {
				seen.rapidReplied = prefix
			}
		case strings.Contains(line, accelRecvRenew), strings.Contains(line, accelRecvRebind):
			// Only a Renew or Rebind after the Rapid Commit Reply speaks
			// for what the CPE kept from it.
			if inPool && seen.rapidReplied.IsValid() {
				seen.extended = prefix
			}
		}
	}
	return seen
}

// delegationVerdict reads accel-ppp's DHCPv6 log and accepts a delegation of
// a /pdDelegatedLength from pool that the CPE took although it hinted
// /pdHintLength (RFC 7084 WPD-3). Both paths open with a Solicit carrying
// IA-PD with the /56 hint (RFC 9915 Section 21.22, the unspecified prefix
// with a length).
//
// The four-message path: a Request carrying an IA-Prefix of the delegated
// length from pool, which a client sends only for the prefix an Advertise
// offered, so the different length was taken, and a Reply delegating that
// prefix.
//
// The two-message path, RFC 9915 Section 18.2.1: "A client that wishes to
// use the Rapid Commit two-message exchange includes a Rapid Commit option
// (see Section 21.14) in its Solicit message." Section 18.3.1: "If the
// client has included a Rapid Commit option in the Solicit message and the
// server has been configured to respond with committed assignments of leases
// and other resources, the server responds to the Solicit with a Reply
// message." No Request is sent, so the server's log alone cannot show the CPE
// kept the different length. The CPE's own word is its next Renew or Rebind,
// Section 18.2.4: "The client includes IA Prefix options (see Section 21.22)
// within IA_PD options (see Section 21.21) for the delegated prefixes
// assigned to the IAs." and "The client MUST NOT include addresses and
// prefixes in any IA option that the client did not obtain from the server or
// that are no longer valid (that have a valid lifetime of 0)." So the path is
// accepted only when the Solicit carried Rapid Commit, accel-ppp's Rapid
// Commit Reply delegated a /pdDelegatedLength from pool, and a later Renew or
// Rebind from the CPE names exactly that prefix: a Reply the CPE never
// adopted leaves its Renew carrying the hint, which fails.
func delegationVerdict(log string, pool netip.Prefix) error {
	seen := readDelegationLog(log, pool)
	if !seen.hint {
		return fmt.Errorf("%w:\n%s", errNoSolicitHint, interoplab.OutputExcerpt(log))
	}
	if seen.requested.IsValid() {
		return fourMessageVerdict(seen, log)
	}
	if seen.rapidReplied.IsValid() {
		return rapidCommitVerdict(seen, log)
	}
	return fmt.Errorf("the CPE sent no DHCPv6 Request for a prefix in %s and accel-ppp sent no Rapid Commit Reply delegating one, so it did not take the /%d offered:\n%s", pool, pdDelegatedLength, interoplab.OutputExcerpt(log))
}

// delegatedPrefix answers the prefix the CPE holds once delegationVerdict
// accepted the log: the one it requested on the four-message path, or the one
// accel-ppp's Rapid Commit Reply delegated and the CPE renewed.
func delegatedPrefix(seen delegationLog) netip.Prefix {
	if seen.requested.IsValid() {
		return seen.requested
	}
	return seen.rapidReplied
}

// fourMessageVerdict judges Solicit, Advertise, Request, Reply: the CPE
// requested the delegated length and accel-ppp's Reply delegated that prefix.
func fourMessageVerdict(seen delegationLog, log string) error {
	if seen.requested.Bits() != pdDelegatedLength {
		return fmt.Errorf("the CPE requested %s, want a /%d", seen.requested, pdDelegatedLength)
	}
	if seen.replied != seen.requested {
		return fmt.Errorf("accel-ppp's Reply delegated %v, want the requested %s:\n%s", seen.replied, seen.requested, interoplab.OutputExcerpt(log))
	}
	return nil
}

// rapidCommitVerdict judges the Rapid Commit Solicit and Reply: the Solicit
// asked for it, the Reply delegated the delegated length, and the CPE's later
// Renew or Rebind names that exact prefix, which it includes only when it
// holds the lease.
func rapidCommitVerdict(seen delegationLog, log string) error {
	if !seen.rapidSolicit {
		return fmt.Errorf("accel-ppp sent a Rapid Commit Reply for %s, but no Solicit carrying the /%d hint also carried the Rapid Commit option:\n%s", seen.rapidReplied, pdHintLength, interoplab.OutputExcerpt(log))
	}
	if seen.rapidReplied.Bits() != pdDelegatedLength {
		return fmt.Errorf("accel-ppp's Rapid Commit Reply delegated %s, want a /%d", seen.rapidReplied, pdDelegatedLength)
	}
	if !seen.extended.IsValid() {
		return fmt.Errorf("the CPE sent no DHCPv6 Renew or Rebind naming %s after accel-ppp's Rapid Commit Reply, so nothing shows it adopted the /%d delegated:\n%s", seen.rapidReplied, pdDelegatedLength, interoplab.OutputExcerpt(log))
	}
	if seen.extended != seen.rapidReplied {
		return fmt.Errorf("the CPE's Renew or Rebind named %s, want the %s accel-ppp's Rapid Commit Reply delegated:\n%s", seen.extended, seen.rapidReplied, interoplab.OutputExcerpt(log))
	}
	return nil
}

// iaPrefixIn answers the first "IA-Prefix <prefix>/<length>" of line that
// lies in pool.
func iaPrefixIn(line string, pool netip.Prefix) (netip.Prefix, bool) {
	rest := line
	for {
		_, after, found := strings.Cut(rest, accelIAPrefixWord)
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
