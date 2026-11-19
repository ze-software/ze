package testqemu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubCachedRuntimeKernel makes the runtime kernel lookup of an appliance
// image build answer a warm entry, for a test whose fake runner never spawns a
// real `ze appliance build`. The lookup reads the real kernel cache, which a
// unit test neither fills nor depends on. Not safe beside a parallel test: it
// writes a package variable, restored when t ends.
func stubCachedRuntimeKernel(t *testing.T) {
	t.Helper()
	setCachedRuntimeKernel(t, func(string, string) (string, error) {
		return "/stub/runtime-kernel", nil
	})
}

// setCachedRuntimeKernel replaces the lookup with lookup until t ends.
func setCachedRuntimeKernel(t *testing.T, lookup func(tree, arch string) (string, error)) {
	t.Helper()
	original := cachedRuntimeKernel
	cachedRuntimeKernel = lookup
	t.Cleanup(func() { cachedRuntimeKernel = original })
}

// TestColdRuntimeKernelRefusesBeforeTheApplianceBuild proves an installer
// image proof refuses a runtime kernel the cache does not hold before it
// spawns anything, with the real lookup's refusal: the entry's path and the
// command that builds the kernel.
//
// VALIDATES: a cold cache stops Installer.buildImage before the host ze build
// and `ze appliance build`, naming the cache entry and
// buildhostdriver.RuntimeKernelBuild's command on the last lines.
// PREVENTS: `ze appliance build` resolving its kernel through
// appliance.RuntimeKernelTree inside the proof's deadline, which builds a
// missing kernel for about thirty minutes, where a kill discards the build.
//
// Method: the checkout is the source tree, XDG_CACHE_HOME is an empty
// directory, so the key the lookup computes has no entry; the fake runner
// counts every command the proof would have spawned.
func TestColdRuntimeKernelRefusesBeforeTheApplianceBuild(t *testing.T) {
	installer := installTestInstaller(t, InstallKindHTTP)
	setCachedRuntimeKernel(t, cachedRuntimeKernelTree)
	tree, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	installer.Tree = tree
	installer.Options.Arch = runtime.GOARCH
	var spawned []commandSpec
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		spawned = append(spawned, spec)
		return commandResult{}, nil
	}

	_, err = installer.buildImage(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("a cold runtime kernel cache was answered, want a refusal")
	}
	if len(spawned) != 0 {
		t.Fatalf("the proof spawned %d commands before refusing, want none: %#v", len(spawned), spawned)
	}
	message := err.Error()
	if want := " is not in the cache at " + cacheHome + string(filepath.Separator); !strings.Contains(message, want) {
		t.Errorf("refusal does not name the entry under the cache %s:\n%s", cacheHome, message)
	}
	if want := "fix: " + runtimeKernelBuildCommand(runtime.GOARCH); !strings.HasSuffix(message, want) {
		t.Errorf("refusal does not end with %q:\n%s", want, message)
	}
	if !strings.Contains(message, "appliance kernel --target runtime --arch "+runtime.GOARCH) {
		t.Errorf("refusal does not name the runtime kernel build for %s:\n%s", runtime.GOARCH, message)
	}
	entries, err := os.ReadDir(cacheHome)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the refusal wrote into the cache: %v", entries)
	}
}

// TestEveryApplianceImageProofRefusesAColdKernelFirst proves each proof that
// spawns `ze appliance build` asks the runtime kernel cache before its first
// spawn, and returns the lookup's refusal unchanged.
//
// VALIDATES: Installer.buildImage, Installer.prepareISOImage (the ISO, OTA,
// Ventoy and config-push proofs), Hugepages.buildImage, CrashCapture.buildImage
// and buildSeededImage (MPLS boot and hostname) refuse before any command runs.
// PREVENTS: one image proof left building a missing kernel inside its deadline.
//
// Method: the lookup answers a sentinel refusal. The installer's fake runner
// counts commands; the Hugepages family runs a fake host ze that writes a
// marker file if it is ever started.
func TestEveryApplianceImageProofRefusesAColdKernelFirst(t *testing.T) {
	cold := errors.New("cold runtime kernel")
	cases := []struct {
		name  string
		build func(t *testing.T, host string) error
	}{
		{"installer image", func(t *testing.T, _ string) error {
			installer := refusingInstaller(t, cold)
			_, err := installer.buildImage(context.Background(), t.TempDir())
			return err
		}},
		{"installer iso image", func(t *testing.T, _ string) error {
			installer := refusingInstaller(t, cold)
			_, err := installer.prepareISOImage(context.Background(), t.TempDir())
			return err
		}},
		{"hugepages", func(t *testing.T, host string) error {
			_, err := newHugepages(t.TempDir()).buildImage(t.Context(), host, t.TempDir())
			return err
		}},
		{"crash capture", func(t *testing.T, host string) error {
			_, err := newCrashCapture(t.TempDir(), CrashLab(0)).buildImage(host, t.TempDir())
			return err
		}},
		{"mpls boot and hostname", func(t *testing.T, host string) error {
			_, err := newMPLSBoot(t.TempDir()).buildImage(host, t.TempDir(), "")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", "unix:///nonexistent/ze-test-docker-stub.sock")
			marker := filepath.Join(t.TempDir(), "spawned")
			t.Setenv("ZE_TEST_SPAWN_MARKER", marker)
			host := filepath.Join(t.TempDir(), "ze")
			script := "#!/bin/sh\ntouch \"$ZE_TEST_SPAWN_MARKER\"\nexit 0\n"
			if err := os.WriteFile(host, []byte(script), 0o700); err != nil { //nolint:gosec // an executable the test runs
				t.Fatal(err)
			}
			setCachedRuntimeKernel(t, func(string, string) (string, error) { return "", cold })

			err := tc.build(t, host)
			if !errors.Is(err, cold) {
				t.Fatalf("error = %v, want the lookup's refusal", err)
			}
			if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("the fake host ze ran before the refusal: %v", statErr)
			}
		})
	}
}

// refusingInstaller answers an installer whose kernel lookup answers cold and
// whose fake runner fails t on any command, so a proof that spawns before it
// asks the kernel cache goes red.
func refusingInstaller(t *testing.T, cold error) *Installer {
	t.Helper()
	installer := installTestInstaller(t, InstallKindISO)
	// installTestInstaller stubs a warm lookup; this proof needs the cold one.
	setCachedRuntimeKernel(t, func(string, string) (string, error) { return "", cold })
	installer.ops.Run = func(_ context.Context, spec commandSpec) (commandResult, error) {
		t.Errorf("command spawned before the kernel cache was asked: %#v", spec)
		return commandResult{}, errors.New("spawned")
	}
	return installer
}
