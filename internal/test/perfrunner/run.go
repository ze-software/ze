package perfrunner

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	subnet     = "172.31.0.0/24"
	senderIP   = "172.31.0.10"
	receiverIP = "172.31.0.11"
)

type DUT struct {
	Name, Image, IP                string
	Port, SenderPort, ReceiverPort int
	Passive                        bool
}

func DUTs() []DUT {
	frr := envOr("FRR_IMAGE", "quay.io/frrouting/frr:10.3.1")
	rust := envOr("RUSTBGPD_IMAGE", "rustbgpd-interop")
	return []DUT{
		{Name: "ze", Image: "ze-interop", IP: "172.31.0.2", Port: 179, SenderPort: 1790, ReceiverPort: 1791},
		{Name: "frr", Image: frr, IP: "172.31.0.3", Port: 179},
		{Name: "bird", Image: "bird-interop", IP: "172.31.0.4", Port: 179},
		{Name: "gobgp", Image: "gobgp-interop", IP: "172.31.0.5", Port: 179},
		{Name: "rustbgpd", Image: rust, IP: "172.31.0.6", Port: 179, Passive: true},
		{Name: "rustybgp", Image: "rustybgp-interop", IP: "172.31.0.7", Port: 179},
		{Name: "freertr", Image: "freertr-interop", IP: "172.31.0.8", Port: 179},
		{Name: "openbgpd", Image: "openbgpd-interop", IP: "172.31.0.9", Port: 179},
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func unmeasuredDUTs(resultFiles []string) []string {
	measured := make(map[string]bool, len(resultFiles))
	for _, path := range resultFiles {
		measured[strings.TrimSuffix(filepath.Base(path), ".json")] = true
	}
	missing := make([]string, 0)
	for _, dut := range DUTs() {
		if !measured[dut.Name] {
			missing = append(missing, dut.Name)
		}
	}
	return missing
}

type CommandRunner func(context.Context, io.Writer, io.Writer, string, []string, []string) error

func systemCommand(ctx context.Context, stdout, stderr io.Writer, dir string, env, args []string) error {
	command := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- benchmark configuration deliberately selects Docker and Go arguments.
	command.Dir, command.Stdout, command.Stderr = dir, stdout, stderr
	if env != nil {
		command.Env = env
	}
	return command.Run()
}

// generateToFile atomically replaces dest only after generator success.
func generateToFile(ctx context.Context, run CommandRunner, command []string, dest string, stderr io.Writer) bool {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: %v\n", err)
		return false
	}
	temporary := dest + ".new"
	file, err := os.Create(temporary) // #nosec G304 -- destination is runner-owned scratch output.
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: %v\n", err)
		return false
	}
	commandCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = run(commandCtx, file, stderr, "", nil, command)
	cancel()
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(temporary)
		_, _ = fmt.Fprintf(stderr, "  warning: %s NOT updated: generator failed; existing file left untouched\n", filepath.Base(dest))
		return false
	}
	if err := os.Rename(temporary, dest); err != nil {
		_ = os.Remove(temporary)
		_, _ = fmt.Fprintf(stderr, "warning: publish %s: %v\n", dest, err)
		return false
	}
	return true
}

type Runner struct {
	Root, PerfBinary, LinuxBinary, ConfigOverlay               string
	Routes, Seed, Repeat                                       int
	NoBuild, PProf, GCTrace                                    bool
	PProfPort, PProfCPUSeconds                                 int
	PProfDir                                                   string
	Stdout, Stderr                                             io.Writer
	Run                                                        CommandRunner
	suffix, network, resultsDir, runDir, interopDir, configDir string
	buildxOnce                                                 sync.Once
	buildx                                                     bool
}

func New(root string, stdout, stderr io.Writer) *Runner {
	perf := envOr("ZE_PERF_BIN", filepath.Join(root, "bin", "ze-perf"))
	routes, _ := strconv.Atoi(envOr("DUT_ROUTES", "100000"))
	seed, _ := strconv.Atoi(envOr("DUT_SEED", "42"))
	repeat, _ := strconv.Atoi(envOr("DUT_REPEAT", "3"))
	pprofPort, _ := strconv.Atoi(envOr("PPROF_PORT", "6060"))
	cpu, _ := strconv.Atoi(envOr("PPROF_CPU_SECONDS", "30"))
	suffix := strconv.Itoa(os.Getpid())
	return &Runner{Root: root, PerfBinary: perf, LinuxBinary: filepath.Join(root, "bin", "ze-perf-linux"), ConfigOverlay: os.Getenv("PERF_CONFIGS_DIR"), Routes: routes, Seed: seed, Repeat: repeat, NoBuild: os.Getenv("NO_BUILD") != "", PProf: os.Getenv("PPROF") != "", GCTrace: os.Getenv("GCTRACE") != "", PProfPort: pprofPort, PProfCPUSeconds: cpu, PProfDir: envOr("PPROF_DIR", filepath.Join(root, "tmp", "perf-run", "pprof")), Stdout: stdout, Stderr: stderr, Run: systemCommand, suffix: suffix, network: "ze-perf-" + suffix, resultsDir: filepath.Join(root, "test", "perf", "results"), runDir: filepath.Join(root, "tmp", "perf-run"), interopDir: filepath.Join(root, "test", "interop"), configDir: filepath.Join(root, "test", "perf", "configs")}
}

func (runner *Runner) command(timeout time.Duration, capture bool, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var output strings.Builder
	stdout := runner.Stdout
	if capture {
		stdout = &output
	}
	err := runner.Run(ctx, stdout, runner.Stderr, runner.Root, nil, args)
	return output.String(), err
}
func (runner *Runner) docker(timeout time.Duration, capture bool, args ...string) (string, error) {
	if len(args) >= 2 && args[0] == "buildx" && args[1] == "build" && !runner.useBuildx() {
		converted := []string{"build"}
		for _, arg := range args[2:] {
			if arg != "--load" {
				converted = append(converted, arg)
			}
		}
		args = converted
	}
	return runner.command(timeout, capture, append([]string{"docker"}, args...)...)
}
func (runner *Runner) useBuildx() bool {
	runner.buildxOnce.Do(func() {
		_, err := runner.command(5*time.Second, true, "docker", "buildx", "version")
		runner.buildx = err == nil
	})
	return runner.buildx
}
func (runner *Runner) config(name string) string {
	if runner.ConfigOverlay != "" {
		candidate := filepath.Join(runner.ConfigOverlay, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return filepath.Join(runner.configDir, name)
}
func (runner *Runner) containerName(name string) string {
	return "ze-perf-" + name + "-" + runner.suffix
}

func (runner *Runner) buildLinuxBinary() error {
	arch := runtime.GOARCH
	env := append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return runner.Run(ctx, runner.Stdout, runner.Stderr, runner.Root, env, []string{"go", "build", "-tags", "ze_perf ze_bgp", "-o", runner.LinuxBinary, "./cmd/ze"})
}

func (runner *Runner) buildImages(duts []DUT) {
	if runner.NoBuild {
		_, _ = fmt.Fprintln(runner.Stdout, "  skipping image builds (NO_BUILD=1)")
		return
	}
	for _, dut := range duts {
		var args []string
		timeout := 10 * time.Minute
		switch dut.Name {
		case "ze":
			args = []string{"buildx", "build", "--load", "-t", "ze-interop", "-f", filepath.Join(runner.interopDir, "Dockerfile.ze"), runner.Root, "--quiet"}
		case "frr":
			args = []string{"pull", dut.Image, "--quiet"}
			timeout = 2 * time.Minute
		case "bird", "gobgp", "rustbgpd", "rustybgp":
			args = []string{"buildx", "build", "--load", "-t", dut.Image, "-f", filepath.Join(runner.interopDir, "Dockerfile."+dut.Name), runner.interopDir, "--quiet"}
		case "freertr":
			source := envOr("FREERTR_DIR", filepath.Join(filepath.Dir(runner.Root), "rtr"))
			args = []string{"buildx", "build", "--load", "-t", dut.Image, "-f", filepath.Join(runner.interopDir, "Dockerfile.freertr"), "--build-context", "freertr=" + source, "--quiet", runner.Root}
		case "openbgpd":
			source := envOr("OPENBGPD_DIR", filepath.Join(filepath.Dir(runner.Root), "openbgpd-portable"))
			args = []string{"buildx", "build", "--load", "-t", dut.Image, "-f", filepath.Join(runner.interopDir, "Dockerfile.openbgpd"), source, "--quiet"}
		}
		_, _ = fmt.Fprintf(runner.Stdout, "Building %s image...\n", dut.Name)
		if _, err := runner.docker(timeout, false, args...); err != nil {
			_, _ = fmt.Fprintf(runner.Stderr, "  warning: %s image build failed: %v\n", dut.Name, err)
		}
	}
}

func (runner *Runner) volumeArgs(name string) []string {
	switch name {
	case "ze":
		return []string{"-v", runner.config("ze.conf") + ":/etc/ze/bgp.conf:ro"}
	case "frr":
		return []string{"-v", runner.config("frr.conf") + ":/etc/frr/frr.conf:ro", "-v", filepath.Join(runner.interopDir, "daemons") + ":/etc/frr/daemons:ro", "-v", filepath.Join(runner.interopDir, "vtysh.conf") + ":/etc/frr/vtysh.conf:ro"}
	case "bird":
		return []string{"-v", runner.config("bird.conf") + ":/etc/bird/bird.conf:ro"}
	case "gobgp":
		return []string{"-v", runner.config("gobgp.toml") + ":/etc/gobgp/gobgp.toml:ro"}
	case "rustbgpd":
		return []string{"-v", runner.config("rustbgpd.toml") + ":/etc/rustbgpd/config.toml:ro"}
	case "rustybgp":
		return []string{"-v", runner.config("rustybgp.toml") + ":/etc/rustybgp/config.toml:ro"}
	case "freertr":
		return []string{"-v", runner.config("freertr-hw.txt") + ":/etc/freertr/freertr-hw.txt:ro", "-v", runner.config("freertr-sw.txt") + ":/etc/freertr/freertr-sw.txt:ro"}
	case "openbgpd":
		return []string{"-v", runner.config("openbgpd.conf") + ":/etc/bgpd.conf:ro"}
	}
	return nil
}
func (runner *Runner) startDUT(dut DUT) error {
	args := []string{"run", "-d", "--name", runner.containerName(dut.Name), "--network", runner.network, "--ip", dut.IP, "--cap-add", "NET_ADMIN"}
	if dut.Name == "frr" {
		args = append(args, "--cap-add", "SYS_ADMIN")
	}
	if dut.Name == "freertr" {
		args = append(args, "--cap-add", "NET_RAW")
	}
	if dut.Name == "openbgpd" {
		args = append(args, "--cap-add", "SYS_CHROOT")
	}
	if dut.Name == "ze" && runner.GCTrace {
		args = append(args, "-e", "GODEBUG=gctrace=1")
	}
	args = append(args, runner.volumeArgs(dut.Name)...)
	args = append(args, dut.Image)
	if dut.Name == "ze" {
		if runner.PProf {
			args = append(args, "--pprof", "127.0.0.1:"+strconv.Itoa(runner.PProfPort))
		}
		args = append(args, "start", "/etc/ze/bgp.conf")
	}
	_, err := runner.docker(30*time.Second, false, args...)
	return err
}
func (runner *Runner) waitDUT(name string) error {
	limit := 30 * time.Second
	if name == "freertr" {
		limit = 45 * time.Second
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		out, err := runner.docker(10*time.Second, true, "inspect", runner.containerName(name), "--format", "{{.State.Running}}")
		if err == nil && strings.Contains(out, "true") {
			time.Sleep(5 * time.Second)
			return nil
		}
		time.Sleep(time.Second)
	}
	_, _ = runner.docker(10*time.Second, false, "logs", runner.containerName(name), "--tail", "20")
	return fmt.Errorf("%s did not start within %s", name, limit)
}
func (runner *Runner) imageExists(image string) bool {
	_, err := runner.docker(10*time.Second, true, "image", "inspect", image)
	return err == nil
}
func (runner *Runner) stop(name string) {
	_, _ = runner.docker(30*time.Second, true, "rm", "-f", runner.containerName(name))
}

func (runner *Runner) runPerf(dut DUT, senderFirst bool) bool {
	container := "ze-perf-runner-" + runner.suffix
	_, err := runner.docker(30*time.Second, false, "run", "-d", "--name", container, "--network", runner.network, "--ip", senderIP, "--cap-add", "NET_ADMIN", "-v", runner.LinuxBinary+":/usr/local/bin/ze-perf:ro", "-v", runner.resultsDir+":/results", "alpine:3.21", "sleep", "3600")
	if err != nil {
		return false
	}
	defer func() { _, _ = runner.docker(30*time.Second, true, "rm", "-f", container) }()
	_, _ = runner.docker(10*time.Second, true, "exec", container, "ip", "addr", "add", receiverIP+"/24", "dev", "eth0")
	_, _ = runner.docker(5*time.Second, true, "exec", container, "arping", "-U", "-c", "1", "-I", "eth0", receiverIP)
	if dut.Name == "freertr" {
		_, _ = runner.docker(30*time.Second, true, "exec", container, "apk", "add", "--no-cache", "ethtool")
		_, _ = runner.docker(10*time.Second, true, "exec", container, "ethtool", "-K", "eth0", "tx", "off")
	}
	convergence := max(30, (runner.Routes/1000)*4+30)
	result := dut.Name + ".json"
	if senderFirst {
		result = dut.Name + "-propagation.json"
	}
	args := []string{"exec", container, "/usr/local/bin/ze-perf", "run", "--dut-addr", dut.IP, "--dut-port", strconv.Itoa(dut.Port), "--dut-asn", "65000", "--dut-name", dut.Name, "--sender-addr", senderIP, "--sender-asn", "65001", "--receiver-addr", receiverIP, "--receiver-asn", "65002", "--routes", strconv.Itoa(runner.Routes), "--seed", strconv.Itoa(runner.Seed), "--repeat", strconv.Itoa(runner.Repeat), "--warmup-runs", "1", "--iter-delay", "8s", "--warmup", "2s", "--connect-timeout", "20s", "--duration", strconv.Itoa(convergence) + "s", "--output", "/results/" + result}
	if dut.SenderPort != 0 {
		args = append(args, "--sender-port", strconv.Itoa(dut.SenderPort))
	}
	if dut.ReceiverPort != 0 {
		args = append(args, "--receiver-port", strconv.Itoa(dut.ReceiverPort))
	}
	if dut.Passive {
		args = append(args, "--passive-listen")
	}
	if senderFirst {
		args = append(args, "-sender-first")
	}
	timeout := time.Duration((1+runner.Repeat)*(convergence+40)+60) * time.Second
	_, err = runner.docker(timeout, false, args...)
	return err == nil
}

func (runner *Runner) captureProfile(container, endpoint, path string, timeout time.Duration) bool {
	file, err := os.Create(path)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	url := "http://127.0.0.1:" + strconv.Itoa(runner.PProfPort) + "/debug/pprof/" + endpoint
	err = runner.Run(ctx, file, runner.Stderr, runner.Root, nil, []string{"docker", "exec", container, "ze-test", "http-get", url})
	closeErr := file.Close()
	return err == nil && closeErr == nil
}
func (runner *Runner) captureProfiles(container, directory string) {
	_ = os.MkdirAll(directory, 0o750)
	time.Sleep(5 * time.Second)
	runner.captureProfile(container, "profile?seconds="+strconv.Itoa(runner.PProfCPUSeconds), filepath.Join(directory, "cpu.pprof"), time.Duration(runner.PProfCPUSeconds+30)*time.Second)
	for _, name := range []string{"heap", "allocs", "goroutine"} {
		runner.captureProfile(container, name, filepath.Join(directory, name+".pprof"), 15*time.Second)
	}
}
func (runner *Runner) cleanup() {
	for _, dut := range DUTs() {
		runner.stop(dut.Name)
	}
	_, _ = runner.docker(30*time.Second, true, "rm", "-f", "ze-perf-runner-"+runner.suffix)
	_, _ = runner.docker(30*time.Second, true, "network", "rm", runner.network)
}

func (runner *Runner) RunCLI(args []string) int {
	flags := flag.NewFlagSet("perf-run", flag.ContinueOnError)
	flags.SetOutput(runner.Stderr)
	build := flags.Bool("build", false, "build Docker images")
	test := flags.Bool("test", false, "run benchmarks")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*build && !*test {
		_, _ = fmt.Fprintln(runner.Stderr, "at least one of --build or --test is required")
		return 2
	}
	requested := make(map[string]bool)
	for _, name := range flags.Args() {
		requested[name] = true
	}
	duts := make([]DUT, 0)
	for _, dut := range DUTs() {
		if len(requested) == 0 || requested[dut.Name] {
			duts = append(duts, dut)
		}
	}
	if len(duts) == 0 {
		_, _ = fmt.Fprintln(runner.Stderr, "error: no matching DUTs")
		return 1
	}
	if *build {
		runner.buildImages(duts)
	}
	if !*test {
		return 0
	}
	if info, err := os.Stat(runner.PerfBinary); err != nil || !info.Mode().IsRegular() {
		_, _ = fmt.Fprintf(runner.Stderr, "error: ze-perf not found at %s. Build it with: ./le perf-bench run, or go build -tags 'ze_perf ze_bgp' -o bin/ze-perf ./cmd/ze\n", runner.PerfBinary)
		return 1
	}
	if err := runner.buildLinuxBinary(); err != nil {
		_, _ = fmt.Fprintf(runner.Stderr, "build Linux binary: %v\n", err)
		return 1
	}
	defer runner.cleanup()
	_ = os.MkdirAll(runner.resultsDir, 0o750)
	if _, err := runner.docker(10*time.Second, true, "network", "create", "--subnet", subnet, runner.network); err != nil {
		return 1
	}
	passed, failed, skipped := 0, 0, 0
	failedNames := []string{}
	skippedNames := []string{}
	results := []string{}
	for _, dut := range duts {
		_, _ = fmt.Fprintf(runner.Stdout, "-- %s --\n", dut.Name)
		if !runner.imageExists(dut.Image) {
			skipped++
			skippedNames = append(skippedNames, dut.Name)
			continue
		}
		if err := runner.startDUT(dut); err != nil {
			failed++
			failedNames = append(failedNames, dut.Name)
			continue
		}
		if err := runner.waitDUT(dut.Name); err != nil {
			runner.stop(dut.Name)
			failed++
			failedNames = append(failedNames, dut.Name)
			continue
		}
		var profile sync.WaitGroup
		if dut.Name == "ze" && runner.PProf {
			profile.Add(1)
			go func() {
				defer profile.Done()
				runner.captureProfiles(runner.containerName(dut.Name), filepath.Join(runner.PProfDir, strconv.Itoa(runner.Routes)))
			}()
		}
		ok := runner.runPerf(dut, false)
		profile.Wait()
		if ok {
			passed++
			results = append(results, filepath.Join(runner.resultsDir, dut.Name+".json"))
			if dut.Name == "ze" && runner.runPerf(dut, true) {
				results = append(results, filepath.Join(runner.resultsDir, "ze-propagation.json"))
			}
		} else {
			failed++
			failedNames = append(failedNames, dut.Name)
		}
		runner.stop(dut.Name)
	}
	if len(results) > 0 {
		ctx := context.Background()
		report := append([]string{runner.PerfBinary, "report", "--md"}, results...)
		_ = runner.Run(ctx, runner.Stdout, runner.Stderr, runner.Root, nil, report)
		generateToFile(ctx, runner.Run, append([]string{runner.PerfBinary, "report", "--html"}, results...), filepath.Join(runner.resultsDir, "report.html"), runner.Stderr)
		snapshot := filepath.Join(runner.runDir, "performance.md")
		if generateToFile(ctx, runner.Run, append([]string{runner.PerfBinary, "report", "--doc"}, results...), snapshot, runner.Stderr) {
			if missing := unmeasuredDUTs(results); len(missing) > 0 {
				_, _ = fmt.Fprintf(runner.Stdout, "  covers this run only; no result for: %s\n", strings.Join(missing, ", "))
			}
			_, _ = fmt.Fprintf(runner.Stdout, "Performance doc snapshot: %s\n%s is NOT written by a benchmark run\n", snapshot, filepath.Join(runner.Root, "docs", "performance.md"))
		}
	}
	parts := []string{}
	if passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", passed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed: %s", failed, strings.Join(failedNames, " ")))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped: %s", skipped, strings.Join(skippedNames, " ")))
	}
	if len(parts) == 0 {
		parts = append(parts, "no DUTs ran")
	}
	verdict := "PASS"
	if failed > 0 {
		verdict = "FAIL"
	}
	_, _ = fmt.Fprintf(runner.Stdout, "%s  %s\n", verdict, strings.Join(parts, ", "))
	if failed > 0 {
		return 1
	}
	return 0
}

func FindRoot(start string) (string, error) {
	path, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if info, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil && info.Mode().IsRegular() {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", errors.New("repository root not found")
		}
		path = parent
	}
}
