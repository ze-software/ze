package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
)

const StressAction = "stress"

const (
	stressZeASN          = 65100
	stressZeStartupWait  = 2 * time.Second
	stressFlapPause      = 2 * time.Second
	stressProfileStartup = time.Second
	stressProfileWait    = 120 * time.Second
)

// stressOptions selects one exact scenario. An empty selection runs the complete registry.
type stressOptions struct {
	Scenario string
}

// StressPeerMetrics preserves the injector's message, byte, build, and wire-rate result.
type StressPeerMetrics struct {
	Messages  int     `json:"messages,omitempty"`
	Bytes     int64   `json:"bytes,omitempty"`
	BuildTime string  `json:"build-time,omitempty"`
	SendTime  string  `json:"send-time,omitempty"`
	MBps      float64 `json:"mbps,omitempty"`
}

// StressRoundReport is one complete injector session.
type StressRoundReport struct {
	PrefixBase      string            `json:"prefix-base"`
	Prefixes        int               `json:"prefixes"`
	Dwell           string            `json:"dwell"`
	TimeoutSeconds  int               `json:"timeout-seconds"`
	ElapsedSeconds  float64           `json:"elapsed-seconds"`
	RoutesPerSecond float64           `json:"routes-per-second"`
	Metrics         StressPeerMetrics `json:"metrics"`
}

// StressProfileReport identifies a profile file and its observed result bytes.
type StressProfileReport struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// StressScenarioReport is the verdict for one exact former checker.
type StressScenarioReport struct {
	Name          string                `json:"name"`
	Passed        bool                  `json:"passed"`
	Rounds        []StressRoundReport   `json:"rounds,omitempty"`
	Profiles      []StressProfileReport `json:"profiles,omitempty"`
	Bird          *StressBirdReport     `json:"bird,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	CleanupErrors []string              `json:"cleanup-errors,omitempty"`
	Failure       string                `json:"failure,omitempty"`
	ExitCode      int                   `json:"exit-code"`
}

// stressReport is the result of the native stress runner.
type stressReport struct {
	Action    string                 `json:"action"`
	Root      string                 `json:"root"`
	Selection string                 `json:"selection,omitempty"`
	Scenarios []StressScenarioReport `json:"scenarios"`
	Passed    int                    `json:"passed"`
	Failed    int                    `json:"failed"`
	Code      int                    `json:"code"`
	Failure   string                 `json:"failure,omitempty"`
}

// Text preserves the runner's pass/fail summary while structured output keeps all metrics.
func (r stressReport) Text() string {
	if r.Failure != "" {
		return r.Failure
	}
	if r.Failed == 0 {
		return fmt.Sprintf("PASS  %d scenario(s)", r.Passed)
	}
	failed := make([]string, 0, r.Failed)
	for _, scenario := range r.Scenarios {
		if !scenario.Passed {
			failed = append(failed, scenario.Name)
		}
	}
	return fmt.Sprintf("FAIL  %d passed, %d failed: %s", r.Passed, r.Failed, strings.Join(failed, " "))
}

type stressScenario struct {
	name   string
	config string
	rounds []stressRound
}

type stressRound struct {
	prefixBase string
	nexthop    string
	prefixes   int
	dwell      string
	timeout    time.Duration
	pause      time.Duration
}

var stressScenarioRegistry = [...]stressScenario{
	{
		name: "01-bulk-ipv4", config: "ze.conf",
		rounds: []stressRound{
			{prefixBase: "10.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 100_000, dwell: "15s", timeout: 120 * time.Second},
			{prefixBase: "10.64.0.0/24", nexthop: stressBirdPeerIP, prefixes: 250_000, dwell: "15s", timeout: 180 * time.Second},
			{prefixBase: "10.128.0.0/24", nexthop: stressBirdPeerIP, prefixes: 500_000, dwell: "15s", timeout: 300 * time.Second},
			{prefixBase: "11.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 1_000_000, dwell: "15s", timeout: 600 * time.Second},
		},
	},
	{
		name: "02-multi-peer", config: "ze.conf",
		rounds: []stressRound{
			{prefixBase: "10.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 500_000, dwell: "30s", timeout: 900 * time.Second},
			{prefixBase: "2001:db8::/48", nexthop: "2001:db8::3", prefixes: 250_000, dwell: "30s", timeout: 600 * time.Second},
		},
	},
	{
		name: "03-session-flap", config: "ze.conf",
		rounds: stressFlapRounds(),
	},
	{name: stressBirdScenario, config: "bird.conf"},
	{
		name: "05-profile-1m", config: "ze.conf",
		rounds: []stressRound{
			{prefixBase: "10.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 1_000_000, dwell: "60s", timeout: 600 * time.Second},
		},
	},
}

func stressFlapRounds() []stressRound {
	rounds := make([]stressRound, 0, 11)
	for range 10 {
		rounds = append(rounds, stressRound{
			prefixBase: "10.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 100_000,
			dwell: "2s", timeout: 180 * time.Second, pause: stressFlapPause,
		})
	}
	return append(rounds, stressRound{
		prefixBase: "10.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 100_000,
		dwell: "5s", timeout: 180 * time.Second,
	})
}

// stressScenarios returns the exact, stable scenario registry in runner order.
func stressScenarios() []string {
	names := make([]string, 0, len(stressScenarioRegistry))
	for _, scenario := range stressScenarioRegistry {
		names = append(names, scenario.name)
	}
	return names
}

// runStressAction runs every former stress checker, or one exact selected checker.
func runStressAction(ctx context.Context, root string, options stressOptions) (stressReport, int) {
	return runStressAt(ctx, root, options, realStressSystem{})
}

func runStressAt(
	ctx context.Context,
	root string,
	options stressOptions,
	system stressSystem,
) (stressReport, int) {
	report := stressReport{Action: StressAction, Root: root, Selection: options.Scenario}
	selected := make([]stressScenario, 0, len(stressScenarioRegistry))
	for _, scenario := range stressScenarioRegistry {
		if options.Scenario == "" || options.Scenario == scenario.name {
			selected = append(selected, scenario)
		}
	}
	if len(selected) == 0 {
		report.Failure = fmt.Sprintf("no scenario matching %q found", options.Scenario)
		report.Code = 1
		return report, report.Code
	}

	for _, scenario := range selected {
		var result StressScenarioReport
		if scenario.name == stressBirdScenario {
			bird, code := runStressBird(ctx, root, system)
			result = StressScenarioReport{
				Name: scenario.name, Passed: code == 0, Bird: &bird, ExitCode: code,
			}
			if bird.Failure != nil {
				result.Failure = bird.Failure.Message
			}
		} else {
			result = runZeStressScenario(ctx, root, scenario, system)
		}
		report.Scenarios = append(report.Scenarios, result)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		if ctx.Err() != nil {
			break
		}
	}
	if report.Failed > 0 {
		report.Code = 1
	}
	return report, report.Code
}

type stressSystem interface {
	stressBirdSystem
	ReadFile(path string) ([]byte, error)
	fileSize(path string) (int64, error)
	MkdirAll(path string, mode os.FileMode) error
}

type stressRunner struct {
	base     stressBirdRunner
	system   stressSystem
	scenario stressScenario
	zeBinary string
	cpu      stressBirdProcess
}

func runZeStressScenario(
	ctx context.Context,
	root string,
	scenario stressScenario,
	system stressSystem,
) (report StressScenarioReport) {
	runner := stressRunner{system: system, scenario: scenario}
	runner.base = stressBirdRunner{root: root, system: system, environ: system.Environ()}
	runner.base.initializeNames()
	report = StressScenarioReport{Name: scenario.name}
	defer func() {
		report.CleanupErrors = runner.base.cleanup(ctx, true)
		_ = system.Remove(filepath.Join("/tmp", "ze-stress-profile-"+runner.base.suffix+".log"))
		report.Warnings = append(report.Warnings, runner.base.warnings...)
		if report.Failure == "" && len(report.CleanupErrors) > 0 {
			report.Failure = "cleanup failed: " + strings.Join(report.CleanupErrors, "; ")
			report.ExitCode = 1
		}
		report.Passed = report.Failure == ""
	}()

	if failure := runner.preflight(ctx); failure != nil {
		report.Failure, report.ExitCode = failure.Message, failure.ExitCode
		return report
	}
	runner.base.cleanup(ctx, false)
	if failure := runner.base.createNamespaces(ctx); failure != nil {
		report.Failure, report.ExitCode = failure.Message, failure.ExitCode
		return report
	}
	if failure := runner.startZe(ctx); failure != nil {
		report.Failure, report.ExitCode = failure.Message, failure.ExitCode
		return report
	}
	if scenario.name == "05-profile-1m" {
		if failure := runner.startCPUProfile(ctx); failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
			return report
		}
	}
	for _, round := range scenario.rounds {
		roundReport, failure := runner.runRound(ctx, round)
		report.Rounds = append(report.Rounds, roundReport)
		if failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
			return report
		}
	}
	if scenario.name == "05-profile-1m" {
		var failure *StressBirdFailure
		report.Profiles, failure = runner.finishProfiles(ctx)
		if failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
		}
	}
	return report
}

func (r *stressRunner) preflight(ctx context.Context) *StressBirdFailure {
	if r.system.effectiveUID() != 0 {
		return stressBirdFailure("preflight", 1, "must run as root for network namespaces")
	}
	missingRuntime := false
	for _, tool := range [...]string{"ip", "ethtool"} {
		if _, err := r.system.LookPath(tool); err != nil {
			missingRuntime = true
		}
	}
	if missingRuntime {
		if failure := r.base.installRuntimeTools(ctx); failure != nil {
			return failure
		}
	}
	for _, tool := range [...]string{"ip", "ethtool"} {
		if _, err := r.system.LookPath(tool); err != nil {
			return stressBirdFailure("preflight", gaterun.CannotStart, "setup completed but required command is still missing: "+tool)
		}
	}

	peer := filepath.Join(r.base.root, "bin", "ze-test")
	if !r.system.FileExists(peer) {
		return stressBirdFailure("preflight", gaterun.CannotStart, "bin/ze-test not found at "+peer+"; build ze-test first")
	}
	r.zeBinary = r.system.Getenv("ZE_BINARY")
	if r.zeBinary == "" || !r.system.FileExists(r.zeBinary) {
		r.zeBinary = filepath.Join(r.base.root, "bin", "ze")
	}
	if !r.system.FileExists(r.zeBinary) {
		if _, err := r.system.LookPath("go"); err != nil {
			return stressBirdFailure("preflight", gaterun.CannotStart, "bin/ze not found and go is not in PATH")
		}
		environ := slices.Clone(r.base.environ)
		environ = append(environ, "CGO_ENABLED=0")
		result, err := r.system.Run(ctx, stressBirdCommand{
			argv: []string{"go", "build", "-tags", "ze_core,ze_distro", "-o", r.zeBinary, "./cmd/ze"},
			dir:  r.base.root, environ: environ, timeout: 120 * time.Second,
		})
		if err != nil {
			return stressBirdFailure("preflight", commandErrorCode(err), "build Ze: "+err.Error())
		}
		if result.code != 0 {
			return stressBirdFailure("preflight", result.code, "build Ze: "+strings.TrimSpace(result.stderr))
		}
	}
	config := filepath.Join(r.base.root, "test", "stress", "scenarios", r.scenario.name, r.scenario.config)
	if !r.system.FileExists(config) {
		return stressBirdFailure("preflight", gaterun.CannotStart, "DUT configuration not found at "+config)
	}
	return nil
}

func (r *stressRunner) startZe(ctx context.Context) *StressBirdFailure {
	config := filepath.Join(r.base.root, "test", "stress", "scenarios", r.scenario.name, r.scenario.config)
	argv := r.base.namespaceArgv(r.base.zeNS, r.zeBinary)
	if r.system.Getenv("ZE_PPROF") != "" {
		argv = append(argv, "--pprof", "127.0.0.1:6060")
	}
	argv = append(argv, "start", config)
	environ := slices.Clone(r.base.environ)
	environ = append(environ, "ze.log.bgp.reactor=info", "ze.log.plugin=info")
	process, err := r.system.Start(ctx, stressBirdCommand{
		argv: argv, environ: environ, outputPath: r.base.paths.zeLog,
	})
	if err != nil {
		return stressBirdFailure("ze-start", gaterun.CannotStart, "start Ze: "+err.Error())
	}
	r.base.processes = append(r.base.processes, process)
	if err := r.system.Sleep(ctx, stressZeStartupWait); err != nil {
		return stressBirdFailure("ze-start", stressBirdTimeoutCode, "Ze startup wait canceled")
	}
	exited, code, err := process.Exited()
	if err != nil {
		return stressBirdFailure("ze-start", 1, "inspect Ze: "+err.Error())
	}
	if exited {
		if code == 0 {
			code = 1
		}
		message := fmt.Sprintf("Ze exited immediately with code %d", code)
		if content, readErr := r.system.ReadFile(r.base.paths.zeLog); readErr == nil && len(content) > 0 {
			if len(content) > 1_000 {
				content = content[:1_000]
			}
			message += ": " + string(content)
		}
		return stressBirdFailure("ze-start", code, message)
	}
	r.base.runOptional(ctx, r.base.namespaceArgv(r.base.zeNS, "ss", "-tlnp", "sport", "=", "179"))
	capture, err := r.system.Start(ctx, stressBirdCommand{
		argv: r.base.namespaceArgv(
			r.base.zeNS, "tcpdump", "-i", r.base.zeVeth, "-nn", "-l", "-c", "100", "tcp", "port", "179",
		),
		environ: r.base.environ, outputPath: r.base.paths.pcapLog,
	})
	if err != nil {
		return stressBirdFailure("capture", gaterun.CannotStart, "start tcpdump: "+err.Error())
	}
	r.base.processes = append(r.base.processes, capture)
	return nil
}

func (r *stressRunner) runRound(
	ctx context.Context,
	round stressRound,
) (StressRoundReport, *StressBirdFailure) {
	report := StressRoundReport{
		PrefixBase: round.prefixBase, Prefixes: round.prefixes, Dwell: round.dwell,
		TimeoutSeconds: int(round.timeout / time.Second),
	}
	started := r.system.Now()
	peer, failure := r.startPeer(ctx, round)
	if failure != nil {
		return report, failure
	}
	code, err := peer.Wait(round.timeout)
	report.ElapsedSeconds = r.system.Now().Sub(started).Seconds()
	if report.ElapsedSeconds > 0 {
		report.RoutesPerSecond = float64(round.prefixes) / report.ElapsedSeconds
	}
	if errors.Is(err, errStressBirdWaitTimeout) {
		_ = peer.Kill()
		_, _ = peer.Wait(stressBirdStopTimeout)
		message := "peer inject timeout"
		if content, readErr := r.system.ReadFile(r.base.paths.peerLog); readErr == nil {
			message += stressPeerLogTail(content)
		}
		return report, stressBirdFailure("peer", stressBirdTimeoutCode, message)
	}
	if err != nil {
		return report, stressBirdFailure("peer", 1, "wait for peer inject: "+err.Error())
	}
	if code != 0 {
		message := fmt.Sprintf("peer inject failed with code %d", code)
		if content, readErr := r.system.ReadFile(r.base.paths.peerLog); readErr == nil {
			message += stressPeerLogTail(content)
		}
		return report, stressBirdFailure("peer", code, message)
	}
	content, readErr := r.system.ReadFile(r.base.paths.peerLog)
	if readErr != nil {
		r.base.warnings = append(r.base.warnings, "read peer metrics: "+readErr.Error())
	} else {
		report.Metrics = parseStressPeerMetrics(content)
	}
	if round.pause > 0 {
		if err := r.system.Sleep(ctx, round.pause); err != nil {
			return report, stressBirdFailure("flap-pause", stressBirdTimeoutCode, "flap pause canceled")
		}
	}
	return report, nil
}

func (r *stressRunner) startPeer(
	ctx context.Context,
	round stressRound,
) (stressBirdProcess, *StressBirdFailure) {
	peerBinary := filepath.Join(r.base.root, "bin", "ze-test")
	argv := r.base.namespaceArgv(
		r.base.peerNS,
		peerBinary, "peer", "--mode", "inject", "--dial", stressBirdZeDial,
		"--inject-prefix", round.prefixBase,
		"--inject-count", strconv.Itoa(round.prefixes),
		"--inject-nexthop", round.nexthop,
		"--inject-asn", strconv.Itoa(stressZeASN),
		"--inject-dwell", round.dwell,
	)
	process, err := r.system.Start(ctx, stressBirdCommand{
		argv: argv, environ: r.base.environ, outputPath: r.base.paths.peerLog,
	})
	if err != nil {
		return nil, stressBirdFailure("peer", gaterun.CannotStart, "start peer inject: "+err.Error())
	}
	r.base.processes = append(r.base.processes, process)
	return process, nil
}

var (
	stressBuiltMetrics = regexp.MustCompile(`(?m)^\s*inject built: (\d+) messages, (\d+) bytes in (\S+)`)
	stressSentMetrics  = regexp.MustCompile(`(?m)^\s*inject sent: \d+ bytes in (\S+) \(([\d.]+) MB/s\)`)
)

func parseStressPeerMetrics(content []byte) StressPeerMetrics {
	var metrics StressPeerMetrics
	if match := stressBuiltMetrics.FindSubmatch(content); len(match) != 0 {
		metrics.Messages, _ = strconv.Atoi(string(match[1]))
		metrics.Bytes, _ = strconv.ParseInt(string(match[2]), 10, 64)
		metrics.BuildTime = string(match[3])
	}
	if match := stressSentMetrics.FindSubmatch(content); len(match) != 0 {
		metrics.SendTime = string(match[1])
		metrics.MBps, _ = strconv.ParseFloat(string(match[2]), 64)
	}
	return metrics
}

func stressPeerLogTail(content []byte) string {
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}
	return "\npeer log tail:\n" + strings.Join(lines, "\n")
}

func (r *stressRunner) startCPUProfile(ctx context.Context) *StressBirdFailure {
	profileDir := filepath.Join(r.base.root, "tmp")
	if err := r.system.MkdirAll(profileDir, 0o755); err != nil {
		return stressBirdFailure("profile", 1, "create profile directory: "+err.Error())
	}
	if r.system.Getenv("ZE_PPROF") == "" {
		return nil
	}
	path := filepath.Join(profileDir, "stress-profile-cpu.pb.gz")
	process, err := r.system.Start(ctx, stressBirdCommand{
		argv: r.base.namespaceArgv(
			r.base.zeNS, "curl", "-sS", "-o", path,
			"http://127.0.0.1:6060/debug/pprof/profile?seconds=90",
		),
		environ:    r.base.environ,
		outputPath: filepath.Join("/tmp", "ze-stress-profile-"+r.base.suffix+".log"),
	})
	if err != nil {
		return stressBirdFailure("profile", gaterun.CannotStart, "start CPU profile: "+err.Error())
	}
	r.cpu = process
	r.base.processes = append(r.base.processes, process)
	if err := r.system.Sleep(ctx, stressProfileStartup); err != nil {
		return stressBirdFailure("profile", stressBirdTimeoutCode, "CPU profile startup wait canceled")
	}
	return nil
}

func (r *stressRunner) finishProfiles(
	ctx context.Context,
) ([]StressProfileReport, *StressBirdFailure) {
	if r.system.Getenv("ZE_PPROF") == "" {
		return nil, nil
	}
	profileDir := filepath.Join(r.base.root, "tmp")
	profiles := make([]StressProfileReport, 0, 3)
	for _, profile := range []struct {
		name string
		url  string
	}{
		{name: "heap", url: "http://127.0.0.1:6060/debug/pprof/heap"},
		{name: "goroutine", url: "http://127.0.0.1:6060/debug/pprof/goroutine?debug=0"},
	} {
		path := filepath.Join(profileDir, "stress-profile-"+profile.name+".pb.gz")
		result, err := r.system.Run(ctx, stressBirdCommand{
			argv:    r.base.namespaceArgv(r.base.zeNS, "curl", "-sS", "-o", path, profile.url),
			environ: r.base.environ, timeout: stressProfileWait,
		})
		if err != nil || result.code != 0 {
			r.base.warnings = append(r.base.warnings, "failed to save "+profile.name+" profile")
			continue
		}
		if size, sizeErr := r.system.fileSize(path); sizeErr == nil && size > 0 {
			profiles = append(profiles, StressProfileReport{Name: profile.name, Path: path, Bytes: size})
		} else {
			r.base.warnings = append(r.base.warnings, "failed to save "+profile.name+" profile")
		}
	}
	if r.cpu != nil {
		code, err := r.cpu.Wait(stressProfileWait)
		if errors.Is(err, errStressBirdWaitTimeout) {
			return profiles, stressBirdFailure("profile", stressBirdTimeoutCode, "CPU profile capture timed out")
		}
		path := filepath.Join(profileDir, "stress-profile-cpu.pb.gz")
		size, sizeErr := r.system.fileSize(path)
		if err == nil && code == 0 && sizeErr == nil && size > 0 {
			profiles = append(profiles, StressProfileReport{Name: "cpu", Path: path, Bytes: size})
		} else {
			r.base.warnings = append(r.base.warnings, "CPU profile capture failed")
		}
	}
	return profiles, nil
}

type realStressSystem struct {
	realStressBirdSystem
}

func (realStressSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (realStressSystem) fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (realStressSystem) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

var _ stressSystem = realStressSystem{}

func stressScenarioNamed(name string) (stressScenario, bool) {
	for _, scenario := range stressScenarioRegistry {
		if scenario.name == name {
			return scenario, true
		}
	}
	return stressScenario{}, false
}

func stressRoundIdentity(round stressRound) string {
	var text textbuf.Buffer
	return text.Str(round.prefixBase).Str("/").Int(int64(round.prefixes)).Str("/").Str(round.dwell).String()
}
