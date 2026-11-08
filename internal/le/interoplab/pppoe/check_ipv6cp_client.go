// Design: docs/labs/pppoe-interop.md -- pppoe-ipv6cp-ze-client and
// pppoe-ipv6cp-rejected-ze-client: the Ze PPPoE client's IPv6CP against a
// real accel-ppp access concentrator.
// RFC: rfc/short/rfc5072.md -- Section 4.1 Interface-Identifier negotiation
// and the link-local address formed from it.
// Related: check_client.go -- the shared IPv4 session, CHAP and resolver proof
// both checkers end with.
// Related: scenarios.go -- the ipv6-session marker that keeps IPv6 on the PPP
// devices and lets the client write its session sysctls.
package pppoe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// accelIPv6PeerID is the identifier accel-ppp's ipv6-peer-intf-id assigns
	// Ze, spelled as accel-ppp 1.14.0 prints it (ppp/ipv6cp_opt_intfid.c,
	// ipaddr_print: "<addr %x:%x:%x:%x>").
	accelIPv6PeerID = "0:0:0:5a"
	// zeIPv6LinkLocal is fe80::/64 plus accelIPv6PeerID, the address Ze owes
	// the session device once IPv6CP opens with that identifier.
	zeIPv6LinkLocal = "fe80::5a/64"
	// accelIPv6LinkLocal is fe80::/64 plus accel-ppp's own ipv6-intf-id
	// (0:0:0:1), the address accel-ppp sets on its PPP device
	// (ifcfg.c, ap_session_ifup) and sends its Router Advertisements from.
	accelIPv6LinkLocal = "fe80::1"
	// accelIPv6PrefixText opens the global address SLAAC forms from the /64
	// accel-ppp's ipv6pool hands the session out of 2001:db8:100::/56.
	accelIPv6PrefixText = "inet6 2001:db8:100::"
	// ipv6cpProtocolText is how accel-ppp prints IPv6CP's protocol number in a
	// Protocol-Reject (ppp/ppp_lcp.c, lcp_send_proto_rej: "<%04x>").
	ipv6cpProtocolText = "<8057>"
	// rejectedPings is the count of one-second pings the rejected scenario
	// sends after the Protocol-Reject before it counts them: past the
	// client's 3 s IPv6CP Restart timer, so a client that kept sending IPv6CP
	// would have drawn a second Protocol-Reject.
	rejectedPings = "5"
	// commandAddr and commandDefault are ip(8) words the IPv6 checks use.
	commandAddr    = "addr"
	commandDefault = "default"
)

// checkZeClientIPv6CP proves the Ze client's IPv6CP against an accel-ppp that
// Naks every identifier but its own ipv6-peer-intf-id: Ze sends one non-zero
// Interface-Identifier, adopts the suggestion (RFC 5072 Section 4.1), opens
// IPv6CP, holds the link-local address on pppoe0, accepts the access
// concentrator's Router Advertisements with forwarding on, installs the
// default route through it, forms a SLAAC address from the advertised prefix,
// and pings accel-ppp over IPv6. The IPv4 session, CHAP and teardown proof
// follows, unchanged.
func checkZeClientIPv6CP(ctx context.Context, check *interoplab.CheckContext) error {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE Ze-client IPv6CP checker has no lab")
	}
	if err := checkZeClientIPv6(ctx, check.Lab); err != nil {
		return appendDiagnostics(ctx, check.Lab, err, zeImageName, accelImageName)
	}
	return checkZeClientCHAPHostFallback(ctx, check)
}

// checkZeClientIPv6 is the IPv6 half of checkZeClientIPv6CP, run while the
// session is up.
func checkZeClientIPv6(ctx context.Context, lab interoplab.CheckerLab) error {
	if err := requireNamedPPPDevice(ctx, lab); err != nil {
		return err
	}
	trace, err := waitAccelTrace(ctx, lab, "the IPv6CP Nak adoption in accel-ppp's trace", accelIPv6CPNakAdoption)
	if err != nil {
		return fmt.Errorf("IPv6CP never opened on the identifier accel-ppp suggested: %w: %w", err, accelIPv6CPNakAdoption(trace))
	}
	if err := waitZeOutput(ctx, lab, "the IPv6CP link-local address on "+namedDevice,
		[]string{"ip", "-6", "-o", commandAddr, commandShow, commandDev, namedDevice, "scope", "link"},
		func(output string) bool { return strings.Contains(output, "inet6 "+zeIPv6LinkLocal) },
	); err != nil {
		return fmt.Errorf("%s does not hold %s, the link-local address of the negotiated identifier: %w", namedDevice, zeIPv6LinkLocal, err)
	}
	for _, sysctl := range [...]struct{ name, want string }{{"accept_ra", "2"}, {"forwarding", "1"}} {
		path := "/proc/sys/net/ipv6/conf/" + namedDevice + "/" + sysctl.name
		if err := waitZeOutput(ctx, lab, path, []string{"cat", path},
			func(output string) bool { return strings.TrimSpace(output) == sysctl.want },
		); err != nil {
			return fmt.Errorf("%s is not %s on the session device: %w", path, sysctl.want, err)
		}
	}
	if err := waitZeOutput(ctx, lab, "the RA default route through "+namedDevice,
		[]string{"ip", "-6", "-o", commandRoute, commandShow, commandDefault, commandDev, namedDevice},
		func(output string) bool { return strings.Contains(output, "via "+accelIPv6LinkLocal+" ") },
	); err != nil {
		return fmt.Errorf("no IPv6 default route via %s on %s from accel-ppp's Router Advertisements: %w", accelIPv6LinkLocal, namedDevice, err)
	}
	if err := waitZeOutput(ctx, lab, "the SLAAC address on "+namedDevice,
		[]string{"ip", "-6", "-o", commandAddr, commandShow, commandDev, namedDevice, "scope", "global"},
		func(output string) bool {
			return strings.Contains(output, accelIPv6PrefixText) && strings.Contains(output, "/64 ")
		},
	); err != nil {
		return fmt.Errorf("%s formed no SLAAC address from the autonomous prefix accel-ppp advertised: %w", namedDevice, err)
	}
	if err := waitZeOutput(ctx, lab, "an IPv6 ping to accel-ppp across the session",
		[]string{pingCommand, "-6", "-c", "3", "-W", "3", "-I", namedDevice, accelIPv6LinkLocal},
		func(string) bool { return true },
	); err != nil {
		return fmt.Errorf("ze cannot ping %s over IPv6 through %s: %w", accelIPv6LinkLocal, namedDevice, err)
	}
	return nil
}

// checkZeClientIPv6CPRejected proves RFC 7084 WLL-3 against an accel-ppp with
// IPv6 denied, which answers IPv6CP with an LCP Protocol-Reject
// (ppp/ppp_ipv6cp.c, ipv6cp_recv): the client stops sending IPv6CP (RFC 1661
// Section 5.7), installs no link-local address, and the IPv4 session stays up
// with no LCP termination and no second session.
func checkZeClientIPv6CPRejected(ctx context.Context, check *interoplab.CheckContext) error {
	if check == nil || check.Lab == nil {
		return errors.New("PPPoE Ze-client IPv6CP-rejected checker has no lab")
	}
	if err := checkZeClientIPv6Rejected(ctx, check.Lab); err != nil {
		return appendDiagnostics(ctx, check.Lab, err, zeImageName, accelImageName)
	}
	return checkZeClientCHAPHostFallback(ctx, check)
}

// checkZeClientIPv6Rejected is the IPv6 half of checkZeClientIPv6CPRejected,
// run while the session is up.
func checkZeClientIPv6Rejected(ctx context.Context, lab interoplab.CheckerLab) error {
	if err := requireNamedPPPDevice(ctx, lab); err != nil {
		return err
	}
	_, err := waitAccelTrace(ctx, lab, "accel-ppp's Protocol-Reject of IPv6CP", func(trace string) error {
		if !hasLineWith(trace, "send [LCP ProtoRej ", ipv6cpProtocolText) {
			return errors.New("accel-ppp sent no Protocol-Reject for IPv6CP")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ze never offered IPv6CP to an access concentrator that denies it: %w", err)
	}
	// The ping both proves IPv4 after the reject and outlasts the client's
	// Restart timer before the trace is counted.
	result, err := lab.Exec(ctx, zeImageName, []string{
		pingCommand, "-c", rejectedPings, "-W", "3", accelGateway,
	}, nil)
	if err != nil {
		return fmt.Errorf("ping %s after the IPv6CP Protocol-Reject: %w", accelGateway, err)
	}
	if result.ExitCode != 0 {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("IPv4 stopped after accel-ppp rejected IPv6CP: ping %s exited %d", accelGateway, result.ExitCode),
			result.Stdout+result.Stderr)
	}
	logs, err := lab.Logs(ctx, accelImageName, accelTraceLines)
	if err != nil {
		return fmt.Errorf("read accel-ppp trace: %w", err)
	}
	if !logs.Available {
		return errors.New("accel-ppp trace unavailable: the IPv6CP Protocol-Reject outcome was not measured")
	}
	if err := accelIPv6CPRejectedEvidence(logs.Text); err != nil {
		return err
	}
	addresses, err := lab.Exec(ctx, zeImageName,
		[]string{"ip", "-6", "-o", commandAddr, commandShow, commandDev, namedDevice}, nil)
	if err != nil {
		return fmt.Errorf("read %s IPv6 addresses: %w", namedDevice, err)
	}
	if addresses.ExitCode != 0 {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("ip -6 addr show dev %s exited %d", namedDevice, addresses.ExitCode), addresses.Stderr)
	}
	if strings.Contains(addresses.Stdout, "fe80:") {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("%s holds a link-local address though IPv6CP was rejected", namedDevice), addresses.Stdout)
	}
	return nil
}

// requireNamedPPPDevice waits for the client's PPP device and requires it to
// be the configured pppoe0, the device every IPv6 assertion reads.
func requireNamedPPPDevice(ctx context.Context, lab interoplab.CheckerLab) error {
	iface, err := waitPPPLink(ctx, lab, zeImageName, 75*time.Second, 2*time.Second)
	if err != nil {
		return fmt.Errorf("ze PPP interface did not come up: %w", err)
	}
	if iface != namedDevice {
		return fmt.Errorf("ze PPP interface is %s, want the configured %s", iface, namedDevice)
	}
	return nil
}

// waitAccelTrace waits until evidence accepts accel-ppp's trace, and answers
// the last trace it read.
func waitAccelTrace(
	ctx context.Context,
	lab interoplab.CheckerLab,
	description string,
	evidence func(string) error,
) (string, error) {
	trace, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     45 * time.Second,
		Interval:    time.Second,
		Description: description,
	}, func(probeCtx context.Context) (string, error) {
		logs, err := lab.Logs(probeCtx, accelImageName, accelTraceLines)
		if err != nil {
			return "", err
		}
		if !logs.Available {
			return "", errors.New("accel-ppp trace unavailable")
		}
		return logs.Text, nil
	}, func(trace string) bool {
		return evidence(trace) == nil
	})
	return trace, err
}

// waitZeOutput runs argv in the Ze container until it exits zero with output
// ready accepts. Its error carries the last output it read.
func waitZeOutput(
	ctx context.Context,
	lab interoplab.CheckerLab,
	description string,
	argv []string,
	ready func(string) bool,
) error {
	var last string
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: description,
	}, func(probeCtx context.Context) (interoplab.CommandResult, error) {
		return lab.Exec(probeCtx, zeImageName, argv, nil)
	}, func(result interoplab.CommandResult) bool {
		last = result.Stdout + result.Stderr
		return result.ExitCode == 0 && ready(result.Stdout)
	})
	if err != nil {
		return interoplab.WithSearchedOutput(err, last)
	}
	return nil
}

// accelIPv6CPNakAdoption reads, in order, from accel-ppp's verbose IPv6CP
// trace (ppp/ppp_ipv6cp.c, ipv6cp_recv_conf_req and send_conf_nak/ack):
// Ze's Configure-Request with one non-zero Interface-Identifier that is not
// accel-ppp's suggestion, accel-ppp's Configure-Nak of that request suggesting
// accelIPv6PeerID, Ze's next Configure-Request carrying exactly that
// identifier and nothing else, and accel-ppp's Configure-Ack of it. It also
// requires Ze's Configure-Ack of accel-ppp's own request, so both directions
// opened.
func accelIPv6CPNakAdoption(trace string) error {
	firstID, nakID := "", ""
	step := 0
	for line := range strings.SplitSeq(trace, "\n") {
		switch step {
		case 0:
			if !strings.Contains(line, "recv [IPV6CP ConfReq ") {
				continue
			}
			identifier, err := soleInterfaceIdentifier(line)
			if err != nil {
				return interoplab.WithSearchedOutput(err, trace)
			}
			if identifier == "0:0:0:0" {
				return interoplab.WithSearchedOutput(errors.New("IPv6CP: Ze sent a zero Interface-Identifier"), trace)
			}
			if identifier == accelIPv6PeerID {
				return interoplab.WithSearchedOutput(
					errors.New("IPv6CP: Ze's first identifier already equals accel-ppp's suggestion, so no Nak was exercised"), trace)
			}
			firstID = packetID(line)
			step++
		case 1:
			if !strings.Contains(line, "send [IPV6CP ConfNak ") {
				continue
			}
			if packetID(line) != firstID {
				continue
			}
			if !strings.Contains(line, "<addr "+accelIPv6PeerID+">") {
				return interoplab.WithSearchedOutput(errors.New("IPv6CP: accel-ppp's Configure-Nak did not suggest "+accelIPv6PeerID), trace)
			}
			step++
		case 2:
			if !strings.Contains(line, "recv [IPV6CP ConfReq ") {
				continue
			}
			identifier, err := soleInterfaceIdentifier(line)
			if err != nil {
				return interoplab.WithSearchedOutput(err, trace)
			}
			if identifier != accelIPv6PeerID {
				return interoplab.WithSearchedOutput(
					errors.New("IPv6CP: Ze's request after the Nak carries "+identifier+", not the suggested "+accelIPv6PeerID), trace)
			}
			nakID = packetID(line)
			step++
		case 3:
			if !strings.Contains(line, "send [IPV6CP ConfAck ") {
				continue
			}
			if packetID(line) != nakID {
				continue
			}
			step++
		}
	}
	missing := [...]string{
		"IPv6CP: accel-ppp received no Configure-Request from Ze",
		"IPv6CP: accel-ppp sent no Configure-Nak for Ze's first request",
		"IPv6CP: Ze sent no Configure-Request after accel-ppp's Configure-Nak",
		"IPv6CP: accel-ppp did not acknowledge Ze's request for the suggested identifier",
	}
	if step < len(missing) {
		return interoplab.WithSearchedOutput(errors.New(missing[step]), trace)
	}
	if !strings.Contains(trace, "recv [IPV6CP ConfAck ") {
		return interoplab.WithSearchedOutput(errors.New("IPv6CP: Ze did not acknowledge accel-ppp's Configure-Request"), trace)
	}
	return nil
}

// accelIPv6CPRejectedEvidence requires, from the trace of an accel-ppp with
// IPv6 denied, exactly one Protocol-Reject naming IPv6CP (the client stopped
// sending IPv6CP), no IPv6CP negotiation, no LCP Terminate-Request either
// way, and exactly one CHAP Success (no second session).
func accelIPv6CPRejectedEvidence(trace string) error {
	rejects, successes := 0, 0
	for line := range strings.SplitSeq(trace, "\n") {
		if strings.Contains(line, "send [LCP ProtoRej ") && strings.Contains(line, ipv6cpProtocolText) {
			rejects++
		}
		if strings.Contains(line, "send [CHAP Success ") {
			successes++
		}
		if strings.Contains(line, "[LCP TermReq ") {
			return interoplab.WithSearchedOutput(errors.New("LCP: the session was terminated after the IPv6CP Protocol-Reject"), trace)
		}
		if strings.Contains(line, "[IPV6CP ") {
			return interoplab.WithSearchedOutput(errors.New("IPv6CP: accel-ppp negotiated IPv6CP although it denies IPv6"), trace)
		}
	}
	if rejects != 1 {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("accel-ppp sent %d Protocol-Rejects for IPv6CP, want 1: Ze MUST stop sending IPv6CP after the first (RFC 1661 Section 5.7)", rejects), trace)
	}
	if successes != 1 {
		return interoplab.WithSearchedOutput(
			fmt.Errorf("accel-ppp authenticated %d sessions, want 1: the IPv6CP Protocol-Reject cost the session", successes), trace)
	}
	return nil
}

// soleInterfaceIdentifier answers the identifier of a Configure-Request trace
// line that carries exactly one option, the Interface-Identifier, and refuses
// any other option set (RFC 5072 Section 4.1: one Interface-Identifier).
func soleInterfaceIdentifier(line string) (string, error) {
	if strings.Count(line, "<") != 1 {
		return "", errors.New("IPv6CP: Ze's Configure-Request does not carry exactly one option: " + strings.TrimSpace(line))
	}
	_, rest, found := strings.Cut(line, "<addr ")
	if !found {
		return "", errors.New("IPv6CP: Ze's Configure-Request carries no Interface-Identifier: " + strings.TrimSpace(line))
	}
	identifier, _, found := strings.Cut(rest, ">")
	if !found {
		return "", errors.New("IPv6CP: unterminated Interface-Identifier in: " + strings.TrimSpace(line))
	}
	return identifier, nil
}

// packetID answers the "id=" value of an accel-ppp packet trace line, or ""
// when it has none.
func packetID(line string) string {
	_, rest, found := strings.Cut(line, " id=")
	if !found {
		return ""
	}
	end := strings.IndexAny(rest, " ]")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// hasLineWith reports whether one line of trace holds both prefix and needle.
func hasLineWith(trace, prefix, needle string) bool {
	for line := range strings.SplitSeq(trace, "\n") {
		if strings.Contains(line, prefix) && strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
