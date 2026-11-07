// Design: docs/labs/pppoe-interop.md -- pppoe-named-device-reconnect-ze-client: the WAN named by its configured name across a reconnect.
// Related: check_client.go -- the accel-ppp session table waits.
// Related: check_helpers.go -- the PPP link, address and route probes.
package pppoe

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// namedDevice is the pppoe-client list key in the scenario's ze.conf, and
	// the name the kernel device MUST carry on every session.
	namedDevice = "pppoe0"
	// namedLANSource is lan0's address in ze.conf. The access concentrator
	// holds no route back to it, so a reply reaches Ze only when the
	// masquerade rule rewrote the source to the session address.
	namedLANSource = "198.51.100.1"
	// namedProbeTarget is an address the checker puts on accel-ppp's loopback.
	// Only the static default route covers it on Ze's side: the session's
	// learned route is the /32 to the AC gateway.
	namedProbeTarget = "192.0.2.1"
	// namedNATTable is the kernel name of ze.conf's `table cpenat`: Ze owns
	// the nft tables it writes by the ze_ prefix (backend_linux.go,
	// zeTablePrefix).
	namedNATTable = "ze_cpenat"
	namedNATChain = "postrouting"
	// namedReconnectAddr is the address accel-ppp assigns the second session.
	// Its pool hands out the head of its free list (accel-pppd/extra/ippool.c,
	// get_ip) and returns a released address to the tail (put_ip), so the
	// session after 10.11.0.2 gets the next address in the [ip-pool] range.
	namedReconnectAddr = "10.11.0.3"
	// fibKernelProtocol is RTPROT_ZE, the protocol fib-kernel stamps on a
	// main-table route (internal/core/rtproto). `ip route show proto 250`
	// filters on it, so no protocol name table has to know it.
	fibKernelProtocol = "250"
)

// namedLinkPattern reads the ifindex from one `ip -o link show dev pppoe0` line.
var namedLinkPattern = regexp.MustCompile(`^(\d+):\s+pppoe0[:@]`)

// namedNATCounterPattern reads the anonymous counter Ze places at the term's
// match/action boundary (lower_linux.go, withCounter).
var namedNATCounterPattern = regexp.MustCompile(`oifname "pppoe0" counter packets (\d+) bytes \d+ masquerade`)

// namedLink is one reading of the Ze container's PPP devices.
type namedLink struct {
	links   []string
	present bool
	index   int
}

// checkNamedDeviceReconnect drives the guide's home CPE configuration: a
// pppoe-client named pppoe0, a static default `next { interface pppoe0 }`
// with no-default-route on the client, and a masquerade rule on
// `output-interface "pppoe0"`. On the first session it requires the kernel
// device to be pppoe0 and the only PPP device Ze holds, the default route at
// fib-kernel's protocol through it, and masqueraded LAN traffic to cross it
// with the nft counter rising. accel-ppp then ends the session from its own
// CLI, and the checker requires a NEW device (another ifindex) under the same
// name, the default route back through it with no reload, and the masqueraded
// traffic and counter again.
// MUTATION: create the client's unit with PPPIOCNEWUNIT again in
// ppp.DevPPPSetup (devppp_linux.go); the kernel names the device pppN and the
// first link wait fails before any route or NAT check.
func checkNamedDeviceReconnect(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil {
		return errors.New("named-device reconnect checker has no context")
	}
	if check.Lab == nil {
		return errors.New("named-device reconnect checker has no lab")
	}
	lab := check.Lab
	defer func() { err = appendDiagnostics(ctx, lab, err, zeImageName, accelImageName) }()

	if err := removeDockerDefaultRoute(ctx, lab); err != nil {
		return err
	}
	if _, err := exec(ctx, lab, accelImageName, []string{"ip", "addr", commandAdd, namedProbeTarget + "/32", commandDev, "lo"}); err != nil {
		return fmt.Errorf("add the probe target %s to accel-ppp's loopback: %w", namedProbeTarget, err)
	}

	first, err := waitNamedSession(ctx, lab, 0, zeClientAddr)
	if err != nil {
		return fmt.Errorf("first session: %w", err)
	}
	if err := namedMasqueradeThroughSession(ctx, lab); err != nil {
		return fmt.Errorf("first session: %w", err)
	}

	// accel-pppd/cli/std_cmd.c terminate_exec2 matches the session by
	// username and calls ap_session_terminate, which ends PPP and sends PADT
	// (ctrl/pppoe/pppoe.c, disconnect).
	if _, err := exec(ctx, lab, accelImageName, []string{commandAccel, "terminate", "username", pppdUsername}); err != nil {
		return fmt.Errorf("accel-cmd terminate username %s: %w", pppdUsername, err)
	}

	second, err := waitNamedSession(ctx, lab, first, namedReconnectAddr)
	if err != nil {
		return fmt.Errorf("session after accel-ppp terminated the first (ifindex %d): %w", first, err)
	}
	if err := namedMasqueradeThroughSession(ctx, lab); err != nil {
		return fmt.Errorf("session after reconnect (ifindex %d): %w", second, err)
	}
	return nil
}

// removeDockerDefaultRoute deletes the default route Docker gives the Ze
// container through eth0. A CPE whose only uplink is the session has none,
// and it sits at metric 0, where fib-kernel's add of the static default would
// meet EEXIST. It MUST run before the session device exists, so the checker
// refuses to go on when Ze already holds a PPP device: the static default may
// then have met Docker's route.
func removeDockerDefaultRoute(ctx context.Context, lab interoplab.CheckerLab) error {
	if _, err := exec(ctx, lab, zeImageName, []string{"ip", commandRoute, "del", "default", commandDev, accessInterface}); err != nil {
		return fmt.Errorf("remove the Docker default route from the Ze container: %w", err)
	}
	links, err := pppLinks(ctx, lab, zeImageName)
	if err != nil {
		return fmt.Errorf("read Ze PPP links: %w", err)
	}
	if len(links) != 0 {
		var tb textbuf.Buffer
		return errors.New(tb.Str("the Ze PPP device [").Join(links, " ").
			Str("] came up before the Docker default route was removed, so the static default may have met it: rerun the scenario").String())
	}
	return nil
}

// waitNamedSession waits for pppoe0 under an ifindex other than previous, then
// for the IPCP address local on it, the static default through it and
// accel-ppp's session. It returns the ifindex it measured.
func waitNamedSession(ctx context.Context, lab interoplab.CheckerLab, previous int, local string) (int, error) {
	link, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     75 * time.Second,
		Interval:    time.Second,
		Description: "PPP device " + namedDevice + " in " + zeImageName,
	}, func(probeCtx context.Context) (namedLink, error) {
		return readNamedLink(probeCtx, lab)
	}, func(found namedLink) bool {
		return found.present && found.index != previous
	})
	if err != nil {
		links, linksErr := pppLinks(ctx, lab, zeImageName)
		if linksErr != nil {
			return 0, fmt.Errorf("%w (Ze PPP links unreadable: %w)", err, linksErr)
		}
		var tb textbuf.Buffer
		return 0, fmt.Errorf("%w; Ze PPP links: [%s]", err, tb.Join(links, " ").String())
	}
	if !slices.Equal(link.links, []string{namedDevice}) {
		var tb textbuf.Buffer
		return 0, errors.New(tb.Str("expected the client's only PPP device to be ").Str(namedDevice).
			Str(", got [").Join(link.links, " ").Byte(']').String())
	}

	if _, err := waitAddress(ctx, lab, zeImageName, namedDevice, local, accelGateway, 20*time.Second, time.Second); err != nil {
		return 0, fmt.Errorf("%s address: %w", namedDevice, err)
	}
	if err := waitNamedDefaultRoute(ctx, lab); err != nil {
		return 0, err
	}
	if _, err := waitAccelSessions(ctx, lab, true, 30*time.Second); err != nil {
		return 0, fmt.Errorf("accel-ppp never reported the subscriber session: %w", err)
	}
	return link.index, nil
}

// readNamedLink reads every PPP device in the Ze container and the ifindex of
// pppoe0 when it is among them.
func readNamedLink(ctx context.Context, lab interoplab.CheckerLab) (namedLink, error) {
	links, err := pppLinks(ctx, lab, zeImageName)
	if err != nil {
		return namedLink{}, err
	}
	if !slices.Contains(links, namedDevice) {
		return namedLink{links: links}, nil
	}
	output, err := query(ctx, lab, zeImageName, []string{"ip", "-o", commandLink, commandShow, commandDev, namedDevice})
	if err != nil {
		return namedLink{}, err
	}
	match := namedLinkPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return namedLink{}, interoplab.WithSearchedOutput(errors.New("no ifindex for "+namedDevice), output)
	}
	index, err := strconv.Atoi(match[1])
	if err != nil {
		return namedLink{}, fmt.Errorf("ifindex of %s: %w", namedDevice, err)
	}
	return namedLink{links: links, present: true, index: index}, nil
}

// waitNamedDefaultRoute waits for the static default in the main table at
// fib-kernel's protocol through pppoe0. The name resolves to the device that
// exists now, and the kernel flushed every route through the old one with it,
// so a match after a reconnect is a route through the new ifindex.
func waitNamedDefaultRoute(ctx context.Context, lab interoplab.CheckerLab) error {
	device := "dev " + namedDevice
	route, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "static default route through " + namedDevice + " at proto " + fibKernelProtocol,
	}, func(probeCtx context.Context) (string, error) {
		result, execErr := exec(probeCtx, lab, zeImageName, []string{"ip", "-o", commandRoute, commandShow, "default", "proto", fibKernelProtocol})
		return result.Stdout, execErr
	}, func(output string) bool {
		return strings.Contains(output, device)
	})
	if err != nil {
		return fmt.Errorf("the static default route did not reach the kernel through %s, got %q: %w", namedDevice, strings.TrimSpace(route), err)
	}
	return nil
}

// namedMasqueradeThroughSession requires the probe target to route through
// pppoe0, a ping sourced from the LAN address to cross the session, and the
// masquerade rule's counter to rise. The ping alone already needs the
// rewrite (the AC cannot answer 198.51.100.1); the counter names the rule
// that did it.
func namedMasqueradeThroughSession(ctx context.Context, lab interoplab.CheckerLab) error {
	routed, err := exec(ctx, lab, zeImageName, []string{"ip", "-o", commandRoute, "get", namedProbeTarget})
	if err != nil {
		return fmt.Errorf("route lookup for %s: %w", namedProbeTarget, err)
	}
	if !strings.Contains(routed.Stdout, "dev "+namedDevice) {
		return interoplab.WithSearchedOutput(errors.New(namedProbeTarget+" does not route through "+namedDevice), routed.Stdout)
	}
	before, err := namedNATCount(ctx, lab)
	if err != nil {
		return err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout:     30 * time.Second,
		Interval:    time.Second,
		Description: "masqueraded ping from " + namedLANSource + " through " + namedDevice,
	}, func(probeCtx context.Context) (interoplab.CommandResult, error) {
		return lab.Exec(probeCtx, zeImageName, []string{pingCommand, "-c", "3", "-W", "3", "-I", namedLANSource, namedProbeTarget}, nil)
	}, func(result interoplab.CommandResult) bool {
		return result.ExitCode == 0
	})
	if err != nil {
		return fmt.Errorf("no reply to %s from %s through the masquerade rule: %w", namedProbeTarget, namedLANSource, err)
	}
	after, err := namedNATCount(ctx, lab)
	if err != nil {
		return err
	}
	if after <= before {
		return fmt.Errorf("masquerade counter on %s did not rise: %d before the ping, %d after", namedDevice, before, after)
	}
	return nil
}

// namedNATCount reads the packet counter of the masquerade rule from the
// kernel's ruleset, not from Ze's own report of it.
func namedNATCount(ctx context.Context, lab interoplab.CheckerLab) (int, error) {
	output, err := query(ctx, lab, zeImageName, []string{"nft", "list", "chain", "ip", namedNATTable, namedNATChain})
	if err != nil {
		return 0, fmt.Errorf("nft list chain ip %s %s: %w", namedNATTable, namedNATChain, err)
	}
	match := namedNATCounterPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, interoplab.WithSearchedOutput(errors.New("no masquerade rule on oifname "+namedDevice+" with a counter"), output)
	}
	count, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("masquerade counter: %w", err)
	}
	return count, nil
}
