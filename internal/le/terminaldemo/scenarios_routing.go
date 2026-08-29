package terminaldemo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const expectedDemoVRPIPv4 = 171

func runRPKI(action string) error {
	const id = demoRPKI
	state := demoState(id)
	switch action {
	case actionPrepare:
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		fmt.Println("RPKI demo prepared")
	case commandStart:
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		environ := scenarioEnv(id, demoPassword)
		pids := make([]int, 0, 3)
		rpkiPID, err := startCommand("ze-test", []string{"rpki", flagBind, "127.0.0.3", flagPort, "3323", "--valid-asn", "65001", "--invalid-asn", "65099"}, environ, filepath.Join(state, "rpki.log"))
		if err != nil {
			return err
		}
		pids = append(pids, rpkiPID)
		peerPID, err := startCommand("ze-test", []string{ipPeer, flagMode, peerModeSink, flagBind, loopbackPeerAddress, flagPort, "1179", flagASN, "65001", filepath.Join(demoDir(id), "routes.msg")}, environ, filepath.Join(state, "peer.log"))
		if err != nil {
			return err
		}
		pids = append(pids, peerPID)
		if err := waitPort("127.0.0.3:3323", "rpki cache", 30); err != nil {
			return err
		}
		if err := waitPort("127.0.0.2:1179", "bgp peer", 30); err != nil {
			return err
		}
		daemonPID, err := startCommand("ze", []string{commandStart, zeConfigFile}, environ, filepath.Join(state, "daemon.log"))
		if err != nil {
			return err
		}
		pids = append(pids, daemonPID)
		if err := writePIDs(filepath.Join(state, "pids"), pids); err != nil {
			return err
		}
		if err := waitForFileText(filepath.Join(state, "daemon.log"), "SSH server listening", 150); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		var status string
		for time.Now().Before(deadline) {
			status, _ = cli(environ, "show bgp rpki status | yaml")
			if strings.Contains(status, fmt.Sprintf("vrp-count-ipv4: %d", expectedDemoVRPIPv4)) {
				fmt.Println("RPKI demo ready")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("timeout waiting for the RTR cache to sync (vrp-count-ipv4: %d)\n%s", expectedDemoVRPIPv4, status)
	case commandStop:
		stopScenario(id, "pids")
		fmt.Println("RPKI demo stopped")
	default:
		return errors.New("rpki action must be prepare, start, or stop")
	}
	return nil
}

func runIRR(action string) error {
	const id = "irr-filter"
	state := demoState(id)
	switch action {
	case actionPrepare:
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		fmt.Println("IRR filter demo prepared")
	case "seed":
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		fmt.Println("Base BGP configuration loaded without IRR filtering")
	case commandStart:
		environ := scenarioEnv(id, demoPassword)
		pid, err := startCommand("ze-test", []string{"irr", flagPort, "4343"}, environ, filepath.Join(state, "irr.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(filepath.Join(state, "pids"), []int{pid}); err != nil {
			return err
		}
		if err := waitPort("127.0.0.1:4343", "irr server", 30); err != nil {
			return err
		}
		daemonPID, err := startCommand("ze", []string{commandStart, zeConfigFile}, environ, filepath.Join(state, "daemon.log"))
		if err != nil {
			return err
		}
		if err := writePIDs(filepath.Join(state, "pids"), []int{pid, daemonPID}); err != nil {
			return err
		}
		if err := waitForFileText(filepath.Join(state, "daemon.log"), "SSH server listening", 200); err != nil {
			return err
		}
		fmt.Println("IRR filter demo ready")
	case "announce":
		data, _ := os.ReadFile(filepath.Join(state, "pids")) //nolint:gosec // the path comes from the closed demo scenario table
		pid, err := startCommand("ze-test", []string{ipPeer, flagMode, peerModeSink, flagBind, loopbackPeerAddress, flagPort, "1179", flagASN, "65001", filepath.Join(demoDir(id), "routes.msg")}, scenarioEnv(id, demoPassword), filepath.Join(state, "peer.log"))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(state, "pids"), append(data, []byte(fmt.Sprintf("%d\n", pid))...), 0o600); err != nil {
			return err
		}
		fmt.Println("Customer routes announced")
	case commandStop:
		stopScenario(id, "pids")
		fmt.Println("IRR filter demo stopped")
	default:
		return errors.New("irr-filter action must be prepare, seed, start, announce, or stop")
	}
	return nil
}

// The remaining repeated tokens: the port flag, the config migrate verb, the
// traceroute edge namespace, the veth link type, the peer loopback address and
// the tape Wait command.
const (
	flagPort            = "--port"
	commandMigrate      = "migrate"
	nsEdge              = "edge"
	ipVeth              = "veth"
	loopbackPeerAddress = "127.0.0.2"
	tapeWaitCommand     = "Wait"
)
