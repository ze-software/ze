// Design: docs/architecture/plugin/feature-gates.md -- normalize a real foreign build.Context seed.
package repoinventory

import (
	"go/build"
	"slices"
	"testing"

	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// TestLinuxContextNormalizesDarwinARM64Seed drives the production normalizer
// with every host-sensitive selector set to a conflicting foreign value, then
// selects target-only source through MatchFile over retained blob bytes.
func TestLinuxContextNormalizesDarwinARM64Seed(t *testing.T) {
	seed := build.Context{
		GOOS: "darwin", GOARCH: "arm64", Compiler: "gccgo", CgoEnabled: true,
		BuildTags: []string{"host_only", "ze_core"}, ToolTags: []string{"arm64.v8.0", "goexperiment.fixture"},
		ReleaseTags: []string{"go1.26"}, GOROOT: "/fixture/toolchain",
	}
	blobs := map[string][]byte{
		"source/register_linux_amd64.go": []byte("//go:build ze_le && ze_example && linux && amd64 && gc && !cgo && amd64.v1 && go1.26 && goexperiment.fixture && !ze_core && !host_only\n\npackage source\n"),
		"source/register_darwin_arm64.go": []byte("package source\n"),
		"source/host.go": []byte("//go:build host_only || ze_core || cgo || gccgo || arm64.v8.0\n\npackage source\n"),
	}
	context := normalizeLinuxContext(&seed, []repofeaturetags.Gate{{Tag: "ze_example"}}, blobs)
	if context.GOOS != "linux" || context.GOARCH != "amd64" || context.Compiler != "gc" || context.CgoEnabled {
		t.Fatalf("target selectors retained foreign seed: %+v", context)
	}
	if !slices.Equal(context.BuildTags, []string{"ze_example", "ze_le"}) || !slices.Equal(context.ToolTags, []string{"amd64.v1", "goexperiment.fixture"}) || !slices.Equal(context.ReleaseTags, []string{"go1.26"}) {
		t.Fatalf("target tags retained foreign seed or lost toolchain tags: %+v", context)
	}
	for name, want := range map[string]bool{"register_linux_amd64.go": true, "register_darwin_arm64.go": false, "host.go": false} {
		matched, err := context.MatchFile("source", name)
		if err != nil {
			t.Fatal(err)
		}
		if matched != want {
			t.Errorf("target selected %s = %v, want %v", name, matched, want)
		}
	}
	if seed.GOOS != "darwin" || seed.GOARCH != "arm64" || !seed.CgoEnabled || seed.ToolTags[0] != "arm64.v8.0" {
		t.Fatalf("normalization mutated caller seed: %+v", seed)
	}
}
