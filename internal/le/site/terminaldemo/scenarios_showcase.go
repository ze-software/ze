// Design: docs/contributing/terminal-demos.md -- the lab a user would build, staged before the screen shows
// Related: register_showcase.go -- registers the showcase-lab scenario
// Related: validate_showcase.go -- validateShowcaseLab, the check of this lab
// Related: scenarios_network.go -- startFRR, the FRR starter this lab shares

package siteterminaldemo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The showcase lab is the base every topic recording starts from: one Ze in
// the container's root network namespace, started with the base config only,
// beside the fixtures each topic needs. The topic tapes load their own
// sections into that running Ze through the SSH editor, so nothing here
// configures a topic.
//
// Topology:
//
//	Ze (root netns, SSH 127.0.0.1:2222, AS 65000)
//	  |-- transit   127.0.0.2:1179  AS 65001, announces transit.msg (RPKI states)
//	  |-- customer  127.0.0.4:1179  AS 65001, announces customer.msg (IRR), started by `announce`
//	  |-- RTR cache 127.0.0.3:3323
//	  |-- IRR       127.0.0.1:4343
//	  |-- frr0 172.29.0.1/24 == eth0 172.29.0.2/24 in netns showcase-frr:
//	  |   FRR zebra, bgpd (AS 65002), bfdd, ospfd; loopback 10.255.1.2/32
//	  |-- vpn0 172.30.0.1/30 == hidden Ze 172.30.0.2/30 in showcase-vpn
//	  |   protected hosts: showcase-host-left 10.78.1.2, showcase-host-right 10.78.2.2
//	  |-- zeptrk0 203.0.113.251/24 == keepalived .252 in showcase-vrrp
//	  |   tracked uplink: zetrk1; VIP .1, VRID 12
//	  `-- traffic0 10.77.0.1/24 == eth0 .2 in traffic-peer
const (
	showcaseLabID        = "showcase-lab"
	showcaseFRRNamespace = "showcase-frr"
	showcaseFRRLink      = "frr0"
	showcaseZeAddress    = "172.29.0.1/24"
	showcaseFRRAddress   = "172.29.0.2/24"
	showcaseFRRRouterID  = "172.29.0.2"
	showcaseFRRLoopback  = "10.255.1.2/32"
	showcaseAnnounce     = "announce"
	showcasePIDs         = "pids"
)

// readinessKind says how the lab observes that a fixture is ready. The lab
// never sleeps in place of an observation, so a fixture with no kind is a
// defect in its declaration.
type readinessKind uint8

const (
	readinessUnspecified readinessKind = iota
	// readinessListen: target is a host:port that accepts a TCP connection.
	readinessListen
	// readinessAnswer: argv runs and its output contains target.
	readinessAnswer
)

// labReadiness is one observation that a fixture is ready.
type labReadiness struct {
	kind   readinessKind
	target string
	argv   []string
}

// labFixture is one process group the showcase lab starts before Ze, and the
// observations that say it is ready.
type labFixture struct {
	name        string
	start       func(state string, environ []string) ([]int, error)
	ready       []labReadiness
	restartable bool
	controls    map[string]func(state string, environ []string) (string, error)
}

// showcaseLabFixtures lists the fixtures in start order. FRR comes first so
// its BGP speaker listens before Ze dials it.
func showcaseLabFixtures() []labFixture {
	frrAnswer := func(daemon, command, expected string) labReadiness {
		return labReadiness{kind: readinessAnswer, target: expected, argv: []string{"ip", ipNetns, commandExec, showcaseFRRNamespace, "vtysh", "-d", daemon, "-c", command}}
	}
	harness := func(name string, args ...string) func(string, []string) ([]int, error) {
		return func(state string, environ []string) ([]int, error) {
			pid, err := startCommand(demoHarness, append([]string{demoHarnessWord}, args...), environ, filepath.Join(state, name+".log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		}
	}
	return []labFixture{
		{
			name: "frr",
			start: func(state string, environ []string) ([]int, error) {
				return startFRR(showcaseFRRNamespace, []string{"bgpd", "bfdd", "ospfd"}, state, environ)
			},
			controls: showcaseLinkControls(showcaseFRRLink),
			ready: []labReadiness{
				{kind: readinessListen, target: "172.29.0.2:179"},
				// bfdd's _display_peers_brief (bfdd/bfdd_vty.c, FRR 10.3 and
				// 10.5) prints "Session count: %lu" on every call, with zero
				// sessions too: this says bfdd answers its vty, not that a
				// BFD session exists.
				frrAnswer("bfdd", "show bfd peers brief", "Session count"),
				// ospfd's show_ip_ospf_common (ospfd/ospf_vty.c) prints
				// " OSPF Routing Process, Router ID: %pI4", so this says ospfd
				// runs the instance frr.conf declares with its static router-id.
				frrAnswer("ospfd", "show ip ospf", "Router ID: "+showcaseFRRRouterID),
			},
		},
		{
			name:        "rtr-cache",
			start:       showcaseMock("rtr-cache", "rpki", flagBind, "127.0.0.3", flagPort, "3323", "--valid-asn", "65001", "--invalid-asn", "65099", "--refresh", "2", "--retry", "1"),
			ready:       []labReadiness{{kind: readinessListen, target: "127.0.0.3:3323"}},
			restartable: true,
		},
		{
			name:  "irr",
			start: showcaseMock("irr", "irr", flagPort, "4343"),
			ready: []labReadiness{{kind: readinessListen, target: "127.0.0.1:4343"}},
		},
		{
			name:  "transit",
			start: harness("transit", ipPeer, flagMode, peerModeSink, flagBind, loopbackPeerAddress, flagPort, "1179", flagASN, "65001", filepath.Join(demoDir(showcaseLabID), "transit.msg")),
			// A TCP readiness probe would itself count as a fixture connection.
			ready: []labReadiness{{kind: readinessAnswer, target: "listening on " + loopbackPeerAddress + ":1179", argv: []string{"cat", filepath.Join(demoState(showcaseLabID), "transit.log")}}},
		},
		showcaseVPNFixture(),
		showcaseKeepalivedFixture(),
		showcaseTrackedFixture(),
		showcaseTrafficFixture(),
	}
}

// showcaseCustomer is the fixture `announce` starts: the customer whose routes
// the IRR topic filters. It is not in the start list because the IRR topic
// refuses routes that arrive after its commit, so the customer connects on cue.
func showcaseCustomer() labFixture {
	return labFixture{
		name:        "customer",
		restartable: true,
		start: func(state string, environ []string) ([]int, error) {
			routes, err := prepareIRRCustomerAnnouncements(state, environ)
			if err != nil {
				return nil, err
			}
			pid, err := startCommand(demoHarness, []string{demoHarnessWord, ipPeer, flagMode, peerModeSink, flagBind, "127.0.0.4", flagPort, "1179", flagASN, "65001", routes}, environ, filepath.Join(state, "customer.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{{kind: readinessListen, target: "127.0.0.4:1179"}},
		controls: map[string]func(string, []string) (string, error){
			"reannounce": func(state string, environ []string) (string, error) {
				if err := stopShowcaseFixture(state, "customer"); err != nil {
					return "", err
				}
				// Do not let a prior connection's routes satisfy this announcement cycle.
				if err := awaitPolicyTopic(requireIRRCustomerCleared); err != nil {
					return "", err
				}
				err := startFixtures(state, filepath.Join(state, showcasePIDs), environ, []labFixture{showcaseCustomer()})
				return "Customer routes announced", err
			},
		},
	}
}

func runShowcaseLab(action string, args []string, output io.Writer) error {
	const id = showcaseLabID
	state := demoState(id)
	pidPath := filepath.Join(state, showcasePIDs)
	switch action {
	case actionPrepare:
		if err := stopShowcaseLab(pidPath); err != nil {
			return err
		}
		if err := prepareScenario(id, showcasePIDs, true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := createShowcaseNetwork(); err != nil {
			return err
		}
		if err := prepareShowcaseVPN(state); err != nil {
			return err
		}
		return writeLine(output, "Showcase lab prepared")
	case commandStart:
		if err := startShowcaseLab(id, state, pidPath); err != nil {
			return err
		}
		return writeLine(output, "Showcase lab ready")
	case showcaseAnnounce:
		return controlShowcaseLab([]string{"customer", "reannounce"}, output)
	case "control":
		return controlShowcaseLab(args, output)
	case commandCLI:
		if len(args) == 0 {
			return errors.New("showcase-lab cli needs a command")
		}
		text, err := cli(scenarioEnv(id, demoPassword), strings.Join(args, " "))
		if err != nil {
			return err
		}
		_, err = io.WriteString(output, text)
		return err
	case commandStop:
		if err := stopShowcaseLab(pidPath); err != nil {
			return err
		}
		return writeLine(output, "Showcase lab stopped")
	}
	return fmt.Errorf("showcase-lab action %q is not one of prepare, start, announce, control, cli, stop", action)
}

// startShowcaseLab starts the fixtures and the shown Ze. The caller MUST stop
// the lab after success or failure; stopShowcaseLab MUST await these processes
// before it removes their networks.
func startShowcaseLab(id, state, pidPath string) error {
	if _, err := runCommand("install", []string{"-o", frrUser, "-g", frrUser, "-m", "640", filepath.Join(demoDir(id), "frr.conf"), frrConfigFile}, commandOptions{}); err != nil {
		return err
	}
	environ := scenarioEnv(id, demoPassword)
	if err := startFixtures(state, pidPath, environ, showcaseLabFixtures()); err != nil {
		return err
	}
	ze := labFixture{
		name: "ze",
		start: func(state string, environ []string) ([]int, error) {
			pid, err := startCommand("ze", daemonArgs(), environ, filepath.Join(state, "ze.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{{kind: readinessListen, target: "127.0.0.1:2222"}},
	}
	if err := startFixtures(state, pidPath, environ, []labFixture{ze}); err != nil {
		return err
	}
	return awaitShowcasePeers(environ, "transit", "frr")
}

// startFixtures starts each fixture in order, records its pids before it is
// awaited so stop reaches it even when it never becomes ready, and returns
// only when every observation of every fixture has succeeded.
func startFixtures(state, pidPath string, environ []string, fixtures []labFixture) error {
	for _, fixture := range fixtures {
		if len(fixture.ready) == 0 {
			return fmt.Errorf("showcase lab fixture %s has no readiness observation; restore its observation and run ./le site terminal-demo build binaries", fixture.name)
		}
		fixturePath := filepath.Join(state, fixture.name+".pid")
		if _, err := os.Stat(fixturePath); err == nil {
			return fmt.Errorf("showcase lab fixture %s already has owned processes; stop it before starting it", fixture.name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		pids, err := fixture.start(state, environ)
		if err != nil {
			return fmt.Errorf("showcase lab fixture %s: %w", fixture.name, err)
		}
		if err := writePIDs(fixturePath, pids); err != nil {
			for _, pid := range pids {
				_ = terminateShowcasePID(pid) //nolint:errcheck // report the ownership write error
			}
			return err
		}
		if err := appendPIDs(pidPath, pids); err != nil {
			return err
		}
		for _, ready := range fixture.ready {
			if err := awaitReadiness(fixture.name, ready); err != nil {
				return err
			}
		}
	}
	return nil
}

// awaitReadiness polls one observation for up to 30 seconds.
func awaitReadiness(name string, ready labReadiness) error {
	if ready.target == "" {
		return fmt.Errorf("showcase lab fixture %s has an empty readiness target; restore its observation and run ./le site terminal-demo build binaries", name)
	}
	switch ready.kind {
	case readinessListen:
		return waitPort(ready.target, name, 300)
	case readinessAnswer:
		if len(ready.argv) == 0 {
			return fmt.Errorf("showcase lab fixture %s has no readiness command; restore its command and run ./le site terminal-demo build binaries", name)
		}
		_, err := waitForCommandText(300, ready.target, func() (string, error) {
			text, err := runCommand(ready.argv[0], ready.argv[1:], commandOptions{})
			return string(text), err
		})
		if err != nil {
			return fmt.Errorf("showcase lab fixture %s: %w", name, err)
		}
		return nil
	case readinessUnspecified:
		panic("BUG: showcase lab fixture " + name + " declares a readiness with no kind")
	}
	panic("BUG: showcase lab fixture " + name + " declares an unknown readiness kind")
}

func appendPIDs(path string, pids []int) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the path comes from the closed demo scenario table
	if err != nil {
		return err
	}
	for _, pid := range pids {
		if _, err := file.WriteString(strconv.Itoa(pid) + "\n"); err != nil {
			_ = file.Close() //nolint:errcheck // the write error is the one reported
			return err
		}
	}
	return file.Close()
}

func createShowcaseNetwork() error {
	removeShowcaseNetwork()
	for _, args := range showcaseNetworkCommands() {
		if _, err := runCommand("ip", args, commandOptions{}); err != nil {
			removeShowcaseNetwork()
			return err
		}
	}
	for _, namespace := range []string{"", showcaseVPNNamespace} {
		args := []string{"-w", "net.ipv4.ip_forward=1", "net.ipv4.conf.all.rp_filter=0", "net.ipv4.conf.default.rp_filter=0"}
		name := "sysctl"
		if namespace != "" {
			args = append([]string{ipNetns, commandExec, namespace, name}, args...)
			name = "ip"
		}
		if _, err := runCommand(name, args, commandOptions{}); err != nil {
			removeShowcaseNetwork()
			return err
		}
	}
	return nil
}

// removeShowcaseNetwork deletes only lab-owned links and namespaces. They may
// be absent on first prepare or after a stop.
func removeShowcaseNetwork() {
	for _, link := range []string{showcaseFRRLink, "vpn0", "vpn-lan", "vpn-xfrm", "zeptrk0", "zetrk1", interfaceTraffic0} {
		_, _ = runCommand("ip", []string{ipLink, ipDel, link}, commandOptions{})
	}
	for _, namespace := range []string{showcaseFRRNamespace, showcaseVPNNamespace, showcaseHostLeft, showcaseHostRight, showcaseVRRPNamespace, trafficPeerNS} {
		_, _ = runCommand("ip", []string{ipNetns, ipDel, namespace}, commandOptions{})
	}
}

// stopShowcaseLab MUST be called after startShowcaseLab, even on startup
// failure. It waits for owned processes before removing their networks.
func stopShowcaseLab(pidPath string) error {
	state := filepath.Dir(pidPath)
	paths, err := filepath.Glob(filepath.Join(state, "*.pid"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := stopShowcaseFixture(state, strings.TrimSuffix(filepath.Base(path), ".pid")); err != nil {
			return err
		}
	}
	_ = os.Remove(pidPath)                               //nolint:errcheck // the per-fixture ownership files are authoritative
	_ = os.Remove(filepath.Join(frrRunDir, "zserv.api")) //nolint:errcheck // absent once zebra stopped cleanly
	removeShowcaseNetwork()
	return nil
}

func writeLine(output io.Writer, line string) error {
	_, err := io.WriteString(output, line+"\n")
	return err
}
