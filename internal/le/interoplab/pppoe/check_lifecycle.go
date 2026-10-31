// Design: docs/architecture/testing/interop.md -- independent PPPoE D-8 lifecycle scenarios.
// Related: check_lifecycle_wire.go -- exact packet and ordering assertions.
package pppoe

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const lifecycleCapture = "/tmp/pppoe-lifecycle.pcap"

// checkCHAPRechallenge starts observing only after network traffic succeeds.
// accel-ppp's native [auth] interval supplies fresh periodic challenges; the
// checker never injects a challenge or substitutes an authenticator.
// MUTATION: drop ProtoCHAP in pppoeclient.keepaliveLoop; no post-IPCP Response appears.
func checkCHAPRechallenge(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil {
		return errors.New("CHAP rechallenge checker has no context")
	}
	if check.Lab == nil {
		return errors.New("CHAP rechallenge checker has no lab")
	}
	lab := check.Lab
	defer func() { err = appendDiagnostics(ctx, lab, err, zeImageName, accelImageName) }()
	iface, err := waitPPPLink(ctx, lab, zeImageName, 75*time.Second, time.Second)
	if err != nil {
		return err
	}
	if _, err := waitAddress(ctx, lab, zeImageName, iface, zeClientAddr, accelGateway, 20*time.Second, time.Second); err != nil {
		return err
	}
	if _, err := waitRoute(ctx, lab, zeImageName, iface, accelGateway, 20*time.Second); err != nil {
		return err
	}
	if _, err := waitAccelSessions(ctx, lab, true, 30*time.Second); err != nil {
		return err
	}
	logs, err := lab.Logs(ctx, accelImageName, accelTraceLines)
	if err != nil {
		return err
	}
	if !logs.Available {
		return errors.New("initial CHAP trace unavailable")
	}
	if err := accelCHAPEvidence(logs.Text); err != nil {
		return err
	}
	if err := lifecyclePing(ctx, lab, zeImageName, iface, accelGateway); err != nil {
		return err
	}
	before, err := query(ctx, lab, zeImageName, []string{commandCat, procNetPPPoE})
	if err != nil {
		return err
	}
	if len(strings.Fields(before)) != 6 {
		return fmt.Errorf("expected one kernel PPPoE session before reauthentication: %s", before)
	}
	local, err := startLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if err := waitLifecycleEvidence(ctx, lab, 50*time.Second, func(frames []lifecycleFrame) error {
		return rechallengeEvidence(frames, local)
	}); err != nil {
		return err
	}
	if err := stopLifecycleCapture(ctx, lab); err != nil {
		return err
	}
	frames, err := readLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if err := rechallengeEvidence(frames, local); err != nil {
		return err
	}
	after, err := query(ctx, lab, zeImageName, []string{commandCat, procNetPPPoE})
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("kernel PPPoE session changed: before %q, after %q", before, after)
	}
	links, err := pppLinks(ctx, lab, zeImageName)
	if err != nil {
		return err
	}
	if !slices.Equal(links, []string{iface}) {
		return fmt.Errorf("reauthentication replaced the PPP interface: %v", links)
	}
	if _, err := waitAddress(ctx, lab, zeImageName, iface, zeClientAddr, accelGateway, 5*time.Second, time.Second); err != nil {
		return err
	}
	if _, err := waitRoute(ctx, lab, zeImageName, iface, accelGateway, 5*time.Second); err != nil {
		return err
	}
	sessions, err := accelSessionCount(ctx, lab)
	if err != nil {
		return err
	}
	if sessions.count != 1 {
		return fmt.Errorf("accel-ppp retained %d sessions, want one", sessions.count)
	}
	return lifecyclePing(ctx, lab, zeImageName, iface, accelGateway)
}

// checkPeerTerminate lets real pppd generate LCP Terminate-Request on SIGTERM.
// Its egress qdisc drops only discovery PADT, preventing the existing PADT
// receive path from masking a missing tlf. Capture is at Ze, beyond that drop.
// MUTATION: remove signalLayerFinished from applyTransition's Stopped branch;
// the Terminate-Ack remains, but Ze never emits PADT or releases the session.
func checkPeerTerminate(ctx context.Context, check *interoplab.CheckContext) (err error) {
	if check == nil {
		return errors.New("peer termination checker has no context")
	}
	if check.Lab == nil {
		return errors.New("peer termination checker has no lab")
	}
	lab := check.Lab
	defer func() {
		err = appendPPPDLog(ctx, lab, err)
		err = appendDiagnostics(ctx, lab, err, zeImageName, clientImageName)
	}()
	if err := waitZePPPoEConfigured(ctx, lab); err != nil {
		return err
	}
	if err := waitZeRESTReady(ctx, lab); err != nil {
		return err
	}
	if err := pppdDial(ctx, lab, pppdPassword, pppoeService); err != nil {
		return err
	}
	sessions, err := waitZeSession(ctx, lab)
	if err != nil {
		return err
	}
	if err := checkDiscoverySession(sessions); err != nil {
		return err
	}
	if sessions[0].SID > 65535 {
		return errors.New("REST session ID exceeds PPPoE wire width")
	}
	iface, err := checkLCPAuthIPCP(ctx, lab)
	if err != nil {
		return err
	}
	if err := lifecyclePing(ctx, lab, clientImageName, iface, zeGateway); err != nil {
		return err
	}
	zeInterface, err := waitPPPLink(ctx, lab, zeImageName, 10*time.Second, time.Second)
	if err != nil {
		return fmt.Errorf("record Ze's live PPP interface before termination: %w", err)
	}
	// u32 offsets begin at the network header: PPPoE Code is octet 1.
	// This matches only EtherType 0x8863, Code 0xa7, not LCP (0x8864).
	// Redirect consumes the matched frame; dummy_xmit frees it without sending.
	// Both facilities ship for Ze's interface/mirroring producers, unlike gact.
	// The sink belongs to this client namespace and dies with its container.
	const padtSink = "padt-sink"
	for _, argv := range [][]string{
		{"ip", commandLink, commandAdd, padtSink, "type", "dummy"},
		{"ip", commandLink, "set", commandDev, padtSink, "up"},
		{"tc", "qdisc", commandAdd, commandDev, accessInterface, "clsact"},
		{"tc", "filter", commandAdd, commandDev, accessInterface, "egress", "protocol", "0x8863", "pref", "1", "u32", "match", "u8", "0xa7", "0xff", "at", "1", "action", "mirred", "egress", "redirect", commandDev, padtSink},
	} {
		if _, err := query(ctx, lab, clientImageName, append([]string{"sh", "-c"}, strings.Join(argv, " ")+" && echo installed")); err != nil {
			return fmt.Errorf("isolate peer termination from client PADT: %w", err)
		}
	}
	local, err := startLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if _, err := exec(ctx, lab, clientImageName, []string{commandPkill, signalTERM, "-x", pppdExecutable}); err != nil {
		return err
	}
	if err := waitLifecycleEvidence(ctx, lab, 15*time.Second, func(frames []lifecycleFrame) error {
		return terminateEvidence(frames, local, uint16(sessions[0].SID)) // #nosec G115 -- checked against 1..65535 above.
	}); err != nil {
		return err
	}
	if err := stopLifecycleCapture(ctx, lab); err != nil {
		return err
	}
	frames, err := readLifecycleCapture(ctx, lab)
	if err != nil {
		return err
	}
	if err := terminateEvidence(frames, local, uint16(sessions[0].SID)); err != nil { // #nosec G115 -- checked against 1..65535 above.
		return err
	}
	if _, err := waitZeSessionsGone(ctx, lab, 10*time.Second); err != nil {
		return err
	}
	if err := waitPPPDExit(ctx, lab, 10*time.Second); err != nil {
		return err
	}
	log, err := pppdLog(ctx, lab)
	if err != nil {
		return err
	}
	for _, marker := range []string{"sent [LCP TermReq", "rcvd [LCP TermAck", "Sent PADT"} {
		if !strings.Contains(log, marker) {
			return interoplab.WithSearchedOutput(fmt.Errorf("pppd termination trace lacks %s", marker), log)
		}
	}
	links, err := pppLinks(ctx, lab, clientImageName)
	if err != nil {
		return err
	}
	if len(links) != 0 {
		return fmt.Errorf("client PPP links remain after termination: %v", links)
	}
	return waitZeKernelRelease(ctx, lab, zeInterface)
}

func lifecyclePing(ctx context.Context, lab interoplab.CheckerLab, peer, iface, target string) error {
	result, err := exec(ctx, lab, peer, []string{pingCommand, "-c", "3", "-W", "3", "-I", iface, target})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("PPP dataplane ping failed: %s %s", result.Stdout, result.Stderr)
	}
	return nil
}

// waitZeKernelRelease observes the kernel separately from REST and PADT:
// neither requires the PPP driver's final descriptor cleanup to have finished.
func waitZeKernelRelease(ctx context.Context, lab interoplab.CheckerLab, iface string) error {
	type kernelState struct {
		Links []string
		Table string
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 10 * time.Second, Interval: time.Second,
		Description: "Ze releases PPP interface " + iface + " and its kernel PPPoE session",
	}, func(probeCtx context.Context) (kernelState, error) {
		links, err := pppLinks(probeCtx, lab, zeImageName)
		if err != nil {
			return kernelState{}, err
		}
		table, err := query(probeCtx, lab, zeImageName, []string{commandCat, procNetPPPoE})
		if err != nil {
			return kernelState{}, err
		}
		return kernelState{Links: links, Table: table}, nil
	}, func(state kernelState) bool {
		if len(state.Links) != 0 {
			return false
		}
		// Linux drivers/net/ppp/pppoe.c, pppoe_seq_show: an empty table
		// retains this header. Missing or malformed output is not cleanup.
		return slices.Equal(strings.Fields(state.Table), []string{"Id", "Address", "Device"})
	})
	return err
}

// startLifecycleCapture MUST be paired with stopLifecycleCapture on success;
// the suite removes the container on a failed assertion. The listening marker
// is emitted after pcap activation, unlike mere process existence.
func startLifecycleCapture(ctx context.Context, lab interoplab.CheckerLab) ([6]byte, error) {
	var local [6]byte
	address, err := query(ctx, lab, zeImageName, []string{commandCat, "/sys/class/net/eth0/address"})
	if err != nil {
		return local, err
	}
	mac, err := net.ParseMAC(strings.TrimSpace(address))
	if err != nil {
		return local, err
	}
	if len(mac) != len(local) {
		return local, errors.New("the access interface of Ze is not Ethernet")
	}
	copy(local[:], mac)
	shell := "exec tcpdump -n -U -s 1600 -c 512 -i eth0 -w " + lifecycleCapture + " 'ether proto 0x8863 or ether proto 0x8864' 2>/tmp/pppoe-lifecycle.log"
	if err := lab.ExecDetached(ctx, zeImageName, []string{"sh", "-c", shell}, nil); err != nil {
		return local, err
	}
	_, _, err = interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 200 * time.Millisecond, Description: "Ze-side PPPoE capture activation"}, func(probeCtx context.Context) (string, error) {
		return lab.Query(probeCtx, zeImageName, []string{commandCat, "/tmp/pppoe-lifecycle.log"}, nil)
	}, func(output string) bool { return strings.Contains(output, "listening on eth0") })
	return local, err
}

// stopLifecycleCapture MUST follow startLifecycleCapture before the final read.
func stopLifecycleCapture(ctx context.Context, lab interoplab.CheckerLab) error {
	if _, err := exec(ctx, lab, zeImageName, []string{commandPkill, "-INT", "-x", "tcpdump"}); err != nil {
		return err
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 10 * time.Second, Interval: 200 * time.Millisecond, Description: "Ze-side capture flush"}, func(probeCtx context.Context) (bool, error) {
		return processRunning(probeCtx, lab, zeImageName, "tcpdump")
	}, func(running bool) bool { return !running })
	return err
}

func readLifecycleCapture(ctx context.Context, lab interoplab.CheckerLab) ([]lifecycleFrame, error) {
	encoded, err := query(ctx, lab, zeImageName, []string{"base64", lifecycleCapture})
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, err
	}
	return lifecycleFrames(raw)
}

func waitLifecycleEvidence(ctx context.Context, lab interoplab.CheckerLab, timeout time.Duration, judge func([]lifecycleFrame) error) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: timeout, Interval: 200 * time.Millisecond, Description: "PPPoE lifecycle wire evidence"}, func(probeCtx context.Context) (string, error) {
		frames, err := readLifecycleCapture(probeCtx, lab)
		if err != nil {
			return "", err
		}
		if err := judge(frames); err != nil {
			return "", err
		}
		return "complete", nil
	}, func(result string) bool { return result == "complete" })
	return err
}
