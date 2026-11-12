// VALIDATES: native image exports carry the identity of the executed image.
// PREVENTS: saving a differently stamped image or trusting the client's arch.
package buildpreview

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type imageRunner func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error)

func (run imageRunner) Run(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	return run(ctx, command)
}

func TestExportImageChecksRuntimeIdentityBeforeSaving(t *testing.T) {
	for _, wrong := range []string{"", "version", "date", "go", "architecture", "cgo"} {
		t.Run(wrong, func(t *testing.T) {
			_, deps := fixture(t)
			source, err := deps.source(t.Context(), "")
			if err != nil {
				t.Fatal(err)
			}
			version := previewVersion(source)
			id := "sha256:" + strings.Repeat("a", 64)
			versionOutput := "ze " + version + " (built " + source.BuildDate + ")\n  go:       " + source.Toolchain + "\n  os/arch:  linux/arm64\n  cgo:      disabled"
			for key, value := range map[string]string{"version": version, "date": source.BuildDate, "go": source.Toolchain, "architecture": "linux/arm64", "cgo": "disabled"} {
				if key == wrong {
					versionOutput = strings.ReplaceAll(versionOutput, value, "wrong")
				}
			}
			saved := false
			docker := interoplab.NewFixtureDocker(imageRunner(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				argv := command.Arguments
				switch {
				case slices.Contains(argv, "inspect"):
					body, err := json.Marshal(map[string]any{"Id": id, "RepoTags": []string{"ze-preview:identified"}})
					return interoplab.ProcessResult{Stdout: string(body)}, err
				case slices.Contains(argv, "run"):
					if !slices.Contains(argv, id) || !slices.Contains(argv, "--extended") {
						t.Fatalf("image runtime probe = %v", argv)
					}
					return interoplab.ProcessResult{Stdout: versionOutput}, nil
				case slices.Contains(argv, "save"):
					saved = true
					if argv[len(argv)-1] != id {
						t.Fatalf("exported mutable reference: %v", argv)
					}
					index := slices.Index(argv, "--output")
					return interoplab.ProcessResult{}, os.WriteFile(argv[index+1], []byte("image export"), 0o644)
				default:
					t.Fatalf("unexpected image command: %v", argv)
					return interoplab.ProcessResult{}, nil
				}
			}))
			artifact, record, err := exportImage(t.Context(), docker, interoplab.ImageResult{Reference: id, Tag: "ze-preview:identified"}, "deployment", "/ze", "arm64", t.TempDir(), version, source)
			if wrong != "" {
				if err == nil || saved {
					t.Fatalf("wrong %s was exported: err=%v saved=%t", wrong, err, saved)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !saved || record.ID != id || record.Arch != "arm64" || artifact.Arch != "arm64" || !slices.Equal(record.Tags, source.Tags) {
				t.Fatalf("artifact=%+v record=%+v", artifact, record)
			}
		})
	}
}
