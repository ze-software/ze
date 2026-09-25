// Design: docs/architecture/core-design.md -- the terminal-demo area, as one le command
// Overview: types.go -- the engine each renderer action constructs
// Detail: render.go -- check, validation, and render behavior

package siteterminaldemo

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
	"github.com/ze-software/ze/internal/le/linuxle"
)

const (
	area = "site terminal-demo"
	// demoKeyword types the value the render action selects one demo by, so a
	// demo id can never sit in an untyped positional slot (ai/rules/cli.md).
	demoKeyword = "name"
	// ptyVerb is the action the renderer container runs to record a tape.
	ptyVerb    = "pty"
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
		leaction.Action{Verb: "image-build", Why: "the container every demo is recorded in, tagged as the manifest names it",
			Writes: true,
			Answer: runImageBuild},
		leaction.Action{Verb: "binaries-build-ze", Why: "the ze a demo drives, and the le that records it, cross-built for the renderer container",
			Writes: true,
			Answer: runBuild},
		leaction.Action{Verb: "render-all", Why: "re-record every website demo from its checked-in tape",
			Writes: true,
			Answer: func() (any, int) { return runRenderer(rendererRenderMode, true) }},
		leaction.Action{Verb: "render", Why: "re-record ONE website demo from its checked-in tape, for a developer iterating on that demo",
			Writes:     true,
			Parameters: []leaction.Parameter{{Keyword: demoKeyword, Value: "demo-id", Requirement: leaction.Required}},
			AnswerArgs: runRenderOne},
		leaction.Action{Verb: ptyVerb, Why: "record one tape, or drive a program through a PTY; the renderer container runs it",
			AnswerWords: runPTYAction},
	)
}

// runPTYAction hands its words to RunPTY verbatim, so every option, a help
// word included, is RunPTY's to parse. RunPTY writes its own output.
func runPTYAction(words []string) (any, int) {
	return nil, RunPTY(words, os.Stdout, os.Stderr)
}

// Actions answers every action with its exact writes metadata.
func Actions() leaction.List { return actionTable().Actions() }

// Subs answers the action hint from the same table.
func Subs() string { return actionTable().Subs() }

// Answer dispatches one action or sweeps several in command-line order.
//
// The words after an argument-aware action are its values, not more action
// names, so that action routes to Answer whatever follows it. A sweep names
// actions alone and carries no value.
func Answer(args []string) (any, int) {
	table := actionTable()
	if len(args) <= 1 || table.TakesArguments(args[0]) {
		return table.Answer(args)
	}
	return table.Sweep(args, leaction.RunEveryAction)
}

func runBuild() (any, int) {
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
	command, report := buildCommand(root, toolchain, arch)
	if err := os.MkdirAll(filepath.Dir(report.Output), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	gaterun.Announce(report.Action)
	code := gaterun.Stream(command.Args, command.Dir, command.Env)
	if code == 0 {
		tags, err := linuxle.Tags(root)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		recorder, output := recorderBuildCommand(root, command.Env, tags, arch)
		report.HelperArgs = recorder.Args
		report.HelperOutput = output
		code = gaterun.Stream(recorder.Args, recorder.Dir, recorder.Env)
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
func buildCommand(root string, toolchain gotoolchain.Toolchain, arch string) (Command, BuildReport) {
	tags := demoTags(toolchain)
	ldflags := []string{"-ldflags", toolchain.LDFlags()}
	action := "terminal-demo binaries-build-ze"
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", "ze")
	args := make([]string, 0, 7+len(ldflags))
	args = append(args, "go", goCommandBuild, "-tags", tags)
	args = append(args, ldflags...)
	args = append(args, "-o", output, "./cmd/ze")
	environ := toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: arch})
	command := Command{Args: args, Dir: root, Env: environ}
	return command, BuildReport{Action: action, Args: args, Output: output}
}

// demoHarness and demoHarnessWord start a harness helper in the demo container:
// every harness command is `le test <name>`, run by the linux le the recorder
// build writes beside ze.
const (
	demoHarness     = linuxle.Name
	demoHarnessWord = "test"
)

// recorderBuildCommand answers the build of the linux le the renderer
// container records with (`le site terminal-demo pty`), and runs every harness
// helper a scenario starts (`le test peer`), and the path it writes.
// The recipe is linuxle's, the one the perf sender container runs, applied over
// the demo toolchain environment; the file name is linuxle.Name because that
// name is what makes the binary le.
func recorderBuildCommand(root string, environ []string, tags, arch string) (Command, string) {
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", linuxle.Name)
	return Command{
		Args: linuxle.Argv(tags, output),
		Dir:  root,
		Env:  append(append([]string(nil), environ...), linuxle.Overrides(arch)...),
	}, output
}

func runtimeBuildCommand(root string, environ []string) Command {
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-demo")
	return Command{
		Args: []string{"go", goCommandBuild, "-o", output, "./demos/terminal/cmd/ze-demo"},
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
	engine, release, err := renderEngine(releaseRequired)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, code, err := executeRenderer(engine, mode, release)
	if err != nil {
		leaction.ReportError(err)
	}
	return report, code
}

// runImageBuild builds the renderer container image. It needs no release,
// because the image carries the renderer's pinned versions and not Ze's.
func runImageBuild() (any, int) {
	engine, _, err := renderEngine(false)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := engine.BuildImage()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	return report, 0
}

// runRenderOne records the one demo the invocation names.
//
// A missing name is refused rather than read as "every demo". The action that
// records the whole gallery is render-all, and a developer who typed `render`
// asked for one demo.
func runRenderOne(args leaction.Arguments) (any, int) {
	demoID := args.One(demoKeyword)
	if demoID == "" {
		leaction.ReportError(errors.New(
			"site terminal-demo render names one demo: le site terminal-demo render name <demo-id>" +
				" (the ids are in demos/terminal/manifest.json; render-all records every one)"))
		return nil, 2
	}
	engine, release, err := renderEngine(true)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report, err := engine.RenderOne(demoID, release)
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	return report, 0
}

// renderEngine builds the engine every renderer action runs, and answers the
// release identity a recording stamps into its artifacts.
func renderEngine(releaseRequired bool) (*Engine, string, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, "", err
	}
	binDir := filepath.Join(root, "tmp", "terminal-demos", "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return nil, "", err
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
				return nil, "", err
			}
			release = toolchain.Version
		}
	}
	return New(Options{Root: root, ArtifactRoot: artifactRoot}), release, nil
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
