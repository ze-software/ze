package terminaldemo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func validateDemoRuntime(id string, stdout, _ io.Writer) (err error) {
	validator, ok := demoValidators[id]
	if !ok {
		return fmt.Errorf("no validator for demo %q", id)
	}
	if err := validator(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "validated %s output\n", id)
	return err
}

var demoValidators = map[string]func() error{
	"cli-dashboard": validateCLIDashboard, demoZefsConfig: validateZeFSConfig,
	"rbac": validateRBAC, "traceroute": validateTraceroute, "launcher": validateLauncher,
	"web-config": validateWebConfig, "commit-confirmed": validateCommitConfirmed,
	demoRPKI: validateRPKI, "irr-filter": validateIRR, "rib-fib": validateRIBFIB,
	"health-reports": validateHealthReports, demoConfigViews: validateConfigViews,
	"bfd-failover": validateBFD, "ospf-adjacency": validateOSPF,
	"traffic-anomaly": validateTraffic, "vrrp-failover": validateVRRP,
	"host-inventory": validateHostInventory, "config-graph": validateConfigGraph,
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

func runPTYFixture(args ...string) (string, error) {
	output, err := runCommand(demoBinary("ze-terminal-pty"), args, commandOptions{env: demoEnvironment()})
	return string(output), err
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
		peers, _ = cli(env, showPeerListRaw)
		if strings.Count(peers, `"state": "established"`) == 3 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if strings.Count(peers, `"state": "established"`) != 3 {
		return fmt.Errorf("validation failed: expected three established sessions\n%s", peers)
	}
	output, err := runPTYFixture("--timeout", "20", "--command", "exit", "--command", `@wait operational\]`, "--command", "monitor bgp", "--command", `@wait 127\.0\.0\.4`, "--command", "@type s", "--command", `@wait ASN \^`, "--command", "@key down", "--command", "@key enter", "--command", `@wait Peer Detail: 127\.0\.0\.2`, "--command", "@escape", "--command", "@type q", "--command", `@wait operational\]`, "--command", "exit", "--", "sshpass", "-e", "ssh", "ze-demo")
	if err != nil {
		return err
	}
	return requireAll(output, "ASN ^", "Peer Detail: 127.0.0.2")
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
	listed, err := runZe([]string{commandConfig, "ls"}, env, nil)
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
	output, err := runPTYFixture("--command", "set environment cli format default table", "--command", "show | compare", "--command", "commit", "--command", "@wait Session committed", "--command", "exit", "--command", `@wait operational\]`, "--command", "exit", "--", "sshpass", "-e", "ssh", "ze-demo")
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
	denied, err := runZe([]string{commandCLI, "--user", "noc", "-c", "clear interface counters"}, env, nil)
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
	output, err := runPTYFixture("--ready", "Operations", "--command", "@type show", "--command", "@wait filter: show", "--command", "@key enter", "--command", "@wait > show", "--command", "@escape", "--command", "@wait Operations", "--command", "@type doctor", "--command", "@wait filter: doctor", "--command", "@escape", "--command", "@escape", "--", "ze")
	if err != nil {
		return err
	}
	return requireAll(output, "filter: show", "> show", "filter: doctor")
}

func validateWebConfig() error {
	defer func() { _ = runWebConfig(commandStop) }()
	if err := runWebConfig(commandStart); err != nil {
		return err
	}
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} // #nosec G402 -- fixed loopback demo server uses a self-signed certificate.
	ctx := context.Background()
	request := func(method, path string, form url.Values) (string, error) {
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, "https://127.0.0.1:8443"+path, body)
		if err != nil {
			return "", err
		}
		req.SetBasicAuth("admin", demoPassword)
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", "https://127.0.0.1:8443")
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return string(data), fmt.Errorf("web demo returned %s", resp.Status)
		}
		return string(data), nil
	}
	editor, err := request(http.MethodGet, "/show/system/identity/", nil)
	if err != nil {
		return err
	}
	if err := contains(editor, "System Identity"); err != nil {
		return err
	}
	if _, err := request(http.MethodPost, "/config/form/", url.Values{"field:system/host": {"edge-demo"}}); err != nil {
		return err
	}
	diff, err := request(http.MethodGet, "/config/diff", nil)
	if err != nil {
		return err
	}
	if err := contains(diff, "host edge-demo"); err != nil {
		return err
	}
	if _, err := request(http.MethodPost, "/config/commit", url.Values{}); err != nil {
		return err
	}
	active, err := request(http.MethodGet, "/show/system/identity/", nil)
	if err != nil {
		return err
	}
	return contains(active, `value="edge-demo"`)
}

func validateCommitConfirmed() error {
	state := demoState("commit-confirmed")
	if err := os.RemoveAll(state); err != nil {
		return err
	}
	if err := os.MkdirAll(state, 0o750); err != nil {
		return err
	}
	config := filepath.Join(state, "ze.conf")
	data, err := os.ReadFile(filepath.Join(demoDir("commit-confirmed"), "identity.conf"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(config, data, 0o600); err != nil {
		return err
	}
	output, err := runPTYFixture("--delay", "2", "--command", "show system host", "--command", "set system host edge-trial", "--command", "show | compare", "--command", "commit confirmed 5", "--command", "@wait Confirm within", "--command", "show system host", "--command", "@wait automatically rolled back", "--command", "show system host", "--command", "set system host edge-confirmed", "--command", "commit confirmed 5", "--command", "@wait Confirm within", "--command", "confirm", "--command", "@wait confirmed and saved permanently", "--command", "@sleep 7", "--command", "show system host", "--command", "exit", "--command", `@wait operational\]`, "--command", "@escape", "--command", `@wait Quit\?`, "--command", "@escape", "--", "ze", "config", "edit", "-f", config)
	if err != nil {
		return err
	}
	if err := requireAll(output, "edge-original", "edge-trial", "Confirm within", "automatically rolled back", "confirmed and saved permanently"); err != nil {
		return err
	}
	afterTimeout := strings.SplitN(output, "automatically rolled back", 2)
	if len(afterTimeout) != 2 {
		return errors.New("rollback marker missing")
	}
	if err := contains(afterTimeout[1], "edge-original"); err != nil {
		return err
	}
	current, _ := os.ReadFile(config) //nolint:gosec // the path comes from the closed demo scenario table
	if err := notContains(string(current), "edge-trial"); err != nil {
		return err
	}
	afterConfirm := strings.SplitN(output, "confirmed and saved permanently", 2)
	if len(afterConfirm) != 2 {
		return errors.New("confirmation marker missing")
	}
	if err := contains(afterConfirm[1], "edge-confirmed"); err != nil {
		return err
	}
	return contains(string(current), "edge-confirmed")
}

func validateRPKI() error {
	const id = demoRPKI
	defer func() { _ = runRPKI(commandStop) }()
	if err := runRPKI(actionPrepare); err != nil {
		return err
	}
	if err := initializeStore(id, filepath.Join(demoState(id), "init.input")); err != nil {
		return err
	}
	if err := runRPKI(commandStart); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	status, err := waitForCommandText(100, fmt.Sprintf("vrp-count-ipv4: %d", expectedDemoVRPIPv4), func() (string, error) { return cli(env, "show bgp rpki status | yaml") })
	if err != nil {
		return err
	}
	if err := requireAll(status, "sessions-synced: 1", "synced: true", fmt.Sprintf("vrp-count-ipv4: %d", expectedDemoVRPIPv4)); err != nil {
		return err
	}
	var routes string
	for range 100 {
		routes, _ = cli(env, "show bgp adj-rib-in | no-more | yaml")
		if strings.Contains(routes, "9.43.0.0/24") && strings.Contains(routes, "11.43.0.0/24") && !strings.Contains(routes, "10.43.0.0/24") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := requireAll(routes, "9.43.0.0/24", "11.43.0.0/24", "validation-state"); err != nil {
		return err
	}
	return notContains(routes, "10.43.0.0/24")
}

func validateIRR() error {
	const id = "irr-filter"
	defer func() { _ = runIRR(commandStop) }()
	if err := runIRR(actionPrepare); err != nil {
		return err
	}
	if err := initializeStore(id, filepath.Join(demoState(id), "init.input")); err != nil {
		return err
	}
	if err := runIRR("seed"); err != nil {
		return err
	}
	env := scenarioEnv(id, demoPassword)
	before, _ := runZe([]string{commandConfig, commandCat, zeConfigFile}, env, nil)
	if err := requireNone(before, "bgp-filter-irr", "AS-TEST"); err != nil {
		return err
	}
	sets := [][]string{{"plugin", "internal", "bgp-filter-irr", "use", "bgp-filter-irr"}, {commandBGP, "policy", demoIRR, "server", "127.0.0.1:4343"}, {commandBGP, "policy", demoIRR, "refresh-interval", "3600"}, {commandBGP, ipPeer, "customer-a", "session", demoIRR, "as-set", "AS-TEST"}, {commandBGP, ipPeer, "customer-a", "filter", "import", "bgp-filter-irr:65001"}}
	for _, tail := range sets {
		args := append([]string{commandConfig, ipSet, zeConfigFile}, tail...)
		if _, err := runZe(args, env, nil); err != nil {
			return err
		}
	}
	configured, _ := runZe([]string{commandConfig, commandCat, zeConfigFile}, env, nil)
	if err := requireAll(configured, "bgp-filter-irr", "AS-TEST", "127.0.0.1:4343"); err != nil {
		return err
	}
	if err := runIRR(commandStart); err != nil {
		return err
	}
	status, err := waitForCommandText(100, "status: ok", func() (string, error) { return cli(env, "show bgp irr | no-more | yaml") })
	if err != nil {
		return err
	}
	if err := requireAll(status, "AS-TEST", "ipv4-count: 3", "status: ok"); err != nil {
		return err
	}
	prefixes, _ := cli(env, "show bgp irr prefix customer-a | no-more | yaml")
	if err := requireAll(prefixes, "10.0.0.0/24", "2001:db8::/32"); err != nil {
		return err
	}
	allowed, _ := cli(env, "show bgp irr check customer-a 10.0.0.0/24 | no-more | yaml")
	if err := requireAll(allowed, "accepted: true", "matched-entry: 10.0.0.0/24"); err != nil {
		return err
	}
	rejected, _ := cli(env, "show bgp irr check customer-a 192.168.0.0/24 | no-more | yaml")
	if err := contains(rejected, "accepted: false"); err != nil {
		return err
	}
	if err := runIRR("announce"); err != nil {
		return err
	}
	routes, err := waitForCommandText(100, "10.0.0.0/24", func() (string, error) { return cli(env, "show bgp adj-rib-in | no-more | yaml") })
	if err != nil {
		return err
	}
	if err := contains(routes, "10.0.0.0/24"); err != nil {
		return err
	}
	return notContains(routes, "192.168.0.0/24")
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
		best, _ = cli(env, "show bgp rib best | no-more")
		rib, _ = cli(env, "show rib | no-more")
		out, _ := runCommand("ip", []string{"-details", ipRoute, commandShow, ipExact, prefix}, commandOptions{})
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
		out, _ := runCommand("ip", []string{ipRoute, commandShow, ipExact, prefix}, commandOptions{})
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
		warnings, _ = cli(env, "show warnings source bgp | no-more | yaml")
		peers, _ = cli(env, "show bgp peer list | no-more | yaml")
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
	setView, err := pipeline([][]string{{"ze", commandConfig, commandMigrate, flagFormat, ipSet, config}, {"ze", pipeKeyword, matchKeyword, "bgp peer transit-a"}}, demoEnvironment())
	if err != nil {
		return err
	}
	if err := requireAll(setView, "set bgp peer transit-a connection local ip 192.0.2.1", "set bgp peer transit-a session asn remote 65001"); err != nil {
		return err
	}
	left, _ := os.ReadFile(filepath.Join(demoState(id), "router.set"))
	right, _ := os.ReadFile(filepath.Join(demoState(id), "roundtrip.set"))
	if !bytes.Equal(left, right) {
		return errors.New("validation failed: hierarchical/set round trip changed canonical output")
	}
	matches, err := pipeline([][]string{{"ze", "--plugins"}, {"ze", pipeKeyword, matchKeyword, "flowspec"}}, demoEnvironment())
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

func validateBFD() error {
	defer func() { _ = runBFD(commandStop, nil, io.Discard) }()
	if err := runBFD(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := runBFD(commandStart, nil, io.Discard); err != nil {
		return err
	}
	var bfd, bgp bytes.Buffer
	if err := runBFD(commandCLI, []string{"show bfd sessions | raw"}, &bfd); err != nil {
		return err
	}
	if err := runBFD(commandCLI, []string{showPeerListRaw}, &bgp); err != nil {
		return err
	}
	if err := requireAll(bfd.String(), `"peer": "172.30.0.3"`, `"state": "up"`); err != nil {
		return err
	}
	if err := requireAll(bgp.String(), `"name": "edge-peer"`, `"state": "established"`); err != nil {
		return err
	}
	if err := runBFD("cut", nil, io.Discard); err != nil {
		return err
	}
	bfd.Reset()
	bgp.Reset()
	if err := runBFD(commandCLI, []string{"show bfd sessions | raw"}, &bfd); err != nil {
		return err
	}
	if err := runBFD(commandCLI, []string{showPeerListRaw}, &bgp); err != nil {
		return err
	}
	if strings.TrimSpace(bfd.String()) != "[]" {
		return fmt.Errorf("validation failed: expected an empty BFD session list after the cut\n%s", bfd.String())
	}
	if err := contains(bgp.String(), `"name": "edge-peer"`); err != nil {
		return err
	}
	if err := notContains(bgp.String(), `"state": "established"`); err != nil {
		return err
	}
	if err := runBFD("restore", nil, io.Discard); err != nil {
		return err
	}
	bgp.Reset()
	if err := runBFD(commandCLI, []string{showPeerListRaw}, &bgp); err != nil {
		return err
	}
	return requireAll(bgp.String(), `"name": "edge-peer"`, `"state": "established"`)
}

func validateOSPF() error {
	defer func() { _ = runOSPF(commandStop, nil, io.Discard) }()
	if err := runOSPF(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := runOSPF(commandStart, nil, io.Discard); err != nil {
		return err
	}
	var output bytes.Buffer
	if err := runOSPF(commandCLI, []string{"show ospf neighbor detail"}, &output); err != nil {
		return err
	}
	if err := requireAll(output.String(), "full", "172.31.0.3"); err != nil {
		return err
	}
	output.Reset()
	if err := runOSPF(commandCLI, []string{"show ospf database router"}, &output); err != nil {
		return err
	}
	if err := contains(output.String(), "172.31.0.3"); err != nil {
		return err
	}
	output.Reset()
	if err := runOSPF(commandCLI, []string{"show ospf route"}, &output); err != nil {
		return err
	}
	return contains(output.String(), "10.255.0.3")
}

func validateTraffic() error {
	defer func() { _ = runTraffic(commandStop, io.Discard) }()
	if err := runTraffic(actionPrepare, io.Discard); err != nil {
		return err
	}
	if err := runTraffic(commandStart, io.Discard); err != nil {
		return err
	}
	var before, after bytes.Buffer
	if err := runTraffic(commandShow, &before); err != nil {
		return err
	}
	if err := contains(before.String(), `"interface":"traffic0"`); err != nil {
		return err
	}
	if err := notContains(before.String(), "10.77.0.2"); err != nil {
		return err
	}
	if err := runTraffic("generate", io.Discard); err != nil {
		return err
	}
	if err := runTraffic(commandShow, &after); err != nil {
		return err
	}
	if err := requireAll(after.String(), `"interface":"traffic0"`, `"ip":"10.77.0.2"`, `"port":8080`, `"protocol":"icmp"`); err != nil {
		return err
	}
	beforeBytes := jsonIPBytes(before.Bytes(), "10.77.0.2")
	afterBytes := jsonIPBytes(after.Bytes(), "10.77.0.2")
	if afterBytes <= beforeBytes {
		return fmt.Errorf("validation failed: bytes attributed to 10.77.0.2 did not rise: %d -> %d", beforeBytes, afterBytes)
	}
	return nil
}

func jsonIPBytes(data []byte, ip string) int64 {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return 0
	}
	var total int64
	var walk func(any)
	walk = func(item any) {
		switch typed := item.(type) {
		case map[string]any:
			if typed["ip"] == ip {
				for _, key := range []string{"bytes", "rx-bytes", "tx-bytes"} {
					if number, ok := typed[key].(float64); ok {
						total += int64(number)
					}
				}
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return total
}

func validateVRRP() error {
	defer func() { _ = runVRRP(commandStop, io.Discard) }()
	if err := runVRRP(actionPrepare, io.Discard); err != nil {
		return err
	}
	if err := runVRRP(commandStart, io.Discard); err != nil {
		return err
	}
	var show, before, after bytes.Buffer
	if err := runVRRP(commandShow, &show); err != nil {
		return err
	}
	if err := runVRRP("owner", &before); err != nil {
		return err
	}
	if err := contains(show.String(), "master"); err != nil {
		return err
	}
	if err := requireAll(before.String(), "VIP owner: Ze", "00:00:5e:00:01:0a"); err != nil {
		return err
	}
	if err := runVRRP("failover", &after); err != nil {
		return err
	}
	return requireAll(after.String(), "VIP owner: keepalived", "00:00:5e:00:01:0a", "2/2 probes answered")
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
	if err := requireAll(cpu, fmt.Sprintf(`"logical-cpus": %d`, logical), `"model-name": "`+model+`"`); err != nil {
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

// The ze-test peer mode the demos run, the iproute2 next-hop keyword, and the
// CLI pipe keyword the runtime validator asserts on.
const (
	peerModeSink = "sink"
	ipVia        = "via"
	pipeKeyword  = "pipe"
)
