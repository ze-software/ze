// Design: docs/architecture/core-design.md -- Docker is the pinned terminal-demo renderer
// Overview: types.go -- pipeline configuration and artifact types
// Related: manifest.go -- artifact verification and digest contracts
// Detail: screen.go -- terminal repaint reconstruction for transcript checks

package siteterminaldemo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/le/tail"
)

var (
	sleepPattern            = regexp.MustCompile(`^Sleep (\d+(?:\.\d+)?)(ms|s|m)$`)
	transcriptPromptPattern = regexp.MustCompile(`^(\$|ze[>#])(?:\s+(\S.*?))?\s*$`)
)

type commandFailure struct {
	code int
	args []string
}

func (f commandFailure) Error() string {
	var buffer textbuf.Buffer
	return buffer.Str("command exited with code ").Int(int64(f.code)).Str(": ").Join(f.args, " ").String()
}

func execLookPath(name string) (string, error) { return exec.LookPath(name) }

func executeCommand(command Command) int {
	if len(command.Args) == 0 {
		return 127
	}
	process := exec.CommandContext(context.Background(), command.Args[0], command.Args[1:]...) // #nosec G204 -- Command.Args[0] is limited to this package's fixed ffmpeg and Go toolchain commands.
	process.Dir = command.Dir
	if command.Env != nil {
		process.Env = command.Env
	}
	process.Stdin = os.Stdin
	process.Stdout = command.Stdout
	if process.Stdout == nil {
		process.Stdout = os.Stdout
	}
	process.Stderr = command.Stderr
	if process.Stderr == nil {
		process.Stderr = os.Stderr
	}
	if err := process.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return exitErr.ExitCode()
		}
		return 127
	}
	return 0
}

// BuildImage builds the renderer image the manifest names, from
// demos/terminal/Dockerfile.
//
// The manifest's image tag is the only name for this image: renderDemo passes
// it to `docker run`, so building under any other tag records nothing. The tag
// carries the renderer's pinned versions, so a Dockerfile change that bumps one
// is a new tag and this action builds it beside the old one.
func (e *Engine) BuildImage() (Report, error) {
	report := Report{Mode: rendererImageBuildMode}
	manifest, _, err := e.loadManifest()
	if err != nil {
		return report, err
	}
	image := manifest.Renderer.Image
	if image == "" {
		return report, errors.New("demos/terminal/manifest.json names no renderer image")
	}
	if _, err := e.lookup(dockerProgram); err != nil {
		return report, errors.New("docker is required to build the terminal-demo renderer image")
	}
	var progress textbuf.Buffer
	io.WriteString(e.output, progress.Str("building ").Str(image).Str(" from demos/terminal/Dockerfile...\n").String()) //nolint:errcheck // progress output
	built, err := e.docker.Build(e.ctx, interoplab.ImageBuild{
		Name: "terminal-demo renderer", Tag: image,
		Dockerfile: filepath.Join(e.demoRoot, "Dockerfile"), Context: e.demoRoot,
	})
	if err != nil {
		return report, fmt.Errorf("build the renderer image %s: %w", image, err)
	}
	// Build tags a run-owned name beside image; the manifest's tag is the
	// one the recorder runs, so the run-owned one goes now.
	if err := e.docker.ReleaseImage(context.WithoutCancel(e.ctx), built); err != nil {
		return report, fmt.Errorf("release the build tag of %s: %w", image, err)
	}
	return report, nil
}

// requireRendererImage refuses a run whose renderer image is not built yet, and
// names the action that builds it.
//
// Docker's own answer to a missing image is to try a registry pull, and this
// image is published to none, so the run dies on "pull access denied" after
// every validator has already run. Nothing else builds the image: it comes from
// demos/terminal/Dockerfile through `le site terminal-demo image-build`.
func (e *Engine) requireRendererImage(manifest Manifest) error {
	image := manifest.Renderer.Image
	if _, err := e.lookup(dockerProgram); err != nil {
		return errors.New("docker is required to record a terminal demo")
	}
	present, err := e.docker.ImagePresent(e.ctx, image)
	if err != nil {
		return fmt.Errorf("check the renderer image %s: %w", image, err)
	}
	if !present {
		var buffer textbuf.Buffer
		return errors.New(buffer.Str("the renderer image ").Str(image).
			Str(" is not built on this host; run: le site terminal-demo image-build").String())
	}
	return nil
}

// externalCommand runs ffmpeg as a child of this process. Every Docker
// operation goes through e.docker instead. The child inherits an environment,
// never the operator's Docker context, so on macOS the daemon socket is named
// here (`internal/le/dockerhost`).
func (e *Engine) externalCommand(args []string, dir string) Command {
	return Command{Args: args, Dir: dir, Env: dockerhost.Inherited(), Stdout: e.output, Stderr: e.output}
}

// checkAll verifies all published artifacts. A non-empty release also verifies
// that every selected artifact carries that release identity.
func (e *Engine) checkAll(release string) (Report, error) {
	manifest, indexed, err := e.loadManifest()
	if err != nil {
		return Report{}, err
	}
	selected := manifestIDs(manifest)
	err = e.withLock(func() error {
		return e.verifyAssets(manifest, indexed, selected, release, false)
	})
	return Report{Mode: "check", Demos: selected}, err
}

// validationCheckAll runs every scenario validator in manifest order. It stops
// on the first failure and publishes no artifacts.
func (e *Engine) validationCheckAll() (Report, error) {
	manifest, indexed, err := e.loadManifest()
	if err != nil {
		return Report{}, err
	}
	selected := manifestIDs(manifest)
	if err := e.preflightContainers(manifest); err != nil {
		return Report{Mode: rendererValidateMode, Demos: selected}, err
	}
	for _, demoID := range selected {
		if err := e.runValidation(manifest, indexed[demoID]); err != nil {
			return Report{Mode: rendererValidateMode, Demos: selected}, err
		}
	}
	return Report{Mode: rendererValidateMode, Demos: selected}, nil
}

// RenderAll validates and renders all demos, then publishes one artifact
// manifest. Release must not be empty.
func (e *Engine) RenderAll(release string) (Report, error) {
	if release == "" {
		return Report{}, errors.New("a release identity is required when rendering")
	}
	manifest, indexed, err := e.loadManifest()
	if err != nil {
		return Report{}, err
	}
	return e.validateAndRender(manifest, indexed, manifestIDs(manifest), release)
}

// RenderOne validates and renders the one demo the manifest declares under
// demoID, then publishes the artifact manifest. Release must not be empty.
//
// It is the tight loop for a developer changing one tape. RenderAll runs the
// validator and the recorder in a container for every demo the manifest holds,
// so a one-line tape edit costs the whole gallery. The artifact manifest keeps
// the entries this run did not record (loadArtifactManifest), so a single-demo
// render publishes beside the others rather than replacing them.
//
// An unknown id is refused by name, with the ids the manifest declares. The
// alternative is a run that records nothing and reports a missing artifact
// much later.
func (e *Engine) RenderOne(demoID, release string) (Report, error) {
	if release == "" {
		return Report{}, errors.New("a release identity is required when rendering")
	}
	manifest, indexed, err := e.loadManifest()
	if err != nil {
		return Report{}, err
	}
	if _, declared := indexed[demoID]; !declared {
		var buffer textbuf.Buffer
		return Report{}, fmt.Errorf("unknown demo id %q; demos/terminal/manifest.json declares: %s",
			demoID, buffer.Join(manifestIDs(manifest), ", ").String())
	}
	return e.validateAndRender(manifest, indexed, []string{demoID}, release)
}

// validateAndRender is the body the whole-manifest and single-demo renders
// share: the container preflight, each selected demo's validators, then the
// recording of the selection under the lock.
func (e *Engine) validateAndRender(manifest Manifest, indexed map[string]Demo, selected []string, release string) (Report, error) {
	report := Report{Mode: rendererRenderMode, Demos: selected}
	if err := e.preflightContainers(manifest); err != nil {
		return report, err
	}
	for _, demoID := range selected {
		if err := e.runValidation(manifest, indexed[demoID]); err != nil {
			return report, err
		}
	}
	return report, e.withLock(func() error {
		return e.renderSelected(manifest, indexed, selected, release)
	})
}

// preflightContainers is what every path that starts a demo container owes
// first, validate and render alike: the demo binary exists, the Docker daemon's
// kernel carries every feature Ze enrolls, and the renderer image is present.
// The demos run Ze in containers on that kernel, so the check comes before any
// validator or recorder starts (owner D-4: a wrong kernel "should not be
// possible - fail").
func (e *Engine) preflightContainers(manifest Manifest) error {
	if !regularFile(e.binaryPath) {
		relative, relErr := filepath.Rel(e.root, e.binaryPath)
		if relErr != nil {
			relative = e.binaryPath
		}
		return fmt.Errorf("missing demo binary: %s", filepath.ToSlash(relative))
	}
	if err := e.checkKernel(); err != nil {
		return err
	}
	return e.requireRendererImage(manifest)
}

// checkKernel runs the Docker host kernel check with the demo binary, bounded
// by kernelCheckTimeout under the engine's root, so a Ctrl-C ends its docker
// client, which heads a process group of its own.
func (e *Engine) checkKernel() error {
	ctx, cancel := context.WithTimeout(e.ctx, kernelCheckTimeout)
	defer cancel()

	return e.kernelCheck(ctx, e.binaryPath)
}

func manifestIDs(manifest Manifest) []string {
	selected := make([]string, 0, len(manifest.Demos))
	for index := range manifest.Demos {
		selected = append(selected, manifest.Demos[index].ID)
	}
	return selected
}

func (e *Engine) runValidation(manifest Manifest, demo Demo) error {
	grants, err := e.demoGrants(&demo)
	if err != nil {
		return err
	}
	command := e.containerCommand(manifest.Renderer, grants, demoBinary("ze-demo"), commandValidate, demo.Validate)
	var buffer textbuf.Buffer
	e.output.Write(buffer.Str("validating ").Str(demo.ID).Str("...\n").Bytes()) //nolint:errcheck // CLI progress output cannot change the validation verdict.
	return e.withLock(func() error {
		return e.runDemoContainer(&demo, command)
	})
}

// demoStderrTailOctets bounds what runDemoContainer keeps of a net-sysctl
// demo's stderr: runc's AppArmor refusal is the last line docker writes.
const demoStderrTailOctets = 4096

// runDemoContainer runs one demo container. A net-sysctl demo's stderr is
// also kept in a bounded tail, because when the host's AppArmor profile list
// is unreadable (a non-root renderer) demoGrants cannot refuse first, and the
// daemon's failure to apply the profile surfaces only there; it is answered
// with the load command, never as a bare exit code.
func (e *Engine) runDemoContainer(demo *Demo, container interoplab.OneShotContainer) error {
	container.Timeout = demoContainerTimeout
	container.Stdin = os.Stdin
	container.Stdout = e.output
	container.Stderr = e.output
	var stderrTail *tail.Buffer
	if demo.isNetSysctl() {
		stderrTail = tail.New(demoStderrTailOctets)
		container.Stderr = io.MultiWriter(e.output, stderrTail)
	}
	result, err := e.docker.RunOneShot(e.ctx, container)
	if err == nil {
		return nil
	}
	if stderrTail != nil {
		if refusal := interoplab.NetSysctlUnapplied(result.ExitCode, stderrTail.String()); refusal != nil {
			return fmt.Errorf("terminal demo %s writes /proc/sys/net: %w", demo.ID, refusal)
		}
	}
	if stopped(err) {
		return fmt.Errorf("terminal demo %s: %w", demo.ID, err)
	}
	return commandFailure{code: result.ExitCode, args: append([]string{container.Image}, container.Command...)}
}

// stopped reports a demo container the root context or its bound stopped,
// rather than one whose command exited non-zero.
func stopped(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// demoContainerTimeout bounds one demo container. The longest render takes
// minutes; the bound is there so a hung browser cannot hold the render lock
// for ever, and an interrupt stops a run sooner through the root context.
const demoContainerTimeout = time.Hour

// containerCommand answers the container one demo step runs in.
//
// The container holds NET_ADMIN, NET_RAW and SYS_ADMIN and keeps Docker's
// default seccomp profile, which admits mount, unshare and setns once
// SYS_ADMIN is granted. grants (demoGrants) follow --rm and carry what the
// demo's manifest entry adds to that.
func (e *Engine) containerCommand(renderer Renderer, grants []string, entries ...string) interoplab.OneShotContainer {
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	scratchRoot := filepath.Join(e.root, "tmp", "terminal-demos")
	resolvedScratch := scratchRoot
	if resolved, err := filepath.EvalSymlinks(scratchRoot); err == nil {
		resolvedScratch = resolved
	}
	var buffer textbuf.Buffer
	uidEnv := buffer.Str("HOST_UID=").Str(uid).String()
	buffer.Reset()
	gidEnv := buffer.Str("HOST_GID=").Str(gid).String()
	buffer.Reset()
	rootVolume := buffer.Str(e.root).Str(":/src").String()
	buffer.Reset()
	artifactVolume := buffer.Str(e.artifactRoot).Str(":/src/demos/terminal/artifacts").String()
	var args []string
	if !strings.HasSuffix(renderer.Platform, "/native") {
		args = append(args, "--platform", renderer.Platform)
	}
	args = append(args, grants...)
	args = append(args,
		"--network", "none",
		"--hostname", demoInstance,
		dockerCapAddToken, "NET_ADMIN",
		dockerCapAddToken, "NET_RAW",
		dockerCapAddToken, "SYS_ADMIN",
		dockerEnvironmentToken, uidEnv,
		dockerEnvironmentToken, gidEnv,
		dockerEnvironmentToken, "ZE_DEMO_LOCK_HELD=1",
		"--volume", rootVolume,
	)
	if resolvedScratch != scratchRoot {
		buffer.Reset()
		scratchVolume := buffer.Str(resolvedScratch).Byte(':').Str(resolvedScratch).String()
		args = append(args, "--volume", scratchVolume)
	}
	args = append(args,
		"--volume", artifactVolume,
		"--workdir", "/src/demos/terminal",
	)
	return interoplab.OneShotContainer{Image: renderer.Image, Arguments: args, Command: entries}
}

// demoGrants answers the docker run arguments a demo takes beyond the default
// grants: --privileged for a privileged demo, systempaths unconfined under
// the net sysctl AppArmor profile for a net-sysctl demo, and none otherwise.
// A net-sysctl demo is refused on a host that has not loaded the profile.
func (e *Engine) demoGrants(demo *Demo) ([]string, error) {
	if demo.isPrivileged() {
		return []string{"--privileged"}, nil
	}
	if !demo.isNetSysctl() {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, kernelCheckTimeout)
	defer cancel()

	grants, err := e.netSysctl(ctx)
	if err != nil {
		return nil, fmt.Errorf("terminal demo %s writes /proc/sys/net: %w", demo.ID, err)
	}
	return grants, nil
}

func (e *Engine) acceleratedTerminalTape(demo Demo) (string, error) {
	data, err := readRootFile(e.demoRoot, demo.Source)
	if err != nil {
		return "", err
	}
	sourceLines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	lines := make([]string, 0, len(sourceLines)+1)
	configured := false
	for _, line := range sourceLines {
		if line == "Source common.tape" {
			lines = append(lines, line)
			var buffer textbuf.Buffer
			lines = append(lines, buffer.Str("Set TypingSpeed ").
				Int(renderTypingSpeedMS).Str("ms").String())
			configured = true
			continue
		}
		match := sleepPattern.FindStringSubmatch(line)
		if match == nil {
			lines = append(lines, line)
			continue
		}
		amount, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return "", err
		}
		units := float64(1)
		switch match[2] {
		case "s":
			units = 1000
		case "m":
			units = 60000
		}
		milliseconds := int64(math.RoundToEven(amount * units / renderSpeedup))
		milliseconds = max(milliseconds, 1)
		var buffer textbuf.Buffer
		lines = append(lines, buffer.Str("Sleep ").Int(milliseconds).Str("ms").String())
	}
	if !configured {
		return "", fmt.Errorf("%s: terminal tape does not source common.tape", demo.ID)
	}
	outputDir := filepath.Join(e.root, "tmp", "terminal-demos", "render-tapes")
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return "", err
	}
	var buffer textbuf.Buffer
	filename := buffer.Str(demo.ID).Str(".tape").String()
	output := filepath.Join(outputDir, filename)
	data = append([]byte(strings.Join(lines, "\n")), '\n')
	if err := os.WriteFile(output, data, 0o600); err != nil {
		return "", err
	}
	return output, nil
}

func captureSpeedup(demo Demo) int {
	if demo.isRealtime() {
		return 1
	}
	return renderSpeedup
}

func (e *Engine) renderTape(demo Demo) (string, error) {
	if demo.isRealtime() {
		return filepath.Join(e.demoRoot, demo.Source), nil
	}
	return e.acceleratedTerminalTape(demo)
}

func (e *Engine) renderSelected(manifest Manifest, indexed map[string]Demo, selected []string, release string) error {
	generated, err := e.loadArtifactManifest(manifest)
	if err != nil {
		return err
	}
	generated.Renderer = manifest.Renderer
	for _, demoID := range selected {
		entry, err := e.renderDemo(manifest, indexed[demoID], release)
		if err != nil {
			return err
		}
		generated.Demos[demoID] = entry
	}
	if err := e.writeArtifactManifest(generated); err != nil {
		return err
	}
	var buffer textbuf.Buffer
	for _, demoID := range selected {
		removed, err := e.removeSupersededAssets(indexed[demoID])
		if err != nil {
			return err
		}
		for _, path := range removed {
			if _, err := e.output.Write(buffer.Reset().Str("removed superseded artifact: ").
				Str(filepath.Base(path)).Byte('\n').Bytes()); err != nil {
				return err
			}
		}
	}
	return e.verifyAssets(manifest, indexed, selected, release, false)
}

func (e *Engine) renderDemo(manifest Manifest, demo Demo, release string) (ArtifactEntry, error) {
	sourcePath := filepath.Join(e.demoRoot, demo.Source)
	speedup := captureSpeedup(demo)
	renderSource := sourcePath
	var err error
	if demo.Kind == terminalLabel {
		renderSource, err = e.renderTape(demo)
		if err != nil {
			return ArtifactEntry{}, err
		}
	}
	if renderSource != sourcePath {
		defer os.Remove(renderSource) //nolint:errcheck // The generated tape is best-effort cleanup after the primary verdict.
	}
	if err := os.MkdirAll(e.artifactRoot, 0o755); err != nil { // #nosec G301 -- Public website artifacts must be traversable by the web server.
		return ArtifactEntry{}, err
	}
	expected, err := e.assetPaths(demo)
	if err != nil {
		return ArtifactEntry{}, err
	}
	names, err := assetNames(demo.Kind)
	if err != nil {
		return ArtifactEntry{}, err
	}
	videoPath := expected["video"]
	castPath := expected["cast"]
	if videoPath != "" {
		if _, err := e.lookup("ffmpeg"); err != nil {
			return ArtifactEntry{}, fmt.Errorf("%s: ffmpeg is required on this host to rescale a %s demo's video and poster; install it and re-run", demo.ID, demo.Kind)
		}
	}
	for _, name := range names {
		if name == transcriptLabel {
			continue
		}
		path := expected[name]
		if err := os.Remove(path); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return ArtifactEntry{}, err
			}
		}
	}
	relativeSource, err := filepath.Rel(e.root, renderSource)
	if err != nil {
		return ArtifactEntry{}, err
	}
	containerSource := path.Join("/src", filepath.ToSlash(relativeSource))
	grants, err := e.demoGrants(&demo)
	if err != nil {
		return ArtifactEntry{}, err
	}
	command := e.containerCommand(manifest.Renderer, grants, containerSource)
	var buffer textbuf.Buffer
	releaseEnv := buffer.Str("ZE_DEMO_RELEASE=").Str(release).String()
	buffer.Reset()
	speedEnv := buffer.Str("ZE_DEMO_SPEEDUP=").Int(int64(speedup)).String()
	command.Arguments = append(command.Arguments,
		dockerEnvironmentToken, releaseEnv,
		dockerEnvironmentToken, speedEnv,
	)
	e.output.Write(buffer.Reset().Str("rendering ").Str(demo.ID).Str("...\n").Bytes()) //nolint:errcheck // CLI progress output cannot change the render verdict.
	if err := e.runDemoContainer(&demo, command); err != nil {
		return ArtifactEntry{}, err
	}
	if videoPath != "" {
		if err := e.expandTimeline(videoPath, speedup); err != nil {
			return ArtifactEntry{}, err
		}
		if err := e.resizePoster(expected["poster"]); err != nil {
			return ArtifactEntry{}, err
		}
	}
	if castPath != "" {
		if err := expandCastTimeline(castPath, speedup); err != nil {
			return ArtifactEntry{}, err
		}
	}
	transcriptSource := filepath.Join(filepath.Dir(sourcePath), transcriptFilename)
	if castPath != "" {
		if err := checkTranscript(demo.ID, castPath, transcriptSource); err != nil {
			rejected := filepath.Join(e.root, "tmp", "terminal-demos", "rejected", filepath.Base(castPath))
			if mkdirErr := os.MkdirAll(filepath.Dir(rejected), 0o750); mkdirErr != nil {
				return ArtifactEntry{}, mkdirErr
			}
			if renameErr := os.Rename(castPath, rejected); renameErr != nil {
				return ArtifactEntry{}, renameErr
			}
			return ArtifactEntry{}, err
		}
	}
	if err := copyFile(transcriptSource, expected[transcriptLabel]); err != nil {
		return ArtifactEntry{}, err
	}
	assets := make(map[string]AssetMetadata, len(expected))
	for _, name := range names {
		path := expected[name]
		info, err := os.Stat(path)
		if err != nil {
			return ArtifactEntry{}, fmt.Errorf("%s: missing generated %s: %s", demo.ID, name, path)
		}
		if info.Size() == 0 {
			return ArtifactEntry{}, fmt.Errorf("%s: missing generated %s: %s", demo.ID, name, path)
		}
		digest, err := hashFile(path)
		if err != nil {
			return ArtifactEntry{}, err
		}
		relative, err := filepath.Rel(e.artifactRoot, path)
		if err != nil {
			return ArtifactEntry{}, err
		}
		assets[name] = AssetMetadata{Path: filepath.ToSlash(relative), Bytes: info.Size(), SHA256: digest}
	}
	binaryDigest, err := hashFile(e.binaryPath)
	if err != nil {
		return ArtifactEntry{}, err
	}
	sourceDigest, err := e.sourceDigest(demo)
	if err != nil {
		return ArtifactEntry{}, err
	}
	definitionDigest, err := e.definitionDigest(demo)
	if err != nil {
		return ArtifactEntry{}, err
	}
	return ArtifactEntry{
		Release:          release,
		BinarySHA256:     binaryDigest,
		SourceSHA256:     sourceDigest,
		DefinitionSHA256: definitionDigest,
		Assets:           assets,
	}, nil
}

func (e *Engine) removeSupersededAssets(demo Demo) ([]string, error) {
	keepPaths, err := e.assetPaths(demo)
	if err != nil {
		return nil, err
	}
	keep := make(map[string]struct{}, len(keepPaths))
	for _, path := range keepPaths {
		keep[filepath.Ext(path)] = struct{}{}
	}
	all := []string{".cast", textExtension, ".webm", ".png"}
	removed := make([]string, 0)
	for _, extension := range all {
		if _, exists := keep[extension]; exists {
			continue
		}
		var buffer textbuf.Buffer
		filename := buffer.Str(demo.ID).Str(extension).String()
		path := filepath.Join(e.artifactRoot, filename)
		if !regularFile(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
		removed = append(removed, path)
	}
	slices.Sort(removed)
	return removed, nil
}

func (e *Engine) expandTimeline(capture string, speedup int) error {
	compressed := siblingWithMarker(capture, ".fast")
	if err := os.Rename(capture, compressed); err != nil {
		return err
	}
	args := []string{
		"ffmpeg", "-y", "-loglevel", "error", "-itsscale", strconv.Itoa(speedup),
		"-i", compressed, "-map", "0:v:0", "-an",
		"-vf", outputScale(),
		"-c:v", "libvpx-vp9", "-deadline", "realtime", "-cpu-used", "8",
		"-crf", "30", "-b:v", "0", "-row-mt", "1", "-tile-columns", "2",
		"-fps_mode", "passthrough", capture,
	}
	if code := e.execute(e.externalCommand(args, "")); code != 0 {
		removeErr := os.Remove(capture)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		if err := os.Rename(compressed, capture); err != nil {
			return errors.Join(err, removeErr, commandFailure{code: code, args: args})
		}
		return errors.Join(commandFailure{code: code, args: args}, removeErr)
	}
	return os.Remove(compressed)
}

func (e *Engine) resizePoster(poster string) error {
	original := siblingWithMarker(poster, ".original")
	if err := os.Rename(poster, original); err != nil {
		return err
	}
	args := []string{
		"ffmpeg", "-y", "-loglevel", "error", "-i", original,
		"-vf", outputScale(), "-frames:v", "1", poster,
	}
	if code := e.execute(e.externalCommand(args, "")); code != 0 {
		removeErr := os.Remove(poster)
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
		if err := os.Rename(original, poster); err != nil {
			return errors.Join(err, removeErr, commandFailure{code: code, args: args})
		}
		return errors.Join(commandFailure{code: code, args: args}, removeErr)
	}
	return os.Remove(original)
}

func siblingWithMarker(path, marker string) string {
	extension := filepath.Ext(path)
	var buffer textbuf.Buffer
	return buffer.Str(strings.TrimSuffix(path, extension)).Str(marker).Str(extension).String()
}

func outputScale() string {
	var buffer textbuf.Buffer
	return buffer.Str("scale=").Int(outputWidth).Byte(':').Int(outputHeight).
		Str(":flags=lanczos").String()
}

func expandCastTimeline(path string, speedup int) error {
	if speedup == 1 {
		return nil
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck // The artifact update owns the write verdict.
	name := filepath.Base(path)
	data, err := root.ReadFile(name)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return fmt.Errorf("%s: the recorder wrote no header", path)
	}
	expanded := make([]string, 1, len(lines))
	expanded[0] = lines[0]
	for index, line := range lines[1:] {
		var event []any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		if len(event) == 0 {
			return fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		timestamp, ok := event[0].(float64)
		if !ok {
			return fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		event[0] = math.Round(timestamp*float64(speedup)*1_000_000) / 1_000_000
		encoded, err := artifactJSON(event)
		if err != nil {
			return err
		}
		expanded = append(expanded, encoded)
	}
	data = append([]byte(strings.Join(expanded, "\n")), '\n')
	return root.WriteFile(name, data, 0o644) // #nosec G306 -- The asciicast is a public website artifact, not private data.
}

func artifactJSON(value any) (string, error) {
	var buffer textbuf.Buffer
	if err := appendArtifactJSON(&buffer, value); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func appendArtifactJSON(buffer *textbuf.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		buffer.Str("null")
	case bool:
		buffer.Bool(typed)
	case string:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return err
		}
		buffer.Str(string(encoded))
	case float64:
		text := strconv.FormatFloat(typed, 'g', -1, 64)
		buffer.Str(text)
		if !strings.ContainsAny(text, ".eE") {
			buffer.Str(".0")
		}
	case []any:
		buffer.Byte('[')
		for index, item := range typed {
			if index > 0 {
				buffer.Str(", ")
			}
			if err := appendArtifactJSON(buffer, item); err != nil {
				return err
			}
		}
		buffer.Byte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		buffer.Byte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.Str(", ")
			}
			encoded, err := json.Marshal(key)
			if err != nil {
				return err
			}
			buffer.Str(string(encoded)).Str(": ")
			if err := appendArtifactJSON(buffer, typed[key]); err != nil {
				return err
			}
		}
		buffer.Byte('}')
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	return nil
}

type transcriptCommand struct {
	line    int
	prompt  string
	command string
}

func transcriptCommands(path string) ([]transcriptCommand, error) {
	data, err := readRootFile(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	commands := make([]transcriptCommand, 0)
	for index, line := range lines {
		match := transcriptPromptPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		commands = append(commands, transcriptCommand{line: index + 1, prompt: match[1], command: match[2]})
	}
	return commands, nil
}

func castVisibleText(path string) ([]string, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // The cast read owns the verdict.
	handle, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer handle.Close() //nolint:errcheck // Scanner errors own the read verdict.
	scanner := bufio.NewScanner(handle)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s: the recorder wrote no header", path)
	}
	var header struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		return nil, err
	}
	screen := newTerminalScreen(header.Height, header.Width)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var event []any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		if len(event) != 3 {
			continue
		}
		kind, kindOK := event[1].(string)
		burst, burstOK := event[2].(string)
		if kindOK && burstOK && kind == "o" {
			screen.settle(burst)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return screen.painted(), nil
}

func checkTranscript(demoID, castPath, transcriptPath string) error {
	claimed, err := transcriptCommands(transcriptPath)
	if err != nil {
		return err
	}
	if len(claimed) == 0 {
		return fmt.Errorf("%s: %s quotes no command line, so it gates nothing", demoID, transcriptPath)
	}
	visible, err := castVisibleText(castPath)
	if err != nil {
		return err
	}
	at := 0
	for matched, claim := range claimed {
		foundLine := -1
		for index := at; index < len(visible); index++ {
			promptAt := strings.Index(visible[index], claim.prompt)
			if promptAt < 0 {
				continue
			}
			if claim.command != "" {
				commandAt := strings.Index(visible[index][promptAt+len(claim.prompt):], claim.command)
				if commandAt < 0 {
					continue
				}
			}
			foundLine = index
			break
		}
		if foundLine >= 0 {
			at = foundLine + 1
			continue
		}
		var buffer textbuf.Buffer
		quoted := strings.TrimRight(buffer.Str(claim.prompt).Byte(' ').Str(claim.command).String(), " ")
		return fmt.Errorf("%s: the recording does not show what %s:%d claims: %q. %d earlier lines matched, the search reached painted line %d of %d", demoID, transcriptPath, claim.line, quoted, matched, at, len(visible))
	}
	return nil
}

func copyFile(source, target string) error {
	sourceRoot, err := os.OpenRoot(filepath.Dir(source))
	if err != nil {
		return err
	}
	defer sourceRoot.Close() //nolint:errcheck // The copy owns the source read verdict.
	input, err := sourceRoot.Open(filepath.Base(source))
	if err != nil {
		return err
	}
	defer input.Close() //nolint:errcheck // Copy owns the read verdict.
	targetRoot, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer targetRoot.Close() //nolint:errcheck // Closing the output file owns the write verdict.
	output, err := targetRoot.Create(filepath.Base(target))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
