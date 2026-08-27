// Design: docs/architecture/testing/verify-freshness-scope.md -- native pre-commit stage execution
// Detail: ../gotoolchain/gotoolchain.go -- the Go command environment
// Related: ../changed/changed.go -- changed Go package population
//
// Package verifydeps runs the five Go-tool stages whose Make recipes used shell
// composition. Each stage is callable in-process, while its subject remains the
// Go toolchain, govulncheck, or the benchmark binaries it measures.
package verifydeps

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/changed"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/perf"
)

const (
	// Area is the gateless root that owns the verifier-only actions.
	Area = "verify-deps"

	VerbEvidenceVet     = "evidence-vet"
	VerbVulnerability   = "vulnerability"
	VerbUnitCached      = "unit-cached"
	VerbUnitRaceChanged = "unit-race-changed"
	VerbAlloc           = "alloc"

	defaultAllocBenchtime = "300x"
)

const (
	gateEvidenceVet     = "ze-evidence-vet"
	gateVulnerability   = "ze-dependency-vulnerability-check"
	gateUnitCached      = "ze-unit-test-cached"
	gateUnitRaceChanged = "ze-unit-test-race-changed"
	gateAlloc           = "ze-alloc-check"
)

var allocPackages = []string{
	"./internal/component/bgp/reactor/...",
	"./internal/component/plugin",
}

var allocBenchtimePattern = regexp.MustCompile(`^[1-9]\d*x$`)

// CommandPlan is one external command in a stage. Environment contains the
// complete child environment and is deliberately omitted from JSON because an
// inherited variable may contain a credential. Overrides records only the
// limits and target settings this package declares.
type CommandPlan struct {
	Name        string   `json:"name"`
	Directory   string   `json:"-"`
	Command     []string `json:"command"`
	Overrides   []string `json:"environment-overrides"`
	Environment []string `json:"-"`
}

// Plan is the complete command and population contract for one action.
type Plan struct {
	Verb         string             `json:"verb"`
	Gate         string             `json:"gate"`
	Packages     []string           `json:"packages,omitempty"`
	Changed      *changed.Selection `json:"changed,omitempty"`
	Benchtime    string             `json:"benchtime,omitempty"`
	BenchmarkLog string             `json:"benchmark-log,omitempty"`
	Commands     []CommandPlan      `json:"commands,omitempty"`
}

// ChildReport records one external process in execution order.
type ChildReport struct {
	Name      string   `json:"name"`
	Command   []string `json:"command"`
	Overrides []string `json:"environment-overrides"`
	Code      int      `json:"code"`
}

// AllocationVerdict records the worst observed sample for one registered
// benchmark and the ceiling that decided it.
type AllocationVerdict struct {
	Name        string `json:"name"`
	AllocsPerOp int    `json:"allocs-per-op"`
	Ceiling     int    `json:"ceiling"`
	Missing     bool   `json:"missing,omitempty"`
	Passed      bool   `json:"passed"`
	Message     string `json:"message,omitempty"`
}

// Report is the structured answer for one stage. Children remain ordered, and
// Code is the first child failure or the native allocation verdict.
type Report struct {
	Verb         string              `json:"verb"`
	Gate         string              `json:"gate"`
	Packages     []string            `json:"packages,omitempty"`
	Changed      *changed.Selection  `json:"changed,omitempty"`
	Benchtime    string              `json:"benchtime,omitempty"`
	BenchmarkLog string              `json:"benchmark-log,omitempty"`
	Children     []ChildReport       `json:"children,omitempty"`
	Allocations  []AllocationVerdict `json:"allocations,omitempty"`
	Code         int                 `json:"code"`
	Error        string              `json:"error,omitempty"`
}

type commandExecutor func(context.Context, CommandPlan, io.Writer) (string, ChildReport)

type dependencies struct {
	toolchain func(string) (gotoolchain.Toolchain, error)
	execute   commandExecutor
	module    func(string) (string, error)
	lookPath  func(string) (string, error)
	getenv    func(string) string
	ceilings  func() map[string]int
}

func productionDependencies() dependencies {
	return dependencies{
		toolchain: gotoolchain.New,
		module:    readModulePath,
		execute:   executeCommand,
		lookPath:  exec.LookPath,
		getenv:    os.Getenv,
		ceilings:  allocationCeilings,
	}
}

// PlanFor derives the exact population, argv, and declared environment for one
// action. Population discovery is context-bound and fails closed.
func PlanFor(ctx context.Context, root, verb string) (Plan, error) {
	plan, _, code, err := planFor(ctx, root, verb, productionDependencies())
	if err != nil {
		return plan, fmt.Errorf("plan %s (code %d): %w", verb, code, err)
	}
	return plan, nil
}

// Run executes one action and returns its structured report and exact verdict.
func Run(ctx context.Context, root, verb string) (Report, int) {
	return run(ctx, root, verb, productionDependencies())
}

func run(ctx context.Context, root, verb string, deps dependencies) (Report, int) {
	plan, discovery, code, err := planFor(ctx, root, verb, deps)
	report := reportFromPlan(plan)
	report.Children = append(report.Children, discovery...)
	if err != nil {
		report.Code = code
		report.Error = err.Error()
		return report, report.Code
	}

	switch verb {
	case VerbEvidenceVet:
		gaterun.Note("Vetting evidence scripts (GOOS=linux)...")
	case VerbVulnerability:
		gaterun.Note("Running govulncheck (SCA: module deps vs vuln.go.dev)...")
	case VerbUnitCached:
		return runUnitCached(ctx, plan, report, deps.execute)
	case VerbUnitRaceChanged:
		return runUnitRaceChanged(ctx, plan, report, deps.execute)
	case VerbAlloc:
		return runAlloc(ctx, plan, report, deps)
	}
	return runCommands(ctx, plan, report, deps.execute)
}

func runUnitCached(ctx context.Context, plan Plan, report Report, execute commandExecutor) (Report, int) {
	gaterun.Note("Unit tests: full pass (cacheable, no -race)...")
	_, child := execute(ctx, plan.Commands[0], os.Stdout)
	report.Children = append(report.Children, child)
	if child.Code != 0 {
		report.Code = child.Code
		return report, report.Code
	}

	gaterun.Note("Unit tests: bare ze_core compile-out checks...")
	_, child = execute(ctx, plan.Commands[1], os.Stdout)
	report.Children = append(report.Children, child)
	report.Code = child.Code
	return report, report.Code
}

func runUnitRaceChanged(ctx context.Context, plan Plan, report Report, execute commandExecutor) (Report, int) {
	if plan.Changed == nil || plan.Changed.Empty() {
		gaterun.Note("No changed .go files -- skipping changed-group pass")
		return report, 0
	}

	var text textbuf.Buffer
	gaterun.Note(text.Str("Unit tests: changed groups (race-instrumented): ").
		Join(plan.Packages, " ").Slice())
	_, child := execute(ctx, plan.Commands[0], os.Stdout)
	report.Children = append(report.Children, child)
	if child.Code != 0 {
		report.Code = child.Code
		return report, report.Code
	}

	gaterun.Note("Unit tests: bare ze_core compile-out checks (race-instrumented)...")
	_, child = execute(ctx, plan.Commands[1], os.Stdout)
	report.Children = append(report.Children, child)
	report.Code = child.Code
	return report, report.Code
}

func runAlloc(ctx context.Context, plan Plan, report Report, deps dependencies) (Report, int) {
	var text textbuf.Buffer
	if err := os.MkdirAll(filepath.Dir(plan.BenchmarkLog), 0o750); err != nil {
		report.Code = 1
		report.Error = text.Str("create allocation benchmark log directory: ").Err(err).String()
		return report, report.Code
	}
	logFile, err := os.Create(plan.BenchmarkLog)
	if err != nil {
		report.Code = 1
		report.Error = text.Str("create allocation benchmark log: ").Err(err).String()
		return report, report.Code
	}

	gaterun.Note("Running hot-path benchmarks (-benchmem) for the alloc-ceiling gate...")
	var captured bytes.Buffer
	writer := io.MultiWriter(os.Stdout, logFile, &captured)
	_, child := deps.execute(ctx, plan.Commands[0], writer)
	report.Children = append(report.Children, child)
	closeErr := logFile.Close()
	if child.Code != 0 {
		report.Code = child.Code
		return report, report.Code
	}
	if closeErr != nil {
		report.Code = 1
		report.Error = text.Str("close allocation benchmark log: ").Err(closeErr).String()
		return report, report.Code
	}

	gaterun.Note("Enforcing per-benchmark allocs/op ceilings (perf.AllocCeilings)...")
	verdicts, err := allocationVerdicts(captured.String(), deps.ceilings())
	if err != nil {
		report.Code = 1
		report.Error = err.Error()
		return report, report.Code
	}
	report.Allocations = verdicts
	for _, verdict := range verdicts {
		if verdict.Passed {
			continue
		}
		gaterun.Note(text.Reset().Str("alloc gate: ").Str(verdict.Message).Slice())
		report.Code = 1
	}
	return report, report.Code
}

func runCommands(ctx context.Context, plan Plan, report Report, execute commandExecutor) (Report, int) {
	for _, command := range plan.Commands {
		_, child := execute(ctx, command, os.Stdout)
		report.Children = append(report.Children, child)
		if child.Code != 0 {
			report.Code = child.Code
			return report, report.Code
		}
	}
	return report, 0
}

func reportFromPlan(plan Plan) Report {
	return Report{
		Verb:         plan.Verb,
		Gate:         plan.Gate,
		Packages:     slices.Clone(plan.Packages),
		Changed:      cloneSelection(plan.Changed),
		Benchtime:    plan.Benchtime,
		BenchmarkLog: plan.BenchmarkLog,
	}
}

func planFor(ctx context.Context, root, verb string, deps dependencies) (Plan, []ChildReport, int, error) {
	gate, err := gateForVerb(verb)
	plan := Plan{Verb: verb, Gate: gate}
	if err != nil {
		return plan, nil, 2, err
	}
	if root == "" {
		return plan, nil, 1, errors.New("verify dependency checkout root is empty")
	}
	chain, err := deps.toolchain(root)
	if err != nil {
		return plan, nil, 1, fmt.Errorf("derive verifier Go toolchain: %w", err)
	}

	switch verb {
	case VerbEvidenceVet:
		return planEvidence(root, chain, plan)
	case VerbVulnerability:
		return planVulnerability(chain, plan, deps.lookPath)
	case VerbUnitCached:
		return planUnitCached(ctx, root, chain, plan, deps)
	case VerbUnitRaceChanged:
		return planUnitRaceChanged(ctx, root, chain, plan, deps)
	case VerbAlloc:
		return planAlloc(root, chain, plan, deps.getenv)
	default:
		return plan, nil, 2, fmt.Errorf("unknown %s action %q", Area, verb)
	}
}

func planEvidence(root string, chain gotoolchain.Toolchain, plan Plan) (Plan, []ChildReport, int, error) {
	const pkg = "./scripts/evidence/..."
	if err := requireDirectory(root, "scripts/evidence"); err != nil {
		return plan, nil, 1, err
	}
	plan.Packages = []string{pkg}
	plan.Commands = []CommandPlan{commandPlan(
		gateEvidenceVet,
		[]string{"go", "vet", pkg},
		chain,
		gotoolchain.EnvOptions{GOOS: "linux"},
	)}
	return plan, nil, 0, nil
}

func planVulnerability(chain gotoolchain.Toolchain, plan Plan, lookPath func(string) (string, error)) (Plan, []ChildReport, int, error) {
	tool, err := lookPath("govulncheck")
	if err != nil {
		return plan, nil, gaterun.CannotStart, fmt.Errorf("resolve installed govulncheck: %w", err)
	}
	plan.Packages = []string{"./..."}
	plan.Commands = []CommandPlan{commandPlan(
		gateVulnerability,
		[]string{tool, "./..."},
		chain,
		gotoolchain.EnvOptions{GOOS: "linux", GOARCH: "amd64"},
	)}
	return plan, nil, 0, nil
}

func planUnitCached(ctx context.Context, root string, chain gotoolchain.Toolchain, plan Plan, deps dependencies) (Plan, []ChildReport, int, error) {
	packages, children, code, err := allPackages(ctx, root, chain, deps)
	if err != nil {
		return plan, children, code, err
	}
	plan.Packages = packages
	plan.Commands = []CommandPlan{
		commandPlan(planName(gateUnitCached, "full"), chain.GoTest(gotoolchain.TestOptions{}, packages...), chain, gotoolchain.EnvOptions{Procs: true}),
		commandPlan(planName(gateUnitCached, "core"), chain.GoTest(gotoolchain.TestOptions{Core: true}, "./cmd/ze/hub"), chain, gotoolchain.EnvOptions{Procs: true}),
	}
	return plan, children, 0, nil
}

func planUnitRaceChanged(ctx context.Context, root string, chain gotoolchain.Toolchain, plan Plan, deps dependencies) (Plan, []ChildReport, int, error) {
	var children []ChildReport
	selection, code, err := changedSelection(ctx, root, chain, deps.execute, &children)
	if err != nil {
		return plan, children, code, fmt.Errorf("derive changed race-test packages: %w", err)
	}
	plan.Changed = &selection
	for _, group := range selection.Groups {
		plan.Packages = append(plan.Packages, group.Pattern)
	}
	plan.Packages = append(plan.Packages, selection.Rest...)
	if selection.Empty() {
		return plan, children, 0, nil
	}
	options := gotoolchain.EnvOptions{CGO: true, Procs: true}
	plan.Commands = []CommandPlan{
		commandPlan(planName(gateUnitRaceChanged, "changed"), chain.GoTest(gotoolchain.TestOptions{Race: true}, plan.Packages...), chain, options),
		commandPlan(planName(gateUnitRaceChanged, "core"), chain.GoTest(gotoolchain.TestOptions{Core: true, Race: true}, "./cmd/ze/hub"), chain, options),
	}
	return plan, children, 0, nil
}

func planAlloc(root string, chain gotoolchain.Toolchain, plan Plan, getenv func(string) string) (Plan, []ChildReport, int, error) {
	for _, pkg := range allocPackages {
		relative := strings.TrimSuffix(strings.TrimPrefix(pkg, "./"), "/...")
		if err := requireDirectory(root, relative); err != nil {
			return plan, nil, 1, err
		}
	}
	benchtime := strings.TrimSpace(getenv("ALLOC_GATE_BENCHTIME"))
	if benchtime == "" {
		benchtime = defaultAllocBenchtime
	}
	if !allocBenchtimePattern.MatchString(benchtime) {
		return plan, nil, 1, fmt.Errorf("allocation benchtime %q is not a positive count", benchtime)
	}
	plan.Packages = slices.Clone(allocPackages)
	plan.Benchtime = benchtime
	plan.BenchmarkLog = filepath.Join(root, "tmp", "verify", "alloc-gate-bench.txt")
	var text textbuf.Buffer
	args := make([]string, 0, 6+len(plan.Packages))
	args = append(args, "-run", "^$", "-bench", ".", "-benchmem",
		text.Str("-benchtime=").Str(benchtime).String())
	args = append(args, plan.Packages...)
	plan.Commands = []CommandPlan{commandPlan(
		planName(gateAlloc, "benchmarks"),
		chain.GoTest(gotoolchain.TestOptions{}, args...),
		chain,
		gotoolchain.EnvOptions{Procs: true},
	)}
	return plan, nil, 0, nil
}

func allPackages(ctx context.Context, root string, chain gotoolchain.Toolchain, deps dependencies) ([]string, []ChildReport, int, error) {
	step := commandPlan("go-package-population", []string{"go", "list", "./..."}, chain, gotoolchain.EnvOptions{})
	output, child := deps.execute(ctx, step, nil)
	children := []ChildReport{child}
	if child.Code != 0 {
		return nil, children, child.Code, fmt.Errorf("go list package population exited %d", child.Code)
	}

	var exclude *regexp.Regexp
	rawExclude := strings.TrimSpace(deps.getenv("ZE_PACKAGES_EXCLUDE"))
	if rawExclude != "" {
		var err error
		exclude, err = regexp.Compile(rawExclude)
		if err != nil {
			return nil, children, 1, fmt.Errorf("compile ZE_PACKAGES_EXCLUDE: %w", err)
		}
	}
	module, err := deps.module(root)
	if err != nil {
		return nil, children, 1, fmt.Errorf("read module path: %w", err)
	}
	packages := make([]string, 0, strings.Count(output, "\n")+1)
	for line := range strings.SplitSeq(output, "\n") {
		pkg := strings.TrimSpace(line)
		if pkg == "" || pkg == module {
			continue
		}
		if exclude != nil && exclude.MatchString(pkg) {
			continue
		}
		packages = append(packages, pkg)
	}
	if len(packages) == 0 {
		return nil, children, 1, errors.New("go list returned no testable package population")
	}
	return packages, children, 0, nil
}

func changedSelection(ctx context.Context, root string, chain gotoolchain.Toolchain, execute commandExecutor, children *[]ChildReport) (changed.Selection, int, error) {
	var firstCode int
	selector := changed.Selector{
		Root: root,
		Run: func(_ string, argv []string) (string, error) {
			step := commandPlan("changed-population", argv, chain, gotoolchain.EnvOptions{})
			output, child := execute(ctx, step, nil)
			*children = append(*children, child)
			if child.Code != 0 {
				if firstCode == 0 {
					firstCode = child.Code
				}
				return "", fmt.Errorf("%s exited %d", strings.Join(argv, " "), child.Code)
			}
			return output, nil
		},
	}
	selection, err := selector.Select()
	if err != nil {
		if firstCode == 0 {
			firstCode = 1
		}
		return changed.Selection{}, firstCode, err
	}
	return selection, 0, nil
}

func commandPlan(name string, argv []string, chain gotoolchain.Toolchain, options gotoolchain.EnvOptions) CommandPlan {
	return CommandPlan{
		Name:        name,
		Directory:   chain.Root,
		Command:     slices.Clone(argv),
		Overrides:   chain.Overrides(options),
		Environment: chain.Environment(options),
	}
}

func executeCommand(ctx context.Context, plan CommandPlan, stdout io.Writer) (string, ChildReport) {
	child := ChildReport{
		Name:      plan.Name,
		Command:   slices.Clone(plan.Command),
		Overrides: slices.Clone(plan.Overrides),
		Code:      gaterun.CannotStart,
	}
	if len(plan.Command) == 0 {
		gaterun.Note("error: a verifier stage declared no command to run")
		return "", child
	}

	var captured bytes.Buffer
	if stdout == nil {
		stdout = &captured
	}
	command := exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...) // #nosec G204 -- plan commands come from this package's closed action table.
	command.Dir = plan.Directory
	command.Env = plan.Environment
	command.Stdin = os.Stdin
	command.Stdout = stdout
	command.Stderr = os.Stderr

	err := command.Run()
	if err == nil {
		child.Code = 0
		return captured.String(), child
	}
	if command.ProcessState == nil {
		var text textbuf.Buffer
		gaterun.Note(text.Str("cannot run ").Str(plan.Command[0]).Str(": ").Err(err).Slice())
		return captured.String(), child
	}
	child.Code = gaterun.ExitCode(err)
	return captured.String(), child
}

func readModulePath(root string) (string, error) {
	checkout, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	raw, readErr := checkout.ReadFile("go.mod")
	if err := errors.Join(readErr, checkout.Close()); err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", errors.New("go.mod declares no module path")
}

func requireDirectory(root, relative string) error {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return fmt.Errorf("read required package %s: %w", relative, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("required package %s is not a directory", relative)
	}
	return nil
}

func gateForVerb(verb string) (string, error) {
	switch verb {
	case VerbEvidenceVet:
		return gateEvidenceVet, nil
	case VerbVulnerability:
		return gateVulnerability, nil
	case VerbUnitCached:
		return gateUnitCached, nil
	case VerbUnitRaceChanged:
		return gateUnitRaceChanged, nil
	case VerbAlloc:
		return gateAlloc, nil
	default:
		return "", fmt.Errorf("unknown %s action %q", Area, verb)
	}
}

func cloneSelection(selection *changed.Selection) *changed.Selection {
	if selection == nil {
		return nil
	}
	clone := &changed.Selection{Rest: slices.Clone(selection.Rest)}
	clone.Groups = slices.Clone(selection.Groups)
	return clone
}

func allocationCeilings() map[string]int {
	ceilings := make(map[string]int, len(perf.AllocCeilings))
	maps.Copy(ceilings, perf.AllocCeilings)
	return ceilings
}

func allocationVerdicts(text string, ceilings map[string]int) ([]AllocationVerdict, error) {
	if len(ceilings) == 0 {
		return nil, errors.New("allocation ceiling registry is empty")
	}
	worst := make(map[string]int, len(ceilings))
	seen := make(map[string]bool, len(ceilings))
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		allocs, ok := allocsFromFields(fields)
		if !ok {
			continue
		}
		name := stripProcSuffix(fields[0])
		if current, present := worst[name]; !present || allocs > current {
			worst[name] = allocs
		}
		seen[name] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse allocation benchmark output: %w", err)
	}

	names := make([]string, 0, len(ceilings))
	for name := range ceilings {
		names = append(names, name)
	}
	sort.Strings(names)
	verdicts := make([]AllocationVerdict, 0, len(names))
	var message textbuf.Buffer
	for _, name := range names {
		ceiling := ceilings[name]
		verdict := AllocationVerdict{Name: name, AllocsPerOp: worst[name], Ceiling: ceiling}
		switch {
		case !seen[name]:
			verdict.Missing = true
			verdict.Message = message.Reset().Str(name).
				Str(": absent from benchmark output (expected allocs/op <= ").Int(int64(ceiling)).
				Str("; did the benchmark build and run?)").String()
		case worst[name] > ceiling:
			verdict.Message = message.Reset().Str(name).Str(": ").Int(int64(worst[name])).
				Str(" allocs/op exceeds ceiling ").Int(int64(ceiling)).String()
		default:
			verdict.Passed = true
		}
		verdicts = append(verdicts, verdict)
	}
	return verdicts, nil
}

func planName(gate, suffix string) string {
	var text textbuf.Buffer
	return text.Str(gate).Byte(':').Str(suffix).String()
}

func allocsFromFields(fields []string) (int, bool) {
	for index, field := range fields {
		if field != "allocs/op" || index == 0 {
			continue
		}
		allocs, err := strconv.Atoi(fields[index-1])
		if err != nil {
			return 0, false
		}
		return allocs, true
	}
	return 0, false
}

func stripProcSuffix(name string) string {
	index := strings.LastIndexByte(name, '-')
	if index <= 0 {
		return name
	}
	if _, err := strconv.Atoi(name[index+1:]); err != nil {
		return name
	}
	return name[:index]
}
