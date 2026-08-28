// Design: docs/architecture/core-design.md -- the terminal-demo area, as one le command
// Overview: types.go -- the engine each renderer action constructs
// Detail: render.go -- check, validation, and render behavior

package terminaldemo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/gotoolchain"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	area       = "terminal-demo"
	goarchEnv  = "TERMINAL_DEMO_GOARCH"
	outputEnv  = "TERMINAL_DEMO_OUTPUT"
	releaseEnv = "TERMINAL_DEMO_RELEASE"
)

// BuildReport is the structured result of one staged binary build.
type BuildReport struct {
	Action        string   `json:"action"`
	Args          []string `json:"args"`
	Output        string   `json:"output"`
	HelperArgs    []string `json:"helper_args,omitempty"`
	HelperOutput  string   `json:"helper_output,omitempty"`
	RuntimeArgs   []string `json:"runtime_args,omitempty"`
	RuntimeOutput string   `json:"runtime_output,omitempty"`
	Code          int      `json:"code"`
}

// Text returns no additional prose. The Go toolchain streams its own output.
func (BuildReport) Text() string { return "" }

func actionTable() leaction.Area {
	return leaction.New(area,
		leaction.Action{Verb: "check-all", Why: "every demo the manifest declares has its published artifacts",
			Answer: func() (any, int) { return runRenderer("check", false) }},
		leaction.Action{Verb: "validation-check-all", Why: "each scenario's output validators pass, so a demo shows the product working",
			Answer: func() (any, int) { return runRenderer(rendererValidateMode, false) }},
		leaction.Action{Verb: "release-check-all", Why: "the published artifacts carry this release identity, which is what a tag ships",
			Answer: func() (any, int) { return runRenderer("check", true) }},
		leaction.Action{Verb: "binaries-build-ze", Why: "the ze a demo drives, cross-built for the renderer container",
			Writes: true,
			Answer: func() (any, int) { return runBuild(false) }},
		leaction.Action{Verb: "binaries-build-ze-test", Why: "the ze-test a demo drives, which carries ze_test alone and no version",
			Writes: true,
			Answer: func() (any, int) { return runBuild(true) }},
		leaction.Action{Verb: "render-all", Why: "re-record every website demo from its checked-in tape",
			Writes: true,
			Answer: func() (any, int) { return runRenderer(rendererRenderMode, true) }},
	)
}

// Actions answers all six actions with their exact writes metadata.
func Actions() leaction.List { return actionTable().Actions() }

// Subs answers the action hint from the same table.
func Subs() string { return actionTable().Subs() }

// Answer dispatches one action or sweeps several in command-line order.
func Answer(args []string) (any, int) {
	if len(args) <= 1 {
		return actionTable().Answer(args)
	}
	return actionTable().Sweep(args, leaction.RunEveryAction)
}

func runBuild(testBinary bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	arch, err := rendererGOARCH()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	command, report := buildCommand(root, toolchain, arch, testBinary)
	if err := os.MkdirAll(filepath.Dir(report.Output), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	gaterun.Announce(report.Action)
	code := gaterun.Stream(command.Args, command.Dir, command.Env)
	if code == 0 && !testBinary {
		helper := ptyBuildCommand(root, command.Env)
		report.HelperArgs = helper.Args
		report.HelperOutput = helper.Args[len(helper.Args)-2]
		code = gaterun.Stream(helper.Args, helper.Dir, helper.Env)
		if code == 0 {
			runtime := runtimeBuildCommand(root, command.Env)
			report.RuntimeArgs = runtime.Args
			report.RuntimeOutput = runtime.Args[len(runtime.Args)-2]
			code = gaterun.Stream(runtime.Args, runtime.Dir, runtime.Env)
		}
	}
	report.Code = code
	return report, code
}

func rendererGOARCH() (string, error) {
	if override := os.Getenv(goarchEnv); override != "" {
		return override, nil
	}
	command := exec.CommandContext(context.Background(), "go", "env", "GOARCH") // #nosec G204 -- The executable is the fixed Go toolchain used by this build action.
	output, err := command.Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return "", fmt.Errorf("read renderer GOARCH: %w", err)
		}
	}
	return strings.TrimSpace(string(output)), nil
}
func buildCommand(root string, toolchain gotoolchain.Toolchain, arch string, testBinary bool) (Command, BuildReport) {
	outputName := "ze"
	tags := demoTags(toolchain)
	ldflags := []string{"-ldflags", toolchain.LDFlags()}
	action := "terminal-demo binaries-build-ze"
	if testBinary {
		outputName = "ze-test"
		tags = "ze_test"
		ldflags = nil
		action = "terminal-demo binaries-build-ze-test"
	}
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", outputName)
	args := make([]string, 0, 7+len(ldflags))
	args = append(args, "go", "build", "-tags", tags)
	args = append(args, ldflags...)
	args = append(args, "-o", output, "./cmd/ze")
	environ := toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch})
	command := Command{Args: args, Dir: root, Env: environ}
	return command, BuildReport{Action: action, Args: args, Output: output}
}

func ptyBuildCommand(root string, environ []string) Command {
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-terminal-pty")
	return Command{
		Args: []string{"go", "build", "-o", output, "./cmd/ze-terminal-pty"},
		Dir:  root,
		Env:  environ,
	}
}

func runtimeBuildCommand(root string, environ []string) Command {
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-demo")
	return Command{
		Args: []string{"go", "build", "-o", output, "./demos/terminal/cmd/ze-demo"},
		Dir:  root,
		Env:  environ,
	}
}

func demoTags(toolchain gotoolchain.Toolchain) string {
	var buffer textbuf.Buffer
	buffer.Str("ze_core ze_distro")
	for _, tag := range toolchain.Features {
		buffer.Byte(' ').Str(tag)
	}
	for _, tag := range toolchain.ExtraTags {
		buffer.Byte(' ').Str(tag)
	}
	return buffer.String()
}

func runRenderer(mode string, releaseRequired bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	binDir := filepath.Join(root, "tmp", "terminal-demos", "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	artifactRoot := os.Getenv(outputEnv)
	if artifactRoot == "" {
		artifactRoot = filepath.Join(filepath.Dir(root), "gh-pages", "assets", "demos")
	}
	release := ""
	if releaseRequired {
		release = os.Getenv(releaseEnv)
		if release == "" {
			toolchain, err := gotoolchain.New(root)
			if err != nil {
				leaction.ReportError(err)
				return nil, 1
			}
			release = toolchain.Version
		}
	}
	engine := New(Options{Root: root, ArtifactRoot: artifactRoot})
	report, code, err := executeRenderer(engine, mode, release)
	if err != nil {
		leaction.ReportError(err)
	}
	return report, code
}

func executeRenderer(engine *Engine, mode, release string) (Report, int, error) {
	var report Report
	var err error
	switch mode {
	case "check":
		report, err = engine.checkAll(release)
	case rendererValidateMode:
		report, err = engine.validationCheckAll()
	case rendererRenderMode:
		report, err = engine.RenderAll(release)
	default:
		err = fmt.Errorf("unknown renderer mode %q", mode)
	}
	if err != nil {
		return report, 1, err
	}
	return report, 0, nil
}
