// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_pppoe_mss_pmtu.go -- the declaration this checks.
package topology

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The two probes of pppoe-mss-pmtu: the MSS lan-host offers on each SYN, the
// destination port that tells the SYNs apart at the server, and the MSS the
// server must receive. 1452 is the PPPoE MTU 1492 less the fixed IPv4 and TCP
// headers (RFC 6691 Section 2); 1200 is already below it and is kept.
var mssPMTUProbes = []mssProbe{
	{offered: 1460, port: 9001, want: 1452},
	{offered: 1200, port: 9002, want: 1200},
}

type mssProbe struct {
	offered uint16
	port    uint16
	want    uint16
}

// checkMSSPMTU proves AC-8 of spec-home-cpe-9 on state: ze-cpe's PPPoE device
// is up with an IPCP address and MTU 1492, the firewall table carries the
// route-MSS clamp, and the server's capture holds each probe SYN with the MSS
// the route allows. lan-host offers the MSS through its route's advmss, so
// the offered value is set, never inferred from a device MTU.
func checkMSSPMTU(ctx context.Context, check *interoplab.CheckContext) error {
	lan, err := check.Member(nodeLANHost, "lan")
	if err != nil {
		return err
	}
	link, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 90 * time.Second, Interval: time.Second,
		Description: "ze-cpe pppoe0 holding an IPCP address from accel-ppp's pool",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, nodeCPE, []string{"ip", "-o", "-4", "address", "show", "dev", "pppoe0"}, nil)
	}, func(answer string) bool { return strings.Contains(answer, " 10.184.0.") })
	if err != nil {
		return fmt.Errorf("ze-cpe's pppoe0 never held an address from 10.184.0.0/24 (last answer %q): %w", link, err)
	}
	device, err := check.Lab.Query(ctx, nodeCPE, []string{"ip", "-o", "link", "show", "dev", "pppoe0"}, nil)
	if err != nil {
		return fmt.Errorf("read ze-cpe's pppoe0: %w", err)
	}
	if !strings.Contains(device, " mtu 1492 ") {
		return fmt.Errorf("ze-cpe's pppoe0 does not report MTU 1492, so the route MTU under test is not the PPPoE one:\n%s", device)
	}
	table, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 30 * time.Second, Interval: time.Second,
		Description: "ze-cpe's firewall table carrying the route-MSS clamp",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, nodeCPE, []string{"nft", "list", "table", "inet", "ze_cpe"}, nil)
	}, func(answer string) bool { return strings.Contains(answer, "maxseg size set rt mtu") })
	if err != nil {
		return fmt.Errorf("ze-cpe's table inet ze_cpe never listed `tcp option maxseg size set rt mtu` (last answer %q): %w", table, err)
	}
	if err := cpeRoutesServer(ctx, check); err != nil {
		return err
	}
	for _, probe := range mssPMTUProbes {
		if err := sendMSSProbe(ctx, check, lan.Device, probe); err != nil {
			return err
		}
	}
	return nil
}

// cpeRoutesServer points ze-cpe's route to the server network at the PPPoE
// device. The default route stays on the lab's eth0, so without this the SYN
// would leave on eth0 and never meet the term.
func cpeRoutesServer(ctx context.Context, check *interoplab.CheckContext) error {
	result, err := check.Lab.Exec(ctx, nodeCPE, []string{"ip", "route", "replace", serverNetwork, "dev", "pppoe0"}, nil)
	if err != nil {
		return fmt.Errorf("route %s through ze-cpe's pppoe0: %w", serverNetwork, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("route %s through ze-cpe's pppoe0 (exit %d): %s", serverNetwork, result.ExitCode, result.Stderr)
	}
	return nil
}

// sendMSSProbe sets lan-host's route to the server with the probe's advmss,
// opens one TCP connection to the probe's port, and waits until the server's
// capture judges that SYN. A refused or unanswered connection is expected:
// the server listens on nothing, and only the SYN is read.
func sendMSSProbe(ctx context.Context, check *interoplab.CheckContext, lanDevice string, probe mssProbe) error {
	offered := decimal(probe.offered)
	route := []string{"ip", "route", "replace", serverNetwork, "via", cpeLANAddress, "dev", lanDevice, "advmss", offered}
	result, err := check.Lab.Exec(ctx, nodeLANHost, route, nil)
	if err != nil {
		return fmt.Errorf("set lan-host's route to %s with advmss %s: %w", serverNetwork, offered, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("set lan-host's route to %s with advmss %s (exit %d): %s", serverNetwork, offered, result.ExitCode, result.Stderr)
	}
	port := decimal(probe.port)
	// The exit code is not read: the connection is refused or times out by
	// design, and the server's capture is the verdict.
	if _, err := check.Lab.Exec(ctx, nodeLANHost, synProbe("nc", "-w", "2", serverAddress, port), nil); err != nil {
		return fmt.Errorf("open TCP from lan-host to %s:%s: %w", serverAddress, port, err)
	}
	var verdict error
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 20 * time.Second, Interval: time.Second,
		Description: "the server's capture of the SYN to port " + port,
	}, func(wait context.Context) (string, error) {
		capture, err := check.Lab.Exec(wait, nodeServer, []string{"tcpdump", "-n", "-r", serverCapture}, nil)
		if err != nil {
			return "", err
		}
		if capture.ExitCode != 0 {
			return "", fmt.Errorf("the server's capture is unreadable (exit %d): %s", capture.ExitCode, capture.Stderr)
		}
		return capture.Stdout, nil
	}, func(capture string) bool {
		verdict = mssPMTUVerdict(capture, probe)
		return !errors.Is(verdict, errNoProbeSYN)
	})
	if err != nil {
		return fmt.Errorf("lan-host offered MSS %s to %s:%s and the server captured no SYN: %w", offered, serverAddress, port, err)
	}
	return verdict
}

// synProbe wraps a connect that only has to SEND its SYN: nothing listens on
// the probe port, so the client always exits non-zero (refused, or timed out
// when no answer comes back), and Lab.Exec turns every non-zero exit into an
// error. The wrapper keeps the client's exit out of the verdict, which the
// capture alone gives, but still fails on 126 and 127, the shell's "cannot
// execute" and "not found": a client that never ran sent no SYN.
func synProbe(argv ...string) []string {
	var tb textbuf.Buffer
	for index, word := range argv {
		if index > 0 {
			tb.Byte(' ')
		}
		tb.Str(interoplab.ShellQuoted(word))
	}
	return []string{"sh", "-c", tb.Str("; [ $? -lt 126 ]").String()}
}

// errNoProbeSYN says the capture holds no SYN to the probe's port yet.
var errNoProbeSYN = errors.New("no SYN to the probe port in the capture")

// mssPMTUVerdict judges one probe against the server's capture, as
// `tcpdump -n -r` prints it: the first SYN to serverAddress at the probe's
// port must carry an MSS option of the wanted value. No such SYN is
// errNoProbeSYN; a SYN with no MSS option, or another value, is refused with
// the line read.
func mssPMTUVerdict(capture string, probe mssProbe) error {
	target := serverAddress + "." + decimal(probe.port) + ":"
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
			return fmt.Errorf("the SYN to %s offered MSS %d and arrived with MSS %d, want %d: %s",
				target, probe.offered, mss, probe.want, line)
		}
		return nil
	}
	return fmt.Errorf("%w: %s", errNoProbeSYN, target)
}

// decimal spells v in base 10, for an argv word and a capture match.
func decimal(v uint16) string {
	var number textbuf.Buffer
	return number.Int(int64(v)).String()
}

// capturedMSS reads the value of "options [mss N" from one tcpdump line.
func capturedMSS(line string) (uint16, bool) {
	_, after, found := strings.Cut(line, "options [mss ")
	if !found {
		return 0, false
	}
	end := strings.IndexAny(after, ",]")
	if end < 0 {
		return 0, false
	}
	value, err := strconv.ParseUint(after[:end], 10, 16)
	if err != nil {
		return 0, false
	}
	return uint16(value), true
}
