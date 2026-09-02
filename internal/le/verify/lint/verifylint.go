// Design: docs/contributing/testing.md -- lint coverage spans every selected Go build
// Detail: matrix.go -- the ordered host, integration, platform, capability, and personality builds
// Related: ../gotoolchain/gotoolchain.go -- the derived toolchain and resource ceilings
package verifylint

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/population"
)

const (
	configName  = ".golangci.yml"
	lintProgram = "golangci-lint"
	// allowSerial makes golangci-lint WAIT for its machine-wide lock. Without
	// it the child gives up after five seconds and exits "parallel golangci-lint
	// is running". Several sessions share this checkout, so a sibling lint made
	// every pass here exit that way with nothing linted, and the stage reported
	// findings it never collected.
	allowSerial    = "--allow-serial-runners"
	listProgram    = "go"
	trackedProgram = "git"
	taglessDir     = "tmp/lint-flavors"
	taglessName    = "notags-"
	cannotPlan     = 2
	listTemplate   = "{{$d:=.Dir}}{{range .GoFiles}}{{$d}}/{{.}}\n{{end}}{{range .CgoFiles}}{{$d}}/{{.}}\n{{end}}{{range .TestGoFiles}}{{$d}}/{{.}}\n{{end}}{{range .XTestGoFiles}}{{$d}}/{{.}}\n{{end}}"
)

var ignoreConstraint = regexp.MustCompile(`(^|[^A-Za-z0-9_])ignore([^A-Za-z0-9_]|$)`)

var residue = map[string]string{
	"examples/plugin/go/main.go": "a separate Go module (`module example/acme-monitor`), so `./...` cannot reach it and golangci-lint would need its own run in that directory. It has no tracked go.sum, so the module does not resolve without `go mod tidy`, and this repository vendors its dependencies rather than fetching them. Needs an owner decision on how the example module joins the build",
	"tools.go":                   "the tools.go idiom: a `//go:build tools` file whose imports are PROGRAMS, kept so `go mod tidy` pins the build tools. golangci-lint reports 'is a program, not an importable package' for each one, and so would any type checker. It stops being blind when the pins move to go.mod's `tool` directives, which is its own change",
}

// PassPlan is one direct golangci-lint invocation, or one deliberately skipped
// flavor whose files an earlier pass already selected.
type PassPlan struct {
	Name        string   `json:"name"`
	GOOS        string   `json:"goos,omitempty"`
	GOARCH      string   `json:"goarch,omitempty"`
	TagsAdded   []string `json:"tags-add,omitempty"`
	TagsDropped []string `json:"tags-drop,omitempty"`
	Packages    []string `json:"packages,omitempty"`
	Command     []string `json:"command,omitempty"`
	Environment []string `json:"-"`
	Skipped     bool     `json:"skipped,omitempty"`
}

// LintPlan is the complete ordered stage derived before any linter starts.
type LintPlan struct {
	Passes         []PassPlan          `json:"passes"`
	Coverage       population.Coverage `json:"coverage"`
	TaglessConfig  string              `json:"tagless-config,omitempty"`
	NeedsTagless   bool                `json:"-"`
	reportCoverage bool
	configContents []byte
}

// PassResult records one planned child and the code it returned.
type PassResult struct {
	PassPlan
	Code  int    `json:"code"`
	Error string `json:"error,omitempty"`
}

// Report is the structured result of the lint stage. Code is the first non-zero
// child status in execution order, with coverage last.
type Report struct {
	Passes   []PassResult        `json:"passes"`
	Coverage population.Coverage `json:"coverage"`
	Code     int                 `json:"code"`
	Error    string              `json:"error,omitempty"`
	// FailingPaths names the files this run's findings were about, so a red can
	// be charged to the commits that touch them (see failuregroup.go).
	FailingPaths []string `json:"failing-paths,omitempty"`
}

// Text returns the failure-group declaration for a red run, and nothing for a
// green one, because every child and the coverage proof have already streamed
// in producer order. Pipe renderers still receive Report.
//
// The declaration belongs HERE rather than on os.Stderr. The verify engine reads
// a stage's groups back out of the stage log, and that log holds what the action
// RETURNED (internal/le/verify/dispatch/dispatch.go, dispatch, which hands
// leroot.Run a capturing writer). A line written straight to the process's own
// stderr goes to the operator's terminal and never reaches the log, so the stage
// reads as declaring nothing.
func (r Report) Text() string {
	if r.Code == 0 {
		return ""
	}
	var out strings.Builder
	if err := declareLintFailureGroup(&out, r.FailingPaths); err != nil {
		return ""
	}

	return out.String()
}

type commandResult struct {
	stdout []byte
	stderr []byte
	code   int
	err    error
}

type runnerOps struct {
	lookPath func(string) (string, error)
	capture  func(context.Context, []string, string, []string) commandResult
	stream   func(context.Context, []string, string, []string, io.Writer) (int, error)
}

// Runner owns the checkout, current lint configuration, toolchain, and process
// seams for one complete run. It is not safe for concurrent use.
type Runner struct {
	ctx        context.Context
	root       string
	toolchain  gotoolchain.Toolchain
	configTags []string
	config     []byte
	ops        runnerOps
}

// NewRunner loads the current toolchain and lint configuration and refuses a
// missing required executable before any population query starts.
func NewRunner(ctx context.Context, root string) (*Runner, error) {
	if ctx == nil {
		return nil, errors.New("lint context is missing")
	}
	if root == "" {
		return nil, errors.New("lint checkout root is empty")
	}
	chain, err := gotoolchain.New(root)
	if err != nil {
		return nil, fmt.Errorf("derive lint toolchain: %w", err)
	}
	return newRunner(ctx, root, chain, realRunnerOps())
}

func newRunner(ctx context.Context, root string, chain gotoolchain.Toolchain, ops runnerOps) (*Runner, error) {
	if ctx == nil {
		return nil, errors.New("lint context is missing")
	}
	if root == "" {
		return nil, errors.New("lint checkout root is empty")
	}
	if ops.lookPath == nil || ops.capture == nil || ops.stream == nil {
		return nil, errors.New("lint process operations are incomplete")
	}
	for _, program := range []string{lintProgram, listProgram, trackedProgram} {
		if _, err := ops.lookPath(program); err != nil {
			return nil, fmt.Errorf("required lint tool %s is unavailable: %w", program, err)
		}
	}
	config, err := os.ReadFile(filepath.Join(root, configName)) //nolint:gosec // the verifier reads its checkout configuration
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configName, err)
	}
	tags, err := parseConfigTags(config)
	if err != nil {
		return nil, err
	}
	return &Runner{ctx: ctx, root: root, toolchain: chain, configTags: tags, config: config, ops: ops}, nil
}

func realRunnerOps() runnerOps {
	return runnerOps{
		lookPath: exec.LookPath,
		capture:  captureCommand,
		stream:   streamCommand,
	}
}

func captureCommand(ctx context.Context, argv []string, dir string, environment []string) commandResult {
	if len(argv) == 0 {
		return commandResult{code: gaterun.CannotStart, err: errors.New("no command to capture")}
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv is derived from the closed lint matrix
	cmd.Dir = dir
	cmd.Env = environment
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{
		stdout: stdout.Bytes(),
		stderr: stderr.Bytes(),
		code:   processCode(err),
		err:    commandStartError(err),
	}
}

// streamCommand runs one lint child, streaming its output to the operator
// unchanged. watch, when non-nil, additionally receives that output so the stage
// can learn which files its findings name; it never alters what is printed.
func streamCommand(ctx context.Context, argv []string, dir string, environment []string, watch io.Writer) (int, error) {
	if len(argv) == 0 {
		return gaterun.CannotStart, errors.New("no lint command to run")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // argv is derived from the closed lint matrix
	cmd.Dir = dir
	cmd.Env = environment
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, discardNil(watch))
	cmd.Stderr = io.MultiWriter(os.Stderr, discardNil(watch))
	err := cmd.Run()
	return processCode(err), commandStartError(err)
}

func processCode(err error) int {
	if err == nil {
		return 0
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return gaterun.ExitCode(exit)
	}
	return gaterun.CannotStart
}

func commandStartError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil
	}
	return err
}

// Plan derives the full-tree flavor package scope and tracked-file coverage
// answer.
func (r *Runner) Plan() (LintPlan, error) {
	return r.plan([]string{packageRoot}, true)
}

// planScope derives the same ordered matrix over the package patterns supplied
// by a scoped verify run. Coverage is meaningful only when that scope is
// exactly ./..., matching the former producer's explicit --scope ./... route.
func (r *Runner) planScope(patterns []string) (LintPlan, error) {
	reportCoverage := len(patterns) == 1 && patterns[0] == packageRoot
	return r.plan(patterns, reportCoverage)
}

func (r *Runner) plan(patterns []string, reportCoverage bool) (LintPlan, error) {
	features := make([]string, 0, len(r.configTags))
	for _, tag := range r.configTags {
		if tag != tagZeCore {
			features = append(features, tag)
		}
	}
	flavors := flavorMatrix(features)
	passes := make([]PassPlan, 0, len(basePasses())+len(flavors))
	if len(patterns) == 0 {
		for _, flavor := range basePasses() {
			passes = append(passes, r.passPlan(flavor, nil, true))
		}
		for _, flavor := range flavors {
			passes = append(passes, r.passPlan(flavor, nil, true))
		}
		return LintPlan{Passes: passes}, nil
	}

	seen := make(map[string]bool)
	for _, flavor := range basePasses() {
		packages, files, err := r.goList(flavor, patterns)
		if err != nil {
			return LintPlan{}, err
		}
		if len(packages) == 0 || len(files) == 0 {
			return LintPlan{}, fmt.Errorf("lint flavor %s selected an empty package population", flavor.Name)
		}
		for path := range files {
			seen[path] = true
		}
		passes = append(passes, r.passPlan(flavor, patterns, false))
	}

	for _, flavor := range flavors {
		packages, files, err := r.goList(flavor, patterns)
		if err != nil {
			return LintPlan{}, err
		}
		if len(packages) == 0 || len(files) == 0 {
			return LintPlan{}, fmt.Errorf("lint flavor %s selected an empty package population", flavor.Name)
		}
		scope := make([]string, 0, len(packages))
		for name, selected := range packages {
			for path := range selected {
				if !seen[path] {
					scope = append(scope, name)
					break
				}
			}
		}
		sort.Strings(scope)
		for path := range files {
			seen[path] = true
		}
		passes = append(passes, r.passPlan(flavor, scope, len(scope) == 0))
	}

	var coverage population.Coverage
	if reportCoverage {
		var err error
		coverage, err = r.coverage(seen)
		if err != nil {
			return LintPlan{}, err
		}
	}
	plan := LintPlan{
		Passes:         passes,
		Coverage:       coverage,
		TaglessConfig:  filepath.Join(r.root, filepath.FromSlash(taglessDir), taglessName+strconv.Itoa(os.Getpid())+".yml"),
		reportCoverage: reportCoverage,
		configContents: slices.Clone(r.config),
	}
	for index := range plan.Passes {
		if len(plan.Passes[index].TagsDropped) == 0 || plan.Passes[index].Skipped {
			continue
		}
		plan.NeedsTagless = true
		plan.Passes[index].Command = replaceConfigPath(plan.Passes[index].Command, plan.TaglessConfig)
	}
	return plan, nil
}

func (r *Runner) goList(flavor Flavor, patterns []string) (map[string]map[string]bool, map[string]bool, error) {
	tags := r.effectiveTags(flavor)
	argv := make([]string, 0, 7+len(patterns))
	argv = append(argv, "go", "list", "-e", "-tags", strings.Join(tags, " "), "-f", listTemplate)
	argv = append(argv, patterns...)
	environment := r.toolchain.Environment(gotoolchain.EnvOptions{
		MemLimit: true,
		GOOS:     flavor.GOOS,
		GOARCH:   flavor.GOARCH,
	})
	result := r.ops.capture(r.ctx, argv, r.root, environment)
	if result.err != nil {
		return nil, nil, fmt.Errorf("run go list for lint flavor %s: %w", flavor.Name, result.err)
	}
	if len(bytes.TrimSpace(result.stdout)) == 0 {
		detail := strings.TrimSpace(string(result.stderr))
		if detail == "" {
			detail = "no output"
		}
		return nil, nil, fmt.Errorf("go list for lint flavor %s returned code %d with %s", flavor.Name, result.code, detail)
	}

	root, err := filepath.Abs(r.root)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve lint root: %w", err)
	}
	var pathText textbuf.Buffer
	prefix := pathText.Str(filepath.Clean(root)).Byte(byte(filepath.Separator)).String()
	parentPrefix := pathText.Reset().Str("..").Byte(byte(filepath.Separator)).String()
	packages := make(map[string]map[string]bool)
	files := make(map[string]bool)
	for line := range strings.SplitSeq(string(result.stdout), "\n") {
		path := filepath.Clean(strings.TrimSpace(line))
		if path == "." || !strings.HasPrefix(path, prefix) {
			continue
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil || relative == "." || strings.HasPrefix(relative, parentPrefix) {
			continue
		}
		relative = filepath.ToSlash(relative)
		directory := filepath.ToSlash(filepath.Dir(relative))
		if directory == "." {
			directory = ""
		}
		if packages[directory] == nil {
			packages[directory] = make(map[string]bool)
		}
		packages[directory][relative] = true
		files[relative] = true
	}
	if len(packages) == 0 {
		return nil, nil, fmt.Errorf("go list for lint flavor %s returned no files inside the checkout", flavor.Name)
	}
	return packages, files, nil
}

func (r *Runner) effectiveTags(flavor Flavor) []string {
	dropped := make(map[string]bool, len(flavor.Without))
	for _, tag := range flavor.Without {
		dropped[tag] = true
	}
	tags := make([]string, 0, len(r.configTags)+len(flavor.Tags))
	for _, tag := range r.configTags {
		if !dropped[tag] {
			tags = append(tags, tag)
		}
	}
	return append(tags, flavor.Tags...)
}

func (r *Runner) passPlan(flavor Flavor, scope []string, skipped bool) PassPlan {
	plan := PassPlan{
		Name:        flavor.Name,
		GOOS:        flavor.GOOS,
		GOARCH:      flavor.GOARCH,
		TagsAdded:   slices.Clone(flavor.Tags),
		TagsDropped: slices.Clone(flavor.Without),
		Skipped:     skipped,
		Environment: r.toolchain.Environment(gotoolchain.EnvOptions{
			MemLimit: true,
			GOOS:     flavor.GOOS,
			GOARCH:   flavor.GOARCH,
		}),
	}
	if skipped {
		return plan
	}
	plan.Packages = make([]string, len(scope))
	var packageText textbuf.Buffer
	for index, name := range scope {
		if strings.HasPrefix(name, "./") {
			plan.Packages[index] = name
		} else {
			plan.Packages[index] = packageText.Reset().Str("./").Str(name).String()
		}
	}
	plan.Command = []string{lintProgram, "run", "-j", strconv.Itoa(r.toolchain.Procs), allowSerial}
	if len(flavor.Without) != 0 {
		plan.Command = append(plan.Command, "-c", "")
	}
	tags := flavor.Tags
	if len(flavor.Without) != 0 {
		tags = r.effectiveTags(flavor)
	}
	if len(tags) != 0 {
		plan.Command = append(plan.Command, "--build-tags", strings.Join(tags, ","))
	}
	plan.Command = append(plan.Command, plan.Packages...)
	return plan
}

func replaceConfigPath(argv []string, path string) []string {
	answer := slices.Clone(argv)
	for index := 0; index+1 < len(answer); index++ {
		if answer[index] == "-c" {
			answer[index+1] = path
			break
		}
	}
	return answer
}

func (r *Runner) coverage(seen map[string]bool) (population.Coverage, error) {
	tracked, err := r.population()
	if err != nil {
		return population.Coverage{}, err
	}
	claim := population.Claim{
		Subject:         "lint tracked Go",
		Population:      tracked,
		Walked:          seen,
		Excused:         residue,
		UnexcusedReason: "NOT COVERED BY ANY PASS",
	}
	return claim.Assess()
}

func (r *Runner) population() (map[string]bool, error) {
	argv := []string{"git", "ls-files", "-z", "--", "*.go"}
	result := r.ops.capture(r.ctx, argv, r.root, r.toolchain.Environment(gotoolchain.EnvOptions{MemLimit: true}))
	if result.err != nil {
		return nil, fmt.Errorf("enumerate tracked Go files: %w", result.err)
	}
	if result.code != 0 {
		return nil, fmt.Errorf("enumerate tracked Go files: git exited %d: %s", result.code, strings.TrimSpace(string(result.stderr)))
	}
	paths := bytes.Split(result.stdout, []byte{0})
	tracked := make(map[string]bool, len(paths))
	for _, raw := range paths {
		path := filepath.ToSlash(string(raw))
		if path == "" || strings.HasPrefix(path, "vendor/") || strings.HasPrefix(path, "gokrazy/modcache/") {
			continue
		}
		// A testdata directory is invisible to the Go tool by convention, so
		// `go list` never reaches it and no flavor can ever select it. It also
		// holds files that are deliberately unparseable, which a pass that did
		// load them would die on. Counting it as blind asks for a flavor that
		// cannot exist. Every other tree walker in internal/le skips the same
		// segment by the same name.
		if testdataSegment(path) {
			continue
		}
		full := filepath.Join(r.root, filepath.FromSlash(path))
		if _, err := os.Stat(full); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read tracked Go file %s: %w", path, err)
		}
		constraint, err := buildConstraint(full)
		if err != nil {
			return nil, fmt.Errorf("read build constraint for %s: %w", path, err)
		}
		if ignoreConstraint.MatchString(constraint) {
			continue
		}
		tracked[path] = true
	}
	return tracked, nil
}

// testdataSegment reports whether a slash-separated repository path has a
// testdata directory anywhere along it. The path is tracked-file relative, so
// the walk is bounded by the depth git reported.
func testdataSegment(path string) bool {
	return slices.Contains(strings.Split(path, "/"), "testdata")
}

func buildConstraint(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // path belongs to the tracked checkout population
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // a read-only file has no buffered writes to lose
	head, err := io.ReadAll(io.LimitReader(file, 4096))
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(head), "\n") {
		if constraint, ok := strings.CutPrefix(line, "//go:build "); ok {
			return strings.TrimSpace(constraint), nil
		}
	}
	return "", nil
}

func parseConfigTags(config []byte) ([]string, error) {
	tags := make([]string, 0)
	inside := false
	hasCore := false
	scanner := bufio.NewScanner(bytes.NewReader(config))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "  build-tags:" {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if !strings.HasPrefix(line, "    - ") {
			break
		}
		tag := strings.TrimSpace(strings.TrimPrefix(line, "    - "))
		if tag == "" {
			return nil, errors.New(".golangci.yml contains an empty build tag")
		}
		if tag == tagZeCore {
			hasCore = true
		}
		tags = append(tags, tag)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse .golangci.yml build tags: %w", err)
	}
	if len(tags) == 0 {
		return nil, errors.New(".golangci.yml declares no build tags")
	}
	if !hasCore {
		return nil, errors.New(".golangci.yml build tags omit ze_core")
	}
	return tags, nil
}

func deriveTaglessConfig(config []byte) ([]byte, error) {
	var output strings.Builder
	dropping := false
	foundRun := false
	scanner := bufio.NewScanner(bytes.NewReader(config))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "  build-tags:" {
			dropping = true
			continue
		}
		if dropping && strings.HasPrefix(line, "    - ") {
			continue
		}
		dropping = false
		output.WriteString(line)
		output.WriteByte('\n')
		if line == "run:" {
			foundRun = true
			output.WriteString("  relative-path-mode: gitroot\n")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("derive tagless lint config: %w", err)
	}
	if !foundRun {
		return nil, errors.New(".golangci.yml has no run section for relative-path-mode")
	}
	return []byte(output.String()), nil
}

// Run executes the full-tree matrix and returns the first failure code. Empty
// flavor scopes are recorded as skipped and never handed to golangci-lint.
func (r *Runner) Run() (Report, int) {
	return r.runPlan(r.Plan())
}

// runScope executes the matrix over exactly the supplied package patterns.
func (r *Runner) runScope(patterns []string) (Report, int) {
	return r.runPlan(r.planScope(patterns))
}

func (r *Runner) runPlan(plan LintPlan, err error) (Report, int) {
	if err != nil {
		report := Report{Code: cannotPlan, Error: err.Error()}
		if writeErr := writeLintf(os.Stderr, "lint: %v\n", err); writeErr != nil {
			recordLintOutputError(&report, "write planning diagnostic", writeErr)
		}
		return report, report.Code
	}
	return r.execute(plan)
}

func (r *Runner) execute(plan LintPlan) (Report, int) {

	// Collects the files the children's findings name, so a red can be charged
	// to the commits that touch them rather than to every commit in the checkout.
	collector := newPathCollector()
	report := Report{Passes: make([]PassResult, 0, len(plan.Passes)), Coverage: plan.Coverage}
	reportCoverage := plan.reportCoverage ||
		plan.Coverage.Population != 0 ||
		plan.Coverage.Walked != 0 ||
		len(plan.Coverage.Blind) != 0 ||
		len(plan.Coverage.Unexcused) != 0 ||
		len(plan.Coverage.Healed) != 0 ||
		plan.Coverage.Code != 0
	var errorText textbuf.Buffer
	if plan.NeedsTagless {
		derived, err := deriveTaglessConfig(plan.configContents)
		if err != nil {
			report.Code, report.Error = cannotPlan, err.Error()
			if writeErr := writeLintf(os.Stderr, "lint: %v\n", err); writeErr != nil {
				recordLintOutputError(&report, "write tagless-config diagnostic", writeErr)
			}
			return report, report.Code
		}
		if err := os.MkdirAll(filepath.Dir(plan.TaglessConfig), 0o750); err != nil {
			report.Code, report.Error = cannotPlan, errorText.Reset().Str("create lint config directory: ").Err(err).String()
			if writeErr := writeLintf(os.Stderr, "lint: %s\n", report.Error); writeErr != nil {
				recordLintOutputError(&report, "write tagless-directory diagnostic", writeErr)
			}
			return report, report.Code
		}
		if err := os.WriteFile(plan.TaglessConfig, derived, 0o600); err != nil {
			report.Code, report.Error = cannotPlan, errorText.Reset().Str("write tagless lint config: ").Err(err).String()
			if writeErr := writeLintf(os.Stderr, "lint: %s\n", report.Error); writeErr != nil {
				recordLintOutputError(&report, "write tagless-config diagnostic", writeErr)
			}
			return report, report.Code
		}
	}

	var flavorFailures []string
	outputReady := true
	for index := range plan.Passes {
		pass := &plan.Passes[index]
		result := PassResult{PassPlan: *pass}
		if pass.Skipped {
			report.Passes = append(report.Passes, result)
			continue
		}
		if err := announcePass(pass); err != nil {
			result.Code = cannotPlan
			result.Error = errorText.Reset().Str("write lint progress: ").Err(err).String()
			report.Passes = append(report.Passes, result)
			recordLintOutputError(&report, "write lint progress", err)
			outputReady = false
			break
		}
		code, runErr := r.ops.stream(r.ctx, pass.Command, r.root, pass.Environment, collector)
		result.Code = code
		if code != 0 {
			if report.Code == 0 {
				report.Code = code
			}
			if pass.Name != passHost && pass.Name != passLinuxIntegration {
				flavorFailures = append(flavorFailures, pass.Name)
			}
		}
		if runErr != nil {
			result.Error = runErr.Error()
			if writeErr := writeLintf(os.Stderr, "lint: cannot run %s: %v\n", pass.Name, runErr); writeErr != nil {
				recordLintOutputError(&report, "write child-start diagnostic", writeErr)
				outputReady = false
			}
		}
		report.Passes = append(report.Passes, result)
		if !outputReady {
			break
		}
	}

	if outputReady && reportCoverage {
		if err := renderCoverage(plan.Coverage); err != nil {
			recordLintOutputError(&report, "write lint coverage", err)
			outputReady = false
		}
	}
	if reportCoverage && plan.Coverage.Code != 0 && report.Code == 0 {
		report.Code = plan.Coverage.Code
	}
	if outputReady && len(flavorFailures) != 0 {
		if err := writeLintf(os.Stderr, "lint_flavors: failed: %s\n", strings.Join(flavorFailures, ", ")); err != nil {
			recordLintOutputError(&report, "write flavor-failure diagnostic", err)
			outputReady = false
		}
	}
	report.FailingPaths = collector.paths()
	if plan.NeedsTagless {
		if err := os.Remove(plan.TaglessConfig); err != nil {
			report.Error = errorText.Reset().Str("remove tagless lint config: ").Err(err).String()
			if report.Code == 0 {
				report.Code = cannotPlan
			}
			if outputReady {
				if writeErr := writeLintf(os.Stderr, "lint: %s\n", report.Error); writeErr != nil {
					recordLintOutputError(&report, "write tagless-cleanup diagnostic", writeErr)
				}
			}
		}
	}
	return report, report.Code
}

func announcePass(pass *PassPlan) error {
	switch pass.Name {
	case passHost:
		return writeLintf(os.Stdout, "Running ze linter...\n")
	case passLinuxIntegration:
		return writeLintf(os.Stdout, "Running ze linter (GOOS=linux, integration tag)...\n")
	default:
		goos := pass.GOOS
		if goos == "" {
			goos = goosHost
		}
		var targetText textbuf.Buffer
		targetText.Str("GOOS=").Str(goos)
		if pass.GOARCH != "" {
			targetText.Str(" GOARCH=").Str(pass.GOARCH)
		}
		target := targetText.String()
		added := strings.Join(pass.TagsAdded, ",")
		if added == "" {
			added = "none"
		}
		removed := strings.Join(pass.TagsDropped, ",")
		if removed == "" {
			removed = "none"
		}
		return writeLintf(
			os.Stdout,
			"Running ze linter (%s: %s, tags add %s drop %s, %d packages)...\n",
			pass.Name, target, added, removed, len(pass.Packages),
		)
	}
}

func renderCoverage(coverage population.Coverage) error {
	for _, blind := range coverage.Blind {
		if err := writeLintf(os.Stdout, "  %s: %s\n", blind.Member, blind.Reason); err != nil {
			return err
		}
	}
	if len(coverage.Unexcused) != 0 {
		return writeLintf(os.Stderr, "lint_flavors: %d tracked Go file(s) are linted by nothing. Add the flavor that selects them, or state the reason in RESIDUE (internal/le/verify/lint/verifylint.go).\n", len(coverage.Unexcused))
	}
	if len(coverage.Healed) != 0 {
		return writeLintf(os.Stderr, "lint_flavors: %d RESIDUE entr(y|ies) are now linted: %s. Delete the entry -- a stated remainder that is no longer a remainder hides the next one.\n", len(coverage.Healed), strings.Join(coverage.Healed, ", "))
	}
	return writeLintf(os.Stdout, "lint_flavors: every tracked Go file is linted, except the %d stated above.\n", len(coverage.Blind))
}

func writeLintf(writer io.Writer, format string, arguments ...any) error {
	output := fmt.Appendf(nil, format, arguments...)
	_, err := writer.Write(output)
	return err
}

func recordLintOutputError(report *Report, operation string, err error) {
	outputErr := fmt.Errorf("%s: %w", operation, err)
	if report.Error == "" {
		report.Error = outputErr.Error()
	} else {
		report.Error = errors.Join(errors.New(report.Error), outputErr).Error()
	}
	if report.Code == 0 {
		report.Code = cannotPlan
	}
}
