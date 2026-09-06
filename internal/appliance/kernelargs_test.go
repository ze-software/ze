package appliance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// coverage MOVED, not dropped. TestDerivedInstanceConfigPreservesFields,
// TestDerivedInstanceConfigNoBaseArgs, TestMaterializeDerivedParent,
// TestMaterializeDerivedParentIsolatesConcurrentBuilds,
// TestCopyBuildDirFailsClosedWithoutModules and
// TestAbsolutizeReplacesLeavesVersionReplaces moved verbatim (assertions
// unchanged) to internal/appliance/instance/prepare_test.go when their subject
// moved to that package, and the instance package added three further tests on
// top. This file keeps the tests for what still lives here: kernel-argument
// assembly and the build-parent seam. Verify with:
//
//	go test ./internal/appliance/instance/ -v

// TestKernelArgsHugepages verifies the assembly emits the three hugepage tokens
// in deterministic order when configured, and nothing when unconfigured (AC-8).
func TestKernelArgsHugepages(t *testing.T) {
	t.Run("unconfigured returns nil", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{})
		if err != nil || got != nil {
			t.Errorf("expected (nil, nil), got (%v, %v)", got, err)
		}
	})

	t.Run("1gb of 2mb pages", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "1gb", PageSize: "2mb"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"default_hugepagesz=2M", "hugepagesz=2M", "hugepages=512"} // 1gb / 2mb = 512
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("8gb of 1gb pages", func(t *testing.T) {
		got, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "8gb", PageSize: "1gb"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"default_hugepagesz=1G", "hugepagesz=1G", "hugepages=8"} // 8gb / 1gb = 8
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("invalid page-size errors", func(t *testing.T) {
		if _, err := hugepageKernelArgs(ImageConfig{Hugepages: &Hugepages{Size: "1gb", PageSize: "4mb"}}); err == nil {
			t.Error("expected an error for an unsupported page-size")
		}
	})
}

// TestKernelArgsIsolatedCPUs verifies the CPU isolation tokens.
//
// VALIDATES: the boot half of spec-vpp-isolated-cpus -- Ze requests the
// isolation it later consumes, so a worker core list drawn from
// /sys/devices/system/cpu/isolated has something to draw from.
// PREVENTS: an appliance that configures VPP worker pinning while its own boot
// cmdline leaves every CPU to the Linux scheduler.
func TestKernelArgsIsolatedCPUs(t *testing.T) {
	t.Run("unconfigured returns nil", func(t *testing.T) {
		got, err := isolatedCPUKernelArgs(ImageConfig{})
		if err != nil || got != nil {
			t.Errorf("expected (nil, nil), got (%v, %v)", got, err)
		}
	})

	t.Run("a range becomes three tokens", func(t *testing.T) {
		got, err := isolatedCPUKernelArgs(ImageConfig{IsolatedCPUs: "2-4"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"isolcpus=2-4", "nohz_full=2-4", "rcu_nocbs=2-4"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("arg[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("an unordered list is rendered canonically", func(t *testing.T) {
		got, err := isolatedCPUKernelArgs(ImageConfig{IsolatedCPUs: "6,3,4,5"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != "isolcpus=3-6" {
			t.Errorf("arg[0] = %q, want %q", got[0], "isolcpus=3-6")
		}
	})

	t.Run("a malformed list errors", func(t *testing.T) {
		if _, err := isolatedCPUKernelArgs(ImageConfig{IsolatedCPUs: "4-2"}); err == nil {
			t.Error("expected an error for a range that counts down")
		}
	})
}

// TestValidateIsolatedCPUs verifies the appliance config refuses an isolation
// request the target could not survive.
//
// VALIDATES: R-2 -- the Linux control plane keeps a CPU of its own.
// PREVENTS: an image that boots with every CPU handed to VPP and no CPU left to
// run ze, sshd or the kernel's own housekeeping.
func TestValidateIsolatedCPUs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"unset", "", false},
		{"a range above CPU 0", "1-3", false},
		{"single CPU", "3", false},
		{"CPU 0 is refused", "0-3", true},
		{"CPU 0 alone is refused", "0", true},
		{"malformed", "1-", true},
		{"duplicate", "2,2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig("lab")
			cfg.Image.IsolatedCPUs = tc.value
			err := cfg.validateIsolatedCPUs()
			if tc.wantErr && err == nil {
				t.Fatalf("validateIsolatedCPUs(%q) accepted the value", tc.value)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateIsolatedCPUs(%q): %v", tc.value, err)
			}
		})
	}
}

// writeGokrazyFixture lays out a minimal checked-in gokrazy tree
// (<root>/gokrazy/ze/{config.json,builddir/...}) and chdirs the test into root,
// which is what resolveBuildParentDir resolves "gokrazy" against.
func writeGokrazyFixture(t *testing.T) (root string) {
	t.Helper()
	root = t.TempDir()
	mod := filepath.Join(root, "gokrazy", "ze", "builddir", "github.com", "ze-software", "ze")
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, data string) {
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(mod, "go.mod"), "module gokrazy/build/github.com/ze-software/ze\n\ngo 1.26\n")
	write(filepath.Join(root, "gokrazy", "ze", "config.json"), `{"Hostname":"ze"}`)
	t.Chdir(root)
	return root
}

// TestResolveBuildParentDirAlwaysPrepares verifies EVERY build gets a prepared
// parent under the project tmp/, not only builds that request hugepages. The
// conditional this replaces is how the pin-discarding defect of 2026-07-18
// survived unnoticed: the prepared path ran only for hugepage images, so nothing
// else exercised it.
//
// VALIDATES: AC-1 -- gok is never pointed at the tracked gokrazy dir.
// PREVENTS: a rarely-exercised conditional build path rotting undetected.
func TestResolveBuildParentDirAlwaysPrepares(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *applianceConfig
	}{
		{"no hugepages", &applianceConfig{}},
		{"hugepages", &applianceConfig{Image: ImageConfig{Hugepages: &Hugepages{Size: "1gb", PageSize: "2mb"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeGokrazyFixture(t)

			parent, cleanup, err := resolveBuildParentDir(tc.cfg)
			if err != nil {
				t.Fatalf("resolveBuildParentDir: %v", err)
			}
			defer cleanup()

			tracked := filepath.Join(root, "gokrazy")
			if parent == tracked {
				t.Fatalf("build would run from the tracked dir %s", tracked)
			}
			wantTmp := filepath.Join(root, "tmp")
			if !strings.HasPrefix(parent, wantTmp+string(filepath.Separator)) {
				t.Errorf("prepared parent %s is not under project tmp %s", parent, wantTmp)
			}

			// The pins travel with it, whether or not kernel args were patched in.
			if _, err := os.Stat(filepath.Join(parent, "ze", "builddir", "github.com", "ze-software", "ze", "go.mod")); err != nil {
				t.Errorf("prepared instance lost the builddir pins: %v", err)
			}

			cleanup()
			if _, err := os.Stat(parent); !os.IsNotExist(err) {
				t.Errorf("cleanup did not remove the prepared dir: %v", err)
			}
		})
	}
}

// TestResolveBuildParentDirPatchesOnlyWhenRequested verifies unconditional
// preparation does not change the cmdline of a plain image: a config with no
// hugepage reservation gets an empty KernelExtraArgs, not a spurious argument.
//
// VALIDATES: AC-9 boundary -- preparing every build is behavior-neutral for
// builds that ask for nothing.
// PREVENTS: AC-1 silently altering every non-hugepage appliance image.
func TestResolveBuildParentDirPatchesOnlyWhenRequested(t *testing.T) {
	writeGokrazyFixture(t)

	parent, cleanup, err := resolveBuildParentDir(&applianceConfig{})
	if err != nil {
		t.Fatalf("resolveBuildParentDir: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(filepath.Join(parent, "ze", "config.json"))
	if err != nil {
		t.Fatalf("read prepared config: %v", err)
	}
	var obj struct {
		Hostname        string
		KernelExtraArgs []string
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("prepared config is not valid JSON: %v", err)
	}
	if len(obj.KernelExtraArgs) != 0 {
		t.Errorf("KernelExtraArgs = %v, want none for a config that requested nothing", obj.KernelExtraArgs)
	}
	if obj.Hostname != "ze" {
		t.Errorf("Hostname = %q, want it preserved", obj.Hostname)
	}
}
