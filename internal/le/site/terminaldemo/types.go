// Design: docs/architecture/core-design.md -- the terminal-demo action area and its external renderer boundary
// Detail: actions.go -- native actions and their metadata.
// Detail: lock.go -- the cross-process artifact-tree lock
// Detail: manifest.go -- manifest contracts, digests, and artifact verification
// Detail: render.go -- Docker and media execution pipeline
//
// Package siteterminaldemo builds the binaries that terminal demonstrations drive,
// verifies published artifacts, and renders those artifacts through the pinned
// Docker renderer.
package siteterminaldemo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// HeroDemo selects the terminal recording shown on the homepage and linked by the README.
const HeroDemo = "showcase"

const (
	manifestSchema  = 2
	lockWaitDefault = 2 * time.Hour
	lockPollDefault = 500 * time.Millisecond
	// kernelCheckTimeout bounds the Docker host kernel check: the release
	// query and the probe container, whose own bound is two minutes.
	kernelCheckTimeout     = 3 * time.Minute
	renderSpeedup          = 5
	renderTypingSpeedMS    = 25
	rendererValidateMode   = "validate"
	rendererRenderMode     = "render"
	rendererImageBuildMode = "image-build"
	dockerProgram          = "docker"
	platformLabel          = "platform"
	sourceLabel            = "source"
	transcriptLabel        = "transcript"
	terminalLabel          = "terminal"
	textExtension          = ".txt"
	transcriptFilename     = transcriptLabel + textExtension
	dockerCapAddToken      = "--cap-add"
	dockerEnvironmentToken = "--env"
)

// The word this package drives the Go toolchain with, and the manifest key
// that happens to share a spelling with a Docker subcommand. Each names one
// thing, so a rename of one must not move the other: goCommandBuild is a Go
// toolchain subcommand, and imageLabel is a field of
// demos/terminal/manifest.json. Docker's own words live in interoplab.Docker.
const (
	goCommandBuild = "build"
	imageLabel     = "image"
)

// Command is one external renderer or media invocation. Args includes argv[0].
type Command struct {
	Args   []string  `json:"args"`
	Dir    string    `json:"dir"`
	Env    []string  `json:"env,omitempty"`
	Stdout io.Writer `json:"-"`
	Stderr io.Writer `json:"-"`
}

// Executor runs one external command and returns its process exit code.
// The renderer invokes the Go toolchain through it. Every Docker operation
// goes through interoplab.Docker instead.
type Executor func(Command) int

// PathLookup resolves a required host binary.
type PathLookup func(string) (string, error)

// KernelCheck refuses a Docker daemon whose kernel lacks a feature Ze enrolls,
// probing it with the linux ze at zePath.
type KernelCheck func(ctx context.Context, zePath string) error

// dockerKernelCheck is the production KernelCheck: the one check every Docker
// run that runs Ze makes (interoplab.DockerKernel).
func dockerKernelCheck(ctx context.Context, zePath string) error {
	return interoplab.DockerKernel(zePath)(ctx, interoplab.NewDocker())
}

// NetSysctlArguments answers the docker run arguments that let a demo
// container write its network namespace's sysctls on this daemon, or the
// refusal naming what the host must load first.
type NetSysctlArguments func(ctx context.Context) ([]string, error)

// dockerNetSysctlArguments is the production NetSysctlArguments: the same
// arguments every lab peer that writes net sysctls at run time takes
// (interoplab.Docker.NetSysctlSecurityArguments).
func dockerNetSysctlArguments(ctx context.Context) ([]string, error) {
	return interoplab.NewDocker().NetSysctlSecurityArguments(ctx)
}

// Options supplies the checkout, publish tree, process boundary, Docker kernel
// check, and lock timing for one Engine. Zero process, check and timing values
// select production behavior.
type Options struct {
	Root         string
	ArtifactRoot string
	Executor     Executor
	Lookup       PathLookup
	Output       io.Writer
	LockWait     time.Duration
	LockPoll     time.Duration
	KernelCheck  KernelCheck
	NetSysctl    NetSysctlArguments
	// Context is the run's root: the actions pass the signal context, so an
	// interrupt stops the demo container and removes it. Nil is a caller
	// that installs no signal root.
	Context context.Context
	// Docker runs every demo container (interoplab.Docker.RunOneShot). Nil
	// selects interoplab.NewDocker.
	Docker *interoplab.Docker
}

// Engine owns one invocation of the renderer pipeline. It is not safe for
// concurrent use. Separate processes serialize through lockPath.
type Engine struct {
	root                 string
	demoRoot             string
	artifactRoot         string
	artifactManifestPath string
	binaryPath           string
	lockPath             string
	execute              Executor
	lookup               PathLookup
	output               io.Writer
	lockWait             time.Duration
	lockPoll             time.Duration
	kernelCheck          KernelCheck
	netSysctl            NetSysctlArguments
	ctx                  context.Context
	docker               *interoplab.Docker
}

// New creates an Engine. Root and ArtifactRoot must be absolute or relative
// directory paths. The caller MUST keep the Engine in one goroutine.
func New(options Options) *Engine {
	output := options.Output
	if output == nil {
		output = os.Stderr
	}
	execute := options.Executor
	if execute == nil {
		execute = executeCommand
	}
	lookup := options.Lookup
	if lookup == nil {
		lookup = execLookPath
	}
	wait := options.LockWait
	if wait == 0 {
		wait = lockWaitDefault
	}
	poll := options.LockPoll
	if poll == 0 {
		poll = lockPollDefault
	}
	kernelCheck := options.KernelCheck
	if kernelCheck == nil {
		kernelCheck = dockerKernelCheck
	}
	netSysctl := options.NetSysctl
	if netSysctl == nil {
		netSysctl = dockerNetSysctlArguments
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	docker := options.Docker
	if docker == nil {
		docker = interoplab.NewDocker()
	}
	demoRoot := filepath.Join(options.Root, "demos", terminalLabel)
	return &Engine{
		root:                 options.Root,
		demoRoot:             demoRoot,
		artifactRoot:         options.ArtifactRoot,
		artifactManifestPath: filepath.Join(options.ArtifactRoot, "manifest.json"),
		binaryPath:           filepath.Join(options.Root, "tmp", "terminal-demos", "bin", "ze"),
		lockPath:             filepath.Join(options.Root, "tmp", "terminal-demos", "demo-run.lock"),
		execute:              execute,
		lookup:               lookup,
		output:               output,
		lockWait:             wait,
		lockPoll:             poll,
		kernelCheck:          kernelCheck,
		ctx:                  ctx,
		docker:               docker,
		netSysctl:            netSysctl,
	}
}

// Renderer is the immutable container identity in the source manifest.
type Renderer struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Image    string `json:"image"`
	Platform string `json:"platform"`
}

// Manifest is the checked-in definition of every demonstration.
type Manifest struct {
	Schema      int      `json:"schema"`
	Renderer    Renderer `json:"renderer"`
	GalleryPage string   `json:"gallery-page"`
	Demos       []Demo   `json:"demos"`
}

// Demo is one checked-in tape and its published artifact contract.
type Demo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Page        string `json:"page"`
	Anchor      string `json:"anchor"`
	Platform    string `json:"platform"`
	Kind        string `json:"kind"`
	Engine      string `json:"engine"`
	Source      string `json:"source"`
	Validate    string `json:"validate"`
	Duration    string `json:"duration,omitempty"`
	Privileged  *bool  `json:"privileged,omitempty"`
	// PrivilegedReason says what the demo does that its default grants
	// (NET_ADMIN, NET_RAW, SYS_ADMIN under Docker's default seccomp and
	// AppArmor profiles) refuse. It is required when Privileged is true and
	// refused otherwise.
	PrivilegedReason string `json:"privileged-reason,omitempty"`
	// NetSysctl says the demo writes its network namespace's sysctls under
	// /proc/sys/net while it runs, which Docker mounts read-only and
	// docker-default denies. The container then runs with systempaths
	// unconfined under the ze-lab-net-sysctl AppArmor profile, which admits
	// writes under net/ only, instead of --privileged. A privileged demo
	// already writes there, so the two are refused together.
	NetSysctl *bool `json:"net-sysctl,omitempty"`
	Realtime  *bool `json:"realtime,omitempty"`
}

func (d Demo) isPrivileged() bool { return d.Privileged != nil && *d.Privileged }

func (d Demo) isNetSysctl() bool { return d.NetSysctl != nil && *d.NetSysctl }

func (d Demo) isRealtime() bool { return d.Realtime != nil && *d.Realtime }

// AssetMetadata records one published file exactly as it exists on disk.
type AssetMetadata struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// ArtifactEntry binds one rendered demo to its release, inputs, binary, and
// published files.
type ArtifactEntry struct {
	Release          string                   `json:"release"`
	BinarySHA256     string                   `json:"binary-sha256"`
	SourceSHA256     string                   `json:"source-sha256"`
	DefinitionSHA256 string                   `json:"definition-sha256"`
	Assets           map[string]AssetMetadata `json:"assets"`
}

// artifactManifest is the generated publish-tree manifest.
type artifactManifest struct {
	Schema   int                      `json:"schema"`
	Renderer Renderer                 `json:"renderer"`
	Demos    map[string]ArtifactEntry `json:"demos"`
}

// Report is the structured answer of a check, validation, or render action.
type Report struct {
	Mode  string   `json:"mode"`
	Demos []string `json:"demos"`
}

// Text returns no additional prose. The pipeline streams the producer-compatible
// progress and verification lines while it runs.
func (Report) Text() string { return "" }
