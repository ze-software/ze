// Design: docs/architecture/chaos-web-dashboard.md -- multi-seed in-process swarm fuzzing runner

package fuzz

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/inprocess"
	"github.com/ze-software/ze/internal/chaos/mocknet"
	"github.com/ze-software/ze/internal/chaos/report"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/chaos/validation"
	_ "github.com/ze-software/ze/internal/component/bgp/plugin"      // register bgp top-level config/bus
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rib" // register bgp-rib plugin for in-process reactor
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/rs"  // register bgp-rs plugin for in-process reactor
	_ "github.com/ze-software/ze/internal/component/plugin/all"      // register YANG schemas and plugins
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	defaultFuzzSeeds               = 10
	defaultFuzzDuration            = 15 * time.Second
	defaultFuzzMaxPeers            = 4
	defaultFuzzMaxRoutes           = 12
	defaultFuzzStepDelay           = 10 * time.Millisecond
	defaultFuzzConvergenceDeadline = 10 * time.Second
	defaultFuzzPerSeedTimeout      = 45 * time.Second
)

// Config controls a multi-seed in-process swarm fuzzing run.
type Config struct {
	Seeds               int
	BaseSeed            uint64
	Duration            time.Duration
	MaxPeers            int
	MaxRoutes           int
	StepDelay           time.Duration
	ConvergenceDeadline time.Duration
	CheckDeterminism    bool
	OutDir              string
	Quiet               bool
	Out                 io.Writer
}

func (c *Config) defaults() {
	if c.Seeds <= 0 {
		c.Seeds = defaultFuzzSeeds
	}
	if c.Duration <= 0 {
		c.Duration = defaultFuzzDuration
	}
	if c.MaxPeers <= 0 {
		c.MaxPeers = defaultFuzzMaxPeers
	}
	if c.MaxRoutes <= 0 {
		c.MaxRoutes = defaultFuzzMaxRoutes
	}
	if c.StepDelay <= 0 {
		c.StepDelay = defaultFuzzStepDelay
	}
	if c.ConvergenceDeadline <= 0 {
		c.ConvergenceDeadline = defaultFuzzConvergenceDeadline
	}
	if c.Out == nil {
		c.Out = os.Stderr
	}
}

// SeedReport holds the verdict and telemetry for one fuzzed seed.
type SeedReport struct {
	Seed           uint64
	Profile        Profile
	Pass           bool
	Error          string
	Violations     []string
	Events         int
	FSMTransitions int
	StreamFaults   mocknet.StreamFaultStats
	FinalStateHash uint64
	Elapsed        time.Duration
	Repro          string
}

// Summary aggregates results across all seeds in a swarm fuzzing run.
type Summary struct {
	BaseSeed            uint64
	TotalSeeds          int
	Passed              int
	Failed              int
	TotalVirtualTime    time.Duration
	TotalEvents         int
	TotalFSMTransitions int
	TotalStreamFaults   mocknet.StreamFaultStats
	Failures            []SeedReport
	Elapsed             time.Duration
}

// Run executes cfg.Seeds deterministic in-process swarm scenarios, validating
// online reactor invariants and BGP properties on every seed.
func Run(ctx context.Context, cfg Config) (*Summary, error) {
	cfg.defaults()

	baseSeed := cfg.BaseSeed
	if baseSeed == 0 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return nil, err
		}
		baseSeed = binary.BigEndian.Uint64(buf[:])
		if baseSeed == 0 {
			baseSeed = 1
		}
	}

	start := time.Now()
	summary := &Summary{BaseSeed: baseSeed}

	for i := range cfg.Seeds {
		if err := ctx.Err(); err != nil {
			summary.Elapsed = time.Since(start)
			return summary, err
		}
		seed := baseSeed + uint64(i)
		rep := runOneSeed(ctx, seed, cfg)
		summary.TotalSeeds++
		summary.TotalVirtualTime += cfg.Duration
		summary.TotalEvents += rep.Events
		summary.TotalFSMTransitions += rep.FSMTransitions
		summary.TotalStreamFaults.ReadFragments += rep.StreamFaults.ReadFragments
		summary.TotalStreamFaults.WriteFragments += rep.StreamFaults.WriteFragments
		summary.TotalStreamFaults.Blackholes += rep.StreamFaults.Blackholes
		summary.TotalStreamFaults.Resets += rep.StreamFaults.Resets

		if rep.Pass {
			summary.Passed++
		} else {
			summary.Failed++
			summary.Failures = append(summary.Failures, rep)
		}

		if !cfg.Quiet {
			writeSeedLine(cfg.Out, i+1, cfg.Seeds, &rep)
		}
	}

	summary.Elapsed = time.Since(start)
	return summary, nil
}

func runOneSeed(ctx context.Context, seed uint64, cfg Config) SeedReport {
	prof := SwarmProfile(seed, cfg.MaxPeers, cfg.MaxRoutes)
	rep := SeedReport{
		Seed:    seed,
		Profile: prof,
		Repro:   prof.ReproCommand(cfg.Duration),
	}

	start := time.Now()
	res, violations, err := executeProfile(ctx, prof, cfg)
	rep.Elapsed = time.Since(start)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}

	rep.Events = len(res.Events)
	rep.FSMTransitions = res.FSMTransitionCount
	rep.StreamFaults = res.StreamFaults
	rep.FinalStateHash = res.FinalStateHash
	rep.Violations = violations

	if cfg.CheckDeterminism && len(rep.Violations) == 0 {
		res2, violations2, err2 := executeProfile(ctx, prof, cfg)
		if err2 != nil {
			rep.Error = "determinism check failed: " + err2.Error()
			return rep
		}
		if len(violations2) > 0 {
			rep.Violations = append(rep.Violations, "determinism: second run produced violations where first passed")
		} else if res2.FinalStateHash != res.FinalStateHash {
			var tb textbuf.Buffer
			tb.Str("determinism: state hash diverged between identical runs (")
			appendHex64(&tb, res.FinalStateHash)
			tb.Str(" vs ")
			appendHex64(&tb, res2.FinalStateHash)
			tb.Byte(')')
			rep.Violations = append(rep.Violations, tb.String())
		}
	}

	if len(rep.Violations) == 0 {
		rep.Pass = true
		return rep
	}

	if cfg.OutDir != "" {
		_ = writeFailureLog(cfg.OutDir, prof, res, cfg)
	}
	return rep
}

func executeProfile(ctx context.Context, prof Profile, cfg Config) (*inprocess.RunResult, []string, error) {
	profiles, err := scenario.Generate(scenario.GeneratorParams{
		Seed:      prof.Seed,
		Peers:     prof.Peers,
		IBGPRatio: prof.IBGPRatio,
		LocalAS:   65000,
		Routes:    prof.Routes,
		BasePort:  1850,
		Families:  prof.Families,
	})
	if err != nil {
		return nil, nil, err
	}

	seedCtx, cancel := context.WithTimeout(ctx, max(cfg.Duration+defaultFuzzPerSeedTimeout, defaultFuzzPerSeedTimeout))
	defer cancel()

	res, err := inprocess.Run(seedCtx, inprocess.RunConfig{
		Profiles:       profiles,
		Seed:           prof.Seed,
		Duration:       cfg.Duration,
		StepDelay:      cfg.StepDelay,
		LocalAS:        65000,
		RouterID:       netip.MustParseAddr("10.0.0.1"),
		LocalAddr:      "127.0.0.1",
		ChaosRate:      prof.ChaosRate,
		ChaosInterval:  1 * time.Second,
		RouteRate:      prof.RouteRate,
		RouteInterval:  3 * time.Second,
		Warmup:         3 * time.Second,
		BaseRoutes:     prof.Routes,
		EnabledActions: prof.EnabledActions,
		ActionParams: map[engine.ActionType]map[string]string{
			engine.ActionClockDrift:      {engine.ParamDrift: "100ms"},
			engine.ActionRouteBurst:      {engine.ParamCount: "8"},
			engine.ActionWithdrawalBurst: {engine.ParamCount: "4"},
			engine.ActionRouteFlap:       {engine.ParamCount: "4", engine.ParamCycles: "2", engine.ParamInterval: "50ms"},
		},
		StreamFaults:  &prof.StreamFaults,
		SettleTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}

	violations := append([]string(nil), res.InvariantViolations...)

	ibgp := make([]bool, len(profiles))
	peerFams := make([][]string, len(profiles))
	for i := range profiles {
		ibgp[i] = profiles[i].IsIBGP
		peerFams[i] = profiles[i].Families
	}
	reachable := validation.ScenarioReachability(ibgp, peerFams)
	props, err := validation.ScenarioProperties("all", len(profiles), cfg.ConvergenceDeadline, reachable)
	if err != nil {
		return nil, nil, err
	}

	for i := range res.Events {
		ev := res.Events[i]
		if ev.PlannedShutdown {
			continue
		}
		if !ev.Time.Before(res.ObservationEnd) {
			continue
		}
		props.ProcessEvent(ev)
	}
	for _, v := range props.AllViolations() {
		var tb textbuf.Buffer
		violations = append(violations, tb.Str(v.Property).Str(": ").Str(v.Message).String())
	}

	return res, violations, nil
}

func writeFailureLog(outDir string, prof Profile, res *inprocess.RunResult, cfg Config) error {
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	var nameBuf textbuf.Buffer
	path := filepath.Join(outDir, nameBuf.Str("seed-").Uint(prof.Seed).Str(".ndjson").String())
	f, err := cliio.Create(path)
	if err != nil {
		return err
	}
	jl := report.NewJSONLog(f, report.JSONLogConfig{
		Start:               time.Now(),
		Seed:                prof.Seed,
		Peers:               prof.Peers,
		ChaosRate:           prof.ChaosRate,
		ConvergenceDeadline: cfg.ConvergenceDeadline,
		Properties:          "all",
	})
	for i := range res.Events {
		ev := res.Events[i]
		if ev.PlannedShutdown {
			continue
		}
		if !ev.Time.Before(res.ObservationEnd) {
			continue
		}
		jl.ProcessEvent(ev)
	}
	jl.EndObservation(res.ObservationEnd)
	return errors.Join(jl.Close(), f.Close())
}

func writeSeedLine(w io.Writer, idx, total int, rep *SeedReport) {
	status := "PASS"
	if !rep.Pass {
		status = "FAIL"
	}
	var tb textbuf.Buffer
	tb.Str("[").Int(int64(idx)).Byte('/').Int(int64(total)).Str("] ").
		Str(status).Str(" seed=").Uint(rep.Seed).
		Str(" peers=").Int(int64(rep.Profile.Peers)).
		Str(" routes=").Int(int64(rep.Profile.Routes)).
		Str(" fsm=").Int(int64(rep.FSMTransitions)).
		Str(" stream-faults=").Int(rep.StreamFaults.Total()).
		Str(" hash=")
	appendHex64(&tb, rep.FinalStateHash)
	tb.Str(" (").Str(rep.Elapsed.Truncate(time.Millisecond).String()).Str(")\n")
	_, _ = w.Write(tb.Bytes())
}

func appendHex64(tb *textbuf.Buffer, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	tb.Hex(buf[:])
}

func writeSummary(w io.Writer, s *Summary) {
	var tb textbuf.Buffer
	tb.Str("\nle chaos fuzz | base-seed: ").Uint(s.BaseSeed).
		Str(" | passed: ").Int(int64(s.Passed)).Byte('/').Int(int64(s.TotalSeeds)).
		Str(" | virtual-time: ").Str(s.TotalVirtualTime.String()).
		Str(" | fsm-transitions: ").Int(int64(s.TotalFSMTransitions)).
		Str(" | stream-faults: ").Int(s.TotalStreamFaults.Total()).
		Str(" | elapsed: ").Str(s.Elapsed.Truncate(time.Millisecond).String()).Byte('\n')
	for i := range s.Failures {
		f := &s.Failures[i]
		tb.Str("FAIL seed=").Uint(f.Seed)
		if f.Error != "" {
			tb.Str(" err=").Str(f.Error)
		}
		tb.Byte('\n')
		for _, v := range f.Violations {
			tb.Str("  - ").Str(v).Byte('\n')
		}
		tb.Str("  repro: ").Str(f.Repro).Byte('\n')
	}
	_, _ = w.Write(tb.Bytes())
}

// CLIRun parses `le chaos fuzz` flags, executes the swarm run, and returns the exit code.
func CLIRun(args []string) int {
	fs := flag.NewFlagSet("le chaos fuzz", flag.ContinueOnError)

	seeds := fs.Int("seeds", defaultFuzzSeeds, "Number of seeds to fuzz")
	baseSeed := fs.Uint64("base-seed", 0, "Starting seed (0 = random base seed)")
	duration := fs.Duration("duration", defaultFuzzDuration, "Virtual simulation duration per seed")
	maxPeers := fs.Int("max-peers", defaultFuzzMaxPeers, "Maximum simulated peers per seed (2-16)")
	maxRoutes := fs.Int("max-routes", defaultFuzzMaxRoutes, "Maximum routes per peer (1-500)")
	convergenceDeadline := fs.Duration("convergence-deadline", defaultFuzzConvergenceDeadline, "Route convergence deadline per seed")
	checkDeterminism := fs.Bool("check-determinism", false, "Run each seed twice and verify identical state hash and verdict")
	outDir := fs.String("out-dir", "", "Directory to save NDJSON event logs for failing seeds")
	quiet := fs.Bool("quiet", false, "Suppress per-seed progress lines")

	fs.Usage = func() {
		var tb textbuf.Buffer
		tb.Str(`le chaos fuzz - VOPR-style multi-seed in-process swarm fuzzer for Ze BGP

Usage:
  le chaos fuzz [options]

Options:
  --seeds <N>                Number of seeds to run (default: 10)
  --base-seed <uint64>       Starting seed (default: random, printed in summary)
  --duration <dur>           Virtual time simulated per seed (default: 15s)
  --max-peers <N>            Maximum peers per seed (2-16, default: 4)
  --max-routes <N>           Maximum routes per peer (1-500, default: 12)
  --convergence-deadline <d> Convergence property deadline (default: 10s)
  --check-determinism        Run each seed twice and assert identical state hash
  --out-dir <dir>            Write NDJSON event logs for failing seeds
  --quiet                    Print summary only
`)
		_, _ = fs.Output().Write(tb.Bytes())
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if err := validateCLIFlags(*seeds, *duration, *maxPeers, *maxRoutes, *convergenceDeadline); err != nil {
		var tb textbuf.Buffer
		tb.Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
		return 1
	}

	summary, err := Run(context.Background(), Config{
		Seeds:               *seeds,
		BaseSeed:            *baseSeed,
		Duration:            *duration,
		MaxPeers:            *maxPeers,
		MaxRoutes:           *maxRoutes,
		ConvergenceDeadline: *convergenceDeadline,
		CheckDeterminism:    *checkDeterminism,
		OutDir:              *outDir,
		Quiet:               *quiet,
		Out:                 os.Stderr,
	})
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI diagnostic
		return 1
	}

	writeSummary(os.Stderr, summary)
	if summary.Failed > 0 {
		return 1
	}
	return 0
}

func validateCLIFlags(seeds int, duration time.Duration, maxPeers, maxRoutes int, deadline time.Duration) error {
	if seeds < 1 {
		return opererr.New("--seeds must be at least 1",
			"Swarm fuzzing needs at least one seed to execute.",
			"./le chaos fuzz --seeds 10")
	}
	if duration <= 0 {
		return opererr.New("--duration must be positive",
			"Each seed needs a positive virtual simulation window.",
			"./le chaos fuzz --duration 15s")
	}
	if maxPeers < 2 {
		return opererr.New("--max-peers must be at least 2",
			"Route propagation checks require at least two BGP peers.",
			"./le chaos fuzz --max-peers 4")
	}
	if maxPeers > 16 {
		return opererr.New("--max-peers must be at most 16",
			"In-process swarm fuzzing caps peers per seed to keep per-seed latency low.",
			"./le chaos fuzz --max-peers 8")
	}
	if maxRoutes < 1 {
		return opererr.New("--max-routes must be at least 1",
			"Route consistency checks need at least one route per peer.",
			"./le chaos fuzz --max-routes 12")
	}
	if maxRoutes > 500 {
		return opererr.New("--max-routes must be at most 500",
			"Swarm fuzzing caps routes per peer to sweep many seeds quickly.",
			"./le chaos fuzz --max-routes 50")
	}
	if deadline <= 0 {
		return opererr.New("--convergence-deadline must be positive",
			"Convergence validation needs a positive latency limit.",
			"./le chaos fuzz --convergence-deadline 10s")
	}
	return nil
}
