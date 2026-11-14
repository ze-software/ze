// Design: docs/architecture/appliance/kernel-profiles.md -- what a Docker host relies on the Ze kernel for
// Related: dockerkernel.go -- dockerKernelInstallOn, the install body these tests drive
// Related: hostdeps.go -- hostUnreadable, the refusal for a host the check cannot read

package dockerkernel

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// recordingSudo stands in for sudo at the head of PATH: it records the root
// command it was given and runs nothing.
const recordingSudo = "#!/bin/sh\necho \"$@\" >> \"$SUDO_RECORD\"\n"

// stageInstallHost answers a host root holding the files the install reads
// before its first step: GRUB's defaults set to `saved`, one interface and an
// empty module list.
func stageInstallHost(t *testing.T, iface string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{filepath.Join("sys", "class", "net", iface), "proc", filepath.Join("etc", "default")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatalf("make %s: %v", dir, err)
		}
	}
	files := map[string]string{
		filepath.Join("proc", "modules"):        "",
		filepath.Join("etc", "default", "grub"): "GRUB_DEFAULT=saved\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

// runConfirmedInstall runs the install body, confirmed, for a Linux host under
// hostRoot whose cache entry is a kernel with no PCI hotplug. A recording sudo
// stands at the head of PATH, so no root command reaches the real host. It
// answers the exit code, what the install wrote to stderr, and the root
// commands it issued.
func runConfirmedInstall(t *testing.T, hostRoot string) (int, string, string) {
	t.Helper()
	release := pinnedRelease(t) + "-ze"
	cache := cachedKernel(t, release, "-ze")
	savedCache := runtimeKernelCacheDir
	t.Cleanup(func() { runtimeKernelCacheDir = savedCache })
	runtimeKernelCacheDir = func(context.Context, string, string, string, io.Writer) (string, error) {
		return cache, nil
	}

	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, sudoBin), []byte(recordingSudo), 0o755); err != nil { //nolint:gosec // a stub on a test's own PATH must be executable
		t.Fatalf("write the sudo stub: %v", err)
	}
	record := filepath.Join(bin, "record")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SUDO_RECORD", record)

	stderr, err := os.CreateTemp(t.TempDir(), "stderr-")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close() //nolint:errcheck // read by name below; a temporary output file
	savedStderr := os.Stderr
	os.Stderr = stderr
	_, code := dockerKernelInstallOn(t.Context(), leaction.Arguments{dockerKernelConfirmKeyword: {release}}, osLinux, hostRoot)
	os.Stderr = savedStderr

	diagnostic, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := os.ReadFile(record) //nolint:gosec // the test's own temp file
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return code, string(diagnostic), string(issued)
}

// VALIDATES: AC-22 through the install body. A confirmed install whose cached
// kernel lacks what the host relies on (a slot-named interface, a kernel with
// no PCI hotplug) refuses, names the dependency and the missing option, and
// issues no root command.
// METHOD: dockerKernelInstallOn runs confirmed over a staged host root, with
// the cache seam answering a kernel without CONFIG_HOTPLUG_PCI and a recording
// sudo at the head of PATH. The control is the same run for a host with no
// slot-named interface: it reaches its steps, so the recording sudo sees the
// first one. Without the control, "no root command" would also pass for an
// install that refused earlier for another reason.
// PREVENTS: the cachedKernelServesHost call leaving dockerKernelInstallOn, or
// moving behind the steps, with every test of the helper still green.
func TestDockerKernelInstallRefusesBeforeAnyStepForAHostTheKernelBreaks(t *testing.T) {
	const firstStep = "install -m 0644 "

	code, diagnostic, issued := runConfirmedInstall(t, stageInstallHost(t, "eth0"))
	if !strings.HasPrefix(issued, firstStep) {
		t.Fatalf("control: a host the kernel serves did not reach its first step (code %d):\nissued: %q\n%s", code, issued, diagnostic)
	}
	if strings.Contains(diagnostic, "lacks what this host relies on") {
		t.Errorf("control: a host with no slot-named interface was refused:\n%s", diagnostic)
	}

	code, diagnostic, issued = runConfirmedInstall(t, stageInstallHost(t, "ens18"))
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if issued != "" {
		t.Errorf("the install issued root commands for a host its kernel would break:\n%s", issued)
	}
	for _, want := range []string{"lacks what this host relies on", "no step ran", "ens18", "CONFIG_HOTPLUG_PCI"} {
		if !strings.Contains(diagnostic, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, diagnostic)
		}
	}
	if strings.Contains(diagnostic, "install the kernel image") {
		t.Errorf("the install printed or started a step before it refused:\n%s", diagnostic)
	}
}

// VALIDATES: a host source the check cannot read refuses the install in the
// operator's terms, as an operator error: no step ran, why the read matters,
// the path as evidence, and the install command, with its builder, as the fix
// on the last line. A cache entry with no readable config says the same and
// ends on the rebuild commands as its fix.
// PREVENTS: a bare `open /proc/modules: no such file or directory` as the
// whole message.
func TestDockerKernelInstallSaysWhyAnUnreadableHostIsRefused(t *testing.T) {
	release := pinnedRelease(t) + "-ze"
	root := stageInstallHost(t, "eth0")
	if err := os.Remove(filepath.Join(root, "proc", "modules")); err != nil {
		t.Fatal(err)
	}
	err := cachedKernelServesHost(cachedKernel(t, release, "-ze"), release, root, "qemu")
	if err == nil {
		t.Fatal("a host whose module list cannot be read was accepted")
	}
	for _, want := range []string{"the install was refused and no step ran", "no network or no firewall", filepath.Join(root, "proc", "modules")} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	lines := strings.Split(err.Error(), "\n")
	if got, want := lines[len(lines)-1], "fix: ./le setup docker kernel install builder qemu"; got != want {
		t.Errorf("last line = %q, want the install command %q", got, want)
	}
	if got, want := installRerun(""), "./le setup docker kernel install"; got != want {
		t.Errorf("rerun with no builder = %q, want %q", got, want)
	}

	err = cachedKernelServesHost(t.TempDir(), release, root, "qemu")
	if err == nil {
		t.Fatal("a cache entry with no config was accepted")
	}
	for _, want := range []string{"the install was refused and no step ran", release, "rebuild the cache entry"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
	if want := "\nfix: " + strings.TrimSpace(runtimeKernelRebuild("qemu")); !strings.HasSuffix(err.Error(), want) {
		t.Errorf("the refusal does not end on the rebuild commands %q: %v", want, err)
	}
}
