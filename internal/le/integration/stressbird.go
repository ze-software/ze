// Design: test/stress/run.py -- the lifecycle and verdict this native gate preserves.
// Detail: test/stress/harness.py -- namespace, process, BIRD, wait, and cleanup behavior.
// Related: test/stress/scenarios/04-bulk-ipv4-bird/check.py -- the four route rounds.
//
// stressbird.go owns the BIRD baseline scenario as callable Go. BIRD, birdc, ip,
// ethtool, ss, and bin/ze-test remain external because they are the systems this
// integration gate exercises. No repository-owned Python, Make, sudo, or go-run
// process sits between the action and this runner.
package integration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// StressBirdGate is the Make gate this runner replaces.
	StressBirdGate = "ze-stress-bird-test"

	stressBirdScenario       = "04-bulk-ipv4-bird"
	stressBirdZeIP           = "172.31.0.2"
	stressBirdZeCIDR         = "172.31.0.2/24"
	stressBirdZeDial         = "172.31.0.2:179"
	stressBirdPeerIP         = "172.31.0.3"
	stressBirdPeerCIDR       = "172.31.0.3/24"
	stressBirdASN            = 65100
	stressBirdDwell          = "30s"
	stressBirdCommandTimeout = 30 * time.Second
	stressBirdStartupWait    = 2 * time.Second
	stressBirdPollInterval   = 2 * time.Second
	stressBirdRouteTimeout   = 120 * time.Second
	stressBirdStopTimeout    = 5 * time.Second
	stressBirdCleanupTimeout = 2 * time.Minute
	stressBirdSetupTimeout   = 10 * time.Minute
	stressBirdTimeoutCode    = 124
)

var (
	errStressBirdWaitTimeout = errors.New("stress-bird: process wait timed out")
	stressBirdRoutePattern   = regexp.MustCompile(`(?m)(\d+)\s+of\s+\d+\s+routes`)
)

// StressBirdRoundReport is one completed or failed injection round.
type StressBirdRoundReport struct {
	PrefixBase       string  `json:"prefix-base"`
	Prefixes         int     `json:"prefixes"`
	PeerTimeoutSecs  int     `json:"peer-timeout-seconds"`
	RouteTimeoutSecs int     `json:"route-timeout-seconds"`
	ObservedRoutes   int     `json:"observed-routes"`
	RouteQueries     int     `json:"route-queries"`
	InjectSeconds    float64 `json:"inject-seconds"`
	IngestSeconds    float64 `json:"ingest-seconds"`
	RoutesPerSecond  float64 `json:"routes-per-second"`
	Passed           bool    `json:"passed"`
}

// StressBirdFailure identifies the first failed phase and keeps its exit code.
type StressBirdFailure struct {
	Phase    string `json:"phase"`
	Round    int    `json:"round-prefixes,omitempty"`
	Message  string `json:"message"`
	ExitCode int    `json:"exit-code"`
}

// StressBirdReport is the structured result of the complete BIRD baseline.
type StressBirdReport struct {
	Gate          string                  `json:"gate"`
	Scenario      string                  `json:"scenario"`
	Root          string                  `json:"root"`
	ZeNamespace   string                  `json:"ze-namespace"`
	PeerNamespace string                  `json:"peer-namespace"`
	Rounds        []StressBirdRoundReport `json:"rounds"`
	CleanupErrors []string                `json:"cleanup-errors,omitempty"`
	Warnings      []string                `json:"warnings,omitempty"`
	Failure       *StressBirdFailure      `json:"failure,omitempty"`
	Passed        bool                    `json:"passed"`
	Code          int                     `json:"code"`
}

// Text answers the producer-compatible default summary. Structured renderers
// still receive StressBirdReport itself.
func (r StressBirdReport) Text() string {
	var text textbuf.Buffer
	if r.Passed {
		return text.Str("PASS  1 scenario(s): ").Str(r.Scenario).String()
	}
	if r.Failure == nil {
		return text.Str("FAIL  ").Str(r.Scenario).String()
	}
	return text.Str("FAIL  ").Str(r.Scenario).Str(": ").Str(r.Failure.Message).String()
}

// RunStressBird is the action callback the integration composition can wire
// directly. It resolves the checkout once and returns the scenario's exact code.
func RunStressBird() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		report := newStressBirdReport("")
		return failStressBird(report, "preflight", 0, 1, err)
	}
	report, code := runStressBird(context.Background(), root, realStressBirdSystem{})
	return report, code
}

// RunStressBirdAt runs the native scenario against root. Callers that already
// resolved the checkout avoid a second filesystem walk.
func RunStressBirdAt(ctx context.Context, root string) (StressBirdReport, int) {
	return runStressBird(ctx, root, realStressBirdSystem{})
}

type stressBirdRound struct {
	prefixBase string
	prefixes   int
	peerWait   time.Duration
}

var stressBirdRounds = [...]stressBirdRound{
	{prefixBase: "10.0.0.0/24", prefixes: 100_000, peerWait: 120 * time.Second},
	{prefixBase: "10.64.0.0/24", prefixes: 250_000, peerWait: 180 * time.Second},
	{prefixBase: "10.128.0.0/24", prefixes: 500_000, peerWait: 300 * time.Second},
	{prefixBase: "11.0.0.0/24", prefixes: 1_000_000, peerWait: 600 * time.Second},
}

type stressBirdCommand struct {
	argv       []string
	environ    []string
	outputPath string
	timeout    time.Duration
}

type stressBirdCommandResult struct {
	stdout string
	stderr string
	code   int
}

// stressBirdProcess is one started BIRD or peer lifecycle. The caller MUST call
// Wait after Terminate or Kill. Safe for concurrent use.
type stressBirdProcess interface {
	Exited() (bool, int, error)
	Wait(time.Duration) (int, error)
	Terminate() error
	Kill() error
}

// stressBirdSystem has two implementations: the host and the recorder fixtures.
// It keeps process effects injectable without changing the scenario control flow.
type stressBirdSystem interface {
	EUID() int
	LookPath(string) (string, error)
	FileExists(string) bool
	PID() int
	Environ() []string
	Getenv(string) string
	Run(context.Context, stressBirdCommand) (stressBirdCommandResult, error)
	Start(context.Context, stressBirdCommand) (stressBirdProcess, error)
	Remove(string) error
	Sleep(context.Context, time.Duration) error
	Now() time.Time
}

type stressBirdPaths struct {
	zeLog    string
	peerLog  string
	pcapLog  string
	birdLog  string
	birdPID  string
	birdSock string
}

type stressBirdRunner struct {
	root      string
	system    stressBirdSystem
	environ   []string
	suffix    string
	zeNS      string
	peerNS    string
	zeVeth    string
	peerVeth  string
	paths     stressBirdPaths
	processes []stressBirdProcess
	warnings  []string
}

func runStressBird(
	ctx context.Context,
	root string,
	system stressBirdSystem,
) (StressBirdReport, int) {
	runner := stressBirdRunner{root: root, system: system, environ: system.Environ()}
	return runner.run(ctx)
}

func (r *stressBirdRunner) run(ctx context.Context) (report StressBirdReport, code int) {
	r.initializeNames()
	report = newStressBirdReport(r.root)
	report.ZeNamespace = r.zeNS
	report.PeerNamespace = r.peerNS

	defer func() {
		cleanupErrors := r.cleanup(ctx, true)
		report.CleanupErrors = append(report.CleanupErrors, cleanupErrors...)
		report.Warnings = append(report.Warnings, r.warnings...)
		if code == 0 && len(cleanupErrors) > 0 {
			report, code = failStressBird(
				report, "cleanup", 0, 1,
				fmt.Errorf("cleanup failed: %s", strings.Join(cleanupErrors, "; ")),
			)
		}
		report.Code = code
		report.Passed = code == 0
	}()

	if failure := r.preflight(ctx); failure != nil {
		return failStressBird(report, failure.Phase, 0, failure.ExitCode, errors.New(failure.Message))
	}

	// The Python Scenario.setup starts by tearing down stale state. Keep that
	// order so an interrupted prior run cannot collide with this one.
	initialCleanupErrors := r.cleanup(ctx, false)
	report.CleanupErrors = append(report.CleanupErrors, initialCleanupErrors...)
	if failure := r.createNamespaces(ctx); failure != nil {
		return failStressBird(report, failure.Phase, 0, failure.ExitCode, errors.New(failure.Message))
	}
	if failure := r.startBird(ctx); failure != nil {
		return failStressBird(report, failure.Phase, 0, failure.ExitCode, errors.New(failure.Message))
	}

	for _, round := range stressBirdRounds {
		roundReport, failure := r.runRound(ctx, round)
		report.Rounds = append(report.Rounds, roundReport)
		if failure != nil {
			return failStressBird(
				report, failure.Phase, round.prefixes, failure.ExitCode,
				errors.New(failure.Message),
			)
		}
	}
	return report, 0
}

func (r *stressBirdRunner) initializeNames() {
	r.suffix = r.system.Getenv("ZE_STRESS_SUFFIX")
	if r.suffix == "" {
		r.suffix = strconv.Itoa(r.system.PID())
	}
	vethSuffix := r.suffix
	if len(vethSuffix) > 6 {
		vethSuffix = vethSuffix[:6]
	}
	var name textbuf.Buffer
	r.zeNS = name.Str("ze-stress-ze-").Str(r.suffix).String()
	r.peerNS = name.Reset().Str("ze-stress-bb-").Str(r.suffix).String()
	r.zeVeth = name.Reset().Str("ze-v-").Str(vethSuffix).String()
	r.peerVeth = name.Reset().Str("bb-v-").Str(vethSuffix).String()
	r.paths = stressBirdPaths{
		zeLog:    name.Reset().Str("/tmp/ze-stress-ze-").Str(r.suffix).Str(".log").String(),
		peerLog:  name.Reset().Str("/tmp/ze-stress-peer-").Str(r.suffix).Str(".log").String(),
		pcapLog:  name.Reset().Str("/tmp/ze-stress-pcap-").Str(r.suffix).Str(".txt").String(),
		birdLog:  name.Reset().Str("/tmp/ze-stress-bird-").Str(r.suffix).Str(".log").String(),
		birdPID:  name.Reset().Str("/tmp/ze-stress-bird-").Str(r.suffix).Str(".pid").String(),
		birdSock: name.Reset().Str("/tmp/ze-stress-bird-").Str(r.suffix).Str(".ctl").String(),
	}
}

func newStressBirdReport(root string) StressBirdReport {
	return StressBirdReport{
		Gate: StressBirdGate, Scenario: stressBirdScenario, Root: root,
		Rounds: make([]StressBirdRoundReport, 0, len(stressBirdRounds)),
	}
}

func failStressBird(
	report StressBirdReport,
	phase string,
	round int,
	code int,
	err error,
) (StressBirdReport, int) {
	if code == 0 {
		code = 1
	}
	report.Failure = &StressBirdFailure{
		Phase: phase, Round: round, Message: err.Error(), ExitCode: code,
	}
	report.Code = code
	report.Passed = false
	return report, code
}

func (r *stressBirdRunner) preflight(ctx context.Context) *StressBirdFailure {
	if r.system.EUID() != 0 {
		return stressBirdFailure("preflight", 1, "must run as root for network namespaces")
	}

	runtimeMissing := false
	for _, tool := range [...]string{"ip", "ethtool"} {
		if _, err := r.system.LookPath(tool); err != nil {
			runtimeMissing = true
		}
	}
	if runtimeMissing {
		if failure := r.installRuntimeTools(ctx); failure != nil {
			return failure
		}
	}
	var message textbuf.Buffer
	for _, tool := range [...]string{"ip", "ethtool"} {
		if _, err := r.system.LookPath(tool); err != nil {
			return stressBirdFailure(
				"preflight", gaterun.CannotStart,
				message.Str("setup completed but required command is still missing: ").
					Str(tool).String(),
			)
		}
	}

	birdMissing := false
	for _, tool := range [...]string{"bird", "birdc"} {
		if _, err := r.system.LookPath(tool); err != nil {
			birdMissing = true
		}
	}
	if birdMissing {
		if failure := r.installBird(ctx); failure != nil {
			return failure
		}
	}
	for _, tool := range [...]string{"bird", "birdc"} {
		if _, err := r.system.LookPath(tool); err != nil {
			return stressBirdFailure(
				"preflight", gaterun.CannotStart,
				message.Reset().Str("BIRD setup completed but required command is still missing: ").
					Str(tool).String(),
			)
		}
	}

	peer := filepath.Join(r.root, "bin", "ze-test")
	if !r.system.FileExists(peer) {
		return stressBirdFailure(
			"preflight", gaterun.CannotStart,
			message.Reset().Str("bin/ze-test not found at ").Str(peer).
				Str("; build ze-test first").String(),
		)
	}
	config := filepath.Join(
		r.root, "test", "stress", "scenarios", stressBirdScenario, "bird.conf",
	)
	if !r.system.FileExists(config) {
		return stressBirdFailure(
			"preflight", gaterun.CannotStart,
			message.Reset().Str("BIRD configuration not found at ").Str(config).String(),
		)
	}
	return nil
}

func (r *stressBirdRunner) installRuntimeTools(ctx context.Context) *StressBirdFailure {
	if _, err := r.system.LookPath("apt-get"); err != nil {
		return stressBirdFailure(
			"preflight", gaterun.CannotStart,
			"ip or ethtool is missing and apt-get is unavailable",
		)
	}
	if failure := r.runRequiredWithTimeout(
		ctx, "preflight", []string{"apt-get", "update", "-qq"}, stressBirdSetupTimeout,
	); failure != nil {
		return failure
	}
	return r.runRequiredWithTimeout(
		ctx,
		"preflight",
		[]string{
			"apt-get", "install", "-y", "--no-install-recommends",
			"iproute2", "ethtool", "tcpdump", "jq",
		},
		stressBirdSetupTimeout,
	)
}

func (r *stressBirdRunner) installBird(ctx context.Context) *StressBirdFailure {
	if _, err := r.system.LookPath("apt-get"); err != nil {
		return stressBirdFailure(
			"preflight", gaterun.CannotStart,
			"BIRD is missing and apt-get is unavailable",
		)
	}
	return r.runRequiredWithTimeout(
		ctx,
		"preflight",
		[]string{"apt-get", "install", "-y", "--no-install-recommends", "bird2"},
		stressBirdRouteTimeout,
	)
}

func (r *stressBirdRunner) createNamespaces(ctx context.Context) *StressBirdFailure {
	commands := [][]string{
		{"ip", "netns", "add", r.zeNS},
		{"ip", "netns", "add", r.peerNS},
		{"ip", "link", "add", r.zeVeth, "type", "veth", "peer", "name", r.peerVeth},
		{"ip", "link", "set", r.zeVeth, "netns", r.zeNS},
		{"ip", "link", "set", r.peerVeth, "netns", r.peerNS},
		r.namespaceArgv(r.zeNS, "ip", "addr", "add", stressBirdZeCIDR, "dev", r.zeVeth),
		r.namespaceArgv(r.zeNS, "ip", "link", "set", r.zeVeth, "up"),
		r.namespaceArgv(r.zeNS, "ip", "link", "set", "lo", "up"),
		r.namespaceArgv(r.peerNS, "ip", "addr", "add", stressBirdPeerCIDR, "dev", r.peerVeth),
		r.namespaceArgv(r.peerNS, "ip", "link", "set", r.peerVeth, "up"),
		r.namespaceArgv(r.peerNS, "ip", "link", "set", "lo", "up"),
	}
	for _, argv := range commands {
		if failure := r.runRequired(ctx, "namespace", argv); failure != nil {
			return failure
		}
	}
	// Offload was historically disabled for a userspace TCP stack. Both DUTs in
	// this scenario use kernel TCP, so the producer explicitly treats it as best effort.
	r.runOptional(ctx, r.namespaceArgv(
		r.zeNS, "ethtool", "-K", r.zeVeth, "tx", "off", "rx", "off",
	))
	r.runOptional(ctx, r.namespaceArgv(
		r.peerNS, "ethtool", "-K", r.peerVeth, "tx", "off", "rx", "off",
	))
	return nil
}

func (r *stressBirdRunner) startBird(ctx context.Context) *StressBirdFailure {
	config := filepath.Join(
		r.root, "test", "stress", "scenarios", stressBirdScenario, "bird.conf",
	)
	command := stressBirdCommand{
		argv: r.namespaceArgv(
			r.zeNS, "bird", "-f", "-c", config,
			"-P", r.paths.birdPID, "-s", r.paths.birdSock,
		),
		environ: r.environ, outputPath: r.paths.birdLog,
	}
	process, err := r.system.Start(ctx, command)
	if err != nil {
		var message textbuf.Buffer
		return stressBirdFailure(
			"bird-start", gaterun.CannotStart,
			message.Str("start BIRD: ").Err(err).String(),
		)
	}
	r.processes = append(r.processes, process)
	if err := r.system.Sleep(ctx, stressBirdStartupWait); err != nil {
		return stressBirdFailure("bird-start", stressBirdTimeoutCode, "BIRD startup wait canceled")
	}
	exited, code, err := process.Exited()
	if err != nil {
		var message textbuf.Buffer
		return stressBirdFailure("bird-start", 1, message.Str("inspect BIRD: ").Err(err).String())
	}
	if exited {
		if code == 0 {
			code = 1
		}
		var message textbuf.Buffer
		return stressBirdFailure(
			"bird-start", code,
			message.Str("BIRD exited immediately with code ").Int(int64(code)).String(),
		)
	}
	// Listening state is diagnostic only in the producer. BIRD remaining alive
	// and the first birdc query are the authoritative readiness checks.
	r.runOptional(ctx, r.namespaceArgv(r.zeNS, "ss", "-tln", "sport", "=", "179"))
	return nil
}

func (r *stressBirdRunner) runRound(
	ctx context.Context,
	round stressBirdRound,
) (StressBirdRoundReport, *StressBirdFailure) {
	report := StressBirdRoundReport{
		PrefixBase: round.prefixBase, Prefixes: round.prefixes,
		PeerTimeoutSecs:  int(round.peerWait / time.Second),
		RouteTimeoutSecs: int(stressBirdRouteTimeout / time.Second),
	}
	injectStart := r.system.Now()
	peer, failure := r.startPeer(ctx, round)
	if failure != nil {
		return report, failure
	}
	code, err := peer.Wait(round.peerWait)
	report.InjectSeconds = r.system.Now().Sub(injectStart).Seconds()
	var message textbuf.Buffer
	if errors.Is(err, errStressBirdWaitTimeout) {
		message.Str("peer inject timed out after ").Str(round.peerWait.String())
		if killErr := peer.Kill(); killErr != nil {
			message.Str("; kill failed: ").Err(killErr)
		}
		if _, reapErr := peer.Wait(stressBirdStopTimeout); reapErr != nil {
			message.Str("; reap failed: ").Err(reapErr)
		}
		return report, stressBirdFailure("peer", stressBirdTimeoutCode, message.String())
	}
	if err != nil {
		return report, stressBirdFailure(
			"peer", 1, message.Str("wait for peer inject: ").Err(err).String(),
		)
	}
	if code != 0 {
		return report, stressBirdFailure(
			"peer", code,
			message.Str("peer inject exited with code ").Int(int64(code)).String(),
		)
	}

	ingestStart := r.system.Now()
	failure = r.waitRoutes(ctx, &report)
	report.IngestSeconds = r.system.Now().Sub(ingestStart).Seconds()
	if report.InjectSeconds > 0 {
		report.RoutesPerSecond = float64(round.prefixes) / report.InjectSeconds
	}
	if failure != nil {
		return report, failure
	}
	report.Passed = true
	return report, nil
}

func (r *stressBirdRunner) startPeer(
	ctx context.Context,
	round stressBirdRound,
) (stressBirdProcess, *StressBirdFailure) {
	peer := filepath.Join(r.root, "bin", "ze-test")
	command := stressBirdCommand{
		argv: r.namespaceArgv(
			r.peerNS,
			peer, "peer", "--mode", "inject", "--dial", stressBirdZeDial,
			"--inject-prefix", round.prefixBase,
			"--inject-count", strconv.Itoa(round.prefixes),
			"--inject-nexthop", stressBirdPeerIP,
			"--inject-asn", strconv.Itoa(stressBirdASN),
			"--inject-dwell", stressBirdDwell,
		),
		environ: r.environ, outputPath: r.paths.peerLog,
	}
	process, err := r.system.Start(ctx, command)
	if err != nil {
		var message textbuf.Buffer
		return nil, stressBirdFailure(
			"peer", gaterun.CannotStart,
			message.Str("start peer inject: ").Err(err).String(),
		)
	}
	r.processes = append(r.processes, process)
	return process, nil
}

func (r *stressBirdRunner) waitRoutes(
	ctx context.Context,
	report *StressBirdRoundReport,
) *StressBirdFailure {
	deadline := r.system.Now().Add(stressBirdRouteTimeout)
	polls := int(stressBirdRouteTimeout / stressBirdPollInterval)
	for range polls {
		remaining := deadline.Sub(r.system.Now())
		if remaining <= 0 {
			break
		}
		queryTimeout := min(stressBirdCommandTimeout, remaining)
		count, failure := r.birdRouteCount(ctx, queryTimeout)
		report.RouteQueries++
		report.ObservedRoutes = count
		if failure != nil {
			return failure
		}
		if count >= report.Prefixes {
			return nil
		}
		sleep := min(stressBirdPollInterval, deadline.Sub(r.system.Now()))
		if sleep <= 0 {
			break
		}
		if err := r.system.Sleep(ctx, sleep); err != nil {
			return stressBirdFailure("bird-routes", stressBirdTimeoutCode, "BIRD route wait canceled")
		}
	}
	count, failure := r.birdRouteCount(ctx, stressBirdCommandTimeout)
	report.RouteQueries++
	report.ObservedRoutes = count
	if failure != nil {
		return failure
	}
	var message textbuf.Buffer
	return stressBirdFailure(
		"bird-routes", stressBirdTimeoutCode,
		message.Str("BIRD RIB has ").Int(int64(count)).
			Str(" routes, expected at least ").Int(int64(report.Prefixes)).String(),
	)
}

func (r *stressBirdRunner) birdRouteCount(
	ctx context.Context,
	timeout time.Duration,
) (int, *StressBirdFailure) {
	argv := r.namespaceArgv(
		r.zeNS, "birdc", "-s", r.paths.birdSock, "show", "route", "count",
	)
	result, err := r.system.Run(ctx, stressBirdCommand{
		argv: argv, environ: r.environ, timeout: timeout,
	})
	var message textbuf.Buffer
	if err != nil {
		return 0, stressBirdFailure(
			"bird-routes", commandErrorCode(err),
			message.Str("query BIRD routes: ").Err(err).String(),
		)
	}
	if result.code != 0 {
		return 0, stressBirdFailure(
			"bird-routes", result.code,
			message.Str("birdc show route count exited with code ").Int(int64(result.code)).
				Str(": ").Str(strings.TrimSpace(result.stderr)).String(),
		)
	}
	match := stressBirdRoutePattern.FindStringSubmatch(result.stdout)
	if len(match) != 2 {
		return 0, stressBirdFailure(
			"bird-routes", 1,
			message.Str("birdc returned no route count: ").
				Quoted(strings.TrimSpace(result.stdout)).String(),
		)
	}
	count, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, stressBirdFailure(
			"bird-routes", 1,
			message.Str("parse BIRD route count: ").Err(err).String(),
		)
	}
	return count, nil
}

func (r *stressBirdRunner) runRequired(
	ctx context.Context,
	phase string,
	argv []string,
) *StressBirdFailure {
	return r.runRequiredWithTimeout(ctx, phase, argv, stressBirdCommandTimeout)
}

func (r *stressBirdRunner) runRequiredWithTimeout(
	ctx context.Context,
	phase string,
	argv []string,
	timeout time.Duration,
) *StressBirdFailure {
	result, err := r.system.Run(ctx, stressBirdCommand{
		argv: argv, environ: r.environ, timeout: timeout,
	})
	var message textbuf.Buffer
	if err != nil {
		return stressBirdFailure(
			phase, commandErrorCode(err),
			message.Str("run ").Join(argv, " ").Str(": ").Err(err).String(),
		)
	}
	if result.code != 0 {
		return stressBirdFailure(
			phase, result.code,
			message.Join(argv, " ").Str(" exited with code ").Int(int64(result.code)).
				Str(": ").Str(strings.TrimSpace(result.stderr)).String(),
		)
	}
	return nil
}

func (r *stressBirdRunner) runOptional(ctx context.Context, argv []string) {
	result, err := r.system.Run(ctx, stressBirdCommand{
		argv: argv, environ: r.environ, timeout: stressBirdCommandTimeout,
	})
	var message textbuf.Buffer
	if err != nil {
		r.warnings = append(
			r.warnings,
			message.Join(argv, " ").Str(": ").Err(err).String(),
		)
		return
	}
	if result.code != 0 {
		r.warnings = append(
			r.warnings,
			message.Join(argv, " ").Str(": exit ").Int(int64(result.code)).
				Str(": ").Str(strings.TrimSpace(result.stderr)).String(),
		)
	}
}

func (r *stressBirdRunner) namespaceArgv(namespace string, argv ...string) []string {
	command := make([]string, 0, len(argv)+4)
	command = append(command, "ip", "netns", "exec", namespace)
	return append(command, argv...)
}

func (r *stressBirdRunner) cleanup(ctx context.Context, reportErrors bool) []string {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stressBirdCleanupTimeout)
	defer cancel()
	ctx = cleanupCtx
	problems := make([]string, 0)
	var message textbuf.Buffer
	for _, process := range r.processes {
		if err := process.Terminate(); err != nil && reportErrors && !errors.Is(err, os.ErrProcessDone) {
			problems = append(problems, message.Str("terminate process: ").Err(err).String())
		}
		if _, err := process.Wait(stressBirdStopTimeout); errors.Is(err, errStressBirdWaitTimeout) {
			if killErr := process.Kill(); killErr != nil && reportErrors {
				problems = append(
					problems,
					message.Reset().Str("kill process: ").Err(killErr).String(),
				)
			}
			if _, waitErr := process.Wait(stressBirdStopTimeout); waitErr != nil && reportErrors {
				problems = append(
					problems,
					message.Reset().Str("reap process: ").Err(waitErr).String(),
				)
			}
		} else if err != nil && reportErrors {
			problems = append(
				problems,
				message.Reset().Str("wait for process cleanup: ").Err(err).String(),
			)
		}
	}
	r.processes = r.processes[:0]

	for _, namespace := range [...]string{r.zeNS, r.peerNS} {
		result, err := r.system.Run(ctx, stressBirdCommand{
			argv:    []string{"ip", "netns", "del", namespace},
			environ: r.environ, timeout: stressBirdCommandTimeout,
		})
		if reportErrors && err != nil {
			problems = append(
				problems,
				message.Reset().Str("delete namespace ").Str(namespace).Str(": ").Err(err).String(),
			)
		}
		if reportErrors && err == nil && result.code != 0 {
			problems = append(
				problems,
				message.Reset().Str("delete namespace ").Str(namespace).
					Str(": exit ").Int(int64(result.code)).String(),
			)
		}
	}
	for _, path := range [...]string{
		r.paths.zeLog,
		r.paths.peerLog,
		r.paths.pcapLog,
		r.paths.birdLog,
		r.paths.birdPID,
		r.paths.birdSock,
	} {
		if err := r.system.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && reportErrors {
			problems = append(
				problems,
				message.Reset().Str("remove ").Str(path).Str(": ").Err(err).String(),
			)
		}
	}
	return problems
}

func stressBirdFailure(phase string, code int, message string) *StressBirdFailure {
	return &StressBirdFailure{Phase: phase, Message: message, ExitCode: code}
}

func commandErrorCode(err error) int {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return stressBirdTimeoutCode
	}
	return gaterun.CannotStart
}

type realStressBirdSystem struct{}

func (realStressBirdSystem) EUID() int { return os.Geteuid() }

func (realStressBirdSystem) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (realStressBirdSystem) FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (realStressBirdSystem) PID() int                 { return os.Getpid() }
func (realStressBirdSystem) Environ() []string        { return os.Environ() }
func (realStressBirdSystem) Getenv(key string) string { return os.Getenv(key) }
func (realStressBirdSystem) Now() time.Time           { return time.Now() }

func (realStressBirdSystem) Run(
	ctx context.Context,
	command stressBirdCommand,
) (stressBirdCommandResult, error) {
	commandCtx := ctx
	cancel := func() {}
	if command.timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, command.timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(commandCtx, command.argv[0], command.argv[1:]...) //nolint:gosec // fixed scenario grammar owns argv
	cmd.Env = command.environ
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := stressBirdCommandResult{
		stdout: stdout.String(), stderr: stderr.String(), code: gaterun.ExitCode(err),
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

func (realStressBirdSystem) Start(
	ctx context.Context,
	command stressBirdCommand,
) (stressBirdProcess, error) {
	output, err := os.OpenFile(command.outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open process log %s: %w", command.outputPath, err)
	}
	cmd := exec.CommandContext(ctx, command.argv[0], command.argv[1:]...) //nolint:gosec // fixed scenario grammar owns argv
	cmd.Env = command.environ
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(err, output.Close())
	}
	process := &realStressBirdProcess{cmd: cmd, output: output, done: make(chan struct{})}
	go process.collect()
	return process, nil
}

func (realStressBirdSystem) Remove(path string) error { return os.Remove(path) }

func (realStressBirdSystem) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// realStressBirdProcess owns one wait goroutine from Start until the child exits.
// The caller MUST call Wait after Terminate or Kill. Safe for concurrent use.
type realStressBirdProcess struct {
	cmd    *exec.Cmd
	output io.Closer
	done   chan struct{}
	mu     sync.Mutex
	code   int
	err    error
}

func (p *realStressBirdProcess) collect() {
	err := p.cmd.Wait()
	closeErr := p.output.Close()
	p.mu.Lock()
	p.code = gaterun.ExitCode(err)
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			p.err = err
		}
	}
	if p.err == nil && closeErr != nil {
		p.err = closeErr
	}
	p.mu.Unlock()
	close(p.done)
}

func (p *realStressBirdProcess) Exited() (bool, int, error) {
	select {
	case <-p.done:
		code, err := p.result()
		return true, code, err
	default:
		return false, 0, nil
	}
}

func (p *realStressBirdProcess) Wait(timeout time.Duration) (int, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return p.result()
	case <-timer.C:
		return 0, errStressBirdWaitTimeout
	}
}

func (p *realStressBirdProcess) Terminate() error {
	select {
	case <-p.done:
		return nil
	default:
		return p.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func (p *realStressBirdProcess) Kill() error {
	select {
	case <-p.done:
		return nil
	default:
		return p.cmd.Process.Kill()
	}
}

func (p *realStressBirdProcess) result() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code, p.err
}

var _ stressBirdSystem = realStressBirdSystem{}
var _ stressBirdProcess = (*realStressBirdProcess)(nil)
