// Design: docs/architecture/core-design.md -- native precommit verification stages
//
// The ExaBGP compatibility stage is separate from the ordinary functional suite
// table. It needs uv to provide Paramiko, but the test population and the DUT are
// owned by ze-test and the functional binary builder respectively.

package functional

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
)

const (
	exaBGPGate            = "ze-functional-exabgp-test"
	exaBGPTimeoutKey      = "ze.functional.exabgp.timeout"
	exaBGPTimeoutAlias    = "ZE_EXABGP_TIMEOUT"
	exaBGPTimeoutDefault  = "180"
	exaBGPBuildOutputFlag = "-o"
)

const exaBGPOutputMax = 8 << 20

var _ = env.MustRegister(env.EnvEntry{
	Key:         exaBGPTimeoutKey,
	Type:        envString,
	Default:     exaBGPTimeoutDefault,
	Description: "per-case timeout in seconds for ExaBGP compatibility tests",
	Aliases:     []string{exaBGPTimeoutAlias},
	Private:     true,
})

// ExaBGPCommand is one context-bound child of the compatibility stage.
// Environment is the complete child environment and can contain credentials;
// callers MUST NOT render it. ReportEnvironment is the safe subset recorded in
// the stage report.
type ExaBGPCommand struct {
	Stage             string
	Arguments         []string
	Directory         string
	Environment       []string
	ReportEnvironment []string
	Artifact          string
}

// ExaBGPExecution is the observable result of one child process.
type ExaBGPExecution struct {
	Stdout string
	Stderr string
	Error  string
	Code   int
}

// ExaBGPRunner runs one child. Implementations MUST bind every process to ctx
// and MUST return the child's exact exit code.
type ExaBGPRunner interface {
	Run(ctx context.Context, command ExaBGPCommand) ExaBGPExecution
}

// ExaBGPChildResult records one build or compatibility-suite child.
type ExaBGPChildResult struct {
	Stage       string   `json:"stage"`
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

// ExaBGPReport is the structured answer from ze-functional-exabgp-test.
type ExaBGPReport struct {
	Gate      string              `json:"gate"`
	Root      string              `json:"root"`
	Timeout   string              `json:"timeout"`
	Artifacts []string            `json:"artifacts"`
	Children  []ExaBGPChildResult `json:"children"`
	Cleanup   ExaBGPCleanup       `json:"cleanup"`
	Error     string              `json:"error,omitempty"`
	Code      int                 `json:"code"`
}

// RunExaBGP builds the exact ze and ze-test subjects, then runs every ExaBGP
// compatibility case through uv. A nil runner selects the real process runner.
// The first failing child supplies both the report code and the returned code.
func RunExaBGP(ctx context.Context, root string, runner ExaBGPRunner) (
	report ExaBGPReport,
	code int,
) {
	report = ExaBGPReport{
		Gate:      exaBGPGate,
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

	last := len(report.Children) - 1
	if report.Children[last].Stdout == "" {
		if report.Children[last].Stderr == "" {
			report.Children[last].Code = 1
			report.Children[last].Error = "ExaBGP suite produced no output"
			report.Code = 1
			return report, 1
		}
	}

	report.Code = 0
	return report, 0
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
		dir := canonicalBinDir(toolchain.Root)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return BinarySet{}, err
		}
		return BinarySet{Dir: dir}, nil
	}

	root, remove := BinaryRoot(toolchain.Root, "exabgp")
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
) ([]ExaBGPCommand, error) {
	buildEnvironment := toolchain.Environment(gotoolchain.EnvOptions{})
	buildReportEnvironment := toolchain.Overrides(gotoolchain.EnvOptions{})
	commands := make([]ExaBGPCommand, 0, 3)
	var tb textbuf.Buffer
	zeFound := false
	zeTestFound := false

	for _, arguments := range BuildCommands(toolchain, set.Dir, false) {
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
		case ZeTest:
			if zeTestFound {
				return nil, errors.New("functional artifact owner declared ze-test more than once")
			}
			zeTestFound = true
		default:
			continue
		}
		commands = append(commands, ExaBGPCommand{
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
	if !zeTestFound {
		return nil, errors.New("functional artifact owner declared no ze-test build")
	}

	runEnvironment := set.Environment(toolchain)
	runReportEnvironment := toolchain.Overrides(gotoolchain.EnvOptions{})
	zePathEnvironment := tb.Reset().Str("ZE_BIN=").Str(filepath.Join(set.Dir, "ze")).String()
	zeTestPathEnvironment := tb.Reset().Str("ZE_TEST_BIN=").Str(set.ZeTestPath()).String()
	runReportEnvironment = append(runReportEnvironment,
		"ZE_TEST_NO_BUILD=1",
		zePathEnvironment,
		zeTestPathEnvironment,
	)
	commands = append(commands, ExaBGPCommand{
		Stage: "exabgp",
		Arguments: []string{
			"uv", "run", "--with", "paramiko", set.ZeTestPath(),
			"exabgp", "--all", "--timeout", timeout,
		},
		Directory:         toolchain.Root,
		Environment:       runEnvironment,
		ReportEnvironment: runReportEnvironment,
	})
	return commands, nil
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
	runner ExaBGPRunner,
	command ExaBGPCommand,
) ExaBGPChildResult {
	execution := runner.Run(ctx, command)
	return ExaBGPChildResult{
		Stage:       command.Stage,
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

func exaBGPCleanupBinarySet(set BinarySet, report *ExaBGPReport, code *int) {
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

func exaBGPSetupFailure(report ExaBGPReport, err error) (ExaBGPReport, int) {
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
	command ExaBGPCommand,
) ExaBGPExecution {
	if len(command.Arguments) == 0 {
		return ExaBGPExecution{
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
	result := ExaBGPExecution{
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
