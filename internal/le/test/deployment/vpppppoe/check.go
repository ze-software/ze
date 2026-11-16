// Design: docs/labs/pppoe-interop.md -- packet and state proof, never image-name proof.
// Related: control_linux.go -- the scratch AC that opens no kernel PPP unit.
package testvpppppoe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func checkMechanism(ctx context.Context, check *interoplab.CheckContext, mode mechanismMode) error {
	if check == nil {
		return errors.New("mechanism checker has no context")
	}
	if check.Lab == nil {
		return errors.New("mechanism checker has no peers")
	}
	lab := check.Lab
	if _, err := query(ctx, lab, vppPeer, "rm", "-f", "/evidence/control.ready", "/evidence/control.state", "/evidence/control.error", "/evidence/control.done", "/evidence/stop"); err != nil {
		return err
	}
	if _, err := query(ctx, lab, vppPeer, "mkdir", "-p", "/run/vpp"); err != nil {
		return err
	}
	if err := lab.ExecDetached(ctx, vppPeer, []string{"sh", "-c", "exec vpp -c /evidence/startup.conf > /evidence/vpp-start.log 2>&1"}, nil); err != nil {
		return err
	}
	if _, err := waitText(ctx, lab, []string{vppctlCommand, "-s", vppCLIPath, showCommand, versionCommand}, "vpp"); err != nil {
		return err
	}
	plugins, err := cli(ctx, lab, showCommand, "plugins")
	if err != nil {
		return err
	}
	for _, plugin := range []string{"pppoe_plugin.so", "linux_cp_plugin.so", "af_packet_plugin.so"} {
		if !strings.Contains(plugins, plugin) {
			return fmt.Errorf("running VPP lacks %s: %s", plugin, plugins)
		}
	}
	mac, err := interfaceMAC(ctx, lab)
	if err != nil {
		return err
	}
	if _, err = query(ctx, lab, vppPeer, "ip", "-4", addressToken, "flush", "dev", accessDevice); err != nil {
		return err
	}
	address, err := networkHost(check.Network.IPv4, 2)
	if err != nil {
		return err
	}
	// Pinned af_packet_create_command_fn accepts hw-addr; without it,
	// af_packet_create_if generates a random 02:fe address unrelated to eth0.
	if _, err = cli(ctx, lab, "create", "host-interface", nameToken, accessDevice, "hw-addr", mac); err != nil {
		return err
	}
	hardware, err := cli(ctx, lab, showCommand, "hardware-interfaces", hostAccessInterface)
	if err != nil {
		return err
	}
	if !strings.Contains(hardware, mac) {
		return fmt.Errorf("VPP access MAC differs from the wire peer's destination %s: %s", mac, hardware)
	}
	if _, err = cli(ctx, lab, setCommand, interfaceKeyword, "state", hostAccessInterface, "up"); err != nil {
		return err
	}
	if _, err = cli(ctx, lab, setCommand, interfaceKeyword, "ip", addressToken, hostAccessInterface, address+"/24"); err != nil {
		return err
	}
	device, namespace, err := prepareControlPair(ctx, lab, mode)
	if err != nil {
		return err
	}
	controlCommand := slices.Concat(
		[]string{"sh", "-c", `exec "$@" > /evidence/control-launch.log 2>&1`, "mechanism-control", helperPath, testCommand, "deployment", "vpp-pppoe", "control", interfaceKeyword, device},
		namespaceArguments(namespace),
	)
	if err := lab.ExecDetached(ctx, vppPeer, controlCommand, nil); err != nil {
		return err
	}
	if _, err = waitText(ctx, lab, []string{readFileCommand, "/evidence/control.ready"}, "ready"); err != nil {
		return err
	}
	clientDevice := accessDevice
	if mode == modeVLAN {
		clientDevice = vlanAccessDevice
		if _, err = query(ctx, lab, clientPeer, "ip", "link", "add", "link", accessDevice, nameToken, clientDevice, "type", "vlan", "id", "100"); err != nil {
			return err
		}
		if _, err = query(ctx, lab, clientPeer, "ip", "link", setCommand, clientDevice, "up"); err != nil {
			return err
		}
	}
	if err := startDiscoveryCapture(ctx, lab); err != nil {
		return err
	}
	// The actual receive-path counterfactual precedes the optional helper CLI.
	// An old image must fail on a malformed frame reaching the CP TAP, not on
	// the absence of a command only the hardening patch supplies.
	if err := checkBoundsWire(ctx, lab, clientDevice, mac); err != nil {
		return err
	}
	bounds, err := cli(ctx, lab, testCommand, "pppoe", "frame-bounds")
	if err != nil {
		return err
	}
	if !strings.Contains(bounds, "PPPoE frame bounds: 31 cases passed") {
		return fmt.Errorf("VPP buffer-bound proof failed: %s", bounds)
	}
	if err := checkReceiveBounds(ctx, lab); err != nil {
		return err
	}
	if err := startPPPD(ctx, lab, clientDevice); err != nil {
		return err
	}
	state, err := waitControl(ctx, lab, func(result controlResult) bool { return result.IPCPOpen })
	if err != nil {
		return err
	}
	if !state.SessionBeforePADS {
		return errors.New("control peer did not observe the VPP session before PADS")
	}
	if !state.LCPRead {
		return errors.New("no LCP request was read from the unattached PPPoX socket")
	}
	if !state.LCPWrite {
		return errors.New("no LCP response was written to the unattached PPPoX socket")
	}
	if !state.NamespaceRestored {
		return errors.New("the socket was not returned to the caller's namespace")
	}
	if mode == modeNamespace {
		if state.SocketNamespace != controlNamespace {
			return errors.New("control socket was not opened in the LCP namespace")
		}
	}
	links, err := query(ctx, lab, vppPeer, "ip", "-o", "link", showCommand, "type", "ppp")
	if err != nil {
		return err
	}
	if strings.TrimSpace(links) != "" {
		return fmt.Errorf("the VPP control peer created a kernel PPP unit: %s", links)
	}
	for _, command := range [][]string{
		{setCommand, interfaceKeyword, "state", sessionInterface, "up"},
		{setCommand, interfaceKeyword, mtuToken, "ip4", "1492", sessionInterface},
		{setCommand, interfaceKeyword, "feature", sessionInterface, "ip4-not-enabled", "arc", "ip4-unicast", "disable"},
		{"ip", "route", "add", peerAddress + "/32", "via", sessionInterface},
	} {
		if _, err = cli(ctx, lab, command...); err != nil {
			return err
		}
	}
	if _, err = query(ctx, lab, trafficPeer, "ip", "route", "add", peerAddress+"/32", "via", address); err != nil {
		return err
	}
	clientPPP, err := waitClientPPP(ctx, lab)
	if err != nil {
		return err
	}
	trafficAddress, err := networkHost(check.Network.IPv4, 4)
	if err != nil {
		return err
	}
	// The connected Docker subnet would otherwise carry the reply directly
	// over eth0. A more-specific route makes the return cross PPP and VPP.
	if _, err = query(ctx, lab, clientPeer, "ip", "route", "replace", trafficAddress+"/32", "dev", clientPPP); err != nil {
		return err
	}
	returnRoute, err := query(ctx, lab, clientPeer, "ip", "-j", "route", "get", trafficAddress)
	if err != nil {
		return err
	}
	if err := routeUsesPPP(returnRoute, clientPPP); err != nil {
		return err
	}
	// Pinned format_vnet_sw_interface_cntrs omits zero counters. Prime one
	// real round trip, then measure a separate burst against explicit rows;
	// an omitted row is never interpreted as a zero baseline.
	if _, err = query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "1", "-W", "3", "-s", "64", peerAddress); err != nil {
		return fmt.Errorf("counter warm-up round trip failed: %w", err)
	}
	baseline, err := waitPacketCounters(ctx, lab)
	if err != nil {
		return err
	}
	if _, err = query(ctx, lab, trafficPeer, pingCommand, "-n", "-c", "2", "-W", "3", "-M", "do", "-s", "1464", peerAddress); err != nil {
		return fmt.Errorf("1492-byte positive-control traffic failed: %w", err)
	}
	if err := waitCounterIncrease(ctx, lab, baseline); err != nil {
		return err
	}
	oversize, oversizeErr := lab.Exec(ctx, trafficPeer, []string{pingCommand, "-n", "-c", "1", "-W", "3", "-M", "do", "-s", "1465", peerAddress}, nil)
	if oversizeErr == nil {
		return errors.New("VPP forwarded a 1493-byte DF packet through a 1492-byte session")
	}
	code, exited := interoplab.ExecExitCode(oversizeErr)
	if !exited {
		return oversizeErr
	}
	if code != 1 {
		return oversizeErr
	}
	if !strings.Contains(oversize.Stdout+oversize.Stderr, "1492") {
		return fmt.Errorf("oversize ping failed without a measured MTU: %w", oversizeErr)
	}
	if err := checkIndependentMTU(ctx, lab, clientPPP); err != nil {
		return err
	}
	counters, err := cli(ctx, lab, showCommand, interfaceKeyword, sessionInterface)
	if err != nil {
		return err
	}
	if err := interfaceTraffic(counters); err != nil {
		return err
	}
	if _, err = query(ctx, lab, clientPeer, helperPath, testCommand, "deployment", "vpp-pppoe", "ipv6", interfaceKeyword, clientDevice, "peer", mac); err != nil {
		return err
	}
	if _, err = waitControl(ctx, lab, func(result controlResult) bool { return result.RouterSolicitation && result.DHCPv6 }); err != nil {
		return err
	}
	if _, err = query(ctx, lab, vppPeer, "touch", "/evidence/stop"); err != nil {
		return err
	}
	if _, err = waitText(ctx, lab, []string{readFileCommand, "/evidence/control.done"}, "done"); err != nil {
		return err
	}
	remaining, err := cli(ctx, lab, showCommand, "pppoe", "session")
	if err != nil {
		return err
	}
	if strings.Contains(remaining, "session-id 17 ") {
		return fmt.Errorf("VPP retained the stopped lab session: %s", remaining)
	}
	return nil
}

// vppctl can exit successfully for an assertion failure. Only the complete
// receive-node regression answer proves all four paths finished their checks.
func checkReceiveBounds(ctx context.Context, lab interoplab.CheckerLab) error {
	output, err := boundsReadback(ctx, lab, testCommand, "pppoe", "receive-bounds")
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "PPPoE receive bounds: 4 paths passed" {
		return fmt.Errorf("VPP receive-bound proof failed: %s", output)
	}
	return nil
}

// The two malformed frames must produce new drops on their actual receiving
// node while leaving the session pool empty. The TAP's PADI observation orders
// the second readback after both injected session frames reached VPP.
func checkBoundsWire(ctx context.Context, lab interoplab.CheckerLab, device, mac string) error {
	if err := boundsEmptySessions(ctx, lab); err != nil {
		return err
	}
	beforeText, err := boundsReadback(ctx, lab, showCommand, "errors")
	if err != nil {
		return err
	}
	before, err := boundsMalformedCount(beforeText)
	if err != nil {
		return err
	}
	if _, err = query(ctx, lab, clientPeer, helperPath, testCommand, "deployment", "vpp-pppoe", "bounds", interfaceKeyword, device, "peer", mac); err != nil {
		return err
	}
	if _, err = waitControl(ctx, lab, func(result controlResult) bool { return result.BoundsBarrier }); err != nil {
		return err
	}
	if err := boundsEmptySessions(ctx, lab); err != nil {
		return err
	}
	afterText, err := boundsReadback(ctx, lab, showCommand, "errors")
	if err != nil {
		return err
	}
	after, err := boundsMalformedCount(afterText)
	if err != nil {
		return err
	}
	if after < before {
		return fmt.Errorf("VPP pppoe-input malformed-length counter decreased across the wire barrier: %d -> %d", before, after)
	}
	if after-before < 2 {
		return fmt.Errorf("VPP pppoe-input did not count both malformed declared lengths after the wire barrier: %d -> %d", before, after)
	}
	return nil
}

// The pinned session command emits this complete answer only for an empty pool.
// Blank output and CLI errors do not prove the absence of installed sessions.
func boundsEmptySessions(ctx context.Context, lab interoplab.CheckerLab) error {
	output, err := boundsReadback(ctx, lab, showCommand, "pppoe", "session")
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "No pppoe sessions configured..." {
		return fmt.Errorf("VPP did not report an empty session table around malformed wire injection: %s", output)
	}
	return nil
}

func boundsReadback(ctx context.Context, lab interoplab.CheckerLab, arguments ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, controlCommandBudget)
	defer cancel()
	result, err := lab.Exec(commandCtx, vppPeer, append([]string{vppctlCommand, "-s", vppCLIPath}, arguments...), nil)
	if err != nil {
		return "", fmt.Errorf("VPP wire-proof readback failed (%s): %w; stdout: %s; stderr: %s", strings.Join(arguments, " "), err, result.Stdout, result.Stderr)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		return "", fmt.Errorf("VPP wire-proof readback refused (%s): stdout: %s; stderr: %s", strings.Join(arguments, " "), result.Stdout, result.Stderr)
	}
	return result.Stdout, nil
}

// Pinned vlib/error.c::show_errors omits zero rows and emits one row per worker
// in its default format. A validated header/table therefore makes absence zero;
// an unreadable or refused answer never becomes a zero baseline.
func boundsMalformedCount(output string) (uint64, error) {
	header := false
	var total uint64
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !header {
			if !slices.Equal(fields, []string{"Count", "Node", "Reason", "Severity"}) {
				return 0, fmt.Errorf("VPP error counter header is unreadable: %s", output)
			}
			header = true
			continue
		}
		if len(fields) < 4 {
			return 0, fmt.Errorf("VPP error counter row is incomplete: %s", output)
		}
		count, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("VPP error counter is unreadable: %w; output: %s", err, output)
		}
		severity := fields[len(fields)-1]
		if !slices.Contains([]string{"error", "warn", "info", "unknown"}, severity) {
			return 0, fmt.Errorf("VPP error counter severity is unreadable: %s", output)
		}
		if fields[1] != "pppoe-input" {
			continue
		}
		if strings.Join(fields[2:len(fields)-1], " ") != "malformed PPPoE frame length" {
			continue
		}
		if count > ^uint64(0)-total {
			return 0, fmt.Errorf("VPP malformed-length counter total overflows: %s", output)
		}
		total += count
	}
	if !header {
		return 0, fmt.Errorf("VPP error counter table is absent: %s", output)
	}
	return total, nil
}

func prepareControlPair(ctx context.Context, lab interoplab.CheckerLab, mode mechanismMode) (device, namespace string, err error) {
	device = controlInterface
	arguments := []string{"lcp", "create", hostAccessInterface, "host-if", controlInterface}
	if mode == modeNamespace {
		namespace = controlNamespace
		if _, err = query(ctx, lab, vppPeer, "ip", "netns", "add", namespace); err != nil {
			return "", "", err
		}
		arguments = append(arguments, "netns", namespace)
	}
	if _, err = cli(ctx, lab, arguments...); err != nil {
		return "", "", err
	}
	pairs, err := cli(ctx, lab, showCommand, "lcp")
	if err != nil {
		return "", "", err
	}
	cp, err := controlVPPName(pairs, device)
	if err != nil {
		return "", "", err
	}
	dp := hostAccessInterface
	if mode == modeVLAN {
		if _, err = cli(ctx, lab, "create", "sub-interfaces", hostAccessInterface, "100"); err != nil {
			return "", "", err
		}
		dp = "host-eth0.100"
		if _, err = cli(ctx, lab, setCommand, interfaceKeyword, "state", dp, "up"); err != nil {
			return "", "", err
		}
		device = "cp0.100"
		// LCP creates both the Linux VLAN and the matching VPP TAP child.
		// A Linux child alone leaves returning tagged discovery unmatched.
		if _, err = cli(ctx, lab, "lcp", "create", dp, "host-if", device); err != nil {
			return "", "", err
		}
		pairs, err = cli(ctx, lab, showCommand, "lcp")
		if err != nil {
			return "", "", err
		}
		cp, err = controlVPPName(pairs, device)
		if err != nil {
			return "", "", err
		}
	}
	if _, err = cli(ctx, lab, "create", "pppoe", "map", "dp", dp, "cp", cp); err != nil {
		return "", "", err
	}
	ipArguments := append([]string{"ip"}, namespaceIPArguments(namespace)...)
	ipArguments = append(ipArguments, "link", setCommand, controlInterface, "up")
	if _, err = query(ctx, lab, vppPeer, ipArguments...); err != nil {
		return "", "", err
	}
	if mode == modeVLAN {
		if _, err = query(ctx, lab, vppPeer, "ip", "link", setCommand, device, "up"); err != nil {
			return "", "", err
		}
	}
	return device, namespace, nil
}

func namespaceArguments(namespace string) []string {
	if namespace == "" {
		return nil
	}
	return []string{"netns", namespace}
}

func namespaceIPArguments(namespace string) []string {
	if namespace == "" {
		return nil
	}
	return []string{"-n", namespace}
}

// format_lcp_itf_pair in the pinned VPP source prints the physical VPP name,
// host VPP name, then Linux name. The parser requires that exact association.
func controlVPPName(output, device string) (string, error) {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		for index, field := range fields {
			if field == device {
				if index > 0 {
					return fields[index-1], nil
				}
			}
		}
	}
	return "", fmt.Errorf("LCP pair has no control interface %s: %s", device, output)
}

func startPPPD(ctx context.Context, lab interoplab.CheckerLab, device string) error {
	return lab.ExecDetached(ctx, clientPeer, []string{pppdCommand, pppdPluginOption, pppdPPPoEPlugin, pppdNICPrefix + device, pppdNoAuthOption, pppdNoACCompOption, pppdNoPCompOption, "nomagic", pppdNoCCPOption, pppdNoVJOption, "noipv6", pppdNoDefaultRoute, "10.20.0.2:10.20.0.1", mtuToken, "1492", pppdMRUOption, "1492", pppdMaxFailOption, "1", pppdNoDetachOption, pppdDebugOption, pppdLogfileOption, "/var/log/ppp/mechanism.log"}, nil)
}

func query(ctx context.Context, lab interoplab.CheckerLab, peer string, args ...string) (string, error) {
	result, err := lab.Exec(ctx, peer, args, nil)
	if err != nil {
		return result.Stdout, err
	}
	return result.Stdout, nil
}

func cli(ctx context.Context, lab interoplab.CheckerLab, args ...string) (string, error) {
	return query(ctx, lab, vppPeer, append([]string{vppctlCommand, "-s", vppCLIPath}, args...)...)
}

func waitText(ctx context.Context, lab interoplab.CheckerLab, args []string, wanted string) (string, error) {
	text, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: controlCommandBudget, Interval: 250 * time.Millisecond, Description: "mechanism readiness " + wanted}, func(ctx context.Context) (string, error) { return query(ctx, lab, vppPeer, args...) }, func(text string) bool { return strings.Contains(text, wanted) })
	return text, err
}

func waitControl(ctx context.Context, lab interoplab.CheckerLab, ready func(controlResult) bool) (controlResult, error) {
	result, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 30 * time.Second, Interval: 250 * time.Millisecond, Description: "PPPoE mechanism control state"}, func(ctx context.Context) (controlResult, error) { return readControlState(ctx, lab) }, ready)
	return result, err
}

// readControlState treats only a completed ENOENT cat as an absent error
// marker. A disconnected Docker transport or a permission failure stays an
// error even if a stale success snapshot could still be read.
func readControlState(ctx context.Context, lab interoplab.CheckerLab) (controlResult, error) {
	failure, err := lab.Exec(ctx, vppPeer, []string{readFileCommand, "/evidence/control.error"}, []interoplab.EnvironmentVariable{{Name: "LC_ALL", Value: "C"}})
	if err == nil {
		if strings.TrimSpace(failure.Stdout) == "" {
			return controlResult{}, errors.New("control helper wrote an empty error marker")
		}
		return controlResult{}, errors.New(failure.Stdout)
	}
	code, exited := interoplab.ExecExitCode(err)
	if !exited {
		return controlResult{}, err
	}
	if code != 1 {
		return controlResult{}, err
	}
	if !strings.Contains(failure.Stderr, "No such file or directory") {
		return controlResult{}, err
	}
	text, err := query(ctx, lab, vppPeer, readFileCommand, "/evidence/control.state")
	if err != nil {
		return controlResult{}, err
	}
	var result controlResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return result, err
	}
	return result, nil
}

func interfaceMAC(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	text, err := query(ctx, lab, vppPeer, readFileCommand, "/sys/class/net/"+accessDevice+"/address")
	if err != nil {
		return "", err
	}
	mac, err := net.ParseMAC(strings.TrimSpace(text))
	if err != nil {
		return "", err
	}
	if len(mac) != 6 {
		return "", fmt.Errorf("interface %s does not have an Ethernet MAC: %s", accessDevice, text)
	}
	return mac.String(), nil
}

func networkHost(prefix netip.Prefix, host byte) (string, error) {
	if !prefix.IsValid() {
		return "", errors.New("mechanism network prefix is absent")
	}
	if !prefix.Addr().Is4() {
		return "", errors.New("mechanism network prefix is not IPv4")
	}
	address := prefix.Addr().As4()
	address[3] = host
	return netip.AddrFrom4(address).String(), nil
}

type packetCounters struct{ received, sent uint64 }

func readPacketCounters(output string) (packetCounters, error) {
	var counters packetCounters
	receivedPresent, sentPresent := false, false
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		for index := 0; index+2 < len(fields); index++ {
			if fields[index+1] != packetsToken {
				continue
			}
			if fields[index] != "rx" {
				if fields[index] != "tx" {
					continue
				}
			}
			value, err := strconv.ParseUint(fields[index+2], 10, 64)
			if err != nil {
				return counters, err
			}
			if fields[index] == "rx" {
				counters.received = value
				receivedPresent = true
			}
			if fields[index] == "tx" {
				counters.sent = value
				sentPresent = true
			}
		}
	}
	if !receivedPresent {
		return counters, fmt.Errorf("VPP session receive counter is absent: %s", output)
	}
	if !sentPresent {
		return counters, fmt.Errorf("VPP session transmit counter is absent: %s", output)
	}
	return counters, nil
}

func counterIncrease(before, after packetCounters) error {
	if after.received <= before.received {
		return fmt.Errorf("controlled ping did not increase VPP received packets: %d -> %d", before.received, after.received)
	}
	if after.sent <= before.sent {
		return fmt.Errorf("controlled ping did not increase VPP transmitted packets: %d -> %d", before.sent, after.sent)
	}
	return nil
}

func waitPacketCounters(ctx context.Context, lab interoplab.CheckerLab) (packetCounters, error) {
	counters, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond, Description: "explicit VPP packet counters after warm-up traffic"}, func(ctx context.Context) (packetCounters, error) {
		text, err := cli(ctx, lab, showCommand, interfaceKeyword, sessionInterface)
		if err != nil {
			return packetCounters{}, err
		}
		return readPacketCounters(text)
	}, func(counters packetCounters) bool {
		if counters.received == 0 {
			return false
		}
		return counters.sent > 0
	})
	return counters, err
}

func waitCounterIncrease(ctx context.Context, lab interoplab.CheckerLab, before packetCounters) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 5 * time.Second, Interval: 250 * time.Millisecond, Description: "both VPP counters to increase across controlled ping"}, func(ctx context.Context) (packetCounters, error) {
		text, err := cli(ctx, lab, showCommand, interfaceKeyword, sessionInterface)
		if err != nil {
			return packetCounters{}, err
		}
		return readPacketCounters(text)
	}, func(after packetCounters) bool { return counterIncrease(before, after) == nil })
	return err
}

func interfaceTraffic(output string) error {
	counters, err := readPacketCounters(output)
	if err != nil {
		return err
	}
	if counters.received == 0 {
		return fmt.Errorf("VPP session has no received data packets: %s", output)
	}
	if counters.sent == 0 {
		return fmt.Errorf("VPP session has no transmitted data packets: %s", output)
	}
	return nil
}

func waitClientPPP(ctx context.Context, lab interoplab.CheckerLab) (string, error) {
	device, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{Timeout: 15 * time.Second, Interval: 250 * time.Millisecond, Description: "pppd's negotiated client interface and address"}, func(ctx context.Context) (string, error) {
		text, err := query(ctx, lab, clientPeer, "ip", "-j", addressToken, showCommand, "type", "ppp")
		if err != nil {
			return "", err
		}
		return clientPPPInterface(text)
	}, func(device string) bool { return device != "" })
	return device, err
}

func clientPPPInterface(output string) (string, error) {
	var links []struct {
		Name      string `json:"ifname"`
		Addresses []struct {
			Local netip.Addr `json:"local"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil {
		return "", err
	}
	wanted, err := netip.ParseAddr(peerAddress)
	if err != nil {
		return "", err
	}
	device := ""
	for _, link := range links {
		for _, address := range link.Addresses {
			if address.Local == wanted {
				if link.Name == "" {
					return "", errors.New("client's assigned PPP address has no interface name")
				}
				if device != "" {
					return "", fmt.Errorf("client's PPP address appears on several interfaces: %s", output)
				}
				device = link.Name
			}
		}
	}
	if device == "" {
		return "", fmt.Errorf("client's PPP address %s is not installed: %s", peerAddress, output)
	}
	return device, nil
}

func routeUsesPPP(output, device string) error {
	var routes []struct {
		Device string `json:"dev"`
	}
	if err := json.Unmarshal([]byte(output), &routes); err != nil {
		return err
	}
	if len(routes) != 1 {
		return fmt.Errorf("subscriber return route is not uniquely resolved: %s", output)
	}
	if routes[0].Device != device {
		return fmt.Errorf("subscriber reply bypasses %s: %s", device, output)
	}
	return nil
}
