// Design: docs/contributing/preview-artifacts.md -- provenance stays pinned at start.
package buildpreview

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHEADMovementKeepsIdenticalInputBundleAndStartingIdentity(t *testing.T) {
	root, deps := fixture(t)
	read := deps.source
	before, err := read(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	deps.source = func(ctx context.Context, root string) (Source, error) {
		source, err := read(ctx, root)
		calls++
		if calls > 1 {
			source.Commit = strings.Repeat("d", 40)
			source.BuildDate = "2026-11-12T01:00:00Z"
			source.Dirty = false
		}
		return source, err
	}
	manifest, err := prepare(t.Context(), root, filepath.Join(t.TempDir(), "preview"), false, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manifest.Source, before) || manifest.Version != previewVersion(before) {
		t.Fatalf("starting provenance replaced: %+v", manifest)
	}
}

func TestInputMovementStillRefusesAfterHEADMoves(t *testing.T) {
	for _, field := range []string{"toolchain", "tags", "recipe"} {
		t.Run(field, func(t *testing.T) {
			root, deps := fixture(t)
			read := deps.source
			calls := 0
			deps.source = func(ctx context.Context, root string) (Source, error) {
				source, err := read(ctx, root)
				calls++
				if calls > 1 {
					source.Commit = strings.Repeat("d", 40)
					switch field {
					case "toolchain":
						source.Toolchain = "go1.27.3"
					case "tags":
						source.Tags = append(source.Tags, "ze_web")
					case "recipe":
						source.Inputs = append(source.Inputs, Input{Path: "docker/Dockerfile.runtime", SHA256: strings.Repeat("e", 64)})
					}
					source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
				}
				return source, err
			}
			if _, err := prepare(t.Context(), root, filepath.Join(t.TempDir(), "preview"), false, deps); err == nil {
				t.Fatalf("changed %s accepted after HEAD movement", field)
			}
		})
	}
}
