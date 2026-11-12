// Design: docs/contributing/preview-artifacts.md -- local immutable images.
package buildpreview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Image records a local image and its portable export. Architecture describes
// the Docker server's native build, never a cross-platform publication.
type Image struct {
	Role          string   `json:"role"`
	Tag           string   `json:"tag"`
	ID            string   `json:"id"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	File          string   `json:"file"`
	Tags          []string `json:"build-tags"`
	VersionOutput string   `json:"version-output"`
}

func prepareImages(ctx context.Context, root, output, version string, source Source) ([]Artifact, []Image, error) {
	return prepareImagesWithDocker(ctx, root, output, version, source, interoplab.NewBuildToolDocker())
}

func prepareImagesWithDocker(ctx context.Context, root, output, version string, source Source, docker *interoplab.Docker) (artifacts []Artifact, images []Image, err error) {
	index := slices.IndexFunc(source.Inputs, func(input Input) bool { return input.Path == "docker/Dockerfile.runtime" })
	if index < 0 {
		return nil, nil, errors.New("runtime recipe docker/Dockerfile.runtime is missing from the captured source inputs; prepare a new bundle from the complete checkout")
	}
	arch, err := docker.ServerArchitecture(ctx)
	if err != nil {
		return nil, nil, err
	}
	if arch != archAMD64 && arch != archARM64 {
		return nil, nil, fmt.Errorf("preview image target %q is not supported", arch)
	}
	work, err := os.MkdirTemp(output, ".runtime-images-")
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(work)) }()
	binary := filepath.Join(output, "ze-"+version+"-linux-"+arch)
	for _, recipe := range []struct{ role, executable string }{{"deployment", "/ze"}, {"lab", "/usr/local/bin/ze"}} {
		tag := "ze-preview-" + recipe.role + ":" + version
		build, err := interoplab.StageRuntimeImage(root, filepath.Join(work, recipe.role), binary, recipe.role)
		if err != nil {
			return nil, nil, err
		}
		sum, err := fileSum(build.Dockerfile)
		if err != nil {
			return nil, nil, err
		}
		if sum != source.Inputs[index].SHA256 {
			return nil, nil, fmt.Errorf("%s runtime recipe differs from captured docker/Dockerfile.runtime; resolve the recipe change before preparing another bundle", recipe.role)
		}
		build.Name = "preview-" + recipe.role
		build.Tag = tag
		build.Progress = os.Stderr
		built, err := docker.Build(ctx, build)
		if err != nil {
			return nil, nil, err
		}
		artifact, record, exportErr := exportImage(ctx, docker, built, recipe.role, recipe.executable, arch, output, version, source)
		// Release only this run's retained tag. The identified local preview tag
		// remains usable, even if a later export fails.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		releaseErr := docker.ReleaseImage(cleanupCtx, built)
		cancel()
		if err := errors.Join(exportErr, releaseErr); err != nil {
			return nil, nil, err
		}
		artifacts = append(artifacts, artifact)
		images = append(images, record)
	}
	return artifacts, images, nil
}

func exportImage(ctx context.Context, docker *interoplab.Docker, built interoplab.ImageResult, role, executable, arch, output, version string, source Source) (Artifact, Image, error) {
	inspection, err := docker.InspectImage(ctx, built.Reference)
	if err != nil {
		return Artifact{}, Image{}, err
	}
	if inspection.ID != built.Reference {
		return Artifact{}, Image{}, fmt.Errorf("preview image identity moved: %s", built.Reference)
	}
	result, err := docker.RunOneShot(ctx, interoplab.OneShotContainer{Image: inspection.ID, Arguments: []string{"--entrypoint", executable}, Command: []string{"version", "--extended"}, Timeout: time.Minute})
	if err != nil {
		return Artifact{}, Image{}, err
	}
	for _, required := range []string{version, source.BuildDate, "go:       " + source.Toolchain, "os/arch:  linux/" + arch, "cgo:      disabled"} {
		if !strings.Contains(result.Stdout, required) {
			return Artifact{}, Image{}, fmt.Errorf("%s image does not report %q: %s", role, required, result.Stdout)
		}
	}
	file := "ze-" + version + "-" + role + "-linux-" + arch + ".docker.tar"
	if err := docker.SaveImage(ctx, inspection.ID, filepath.Join(output, file)); err != nil {
		return Artifact{}, Image{}, err
	}
	sum, err := fileSum(filepath.Join(output, file))
	if err != nil {
		return Artifact{}, Image{}, err
	}
	return Artifact{File: file, Kind: "docker", OS: targetOS, Arch: arch, SHA256: sum}, Image{Role: role, Tag: built.Tag, ID: inspection.ID, OS: targetOS, Arch: arch, File: file, Tags: slices.Clone(source.Tags), VersionOutput: strings.TrimSpace(result.Stdout)}, nil
}
