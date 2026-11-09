// Design: docs/architecture/testing/interop.md -- the home-CPE lab.
// Related: register_home_lab.go -- the declaration this checks.
// Related: check_vpn.go -- the VPN scenarios, which share these helpers.
package topology

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The LAN probes of lan-services: the MSS lan-host offers on each SYN, the
// port that tells them apart at the web server, and what the server must
// receive. tcp-mss-set 1400 lowers 1460 and leaves 1300 alone: the kernel
// skips a write that would raise the MSS (spec-home-cpe-21).
var lanMSSProbes = []mssProbe{
	{offered: 1460, port: 9101, want: 1400},
	{offered: 1300, port: 9102, want: 1300},
}

// defaultDropPrefix is the log prefix of the CPE's IPv4 input default term,
// which also counts what it drops (ze-cpe.conf, term default-action).
const defaultDropPrefix = "[ipv4-input-default-D]"

// checkLANServices proves spec-home-cpe-11 AC-2 to AC-7 on state: the range
// lease, both reservations, masquerade to the web server, the MSS rewrite and
// its no-raise twin, the WAN input drop beside the LAN answer, and active
// FTP through the helper. Each absence clause reads its positive control in
// the same step (vacuity trap 2).
func checkLANServices(ctx context.Context, check *interoplab.CheckContext) error {
	if _, err := waitCPEAddress(ctx, check, subscriberAddress, func(held netip.Addr) bool { return held.String() == subscriberAddress }); err != nil {
		return err
	}
	if err := checkLease(ctx, check, nodeLANClient, inRange); err != nil {
		return err
	}
	if err := checkLease(ctx, check, nodeLANResA, exactly(resAAddress)); err != nil {
		return err
	}
	if err := checkLease(ctx, check, nodeLANResB, exactly(resBAddress)); err != nil {
		return err
	}
	if err := checkMasquerade(ctx, check, nodeLANStatic, subscriberAddress); err != nil {
		return err
	}
	for _, probe := range lanMSSProbes {
		if err := sendLANMSSProbe(ctx, check, probe); err != nil {
			return err
		}
	}
	if err := checkWANInputDropped(ctx, check); err != nil {
		return err
	}
	return checkActiveFTP(ctx, check)
}

// leaseJudge judges the address a LAN client bound.
type leaseJudge func(netip.Addr) error

// inRange accepts an address inside the first subnet's configured range.
func inRange(address netip.Addr) error {
	first := netip.MustParseAddr(rangeFirst)
	last := netip.MustParseAddr(rangeLast)
	if address.Less(first) {
		return fmt.Errorf("bound %s, below the range %s-%s", address, rangeFirst, rangeLast)
	}
	if last.Less(address) {
		return fmt.Errorf("bound %s, above the range %s-%s", address, rangeFirst, rangeLast)
	}
	return nil
}

// exactly accepts only the reserved address want.
func exactly(want string) leaseJudge {
	return func(address netip.Addr) error {
		if address.String() != want {
			return fmt.Errorf("bound %s, want its reservation %s", address, want)
		}
		return nil
	}
}

// checkLease runs dhcpcd once on the client's LAN device and judges the
// address the kernel then holds there.
func checkLease(ctx context.Context, check *interoplab.CheckContext, client string, judge leaseJudge) error {
	lan, err := check.Member(client, homeLANLink)
	if err != nil {
		return err
	}
	if _, err := execOK(ctx, check, client, []string{commandDHCPCD, "-4", "-1", "-w", "-t", "60", lan.Device}); err != nil {
		return fmt.Errorf("%s's dhcpcd bound no lease from the CPE: %w", client, err)
	}
	answer, err := execOK(ctx, check, client, []string{"ip", "-o", "-4", "address", "show", "dev", lan.Device})
	if err != nil {
		return err
	}
	address, err := pppAddressOn(answer, lan.Device)
	if err != nil {
		return fmt.Errorf("%s: %w", client, err)
	}
	if err := judge(address); err != nil {
		return fmt.Errorf("%s %w", client, err)
	}
	return nil
}

// checkMasquerade fetches the web server's page from peer and proves the
// server's capture holds that connection's SYN from source, the CPE's PPP
// address. The server has no route to the LAN, so an unmasqueraded SYN gets
// no answer and the fetch fails too (vacuity trap 4).
func checkMasquerade(ctx context.Context, check *interoplab.CheckContext, peer, source string) error {
	page, err := execOK(ctx, check, peer, []string{"curl", "-s", "--max-time", "10", "http://" + webAddress + "/"})
	if err != nil {
		return fmt.Errorf("%s fetched nothing from the web server: %w", peer, err)
	}
	if !strings.Contains(page, "ze home lab web") {
		return fmt.Errorf("%s fetched %q from the web server, not its page", peer, page)
	}
	return waitWebSYNFrom(ctx, check, webAddress+".80:", source)
}

// waitWebSYNFrom polls the web server's capture until it holds a SYN to
// target, then judges its source. A fetch that already returned does not mean
// the capture holds its SYN: tcpdump without --immediate-mode receives packets
// from the kernel a buffer timeout late (up to a second), and -U only flushes
// what it has received, so a capture read the moment curl exits can still be
// empty. sendLANMSSProbe waits the same way.
func waitWebSYNFrom(ctx context.Context, check *interoplab.CheckContext, target, source string) error {
	var verdict error
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 20 * time.Second, Interval: time.Second,
		Description: "the web server's capture of the SYN to " + target},
		func(probe context.Context) (string, error) { return readWebCapture(probe, check) },
		func(capture string) bool {
			verdict = synSourceVerdict(capture, target, source)
			return !errors.Is(verdict, errNoSourceSYN)
		})
	if err != nil {
		return fmt.Errorf("the web server's capture holds no SYN to %s from %s: %w", target, source, err)
	}
	return verdict
}

// errNoSourceSYN says the capture holds no SYN to the target yet.
var errNoSourceSYN = errors.New("no SYN to the target in the capture")

// readWebCapture answers the web server's capture as tcpdump prints it.
func readWebCapture(ctx context.Context, check *interoplab.CheckContext) (string, error) {
	return execOK(ctx, check, nodeWeb, []string{"tcpdump", "-n", "-r", webCapture})
}

// synSourceVerdict proves the capture holds a SYN to target from source, and
// refuses one from any other source: the address the server saw is the
// masquerade's whole claim.
func synSourceVerdict(capture, target, source string) error {
	seen := false
	for line := range strings.SplitSeq(capture, "\n") {
		if !strings.Contains(line, " > "+target) {
			continue
		}
		if !strings.Contains(line, "Flags [S]") {
			continue
		}
		if !strings.Contains(line, " IP "+source+".") {
			return fmt.Errorf("the web server received a SYN to %s from another source than %s: %s", target, source, line)
		}
		seen = true
	}
	if !seen {
		return fmt.Errorf("the web server's capture holds no SYN to %s from %s: %w", target, source, errNoSourceSYN)
	}
	return nil
}

// sendLANMSSProbe offers the probe's MSS from lan-host (its route's advmss)
// and judges the SYN the web server captured.
func sendLANMSSProbe(ctx context.Context, check *interoplab.CheckContext, probe mssProbe) error {
	lan, err := check.Member(nodeLANStatic, homeLANLink)
	if err != nil {
		return err
	}
	offered := decimal(probe.offered)
	if _, err := execOK(ctx, check, nodeLANStatic, []string{"ip", "route", "replace", webAddress + "/32",
		"via", cpeLANGateway, "dev", lan.Device, "advmss", offered}); err != nil {
		return err
	}
	port := decimal(probe.port)
	// The exit code is not read (synProbe): nothing listens on the port, and
	// only the SYN the server captured is judged.
	if _, err := check.Lab.Exec(ctx, nodeLANStatic, synProbe("curl", "-s", "--connect-timeout", "3", "http://"+webAddress+":"+port+"/"), nil); err != nil {
		return fmt.Errorf("open TCP from lan-host to %s:%s: %w", webAddress, port, err)
	}
	var verdict error
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 20 * time.Second, Interval: time.Second,
		Description: "the web server's capture of the SYN to port " + port},
		func(probe context.Context) (string, error) { return readWebCapture(probe, check) },
		func(capture string) bool {
			verdict = synMSSVerdict(capture, webAddress+"."+port+":", probe)
			return !errors.Is(verdict, errNoProbeSYN)
		})
	if err != nil {
		return fmt.Errorf("lan-host offered MSS %s to port %s and the web server captured no SYN: %w", offered, port, err)
	}
	return verdict
}

// synMSSVerdict judges the first SYN to target in the capture: it must carry
// an MSS option of the probe's wanted value.
func synMSSVerdict(capture, target string, probe mssProbe) error {
	for line := range strings.SplitSeq(capture, "\n") {
		if !strings.Contains(line, " > "+target) {
			continue
		}
		if !strings.Contains(line, "Flags [S]") {
			continue
		}
		mss, found := capturedMSS(line)
		if !found {
			return fmt.Errorf("the SYN to %s offered MSS %d and arrived with no MSS option: %s", target, probe.offered, line)
		}
		if mss != probe.want {
			return fmt.Errorf("the SYN to %s offered MSS %d and arrived with MSS %d, want %d: %s", target, probe.offered, mss, probe.want, line)
		}
		return nil
	}
	return fmt.Errorf("%w: %s", errNoProbeSYN, target)
}

// checkWANInputDropped connects to the CPE's SSH port from the internet and
// from the LAN. The internet attempt must get no answer while the CPE's
// default-drop counter advances; the LAN attempt, the positive control, must
// complete the handshake.
func checkWANInputDropped(ctx context.Context, check *interoplab.CheckContext) error {
	before, err := readDropCounter(ctx, check)
	if err != nil {
		return err
	}
	_, err = check.Lab.Exec(ctx, nodeWeb, bareConnect(subscriberAddress, "22"), nil)
	if verdict := wanConnectVerdict(err); verdict != nil {
		return verdict
	}
	after, err := readDropCounter(ctx, check)
	if err != nil {
		return err
	}
	if after <= before {
		return fmt.Errorf("the CPE's %s counter stayed at %d across a dropped WAN connection", defaultDropPrefix, after)
	}
	if _, err := execOK(ctx, check, nodeLANStatic, bareConnect(cpeLANGateway, "22")); err != nil {
		return fmt.Errorf("lan-host could not complete TCP to the CPE's SSH port on %s, the positive control: %w", cpeLANGateway, err)
	}
	return nil
}

// bareConnectUnanswered is the exit `timeout` gives bareConnect when its bound
// ended a connect nobody answered. A refused connect exits 1 at once.
const bareConnectUnanswered = 124

// wanConnectVerdict judges the web server's bareConnect to the CPE's SSH
// port from the error Exec returned for it, since Exec returns one for every
// non-zero exit. Only the bound ending the connect passes: no error means the
// handshake completed, another exit means the CPE answered (a reset is an
// answer, and the WAN input term drops), and an error that is no exit means
// the command never ran to one.
func wanConnectVerdict(execErr error) error {
	if execErr == nil {
		return fmt.Errorf("the web server completed TCP to the CPE's SSH port on %s; the WAN input term must drop it", subscriberAddress)
	}
	code, exited := interoplab.ExecExitCode(execErr)
	if !exited {
		return fmt.Errorf("connect from the web server to %s:22: %w", subscriberAddress, execErr)
	}
	return wanExitVerdict(code, execErr)
}

// wanExitVerdict judges the exit the web server's connect ran to: only the
// bound ending it passes.
func wanExitVerdict(code int, execErr error) error {
	if code != bareConnectUnanswered {
		return fmt.Errorf("the CPE answered the web server's TCP to its SSH port on %s (exit %d, not the %d of an unanswered connect); the WAN input term must drop it: %w", subscriberAddress, code, bareConnectUnanswered, execErr)
	}
	return nil
}

// bareConnect is a TCP connect with no payload, exit 0 only on a completed
// handshake, bounded at five seconds.
func bareConnect(address, port string) []string {
	return []string{"timeout", "5", "bash", "-c", "</dev/tcp/" + address + "/" + port}
}

// readDropCounter answers the packet count of the CPE's default-drop term.
func readDropCounter(ctx context.Context, check *interoplab.CheckContext) (uint64, error) {
	ruleset, err := execOK(ctx, check, nodeCPE, []string{"nft", "-j", "list", "ruleset"})
	if err != nil {
		return 0, err
	}
	return dropCounter(ruleset, defaultDropPrefix)
}

// nftRuleset is the part of `nft -j list ruleset` a counter read needs.
type nftRuleset struct {
	Nftables []struct {
		Rule *struct {
			Expr []struct {
				Counter *struct {
					Packets uint64 `json:"packets"`
				} `json:"counter"`
				Log *struct {
					Prefix string `json:"prefix"`
				} `json:"log"`
			} `json:"expr"`
		} `json:"rule"`
	} `json:"nftables"`
}

// dropCounter answers the counter of the one rule whose log prefix is prefix,
// and refuses a ruleset with none or several such rules, or one whose rule
// counts nothing: a missing counter is not a zero.
func dropCounter(ruleset, prefix string) (uint64, error) {
	var parsed nftRuleset
	if err := json.Unmarshal([]byte(ruleset), &parsed); err != nil {
		return 0, fmt.Errorf("the CPE's nft ruleset is not JSON: %w", err)
	}
	var counts []uint64
	for _, entry := range parsed.Nftables {
		if entry.Rule == nil {
			continue
		}
		logged := false
		var counted *uint64
		for _, expression := range entry.Rule.Expr {
			if expression.Log != nil && expression.Log.Prefix == prefix {
				logged = true
			}
			if expression.Counter != nil {
				packets := expression.Counter.Packets
				counted = &packets
			}
		}
		if !logged {
			continue
		}
		if counted == nil {
			return 0, fmt.Errorf("the CPE's rule logging %q carries no counter", prefix)
		}
		counts = append(counts, *counted)
	}
	if len(counts) != 1 {
		return 0, fmt.Errorf("the CPE's ruleset holds %d rules logging %q, want 1", len(counts), prefix)
	}
	return counts[0], nil
}

// checkActiveFTP downloads the FTP server's file from lan-host in active
// mode (PORT). The data connection crosses the masquerade only when the
// FTP helper rewrote the PORT command and expects the server's connection; a
// failure says whether the control connection completed, the positive
// control the helper's red walk reads.
func checkActiveFTP(ctx context.Context, check *interoplab.CheckContext) error {
	result, err := check.Lab.Exec(ctx, nodeLANStatic, []string{"curl", "-sv", "--max-time", "20", "--ftp-port", "-",
		"ftp://" + webAddress + "/file.txt"}, nil)
	if err != nil {
		return fmt.Errorf("run curl on lan-host: %w", err)
	}
	if result.ExitCode == 0 {
		if strings.Contains(result.Stdout, "ze home lab ftp") {
			return nil
		}
		return fmt.Errorf("the active FTP download returned %q, not the server's file", result.Stdout)
	}
	if strings.Contains(result.Stderr, "< 220") {
		return fmt.Errorf("the FTP control connection completed but the active data connection failed (exit %d): %s", result.ExitCode, result.Stderr)
	}
	return fmt.Errorf("the FTP control connection to %s failed (exit %d): %s", webAddress, result.ExitCode, result.Stderr)
}

// execOK runs argv on peer and answers its standard output, refusing a
// non-zero exit with what the command wrote to standard error.
func execOK(ctx context.Context, check *interoplab.CheckContext, peer string, argv []string) (string, error) {
	result, err := check.Lab.Exec(ctx, peer, argv, nil)
	if err != nil {
		return "", fmt.Errorf("%s: %s: %w", peer, strings.Join(argv, " "), err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("%s: %s exited %d: %s", peer, strings.Join(argv, " "), result.ExitCode, result.Stderr)
	}
	return result.Stdout, nil
}
