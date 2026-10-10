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
	"time"
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
//	  `-- frr0 172.29.0.1/24 == eth0 172.29.0.2/24 in netns showcase-frr:
//	      FRR zebra, bgpd (AS 65002), bfdd, ospfd; loopback 10.255.1.2/32
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
	// showcaseEstablished is the number of BGP sessions up once start
	// returns: transit and frr. The customer session joins on announce.
	showcaseEstablished = 2
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
	name  string
	start func(state string, environ []string) ([]int, error)
	ready []labReadiness
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
			ready: []labReadiness{
				{kind: readinessListen, target: "172.29.0.2:179"},
				frrAnswer("bfdd", "show bfd peers brief", "Session count"),
				frrAnswer("ospfd", "show ip ospf", "Router ID: "+showcaseFRRRouterID),
			},
		},
		{
			name:  "rtr-cache",
			start: harness("rtr-cache", "rpki", flagBind, "127.0.0.3", flagPort, "3323", "--valid-asn", "65001", "--invalid-asn", "65099"),
			ready: []labReadiness{{kind: readinessListen, target: "127.0.0.3:3323"}},
		},
		{
			name:  "irr",
			start: harness("irr", "irr", flagPort, "4343"),
			ready: []labReadiness{{kind: readinessListen, target: "127.0.0.1:4343"}},
		},
		{
			name:  "transit",
			start: harness("transit", ipPeer, flagMode, peerModeSink, flagBind, loopbackPeerAddress, flagPort, "1179", flagASN, "65001", filepath.Join(demoDir(showcaseLabID), "transit.msg")),
			ready: []labReadiness{{kind: readinessListen, target: loopbackPeerAddress + ":1179"}},
		},
	}
}

// showcaseCustomer is the fixture `announce` starts: the customer whose routes
// the IRR topic filters. It is not in the start list because the IRR topic
// refuses routes that arrive after its commit, so the customer connects on cue.
func showcaseCustomer() labFixture {
	return labFixture{
		name: "customer",
		start: func(state string, environ []string) ([]int, error) {
			pid, err := startCommand(demoHarness, []string{demoHarnessWord, ipPeer, flagMode, peerModeSink, flagBind, "127.0.0.4", flagPort, "1179", flagASN, "65001", filepath.Join(demoDir(showcaseLabID), "customer.msg")}, environ, filepath.Join(state, "customer.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{{kind: readinessListen, target: "127.0.0.4:1179"}},
	}
}

func runShowcaseLab(action string, args []string, output io.Writer) error {
	const id = showcaseLabID
	state := demoState(id)
	pidPath := filepath.Join(state, showcasePIDs)
	switch action {
	case actionPrepare:
		stopShowcaseLab(pidPath)
		if err := prepareScenario(id, showcasePIDs, true); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := createShowcaseNetwork(); err != nil {
			return err
		}
		return writeLine(output, "Showcase lab prepared")
	case commandStart:
		if err := startShowcaseLab(id, state, pidPath); err != nil {
			return err
		}
		return writeLine(output, "Showcase lab ready")
	case showcaseAnnounce:
		if err := startFixtures(state, pidPath, scenarioEnv(id, demoPassword), []labFixture{showcaseCustomer()}); err != nil {
			return err
		}
		return writeLine(output, "Customer routes announced")
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
		stopShowcaseLab(pidPath)
		return writeLine(output, "Showcase lab stopped")
	}
	return fmt.Errorf("showcase-lab action %q is not one of prepare, start, announce, cli, stop", action)
}

// startShowcaseLab installs the FRR config, starts every fixture and waits for
// each to report ready, then starts Ze and waits for its SSH server and its
// two always-on BGP sessions.
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
	var peers string
	for range 300 {
		peers, _ = cli(environ, showPeerListRaw) //nolint:errcheck // a refused poll is retried; the count below decides
		if strings.Count(peers, `"state": "established"`) == showcaseEstablished {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("showcase lab: %d BGP sessions (transit, frr) did not reach established\n%s", showcaseEstablished, peers)
}

// startFixtures starts each fixture in order, records its pids before it is
// awaited so stop reaches it even when it never becomes ready, and returns
// only when every observation of every fixture has succeeded.
func startFixtures(state, pidPath string, environ []string, fixtures []labFixture) error {
	for _, fixture := range fixtures {
		pids, err := fixture.start(state, environ)
		if err != nil {
			return fmt.Errorf("showcase lab fixture %s: %w", fixture.name, err)
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
	switch ready.kind {
	case readinessListen:
		return waitPort(ready.target, name, 300)
	case readinessAnswer:
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
	commands := [][]string{
		{ipNetns, ipAdd, showcaseFRRNamespace},
		{ipLink, ipAdd, showcaseFRRLink, ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, showcaseFRRNamespace},
		{ipAddr, ipAdd, showcaseZeAddress, ipDev, showcaseFRRLink},
		{ipLink, ipSet, showcaseFRRLink, "up"},
		{"-n", showcaseFRRNamespace, ipLink, ipSet, "lo", "up"},
		{"-n", showcaseFRRNamespace, ipLink, ipSet, interfaceEth0, "up"},
		{"-n", showcaseFRRNamespace, ipAddr, ipAdd, showcaseFRRAddress, ipDev, interfaceEth0},
		{"-n", showcaseFRRNamespace, ipAddr, ipAdd, showcaseFRRLoopback, ipDev, "lo"},
	}
	for _, args := range commands {
		if _, err := runCommand("ip", args, commandOptions{}); err != nil {
			removeShowcaseNetwork()
			return err
		}
	}
	return nil
}

// removeShowcaseNetwork deletes the lab's link and namespace. Either may be
// absent, on a first prepare or after a stop, so its errors are not failures.
func removeShowcaseNetwork() {
	_, _ = runCommand("ip", []string{ipLink, ipDel, showcaseFRRLink}, commandOptions{})
	_, _ = runCommand("ip", []string{ipNetns, ipDel, showcaseFRRNamespace}, commandOptions{})
}

func stopShowcaseLab(pidPath string) {
	stopPIDs(pidPath)
	_ = os.Remove(filepath.Join(frrRunDir, "zserv.api")) //nolint:errcheck // absent once zebra stopped cleanly
	removeShowcaseNetwork()
}

func writeLine(output io.Writer, line string) error {
	_, err := io.WriteString(output, line+"\n")
	return err
}
