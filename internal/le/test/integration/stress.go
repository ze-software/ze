package testintegration

import (
	"context"
	"encoding/json"
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
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

const StressAction = "stress"

const (
	stressZeASN          = 65100
	stressZeStartupWait  = 2 * time.Second
	stressFlapPause      = 2 * time.Second
	stressProfileStartup = time.Second
	stressProfileWait    = 120 * time.Second
	// stressZeBuildTimeout bounds the per-run DUT build. A warm build cache
	// finishes in seconds; a cold one (first run as root) compiles every gate.
	stressZeBuildTimeout = 15 * time.Minute
	// stressZeBinaryRel is where the per-run DUT build lands, under the
	// checkout's tmp/. It is never bin/ze, which other tools build with their
	// own tags and which the runner must not mistake for the tree under test.
	stressZeBinaryRel = "tmp/stress/ze"
	// stressProfileRoot is the directory the profile log is written to, inside
	// the network namespace the scenario runs in.
	stressProfileRoot = "/tmp"
)

// stressProfilePath answers the CPU profile log of one stress run.
func stressProfilePath(suffix string) string {
	return filepath.Join(stressProfileRoot, "ze-stress-profile-"+suffix+".log")
}

// stressReceiverLogPath answers the receiver sink's log of one stress run.
func stressReceiverLogPath(suffix string) string {
	return filepath.Join(stressProfileRoot, "ze-stress-receiver-"+suffix+".log")
}

// stressConfigDir answers the directory one stress run copies the DUT's config
// into, and so the directory its config store opens in.
//
// Ze keeps its config store beside the config it starts on, and refuses a store
// whose files another user owns. The scenario directory is in the checkout,
// which a VM guest reads over 9p with the host user's uid while the DUT runs as
// root, so started there the DUT refuses to start. A directory this run creates
// is owned by the user the DUT runs as, on a host run and in a guest alike.
func stressConfigDir(suffix string) string {
	return filepath.Join(stressProfileRoot, "ze-stress-config-"+suffix)
}

// stressOptions selects one exact scenario. An empty selection runs the complete registry.
// Prefixes, when above zero, replaces every Ze round's prefix count: a smoke run
// that proves a scenario's wiring without its full load. Zero keeps the registry's
// counts, which are the only ones a measurement records.
type stressOptions struct {
	Scenario string
	Prefixes int
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
	// Routes is the best table's route total when a query rendered it; a
	// profile file carries none.
	Routes int64 `json:"routes,omitempty"`
}

// StressScenarioReport is the verdict for one exact former checker.
type StressScenarioReport struct {
	Name          string                `json:"name"`
	Passed        bool                  `json:"passed"`
	Rounds        []StressRoundReport   `json:"rounds,omitempty"`
	Profiles      []StressProfileReport `json:"profiles,omitempty"`
	Queries       []StressProfileReport `json:"queries,omitempty"`
	Binary        string                `json:"binary,omitempty"`
	Bird          *StressBirdReport     `json:"bird,omitempty"`
	Warnings      []string              `json:"warnings,omitempty"`
	CleanupErrors []string              `json:"cleanup-errors,omitempty"`
	Failure       string                `json:"failure,omitempty"`
	ExitCode      int                   `json:"exit-code"`
}

// StressReport is the result of the native stress runner.
type StressReport struct {
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
func (r StressReport) Text() string {
	if r.Failure != "" {
		return r.Failure
	}
	if r.Failed == 0 {
		return fmt.Sprintf("PASS  %d scenario(s)", r.Passed)
	}
	failed := make([]string, 0, r.Failed)
	for i := range r.Scenarios {
		if !r.Scenarios[i].Passed {
			failed = append(failed, r.Scenarios[i].Name)
		}
	}
	return fmt.Sprintf("FAIL  %d passed, %d failed: %s", r.Passed, r.Failed, strings.Join(failed, " "))
}

type stressScenario struct {
	name   string
	config string
	rounds []stressRound
	// reach is nil for a scenario that measures the injector-to-DUT path
	// alone. The profile scenario sets it, see stressProfileReach.
	reach *stressReach
}

// stressReach is what a scenario adds around the injector so that its profile
// carries the paths perf round 3 changed (docs/architecture/perf-round-3.md).
// A single injector and a DUT that keeps the routes reach none of them: no
// UPDATE is forwarded, no filter rewrites one, and no route is ever rendered.
type stressReach struct {
	// receiverCIDR and receiverIP are the eBGP sink's address on the peer
	// namespace's link; receiverASN is the AS it opens with. The scenario's
	// ze.conf names the same address and AS for its "receiver" peer.
	receiverCIDR string
	receiverIP   string
	receiverASN  int
	// queryURL is fetched in the DUT namespace while the routes are held. The
	// looking glass serves it from `show bgp rib best`, which renders each
	// route's communities through Community.AppendText.
	queryURL string
}

// stressProfileReach is the profile scenario's reach: one eBGP receiver the
// DUT forwards to through an export modify policy, and one best-table query.
var stressProfileReach = stressReach{
	receiverCIDR: "172.31.0.4/24",
	receiverIP:   "172.31.0.4",
	receiverASN:  65200,
	queryURL:     "http://127.0.0.1:8443/api/looking-glass/routes/table/ipv4%2Funicast",
}

// probeURL answers the query asking for one route, whose pagination still
// carries the whole table's route total.
func (reach *stressReach) probeURL() string { return reach.queryURL + "?limit=1" }

type stressRound struct {
	prefixBase string
	nexthop    string
	prefixes   int
	dwell      string
	timeout    time.Duration
	pause      time.Duration
	// perUpdate caps the prefixes one UPDATE carries (0 packs each message)
	// and varyAttrs gives every UPDATE its own MED and COMMUNITIES. The
	// profile scenario sets both: the paths it profiles run once per UPDATE,
	// so a packed stream (20000 prefixes in 21 UPDATEs) never reaches them.
	perUpdate int
	varyAttrs bool
}

var stressScenarioRegistry = [...]stressScenario{
	{
		name: "01-bulk-ipv4", config: zeConfigFile,
		rounds: []stressRound{
			{prefixBase: stressPrefixBase, nexthop: stressBirdPeerIP, prefixes: 100_000, dwell: dwellFifteenSeconds, timeout: 120 * time.Second},
			{prefixBase: "10.64.0.0/24", nexthop: stressBirdPeerIP, prefixes: 250_000, dwell: dwellFifteenSeconds, timeout: 180 * time.Second},
			{prefixBase: "10.128.0.0/24", nexthop: stressBirdPeerIP, prefixes: 500_000, dwell: dwellFifteenSeconds, timeout: 300 * time.Second},
			{prefixBase: "11.0.0.0/24", nexthop: stressBirdPeerIP, prefixes: 1_000_000, dwell: dwellFifteenSeconds, timeout: 600 * time.Second},
		},
	},
	{
		name: "02-multi-peer", config: zeConfigFile,
		rounds: []stressRound{
			{prefixBase: stressPrefixBase, nexthop: stressBirdPeerIP, prefixes: 500_000, dwell: "30s", timeout: 900 * time.Second},
			{prefixBase: "2001:db8::/48", nexthop: "2001:db8::3", prefixes: 250_000, dwell: "30s", timeout: 600 * time.Second},
		},
	},
	{
		name: "03-session-flap", config: zeConfigFile,
		rounds: stressFlapRounds(),
	},
	{name: stressBirdScenario, config: "bird.conf"},
	{
		name: scenarioProfile1M, config: zeConfigFile, reach: &stressProfileReach,
		rounds: []stressRound{
			{
				prefixBase: stressPrefixBase, nexthop: stressBirdPeerIP, prefixes: 1_000_000, dwell: "60s", timeout: 600 * time.Second,
				perUpdate: 1, varyAttrs: true,
			},
		},
	},
}

// stressRoundsWithPrefixes answers a copy of rounds with every prefix count
// replaced, leaving the registry itself untouched.
func stressRoundsWithPrefixes(rounds []stressRound, prefixes int) []stressRound {
	smoke := slices.Clone(rounds)
	for i := range smoke {
		smoke[i].prefixes = prefixes
	}
	return smoke
}

func stressFlapRounds() []stressRound {
	rounds := make([]stressRound, 0, 11)
	for range 10 {
		rounds = append(rounds, stressRound{
			prefixBase: stressPrefixBase, nexthop: stressBirdPeerIP, prefixes: 100_000,
			dwell: "2s", timeout: 180 * time.Second, pause: stressFlapPause,
		})
	}
	return append(rounds, stressRound{
		prefixBase: stressPrefixBase, nexthop: stressBirdPeerIP, prefixes: 100_000,
		dwell: "5s", timeout: 180 * time.Second,
	})
}

// StressScenarios returns the exact, stable scenario registry in runner order.
func StressScenarios() []string {
	names := make([]string, 0, len(stressScenarioRegistry))
	for _, scenario := range stressScenarioRegistry {
		names = append(names, scenario.name)
	}
	return names
}

// runStressAction runs every former stress checker, or one exact selected checker.
func runStressAction(ctx context.Context, root string, options stressOptions) (StressReport, int) {
	return runStressAt(ctx, root, options, realStressSystem{})
}

func runStressAt(
	ctx context.Context,
	root string,
	options stressOptions,
	system stressSystem,
) (StressReport, int) {
	report := StressReport{Action: StressAction, Root: root, Selection: options.Scenario}
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
	if options.Prefixes < 0 {
		report.Failure = fmt.Sprintf("stress.prefixes %d is negative; use 0 for the registry counts", options.Prefixes)
		report.Code = 1
		return report, report.Code
	}
	if options.Prefixes > 0 {
		for i := range selected {
			selected[i].rounds = stressRoundsWithPrefixes(selected[i].rounds, options.Prefixes)
		}
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
	WriteFile(path string, content []byte, mode os.FileMode) error
	RemoveAll(path string) error
	// daemonBuildTags answers the -tags value that builds the DUT from root.
	daemonBuildTags(root string) (string, error)
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
		_ = system.Remove(stressProfilePath(runner.base.suffix))
		_ = system.Remove(stressReceiverLogPath(runner.base.suffix))
		if err := system.RemoveAll(stressConfigDir(runner.base.suffix)); err != nil {
			report.Warnings = append(report.Warnings, "remove DUT config copy: "+err.Error())
		}
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
	report.Binary = runner.zeBinary
	if scenario.reach != nil {
		if failure := runner.startReceiver(ctx, scenario.reach); failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
			return report
		}
	}
	if failure := runner.startZe(ctx); failure != nil {
		report.Failure, report.ExitCode = failure.Message, failure.ExitCode
		return report
	}
	if scenario.name == scenarioProfile1M {
		if failure := runner.startCPUProfile(ctx); failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
			return report
		}
	}
	for _, round := range scenario.rounds {
		roundReport, query, failure := runner.runRound(ctx, round)
		report.Rounds = append(report.Rounds, roundReport)
		if query != nil {
			report.Queries = append(report.Queries, *query)
		}
		if failure != nil {
			report.Failure, report.ExitCode = failure.Message, failure.ExitCode
			return report
		}
	}
	if scenario.name == scenarioProfile1M {
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
	for _, tool := range [...]string{"ip", toolEthtool} {
		if _, err := r.system.LookPath(tool); err != nil {
			missingRuntime = true
		}
	}
	if missingRuntime {
		if failure := r.base.installRuntimeTools(ctx); failure != nil {
			return failure
		}
	}
	for _, tool := range [...]string{"ip", toolEthtool} {
		if _, err := r.system.LookPath(tool); err != nil {
			return stressBirdFailure("preflight", gaterun.CannotStart, "setup completed but required command is still missing: "+tool)
		}
	}

	if _, err := r.system.Executable(); err != nil {
		return stressBirdFailure("preflight", gaterun.CannotStart, "the BGP peer is this le (le test peer), and it cannot name its own file: "+err.Error())
	}
	if failure := r.buildZe(ctx); failure != nil {
		return failure
	}
	config := filepath.Join(r.base.root, "test", "stress", "scenarios", r.scenario.name, r.scenario.config)
	if !r.system.FileExists(config) {
		return stressBirdFailure("preflight", gaterun.CannotStart, "DUT configuration not found at "+config)
	}
	return nil
}

// buildZe compiles the DUT from the checkout under test, on every run.
//
// It never reuses a binary it finds. The runner used to start bin/ze whenever
// one existed, so a profile could measure a daemon weeks older than the tree
// it was recorded against, with nothing in the report to say so. Its fallback
// build was no better: `-tags ze_core,ze_distro` alone compiles every feature
// gate out, BGP included. The tags come from feature-gates.txt, as every other
// daemon build in le derives them, and the go build cache keeps a rebuild of
// an unchanged tree cheap.
func (r *stressRunner) buildZe(ctx context.Context) *StressBirdFailure {
	if _, err := r.system.LookPath("go"); err != nil {
		return stressBirdFailure("preflight", gaterun.CannotStart, "go is not in PATH, and the DUT is built from the checkout on every run")
	}
	tags, err := r.system.daemonBuildTags(r.base.root)
	if err != nil {
		return stressBirdFailure("preflight", gaterun.CannotStart, "derive the DUT build tags: "+err.Error())
	}
	r.zeBinary = filepath.Join(r.base.root, stressZeBinaryRel)
	environ := slices.Clone(r.base.environ)
	environ = append(environ, "CGO_ENABLED=0")
	result, err := r.system.Run(ctx, stressBirdCommand{
		argv: []string{"go", "build", "-tags", tags, "-o", r.zeBinary, "./cmd/ze"},
		dir:  r.base.root, environ: environ, timeout: stressZeBuildTimeout,
	})
	if err != nil {
		return stressBirdFailure("preflight", commandErrorCode(err), "build Ze: "+err.Error())
	}
	if result.code != 0 {
		return stressBirdFailure("preflight", result.code, stressBuildFailureMessage(result))
	}
	return nil
}

// stressBuildOutputLines bounds the compiler output a failed DUT build carries
// into the report. The compiler stops after ten errors and a cold build prints
// its downloads first, so the tail is where the errors are.
const stressBuildOutputLines = 20

// stressBuildFailureMessage answers why the DUT build failed: its exit status,
// then the tail of stderr, or of stdout when stderr is empty. It never answers
// the bare prefix: a smoke run once reported `build Ze: ` and nothing else,
// because the message was stderr alone and stderr was empty.
func stressBuildFailureMessage(result stressBirdCommandResult) string {
	var message textbuf.Buffer
	message.Str("build Ze: go build exited ").Int(int64(result.code))
	output := strings.TrimSpace(result.stderr)
	if output == "" {
		output = strings.TrimSpace(result.stdout)
	}
	if output == "" {
		return message.Str(" and printed nothing on stderr or stdout").String()
	}
	return message.Str(": ").Str(stressOutputTail(output, stressBuildOutputLines)).String()
}

// stressOutputTail answers the last lines of output, or all of it when it is
// shorter.
func stressOutputTail(output string, lines int) string {
	at := len(output)
	for range lines {
		at = strings.LastIndexByte(output[:at], '\n')
		if at < 0 {
			return output
		}
	}
	return output[at+1:]
}

// startReceiver brings up the eBGP sink the profile scenario forwards to: a
// second address on the peer namespace's link, and `le test peer --mode sink`
// listening on it. The DUT dials it (connect true in the scenario config), so
// it is up before the DUT starts.
func (r *stressRunner) startReceiver(ctx context.Context, reach *stressReach) *StressBirdFailure {
	argv := r.base.namespaceArgv(r.base.peerNS, "ip", "addr", ipAdd, reach.receiverCIDR, "dev", r.base.peerVeth)
	if failure := r.base.runRequired(ctx, "receiver", argv); failure != nil {
		return failure
	}
	peerBinary, err := r.system.Executable()
	if err != nil {
		return stressBirdFailure("receiver", gaterun.CannotStart, err.Error())
	}
	process, err := r.system.Start(ctx, stressBirdCommand{
		argv: r.base.namespaceArgv(
			r.base.peerNS,
			peerBinary, "test", "peer", "--mode", "sink",
			"--bind", reach.receiverIP, "--port", "179",
			"--asn", strconv.Itoa(reach.receiverASN),
		),
		environ: r.base.environ, outputPath: stressReceiverLogPath(r.base.suffix),
	})
	if err != nil {
		return stressBirdFailure("receiver", gaterun.CannotStart, "start receiver sink: "+err.Error())
	}
	r.base.processes = append(r.base.processes, process)
	return nil
}

// queryRIB waits for the injector to report its last byte sent, then fetches
// the best table through the looking glass while the routes are still held.
// The injector's dwell is the only window: once its session closes the routes
// are withdrawn and the table is empty.
func (r *stressRunner) queryRIB(
	ctx context.Context,
	reach *stressReach,
	round stressRound,
) (StressProfileReport, *StressBirdFailure) {
	report := StressProfileReport{Name: "looking-glass-best", Path: reach.queryURL}
	deadline := r.system.Now().Add(round.timeout)
	for {
		content, err := r.system.ReadFile(r.base.paths.peerLog)
		if err == nil && stressSentMetrics.Match(content) {
			break
		}
		if r.system.Now().After(deadline) {
			return report, stressBirdFailure("query", stressBirdTimeoutCode, "the injector never reported its last byte sent")
		}
		if err := r.system.Sleep(ctx, time.Second); err != nil {
			return report, stressBirdFailure("query", stressBirdTimeoutCode, "wait for the injector canceled")
		}
	}
	routes, failure := r.awaitBestRoutes(ctx, reach, deadline)
	if failure != nil {
		return report, failure
	}
	report.Routes = routes
	result, err := r.system.Run(ctx, stressBirdCommand{
		argv: r.base.namespaceArgv(
			r.base.zeNS, "curl", "-sS", "-f", "-o", "/dev/null", "-w", "%{size_download}", reach.queryURL,
		),
		environ: r.base.environ, timeout: stressProfileWait,
	})
	if err != nil {
		return report, stressBirdFailure("query", commandErrorCode(err), "query the looking glass: "+err.Error())
	}
	if result.code != 0 {
		return report, stressBirdFailure("query", result.code, "query the looking glass: "+strings.TrimSpace(result.stderr))
	}
	size, err := strconv.ParseInt(strings.TrimSpace(result.stdout), 10, 64)
	if err != nil {
		return report, stressBirdFailure("query", 1, "read the looking glass reply size: "+err.Error())
	}
	if size == 0 {
		return report, stressBirdFailure("query", 1, "the looking glass answered an empty best table")
	}
	report.Bytes = size
	return report, nil
}

// awaitBestRoutes probes the best table once a second until it holds a route,
// and answers the route total of the probe that found one.
//
// The injector's last byte reaches the DUT's socket before the RIB plugin has
// stored anything, so a query made at that moment renders an empty table and
// proves no route was rendered: a 2026-10-08 guest smoke answered 122 bytes for
// 20000 prefixes. The probe asks for one route, so its answer stays small at a
// million, and it reads the total the looking glass counts before paginating.
func (r *stressRunner) awaitBestRoutes(
	ctx context.Context,
	reach *stressReach,
	deadline time.Time,
) (int64, *StressBirdFailure) {
	for {
		result, err := r.system.Run(ctx, stressBirdCommand{
			argv:    r.base.namespaceArgv(r.base.zeNS, "curl", "-sS", "-f", reach.probeURL()),
			environ: r.base.environ, timeout: stressProfileWait,
		})
		if err != nil {
			return 0, stressBirdFailure("query", commandErrorCode(err), "probe the looking glass: "+err.Error())
		}
		if result.code != 0 {
			return 0, stressBirdFailure("query", result.code, "probe the looking glass: "+strings.TrimSpace(result.stderr))
		}
		var answer struct {
			Pagination *struct {
				TotalResults int64 `json:"total_results"`
			} `json:"pagination"`
		}
		if err := json.Unmarshal([]byte(result.stdout), &answer); err != nil {
			return 0, stressBirdFailure("query", 1, "read the looking glass probe: "+err.Error())
		}
		if answer.Pagination == nil {
			return 0, stressBirdFailure("query", 1, "the looking glass probe answered no pagination, so no route total")
		}
		if answer.Pagination.TotalResults > 0 {
			return answer.Pagination.TotalResults, nil
		}
		if r.system.Now().After(deadline) {
			return 0, stressBirdFailure("query", stressBirdTimeoutCode, "the looking glass best table stayed empty for the whole round")
		}
		if err := r.system.Sleep(ctx, time.Second); err != nil {
			return 0, stressBirdFailure("query", stressBirdTimeoutCode, "wait for the best table canceled")
		}
	}
}

func (r *stressRunner) startZe(ctx context.Context) *StressBirdFailure {
	config, failure := r.stageConfig()
	if failure != nil {
		return failure
	}
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

// stageConfig copies the scenario's config into this run's own directory and
// answers the copy's path, which is what the DUT starts on (stressConfigDir
// says why). A directory left by an earlier run with the same suffix is removed
// first, so the DUT never opens a store some other run wrote.
func (r *stressRunner) stageConfig() (string, *StressBirdFailure) {
	source := filepath.Join(r.base.root, "test", "stress", "scenarios", r.scenario.name, r.scenario.config)
	content, err := r.system.ReadFile(source)
	if err != nil {
		return "", stressBirdFailure("ze-config", gaterun.CannotStart, "read DUT configuration: "+err.Error())
	}
	dir := stressConfigDir(r.base.suffix)
	if err := r.system.RemoveAll(dir); err != nil {
		return "", stressBirdFailure("ze-config", gaterun.CannotStart, "clear DUT config directory: "+err.Error())
	}
	if err := r.system.MkdirAll(dir, 0o700); err != nil {
		return "", stressBirdFailure("ze-config", gaterun.CannotStart, "create DUT config directory: "+err.Error())
	}
	config := filepath.Join(dir, r.scenario.config)
	if err := r.system.WriteFile(config, content, 0o600); err != nil {
		return "", stressBirdFailure("ze-config", gaterun.CannotStart, "copy DUT configuration: "+err.Error())
	}
	return config, nil
}

func (r *stressRunner) runRound(
	ctx context.Context,
	round stressRound,
) (StressRoundReport, *StressProfileReport, *StressBirdFailure) {
	report := StressRoundReport{
		PrefixBase: round.prefixBase, Prefixes: round.prefixes, Dwell: round.dwell,
		TimeoutSeconds: int(round.timeout / time.Second),
	}
	started := r.system.Now()
	var query *StressProfileReport
	peer, failure := r.startPeer(ctx, round)
	if failure != nil {
		return report, query, failure
	}
	if r.scenario.reach != nil {
		// The peer stays in r.base.processes, so cleanup stops it on failure.
		result, failure := r.queryRIB(ctx, r.scenario.reach, round)
		query = &result
		if failure != nil {
			return report, query, failure
		}
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
		return report, query, stressBirdFailure("peer", stressBirdTimeoutCode, message)
	}
	if err != nil {
		return report, query, stressBirdFailure("peer", 1, "wait for peer inject: "+err.Error())
	}
	if code != 0 {
		message := fmt.Sprintf("peer inject failed with code %d", code)
		if content, readErr := r.system.ReadFile(r.base.paths.peerLog); readErr == nil {
			message += stressPeerLogTail(content)
		}
		return report, query, stressBirdFailure("peer", code, message)
	}
	content, readErr := r.system.ReadFile(r.base.paths.peerLog)
	if readErr != nil {
		r.base.warnings = append(r.base.warnings, "read peer metrics: "+readErr.Error())
	} else {
		report.Metrics = parseStressPeerMetrics(content)
	}
	if round.pause > 0 {
		if err := r.system.Sleep(ctx, round.pause); err != nil {
			return report, query, stressBirdFailure("flap-pause", stressBirdTimeoutCode, "flap pause canceled")
		}
	}
	return report, query, nil
}

func (r *stressRunner) startPeer(
	ctx context.Context,
	round stressRound,
) (stressBirdProcess, *StressBirdFailure) {
	peerBinary, err := r.system.Executable()
	if err != nil {
		return nil, stressBirdFailure("peer", gaterun.CannotStart, err.Error())
	}
	argv := r.base.namespaceArgv(
		r.base.peerNS,
		peerBinary, "test", "peer", "--mode", "inject", "--dial", stressBirdZeDial,
		"--inject-prefix", round.prefixBase,
		"--inject-count", strconv.Itoa(round.prefixes),
		"--inject-nexthop", round.nexthop,
		"--inject-asn", strconv.Itoa(stressZeASN),
		"--inject-dwell", round.dwell,
	)
	if round.perUpdate > 0 {
		argv = append(argv, "--inject-per-update", strconv.Itoa(round.perUpdate))
	}
	if round.varyAttrs {
		argv = append(argv, "--inject-vary-attrs")
	}
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
		outputPath: stressProfilePath(r.base.suffix),
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

func (realStressSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) } //nolint:gosec // a development tool reads the checkout it was pointed at

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

func (realStressSystem) WriteFile(path string, content []byte, mode os.FileMode) error {
	return os.WriteFile(path, content, mode)
}

func (realStressSystem) RemoveAll(path string) error { return os.RemoveAll(path) }

func (realStressSystem) daemonBuildTags(root string) (string, error) {
	return repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
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

// The scenario inputs and host tool names the stress runners repeat. ipAdd and
// ipLink are the iproute2 object and verb the namespace setup uses.
const (
	stressPrefixBase  = "10.0.0.0/24"
	toolEthtool       = "ethtool"
	zeConfigFile      = "ze.conf"
	scenarioProfile1M = "05-profile-1m"
	ipAdd             = "add"
	aptGet            = "apt-get"
	ipLink            = "link"
)

// The default per-round dwell, and the iproute2 object the lab namespaces use.
const (
	dwellFifteenSeconds = "15s"
	ipNetns             = "netns"
)
