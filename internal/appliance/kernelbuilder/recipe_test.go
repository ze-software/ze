package kernelbuilder

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestBuildGuestWorkerTakesTheRecipe proves the host compiles the QEMU guest's
// worker with exactly the argv and environment guestWorkerBuild declares, so an
// edit to how the worker is built changes a file the cache key hashes.
//
// VALIDATES: round-3 ISSUE 1, the worker build is keyed.
// PREVENTS: a host-side edit to the go command changing the build and keeping the key.
func TestBuildGuestWorkerTakesTheRecipe(t *testing.T) {
	hostIs(t, "amd64")
	req := Request{Root: t.TempDir(), Arch: "amd64"}
	old := runWorkerBuild
	t.Cleanup(func() { runWorkerBuild = old })
	var gotArgs, gotEnv []string
	runWorkerBuild = func(_ context.Context, _ Request, env []string, args ...string) error {
		gotArgs, gotEnv = args, env
		return nil
	}
	rel, err := buildGuestWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	wantRel, wantArgs, wantEnv := guestWorkerBuild(req)
	if rel != filepath.ToSlash(wantRel) {
		t.Errorf("worker path = %s, want %s", rel, wantRel)
	}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Errorf("go argv = %v, want guestWorkerBuild %v", gotArgs, wantArgs)
	}
	if !slices.Equal(gotEnv, wantEnv) {
		t.Errorf("go env = %v, want guestWorkerBuild %v", gotEnv, wantEnv)
	}
	if !slices.Contains(wantEnv, "GOARCH=amd64") || !slices.Contains(wantEnv, "CGO_ENABLED=0") {
		t.Errorf("guestWorkerBuild env = %v, want a static linux/amd64 worker", wantEnv)
	}
}

// TestTargetDecidesModules proves the target alone decides the modules switch,
// in the recipe the cache key hashes: a caller's modules value is overridden,
// and a target with no answer is refused.
//
// VALIDATES: round-3 ISSUE 1, the modules value is keyed.
// PREVENTS: a runtime kernel built without modules served from a modules key.
func TestTargetDecidesModules(t *testing.T) {
	for target, want := range map[string]string{"runtime": modulesYes, "installer": modulesNo} {
		req := Request{Target: target, Modules: "maybe"}
		if err := normalizeRequest(&req); err != nil {
			t.Fatal(err)
		}
		if req.Modules != want {
			t.Errorf("target %s: modules = %q, want %q", target, req.Modules, want)
		}
	}
	req := Request{Root: t.TempDir(), Version: "7.1.1", Arch: hostGOARCH, Profile: "qemu", Target: "bogus", SourceDir: "configs", OutputDir: "out", Fragments: []string{"configs/kernel.config"}}
	err := validateRequest(&req)
	if err == nil || !strings.Contains(err.Error(), `unsupported kernel target "bogus" (expected installer or runtime)`) {
		t.Fatalf("validateRequest(target bogus) = %v, want the target refusal", err)
	}
}
