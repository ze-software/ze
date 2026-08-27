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
	Gate   string   `json:"gate"`
	Args   []string `json:"args"`
	Output string   `json:"output"`
	Code   int      `json:"code"`
}

// Text returns no additional prose. The Go toolchain streams its own output.
func (BuildReport) Text() string { return "" }

func actionTable() leaction.Area {
	return leaction.New(area,
		leaction.Action{
			Gate:   "ze-terminal-demo-check-all",
			Why:    "every demo the manifest declares has its published artifacts",
			Answer: func() (any, int) { return runRenderer("check", false) },
		},
		leaction.Action{
			Gate:   "ze-terminal-demo-validation-check-all",
			Why:    "each scenario's output validators pass, so a demo shows the product working",
			Answer: func() (any, int) { return runRenderer(rendererValidateMode, false) },
		},
		leaction.Action{
			Gate:   "ze-terminal-demo-release-check-all",
			Why:    "the published artifacts carry this release identity, which is what a tag ships",
			Answer: func() (any, int) { return runRenderer("check", true) },
		},
		leaction.Action{
			Gate:   "ze-terminal-demo-binaries-build-ze",
			Why:    "the ze a demo drives, cross-built for the renderer container",
			Writes: true,
			Answer: func() (any, int) { return runBuild(false) },
		},
		leaction.Action{
			Gate:   "ze-terminal-demo-binaries-build-ze-test",
			Why:    "the ze-test a demo drives, which carries ze_test alone and no version",
			Writes: true,
			Answer: func() (any, int) { return runBuild(true) },
		},
		leaction.Action{
			Gate:   "ze-terminal-demo-render-all",
			Why:    "re-record every website demo from its checked-in tape",
			Writes: true,
			Answer: func() (any, int) { return runRenderer(rendererRenderMode, true) },
		},
	)
}

// Actions answers all six gates with their exact writes metadata.
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
	gaterun.Announce(report.Gate)
	code := gaterun.Stream(command.Args, command.Dir, command.Env)
	report.Code = code
	return report, code
}

func rendererGOARCH() (string, error) {
	if override := os.Getenv(goarchEnv); override != "" {
		return override, nil
	}
	command := exec.CommandContext(context.Background(), "go", "env", "GOARCH") // #nosec G204 -- The executable is the fixed Go toolchain used by this build gate.
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
	gate := "ze-terminal-demo-binaries-build-ze"
	if testBinary {
		outputName = "ze-test"
		tags = "ze_test"
		ldflags = nil
		gate = "ze-terminal-demo-binaries-build-ze-test"
	}
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", outputName)
	args := make([]string, 0, 7+len(ldflags))
	args = append(args, "go", "build", "-tags", tags)
	args = append(args, ldflags...)
	args = append(args, "-o", output, "./cmd/ze")
	environ := toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch})
	command := Command{Args: args, Dir: root, Env: environ}
	return command, BuildReport{Gate: gate, Args: args, Output: output}
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
		report, err = engine.CheckAll(release)
	case rendererValidateMode:
		report, err = engine.ValidationCheckAll()
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
