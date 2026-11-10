// Design: docs/architecture/appliance/build-artifacts.md -- explicit source and CLI staging ownership
// Related: runtimekernel_ownership_test.go -- retained old-API ownership baselines

package appliance

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
)

// TestRuntimeKernelExplicitRootOwnsManifestEnforcement challenges the selected
// source root with a different cwd's manifests. The actual public resolver and
// default builder run; only the native backend emits controlled artifacts.
func TestRuntimeKernelExplicitRootOwnsManifestEnforcement(t *testing.T) {
	for _, selectedValid := range []bool{false, true} {
		name := "selected-root-rejects-decoy-permits"
		if selectedValid {
			name = "selected-root-permits-decoy-rejects"
		}
		t.Run(name, func(t *testing.T) {
			root, cached, requested := runtimeKernelOwnershipFixture(t)
			decoy := t.TempDir()
			t.Chdir(decoy)
			writeRuntimeKernelRegistry(t)
			if selectedValid {
				manifest := filepath.Join(runtimeKernelConfigDir, runtimeKernelProfile+".require")
				if err := os.WriteFile(manifest, []byte("CONFIG_DECOY_ONLY_REQUIREMENT\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			oldNative := nativeKernelBuildFn
			nativeKernelBuildFn = func(_ context.Context, request kernelbuilder.Request) error {
				calls++
				if request.Root != root {
					t.Errorf("selected root %q changed to %q before native build", root, request.Root)
				}
				ownedParent := filepath.Join(root, "tmp", "kernel") + string(filepath.Separator)
				if !filepath.IsAbs(request.OutputDir) || !strings.HasPrefix(request.OutputDir, ownedParent) || request.OutputDir == filepath.Dir(requested) {
					t.Fatalf("selected-root build did not supply a private absolute output under %q: %q", ownedParent, request.OutputDir)
				}
				writeRuntimeKernelTree(t, request.OutputDir, runtime.GOARCH)
				config := fakeRuntimeConfig()
				if selectedValid {
					config += "CONFIG_RUNTIME_OWNERSHIP_FIXTURE=y\n"
				}
				return os.WriteFile(filepath.Join(request.OutputDir, "config"), []byte(config), 0o600)
			}
			t.Cleanup(func() { nativeKernelBuildFn = oldNative })
			tree, err := RuntimeKernelTree(root, runtime.GOARCH)
			if calls != 1 {
				t.Fatalf("native backend calls=%d, want one; error=%v", calls, err)
			}
			if selectedValid {
				if err != nil || tree != cached {
					t.Errorf("valid selected-root requirements were replaced by the decoy: tree=%q error=%v", tree, err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "CONFIG_RUNTIME_OWNERSHIP_FIXTURE") {
					t.Errorf("selected-root manifest did not enforce its missing symbol: %v", err)
				}
				if _, err := os.Stat(filepath.Join(cached, "vmlinuz")); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("requirement rejection published a runtime kernel: %v", err)
				}
			}
			assertRuntimeKernelSentinel(t, requested)
			if cwd, err := os.Getwd(); err != nil || cwd != decoy {
				t.Errorf("resolver changed invocation cwd: got %q error=%v, want %q", cwd, err, decoy)
			}
		})
	}
}

// TestRuntimeKernelCacheIdentityIgnoresCheckoutPrefix builds two independent
// minimal input fixtures, not source-tree copies, and compares their real keys.
func TestRuntimeKernelCacheIdentityIgnoresCheckoutPrefix(t *testing.T) {
	_, first, _ := runtimeKernelOwnershipFixture(t)
	_, second, _ := runtimeKernelOwnershipFixture(t)
	if filepath.Base(first) != filepath.Base(second) {
		t.Fatalf("absolute checkout prefixes changed otherwise identical kernel keys: %q versus %q", first, second)
	}
}

// TestRuntimeKernelCLIAloneMaterializesStage proves deliberate warm/cold CLI
// staging, the existing internal output relocation control, and nonmaterializing
// cache commands. It adds no public flag and never replaces the resolver.
func TestRuntimeKernelCLIAloneMaterializesStage(t *testing.T) {
	for _, warmed := range []bool{false, true} {
		for _, outputKind := range []string{"default", "absolute-internal-override", "relative-internal-override"} {
			for _, action := range []string{"resolve", "print-cache-dir", "evict-cache"} {
				name := "cold/" + outputKind + "/" + action
				if warmed {
					name = "warm/" + outputKind + "/" + action
				}
				t.Run(name, func(t *testing.T) {
					root, cached, requested := runtimeKernelOwnershipFixture(t)
					output := filepath.Dir(requested)
					if outputKind == "absolute-internal-override" {
						output = filepath.Join(t.TempDir(), "operator-stage")
						t.Setenv(kernelTestOutputDirEnv, output)
					}
					if outputKind == "relative-internal-override" {
						output = filepath.Join(root, "operator-stage")
						t.Setenv(kernelTestOutputDirEnv, "operator-stage")
					}
					if err := os.MkdirAll(output, 0o700); err != nil {
						t.Fatal(err)
					}
					staged := filepath.Join(output, "vmlinuz")
					if err := os.WriteFile(staged, []byte(runtimeKernelRequestedSentinel), 0o600); err != nil {
						t.Fatal(err)
					}
					if warmed {
						writeRuntimeKernelTree(t, cached, runtime.GOARCH)
					}
					calls := 0
					oldNative := nativeKernelBuildFn
					nativeKernelBuildFn = func(_ context.Context, request kernelbuilder.Request) error {
						calls++
						if request.Root != root {
							t.Errorf("CLI source root was not forwarded: got %q, want %q", request.Root, root)
						}
						if request.OutputDir == output || request.OutputDir == filepath.Dir(requested) {
							t.Errorf("resolution built into a shared/CLI stage rather than owned output: %q", request.OutputDir)
						}
						writeRuntimeKernelTree(t, request.OutputDir, runtime.GOARCH)
						return os.WriteFile(filepath.Join(request.OutputDir, "config"), []byte(fakeRuntimeConfig()+"CONFIG_RUNTIME_OWNERSHIP_FIXTURE=y\n"), 0o600)
					}
					t.Cleanup(func() { nativeKernelBuildFn = oldNative })
					args := []string{"--target", "runtime", "--arch", runtime.GOARCH}
					if action != "resolve" {
						args = append(args, "--"+action)
					}
					if code := runKernel(args); code != exitOK {
						t.Fatalf("actual kernel CLI failed: code=%d", code)
					}
					wantCalls := 0
					if !warmed && action == "resolve" {
						wantCalls = 1
					}
					if calls != wantCalls {
						t.Errorf("native builds=%d, want %d", calls, wantCalls)
					}
					if action == "resolve" {
						stageBytes, stageErr := os.ReadFile(staged)
						cacheBytes, cacheErr := os.ReadFile(filepath.Join(cached, "vmlinuz"))
						if stageErr != nil || cacheErr != nil || !bytes.Equal(stageBytes, cacheBytes) {
							t.Errorf("CLI did not deliberately materialize selected kernel: stage error=%v cache error=%v", stageErr, cacheErr)
						}
						if bytes.Equal(stageBytes, []byte(runtimeKernelRequestedSentinel)) {
							t.Error("CLI output still holds the old staged input rather than selected kernel")
						}
					} else {
						assertRuntimeKernelSentinel(t, staged)
					}
					if outputKind != "default" {
						assertRuntimeKernelSentinel(t, requested)
					}
				})
			}
		}
	}
}
