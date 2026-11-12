// Design: docs/contributing/preview-artifacts.md -- image preparation reuses inspected outputs.
package buildpreview

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func TestPreviewImagesReuseOnlyTheCheckedNativeBinary(t *testing.T) {
	root, deps := fixture(t)
	source, err := deps.source(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	put(t, root, "docker/Dockerfile.runtime", "FROM scratch AS deployment\nCOPY ze /ze\nFROM scratch AS lab\nCOPY ze /usr/local/bin/ze\n")
	sum, err := fileSum(filepath.Join(root, "docker/Dockerfile.runtime"))
	if err != nil {
		t.Fatal(err)
	}
	source.Inputs = append(source.Inputs, Input{Path: "docker/Dockerfile.runtime", SHA256: sum})
	source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
	version := previewVersion(source)
	put(t, root, "test/interop-stale/ze-linux", "must not enter image context")
	for _, arch := range []string{"amd64", "arm64"} {
		path := filepath.Join(output, "ze-"+version+"-linux-"+arch)
		if err := os.WriteFile(path, []byte("checked binary "+arch), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	id := "sha256:" + strings.Repeat("a", 64)
	var targets []string
	docker := interoplab.NewFixtureDocker(imageRunner(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
		argv := command.Arguments
		switch {
		case argv[1] == "version":
			return interoplab.ProcessResult{Stdout: "arm64\n"}, nil
		case argv[1] == "build":
			index := slices.Index(argv, "--target")
			if index < 0 || index+1 >= len(argv) {
				t.Fatalf("image preparation did not select a runtime target: %v", argv)
			}
			targets = append(targets, argv[index+1])
			entries := map[string]string{}
			reader := tar.NewReader(command.Stdin)
			for {
				header, err := reader.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				entries[header.Name] = string(body)
			}
			if len(entries) != 2 || entries["ze"] != "checked binary arm64" || entries["Dockerfile"] == "" {
				t.Fatalf("preview sent another binary or broad source context: %v", entries)
			}
			return interoplab.ProcessResult{Stdout: id}, nil
		case slices.Contains(argv, "inspect"):
			if slices.Contains(argv, "{{.Id}}") {
				return interoplab.ProcessResult{Stdout: id}, nil
			}
			body, err := json.Marshal(map[string]any{"Id": id})
			return interoplab.ProcessResult{Stdout: string(body)}, err
		case slices.Contains(argv, "run"):
			text := "ze " + version + " (built " + source.BuildDate + ")\n  go:       " + source.Toolchain + "\n  os/arch:  linux/arm64\n  cgo:      disabled"
			return interoplab.ProcessResult{Stdout: text}, nil
		case slices.Contains(argv, "save"):
			index := slices.Index(argv, "--output")
			return interoplab.ProcessResult{}, os.WriteFile(argv[index+1], []byte("saved image"), 0o644)
		default:
			return interoplab.ProcessResult{}, nil
		}
	}))
	artifacts, images, err := prepareImagesWithDocker(t.Context(), root, output, version, source, docker)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(targets, []string{"deployment", "lab"}) || len(artifacts) != 2 || len(images) != 2 {
		t.Fatalf("runtime preparations targets=%v artifacts=%v images=%v", targets, artifacts, images)
	}
	for _, image := range images {
		if !slices.Equal(image.Tags, source.Tags) {
			t.Fatalf("image omits checked binary features: got %v want %v", image.Tags, source.Tags)
		}
	}
}
