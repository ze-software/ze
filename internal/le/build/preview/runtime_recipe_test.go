// Design: docs/contributing/preview-artifacts.md -- each staged recipe binds its captured hash.
package buildpreview

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestEachImageRefusesChangedRuntimeRecipe drives both role transitions
// through the real context producer and counts Docker builds, not messages.
func TestEachImageRefusesChangedRuntimeRecipe(t *testing.T) {
	for _, changedRole := range []string{"deployment", "lab"} {
		t.Run(changedRole, func(t *testing.T) {
			root, deps := fixture(t)
			source, err := deps.source(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			const recipe = "FROM scratch AS deployment\nCOPY ze /ze\nFROM scratch AS lab\nCOPY ze /usr/local/bin/ze\n"
			put(t, root, "docker/Dockerfile.runtime", recipe)
			sum, err := fileSum(filepath.Join(root, "docker/Dockerfile.runtime"))
			if err != nil {
				t.Fatal(err)
			}
			source.Inputs = append(source.Inputs, Input{Path: "docker/Dockerfile.runtime", SHA256: sum})
			source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
			version := previewVersion(source)
			output := t.TempDir()
			if err := os.WriteFile(filepath.Join(output, "ze-"+version+"-linux-amd64"), []byte("completed native output"), 0o755); err != nil {
				t.Fatal(err)
			}
			if changedRole == "deployment" {
				put(t, root, "docker/Dockerfile.runtime", recipe+"# changed input\n")
			}
			id := "sha256:" + strings.Repeat("a", 64)
			builds := 0
			docker := interoplab.NewFixtureDocker(imageRunner(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				argv := command.Arguments
				switch {
				case argv[1] == "version":
					return interoplab.ProcessResult{Stdout: "amd64\n"}, nil
				case argv[1] == "build":
					builds++
					if _, err := io.Copy(io.Discard, command.Stdin); err != nil {
						return interoplab.ProcessResult{}, err
					}
					// A later restoration cannot make already staged changed bytes valid.
					put(t, root, "docker/Dockerfile.runtime", recipe)
					return interoplab.ProcessResult{Stdout: id}, nil
				case slices.Contains(argv, "inspect"):
					if slices.Contains(argv, "{{.Id}}") {
						return interoplab.ProcessResult{Stdout: id}, nil
					}
					body, err := json.Marshal(map[string]any{"Id": id})
					return interoplab.ProcessResult{Stdout: string(body)}, err
				case slices.Contains(argv, "run"):
					return interoplab.ProcessResult{Stdout: "ze " + version + " (built " + source.BuildDate + ")\n  go:       " + source.Toolchain + "\n  os/arch:  linux/amd64\n  cgo:      disabled"}, nil
				case slices.Contains(argv, "save"):
					index := slices.Index(argv, "--output")
					err := os.WriteFile(argv[index+1], []byte("saved image"), 0o644)
					if changedRole == "lab" {
						put(t, root, "docker/Dockerfile.runtime", recipe+"# changed input\n")
					}
					return interoplab.ProcessResult{}, err
				default:
					return interoplab.ProcessResult{}, nil
				}
			}))
			_, _, err = prepareImagesWithDocker(t.Context(), root, output, version, source, docker)
			if err == nil {
				t.Fatal("changed runtime recipe was accepted for image export")
			}
			if !strings.Contains(err.Error(), "runtime recipe") {
				t.Fatalf("recipe refusal lost its cause: %v", err)
			}
			want := 0
			if changedRole == "lab" {
				want = 1
			}
			if builds != want {
				t.Fatalf("changed %s recipe reached Docker: builds=%d want=%d", changedRole, builds, want)
			}
		})
	}
}
