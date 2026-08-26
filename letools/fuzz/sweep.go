// Design: docs/architecture/core-design.md -- what a fuzz run does and what it answers
//
// sweep.go defines a fuzz run and its two payloads. discover.go finds the targets.
//
// A fuzz run writes progress for a long time, so its progress log goes to stderr.
// The payload goes to stdout for the pipe operators.
// letools/deployment uses the same shape for a proof that drives a real peer.
// Thus, `le fuzz run | json` returns one sweep document while go test writes to the terminal.

package fuzz

import (
	"context"
	"io"
	"os"
	"os/exec"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/gotoolchain"
	"github.com/ze-software/ze/letools/leaction"
)

// nameColumn is how wide the target-name column of the listing is. It keeps a
// hundred rows aligned, which is the whole reason a reader can scan the page.
const nameColumn = 40

// Run is one planned or completed fuzz invocation: the target, the argv that
// runs it, and what it answered.
type Run struct {
	Name    string   `json:"name,omitempty"`
	Package string   `json:"package,omitempty"`
	Argv    []string `json:"argv"`
	// Code is what go test answered. It is absent from a plan, which has not
	// run anything.
	Code int  `json:"code,omitempty"`
	OK   bool `json:"ok,omitempty"`
}

// Plan is what `le fuzz list` answers: every invocation a run would make.
type Plan struct {
	FuzzTime string `json:"fuzztime"`
	Timeout  string `json:"timeout"`
	Runs     []Run  `json:"runs"`
	// Named records whether the caller selected a target instead of discovery.
	// It selects either a target table or one argv for the listing.
	Named bool `json:"named,omitempty"`
}

// Text renders the plan like the Python output.
// It returns either a padded target table and count or one argv for a named run.
func (p Plan) Text() string {
	var tb textbuf.Buffer
	if p.Named {
		for _, run := range p.Runs {
			tb.Str("  ").Join(run.Argv, " ").Byte('\n')
		}
		return tb.String()
	}

	for _, run := range p.Runs {
		tb.Str("  ").PadRight(run.Name, nameColumn).Byte(' ').Str(run.Package).Byte('\n')
	}
	tb.Int(int64(len(p.Runs))).Str(" fuzz target(s), ").Str(p.FuzzTime).Str(" each\n")
	return tb.String()
}

// Sweep is what `le fuzz run` answers: what ran, in order, and how it went.
type Sweep struct {
	FuzzTime string `json:"fuzztime"`
	Timeout  string `json:"timeout"`
	// Results is the row set, and it is the ONLY one: two keys carrying the
	// same rows would leave `| table` and `| count` choosing between them.
	Results []Run  `json:"results"`
	Failed  string `json:"failed,omitempty"`
	OK      bool   `json:"ok"`
}

// Text renders the verdict for a person. The per-target progress has already
// gone to stderr while the run was happening, so this is the last line only.
func (s Sweep) Text() string {
	var tb textbuf.Buffer
	if !s.OK {
		return tb.Str("Failed: ").Str(s.Failed).Byte('\n').String()
	}
	return tb.Int(int64(len(s.Results))).Str(" fuzz target(s) passed.\n").String()
}

// Sweeper runs fuzz targets. Each input is a field instead of a package variable.
// Tests therefore drive the command code and leave no state for the next caller.
type Sweeper struct {
	// Chain supplies the tags, the timeout and the environment.
	Chain gotoolchain.Toolchain
	// Root is the checkout discovery walks and the commands run in.
	Root string
	// Name and Package are what the caller named. Either one selects a single
	// run and neither consults discovery.
	Name    string
	Package string
	// FuzzTime overrides the per-target budget.
	FuzzTime string
	// Timeout overrides the hard per-target ceiling.
	Timeout string
	// Targets bypasses discovery. A test fills it.
	// The command leaves it nil so Plan walks the tree.
	Targets []Target
	// Exec runs one argv and answers its exit code. It defaults to forking go
	// test with the toolchain environment.
	Exec func(argv []string) int
	// Progress is where the run's log goes. It defaults to stderr.
	Progress io.Writer
	// Ctx cancels a running fuzzer. It defaults to a background context: each
	// target already carries its own -timeout, so the ceiling is per target
	// rather than on the sweep.
	Ctx context.Context
}

// context answers what a forked fuzzer runs under.
func (s *Sweeper) context() context.Context {
	if s.Ctx != nil {
		return s.Ctx
	}
	return context.Background()
}

// budget answers the per-target fuzz duration this run uses.
func (s *Sweeper) budget() string {
	if s.FuzzTime != "" {
		return s.FuzzTime
	}
	if s.named() {
		return NamedFuzzTime
	}
	return DefaultFuzzTime
}

// ceiling answers the hard per-target timeout this run uses.
func (s *Sweeper) ceiling() string {
	if s.Timeout != "" {
		return s.Timeout
	}
	return DefaultTimeout
}

// named reports whether the caller chose the target rather than discovery.
func (s *Sweeper) named() bool { return s.Name != "" || s.Package != "" }

// Plan returns every invocation for this run and performs none.
//
// A named run does not use discovery. Both arguments pass to Go unchanged.
// FUZZ is a Go regexp, and PKG can be a wildcard.
// The documented invocation is PKG=./internal/component/bgp/wireu/....
// Earlier Python code required exact equality, so that invocation exited 2 and matched nothing.
func (s *Sweeper) Plan() (Plan, error) {
	plan := Plan{FuzzTime: s.budget(), Timeout: s.ceiling(), Named: s.named()}

	if s.named() {
		plan.Runs = []Run{{
			Name:    s.orDefault(s.Name, DefaultName),
			Package: s.orDefault(s.Package, DefaultPackage),
			Argv:    s.namedArgv(),
		}}
		return plan, nil
	}

	targets, err := s.targets()
	if err != nil {
		return Plan{}, err
	}
	for _, target := range targets {
		plan.Runs = append(plan.Runs, Run{
			Name:    target.Name,
			Package: target.Package,
			Argv:    target.Command(s.Chain, plan.FuzzTime, plan.Timeout),
		})
	}
	return plan, nil
}

// orDefault answers value when it is set, and fallback when it is not.
func (s *Sweeper) orDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// namedArgv builds the invocation for a caller-selected target.
// Unlike Target.Command, it leaves the name unanchored because the caller can supply a regexp.
func (s *Sweeper) namedArgv() []string {
	var tb textbuf.Buffer
	name := tb.Str("-fuzz=").Str(s.orDefault(s.Name, DefaultName)).String()
	tb.Reset()
	budget := tb.Str("-fuzztime=").Str(s.budget()).String()
	tb.Reset()
	ceiling := tb.Str("-timeout=").Str(s.ceiling()).String()

	return s.Chain.GoTest(gotoolchain.TestOptions{}, name, budget, ceiling,
		s.orDefault(s.Package, DefaultPackage))
}

// targets answers what this run will fuzz: the caller's list when it set one,
// and the tree's otherwise.
func (s *Sweeper) targets() ([]Target, error) {
	if s.Targets != nil {
		return s.Targets, nil
	}
	return Discover(s.Root)
}

// Run runs the planned invocations and answers the sweep and its exit code.
//
// Run stops at the first failure, as the Make recipe did.
// Each `go test` was a separate recipe line, so a nonzero result stopped the target.
// Continuing would spend another quarter hour on fuzzing after a crash.
// It would also hide that crash under later output.
func (s *Sweeper) Run() (Sweep, int) {
	plan, err := s.Plan()
	if err != nil {
		leaction.ReportError(err)
		// Code 2 distinguishes an unreadable tree from a fuzzer crash.
		// Callers can continue to distinguish those results.
		return Sweep{}, 2
	}
	sweep := Sweep{FuzzTime: plan.FuzzTime, Timeout: plan.Timeout}

	if len(plan.Runs) == 0 {
		s.log("no `func Fuzz` found under internal/\n")
		sweep.Failed = "no fuzz target was found"
		return sweep, 1
	}

	if !plan.Named {
		var tb textbuf.Buffer
		s.log(tb.Str("Running ").Int(int64(len(plan.Runs))).Str(" fuzz target(s), ").
			Str(plan.FuzzTime).Str(" each...\n").String())
	}

	for _, run := range plan.Runs {
		var tb textbuf.Buffer
		s.log(tb.Str("==> ").Str(run.Name).Str(" (").Str(run.Package).Str(")\n").String())

		run.Code = s.exec(run.Argv)
		run.OK = run.Code == 0
		sweep.Results = append(sweep.Results, run)

		if !run.OK {
			tb.Reset()
			sweep.Failed = tb.Str(run.Name).Str(" in ").Str(run.Package).String()
			return sweep, run.Code
		}
	}

	sweep.OK = true
	return sweep, 0
}

// exec runs one invocation, defaulting to a real fork.
func (s *Sweeper) exec(argv []string) int {
	if s.Exec != nil {
		return s.Exec(argv)
	}
	return s.fork(argv)
}

// fork runs go test with the toolchain environment, letting its output reach
// the terminal as it happens. Both streams go to the progress writer: a fuzz
// run is watched rather than parsed, and the payload is what the caller reads.
func (s *Sweeper) fork(argv []string) int {
	cmd := exec.CommandContext(s.context(), argv[0], argv[1:]...) //nolint:gosec // the argv is built here from the toolchain and the target
	cmd.Dir = s.Root
	cmd.Env = s.Chain.Environment(gotoolchain.EnvOptions{Procs: true})
	cmd.Stdout = s.progress()
	cmd.Stderr = s.progress()

	if err := cmd.Run(); err != nil {
		if code := cmd.ProcessState.ExitCode(); code > 0 {
			return code
		}
		var tb textbuf.Buffer
		s.log(tb.Str("could not run go test: ").Err(err).Byte('\n').String())
		return 1
	}
	return 0
}

// progress answers where the run's log goes, defaulting to stderr.
func (s *Sweeper) progress() io.Writer {
	if s.Progress != nil {
		return s.Progress
	}
	return os.Stderr
}

// log writes one line of progress.
func (s *Sweeper) log(line string) {
	io.WriteString(s.progress(), line) //nolint:errcheck // progress output
}
