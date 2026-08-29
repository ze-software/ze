package terminaldemo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func labCleanup(prefix string) {
	commands := [][]string{{ipLink, ipDel, prefix + "-br"}, {ipNetns, ipDel, prefix + "-ze"}, {ipNetns, ipDel, prefix + "-peer"}}
	for _, args := range commands {
		_, _ = runCommand("ip", args, commandOptions{})
	}
}

func labCreatePair(prefix, zeAddress, peerAddress string) error {
	labCleanup(prefix)
	commands := [][]string{
		{ipNetns, ipAdd, prefix + "-ze"}, {ipNetns, ipAdd, prefix + "-peer"},
		{ipLink, ipAdd, prefix + "-br", ipType, "bridge"}, {ipLink, ipSet, prefix + "-br", "up"},
		{ipLink, ipAdd, prefix + "-z", ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, prefix + "-ze"},
		{ipLink, ipAdd, prefix + "-p", ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, prefix + "-peer"},
		{ipLink, ipSet, prefix + "-z", "master", prefix + "-br"}, {ipLink, ipSet, prefix + "-p", "master", prefix + "-br"},
		{ipLink, ipSet, prefix + "-z", "up"}, {ipLink, ipSet, prefix + "-p", "up"},
		{"-n", prefix + "-ze", ipLink, ipSet, "lo", "up"}, {"-n", prefix + "-peer", ipLink, ipSet, "lo", "up"},
		{"-n", prefix + "-ze", ipLink, ipSet, interfaceEth0, "up"}, {"-n", prefix + "-peer", ipLink, ipSet, interfaceEth0, "up"},
		{"-n", prefix + "-ze", ipAddr, ipAdd, zeAddress, ipDev, interfaceEth0}, {"-n", prefix + "-peer", ipAddr, ipAdd, peerAddress, ipDev, interfaceEth0},
	}
	for _, args := range commands {
		if _, err := runCommand("ip", args, commandOptions{}); err != nil {
			labCleanup(prefix)
			return err
		}
	}
	return nil
}

func netnsCLI(prefix, id, commandText string) (string, error) {
	environ := scenarioEnv(id, demoPassword)
	args := []string{ipNetns, commandExec, prefix + "-ze", envBin, "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", commandCLI, "-c", commandText}
	return runBounded("ip", args, environ)
}

// frrRunDir is the runtime directory the FRR daemons share.
const frrRunDir = "/run/frr"

func startFRRPair(namespace, protocol, state string, environ []string) ([]int, error) {
	if _, err := runCommand("install", []string{"-d", "-o", frrUser, "-g", frrUser, "-m", "775", frrRunDir}, commandOptions{}); err != nil {
		return nil, err
	}
	for _, name := range []string{"zserv.api", "zebra.pid", protocol + ".pid"} {
		_ = os.Remove(filepath.Join(frrRunDir, name))
	}
	zebraPID, err := startCommand("ip", []string{ipNetns, commandExec, namespace, "/usr/lib/frr/zebra", "-f", frrConfigFile, "-i", "/run/frr/zebra.pid"}, environ, filepath.Join(state, "zebra.log"))
	if err != nil {
		return nil, err
	}
	for range 100 {
		if _, err := os.Stat("/run/frr/zserv.api"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, err := os.Stat("/run/frr/zserv.api"); err != nil {
		return nil, fmt.Errorf("FRR zebra did not create zserv.api: %w", err)
	}
	protocolPID, err := startCommand("ip", []string{ipNetns, commandExec, namespace, "/usr/lib/frr/" + protocol, "-f", frrConfigFile, "-i", "/run/frr/" + protocol + ".pid"}, environ, filepath.Join(state, protocol+".log"))
	if err != nil {
		return nil, err
	}
	return []int{zebraPID, protocolPID}, nil
}

func runBFD(action string, args []string, output io.Writer) error {
	const id = "bfd-failover"
	const lab = "bfd"
	state := demoState(id)
	pidPath := filepath.Join(state, "pids")
	stop := func() {
		stopPIDs(pidPath)
		_ = os.Remove("/run/frr/zserv.api")
		labCleanup(lab)
	}
	switch action {
	case actionPrepare:
		stop()
		if err := prepareScenario(id, "pids", true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := labCreatePair(lab, "172.30.0.2/24", "172.30.0.3/24"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "BFD failover demo prepared"); err != nil {
			return err
		}
	case commandStart:
		if _, err := runCommand("install", []string{"-o", frrUser, "-g", frrUser, "-m", "640", filepath.Join(demoDir(id), "frr.conf"), frrConfigFile}, commandOptions{}); err != nil {
			return err
		}
		environ := scenarioEnv(id, demoPassword)
		frrPIDs, err := startFRRPair(lab+"-peer", "bfdd", state, environ)
		if err != nil {
			return err
		}
		peerPID, err := startCommand("ip", []string{ipNetns, commandExec, lab + "-peer", "ze-test", ipPeer, flagMode, peerModeSink, flagBind, "172.30.0.3", flagPort, "1179", flagASN, "65002"}, environ, filepath.Join(state, "peer.log"))
		if err != nil {
			return err
		}
		zePID, err := startCommand("ip", []string{ipNetns, commandExec, lab + "-ze", envBin, "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", commandStart, zeConfigFile}, environ, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		pids := make([]int, 0, len(frrPIDs)+2)
		pids = append(pids, frrPIDs...)
		pids = append(pids, peerPID, zePID)
		if err := writePIDs(pidPath, pids); err != nil {
			return err
		}
		if err := waitForFileText(filepath.Join(state, "ze.log"), "SSH server listening", 200); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "established", func() (string, error) { return netnsCLI(lab, id, "show bgp peer list") }); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "up", func() (string, error) { return netnsCLI(lab, id, "show bfd sessions") }); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(state, "bgp-state"), []byte("BGP state: established\n"), 0o600); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "BFD and BGP sessions are up"); err != nil {
			return err
		}
	case commandCLI:
		if len(args) == 0 {
			return errors.New("bfd cli needs a command")
		}
		text, err := netnsCLI(lab, id, strings.Join(args, " "))
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(output, text); err != nil {
			return err
		}
	case commandBGP:
		data, err := os.ReadFile(filepath.Join(state, "bgp-state")) //nolint:gosec // the scenario state directory this driver composed
		if err != nil {
			return err
		}
		_, err = output.Write(data)
		return err
	case "cut", "record-cut":
		if _, err := runCommand("ip", []string{ipLink, ipSet, lab + "-p", "down"}, commandOptions{}); err != nil {
			return err
		}
		attempts := 100
		if action == "record-cut" {
			time.Sleep(5 * time.Second)
			attempts = 1
		}
		for range attempts {
			text, _ := netnsCLI(lab, id, "show bgp peer list")
			if !strings.Contains(text, "established") {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err := os.WriteFile(filepath.Join(state, "bgp-state"), []byte("BGP state: down\n"), 0o600); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "Peer link cut; BFD notified BGP"); err != nil {
			return err
		}
	case "restore":
		if _, err := runCommand("ip", []string{ipLink, ipSet, lab + "-p", "up"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "established", func() (string, error) { return netnsCLI(lab, id, "show bgp peer list") }); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(state, "bgp-state"), []byte("BGP state: established\n"), 0o600); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "Peer link restored; BGP re-established"); err != nil {
			return err
		}
	case demoWalkthrough:
		beforeBFD, err := netnsCLI(lab, id, "show bfd sessions")
		if err != nil {
			return err
		}
		beforeBGP, err := netnsCLI(lab, id, "show bgp peer list")
		if err != nil {
			return err
		}
		if !strings.Contains(beforeBFD, "up") || !strings.Contains(beforeBGP, "established") {
			return errors.New("BFD or BGP was not ready")
		}
		if _, err := fmt.Fprintln(output, "BFD state: up\nBGP state: established"); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		if err := runBFD("cut", nil, io.Discard); err != nil {
			return err
		}
		afterBFD, _ := netnsCLI(lab, id, "show bfd sessions")
		afterBGP, _ := netnsCLI(lab, id, "show bgp peer list")
		if strings.Contains(afterBFD, "up") || strings.Contains(afterBGP, "established") {
			return errors.New("BFD failover did not take BGP down")
		}
		if _, err := fmt.Fprintln(output, "Peer link cut; BFD notified BGP\nBFD state: down\nBGP state: down"); err != nil {
			return err
		}
		time.Sleep(8 * time.Second)
		if err := runBFD("restore", nil, io.Discard); err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(output, "Peer link restored; BGP state: established")
		stop()
		if writeErr != nil {
			return writeErr
		}
		if _, err := fmt.Fprintln(output, "BFD walkthrough complete"); err != nil {
			return err
		}
	case commandStop:
		stop()
		if _, err := fmt.Fprintln(output, "BFD failover demo stopped"); err != nil {
			return err
		}
	default:
		return errors.New("invalid bfd-failover action")
	}
	return nil
}

func runOSPF(action string, args []string, output io.Writer) error {
	const id = "ospf-adjacency"
	const lab = "ospf"
	state := demoState(id)
	pidPath := filepath.Join(state, "ze.pid")
	stop := func() {
		stopPIDs(pidPath)
		_ = os.Remove("/run/frr/zserv.api")
		labCleanup(lab)
	}
	switch action {
	case actionPrepare:
		stop()
		if err := prepareScenario(id, "ze.pid", true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := labCreatePair(lab, "172.31.0.2/24", "172.31.0.3/24"); err != nil {
			return err
		}
		for _, values := range [][]string{{"-n", lab + "-ze", ipAddr, ipAdd, "10.255.0.2/32", ipDev, "lo"}, {"-n", lab + "-peer", ipAddr, ipAdd, "10.255.0.3/32", ipDev, "lo"}} {
			if _, err := runCommand("ip", values, commandOptions{}); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(output, "OSPF lab prepared"); err != nil {
			return err
		}
	case commandStart:
		if _, err := runCommand("install", []string{"-o", frrUser, "-g", frrUser, "-m", "640", filepath.Join(demoDir(id), "frr.conf"), frrConfigFile}, commandOptions{}); err != nil {
			return err
		}
		environ := scenarioEnv(id, demoPassword)
		frrPIDs, err := startFRRPair(lab+"-peer", "ospfd", state, environ)
		if err != nil {
			return err
		}
		pid, err := startCommand("ip", []string{ipNetns, commandExec, lab + "-ze", envBin, "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", commandStart, zeConfigFile}, environ, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(pidPath, append(frrPIDs, pid)); err != nil {
			return err
		}
		if err := waitForFileText(filepath.Join(state, "ze.log"), "SSH server listening", 200); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "full", func() (string, error) { return netnsCLI(lab, id, "show ospf neighbor") }); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		if _, err := runCommand("ip", []string{ipNetns, commandExec, lab + "-peer", "vtysh", "-c", "configure terminal", "-c", "router ospf", "-c", "no redistribute connected", "-c", "redistribute connected"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "10.255.0.3", func() (string, error) { return netnsCLI(lab, id, "show ospf route") }); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "OSPF adjacency converged"); err != nil {
			return err
		}
	case commandCLI:
		if len(args) == 0 {
			return errors.New("ospf cli needs a command")
		}
		text, err := netnsCLI(lab, id, strings.Join(args, " "))
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(output, text); err != nil {
			return err
		}
	case demoWalkthrough:
		neighbor, _ := netnsCLI(lab, id, "show ospf neighbor detail")
		database, _ := netnsCLI(lab, id, "show ospf database router")
		routes, _ := netnsCLI(lab, id, "show ospf route")
		if !strings.Contains(neighbor, "full") || !strings.Contains(neighbor, "172.31.0.3") || !strings.Contains(database, "172.31.0.3") || !strings.Contains(routes, "10.255.0.3") {
			return errors.New("OSPF walkthrough state is incomplete")
		}
		if _, err := fmt.Fprintln(output, "$ ze show ospf neighbor detail\nNeighbor 172.31.0.3 state: full"); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		if _, err := fmt.Fprintln(output, "$ ze show ospf database router\nRouter-LSA 172.31.0.3 present in area 0.0.0.0"); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		if _, err := fmt.Fprintln(output, "$ ze show ospf route\nRoute 10.255.0.3/32 via 172.31.0.3 on eth0"); err != nil {
			return err
		}
		time.Sleep(8 * time.Second)
		stop()
		if _, err := fmt.Fprintln(output, "OSPF walkthrough complete"); err != nil {
			return err
		}
	case commandStop:
		stop()
		if _, err := fmt.Fprintln(output, "OSPF lab stopped"); err != nil {
			return err
		}
	default:
		return errors.New("invalid ospf-adjacency action")
	}
	return nil
}

func runTraffic(action string, output io.Writer) error {
	const id = "traffic-anomaly"
	state := demoState(id)
	pidPath := filepath.Join(state, "pids")
	stop := func() {
		stopPIDs(pidPath)
		_, _ = runCommand("ip", []string{ipLink, ipDel, interfaceTraffic0}, commandOptions{})
		_, _ = runCommand("ip", []string{ipNetns, ipDel, trafficPeerNS}, commandOptions{})
	}
	show := func() (string, error) {
		return runBounded("ze", []string{commandCLI, "-c", "show traffic usage name traffic0 | no-more | raw"}, scenarioEnv(id, demoPassword))
	}
	generate := func() error {
		if _, err := runCommand("ip", []string{ipNetns, commandExec, trafficPeerNS, "ping", "-q", "-c", "8", "10.77.0.1"}, commandOptions{}); err != nil {
			return err
		}
		for range 12 {
			if _, err := runCommand("ip", []string{ipNetns, commandExec, trafficPeerNS, "curl", "-fsS", "http://10.77.0.1:8080/payload.txt"}, commandOptions{}); err != nil {
				return err
			}
		}
		time.Sleep(time.Second)
		return nil
	}
	switch action {
	case actionPrepare:
		stop()
		if err := prepareScenario(id, "pids", true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		commands := [][]string{{ipNetns, ipAdd, trafficPeerNS}, {ipLink, ipAdd, interfaceTraffic0, ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, trafficPeerNS}, {ipAddr, ipAdd, "10.77.0.1/24", ipDev, interfaceTraffic0}, {ipLink, ipSet, interfaceTraffic0, "up"}, {"-n", trafficPeerNS, ipLink, ipSet, "lo", "up"}, {"-n", trafficPeerNS, ipLink, ipSet, interfaceEth0, "up"}, {"-n", trafficPeerNS, ipAddr, ipAdd, "10.77.0.2/24", ipDev, interfaceEth0}}
		for _, args := range commands {
			if _, err := runCommand("ip", args, commandOptions{}); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(output, "Traffic lab prepared"); err != nil {
			return err
		}
	case commandStart:
		env := scenarioEnv(id, demoPassword)
		httpPID, err := startCommand("ze-test", []string{"static-http", flagBind, "10.77.0.1:8080", "--directory", demoDir(id)}, env, filepath.Join(state, "http.log"))
		if err != nil {
			return err
		}
		zePID, err := startCommand("ze", []string{commandStart, zeConfigFile}, env, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(pidPath, []int{httpPID, zePID}); err != nil {
			return err
		}
		for range 300 {
			log, _ := os.ReadFile(filepath.Join(state, "ze.log")) //nolint:gosec // the scenario state directory this driver composed
			if bytes.Contains(log, []byte("SSH server listening")) && !bytes.Contains(log, []byte("traffic usage")) {
				text, _ := show()
				if strings.Contains(text, interfaceTraffic0) {
					_, err := fmt.Fprintln(output, "Traffic monitor attached to traffic0")
					return err
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("traffic monitor did not attach")
	case "generate":
		if err := generate(); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "Generated ICMP and HTTP burst from 10.77.0.2"); err != nil {
			return err
		}
	case commandShow:
		text, err := show()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(output, text); err != nil {
			return err
		}
	case demoWalkthrough:
		before, err := show()
		if err != nil || !strings.Contains(before, interfaceTraffic0) {
			return errors.New("traffic baseline unavailable")
		}
		if _, err := fmt.Fprintln(output, "$ ze show traffic usage name traffic0\nInterface traffic0 baseline: no counters"); err != nil {
			return err
		}
		time.Sleep(4 * time.Second)
		if err := generate(); err != nil {
			return err
		}
		after, err := show()
		if err != nil {
			return err
		}
		if !strings.Contains(after, "10.77.0.2") || !strings.Contains(after, "8080") || !strings.Contains(after, "icmp") {
			return errors.New("traffic burst was not attributed")
		}
		if _, err := fmt.Fprintln(output, "$ ze show traffic usage name traffic0\nSource 10.77.0.2: ICMP and TCP/8080 accounted"); err != nil {
			return err
		}
		time.Sleep(8 * time.Second)
		stop()
		if _, err := fmt.Fprintln(output, "Traffic walkthrough complete"); err != nil {
			return err
		}
	case commandStop:
		stop()
		if _, err := fmt.Fprintln(output, "Traffic lab stopped"); err != nil {
			return err
		}
	default:
		return errors.New("invalid traffic-anomaly action")
	}
	return nil
}

func runVRRP(action string, output io.Writer) error {
	const id = "vrrp-failover"
	const lab = "vrrp"
	state := demoState(id)
	zePID := filepath.Join(state, "ze.pid")
	kaPID := filepath.Join(state, "keepalived.pid")
	stop := func() { stopPIDs(zePID); stopPIDs(kaPID); labCleanup(lab) }
	owner := func() (string, error) {
		for _, candidate := range []struct{ ns, label string }{{lab + "-ze", "Ze"}, {lab + "-peer", "keepalived"}} {
			addresses, _ := runCommand("ip", []string{"-n", candidate.ns, "-o", ipAddr, commandShow}, commandOptions{})
			for line := range strings.SplitSeq(string(addresses), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 4 || fields[3] != "192.0.2.1/24" {
					continue
				}
				dev := strings.TrimSuffix(fields[1], ":")
				links, _ := runCommand("ip", []string{"-n", candidate.ns, "-o", ipLink, commandShow, dev}, commandOptions{})
				parts := strings.Fields(string(links))
				mac := ""
				for i := range parts {
					if parts[i] == "link/ether" && i+1 < len(parts) {
						mac = parts[i+1]
						break
					}
				}
				return fmt.Sprintf("VIP owner: %s (%s), virtual MAC %s", candidate.label, dev, mac), nil
			}
		}
		return "VIP owner: none", errors.New("VIP has no owner")
	}
	failover := func() (string, error) {
		if err := terminatePID(zePID, syscall.SIGTERM); err != nil {
			return "", err
		}
		_, _ = runCommand("ip", []string{ipNetns, ipDel, lab + "-ze"}, commandOptions{})
		for range 150 {
			current, err := owner()
			if err == nil && strings.Contains(current, "keepalived") {
				if _, pingErr := runCommand("ping", []string{"-q", "-c", "2", "-W", "1", "192.0.2.1"}, commandOptions{}); pingErr == nil {
					return current + "\nVIP 192.0.2.1: 2/2 probes answered after failover", nil
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		return "", errors.New("VIP did not fail over")
	}
	switch action {
	case actionPrepare:
		stop()
		if err := prepareScenario(id, "ze.pid", true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := labCreatePair(lab, "192.0.2.251/24", "192.0.2.252/24"); err != nil {
			return err
		}
		if _, err := runCommand("ip", []string{ipAddr, ipAdd, "192.0.2.253/24", ipDev, lab + "-br"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "VRRP lab prepared"); err != nil {
			return err
		}
	case commandStart:
		env := scenarioEnv(id, demoPassword)
		pid, err := startCommand("ip", []string{ipNetns, commandExec, lab + "-ze", envBin, "ZE_CONFIG_DIR=" + envValue(env, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", commandStart, zeConfigFile}, env, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(zePID, []int{pid}); err != nil {
			return err
		}
		if err := waitForFileText(filepath.Join(state, "ze.log"), "SSH server listening", 200); err != nil {
			return err
		}
		if _, err := waitForCommandText(450, "master", func() (string, error) { return netnsCLI(lab, id, "show vrrp") }); err != nil {
			return err
		}
		ka, err := startCommand("ip", []string{ipNetns, commandExec, lab + "-peer", "keepalived", "--dont-fork", "--log-console", "--use-file", filepath.Join(demoDir(id), "keepalived.conf")}, env, filepath.Join(state, "keepalived.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(kaPID, []int{ka}); err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
		if _, err := fmt.Fprintln(output, "Ze is master; keepalived is backup"); err != nil {
			return err
		}
	case commandShow:
		text, err := netnsCLI(lab, id, "show vrrp | no-more")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(output, text); err != nil {
			return err
		}
	case "owner":
		text, err := owner()
		_, writeErr := fmt.Fprintln(output, text)
		if err != nil {
			return err
		}
		return writeErr
	case "crash":
		if err := terminatePID(zePID, syscall.SIGKILL); err != nil {
			return err
		}
		if _, err := runCommand("ip", []string{ipNetns, ipDel, lab + "-ze"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "VRRP node namespace removed"); err != nil {
			return err
		}
	case "proof-show":
		if _, err := fmt.Fprintln(output, "VRRP failover proof:\n  stop Ze with SIGKILL\n  remove namespace vrrp-ze\n  inspect keepalived VIP\n  send two ICMP probes"); err != nil {
			return err
		}
	case "failover", "proof":
		text, err := failover()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, text); err != nil {
			return err
		}
		if action == "proof" {
			if _, err := fmt.Fprintln(output, "2 packets transmitted, 2 received"); err != nil {
				return err
			}
		}
	case demoWalkthrough:
		show, _ := netnsCLI(lab, id, "show vrrp | no-more")
		before, _ := owner()
		if !strings.Contains(show, "master") || !strings.Contains(before, "VIP owner: Ze") {
			return errors.New("VRRP was not master")
		}
		if _, err := fmt.Fprintln(output, "$ ze show vrrp\ngateway state: master, priority: 200\n"+before); err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
		after, err := failover()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, after); err != nil {
			return err
		}
		time.Sleep(8 * time.Second)
		stop()
		if _, err := fmt.Fprintln(output, "VRRP walkthrough complete"); err != nil {
			return err
		}
	case commandStop:
		stop()
		if _, err := fmt.Fprintln(output, "VRRP lab stopped"); err != nil {
			return err
		}
	default:
		return errors.New("invalid vrrp-failover action")
	}
	return nil
}

// The iproute2 keywords, namespace and link names, and tape directives the
// network scenarios repeat. Each constant names the exact token the tool
// receives.
const (
	ipLink            = "link"
	ipPeer            = "peer"
	ipDev             = "dev"
	ipType            = "type"
	ipRoute           = "route"
	ipNetns           = "netns"
	envBin            = "env"
	nsCore            = "core"
	linkEdgeCore      = "edge-core"
	flagMode          = "--mode"
	tapeWaitDirective = "@wait"
	tapeTypeCommand   = "Type"
	matchKeyword      = "match"
)
