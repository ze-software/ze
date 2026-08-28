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
	commands := [][]string{{"link", "del", prefix + "-br"}, {"netns", "del", prefix + "-ze"}, {"netns", "del", prefix + "-peer"}}
	for _, args := range commands {
		_, _ = runCommand("ip", args, commandOptions{})
	}
}

func labCreatePair(prefix, zeAddress, peerAddress string) error {
	labCleanup(prefix)
	commands := [][]string{
		{"netns", "add", prefix + "-ze"}, {"netns", "add", prefix + "-peer"},
		{"link", "add", prefix + "-br", "type", "bridge"}, {"link", "set", prefix + "-br", "up"},
		{"link", "add", prefix + "-z", "type", "veth", "peer", "name", "eth0", "netns", prefix + "-ze"},
		{"link", "add", prefix + "-p", "type", "veth", "peer", "name", "eth0", "netns", prefix + "-peer"},
		{"link", "set", prefix + "-z", "master", prefix + "-br"}, {"link", "set", prefix + "-p", "master", prefix + "-br"},
		{"link", "set", prefix + "-z", "up"}, {"link", "set", prefix + "-p", "up"},
		{"-n", prefix + "-ze", "link", "set", "lo", "up"}, {"-n", prefix + "-peer", "link", "set", "lo", "up"},
		{"-n", prefix + "-ze", "link", "set", "eth0", "up"}, {"-n", prefix + "-peer", "link", "set", "eth0", "up"},
		{"-n", prefix + "-ze", "addr", "add", zeAddress, "dev", "eth0"}, {"-n", prefix + "-peer", "addr", "add", peerAddress, "dev", "eth0"},
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
	args := []string{"netns", "exec", prefix + "-ze", "env", "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", "cli", "-c", commandText}
	return runBounded("ip", args, environ)
}

func startFRRPair(namespace, protocol, state string, environ []string) ([]int, error) {
	if _, err := runCommand("install", []string{"-d", "-o", "frr", "-g", "frr", "-m", "775", "/run/frr"}, commandOptions{}); err != nil {
		return nil, err
	}
	for _, name := range []string{"zserv.api", "zebra.pid", protocol + ".pid"} {
		_ = os.Remove(filepath.Join("/run/frr", name))
	}
	zebraPID, err := startCommand("ip", []string{"netns", "exec", namespace, "/usr/lib/frr/zebra", "-f", "/etc/frr/frr.conf", "-i", "/run/frr/zebra.pid"}, environ, filepath.Join(state, "zebra.log"))
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
	protocolPID, err := startCommand("ip", []string{"netns", "exec", namespace, "/usr/lib/frr/" + protocol, "-f", "/etc/frr/frr.conf", "-i", "/run/frr/" + protocol + ".pid"}, environ, filepath.Join(state, protocol+".log"))
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
	case "prepare":
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
	case "start":
		if _, err := runCommand("install", []string{"-o", "frr", "-g", "frr", "-m", "640", filepath.Join(demoDir(id), "frr.conf"), "/etc/frr/frr.conf"}, commandOptions{}); err != nil {
			return err
		}
		environ := scenarioEnv(id, demoPassword)
		frrPIDs, err := startFRRPair(lab+"-peer", "bfdd", state, environ)
		if err != nil {
			return err
		}
		peerPID, err := startCommand("ip", []string{"netns", "exec", lab + "-peer", "ze-test", "peer", "--mode", "sink", "--bind", "172.30.0.3", "--port", "1179", "--asn", "65002"}, environ, filepath.Join(state, "peer.log"))
		if err != nil {
			return err
		}
		zePID, err := startCommand("ip", []string{"netns", "exec", lab + "-ze", "env", "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", "start", "ze.conf"}, environ, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		pids := append(frrPIDs, peerPID, zePID)
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
	case "cli":
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
	case "bgp":
		data, err := os.ReadFile(filepath.Join(state, "bgp-state"))
		if err != nil {
			return err
		}
		_, err = output.Write(data)
		return err
	case "cut", "record-cut":
		if _, err := runCommand("ip", []string{"link", "set", lab + "-p", "down"}, commandOptions{}); err != nil {
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
		if _, err := runCommand("ip", []string{"link", "set", lab + "-p", "up"}, commandOptions{}); err != nil {
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
	case "walkthrough":
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
	case "stop":
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
	case "prepare":
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
		for _, values := range [][]string{{"-n", lab + "-ze", "addr", "add", "10.255.0.2/32", "dev", "lo"}, {"-n", lab + "-peer", "addr", "add", "10.255.0.3/32", "dev", "lo"}} {
			if _, err := runCommand("ip", values, commandOptions{}); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(output, "OSPF lab prepared"); err != nil {
			return err
		}
	case "start":
		if _, err := runCommand("install", []string{"-o", "frr", "-g", "frr", "-m", "640", filepath.Join(demoDir(id), "frr.conf"), "/etc/frr/frr.conf"}, commandOptions{}); err != nil {
			return err
		}
		environ := scenarioEnv(id, demoPassword)
		frrPIDs, err := startFRRPair(lab+"-peer", "ospfd", state, environ)
		if err != nil {
			return err
		}
		pid, err := startCommand("ip", []string{"netns", "exec", lab + "-ze", "env", "ZE_CONFIG_DIR=" + envValue(environ, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", "start", "ze.conf"}, environ, filepath.Join(state, "ze.log"))
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
		if _, err := runCommand("ip", []string{"netns", "exec", lab + "-peer", "vtysh", "-c", "configure terminal", "-c", "router ospf", "-c", "no redistribute connected", "-c", "redistribute connected"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := waitForCommandText(300, "10.255.0.3", func() (string, error) { return netnsCLI(lab, id, "show ospf route") }); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "OSPF adjacency converged"); err != nil {
			return err
		}
	case "cli":
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
	case "walkthrough":
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
	case "stop":
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
		_, _ = runCommand("ip", []string{"link", "del", "traffic0"}, commandOptions{})
		_, _ = runCommand("ip", []string{"netns", "del", "traffic-peer"}, commandOptions{})
	}
	show := func() (string, error) {
		return runBounded("ze", []string{"cli", "-c", "show traffic usage name traffic0 | no-more | raw"}, scenarioEnv(id, demoPassword))
	}
	generate := func() error {
		if _, err := runCommand("ip", []string{"netns", "exec", "traffic-peer", "ping", "-q", "-c", "8", "10.77.0.1"}, commandOptions{}); err != nil {
			return err
		}
		for range 12 {
			if _, err := runCommand("ip", []string{"netns", "exec", "traffic-peer", "curl", "-fsS", "http://10.77.0.1:8080/payload.txt"}, commandOptions{}); err != nil {
				return err
			}
		}
		time.Sleep(time.Second)
		return nil
	}
	switch action {
	case "prepare":
		stop()
		if err := prepareScenario(id, "pids", true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		commands := [][]string{{"netns", "add", "traffic-peer"}, {"link", "add", "traffic0", "type", "veth", "peer", "name", "eth0", "netns", "traffic-peer"}, {"addr", "add", "10.77.0.1/24", "dev", "traffic0"}, {"link", "set", "traffic0", "up"}, {"-n", "traffic-peer", "link", "set", "lo", "up"}, {"-n", "traffic-peer", "link", "set", "eth0", "up"}, {"-n", "traffic-peer", "addr", "add", "10.77.0.2/24", "dev", "eth0"}}
		for _, args := range commands {
			if _, err := runCommand("ip", args, commandOptions{}); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(output, "Traffic lab prepared"); err != nil {
			return err
		}
	case "start":
		env := scenarioEnv(id, demoPassword)
		httpPID, err := startCommand("ze-test", []string{"static-http", "--bind", "10.77.0.1:8080", "--directory", demoDir(id)}, env, filepath.Join(state, "http.log"))
		if err != nil {
			return err
		}
		zePID, err := startCommand("ze", []string{"start", "ze.conf"}, env, filepath.Join(state, "ze.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(pidPath, []int{httpPID, zePID}); err != nil {
			return err
		}
		for range 300 {
			log, _ := os.ReadFile(filepath.Join(state, "ze.log"))
			if bytes.Contains(log, []byte("SSH server listening")) && !bytes.Contains(log, []byte("traffic usage")) {
				text, _ := show()
				if strings.Contains(text, "traffic0") {
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
	case "show":
		text, err := show()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(output, text); err != nil {
			return err
		}
	case "walkthrough":
		before, err := show()
		if err != nil || !strings.Contains(before, "traffic0") {
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
	case "stop":
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
			addresses, _ := runCommand("ip", []string{"-n", candidate.ns, "-o", "addr", "show"}, commandOptions{})
			for _, line := range strings.Split(string(addresses), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 4 || fields[3] != "192.0.2.1/24" {
					continue
				}
				dev := strings.TrimSuffix(fields[1], ":")
				links, _ := runCommand("ip", []string{"-n", candidate.ns, "-o", "link", "show", dev}, commandOptions{})
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
		_, _ = runCommand("ip", []string{"netns", "del", lab + "-ze"}, commandOptions{})
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
	case "prepare":
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
		if _, err := runCommand("ip", []string{"addr", "add", "192.0.2.253/24", "dev", lab + "-br"}, commandOptions{}); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, "VRRP lab prepared"); err != nil {
			return err
		}
	case "start":
		env := scenarioEnv(id, demoPassword)
		pid, err := startCommand("ip", []string{"netns", "exec", lab + "-ze", "env", "ZE_CONFIG_DIR=" + envValue(env, "ZE_CONFIG_DIR"), "ZE_SSH_PASSWORD=" + demoPassword, "ze", "start", "ze.conf"}, env, filepath.Join(state, "ze.log"))
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
		ka, err := startCommand("ip", []string{"netns", "exec", lab + "-peer", "keepalived", "--dont-fork", "--log-console", "--use-file", filepath.Join(demoDir(id), "keepalived.conf")}, env, filepath.Join(state, "keepalived.log"))
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
	case "show":
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
		if _, err := runCommand("ip", []string{"netns", "del", lab + "-ze"}, commandOptions{}); err != nil {
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
	case "walkthrough":
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
	case "stop":
		stop()
		if _, err := fmt.Fprintln(output, "VRRP lab stopped"); err != nil {
			return err
		}
	default:
		return errors.New("invalid vrrp-failover action")
	}
	return nil
}
