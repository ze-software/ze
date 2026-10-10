package setup

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// cachedKernel writes a runtime-kernel cache entry holding the files the
// install reads: vmlinuz, a config carrying CONFIG_LOCALVERSION=localVersion
// (none when empty) and one lib/modules/<release> tree.
func cachedKernel(t *testing.T, release, localVersion string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib", "modules", release), 0o750); err != nil {
		t.Fatalf("make the modules tree: %v", err)
	}
	config := "CONFIG_MODULES=y\n"
	if localVersion != "" {
		config += "CONFIG_LOCALVERSION=\"" + localVersion + "\"\n"
	}
	files := map[string]string{"vmlinuz": "vmlinuz", "config": config}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
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
// the GRUB menu, every step through sudo, and none of them reboots. A
// reinstall replaces: the modules tree and the initramfs of the same release
// are removed before they are written again.
// PREVENTS: an install that reboots the operator's host, a step that runs with
// root without saying so, and `cp -a` nesting the new modules inside the old
// tree (/lib/modules/<release>/<release>) while the old modules.dep stays.
func TestDockerKernelInstallSteps(t *testing.T) {
	cache := cachedKernel(t, "7.2.9-ze", "-ze")
	release, err := cachedKernelRelease(cache)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if release != "7.2.9-ze" {
		t.Fatalf("release = %q, want 7.2.9-ze", release)
	}
	steps := dockerKernelInstallSteps(cache, release)
	want := [][]string{
		{"sudo", "install", "-m", "0644", filepath.Join(cache, "vmlinuz"), "/boot/vmlinuz-7.2.9-ze"},
		{"sudo", "install", "-m", "0644", filepath.Join(cache, "config"), "/boot/config-7.2.9-ze"},
		{"sudo", "rm", "-rf", "/lib/modules/7.2.9-ze"},
		{"sudo", "cp", "-a", filepath.Join(cache, "lib", "modules", "7.2.9-ze"), "/lib/modules/7.2.9-ze"},
		{"sudo", "rm", "-f", "/boot/initrd.img-7.2.9-ze"},
		{"sudo", "update-initramfs", "-c", "-k", "7.2.9-ze"},
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

// VALIDATES: every path the install removes is named for Ze's own release, so
// a removal can only reach a tree an earlier install of this build wrote.
// PREVENTS: a removal step reaching another kernel's modules or initramfs.
func TestDockerKernelInstallRemovesOnlyItsOwnRelease(t *testing.T) {
	release := "7.2.9-ze"
	for _, step := range dockerKernelInstallSteps(cachedKernel(t, release, "-ze"), release) {
		if step.Argv[1] != "rm" {
			continue
		}
		target := step.Argv[len(step.Argv)-1]
		if !strings.HasSuffix(target, "-"+release) && !strings.HasSuffix(target, "/"+release) {
			t.Errorf("removal %v reaches %s, which is not named for %s", step.Argv, target, release)
		}
	}
}

// VALIDATES: the install takes only a release carrying the CONFIG_LOCALVERSION
// suffix its own build config declares, never a bare upstream release.
// PREVENTS: /boot/vmlinuz-7.2.0 and /lib/modules/7.2.0 overwriting, or being
// removed as, another kernel built from the same upstream release.
func TestDockerKernelReleaseNeedsTheZeSuffix(t *testing.T) {
	refused := map[string]*struct{ release, localVersion string }{
		"bare release, no suffix":       {"7.2.0", ""},
		"suffix declared, bare release": {"7.2.9", "-ze"},
		"release ends otherwise":        {"7.2.9-generic", "-ze"},
	}
	for name, entry := range refused {
		_, err := cachedKernelRelease(cachedKernel(t, entry.release, entry.localVersion))
		if err == nil {
			t.Errorf("%s: release %q accepted", name, entry.release)
			continue
		}
		for _, want := range []string{"CONFIG_LOCALVERSION", "gokrazy/kernel/runtime.config", "./ze appliance kernel --target runtime"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: refusal does not name %q: %v", name, want, err)
			}
		}
	}
}

// VALIDATES: the runtime kernel build config declares the release suffix the
// install requires, so a kernel Ze builds is one the install accepts.
// PREVENTS: the install refusing every kernel Ze builds, or a build that
// ships a bare upstream release name.
func TestRuntimeKernelConfigDeclaresALocalVersion(t *testing.T) {
	config, err := os.ReadFile(filepath.Join("..", "..", "..", "gokrazy", "kernel", "runtime.config"))
	if err != nil {
		t.Fatalf("read runtime.config: %v", err)
	}
	suffix, err := kernelLocalVersion(string(config))
	if err != nil {
		t.Fatalf("runtime.config: %v", err)
	}
	if !strings.HasPrefix(suffix, "-") {
		t.Errorf("CONFIG_LOCALVERSION = %q, want a suffix starting with a dash", suffix)
	}
}

// VALIDATES: the install runs no root step until the operator types the
// release it is about to install after `confirm`; without it, or with another
// release, it refuses naming the exact command that confirms.
// PREVENTS: one command running every root step on a host with passwordless
// sudo, where sudo asks nothing.
func TestDockerKernelInstallNeedsConfirmation(t *testing.T) {
	release := "7.2.9-ze"
	for name, args := range map[string]leaction.Arguments{
		"no confirm":    {},
		"wrong release": {dockerKernelConfirmKeyword: {"7.2.0"}},
		"empty confirm": {dockerKernelConfirmKeyword: {""}},
	} {
		err := installConfirmed(args, release)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !strings.Contains(err.Error(), "./le setup docker-kernel install confirm 7.2.9-ze") {
			t.Errorf("%s: refusal does not name the confirming command: %v", name, err)
		}
	}
	if err := installConfirmed(leaction.Arguments{dockerKernelConfirmKeyword: {release}}, release); err != nil {
		t.Errorf("the release confirmed was refused: %v", err)
	}
}

// VALIDATES: a cache entry with no modules tree, or with two, is refused: the
// release is what every installed path is named for.
func TestDockerKernelReleaseNeedsOneModulesTree(t *testing.T) {
	if _, err := cachedKernelRelease(t.TempDir()); err == nil {
		t.Error("an entry with no lib/modules answered a release")
	}
	cache := cachedKernel(t, "7.2.0-ze", "-ze")
	if err := os.MkdirAll(filepath.Join(cache, "lib", "modules", "7.2.1-ze"), 0o750); err != nil {
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
// taking the ze to probe with and install the release to confirm.
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
	if !slices.Contains(verbs[dockerKernelInstallVerb], dockerKernelConfirmKeyword) {
		t.Errorf("%s does not take %q: %v", dockerKernelInstallVerb, dockerKernelConfirmKeyword, verbs)
	}
	if !slices.Contains(verbs[dockerKernelCheckVerb], dockerKernelZeKeyword) {
		t.Errorf("%s does not take %q: %v", dockerKernelCheckVerb, dockerKernelZeKeyword, verbs)
	}
}
