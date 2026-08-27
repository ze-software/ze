// Design: docs/architecture/core-design.md -- the terminal-demo gate area and its external renderer boundary
// Detail: actions.go -- the six gate actions and their metadata
// Detail: lock.go -- the cross-process artifact-tree lock
// Detail: manifest.go -- manifest contracts, digests, and artifact verification
// Detail: render.go -- Docker and media execution pipeline
//
// Package terminaldemo builds the binaries that terminal demonstrations drive,
// verifies published artifacts, and renders those artifacts through the pinned
// Docker renderer.
package terminaldemo

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	manifestSchema         = 2
	lockWaitDefault        = 2 * time.Hour
	lockPollDefault        = 500 * time.Millisecond
	renderSpeedup          = 5
	renderTypingSpeedMS    = 25
	outputWidth            = 1680
	outputHeight           = 1008
	rendererValidateMode   = "validate"
	rendererRenderMode     = "render"
	platformLabel          = "platform"
	sourceLabel            = "source"
	transcriptLabel        = "transcript"
	terminalLabel          = "terminal"
	textExtension          = ".txt"
	transcriptFilename     = transcriptLabel + textExtension
	dockerCapAddToken      = "--cap-add"
	dockerEnvironmentToken = "--env"
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
// The renderer invokes Docker, ffmpeg, and the Go toolchain because those
// external implementations are the behavior these gates exercise.
type Executor func(Command) int

// PathLookup resolves a required host binary.
type PathLookup func(string) (string, error)

// Options supplies the checkout, publish tree, process boundary, and lock
// timing for one Engine. Zero process and timing values select production
// behavior.
type Options struct {
	Root         string
	ArtifactRoot string
	Executor     Executor
	Lookup       PathLookup
	Output       io.Writer
	LockWait     time.Duration
	LockPoll     time.Duration
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
	Realtime    *bool  `json:"realtime,omitempty"`
}

func (d Demo) isPrivileged() bool { return d.Privileged != nil && *d.Privileged }

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

// ArtifactManifest is the generated publish-tree manifest.
type ArtifactManifest struct {
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
