package siteterminaldemo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func validateDemoRuntime(id string, stdout, _ io.Writer) (err error) {
	sc, err := scenarios.lookup(id)
	if err != nil {
		return err
	}
	if err := sc.validate(); err != nil {
		return err
	}
	_, err = io.WriteString(stdout, "validated "+id+" output\n")
	return err
}

func contains(value, expected string) error {
	if strings.Contains(value, expected) {
		return nil
	}
	return fmt.Errorf("validation failed: expected output containing %q\n%s", expected, value)
}
func notContains(value, unexpected string) error {
	if !strings.Contains(value, unexpected) {
		return nil
	}
	return fmt.Errorf("validation failed: output unexpectedly contained %q\n%s", unexpected, value)
}
func requireAll(value string, expected ...string) error {
	for _, item := range expected {
		if err := contains(value, item); err != nil {
			return err
		}
	}
	return nil
}
func requireNone(value string, unexpected ...string) error {
	for _, item := range unexpected {
		if err := notContains(value, item); err != nil {
			return err
		}
	}
	return nil
}

// runPTYFixture drives `le site terminal-demo pty` under environ. A fixture that
// reaches the daemon over SSH MUST be given the scenario environment: sshpass
// reads the password from SSHPASS, which scenarioEnv sets and demoEnvironment
// does not.
func runPTYFixture(environ []string, args ...string) (string, error) {
	name, arguments := recorderCommand(args...)
	output, err := runCommand(name, arguments, commandOptions{env: environ, deadline: ptyFixtureDeadline})
	return string(output), err
}

// ptyFixtureDeadline bounds one whole recorder session. The recorder's
// --timeout bounds each wait inside the session, and a session holds several
// waits and authored sleeps, so the whole gets ten minutes where runCommand's
// default would cut a session that is still making progress.
const ptyFixtureDeadline = 10 * time.Minute

// runPTYFixtureObserved runs the recorder as runPTYFixture does and calls
// observe while the session is open. A check uses it to read lab state that
// exists only during the session, such as the store inside a confirmed
// commit's window, which the transcript alone cannot prove. It answers the
// transcript; the session's own failure is reported before observe's.
func runPTYFixtureObserved(environ []string, observe func() error, args ...string) (string, error) {
	name, arguments := recorderCommand(args...)
	session, err := beginCommand(name, arguments, commandOptions{env: environ, deadline: ptyFixtureDeadline})
	if err != nil {
		return "", err
	}
	observed := observe()
	output, err := session.wait()
	if err != nil {
		return string(output), errors.Join(err, observed)
	}
	return string(output), observed
}

func validateCLIDashboard() error {
	const id = "cli-dashboard"
	defer func() { _ = runCLIDashboard(commandStop) }()
	if err := runCLIDashboard(commandStart); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	var peers string
	for range 100 {
		var readErr error
		peers, readErr = cli(env, showPeerListRaw)
		if hung := hungCommand(readErr); hung != nil {
			return hung
		}
		if strings.Count(peers, `"state": "established"`) == 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if strings.Count(peers, `"state": "established"`) != 3 {
		return fmt.Errorf("validation failed: expected three established sessions\n%s", peers)
	}
	// The keystrokes are the tape's: the table opens in address order with
	// 127.0.0.2 on the first row, `s` re-orders it by ASN to .4, .2, .3, and the
	// highlight follows its peer to the second row. One `down` therefore reaches
	// 127.0.0.3, and Enter opens that session.
	//
	// The peer this ends on is a FIXED answer, and it was not always one. The
	// highlight used to stay at a row NUMBER across a re-sort, so `down` reached
	// .2 or .3 depending on whether a poll landed in between and re-resolved the
	// index (comparePeers and followSelection, internal/component/cli).
	output, err := runPTYFixture(env, cliDashboardSession()...)
	if err != nil {
		return err
	}
	return requireAll(output, "ASN ^", "Peer Detail: 127.0.0.3")
}

// cliDashboardSession answers the recorder arguments of the dashboard demo's
// check, which validateCLIDashboard explains key by key.
func cliDashboardSession() []string {
	return []string{flagTimeout, "20",
		flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine),
		flagCommand, "monitor bgp", flagCommand, ptyWait(`127\.0\.0\.4`),
		flagCommand, ptyType("s"), flagCommand, ptyWait(`ASN \^`),
		flagCommand, ptyKey(ptyKeyDown), flagCommand, ptyKey(keyEnter),
		flagCommand, ptyWait(`Peer Detail: 127\.0\.0\.3`),
		flagCommand, tapeEscapeDirective, flagCommand, ptyType("q"), flagCommand, dashboardReturnWait,
		flagCommand, cliExitCommand, "--", "sshpass", "-e", "ssh", "ze-demo"}
}

func validateZeFSConfig() error {
	const id = demoZefsConfig
	defer func() { _ = runZeFSConfig(commandStop) }()
	if err := runZeFSConfig(actionPrepare); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	input := filepath.Join(demoState(id), "init.input")
	if err := initializeStore(id, input); err != nil {
		return err
	}
	listed, err := runZe([]string{commandConfig, "list"}, env, nil)
	if err != nil {
		return err
	}
	if err := contains(listed, zeConfigFile); err != nil {
		return err
	}
	if err := runZeFSConfig(commandStart); err != nil {
		return err
	}
	before, err := cli(env, "show bgp")
	if err != nil {
		return err
	}
	if err := requireAll(before, "router-id"); err != nil {
		return err
	}
	if err := requireNone(before, "┌"); err != nil {
		return err
	}
	output, err := runPTYFixture(env, flagCommand, "set environment cli format default table", flagCommand, "show | compare", flagCommand, "commit now", flagCommand, ptyWait(sessionCommitted), flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine), flagCommand, cliExitCommand, "--", "sshpass", "-e", "ssh", "ze-demo")
	if err != nil {
		return err
	}
	after, err := cli(env, "show bgp")
	if err != nil {
		return err
	}
	if err := requireAll(output, "default table", "Session committed", "router-id"); err != nil {
		return err
	}
	if err := contains(after, "┌"); err != nil {
		return err
	}
	active, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, env, nil)
	if err != nil {
		return err
	}
	if err := contains(active, "default table"); err != nil {
		return err
	}
	text, _ := cli(env, "show bgp | text")
	if err := notContains(text, "┌"); err != nil {
		return err
	}
	raw, _ := cli(env, "show bgp | raw")
	if err := contains(raw, `"router-id"`); err != nil {
		return err
	}
	displayed, _ := cli(env, "show bgp | display router-id local-as peers-established")
	if err := requireAll(displayed, "router-id", "peers-established"); err != nil {
		return err
	}
	if err := notContains(displayed, "peers-configured"); err != nil {
		return err
	}
	filled, _ := cli(env, "show bgp | display router-id | fill alpha")
	if err := contains(filled, "peers-configured"); err != nil {
		return err
	}
	peers, _ := cli(env, "show bgp | peers")
	if err := contains(peers, ipAddress); err != nil {
		return err
	}
	return notContains(peers, "peers-established")
}

func validateRBAC() error {
	const id = "rbac"
	defer func() { _ = runRBAC(commandStop) }()
	if err := runRBAC(commandStart); err != nil {
		return err
	}
	env := scenarioEnv(id, "noc-secret")
	allowed, err := runZe([]string{commandCLI, "--user", "noc", "-c", "show version"}, env, nil)
	if err != nil {
		return err
	}
	version, err := runZe([]string{commandVersion}, env, nil)
	if err != nil {
		return err
	}
	if err := contains(allowed, strings.TrimSpace(version)); err != nil {
		return err
	}
	denied, err := runZe([]string{commandCLI, "--user", "noc", "-c", "clear interface counters all"}, env, nil)
	if err == nil {
		return errors.New("validation failed: denied RBAC command succeeded")
	}
	return contains(denied, "restricted by access control")
}

func validateTraceroute() error {
	const id = "traceroute"
	defer func() { _ = runTraceroute(commandStop) }()
	if err := runTraceroute(commandStart); err != nil {
		return err
	}
	output, err := cli(scenarioEnv(id, demoPassword), "show traceroute 192.0.2.53 | json")
	if err != nil {
		return err
	}
	return requireAll(output, "198.51.100.2", "192.0.2.53")
}

func validateLauncher() error {
	output, err := runPTYFixture(demoEnvironment(), flagReady, "Operations", flagCommand, ptyType("show"), flagCommand, ptyWait("filter: show"), flagCommand, ptyKey(keyEnter), flagCommand, ptyWait("> show"), flagCommand, tapeEscapeDirective, flagCommand, ptyWait("Operations"), flagCommand, ptyType("doctor"), flagCommand, ptyWait("filter: doctor"), flagCommand, tapeEscapeDirective, flagCommand, tapeEscapeDirective, "--", "ze")
	if err != nil {
		return err
	}
	return requireAll(output, "filter: show", "> show", "filter: doctor")
}

func validateWebConfig() error {
	return runWebConfigJourney(false)
}

func validateRIBFIB() error {
	const id = "rib-fib"
	const prefix = "198.51.100.0/24"
	defer func() { _ = runRIBFIB(commandStop, nil, io.Discard) }()
	if err := runRIBFIB(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := initializeStore(id, filepath.Join(demoState(id), "init.input")); err != nil {
		return err
	}
	if err := runRIBFIB(commandStart, nil, io.Discard); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	inject, err := cli(env, "request bgp rib inject 192.0.2.10 ipv4/unicast "+prefix+" origin igp nexthop 127.0.0.1 med 42")
	if err != nil {
		return err
	}
	if err := notContains(inject, "error"); err != nil {
		return err
	}
	var best, rib, kernel string
	for range 100 {
		var bestErr, ribErr error
		best, bestErr = cli(env, "show bgp rib best | no-more")
		rib, ribErr = cli(env, "show rib | no-more")
		out, kernelErr := runCommand("ip", []string{"-details", ipRoute, commandShow, ipExact, prefix}, commandOptions{})
		if hung := hungCommand(bestErr, ribErr, kernelErr); hung != nil {
			return hung
		}
		kernel = string(out)
		if strings.Contains(best, prefix) && strings.Contains(rib, prefix) && strings.Contains(kernel, prefix) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := requireAll(best, prefix); err != nil {
		return err
	}
	if err := requireAll(rib, prefix); err != nil {
		return err
	}
	if err := requireAll(kernel, prefix, "proto 250"); err != nil {
		return err
	}
	withdraw, err := cli(env, "request bgp rib withdraw 192.0.2.10 ipv4/unicast "+prefix)
	if err != nil {
		return err
	}
	if err := notContains(withdraw, "error"); err != nil {
		return err
	}
	for range 100 {
		out, readErr := runCommand("ip", []string{ipRoute, commandShow, ipExact, prefix}, commandOptions{})
		if hung := hungCommand(readErr); hung != nil {
			return hung
		}
		kernel = strings.TrimSpace(string(out))
		if kernel == "" {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("validation failed: kernel route survived withdrawal\n%s", kernel)
}

func validateHealthReports() error {
	const id = "health-reports"
	defer func() { _ = runHealthReports(commandStop) }()
	if err := runHealthReports(actionPrepare); err != nil {
		return err
	}
	if err := initializeStore(id, filepath.Join(demoState(id), "init.input")); err != nil {
		return err
	}
	if err := runHealthReports(commandStart); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	var warnings, peers string
	for range 100 {
		var warningsErr, peersErr error
		warnings, warningsErr = cli(env, "show warnings source bgp | no-more | yaml")
		peers, peersErr = cli(env, "show bgp peer list | no-more | yaml")
		if hung := hungCommand(warningsErr, peersErr); hung != nil {
			return hung
		}
		if strings.Contains(warnings, "prefix-stale") && strings.Contains(strings.ToLower(peers), "established") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := requireAll(warnings, "prefix-stale", "2024-01-01", loopbackPeerAddress); err != nil {
		return err
	}
	health, _ := cli(env, "show health | no-more | yaml")
	if err := contains(health, "status: down"); err != nil {
		return err
	}
	teardown, err := cli(env, "request peer 127.0.0.2 teardown 4 | yaml")
	if err != nil {
		return err
	}
	if err := requireAll(teardown, "peer: 127.0.0.2", "subcode: 4"); err != nil {
		return err
	}
	events, err := waitForCommandText(100, "notification-sent", func() (string, error) { return cli(env, "show errors source bgp | no-more | yaml") })
	if err != nil {
		return err
	}
	return requireAll(events, "notification-sent", "direction: sent", "subcode: 4")
}

func validateConfigViews() error {
	const id = demoConfigViews
	if err := runConfigViews(actionPrepare); err != nil {
		return err
	}
	config := filepath.Join(demoDir(id), "router.conf")
	tree, err := runZe([]string{commandConfig, commandShow, config, commandBGP, ipPeer, "transit-a"}, demoEnvironment(), nil)
	if err != nil {
		return err
	}
	if err := requireAll(tree, "local ip 192.0.2.1", "remote ip 192.0.2.2"); err != nil {
		return err
	}
	setView, err := pipeline([][]string{{"ze", commandConfig, commandMigrate, keywordFormat, ipSet, config}, {"ze", pipeKeyword, matchKeyword, "bgp peer transit-a"}}, demoEnvironment())
	if err != nil {
		return err
	}
	if err := requireAll(setView, "set bgp peer transit-a connection local ip 192.0.2.1", "set bgp peer transit-a session asn remote 65001"); err != nil {
		return err
	}
	// Two unreadable files would compare equal as two empty slices, so a
	// failed read is the verdict rather than an input to the comparison.
	original, err := os.ReadFile(filepath.Join(demoState(id), "router.set"))
	if err != nil {
		return err
	}
	roundTrip, err := os.ReadFile(filepath.Join(demoState(id), "roundtrip.set"))
	if err != nil {
		return err
	}
	if !bytes.Equal(original, roundTrip) {
		return errors.New("validation failed: hierarchical/set round trip changed canonical output")
	}
	matches, err := pipeline([][]string{{"ze", commandShow, "plugin", "list"}, {"ze", pipeKeyword, matchKeyword, "flowspec"}}, demoEnvironment())
	if err != nil {
		return err
	}
	if err := contains(matches, "bgp-nlri-flowspec"); err != nil {
		return err
	}
	count, err := pipelineInput(matches, [][]string{{"ze", pipeKeyword, "count"}}, demoEnvironment())
	if err != nil {
		return err
	}
	var object struct {
		Count int `json:"count"`
	}
	if json.Unmarshal([]byte(count), &object) != nil || object.Count < 1 {
		return fmt.Errorf("validation failed: expected positive FlowSpec plugin count, got %s", count)
	}
	return nil
}

func validateHostInventory() error {
	env := demoEnvironment()
	kernel, err := runZe([]string{commandShow, commandHost, "kernel"}, env, nil)
	if err != nil {
		return err
	}
	cpu, err := runZe([]string{commandShow, commandHost, "cpu"}, env, nil)
	if err != nil {
		return err
	}
	memory, err := runZe([]string{commandShow, commandHost, "memory"}, env, nil)
	if err != nil {
		return err
	}
	nic, err := runZe([]string{commandShow, commandHost, "nic"}, env, nil)
	if err != nil {
		return err
	}
	release, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	arch := runtime.GOARCH
	if err := requireAll(kernel, `"release": "`+strings.TrimSpace(string(release))+`"`, `"architecture": "`+arch+`"`); err != nil {
		return err
	}
	cpuInfo, _ := os.ReadFile("/proc/cpuinfo")
	logical := strings.Count(string(cpuInfo), "processor\t:")
	model := ""
	for line := range strings.SplitSeq(string(cpuInfo), "\n") {
		if strings.HasPrefix(line, "model name") {
			_, model, _ = strings.Cut(line, ":")
			model = strings.TrimSpace(model)
			break
		}
	}
	// An aarch64 /proc/cpuinfo carries no `model name` line, and the product
	// builds the name from `CPU implementer` and `CPU part`
	// (internal/component/host/cpu_linux.go). `model-name` is omitempty, so
	// requiring the key alone requires a name, and the demo does not restate a
	// derivation the product owns.
	modelAssertion := `"model-name": "`
	if model != "" {
		modelAssertion = modelAssertion + model + `"`
	}
	if err := requireAll(cpu, `"logical-cpus": `+strconv.Itoa(logical), modelAssertion); err != nil {
		return err
	}
	memInfo, _ := os.ReadFile("/proc/meminfo")
	total := meminfoBytes(string(memInfo), "MemTotal")
	if err := contains(memory, fmt.Sprintf(`"total-bytes": %d`, total)); err != nil {
		return err
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(memory), &parsed); err != nil {
		return err
	}
	available, _ := parsed["available-bytes"].(float64)
	reportedTotal, _ := parsed["total-bytes"].(float64)
	if available <= 0 || available > reportedTotal {
		return fmt.Errorf("validation failed: available-bytes %v is not within (0, %v]", available, reportedTotal)
	}
	if !strings.HasPrefix(strings.TrimSpace(nic), "[") {
		return fmt.Errorf("validation failed: NIC inventory is not a JSON array\n%s", nic)
	}
	return nil
}
func meminfoBytes(content, key string) int64 {
	for line := range strings.SplitSeq(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == key+":" {
			value, _ := strconv.ParseInt(fields[1], 10, 64)
			return value * 1024
		}
	}
	return 0
}

func validateConfigGraph() error {
	config := filepath.Join(demoDir("config-graph"), "router.conf")
	validation, err := runZe([]string{commandConfig, commandValidate, config}, demoEnvironment(), nil)
	if err != nil {
		return err
	}
	if err := contains(validation, "configuration valid"); err != nil {
		return err
	}
	graph, err := runZe([]string{commandConfig, "graph", config}, demoEnvironment(), nil)
	if err != nil {
		return err
	}
	if err := requireAll(graph, `"nodes"`, `"edges"`, "peer/upstream-a", "peer/upstream-b", "group/transit", `"kind": "inherits"`); err != nil {
		return err
	}
	for _, needle := range []string{"peer/upstream", "group/transit", "inherits"} {
		view, err := pipelineInput(graph, [][]string{{"ze", pipeKeyword, "text"}, {"ze", pipeKeyword, matchKeyword, needle}}, demoEnvironment())
		if err != nil {
			return err
		}
		if err := contains(view, needle); err != nil {
			return err
		}
		if needle == "inherits" {
			if err := requireAll(view, "peer/upstream-a", "peer/upstream-b", "group/transit"); err != nil {
				return err
			}
		}
	}
	return nil
}

func pipeline(commands [][]string, environ []string) (string, error) {
	return pipelineInput("", commands, environ)
}
func pipelineInput(input string, commands [][]string, environ []string) (string, error) {
	current := []byte(input)
	for _, argv := range commands {
		if len(argv) == 0 {
			return "", errors.New("empty pipeline command")
		}
		output, err := runCommand(argv[0], argv[1:], commandOptions{stdin: bytes.NewReader(current), env: environ})
		if err != nil {
			return string(output), err
		}
		current = output
	}
	return string(current), nil
}

// The `le test peer` mode the demos run, the iproute2 next-hop keyword, and the
// CLI pipe keyword the runtime validator asserts on.
const (
	peerModeSink = "sink"
	ipVia        = "via"
	pipeKeyword  = "pipe"
)
