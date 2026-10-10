package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// cachedKernel writes a runtime-kernel cache entry holding the files the
// install reads: vmlinuz, config and one lib/modules/<release> tree.
func cachedKernel(t *testing.T, release string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib", "modules", release), 0o750); err != nil {
		t.Fatalf("make the modules tree: %v", err)
	}
	for _, name := range []string{"vmlinuz", "config"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// VALIDATES: D-6, the install is Linux only, and its refusal elsewhere names
// the route that platform takes: the Ze-kernel QEMU guest on macOS.
// PREVENTS: an install attempt on a Mac, whose Docker runs in a VM this action
// does not own.
func TestDockerKernelInstallRefusesOffLinux(t *testing.T) {
	err := dockerKernelInstallPlatform("darwin")
	if err == nil {
		t.Fatal("darwin was accepted")
	}
	for _, want := range []string{"Linux", "./le test qemu docker-lab"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %q: %v", want, err)
		}
	}
	if err := dockerKernelInstallPlatform("linux"); err != nil {
		t.Fatalf("linux refused: %v", err)
	}
}

// VALIDATES: AC-9, the install places the cached kernel, its config and its
// modules under the release the cache entry holds, rebuilds the initramfs and
// the GRUB menu, every step through sudo, and none of them reboots.
// PREVENTS: an install that reboots the operator's host, or a step that runs
// with root without saying so.
func TestDockerKernelInstallSteps(t *testing.T) {
	cache := cachedKernel(t, "7.2.0")
	release, err := cachedKernelRelease(cache)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if release != "7.2.0" {
		t.Fatalf("release = %q, want 7.2.0", release)
	}
	steps := dockerKernelInstallSteps(cache, release)
	want := [][]string{
		{"sudo", "install", "-m", "0644", filepath.Join(cache, "vmlinuz"), "/boot/vmlinuz-7.2.0"},
		{"sudo", "install", "-m", "0644", filepath.Join(cache, "config"), "/boot/config-7.2.0"},
		{"sudo", "cp", "-a", filepath.Join(cache, "lib", "modules", "7.2.0"), "/lib/modules/7.2.0"},
		{"sudo", "update-initramfs", "-c", "-k", "7.2.0"},
		{"sudo", "update-grub"},
	}
	if len(steps) != len(want) {
		t.Fatalf("%d steps, want %d: %v", len(steps), len(want), steps)
	}
	for index, step := range steps {
		if !slices.Equal(step.Argv, want[index]) {
			t.Errorf("step %d = %v, want %v", index, step.Argv, want[index])
		}
		if step.Why == "" {
			t.Errorf("step %d states no reason", index)
		}
		for _, word := range step.Argv {
			if strings.Contains(word, "reboot") || word == "shutdown" || word == "systemctl" {
				t.Errorf("step %d can restart the host: %v", index, step.Argv)
			}
		}
	}
}

// VALIDATES: a cache entry with no modules tree, or with two, is refused: the
// release is what every installed path is named for.
func TestDockerKernelReleaseNeedsOneModulesTree(t *testing.T) {
	if _, err := cachedKernelRelease(t.TempDir()); err == nil {
		t.Error("an entry with no lib/modules answered a release")
	}
	cache := cachedKernel(t, "7.2.0")
	if err := os.MkdirAll(filepath.Join(cache, "lib", "modules", "7.2.1"), 0o750); err != nil {
		t.Fatalf("second modules tree: %v", err)
	}
	if _, err := cachedKernelRelease(cache); err == nil {
		t.Error("an entry with two modules trees answered a release")
	}
}

// VALIDATES: the install selects the new kernel by its GRUB menu title, inside
// its submenu, and only when GRUB_DEFAULT is saved; otherwise it refuses before
// any step runs, naming the edit.
// PREVENTS: a numeric GRUB_DEFAULT that boots whatever entry lands at that index.
func TestDockerKernelGrubDefault(t *testing.T) {
	grubCfg := `menuentry 'Ubuntu' --class ubuntu {
	linux /boot/vmlinuz-6.8.0-117-generic
}
submenu 'Advanced options for Ubuntu' $menuentry_id_option 'gnulinux-advanced' {
	menuentry 'Ubuntu, with Linux 7.2.0' --class ubuntu {
		linux /boot/vmlinuz-7.2.0
	}
	menuentry 'Ubuntu, with Linux 7.2.0 (recovery mode)' --class ubuntu {
		linux /boot/vmlinuz-7.2.0 single
	}
	menuentry 'Ubuntu, with Linux 6.8.0-117-generic' --class ubuntu {
		linux /boot/vmlinuz-6.8.0-117-generic
	}
}
`
	step, err := grubDefaultStep(grubCfg, "7.2.0")
	if err != nil {
		t.Fatalf("grub default: %v", err)
	}
	want := []string{"sudo", "grub-set-default", "Advanced options for Ubuntu>Ubuntu, with Linux 7.2.0"}
	if !slices.Equal(step.Argv, want) {
		t.Fatalf("step = %v, want %v", step.Argv, want)
	}
	if _, err := grubDefaultStep(grubCfg, "7.3.0"); err == nil {
		t.Error("a release with no menu entry answered a step")
	}

	if err := grubDefaultSaved("GRUB_TIMEOUT=0\nGRUB_DEFAULT=saved\n"); err != nil {
		t.Errorf("GRUB_DEFAULT=saved refused: %v", err)
	}
	if err := grubDefaultSaved("GRUB_DEFAULT=\"saved\"\n"); err != nil {
		t.Errorf("quoted saved refused: %v", err)
	}
	for _, content := range []string{"GRUB_DEFAULT=0\n", "GRUB_TIMEOUT=5\n", "# GRUB_DEFAULT=saved\n"} {
		err := grubDefaultSaved(content)
		if err == nil {
			t.Errorf("%q accepted", content)
			continue
		}
		if !strings.Contains(err.Error(), "GRUB_DEFAULT=saved") {
			t.Errorf("refusal does not name the edit: %v", err)
		}
	}
}

// VALIDATES: the docker-kernel area is reachable with its two actions, check
// taking the ze to probe with.
func TestDockerKernelActionsAreRegistered(t *testing.T) {
	verbs := map[string][]string{}
	for _, action := range DockerKernelActions().Actions {
		for _, parameter := range action.Parameters {
			verbs[action.Verb] = append(verbs[action.Verb], parameter.Keyword)
		}
		if _, ok := verbs[action.Verb]; !ok {
			verbs[action.Verb] = nil
		}
	}
	if _, ok := verbs[dockerKernelInstallVerb]; !ok {
		t.Errorf("no %s action: %v", dockerKernelInstallVerb, verbs)
	}
	if !slices.Contains(verbs[dockerKernelCheckVerb], dockerKernelZeKeyword) {
		t.Errorf("%s does not take %q: %v", dockerKernelCheckVerb, dockerKernelZeKeyword, verbs)
	}
}
