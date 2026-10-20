package dockerkernel

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// VALIDATES: the install asks ze-host for the cache entry of the backend the
// operator named after `builder`, and for ze-host's own choice when none.
// PREVENTS: a kernel built with QEMU, while Docker is on PATH, being
// unreachable by the install, which then looks only in the docker entry.
func TestDockerKernelInstallAsksForTheNamedBuilder(t *testing.T) {
	saved := runtimeKernelCacheDir
	t.Cleanup(func() { runtimeKernelCacheDir = saved })
	var asked []string
	runtimeKernelCacheDir = func(_ context.Context, tree, arch, builder string, _ io.Writer) (string, error) {
		if tree != "/checkout" || arch != runtime.GOARCH {
			t.Errorf("asked for tree %q arch %q, want /checkout %s", tree, arch, runtime.GOARCH)
		}
		asked = append(asked, builder)
		return "/cache/" + builder, nil
	}
	for _, builder := range []string{"qemu", ""} {
		cache, err := installCache(t.Context(), "/checkout", builder)
		if err != nil || cache != "/cache/"+builder {
			t.Errorf("builder %q: cache %q, err %v", builder, cache, err)
		}
	}
	if !slices.Equal(asked, []string{"qemu", ""}) {
		t.Errorf("ze-host was asked for backends %q, want [qemu \"\"]", asked)
	}
}

// VALIDATES: the confirming command the dry run prints keeps the `builder` the
// operator named, keyword before value and before `confirm`.
// PREVENTS: a confirm that drops the backend and resolves another cache entry.
func TestDockerKernelInstallConfirmKeepsTheBuilder(t *testing.T) {
	err := installConfirmed(leaction.Arguments{dockerKernelBuilderKeyword: {"qemu"}}, "7.2.9-ze")
	if err == nil {
		t.Fatal("no confirm was accepted")
	}
	if !slices.Contains(strings.Split(err.Error(), "\n"), "  ./le setup docker-kernel install builder qemu confirm 7.2.9-ze") {
		t.Errorf("refusal does not carry the confirming command with its builder: %v", err)
	}
}

// VALIDATES: a refusal of the cache entry names the rebuild with the backend
// whose entry the install resolved, for the suffix check and the host check.
// PREVENTS: telling the operator to build with ze-host's default backend,
// which fills an entry the install with `builder qemu` never reads.
func TestDockerKernelRefusalsRebuildWithTheBuilder(t *testing.T) {
	want := "appliance kernel --target runtime --arch " + runtime.GOARCH + " --builder qemu"
	if _, err := cachedKernelRelease(cachedKernel(t, "7.2.9", "-ze"), "qemu"); err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("suffix refusal does not rebuild with qemu: %v", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sys", "class", "net", "ens18"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proc", "modules"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := cachedKernelServesHost(cachedKernel(t, "7.2.9-ze", "-ze"), "7.2.9-ze", root, "qemu")
	if err == nil {
		t.Fatal("a kernel without PCI hotplug was accepted for a slot-named interface")
	}
	if !strings.Contains(err.Error(), "CONFIG_HOTPLUG_PCI") || !strings.Contains(err.Error(), want) {
		t.Errorf("host refusal does not name the options and the qemu rebuild: %v", err)
	}
}
