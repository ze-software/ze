// Design: docs/architecture/appliance/build-artifacts.md -- runtime kernel input ownership
// Related: runtimekernel.go -- the exported image/runtime resolution entry points
// Related: cmd_kernel.go -- actual defaultKernelBuild and runtime cache publication

package appliance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// TestRuntimeKernelLibraryWarmPreservesRequestedKernel reaches the old public
// API with a valid warm entry and a different operator-selected staged kernel.
// Baseline cwd is deliberately the complete fixture root, so a missing profile
// cannot mask the actual warm copyTree mutation. Explicit-root/foreign-cwd
// coverage is a separate strengthening after the reviewed API cutover.
func TestRuntimeKernelLibraryWarmPreservesRequestedKernel(t *testing.T) {
	root, cached, requested := runtimeKernelOwnershipFixture(t)
	writeRuntimeKernelTree(t, cached, runtime.GOARCH)
	oldNative := nativeKernelBuildFn
	nativeKernelBuildFn = func(context.Context, kernelbuilder.Request) error {
		t.Fatal("a valid warmed runtime entry started a native builder")
		return nil
	}
	t.Cleanup(func() { nativeKernelBuildFn = oldNative })

	tree, err := ImageRuntimeKernelTree(root, runtime.GOARCH, func(arch string) string {
		return "./le setup docker host build arch " + arch
	})
	if err != nil {
		t.Fatalf("valid warmed image/runtime resolver refused before ownership could be checked: %v", err)
	}
	if tree != cached {
		t.Errorf("returned cache identity %q, want exact keyed entry %q", tree, cached)
	}
	if _, err := os.Stat(filepath.Join(cached, "vmlinuz")); err != nil {
		t.Fatalf("the warmed fixture was not a usable cache hit: %v", err)
	}
	assertRuntimeKernelSentinel(t, requested)
	if cwd, err := os.Getwd(); err != nil || cwd != root {
		t.Errorf("library changed invocation cwd: got %q, error %v; want %q", cwd, err, root)
	}
}

// TestRuntimeKernelLibraryColdOwnsOutput reaches the actual exported resolver
// and defaultKernelBuild, replacing only the native backend. The backend writes
// real controlled artifacts to the submitted output and records the untouched
// request, so omitted Root/private-output propagation cannot be hidden by a
// stub at kernelBuildFn. Every case preserves the requested kernel oracle.
func TestRuntimeKernelLibraryColdOwnsOutput(t *testing.T) {
	for _, scenario := range []string{"publish", "requirements-refuse", "builder-error"} {
		t.Run(scenario, func(t *testing.T) {
			root, cached, requested := runtimeKernelOwnershipFixture(t)
			calls := 0
			var output string
			var emitted []byte
			oldNative := nativeKernelBuildFn
			nativeKernelBuildFn = func(_ context.Context, request kernelbuilder.Request) error {
				calls++
				if request.Root != root {
					t.Errorf("actual defaultKernelBuild handed native backend Root=%q, want selected root %q", request.Root, root)
				}
				if !filepath.IsAbs(request.OutputDir) {
					t.Errorf("native backend received non-absolute owned output %q", request.OutputDir)
				}
				// This follows the submitted path's actual invocation semantics;
				// it does not synthesize a missing Request.Root for the assertion.
				var err error
				output, err = filepath.Abs(request.OutputDir)
				if err != nil {
					return err
				}
				shared := filepath.Join(root, runtimeKernelOutputDir)
				if output == shared {
					t.Errorf("library submitted the operator's shared stage as native output: %s", output)
				}
				if !strings.HasPrefix(output, filepath.Join(root, "tmp")+string(filepath.Separator)) {
					t.Errorf("native output %q is not run-owned beneath selected root tmp", output)
				}
				if request.SourceDir != runtimeKernelConfigDir || request.PatchesDir != runtimeKernelPatchesDir {
					t.Errorf("native source/patch labels changed: source=%q patches=%q", request.SourceDir, request.PatchesDir)
				}
				wantFragments := []string{
					filepath.Join(runtimeKernelConfigDir, kernelConfigName),
					filepath.Join(runtimeKernelConfigDir, runtimeKernelProfile+".config"),
					filepath.Join(kernelCommonDir, "ownership.config"),
				}
				if !slices.Equal(request.Fragments, wantFragments) {
					t.Errorf("native fragment labels/order=%q, want %q", request.Fragments, wantFragments)
				}
				// Empty builder/common fields retain the native Request defaults;
				// a supplied override must still be its repository-relative label.
				if request.BuilderDir != "" && request.BuilderDir != kernelbuilder.DefaultBuilderDir {
					t.Errorf("native builder label %q is not the declared default", request.BuilderDir)
				}
				if request.CommonDir != "" && request.CommonDir != kernelbuilder.DefaultCommonDir {
					t.Errorf("native common label %q is not the declared default", request.CommonDir)
				}
				writeRuntimeKernelTree(t, output, runtime.GOARCH)
				config := fakeRuntimeConfig()
				if scenario != "requirements-refuse" {
					config += "CONFIG_RUNTIME_OWNERSHIP_FIXTURE=y\n"
				}
				if err := os.WriteFile(filepath.Join(output, "config"), []byte(config), 0o600); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(output, "lib", "modules", defaultKernelVersion+"-ze", "modules.dep"), []byte("kernel/fixture.ko:\n"), 0o600); err != nil {
					return err
				}
				emitted, err = os.ReadFile(filepath.Join(output, "vmlinuz"))
				if err != nil {
					return err
				}
				if scenario == "builder-error" {
					return errors.New("fixture native worker teardown unconfirmed")
				}
				return nil
			}
			t.Cleanup(func() { nativeKernelBuildFn = oldNative })

			tree, err := RuntimeKernelTree(root, runtime.GOARCH)
			if calls != 1 {
				t.Fatalf("actual defaultKernelBuild reached native backend %d times, want exactly once; resolver error: %v", calls, err)
			}
			assertRuntimeKernelSentinel(t, requested)
			if scenario == "publish" {
				if err != nil {
					t.Fatalf("valid controlled cold build refused: %v", err)
				}
				if tree != cached {
					t.Errorf("cold publication returned %q, want exact keyed entry %q", tree, cached)
				}
				published, readErr := os.ReadFile(filepath.Join(cached, "vmlinuz"))
				if readErr != nil || !bytes.Equal(published, emitted) {
					t.Errorf("published kernel differs from emitted artifact: read error %v", readErr)
				}
				for _, path := range []string{"config", filepath.Join("lib", "modules", defaultKernelVersion+"-ze", "modules.dep"), kernelbuilder.ProvenanceName} {
					if _, err := os.Stat(filepath.Join(cached, path)); err != nil {
						t.Errorf("cold publication lost %s: %v", path, err)
					}
				}
			}
			if scenario == "requirements-refuse" {
				if err == nil || !strings.Contains(err.Error(), "CONFIG_RUNTIME_OWNERSHIP_FIXTURE") {
					t.Errorf("selected-root shared manifest did not reject missing symbol: %v", err)
				}
			}
			if scenario == "builder-error" {
				if err == nil || !strings.Contains(err.Error(), "fixture native worker teardown unconfirmed") {
					t.Errorf("native teardown failure was lost: %v", err)
				}
				if err == nil || !strings.Contains(err.Error(), output) {
					t.Errorf("builder failure did not identify its retained owned output %q: %v", output, err)
				}
				if _, err := os.Stat(output); err != nil {
					t.Errorf("unconfirmed native writers' output was discarded: %v", err)
				}
			}
			if scenario != "publish" {
				if _, err := os.Stat(filepath.Join(cached, "vmlinuz")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("failed cold build published a kernel: %v", err)
				}
			}
			if scenario != "builder-error" {
				if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("joined builder's run-owned output was not cleaned after %s: %v", scenario, err)
				}
			}
		})
	}
}

const runtimeKernelRequestedSentinel = "operator-selected kernel: library resolution must not overwrite these bytes\n"

// runtimeKernelOwnershipFixture uses the existing valid registry/key fixture.
// Its baseline cwd satisfies the old API's actual prerequisite intentionally;
// the source repair must later remove that prerequisite without adding chdir.
func runtimeKernelOwnershipFixture(t *testing.T) (root, cached, requested string) {
	t.Helper()
	root = t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv(kernelTestOutputDirEnv, "")
	writeRuntimeKernelRegistry(t)
	if err := os.MkdirAll(kernelCommonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(runtimeKernelConfigDir, runtimeKernelProfile+".config")
	config, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	config = append(config, []byte("# ze-include: ownership\n")...)
	for path, content := range map[string][]byte{
		profile: config,
		filepath.Join(kernelCommonDir, "ownership.config"):  []byte("CONFIG_RUNTIME_OWNERSHIP_FIXTURE=y\n"),
		filepath.Join(kernelCommonDir, "ownership.require"): []byte("CONFIG_RUNTIME_OWNERSHIP_FIXTURE\n"),
	} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cached, err = kernelCachePathFor(root, defaultKernelVersion, runtime.GOARCH, runtimeKernelProfile, "", kernelTargetRuntime)
	if err != nil {
		t.Fatalf("valid registry/builder/patch fixture cannot produce current runtime identity: %v", err)
	}
	requested = filepath.Join(root, runtimeKernelOutputDir, "vmlinuz")
	if err := os.MkdirAll(filepath.Dir(requested), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(requested, []byte(runtimeKernelRequestedSentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, cached, requested
}

// assertRuntimeKernelSentinel observes both input bytes and operator mode.
func assertRuntimeKernelSentinel(t *testing.T, requested string) {
	t.Helper()
	body, err := os.ReadFile(requested)
	if err != nil {
		t.Errorf("requested kernel became unreadable: %v", err)
	} else if string(body) != runtimeKernelRequestedSentinel {
		t.Errorf("library replaced requested kernel bytes: got %q", body)
	}
	info, err := os.Stat(requested)
	if err != nil {
		t.Errorf("requested kernel lost its filesystem identity: %v", err)
		return
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("library changed requested kernel mode to %04o, want 0600", info.Mode().Perm())
	}
}
