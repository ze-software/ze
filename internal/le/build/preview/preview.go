// Design: docs/contributing/preview-artifacts.md -- prepared, unpublished bundles.
package buildpreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
)

// A preview targets Linux only, on the two architectures Ze ships. The values
// are Go's GOOS and GOARCH names, which the OCI platform fields also use.
const (
	targetOS  = "linux"
	archAMD64 = "amd64"
	archARM64 = "arm64"
)

// Artifact is one independently checksummed, downloadable file.
type Artifact struct {
	File   string `json:"file"`
	Kind   string `json:"kind"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	SHA256 string `json:"sha256"`
}

// Manifest describes prepared files; it is never a publication receipt.
type Manifest struct {
	Schema    int        `json:"schema"`
	Channel   string     `json:"channel"`
	Published bool       `json:"published"`
	Version   string     `json:"version"`
	Source    Source     `json:"source"`
	Artifacts []Artifact `json:"artifacts"`
	Images    []Image    `json:"images,omitempty"`
}

type dependencies struct {
	source  func(context.Context, string) (Source, error)
	build   func(context.Context, string, string, string, string, string) error
	inspect func(string, string, Source, string) error
	images  func(context.Context, string, string, string, Source) ([]Artifact, []Image, error)
}

func realDependencies() dependencies {
	return dependencies{source: captureSource, build: buildcompile.BuildDaemon, inspect: inspectBinary, images: prepareImages}
}

func previewVersion(source Source) string {
	version := "preview-" + source.Commit[:12] + "-" + source.Digest[:12]
	if source.Dirty {
		version += "-dirty"
	}
	return version
}

// prepare owns a directory created exclusively for this request. A failed
// request removes only that directory; no existing output is overwritten.
func prepare(ctx context.Context, root, output string, images bool, deps dependencies) (manifest Manifest, err error) {
	output, err = filepath.Abs(output)
	if err != nil {
		return manifest, err
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return manifest, fmt.Errorf("preview output must be a new directory: %s", output)
	}
	source, err := deps.source(ctx, root)
	if err != nil {
		return manifest, err
	}
	if len(source.Commit) != 40 || len(source.Digest) != 64 {
		return manifest, fmt.Errorf("preview source identity is incomplete")
	}
	version := previewVersion(source)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil { //nolint:gosec // G301: parent directories this call creates for the published bundle are world-readable, as a download location must be
		return manifest, err
	}
	if err := os.Mkdir(output, 0o755); err != nil { //nolint:gosec // G301: the bundle directory is a published download
		return manifest, err
	}
	complete := false
	defer func() {
		if !complete {
			err = joinCleanup(err, os.RemoveAll(output))
		}
	}()
	manifest = Manifest{Schema: 1, Channel: "preview", Published: false, Version: version, Source: source}
	for _, arch := range []string{archAMD64, archARM64} {
		name := "ze-" + version + "-linux-" + arch
		binary := filepath.Join(output, name)
		if err := deps.build(ctx, root, arch, binary, version, source.BuildDate); err != nil {
			return manifest, err
		}
		if err := deps.inspect(binary, arch, source, version); err != nil {
			return manifest, err
		}
		archive := name + ".tar.gz"
		if err := writeArchive(root, binary, filepath.Join(output, archive), source); err != nil {
			return manifest, err
		}
		for _, entry := range []struct{ name, kind string }{{name, "binary"}, {archive, "archive"}} {
			sum, err := fileSum(filepath.Join(output, entry.name))
			if err != nil {
				return manifest, err
			}
			manifest.Artifacts = append(manifest.Artifacts, Artifact{File: entry.name, Kind: entry.kind, OS: targetOS, Arch: arch, SHA256: sum})
		}
	}
	after, err := deps.source(ctx, root)
	if err != nil {
		return manifest, err
	}
	// Provenance records the starting checkout. A concurrent commit or our
	// output may move HEAD/status without changing any selected input.
	// Validate after the last compiler/archive source read. Later image work
	// consumes completed outputs and checks its own staged runtime recipe.
	// Content identity, population, tags and toolchain MUST remain identical
	// here; the version and build date stay pinned to the original capture.
	after.Commit = source.Commit
	after.Dirty = source.Dirty
	after.BuildDate = source.BuildDate
	if !reflect.DeepEqual(source, after) {
		changed := "toolchain, tags or input digest"
		for index := range max(len(source.Inputs), len(after.Inputs)) {
			if index >= len(source.Inputs) {
				changed = after.Inputs[index].Path
				break
			}
			if index >= len(after.Inputs) {
				changed = source.Inputs[index].Path
				break
			}
			if source.Inputs[index] != after.Inputs[index] {
				changed = source.Inputs[index].Path
				if source.Inputs[index].Path != after.Inputs[index].Path {
					changed += " / " + after.Inputs[index].Path
				}
				break
			}
		}
		return manifest, fmt.Errorf("preview source inputs changed while preparing binaries and archives (%s); resolve this change before preparing another bundle", changed)
	}
	if images {
		artifacts, records, err := deps.images(ctx, root, output, version, source)
		if err != nil {
			return manifest, err
		}
		manifest.Artifacts = append(manifest.Artifacts, artifacts...)
		manifest.Images = records
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	if err := os.WriteFile(filepath.Join(output, "manifest.json"), append(body, '\n'), 0o644); err != nil { //nolint:gosec // G306: the manifest is part of a published download
		return manifest, err
	}
	var sums strings.Builder
	for _, artifact := range manifest.Artifacts {
		fmt.Fprintf(&sums, "%s  %s\n", artifact.SHA256, artifact.File)
	}
	sum, err := fileSum(filepath.Join(output, "manifest.json"))
	if err != nil {
		return manifest, err
	}
	fmt.Fprintf(&sums, "%s  manifest.json\n", sum)
	if err := os.WriteFile(filepath.Join(output, "SHA256SUMS"), []byte(sums.String()), 0o644); err != nil { //nolint:gosec // G306: the checksum list is part of a published download
		return manifest, err
	}
	if _, err := check(output, deps.inspect); err != nil {
		return manifest, err
	}
	complete = true
	return manifest, nil
}

func joinCleanup(cause, cleanup error) error {
	if cleanup != nil {
		return fmt.Errorf("%w; remove incomplete preview: %w", cause, cleanup)
	}
	return cause
}
