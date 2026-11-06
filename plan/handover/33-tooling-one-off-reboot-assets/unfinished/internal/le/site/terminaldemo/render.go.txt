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
	process := exec.CommandContext(context.Background(), command.Args[0], command.Args[1:]...) // #nosec G204 -- Command.Args[0] is limited to this package's fixed Go toolchain commands.
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
// The manifest's tag is the recorder's name for this image. Peer release build
// arguments come from the canonical interop pins, so a pin change invalidates
// Docker's peer layers without changing recorder, browser or font bindings.
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
	arguments, err := rendererPeerArguments(e.root)
	if err != nil {
		return report, err
	}
	var progress textbuf.Buffer
	io.WriteString(e.output, progress.Str("building ").Str(image).Str(" from demos/terminal/Dockerfile...\n").String()) //nolint:errcheck // progress output
	built, err := e.docker.Build(e.ctx, interoplab.ImageBuild{
		Name: "terminal-demo renderer", Tag: image,
		Dockerfile: filepath.Join(e.demoRoot, "Dockerfile"), Context: e.demoRoot,
		BuildArgs: arguments, Progress: e.output,
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

// requireRendererImage refuses a run whose renderer image is not built yet, or
// was built from peer pins the checkout no longer declares, and names the
// action that builds it.
//
// Docker's own answer to a missing image is to try a registry pull, and this
// image is published to none, so the run dies on "pull access denied" after
// every validator has already run. Nothing else builds the image: it comes from
// demos/terminal/Dockerfile through `le site terminal-demo build image`. The
// manifest tag does not change when a peer pin does, so an image built before a
// pin bump still answers to it, and only its labels tell the two apart.
func (e *Engine) requireRendererImage(manifest Manifest) error {
	image := manifest.Renderer.Image
	if _, err := e.lookup(dockerProgram); err != nil {
		return errors.New("docker is required to record a terminal demo")
	}
	inspection, err := e.docker.InspectImage(e.ctx, image)
	if errors.Is(err, interoplab.ErrImageAbsent) {
		var buffer textbuf.Buffer
		return errors.New(buffer.Str("the renderer image ").Str(image).
			Str(" is not built on this host; run: le site terminal-demo build image").String())
	}
	if err != nil {
		return fmt.Errorf("check the renderer image %s: %w", image, err)
	}
	return e.requireRendererPins(image, inspection.Labels)
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
	return Report{Mode: rendererCheckMode, Demos: selected}, err
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
// with the fix for this client (the load command on Linux, the QEMU
// docker-lab route elsewhere), never as a bare exit code.
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
		if refusal := e.docker.NetSysctlUnapplied(result.ExitCode, stderrTail.String()); refusal != nil {
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
	if err := e.retireDeletedAssets(&generated, indexed); err != nil {
		return err
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
	scratch := filepath.Join(e.root, "tmp", "terminal-demos")
	if err := os.MkdirAll(scratch, 0o750); err != nil {
		return ArtifactEntry{}, err
	}
	stage, err := os.MkdirTemp(scratch, "artifact-")
	if err != nil {
		return ArtifactEntry{}, err
	}
	defer os.RemoveAll(stage) //nolint:errcheck // Staging cleanup cannot replace the publication verdict.
	// Only the completed container writes this private stage. All subsequent
	// checks and publishers consume retained bytes, not its mutable file names.
	recorder := *e
	recorder.artifactRoot = stage
	expected, err := recorder.assetPaths(demo)
	if err != nil {
		return ArtifactEntry{}, err
	}
	names, err := assetNames(demo.Kind)
	if err != nil {
		return ArtifactEntry{}, err
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
	command := recorder.containerCommand(manifest.Renderer, grants, containerSource)
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
	payloads := make(map[string]Asset, len(names))
	for _, name := range names {
		if name == transcriptLabel {
			continue
		}
		asset, err := acquireAsset(stage, filepath.Base(expected[name]), nil)
		if err != nil {
			return ArtifactEntry{}, fmt.Errorf("%s: acquire generated %s: %w", demo.ID, name, err)
		}
		if name == castLabel {
			asset, err = expandCastTimeline(asset, speedup)
			if err != nil {
				return ArtifactEntry{}, err
			}
		}
		payloads[name] = asset
	}
	transcript, err := acquireAsset(e.demoRoot, filepath.Join(filepath.Dir(demo.Source), transcriptFilename), nil)
	if err != nil {
		return ArtifactEntry{}, err
	}
	payloads[transcriptLabel] = transcript
	if cast, recorded := payloads[castLabel]; recorded {
		if err := checkTranscript(demo.ID, cast, transcript); err != nil {
			rejected := filepath.Join(scratch, "rejected")
			if mkdirErr := os.MkdirAll(rejected, 0o750); mkdirErr != nil {
				return ArtifactEntry{}, mkdirErr
			}
			if copyErr := copyFile(cast, filepath.Join(rejected, cast.Metadata().Path)); copyErr != nil {
				return ArtifactEntry{}, copyErr
			}
			return ArtifactEntry{}, err
		}
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
	if err := os.MkdirAll(e.artifactRoot, 0o755); err != nil {
		return ArtifactEntry{}, err
	}
	root, err := os.OpenRoot(e.artifactRoot)
	if err != nil {
		return ArtifactEntry{}, err
	}
	defer root.Close() //nolint:errcheck // Each per-file publication owns its write verdict.
	assets := make(map[string]AssetMetadata, len(names))
	for _, name := range names {
		asset := payloads[name]
		relative := filepath.Base(expected[name])
		if err := asset.Publish(root, relative); err != nil {
			return ArtifactEntry{}, err
		}
		metadata := asset.Metadata()
		metadata.Path = relative
		assets[name] = metadata
	}
	return ArtifactEntry{
		Release: release, BinarySHA256: binaryDigest,
		SourceSHA256: sourceDigest, DefinitionSHA256: definitionDigest, Assets: assets,
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
	all := []string{castExtension, textExtension, posterExtension, ".webm", ".mp4", ".png"}
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

func expandCastTimeline(asset Asset, speedup int) (Asset, error) {
	if speedup == 1 {
		return asset, nil
	}
	content, err := asset.Text()
	if err != nil {
		return Asset{}, err
	}
	path := asset.Metadata().Path
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return Asset{}, fmt.Errorf("%s: the recorder wrote no header", path)
	}
	expanded := make([]string, 1, len(lines))
	expanded[0] = lines[0]
	for index, line := range lines[1:] {
		var event []any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return Asset{}, fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		if len(event) == 0 {
			return Asset{}, fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		timestamp, ok := event[0].(float64)
		if !ok {
			return Asset{}, fmt.Errorf("%s:%d: not an asciicast event", path, index+2)
		}
		event[0] = math.Round(timestamp*float64(speedup)*1_000_000) / 1_000_000
		encoded, err := artifactJSON(event)
		if err != nil {
			return Asset{}, err
		}
		expanded = append(expanded, encoded)
	}
	return assetFromText(path, strings.Join(expanded, "\n")+"\n"), nil
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

func transcriptCommands(asset Asset) ([]transcriptCommand, error) {
	data, err := asset.Text()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(data, "\n")
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

func castVisibleText(asset Asset) ([]string, error) {
	content, err := asset.Text()
	if err != nil {
		return nil, err
	}
	path := asset.Metadata().Path
	scanner := bufio.NewScanner(strings.NewReader(content))
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

func checkTranscript(demoID string, cast, transcript Asset) error {
	transcriptPath := transcript.Metadata().Path
	claimed, err := transcriptCommands(transcript)
	if err != nil {
		return err
	}
	if len(claimed) == 0 {
		return fmt.Errorf("%s: %s quotes no command line, so it gates nothing", demoID, transcriptPath)
	}
	visible, err := castVisibleText(cast)
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

func copyFile(source Asset, target string) error {
	targetRoot, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer targetRoot.Close() //nolint:errcheck // Asset.Publish owns the write verdict.
	return source.Publish(targetRoot, filepath.Base(target))
}
