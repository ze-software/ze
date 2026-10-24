// Design: docs/architecture/core-design.md -- native precommit verification stages
//
// The ExaBGP compatibility stage is separate from the ordinary functional suite
// table. It needs uv to provide Paramiko, but the test population and the DUT are
// owned by le test and the functional binary builder respectively.

package testfunctional

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
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/test/runner"
)

const (
	exaBGPAction          = "functional/exabgp-test"
	exaBGPTimeoutKey      = "ze.functional.exabgp.timeout"
	exaBGPTimeoutAlias    = "ZE_EXABGP_TIMEOUT"
	exaBGPTimeoutDefault  = "180"
	exaBGPBuildOutputFlag = "-o"
)

const exaBGPOutputMax = 8 << 20

// exaBGPPin is one ExaBGP distribution the compatibility populations run
// under. The distribution is the predecessor's own Python code, and it is a
// SUBJECT of this stage rather than a convenience for it. api-healthcheck-module
// runs ExaBGP's healthcheck application in process
// (`from exabgp.application.healthcheck import main`, test/exabgp-compat/etc/run/),
// so what that case proves is that ze answers the API line protocol ExaBGP's own
// code writes. Without the distribution the script dies on ModuleNotFoundError,
// and a stub in its place would leave the case green while proving nothing.
//
// Each pin is exact for the reason StaticcheckVersion is pinned
// (internal/le/setup/tools.go): a release that changes the healthcheck's option
// set, its announce line, or its ack handling changes what the case drives, and
// an unpinned dependency makes that a silent change on a machine that happened
// to resolve a newer release.
type exaBGPPin struct {
	// Name is the pin's part of each stage name, `exabgp-<Name>-<suite>`.
	Name string
	// Requirement is what `uv run --with` installs.
	Requirement string
}

// exaBGPPython is the interpreter every pin runs on, passed to uv as
// `--python`. Left to uv, the choice followed the machine: on the owner's
// workstation it picked a managed CPython 3.8.20, which 5.0.15's healthcheck
// cannot run on (plan/journal/gate-verdict-depends-on-the-machine.md, row
// 2026-10-17). ExaBGP main declares `requires-python = ">=3.12"` in its
// pyproject.toml, and 5.0.15 runs on it too, so one interpreter serves both.
const exaBGPPython = "3.12"

// exaBGPPins are the two ExaBGP versions ze answers.
//
// 5.0.15 is the last 5.0 release and supplies every option the healthcheck
// fixture passes, `--no-ack` and `--fast-interval` among them.
//
// main is ExaBGP 6.0 before its release, pinned to a commit of
// github.com/Exa-Networks/exabgp, which uv builds once and caches. The nightly
// zipapp was the first choice and cannot be pinned: the release workflow
// replaces the asset of the `nightly` pre-release and moves its tag each night
// main changes (.github/workflows/zipapp.yml in that repository), so a recorded
// sha256 stops matching anything that can be fetched. A commit is fetched as
// long as the repository exists. Bump it deliberately, with the cases that the
// new commit changes.
var exaBGPPins = [...]exaBGPPin{
	{Name: "5.0.15", Requirement: "exabgp==5.0.15"},
	{
		Name:        "main",
		Requirement: "exabgp @ git+https://github.com/Exa-Networks/exabgp@f9bfb853b924e97deba646b0bd50693ab07c4f3e",
	},
}

// exaBGPSuites are the predecessor populations each pin runs, in order.
// `encoding` drives ze from a migrated config alone; `api` drives it through
// the ExaBGP bridge, which runs the script the config's process block names.
//
// Until 2026-09-05 this stage ran `encoding` and nothing else, and
// zeTestExabgpMain refused every other name with "only predecessor encoding
// tests are available". So `exabgp-test` was 42 config-to-wire cases and the
// ExaBGP API line protocol had no gate at all, which is how 46 run scripts
// came to sit in the tree with no runner.
var exaBGPSuites = [...]string{"encoding", "api"}

// exaBGPOldSuites are the populations copied from ExaBGP's 4.2 and 5.0
// branches (qa/old/<branch> at the main pin's commit). They run once per
// stage, under the last pin's uv environment for its named Python, rather than
// once per pin: ze is the daemon, their helpers import no ExaBGP (only the
// exabgp_api.py beside them), so a run under each pin would drive the same code
// twice.
var exaBGPOldSuites = [...]string{"old-4.2", "old-5.0"}

// exaBGPVersionProbe prints the version of the distribution a pin resolved,
// so the report names what ran rather than what was asked for.
const exaBGPVersionProbe = "from exabgp.version import version; print(version)"

// exaBGPPythonPathKey is cleared on every uv child. A shell that exports it
// (an ExaBGP developer's does, pointing at a checkout) puts that checkout ahead
// of the pinned distribution, and the stage then drives code it does not name
// (plan/journal/gate-verdict-depends-on-the-machine.md, row 2026-09-13).
const exaBGPPythonPathKey = "PYTHONPATH"

var _ = env.MustRegister(env.EnvEntry{
	Key:         exaBGPTimeoutKey,
	Type:        envString,
	Default:     exaBGPTimeoutDefault,
	Description: "per-case timeout in seconds for ExaBGP compatibility tests",
	Aliases:     []string{exaBGPTimeoutAlias},
	Private:     true,
})

// exaBGPCommand is one context-bound child of the compatibility stage.
// Environment is the complete child environment and can contain credentials;
// callers MUST NOT render it. ReportEnvironment is the safe subset recorded in
// the stage report.
type exaBGPCommand struct {
	Stage string
	// Pin names the exaBGPPins entry a uv child runs under, empty for a build.
	Pin               string
	Arguments         []string
	Directory         string
	Environment       []string
	ReportEnvironment []string
	Artifact          string
}

// exaBGPExecution is the observable result of one child process.
type exaBGPExecution struct {
	Stdout string
	Stderr string
	Error  string
	Code   int
}

// exaBGPRunner runs one child. Implementations MUST bind every process to ctx
// and MUST return the child's exact exit code.
type exaBGPRunner interface {
	Run(ctx context.Context, command exaBGPCommand) exaBGPExecution
}

// ExaBGPChildResult records one build or compatibility-suite child.
type ExaBGPChildResult struct {
	Stage       string   `json:"stage"`
	Pin         string   `json:"pin,omitempty"`
	Command     []string `json:"command"`
	Directory   string   `json:"directory"`
	Environment []string `json:"environment,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Stdout      string   `json:"stdout,omitempty"`
	Stderr      string   `json:"stderr,omitempty"`
	Error       string   `json:"error,omitempty"`
	Code        int      `json:"code"`
}

// ExaBGPCleanup records whether the invocation removed its isolated artifacts
// or deliberately retained a named/canonical binary set.
type ExaBGPCleanup struct {
	Path    string `json:"path,omitempty"`
	Removed bool   `json:"removed"`
	Kept    bool   `json:"kept"`
	Error   string `json:"error,omitempty"`
}

// exaBGPReport is the structured answer from `le test functional exabgp-test`.
type exaBGPReport struct {
	Action    string              `json:"action"`
	Root      string              `json:"root"`
	Timeout   string              `json:"timeout"`
	Artifacts []string            `json:"artifacts"`
	Children  []ExaBGPChildResult `json:"children"`
	Cleanup   ExaBGPCleanup       `json:"cleanup"`
	Error     string              `json:"error,omitempty"`
	Code      int                 `json:"code"`
}

// Text names the setup error and every failing child, because nothing else
// prints them: a child that RAN streamed its own output through the process
// runner, and a child that never started has only this line
// (plan/journal/failing-gate-prints-no-cause.md). A missing `uv` is the case
// this exists for, since the stage then answers 127 with an empty terminal.
func (r exaBGPReport) Text() string {
	var tb textbuf.Buffer
	action := r.Action
	if action == "" {
		action = exaBGPAction
	}
	if r.Error != "" {
		tb.Str(action).Str(": ").Str(r.Error).Byte('\n')
	}
	for index := range r.Children {
		child := &r.Children[index]
		if child.Code == 0 {
			continue
		}
		tb.Str(action).Str(": stage ").Str(child.Stage).Str(" exited ").Int(int64(child.Code))
		if child.Error != "" {
			tb.Str(": ").Str(child.Error)
		}
		tb.Byte('\n')
	}
	if r.Cleanup.Error != "" {
		tb.Str(action).Str(": cleanup of ").Str(r.Cleanup.Path).Str(" failed: ").
			Str(r.Cleanup.Error).Byte('\n')
	}
	failed := tb.Len() != 0
	// Each pin's probe names the ExaBGP that ran, pass or fail, because the
	// requirement is what was asked for and the probe is what uv resolved.
	for index := range r.Children {
		child := &r.Children[index]
		if child.Pin == "" {
			continue
		}
		if child.Code != 0 {
			continue
		}
		if !strings.HasPrefix(child.Stage, "version-") {
			continue
		}
		tb.Str(action).Str(": pin ").Str(child.Pin).Str(" ran ExaBGP ").
			Str(strings.TrimSpace(child.Stdout)).Byte('\n')
	}
	if failed {
		return tb.String()
	}
	return tb.Str(action).Str(": ").Int(int64(len(r.Children))).
		Str(" children passed\n").String()
}

// runExaBGP builds the exact ze and le test subjects, then runs every ExaBGP
// compatibility case through uv. A nil runner selects the real process runner.
// The first failing child supplies both the report code and the returned code.
func runExaBGP(ctx context.Context, root string, runner exaBGPRunner) (
	report exaBGPReport,
	code int,
) {
	report = exaBGPReport{
		Action:    exaBGPAction,
		Root:      root,
		Timeout:   exaBGPTimeout(),
		Children:  []ExaBGPChildResult{},
		Artifacts: []string{},
	}
	if ctx == nil {
		return exaBGPSetupFailure(report, errors.New("ExaBGP stage has no context"))
	}
	if root == "" {
		return exaBGPSetupFailure(report, errors.New("ExaBGP stage has no repository root"))
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return exaBGPSetupFailure(report, fmt.Errorf("resolve ExaBGP repository root: %w", err))
	}
	report.Root = absoluteRoot

	toolchain, err := gotoolchain.New(absoluteRoot)
	if err != nil {
		return exaBGPSetupFailure(report, fmt.Errorf("resolve ExaBGP Go toolchain: %w", err))
	}

	set, err := exaBGPBinarySet(toolchain)
	if err != nil {
		return exaBGPSetupFailure(report, fmt.Errorf("create ExaBGP binary set: %w", err))
	}
	defer func() {
		exaBGPCleanupBinarySet(set, &report, &code)
	}()

	commands, err := exaBGPCommands(toolchain, set, report.Timeout)
	if err != nil {
		return exaBGPSetupFailure(report, err)
	}
	for _, command := range commands {
		if command.Artifact != "" {
			report.Artifacts = append(report.Artifacts, command.Artifact)
		}
	}

	if runner == nil {
		runner = exaBGPProcessRunner{}
	}
	for _, command := range commands {
		child := exaBGPRunChild(ctx, runner, command)
		report.Children = append(report.Children, child)
		if child.Code != 0 {
			report.Code = child.Code
			return report, child.Code
		}
		if command.Artifact == "" {
			continue
		}
		artifactErr := exaBGPCheckArtifact(command.Artifact)
		if artifactErr == nil {
			continue
		}
		last := len(report.Children) - 1
		report.Children[last].Code = 1
		report.Children[last].Error = artifactErr.Error()
		report.Code = 1
		return report, 1
	}

	// A suite that says nothing has not passed: it did not run. The check reads
	// EVERY subject rather than the last one, because this stage runs several
	// suites since 2026-09-05 and a guard on the last child alone would let a
	// silent encoding run through on the strength of the api run's output. A
	// version probe that prints nothing named no version, so it is held to the
	// same rule.
	for index := range report.Children {
		child := &report.Children[index]
		if child.Pin == "" {
			continue
		}
		if child.Stdout != "" || child.Stderr != "" {
			continue
		}
		child.Code = 1
		child.Error = exaBGPSilentChild(child)
		report.Code = 1
		return report, 1
	}

	report.Code = 0
	return report, 0
}

// exaBGPSilentChild names a child that printed nothing. A version probe is
// named as one, with the command to run by hand, because "suite" sent the
// reader to a suite that never started.
func exaBGPSilentChild(child *ExaBGPChildResult) string {
	if !strings.HasPrefix(child.Stage, "version-") {
		return "ExaBGP suite produced no output"
	}
	var tb textbuf.Buffer
	return tb.Str("ExaBGP version probe ").Str(child.Stage).
		Str(" printed nothing, so the report cannot name the ExaBGP the pin ").Str(child.Pin).
		Str(" ran. Run the probe by hand to see why: ").Str(strings.Join(child.Command, " ")).String()
}

func exaBGPTimeout() string {
	timeout := env.Get(exaBGPTimeoutAlias)
	if timeout == "" {
		timeout = exaBGPTimeoutDefault
	}
	var tb textbuf.Buffer
	return tb.Str(timeout).Byte('s').String()
}

func exaBGPBinarySet(toolchain gotoolchain.Toolchain) (BinarySet, error) {
	if env.Get("ze.test.canonical") != "" {
		dir, err := canonicalBinDir(toolchain.Root)
		if err != nil {
			return BinarySet{}, err
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return BinarySet{}, err
		}
		return BinarySet{Dir: dir}, nil
	}

	root, remove, err := binaryRoot(toolchain.Root, "exabgp")
	if err != nil {
		return BinarySet{}, err
	}
	dir := filepath.Join(root, "bin")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return BinarySet{}, err
	}
	return BinarySet{Dir: dir, Remove: remove}, nil
}

func exaBGPCommands(
	toolchain gotoolchain.Toolchain,
	set BinarySet,
	timeout string,
) ([]exaBGPCommand, error) {
	buildEnvironment := toolchain.Environment(gotoolchain.EnvOptions{})
	buildReportEnvironment := toolchain.Overrides(gotoolchain.EnvOptions{})
	commands := make([]exaBGPCommand, 0, 3)
	var tb textbuf.Buffer
	zeFound := false
	leFound := false

	for _, arguments := range buildCommands(toolchain, set.Dir) {
		artifact, ok := exaBGPBuildArtifact(arguments)
		if !ok {
			return nil, errors.New("functional artifact owner declared a build without an output")
		}
		name := filepath.Base(artifact)
		switch name {
		case "ze":
			if zeFound {
				return nil, errors.New("functional artifact owner declared ze more than once")
			}
			zeFound = true
		case LE:
			if leFound {
				return nil, errors.New("functional artifact owner declared le more than once")
			}
			leFound = true
		default:
			continue
		}
		commands = append(commands, exaBGPCommand{
			Stage:             tb.Reset().Str("build-").Str(name).String(),
			Arguments:         append([]string(nil), arguments...),
			Directory:         toolchain.Root,
			Environment:       append([]string(nil), buildEnvironment...),
			ReportEnvironment: append([]string(nil), buildReportEnvironment...),
			Artifact:          artifact,
		})
	}
	if !zeFound {
		return nil, errors.New("functional artifact owner declared no ze build")
	}
	if !leFound {
		return nil, errors.New("functional artifact owner declared no le build")
	}

	pythonPathCleared := exaBGPPythonPathKey + "="
	runEnvironment := append(env.Without(set.Environment(toolchain), exaBGPPythonPathKey), pythonPathCleared)
	runReportEnvironment := toolchain.Overrides(gotoolchain.EnvOptions{})
	zePathEnvironment := tb.Reset().Str("ZE_BIN=").Str(filepath.Join(set.Dir, "ze")).String()
	runReportEnvironment = append(runReportEnvironment,
		runner.EnvNoBuild+"=1",
		zePathEnvironment,
		pythonPathCleared,
	)
	for _, pin := range exaBGPPins {
		// The probe runs first, so a pin that cannot be installed stops the
		// stage before any population and its report names the install, not a
		// case.
		commands = append(commands, exaBGPCommand{
			Stage:             tb.Reset().Str("version-").Str(pin.Name).String(),
			Pin:               pin.Name,
			Arguments:         exaBGPUVRun(pin, "python", "-c", exaBGPVersionProbe),
			Directory:         toolchain.Root,
			Environment:       runEnvironment,
			ReportEnvironment: runReportEnvironment,
		})
		for _, suite := range exaBGPSuites {
			commands = append(commands, exaBGPCommand{
				Stage: tb.Reset().Str("exabgp-").Str(pin.Name).Byte('-').Str(suite).String(),
				Pin:   pin.Name,
				Arguments: exaBGPUVRun(pin, set.lePath(),
					leTestWord, "exabgp", suite, "--all", "--timeout", timeout),
				Directory:         toolchain.Root,
				Environment:       runEnvironment,
				ReportEnvironment: runReportEnvironment,
			})
		}
	}
	oldPin := exaBGPPins[len(exaBGPPins)-1]
	for _, suite := range exaBGPOldSuites {
		commands = append(commands, exaBGPCommand{
			Stage: tb.Reset().Str("exabgp-").Str(oldPin.Name).Byte('-').Str(suite).String(),
			Pin:   oldPin.Name,
			Arguments: exaBGPUVRun(oldPin, set.lePath(),
				leTestWord, "exabgp", suite, "--all", "--timeout", timeout),
			Directory:         toolchain.Root,
			Environment:       runEnvironment,
			ReportEnvironment: runReportEnvironment,
		})
	}
	return commands, nil
}

// exaBGPUVRun is the argv that runs command under pin. Every option uv would
// otherwise pick for itself is passed: the interpreter, and no project, so a
// pyproject.toml above the checkout cannot add its own dependencies.
func exaBGPUVRun(pin exaBGPPin, command ...string) []string {
	return append([]string{
		"uv", "run", "--no-project", "--python", exaBGPPython,
		"--with", "paramiko", "--with", pin.Requirement,
	}, command...)
}

func exaBGPBuildArtifact(arguments []string) (string, bool) {
	for index := range len(arguments) - 1 {
		if arguments[index] != exaBGPBuildOutputFlag {
			continue
		}
		return arguments[index+1], true
	}
	return "", false
}

func exaBGPRunChild(
	ctx context.Context,
	runner exaBGPRunner,
	command exaBGPCommand,
) ExaBGPChildResult {
	execution := runner.Run(ctx, command)
	return ExaBGPChildResult{
		Stage:       command.Stage,
		Pin:         command.Pin,
		Command:     append([]string(nil), command.Arguments...),
		Directory:   command.Directory,
		Environment: append([]string(nil), command.ReportEnvironment...),
		Artifact:    command.Artifact,
		Stdout:      execution.Stdout,
		Stderr:      execution.Stderr,
		Error:       execution.Error,
		Code:        execution.Code,
	}
}

func exaBGPCheckArtifact(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("ExaBGP build did not produce %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("ExaBGP build output is not a regular file: %s", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("ExaBGP build output is empty: %s", path)
	}
	return nil
}

func exaBGPCleanupBinarySet(set BinarySet, report *exaBGPReport, code *int) {
	path := filepath.Dir(set.Dir)
	if !set.Remove {
		report.Cleanup = ExaBGPCleanup{Path: path, Kept: true}
		return
	}
	if err := os.RemoveAll(path); err != nil {
		report.Cleanup = ExaBGPCleanup{Path: path, Error: err.Error()}
		if *code == 0 {
			*code = 1
			report.Code = 1
		}
		return
	}
	report.Cleanup = ExaBGPCleanup{Path: path, Removed: true}
}

func exaBGPSetupFailure(report exaBGPReport, err error) (exaBGPReport, int) {
	report.Error = err.Error()
	report.Code = 1
	return report, 1
}

type exaBGPProcessRunner struct{}

type exaBGPCapture struct {
	bytes.Buffer
}

func (capture *exaBGPCapture) Write(output []byte) (int, error) {
	remaining := exaBGPOutputMax + 1 - capture.Len()
	if remaining > 0 {
		count := min(len(output), remaining)
		if _, err := capture.Buffer.Write(output[:count]); err != nil {
			return 0, err
		}
	}
	return len(output), nil
}

func (exaBGPProcessRunner) Run(
	ctx context.Context,
	command exaBGPCommand,
) exaBGPExecution {
	if len(command.Arguments) == 0 {
		return exaBGPExecution{
			Error: "ExaBGP child declared no command",
			Code:  gaterun.CannotStart,
		}
	}

	arguments := command.Arguments
	cmd := exec.CommandContext(ctx, arguments[0], arguments[1:]...) //nolint:gosec // closed stage grammar
	cmd.Dir = command.Directory
	cmd.Env = command.Environment
	cmd.Stdin = os.Stdin
	var stdout exaBGPCapture
	var stderr exaBGPCapture
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	cmd.WaitDelay = 5 * time.Second

	err := cmd.Run()
	result := exaBGPExecution{
		Stdout: exaBGPOutput(stdout.Bytes()),
		Stderr: exaBGPOutput(stderr.Bytes()),
	}
	if err == nil {
		if stdout.Len() > exaBGPOutputMax {
			result.Error = "ExaBGP child stdout exceeded the 8 MiB report limit"
			result.Code = 1
			return result
		}
		if stderr.Len() > exaBGPOutputMax {
			result.Error = "ExaBGP child stderr exceeded the 8 MiB report limit"
			result.Code = 1
		}
		return result
	}
	result.Error = err.Error()
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		result.Code = gaterun.CannotStart
		return result
	}
	result.Code = gaterun.ExitCode(err)
	return result
}

func exaBGPOutput(output []byte) string {
	output = output[:min(len(output), exaBGPOutputMax)]
	return string(output)
}
