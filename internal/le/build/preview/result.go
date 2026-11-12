// Design: docs/contributing/preview-artifacts.md -- concise command answers.
package buildpreview

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// Result answers a native action without copying the full source population
// into its renderer. Manifest points to the complete, unchanged wire document.
type Result struct {
	Directory string         `json:"directory"`
	Manifest  string         `json:"manifest"`
	Checksums string         `json:"checksums"`
	Channel   string         `json:"channel"`
	Published bool           `json:"published"`
	Version   string         `json:"version"`
	Source    SourceSummary  `json:"source"`
	Artifacts []Artifact     `json:"artifacts"`
	Images    []ImageSummary `json:"images,omitempty"`
}

// SourceSummary identifies the build; file-by-file hashes stay in the manifest.
type SourceSummary struct {
	Commit     string `json:"commit"`
	Dirty      bool   `json:"dirty"`
	Digest     string `json:"digest"`
	BuildDate  string `json:"build-date"`
	Toolchain  string `json:"toolchain"`
	InputCount int    `json:"input-count"`
}

// ImageSummary gives the usable local identity and the downloadable filename.
// Runtime version output and build-tag inventories stay in the full manifest.
type ImageSummary struct {
	Role     string `json:"role"`
	Tag      string `json:"tag"`
	ID       string `json:"id"`
	Platform string `json:"platform"`
	File     string `json:"file"`
}

func summarizePreview(output string, manifest Manifest) Result {
	result := Result{Directory: output, Manifest: filepath.Join(output, "manifest.json"), Checksums: filepath.Join(output, "SHA256SUMS"), Channel: manifest.Channel, Published: manifest.Published, Version: manifest.Version,
		Source: SourceSummary{Commit: manifest.Source.Commit, Dirty: manifest.Source.Dirty, Digest: manifest.Source.Digest, BuildDate: manifest.Source.BuildDate, Toolchain: manifest.Source.Toolchain, InputCount: len(manifest.Source.Inputs)}, Artifacts: slices.Clone(manifest.Artifacts)}
	for _, image := range manifest.Images {
		result.Images = append(result.Images, ImageSummary{Role: image.Role, Tag: image.Tag, ID: image.ID, Platform: image.OS + "/" + image.Arch, File: image.File})
	}
	return result
}

func runPreview(ctx context.Context, root, output string, images bool, deps dependencies) (any, int) {
	directory, err := filepath.Abs(output)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "preview output path cannot be resolved: "+output,
			"No bundle can be prepared at this path. Use an existing working directory and an absolute path to a new output directory.",
			previewRunCommand("/absolute/path/to/new-preview", images)))
		return nil, 1
	}
	manifest, err := prepare(ctx, root, directory, images, deps)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "preview preparation failed for "+directory,
			"No verified bundle was prepared. Inspect the named path and resolve the reported input or build failure before running this command; choose a new output directory if this one already exists.",
			previewRunCommand(directory, images)))
		return nil, 1
	}
	return summarizePreview(directory, manifest), 0
}

func checkPreview(output string, inspect func(string, string, Source, string) error) (any, int) {
	directory, err := filepath.Abs(output)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "preview bundle path cannot be resolved: "+output,
			"The bundle cannot be located. Use an existing working directory and an absolute path to the prepared bundle.",
			previewCheckCommand("/absolute/path/to/prepared-preview")))
		return nil, 1
	}
	manifest, err := check(directory, inspect)
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "preview bundle could not be verified at "+directory,
			"Do not install or distribute this bundle. Check the named artifact and resolve the reported mismatch or missing input; recover trusted original files or prepare a fresh bundle before checking again.",
			previewCheckCommand(directory)))
		return nil, 1
	}
	return summarizePreview(directory, manifest), 0
}

func previewRunCommand(output string, images bool) string {
	command := "./le build preview run output " + interoplab.ShellQuoted(output)
	if images {
		command += " images"
	}
	return command
}

func previewCheckCommand(output string) string {
	return "./le build preview check path " + interoplab.ShellQuoted(output)
}
