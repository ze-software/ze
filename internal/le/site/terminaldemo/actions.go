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
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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

// The targets `site terminal-demo check` and `site terminal-demo build` take in
// their first slot, and the switch that makes `render` record every demo
// (D-P4-positional of spec-le-cli-usability).
const (
	targetValue     = "target"
	checkArtifacts  = "artifacts"
	checkRelease    = "release"
	checkValidation = "validation"
	buildBinaries   = "binaries"
	buildImage      = "image"
	renderAll       = "all"
)

// checkTargets answers each check target's renderer run. None of them writes.
var checkTargets = map[string]func() (any, int){
	checkArtifacts:  func() (any, int) { return runRenderer("check", false) },
	checkRelease:    func() (any, int) { return runRenderer("check", true) },
	checkValidation: func() (any, int) { return runRenderer(rendererValidateMode, false) },
}

// buildTargets answers each build target's builder.
var buildTargets = map[string]func() (any, int){
	buildBinaries: runBuild,
	buildImage:    runImageBuild,
}

func actionTable() leaction.Area {
	return leaction.New(area,
		leaction.Action{Verb: "check", Summary: "check one property of the published demos", Why: "the published demos hold up: every demo the manifest declares has its published artifacts (artifacts), " +
			"each scenario's output validators pass, so a demo shows the product working (validation), " +
			"or the published artifacts carry this release identity, which is what a tag ships (release)",
			Parameters: []leaction.Parameter{{
				Kind: leaction.KindEnum, Value: targetValue, Requirement: leaction.Required,
				Help:   "the property of the published demos to check",
				Values: []string{checkArtifacts, checkRelease, checkValidation},
				ValueHelp: map[string]string{
					checkArtifacts:  "every demo has its published artifacts",
					checkRelease:    "the demo artifacts carry this release identity",
					checkValidation: "every demo's output validators pass",
				},
			}},
			AnswerArgs: func(args leaction.Arguments) (any, int) { return targetAnswer(checkTargets, args) }},
		leaction.Action{Verb: "build", Summary: "build the demo renderer image or its binaries", Why: "the container every demo is recorded in, tagged as the manifest names it (image), " +
			"or the ze a demo drives, and the le that records it, cross-built for the renderer container (binaries)",
			Writes: true,
			Parameters: []leaction.Parameter{{
				Kind: leaction.KindEnum, Value: targetValue, Requirement: leaction.Required,
				Help:   "what to build for the demo renderer",
				Values: []string{buildBinaries, buildImage},
				ValueHelp: map[string]string{
					buildBinaries: "cross-build ze and le for the demo renderer",
					buildImage:    "build the container demos are recorded in",
				},
			}},
			AnswerArgs: func(args leaction.Arguments) (any, int) { return targetAnswer(buildTargets, args) }},
		leaction.Action{Verb: "render", Summary: "re-record one website demo, or every one", Why: "re-record ONE website demo from its checked-in tape, for a developer iterating on that demo, " +
			"or every website demo (all)",
			Writes: true,
			Parameters: []leaction.Parameter{
				{Keyword: demoKeyword, Kind: leaction.KindLabel, Help: "the demo to re-record, by its manifest id", Value: "demo-id", Requirement: leaction.Optional},
				{Keyword: renderAll, Kind: leaction.KindSwitch, Help: "re-record every website demo"},
			},
			AnswerArgs: runRender},
		leaction.Action{Verb: "readme-carrier", Summary: "render or select the recorded hero's README carrier", Why: "measure real SVG, video and thumbnail candidates, then select only from recorded GitHub observations",
			Writes: true,
			Parameters: []leaction.Parameter{
				{Keyword: "output", Kind: leaction.KindPath, Help: "candidate directory; must be new when rendering", Value: "directory", Requirement: leaction.Required},
				{Keyword: "probe", Kind: leaction.KindPath, Help: "observed GitHub results; omit to render candidates", Value: "file", Requirement: leaction.Optional},
			},
			AnswerArgs: runReadmeCarrier},
		leaction.Action{Verb: ptyVerb, Does: "run", Summary: "record one tape or drive a program through a PTY", Why: "record one tape, or drive a program through a PTY; the renderer container runs it",
			AnswerWords: runPTYAction},
	)
}

// targetAnswer runs the answer a declared target selects. The parser has
// already refused a target the action does not declare.
func targetAnswer(targets map[string]func() (any, int), args leaction.Arguments) (any, int) {
	answer, declared := targets[args.One(targetValue)]
	if !declared {
		panic("BUG: site terminal-demo: a declared target has no answer: " + args.One(targetValue))
	}
	return answer()
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

// Answer dispatches one action.
func Answer(args []string) (any, int) { return actionTable().Answer(args) }

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
	tags := demoTags(toolchain)
	command, report := buildCommand(root, toolchain, arch, tags)
	if err := os.MkdirAll(filepath.Dir(report.Output), 0o750); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	gaterun.Announce(report.Action)
	code := gaterun.Stream(command.Args, command.Dir, command.Env)
	if code == 0 {
		recorderTags, err := linuxle.Tags(root)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		recorder, output := recorderBuildCommand(root, command.Env, recorderTags, arch)
		report.HelperArgs = recorder.Args
		report.HelperOutput = output
		code = gaterun.Stream(recorder.Args, recorder.Dir, recorder.Env)
		if code == 0 {
			runtime := runtimeBuildCommand(root, command.Env, tags)
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
func buildCommand(root string, toolchain gotoolchain.Toolchain, arch, tags string) (Command, BuildReport) {
	ldflags := []string{"-ldflags", toolchain.LDFlags()}
	action := "terminal-demo build binaries"
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

func runtimeBuildCommand(root string, environ []string, tags string) Command {
	output := filepath.Join(root, "tmp", "terminal-demos", "bin", "ze-demo")
	return Command{
		Args: []string{"go", goCommandBuild, "-tags", tags, "-o", output, "./demos/terminal/cmd/ze-demo"},
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
	return signalRooted(func(ctx context.Context) (any, int) {
		return runRendererIn(ctx, mode, releaseRequired)
	})
}

// signalRooted runs one renderer action under a root context SIGINT or
// SIGTERM cancels, so an interrupt stops the demo container and its removal
// runs (interoplab.Docker.RunOneShot) instead of the process dying with the
// container still recording.
func signalRooted(run func(context.Context) (any, int)) (any, int) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

func runRendererIn(ctx context.Context, mode string, releaseRequired bool) (any, int) {
	engine, release, err := renderEngine(ctx, releaseRequired)
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

// imageBuildEngine answers the engine `build image` runs under ctx. It is the
// one seam a test replaces, with an engine over a fixture Docker, so the test
// drives runImageBuild itself and not a copy of its rooting.
var imageBuildEngine = func(ctx context.Context) (*Engine, error) {
	engine, _, err := renderEngine(ctx, false)
	return engine, err
}

// runImageBuild builds the renderer container image. It needs no release,
// because the image carries the renderer's pinned versions and not Ze's.
//
// It is signal rooted like every other renderer action: the docker build runs
// under the root context, so a SIGINT or SIGTERM to le alone stops the build
// rather than leaving it running after le exits.
func runImageBuild() (any, int) {
	return signalRooted(runImageBuildIn)
}

func runImageBuildIn(ctx context.Context) (any, int) {
	engine, err := imageBuildEngine(ctx)
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

// runRender records every demo when the invocation says all, and otherwise the
// one demo it names.
//
// A missing name is refused rather than read as "every demo": a developer who
// typed `render` asked for one demo, and the whole gallery is `render all`.
func runRender(args leaction.Arguments) (any, int) {
	if !args.Has(renderAll) {
		return signalRooted(func(ctx context.Context) (any, int) {
			return runRenderOneIn(ctx, args)
		})
	}
	if args.Has(demoKeyword) {
		leaction.ReportError(errors.New(
			"site terminal-demo render records one demo or all of them, not both:" +
				" run le site terminal-demo render name <demo-id>, or le site terminal-demo render all"))
		return nil, 2
	}
	return runRenderer(rendererRenderMode, true)
}

func runRenderOneIn(ctx context.Context, args leaction.Arguments) (any, int) {
	demoID := args.One(demoKeyword)
	if demoID == "" {
		leaction.ReportError(errors.New(
			"site terminal-demo render names one demo: le site terminal-demo render name <demo-id>" +
				" (the ids are in demos/terminal/manifest.json; render all records every one)"))
		return nil, 2
	}
	engine, release, err := renderEngine(ctx, true)
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
func renderEngine(ctx context.Context, releaseRequired bool) (*Engine, string, error) {
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
	return New(Options{Root: root, ArtifactRoot: artifactRoot, Context: ctx}), release, nil
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
