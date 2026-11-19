// Design: docs/labs/pppoe-interop.md -- independent real pppd process ownership
//go:build ze_l2tp

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Fixed fixture words preserve the existing pppd argv and ip selector values.
const (
	sharedAccessPPPDPlugin          = "plugin"
	sharedAccessPPPDPluginName      = "pppoe.so"
	sharedAccessPPPDUser            = "user"
	sharedAccessPPPDPassword        = "password"
	sharedAccessPPPDNoAuth          = "noauth"
	sharedAccessPPPDRefuseEAP       = "refuse-eap"
	sharedAccessPPPDRefuseMSCHAP    = "refuse-mschap"
	sharedAccessPPPDRefuseMSCHAPV2  = "refuse-mschap-v2"
	sharedAccessPPPDNoIPDefault     = "noipdefault"
	sharedAccessPPPDNoDefaultRoute  = "nodefaultroute"
	sharedAccessPPPDNoACComp        = "noaccomp"
	sharedAccessPPPDNoPComp         = "nopcomp"
	sharedAccessPPPDMTU             = "mtu"
	sharedAccessPPPDMRU             = "mru"
	sharedAccessPPPDLCPEchoInterval = "lcp-echo-interval"
	sharedAccessPPPDLCPEchoFailure  = "lcp-echo-failure"
	sharedAccessPPPDMaxFail         = "maxfail"
	sharedAccessPPPDNoDetach        = "nodetach"
	sharedAccessPPPDDebug           = "debug"
	sharedAccessCommandType         = "type"
)

// create_linkpidfile in pinned pppd2.5.5 main.c writes PID then actual ifname.
// Both health and TERM re-read this original file inside the selected container.
const sharedAccessPPPIdentity = `read -r pid < "$1" || exit 2
case "$pid" in ''|*[!0-9]*) exit 3;; esac
test "$pid" -gt 1 || exit 4
test "$(sed -n '2p' "$1")" = "$2" || exit 5
test "$(cat "/proc/$pid/comm")" = pppd || exit 6`

func sharedAccessPPPCommand(ctx context.Context, lab interoplab.CheckerLab, peer sharedAccessPeer, expectedPID int, stop bool) (int, error) {
	if stop {
		if expectedPID <= 1 {
			return 0, errors.New("stop has no original admitted pppd PID")
		}
	}
	script := sharedAccessPPPIdentity
	if expectedPID > 0 {
		script += "\ntest \"$pid\" = \"$3\" || exit 7"
	}
	if stop {
		script += "\nkill -TERM \"$pid\" || exit 8"
	}
	script += "\nprintf '%s\\n' \"$pid\""
	result, err := lab.Exec(ctx, peer.name, []string{"sh", "-c", script, "peer-session", peer.pidPath(), peer.device, strconv.Itoa(expectedPID)}, nil)
	if err != nil {
		return 0, interoplab.WithSearchedOutput(err, result.Stdout+result.Stderr)
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("%s pppd identity command exited %d: %s", peer.name, result.ExitCode, result.Stderr)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if err != nil {
		return 0, err
	}
	if pid <= 1 {
		return 0, errors.New("peer reported an invalid pppd PID")
	}
	return pid, nil
}

func sharedAccessDial(ctx context.Context, lab interoplab.CheckerLab, peer sharedAccessPeer) error {
	// These are fixed fixture arguments; no operator or peer input enters shell.
	args := []string{pppdExecutable, sharedAccessPPPDPlugin, sharedAccessPPPDPluginName, "nic-" + sharedAccessDevice,
		sharedAccessPPPDUser, peer.username, sharedAccessPPPDPassword, pppdPassword, sharedAccessPPPDNoAuth, refusePAP,
		sharedAccessPPPDRefuseEAP, sharedAccessPPPDRefuseMSCHAP, sharedAccessPPPDRefuseMSCHAPV2, sharedAccessPPPDNoIPDefault,
		sharedAccessPPPDNoDefaultRoute, sharedAccessPPPDNoACComp, sharedAccessPPPDNoPComp, "noipv6", "nopersist", sharedAccessPPPDMaxFail, "1",
		sharedAccessPPPDMTU, "1492", sharedAccessPPPDMRU, "1492", sharedAccessPPPDLCPEchoInterval, "10", sharedAccessPPPDLCPEchoFailure, "5",
		"ifname", peer.device, "linkname", peer.name, "logfile", peer.logPath(),
		"rp_pppoe_service", pppoeService, sharedAccessPPPDNoDetach, sharedAccessPPPDDebug}
	return lab.ExecDetached(ctx, peer.name, append([]string{"sh", "-c", `exec "$@"`, "peer-pppd"}, args...), nil)
}

func sharedAccessCreatePeerVLAN(ctx context.Context, lab interoplab.CheckerLab, peer sharedAccessPeer) error {
	for _, args := range [][]string{
		{"ip", commandLink, commandAdd, commandLink, "eth0", "name", sharedAccessDevice, sharedAccessCommandType, "vlan", "id", strconv.Itoa(sharedAccessVLAN)},
		{"ip", commandLink, "set", commandDev, sharedAccessDevice, "up"},
	} {
		result, err := lab.Exec(ctx, peer.name, args, nil)
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("%s VLAN command failed: %s", peer.name, result.Stderr)
		}
	}
	_, err := sharedAccessReadVLAN(ctx, lab, peer.name)
	return err
}

func sharedAccessAuthEvidence(log string, peer sharedAccessPeer) error {
	for _, pair := range [][2]string{
		{"sent [LCP ConfReq", "<mru 1492>"}, {"rcvd [LCP ConfAck", "<mru 1492>"},
		{"rcvd [LCP ConfReq", "<auth chap MD5>"}, {"sent [LCP ConfAck", "<auth chap MD5>"},
		{"sent [CHAP Response", `name = "` + peer.username + `"`},
	} {
		if !logLineWith(log, pair[0], pair[1]) {
			return fmt.Errorf("%s missing %s %s", peer.name, pair[0], pair[1])
		}
	}
	for _, marker := range []string{"rcvd [CHAP Challenge", "rcvd [CHAP Success", "rcvd [IPCP ConfAck"} {
		if !strings.Contains(log, marker) {
			return fmt.Errorf("%s missing %s", peer.name, marker)
		}
	}
	return nil
}

func sharedAccessTraffic(ctx context.Context, lab interoplab.CheckerLab, peer sharedAccessPeer, row sharedAccessRow, local string) error {
	for _, probe := range []struct{ peer, device, target string }{
		{peer.name, peer.device, zeGateway}, {zeImageName, row.Device, local},
	} {
		route, err := lab.Exec(ctx, probe.peer, []string{"ip", "-j", commandRoute, "get", probe.target}, nil)
		if err != nil {
			return err
		}
		if err := sharedAccessRequireRoute([]byte(route.Stdout), probe.device); err != nil {
			return err
		}
		ping, err := lab.Exec(ctx, probe.peer, []string{pingCommand, "-c", "3", "-W", "3", "-I", probe.device, probe.target}, nil)
		if err != nil {
			return err
		}
		if ping.ExitCode != 0 {
			return errors.New("bound PPP traffic failed: " + ping.Stdout + ping.Stderr)
		}
	}
	return nil
}
