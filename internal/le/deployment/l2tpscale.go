package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/lepath"
)

const L2TPScaleAction = "l2tp-scale-test"

const (
	l2tpScaleStartupWait = 500 * time.Millisecond
	l2tpScaleStopWait    = 10 * time.Second
	l2tpScaleKillWait    = 5 * time.Second
	l2tpScaleTimeout     = 120 * time.Second
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "l2tp.scale.scenario",
	Type:        "string",
	Default:     "",
	Description: "one scenario in the native L2TP scale registry; empty runs all four",
	Private:     true,
})

// l2tpScaleRunOptions selects one exact scenario. An empty selection runs the registry.
type l2tpScaleRunOptions struct {
	Scenario string
}

// L2TPScaleResult is the exact JSON contract emitted by ze-test l2tp-scale.
type L2TPScaleResult struct {
	TunnelsRequested  int           `json:"tunnels-requested"`
	TunnelsUp         int           `json:"tunnels-up"`
	SessionsRequested int           `json:"sessions-requested"`
	SessionsUp        int           `json:"sessions-up"`
	SetupTime         time.Duration `json:"setup-time-ns"`
	TeardownTime      time.Duration `json:"teardown-time-ns"`
	SessionsPerSec    float64       `json:"sessions-per-sec"`
	RADIUSAuth        int64         `json:"radius-auth"`
	RADIUSAcctStart   int64         `json:"radius-acct-start"`
	RADIUSAcctStop    int64         `json:"radius-acct-stop"`
	RADIUSAcctInterim int64         `json:"radius-acct-interim"`
	Errors            []string      `json:"errors,omitempty"`
}

// L2TPScaleScenarioReport preserves both the decoded metrics and exact result bytes.
type L2TPScaleScenarioReport struct {
	Name        string           `json:"name"`
	Passed      bool             `json:"passed"`
	Result      *L2TPScaleResult `json:"result,omitempty"`
	ResultBytes []byte           `json:"result-bytes,omitempty"`
	ErrorBytes  []byte           `json:"error-bytes,omitempty"`
	CommandCode int              `json:"command-code"`
	Warnings    []string         `json:"warnings,omitempty"`
	Failure     string           `json:"failure,omitempty"`
}

// l2tpScaleReport is the result of the native scale runner.
type l2tpScaleReport struct {
	Action    string                    `json:"action"`
	Root      string                    `json:"root"`
	Selection string                    `json:"selection,omitempty"`
	Scenarios []L2TPScaleScenarioReport `json:"scenarios"`
	Passed    int                       `json:"passed"`
	Failed    int                       `json:"failed"`
	Code      int                       `json:"code"`
	Failure   string                    `json:"failure,omitempty"`
}

// Text preserves the former runner's summary contract.
func (r l2tpScaleReport) Text() string {
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

type l2tpScaleScenario struct {
	name          string
	tunnels       int
	sessions      int
	radiusDelay   time.Duration
	dwell         time.Duration
	poolExhausted bool
	check         func(*L2TPScaleResult) ([]string, error)
}

var l2tpScaleScenarioRegistry = [...]l2tpScaleScenario{
	{
		name: "2k-sessions", tunnels: 10, sessions: 200, dwell: time.Second,
		check: checkL2TPScale2K,
	},
	{
		name: "clean-teardown", tunnels: 2, sessions: 10, dwell: time.Second,
		check: checkL2TPScaleCleanTeardown,
	},
	{
		name: "pool-exhaustion", tunnels: 1, sessions: 300, dwell: time.Second,
		poolExhausted: true, check: checkL2TPScalePoolExhaustion,
	},
	{
		name: "slow-radius", tunnels: 2, sessions: 5, radiusDelay: 500 * time.Millisecond,
		dwell: time.Second, check: checkL2TPScaleSlowRADIUS,
	},
}

// l2tpScaleScenarios returns the exact, stable scenario registry in runner order.
func l2tpScaleScenarios() []string {
	names := make([]string, 0, len(l2tpScaleScenarioRegistry))
	for _, scenario := range l2tpScaleScenarioRegistry {
		names = append(names, scenario.name)
	}
	return names
}

func l2tpScaleOptions() l2tpScaleRunOptions {
	return l2tpScaleRunOptions{Scenario: env.Get("l2tp.scale.scenario")}
}

// runL2TPScale resolves the checkout and runs the native scale registry.
func runL2TPScale(ctx context.Context, options l2tpScaleRunOptions) (l2tpScaleReport, int) {
	root, err := lepath.Root()
	if err != nil {
		report := l2tpScaleReport{Action: L2TPScaleAction, Failure: err.Error(), Code: 1}
		return report, report.Code
	}
	return runL2TPScaleAtRoot(ctx, root, options)
}

// runL2TPScaleAtRoot runs every former checker, or one exact selected checker.
func runL2TPScaleAtRoot(
	ctx context.Context,
	root string,
	options l2tpScaleRunOptions,
) (l2tpScaleReport, int) {
	return runL2TPScaleAt(ctx, root, options, realL2TPScaleSystem{})
}

func runL2TPScaleAt(
	ctx context.Context,
	root string,
	options l2tpScaleRunOptions,
	system l2tpScaleSystem,
) (l2tpScaleReport, int) {
	report := l2tpScaleReport{Action: L2TPScaleAction, Root: root, Selection: options.Scenario}
	selected := make([]l2tpScaleScenario, 0, len(l2tpScaleScenarioRegistry))
	for _, scenario := range l2tpScaleScenarioRegistry {
		if options.Scenario == "" || options.Scenario == scenario.name {
			selected = append(selected, scenario)
		}
	}
	if len(selected) == 0 {
		report.Failure = fmt.Sprintf("no scenario matching %q found", options.Scenario)
		report.Code = 1
		return report, report.Code
	}

	zeBinary := findL2TPScaleBinary(system, root, "ze")
	zeTestBinary := findL2TPScaleBinary(system, root, "ze-test")
	if zeBinary == "" || zeTestBinary == "" {
		missing := "ze"
		if zeBinary != "" {
			missing = "ze-test"
		}
		report.Failure = "bin/" + missing + " not found"
		report.Code = 1
		return report, report.Code
	}

	for _, scenario := range selected {
		result := runL2TPScaleScenario(ctx, system, zeBinary, zeTestBinary, scenario)
		report.Scenarios = append(report.Scenarios, result)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	if report.Failed > 0 {
		report.Code = 1
	}
	return report, report.Code
}

func findL2TPScaleBinary(system l2tpScaleSystem, root, name string) string {
	path := filepath.Join(root, "bin", name)
	if system.FileExists(path) {
		return path
	}
	envName := strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_BINARY"
	path = system.Getenv(envName)
	if path != "" && system.FileExists(path) {
		return path
	}
	return ""
}

func runL2TPScaleScenario(
	ctx context.Context,
	system l2tpScaleSystem,
	zeBinary string,
	zeTestBinary string,
	scenario l2tpScaleScenario,
) (report L2TPScaleScenarioReport) {
	report.Name = scenario.name
	port, err := system.freeUDPPort()
	if err != nil {
		report.Failure = "find free L2TP port: " + err.Error()
		return report
	}
	config := defaultL2TPScaleConfig(port)
	if scenario.poolExhausted {
		config = exhaustedL2TPScaleConfig(port)
	}
	environ := slices.Clone(system.Environ())
	environ = append(environ, "ze.log.l2tp=warn", "ze.l2tp.skip-kernel-probe=true")
	ze, err := system.Start(ctx, l2tpScaleStart{
		argv: []string{zeBinary, "-"}, environ: environ, stdin: []byte(config),
	})
	if err != nil {
		report.Failure = "start ze: " + err.Error()
		return report
	}
	defer func() {
		if stopErr := ze.Stop(l2tpScaleStopWait, l2tpScaleKillWait); stopErr != nil && report.Failure == "" {
			report.Failure = "stop ze: " + stopErr.Error()
		}
		report.Passed = report.Failure == ""
	}()
	if err := system.Sleep(ctx, l2tpScaleStartupWait); err != nil {
		report.Failure = "ze startup wait canceled"
		return report
	}
	if exited, code, stderr, inspectErr := ze.Exited(); inspectErr != nil {
		report.Failure = "inspect ze: " + inspectErr.Error()
		return report
	} else if exited {
		report.CommandCode = code
		report.Failure = "ze exited early: " + firstL2TPScaleBytes(stderr, 500)
		return report
	}

	argv := []string{
		zeTestBinary, "l2tp-scale",
		"--target", fmt.Sprintf("127.0.0.1:%d", port),
		"--tunnels", strconv.Itoa(scenario.tunnels),
		"--sessions", strconv.Itoa(scenario.sessions),
		"--secret", "s3cr3t",
		"--radius-delay", scenario.radiusDelay.String(),
		"--dwell", scenario.dwell.String(),
		"--json",
	}
	command, runErr := system.Run(ctx, l2tpScaleCommand{
		argv: argv, environ: system.Environ(), timeout: l2tpScaleSessionTimeout(system.Getenv("SESSION_TIMEOUT")),
	})
	report.CommandCode = command.code
	report.ResultBytes = command.stdout
	report.ErrorBytes = command.stderr
	if runErr != nil {
		report.Failure = "run ze-test l2tp-scale: " + runErr.Error()
		return report
	}
	if len(bytes.TrimSpace(command.stdout)) == 0 {
		report.Failure = "no result from ze-test l2tp-scale"
		return report
	}
	var result L2TPScaleResult
	if err := json.Unmarshal(command.stdout, &result); err != nil {
		report.Failure = "invalid JSON output: " + firstL2TPScaleBytes(command.stdout, 200)
		return report
	}
	report.Result = &result
	report.Warnings, err = scenario.check(&result)
	if err != nil {
		report.Failure = err.Error()
	}
	return report
}

func l2tpScaleSessionTimeout(value string) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return l2tpScaleTimeout
	}
	return time.Duration(seconds) * time.Second
}

func checkL2TPScale2K(result *L2TPScaleResult) ([]string, error) {
	const tunnels = 10
	const sessions = 2_000
	if result.TunnelsUp != tunnels {
		return nil, fmt.Errorf("tunnels: %d/%d", result.TunnelsUp, tunnels)
	}
	if result.SessionsUp != sessions {
		return nil, fmt.Errorf("sessions: %d/%d", result.SessionsUp, sessions)
	}
	if result.SessionsPerSec < 100 {
		return []string{fmt.Sprintf("rate %.1f sessions/s < 100", result.SessionsPerSec)}, nil
	}
	return nil, nil
}

func checkL2TPScaleCleanTeardown(result *L2TPScaleResult) ([]string, error) {
	const sessions = 20
	if result.SessionsUp != sessions {
		return nil, fmt.Errorf("sessions: %d/%d", result.SessionsUp, sessions)
	}
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("%d errors during test", len(result.Errors))
	}
	return nil, nil
}

func checkL2TPScalePoolExhaustion(result *L2TPScaleResult) ([]string, error) {
	const poolSize = 254
	if result.SessionsUp > poolSize {
		return nil, fmt.Errorf("more sessions than pool addresses: %d > %d", result.SessionsUp, poolSize)
	}
	if result.SessionsUp == 0 {
		return nil, errors.New("no sessions established at all")
	}
	return nil, nil
}

func checkL2TPScaleSlowRADIUS(result *L2TPScaleResult) ([]string, error) {
	const sessions = 10
	if result.SessionsUp != sessions {
		return nil, fmt.Errorf("sessions: %d/%d with 500ms RADIUS delay", result.SessionsUp, sessions)
	}
	return nil, nil
}

func defaultL2TPScaleConfig(port int) string {
	return fmt.Sprintf(`l2tp {
    enabled true
    shared-secret s3cr3t
    auth {
        local {
            user test {
                password testpass
            }
        }
    }
    pool {
        ipv4 {
            gateway 10.255.0.1
            start 10.255.0.2
            end 10.255.15.254
            dns-primary 8.8.8.8
        }
    }
}
environment {
    l2tp {
        server main {
            ip 127.0.0.1
            port %d
        }
    }
}
`, port)
}

func exhaustedL2TPScaleConfig(port int) string {
	return fmt.Sprintf(`l2tp {
    enabled true
    shared-secret s3cr3t
    auth {
        local {
            user test {
                password testpass
            }
        }
    }
    pool {
        ipv4 {
            gateway 10.99.0.1
            start 10.99.0.2
            end 10.99.0.255
            dns-primary 8.8.8.8
        }
    }
}
environment {
    l2tp {
        server main {
            ip 127.0.0.1
            port %d
        }
    }
}
`, port)
}

func firstL2TPScaleBytes(content []byte, maximum int) string {
	if len(content) > maximum {
		content = content[:maximum]
	}
	return string(content)
}

type l2tpScaleStart struct {
	argv    []string
	environ []string
	stdin   []byte
}

type l2tpScaleCommand struct {
	argv    []string
	environ []string
	timeout time.Duration
}

type l2tpScaleCommandResult struct {
	stdout []byte
	stderr []byte
	code   int
}

// l2tpScaleProcess owns one Ze child. The caller MUST call Stop after Start,
// and Stop MUST terminate and reap the child within its two supplied bounds.
type l2tpScaleProcess interface {
	Exited() (exited bool, code int, stderr []byte, err error)
	Stop(termWait, killWait time.Duration) error
}

type l2tpScaleSystem interface {
	FileExists(path string) bool
	Getenv(key string) string
	Environ() []string
	freeUDPPort() (int, error)
	Start(context.Context, l2tpScaleStart) (l2tpScaleProcess, error)
	Run(context.Context, l2tpScaleCommand) (l2tpScaleCommandResult, error)
	Sleep(context.Context, time.Duration) error
}

type realL2TPScaleSystem struct{}

func (realL2TPScaleSystem) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (realL2TPScaleSystem) Getenv(key string) string { return os.Getenv(key) }

func (realL2TPScaleSystem) Environ() []string { return os.Environ() }

func (realL2TPScaleSystem) freeUDPPort() (port int, err error) {
	address, err := net.ResolveUDPAddr("udp4", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	conn, err := net.ListenUDP("udp4", address)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = errors.Join(err, conn.Close())
	}()
	localAddress, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected UDP listener address %T", conn.LocalAddr())
	}
	return localAddress.Port, nil
}

func (realL2TPScaleSystem) Start(
	ctx context.Context,
	start l2tpScaleStart,
) (l2tpScaleProcess, error) {
	cmd := exec.CommandContext(ctx, start.argv[0], start.argv[1:]...) //nolint:gosec // fixed native scenario grammar owns argv
	cmd.Env = start.environ
	cmd.Stdin = bytes.NewReader(start.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	process := &realL2TPScaleProcess{
		cmd: cmd, stdout: &stdout, stderr: &stderr, done: make(chan struct{}),
	}
	go process.collect()
	return process, nil
}

func (realL2TPScaleSystem) Run(
	ctx context.Context,
	command l2tpScaleCommand,
) (l2tpScaleCommandResult, error) {
	commandCtx, cancel := context.WithTimeout(ctx, command.timeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, command.argv[0], command.argv[1:]...) //nolint:gosec // fixed native scenario grammar owns argv
	cmd.Env = command.environ
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := l2tpScaleCommandResult{
		stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: gaterun.ExitCode(err),
	}
	if commandCtx.Err() != nil {
		return result, commandCtx.Err()
	}
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return result, err
		}
	}
	return result, nil
}

func (realL2TPScaleSystem) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type realL2TPScaleProcess struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

func (p *realL2TPScaleProcess) collect() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	close(p.done)
}

func (p *realL2TPScaleProcess) Exited() (bool, int, []byte, error) {
	select {
	case <-p.done:
		code, err := p.result()
		return true, code, p.stderr.Bytes(), ignoreL2TPScaleExit(err)
	default:
		return false, 0, nil, nil
	}
}

func (p *realL2TPScaleProcess) Stop(termWait, killWait time.Duration) error {
	select {
	case <-p.done:
		_, err := p.result()
		return ignoreL2TPScaleExit(err)
	default:
	}
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if waitL2TPScaleProcess(p.done, termWait) {
		_, err := p.result()
		return ignoreL2TPScaleExit(err)
	}
	if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	if !waitL2TPScaleProcess(p.done, killWait) {
		return errors.New("ze did not exit after kill")
	}
	_, err := p.result()
	return ignoreL2TPScaleExit(err)
}

func (p *realL2TPScaleProcess) result() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return gaterun.ExitCode(p.err), p.err
}

func waitL2TPScaleProcess(done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func ignoreL2TPScaleExit(err error) error {
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
}

var _ l2tpScaleSystem = realL2TPScaleSystem{}
var _ l2tpScaleProcess = (*realL2TPScaleProcess)(nil)
