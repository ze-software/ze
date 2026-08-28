package terminaldemo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const demoPassword = "secret123"

func runScenario(id, action string, args []string, stdout, stderr io.Writer) error {
	var err error
	switch id {
	case "cli-dashboard":
		err = runCLIDashboard(action)
	case "zefs-config":
		err = runZeFSConfig(action)
	case "rbac":
		err = runRBAC(action)
	case "traceroute":
		err = runTraceroute(action)
	case "web-config":
		err = runWebConfig(action)
	case "rpki":
		err = runRPKI(action)
	case "irr-filter":
		err = runIRR(action)
	case "rib-fib":
		err = runRIBFIB(action, args, stdout)
	case "health-reports":
		err = runHealthReports(action)
	case "config-views":
		err = runConfigViews(action)
	case "bfd-failover":
		err = runBFD(action, args, stdout)
	case "ospf-adjacency":
		err = runOSPF(action, args, stdout)
	case "traffic-anomaly":
		err = runTraffic(action, stdout)
	case "vrrp-failover":
		err = runVRRP(action, stdout)
	default:
		return fmt.Errorf("unknown demo %q", id)
	}
	if err != nil {
		reportDemoLogs(id, stderr)
	}
	return err
}

func scenarioEnv(id, password string) []string {
	environ := demoEnvironment()
	environ = setEnv(environ, "ZE_CONFIG_DIR", filepath.Join(demoState(id), "config"))
	if password != "" {
		environ = setEnv(environ, "ZE_SSH_PASSWORD", password)
		environ = setEnv(environ, "SSHPASS", password)
	}
	return environ
}

func initText(user, password string) string {
	return strings.Join([]string{user, password, "127.0.0.1", "2222", "ze-demo", ""}, "\n")
}

func stopScenario(id, pidName string) {
	stopPIDs(filepath.Join(demoState(id), pidName))
}

func prepareScenario(id, pidName string, initialize bool) error {
	stopScenario(id, pidName)
	state := demoState(id)
	if err := os.RemoveAll(state); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(state, "config"), 0o750); err != nil {
		return err
	}
	inputPath := filepath.Join(state, "init.input")
	if err := os.WriteFile(inputPath, []byte(initText("admin", demoPassword)), 0o600); err != nil {
		return err
	}
	if initialize {
		return initializeStore(id, inputPath)
	}
	return nil
}

func initializeStore(id, inputPath string) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	_, runErr := runZe([]string{"init"}, scenarioEnv(id, demoPassword), input)
	closeErr := input.Close()
	if runErr != nil {
		return runErr
	}
	return closeErr
}

func importScenarioConfig(id string, sources ...string) error {
	state := demoState(id)
	environ := scenarioEnv(id, demoPassword)
	activePath := filepath.Join(state, "active.conf")
	active, err := runZe([]string{"config", "cat", "ze.conf"}, environ, nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(activePath, []byte(active), 0o600); err != nil {
		return err
	}
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(activePath, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return runForegroundLog("ze", []string{"config", "import", "--name", "ze.conf", activePath}, environ, filepath.Join(state, "import.log"), nil)
}

func runForegroundLog(name string, args []string, environ []string, path string, input io.Reader) error {
	log, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	process := newProcess(name, args, commandOptions{stdin: input, stdout: log, stderr: log, env: environ})
	runErr := process.Run()
	closeErr := log.Close()
	if runErr != nil {
		data, _ := os.ReadFile(path)
		return fmt.Errorf("%s: %w\n%s", name, runErr, data)
	}
	return closeErr
}

func startDaemon(id, pidName, logName, ready string, attempts int, password string) error {
	state := demoState(id)
	pid, err := startCommand("ze", []string{"start", "ze.conf"}, scenarioEnv(id, password), filepath.Join(state, logName))
	if err != nil {
		return err
	}
	if err := writePIDs(filepath.Join(state, pidName), []int{pid}); err != nil {
		return err
	}
	return waitForFileText(filepath.Join(state, logName), ready, attempts)
}

func startPeersAndDaemon(id string, peers [][]string, logName string, attempts int) error {
	state := demoState(id)
	environ := scenarioEnv(id, demoPassword)
	pids := make([]int, 0, len(peers)+1)
	for index, args := range peers {
		pid, err := startCommand("ze-test", args, environ, filepath.Join(state, fmt.Sprintf("peer-%d.log", index)))
		if err != nil {
			return err
		}
		pids = append(pids, pid)
	}
	pid, err := startCommand("ze", []string{"start", "ze.conf"}, environ, filepath.Join(state, logName))
	if err != nil {
		return err
	}
	pids = append(pids, pid)
	if err := writePIDs(filepath.Join(state, "pids"), pids); err != nil {
		return err
	}
	return waitForFileText(filepath.Join(state, logName), "SSH server listening", attempts)
}

func runCLIDashboard(action string) error {
	const id = "cli-dashboard"
	switch action {
	case "start":
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		input := strings.NewReader(initText("admin", demoPassword))
		if err := runForegroundLog("ze", []string{"init"}, scenarioEnv(id, demoPassword), filepath.Join(demoState(id), "init.log"), input); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		peers := [][]string{
			{"peer", "--mode", "sink", "--bind", "127.0.0.2", "--port", "1179", "--asn", "65001"},
			{"peer", "--mode", "sink", "--bind", "127.0.0.3", "--port", "1179", "--asn", "65002"},
			{"peer", "--mode", "sink", "--bind", "127.0.0.4", "--port", "1179", "--asn", "64496"},
		}
		if err := startPeersAndDaemon(id, peers, "daemon.log", 100); err != nil {
			return err
		}
		fmt.Println("terminal demo ready")
	case "stop":
		stopScenario(id, "pids")
		fmt.Println("terminal demo stopped")
	default:
		return fmt.Errorf("cli-dashboard action must be start or stop")
	}
	return nil
}

func runHealthReports(action string) error {
	const id = "health-reports"
	switch action {
	case "prepare":
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		fmt.Println("Health reports demo prepared")
	case "start":
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		peers := [][]string{{"peer", "--mode", "echo", "--bind", "127.0.0.2", "--port", "1179", "--asn", "65001"}}
		if err := startPeersAndDaemon(id, peers, "daemon.log", 150); err != nil {
			return err
		}
		fmt.Println("Health reports demo ready")
	case "stop":
		stopScenario(id, "pids")
		fmt.Println("Health reports demo stopped")
	default:
		return fmt.Errorf("health-reports action must be prepare, start, or stop")
	}
	return nil
}

func runZeFSConfig(action string) error {
	id := os.Getenv("ZE_DEMO_ID")
	if id == "" {
		id = "zefs-config"
	}
	sources := strings.Fields(os.Getenv("ZE_DEMO_CONFIG_SOURCES"))
	if len(sources) == 0 {
		sources = []string{filepath.Join(demoDir("zefs-config"), "ze.conf")}
	}
	switch action {
	case "prepare":
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		fmt.Println("terminal demo prepared")
	case "start":
		if err := importScenarioConfig(id, sources...); err != nil {
			return err
		}
		peers := [][]string{{"peer", "--mode", "sink", "--bind", "127.0.0.2", "--port", "1179", "--asn", "65001"}}
		if err := startPeersAndDaemon(id, peers, "daemon.log", 100); err != nil {
			return err
		}
		fmt.Println("terminal demo ready")
	case "stop":
		stopScenario(id, "pids")
		fmt.Println("terminal demo stopped")
	default:
		return fmt.Errorf("zefs-config action must be prepare, start, or stop")
	}
	return nil
}

func runRBAC(action string) error {
	const id = "rbac"
	switch action {
	case "start":
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		if err := runForegroundLog("ze", []string{"init"}, scenarioEnv(id, "admin-secret"), filepath.Join(demoState(id), "init.log"), strings.NewReader(initText("admin", "admin-secret"))); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "rbac.conf")); err != nil {
			return err
		}
		if err := startDaemon(id, "pids", "daemon.log", "SSH server listening", 100, "admin-secret"); err != nil {
			return err
		}
		fmt.Println("terminal demo ready")
	case "stop":
		stopScenario(id, "pids")
		fmt.Println("terminal demo stopped")
	default:
		return fmt.Errorf("rbac action must be start or stop")
	}
	return nil
}

func runWebConfig(action string) error {
	const id = "web-config"
	switch action {
	case "start":
		if err := prepareScenario(id, "pids", false); err != nil {
			return err
		}
		if err := runForegroundLog("ze", []string{"init"}, scenarioEnv(id, demoPassword), filepath.Join(demoState(id), "init.log"), strings.NewReader(initText("admin", demoPassword))); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := startDaemon(id, "pids", "daemon.log", "web server listening", 150, demoPassword); err != nil {
			return err
		}
		fmt.Println("web demo ready")
	case "stop":
		stopScenario(id, "pids")
		fmt.Println("web demo stopped")
	default:
		return fmt.Errorf("web-config action must be start or stop")
	}
	return nil
}

func runRIBFIB(action string, args []string, stdout io.Writer) error {
	const id = "rib-fib"
	switch action {
	case "prepare":
		if err := prepareScenario(id, "daemon.pid", false); err != nil {
			return err
		}
		fmt.Println("RIB/FIB demo prepared")
	case "start":
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := startDaemon(id, "daemon.pid", "daemon.log", "SSH server listening", 150, demoPassword); err != nil {
			return err
		}
		fmt.Println("RIB/FIB demo ready")
	case "kernel-route":
		if len(args) != 1 {
			return errors.New("kernel-route needs PREFIX")
		}
		output, err := runCommand("ip", []string{"-details", "route", "show", "exact", args[0]}, commandOptions{})
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(output)) == 0 {
			if _, err := fmt.Fprintf(stdout, "kernel FIB: %s absent\n", args[0]); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(stdout, "kernel FIB: %s\n", bytes.TrimSpace(output)); err != nil {
				return err
			}
		}
	case "stop":
		stopScenario(id, "daemon.pid")
		fmt.Println("RIB/FIB demo stopped")
	default:
		return fmt.Errorf("rib-fib action must be prepare, start, kernel-route, or stop")
	}
	return nil
}

func runConfigViews(action string) error {
	if action != "prepare" {
		return errors.New("config-views action must be prepare")
	}
	id := "config-views"
	state := demoState(id)
	if err := os.RemoveAll(state); err != nil {
		return err
	}
	if err := os.MkdirAll(state, 0o750); err != nil {
		return err
	}
	config := filepath.Join(demoDir(id), "router.conf")
	if _, err := runZe([]string{"config", "validate", config}, demoEnvironment(), nil); err != nil {
		return err
	}
	commands := [][]string{
		{"config", "migrate", "--format", "set", "-o", filepath.Join(state, "router.set"), config},
		{"config", "migrate", "--format", "hierarchical", "-o", filepath.Join(state, "roundtrip.conf"), filepath.Join(state, "router.set")},
		{"config", "migrate", "--format", "set", "-o", filepath.Join(state, "roundtrip.set"), filepath.Join(state, "roundtrip.conf")},
	}
	for _, args := range commands {
		if _, err := runZe(args, demoEnvironment(), nil); err != nil {
			return err
		}
	}
	fmt.Println("Configuration views prepared")
	return nil
}

func runTraceroute(action string) error {
	const id = "traceroute"
	switch action {
	case "start":
		stopTraceroute()
		state := demoState(id)
		if err := os.RemoveAll(state); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(state, "config"), 0o750); err != nil {
			return err
		}
		if err := createTracerouteNetwork(); err != nil {
			return err
		}
		if err := runForegroundLog("ze", []string{"init"}, scenarioEnv(id, demoPassword), filepath.Join(state, "init.log"), strings.NewReader(initText("admin", demoPassword))); err != nil {
			return err
		}
		if err := importScenarioConfig(id, filepath.Join(demoDir(id), "ze.conf")); err != nil {
			return err
		}
		if err := startDaemon(id, "pids", "daemon.log", "SSH server listening", 100, demoPassword); err != nil {
			return err
		}
		fmt.Println("terminal demo ready")
	case "stop":
		stopTraceroute()
		fmt.Println("terminal demo stopped")
	default:
		return errors.New("traceroute action must be start or stop")
	}
	return nil
}

func stopTraceroute() {
	stopScenario("traceroute", "pids")
	commands := [][]string{{"route", "del", "192.0.2.53/32", "via", "198.51.100.2"}, {"link", "del", "ze-edge"}, {"netns", "del", "edge"}, {"netns", "del", "core"}}
	for _, args := range commands {
		_, _ = runCommand("ip", args, commandOptions{})
	}
}

func createTracerouteNetwork() error {
	commands := [][]string{
		{"netns", "add", "edge"}, {"netns", "add", "core"},
		{"link", "add", "ze-edge", "type", "veth", "peer", "name", "edge-ze"}, {"link", "set", "edge-ze", "netns", "edge"},
		{"address", "add", "198.51.100.1/30", "dev", "ze-edge"}, {"link", "set", "ze-edge", "up"},
		{"-n", "edge", "address", "add", "198.51.100.2/30", "dev", "edge-ze"}, {"-n", "edge", "link", "set", "edge-ze", "up"}, {"-n", "edge", "link", "set", "lo", "up"},
		{"link", "add", "edge-core", "type", "veth", "peer", "name", "core-edge"}, {"link", "set", "edge-core", "netns", "edge"}, {"link", "set", "core-edge", "netns", "core"},
		{"-n", "edge", "address", "add", "203.0.113.1/30", "dev", "edge-core"}, {"-n", "edge", "link", "set", "edge-core", "up"},
		{"-n", "core", "address", "add", "203.0.113.2/30", "dev", "core-edge"}, {"-n", "core", "link", "set", "core-edge", "up"},
		{"-n", "core", "address", "add", "192.0.2.53/32", "dev", "lo"}, {"-n", "core", "link", "set", "lo", "up"},
		{"netns", "exec", "edge", "sysctl", "-q", "-w", "net.ipv4.ip_forward=1"}, {"netns", "exec", "core", "sysctl", "-q", "-w", "net.ipv4.ip_forward=1"},
		{"route", "add", "192.0.2.53/32", "via", "198.51.100.2"}, {"-n", "edge", "route", "add", "192.0.2.53/32", "via", "203.0.113.2"}, {"-n", "core", "route", "add", "198.51.100.0/30", "via", "203.0.113.1"},
	}
	for _, args := range commands {
		if _, err := runCommand("ip", args, commandOptions{}); err != nil {
			return err
		}
	}
	hosts, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return err
	}
	if !bytes.Contains(hosts, []byte("dns.demo")) {
		file, err := os.OpenFile("/etc/hosts", os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, err = io.WriteString(file, "198.51.100.2 edge-gw.demo\n203.0.113.2 core-gw.demo\n192.0.2.53 dns.demo\n")
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func reportDemoLogs(id string, output io.Writer) {
	matches, _ := filepath.Glob(filepath.Join(demoState(id), "*.log"))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
		_, _ = fmt.Fprintf(output, "--- %s\n%s\n", path, strings.Join(lines, "\n"))
	}
}

func terminatePID(path string, signal syscall.Signal) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var pid int
	if _, err := fmt.Sscan(string(data), &pid); err != nil {
		return err
	}
	if err := syscall.Kill(pid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return os.Remove(path)
}

func runBounded(name string, args []string, environ []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	process := execCommandContext(ctx, name, args, environ)
	output, err := process.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), nil
	}
	return string(output), err
}

var execCommandContext = func(ctx context.Context, name string, args []string, environ []string) *osExecCmd {
	return &osExecCmd{cmd: commandContext(ctx, name, args, environ)}
}

type osExecCmd struct {
	cmd interface{ CombinedOutput() ([]byte, error) }
}

func (c *osExecCmd) CombinedOutput() ([]byte, error) { return c.cmd.CombinedOutput() }

func commandContext(ctx context.Context, name string, args []string, environ []string) *exec.Cmd {
	process := exec.CommandContext(ctx, name, args...) // #nosec G204 -- closed scenario table.
	process.Env = environ
	return process
}
