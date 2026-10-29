// Design: docs/contributing/gh-pages.md -- the shared recording lab and its controls
// Related: scenarios_showcase.go -- labFixture and the single lab lifecycle

package siteterminaldemo

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	showcaseVPNNamespace  = "showcase-vpn"
	showcaseHostLeft      = "showcase-host-left"
	showcaseHostRight     = "showcase-host-right"
	showcaseVRRPNamespace = "showcase-vrrp"
)

// showcaseNetworkCommands creates only plumbing. The shown Ze creates its
// XFRM interface, tunnel route and protocol configuration on the live commit.
func showcaseNetworkCommands() [][]string {
	commands := [][]string{}
	for _, namespace := range []string{showcaseFRRNamespace, showcaseVPNNamespace, showcaseHostLeft, showcaseHostRight, showcaseVRRPNamespace, trafficPeerNS} {
		commands = append(commands,
			[]string{ipNetns, ipAdd, namespace},
			[]string{"-n", namespace, ipLink, ipSet, "lo", "up"})
	}
	for _, link := range []struct{ name, namespace, local, remote string }{
		{showcaseFRRLink, showcaseFRRNamespace, showcaseZeAddress, showcaseFRRAddress},
		{"vpn0", showcaseVPNNamespace, "172.30.0.1/30", "172.30.0.2/30"},
		{"vpn-lan", showcaseHostLeft, "10.78.1.1/24", "10.78.1.2/24"},
		{showcaseVRRPLink, showcaseVRRPNamespace, "203.0.113.251/24", "203.0.113.252/24"},
		{interfaceTraffic0, trafficPeerNS, "10.77.0.1/24", "10.77.0.2/24"},
	} {
		commands = append(commands,
			[]string{ipLink, ipAdd, link.name, ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, link.namespace},
			[]string{ipAddr, ipAdd, link.local, ipDev, link.name},
			[]string{ipLink, ipSet, link.name, "up"},
			[]string{"-n", link.namespace, ipAddr, ipAdd, link.remote, ipDev, interfaceEth0},
			[]string{"-n", link.namespace, ipLink, ipSet, interfaceEth0, "up"})
	}
	return append(commands,
		[]string{"-n", showcaseFRRNamespace, ipAddr, ipAdd, showcaseFRRLoopback, ipDev, "lo"},
		[]string{"-n", showcaseVPNNamespace, ipLink, ipAdd, showcaseVPNLANLink, ipType, ipVeth, ipPeer, ipName, interfaceEth0, ipNetns, showcaseHostRight},
		[]string{"-n", showcaseVPNNamespace, ipAddr, ipAdd, "10.78.2.1/24", ipDev, showcaseVPNLANLink},
		[]string{"-n", showcaseVPNNamespace, ipLink, ipSet, showcaseVPNLANLink, "up"},
		[]string{"-n", showcaseHostRight, ipAddr, ipAdd, "10.78.2.2/24", ipDev, interfaceEth0},
		[]string{"-n", showcaseHostRight, ipLink, ipSet, interfaceEth0, "up"},
		[]string{"-n", showcaseHostLeft, ipRoute, ipAdd, "default", "via", "10.78.1.1"},
		[]string{"-n", showcaseHostRight, ipRoute, ipAdd, "default", "via", "10.78.2.1"},
		[]string{ipLink, ipAdd, showcaseTrackedLink, ipType, "dummy"},
		[]string{ipLink, ipSet, showcaseTrackedLink, "up"},
	)
}

func showcaseVPNEnv(state string) []string {
	environ := scenarioEnv(showcaseLabID, demoPassword)
	return setEnv(environ, "ZE_CONFIG_DIR", filepath.Join(state, "far-end", "config"))
}

// prepareShowcaseVPN initializes the hidden router in its own namespace and
// store. It MUST run after createShowcaseNetwork and before startFixtures.
func prepareShowcaseVPN(state string) error {
	config := filepath.Join(state, "far-end", "config")
	if err := os.MkdirAll(config, 0o750); err != nil {
		return err
	}
	input, err := os.Open(filepath.Join(state, "init.input")) //nolint:gosec // the path comes from the closed demo scenario table
	if err != nil {
		return err
	}
	_, runErr := runCommand("ip", []string{ipNetns, commandExec, showcaseVPNNamespace, "ze", commandInit}, commandOptions{env: showcaseVPNEnv(state), stdin: input})
	closeErr := input.Close()
	if runErr != nil {
		return runErr
	}
	if closeErr != nil {
		return closeErr
	}
	environ := showcaseVPNEnv(state)
	active, err := runCommand("ip", []string{ipNetns, commandExec, showcaseVPNNamespace, "ze", commandConfig, commandCat, zeConfigFile}, commandOptions{env: environ})
	if err != nil {
		return err
	}
	section, err := os.ReadFile(filepath.Join(demoDir(showcaseLabID), "far-end.conf"))
	if err != nil {
		return err
	}
	path := filepath.Join(state, "far-end", "active.conf")
	if err := os.WriteFile(path, append(active, section...), 0o600); err != nil {
		return err
	}
	_, err = runCommand("ip", append([]string{ipNetns, commandExec, showcaseVPNNamespace, "ze"}, importScenarioConfigArgs(path)...), commandOptions{env: environ})
	return err
}

func showcaseVPNFixture() labFixture {
	return labFixture{
		name: showcaseVPN,
		start: func(state string, _ []string) ([]int, error) {
			pid, err := startCommand("ip", append([]string{ipNetns, commandExec, showcaseVPNNamespace, "ze"}, daemonArgs()...), showcaseVPNEnv(state), filepath.Join(state, "far-end.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{
			{kind: readinessAnswer, target: "0.0.0.0:500", argv: []string{"ip", ipNetns, commandExec, showcaseVPNNamespace, "ss", "-H", "-lun"}},
			{kind: readinessAnswer, target: showcaseXFRMLink, argv: []string{"ip", "-n", showcaseVPNNamespace, "-o", ipLink, commandShow, showcaseXFRMLink}},
			{kind: readinessAnswer, target: "1 received", argv: []string{"ip", ipNetns, commandExec, showcaseHostLeft, programPing, "-c", "1", "-W", "1", "10.78.1.1"}},
			{kind: readinessAnswer, target: "1 received", argv: []string{"ip", ipNetns, commandExec, showcaseHostRight, programPing, "-c", "1", "-W", "1", "10.78.2.1"}},
		},
		controls: map[string]func(string, []string) (string, error){
			controlPing: func(_ string, _ []string) (string, error) {
				answer, err := runCommand("ip", []string{ipNetns, commandExec, showcaseHostLeft, programPing, "-c", "4", "-W", "2", "10.78.2.2"}, commandOptions{})
				return string(answer), err
			},
		},
	}
}

func showcaseKeepalivedFixture() labFixture {
	return labFixture{
		name:        showcaseKeepalived,
		restartable: true,
		start: func(state string, environ []string) ([]int, error) {
			pid, err := startCommand("ip", []string{ipNetns, commandExec, showcaseVRRPNamespace, "keepalived", "--dont-fork", "--log-console", "--use-file", filepath.Join(demoDir(showcaseLabID), "keepalived.conf"), "--pid", filepath.Join(state, "keepalived-daemon.run"), "--vrrp_pid", filepath.Join(state, "keepalived-vrrp.run")}, environ, filepath.Join(state, "keepalived.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{{kind: readinessAnswer, target: "vrrp.12", argv: []string{"ip", "-n", showcaseVRRPNamespace, "-o", ipLink, commandShow, "vrrp.12"}}},
	}
}

func showcaseTrackedFixture() labFixture {
	return labFixture{
		name: showcaseTrackedUplink,
		start: func(_ string, _ []string) ([]int, error) {
			// This fixture owns a link, not a process; the network lifecycle creates it.
			return []int{}, nil
		},
		ready:    []labReadiness{{kind: readinessAnswer, target: "UP", argv: []string{"ip", "-o", ipLink, commandShow, showcaseTrackedLink}}},
		controls: showcaseLinkControls(showcaseTrackedLink),
	}
}

func showcaseTrafficFixture() labFixture {
	return labFixture{
		name: showcaseTraffic,
		start: func(state string, environ []string) ([]int, error) {
			pid, err := startCommand(demoHarness, []string{"mock", "httpd", flagBind, "10.77.0.1:8080", "--directory", demoDir(showcaseLabID)}, environ, filepath.Join(state, "http.log"))
			if err != nil {
				return nil, err
			}
			return []int{pid}, nil
		},
		ready: []labReadiness{{kind: readinessAnswer, target: "Showcase traffic payload", argv: []string{"ip", ipNetns, commandExec, trafficPeerNS, programCurl, "-fsS", curlMaxTime, "2", "http://10.77.0.1:8080/payload.txt"}}},
		controls: map[string]func(string, []string) (string, error){
			"burst": startShowcaseTraffic,
			controlStop: func(state string, _ []string) (string, error) {
				return "Traffic burst stopped", stopShowcaseFixture(state, "traffic-burst")
			},
		},
	}
}

// startShowcaseTraffic owns ICMP and HTTP senders sized for the full recording
// container budget. Callers MUST stop the burst through its stop control or lab
// cleanup; the stop control MUST terminate both senders, not the server or Ze.
func startShowcaseTraffic(state string, environ []string) (string, error) {
	const lifetimeSeconds = int(demoContainerTimeout / time.Second)
	fixture := labFixture{
		name: "traffic-burst",
		start: func(state string, environ []string) ([]int, error) {
			// Include the immediate first send so neither count expires before the budget.
			ping, err := startCommand("ip", []string{ipNetns, commandExec, trafficPeerNS, programPing, "-n", "-i", "0.1", "-c", strconv.Itoa(lifetimeSeconds*10 + 1), "-w", strconv.Itoa(lifetimeSeconds), "10.77.0.1"}, environ, filepath.Join(state, "traffic-ping.log"))
			if err != nil {
				return nil, err
			}
			http, err := startCommand("ip", []string{ipNetns, commandExec, trafficPeerNS, programCurl, "-fsS", "--no-buffer", curlMaxTime, "2", "--rate", "20/s", "--output", "/dev/null", "--write-out", "HTTP %{http_code}\n", fmt.Sprintf("http://10.77.0.1:8080/payload.txt?sample=[1-%d]", lifetimeSeconds*20+1)}, environ, filepath.Join(state, "traffic-http.log"))
			if err != nil {
				_ = terminateShowcasePID(ping) //nolint:errcheck // preserve the failed HTTP launch
				return nil, err
			}
			return []int{ping, http}, nil
		},
		ready: []labReadiness{{kind: readinessAnswer, target: "HTTP 200", argv: []string{"cat", filepath.Join(state, "traffic-http.log")}}},
	}
	if err := startFixtures(state, filepath.Join(state, showcasePIDs), environ, []labFixture{fixture}); err != nil {
		return "", fmt.Errorf("start traffic burst: %w", err)
	}
	return "ICMP and HTTP burst from 10.77.0.2 started", nil
}
