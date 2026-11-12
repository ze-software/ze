// Design: docs/contributing/preview-artifacts.md -- validate inputs at their last read.
package buildpreview

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestImagePackagingAllowsLaterGoEdits preserves the captured identity after
// both binaries and archives consumed source, while image work uses outputs.
func TestImagePackagingAllowsLaterGoEdits(t *testing.T) {
	root, deps := fixture(t)
	put(t, root, "cmd/ze/main.go", "compiled source")
	read := deps.source
	reads := 0
	deps.source = func(ctx context.Context, root string) (Source, error) {
		source, err := read(ctx, root)
		if err != nil {
			return source, err
		}
		reads++
		sum, err := fileSum(filepath.Join(root, "cmd/ze/main.go"))
		if err != nil {
			return source, err
		}
		source.Inputs = []Input{{Path: "cmd/ze/main.go", SHA256: sum}}
		source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
		return source, nil
	}
	original, err := deps.source(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	reads = 0
	packaged := false
	deps.images = func(_ context.Context, root, _, _ string, source Source) ([]Artifact, []Image, error) {
		if reads != 2 {
			t.Fatalf("image packaging began before full source revalidation: reads=%d", reads)
		}
		if !reflect.DeepEqual(source, original) {
			t.Fatal("packaging did not receive starting identity")
		}
		put(t, root, "cmd/ze/main.go", "next session's source edit")
		packaged = true
		return nil, nil, nil
	}
	manifest, err := prepare(t.Context(), root, filepath.Join(t.TempDir(), "preview"), true, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !packaged || reads != 2 || !reflect.DeepEqual(manifest.Source, original) {
		t.Fatalf("completed output identity changed: packaged=%t reads=%d source=%+v", packaged, reads, manifest.Source)
	}
}

// TestCompilePhaseDriftRefusesBeforeImagePackaging changes a selected input
// during the last compile and requires refusal before the image callback.
func TestCompilePhaseDriftRefusesBeforeImagePackaging(t *testing.T) {
	root, deps := fixture(t)
	read := deps.source
	build := deps.build
	changed := false
	deps.build = func(ctx context.Context, root, arch, output, version, date string) error {
		if err := build(ctx, root, arch, output, version, date); err != nil {
			return err
		}
		if arch == "arm64" {
			changed = true
		}
		return nil
	}
	deps.source = func(ctx context.Context, root string) (Source, error) {
		source, err := read(ctx, root)
		if changed {
			source.Inputs = []Input{{Path: "cmd/ze/main.go", SHA256: "changed during compilation"}}
			source.Digest = sourceDigest(inputDigest(source.Inputs), source.Toolchain, source.Tags)
		}
		return source, err
	}
	images := 0
	deps.images = func(context.Context, string, string, string, Source) ([]Artifact, []Image, error) {
		images++
		return nil, nil, nil
	}
	_, err := prepare(t.Context(), root, filepath.Join(t.TempDir(), "preview"), true, deps)
	if err == nil {
		t.Fatal("compiler input drift accepted")
	}
	if images != 0 {
		t.Fatalf("image packaging ran after compiler input drift: calls=%d", images)
	}
	if !strings.Contains(err.Error(), "cmd/ze/main.go") {
		t.Fatalf("source drift refusal omitted its changed input: %v", err)
	}
}
