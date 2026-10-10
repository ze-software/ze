// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: register.go -- registers this area beside `setup`
// Related: ../interoplab/kernelcheck.go -- DockerKernel, the check `check` runs
// Related: ../test/qemu/dockerlab.go -- the macOS route, which runs `check` in the guest, and BuildLinuxZe, the ze `check` probes with
// Related: ../test/deployment/gokrazyimage.go -- RuntimeKernelCacheDir, the kernel `install` places
//
// dockerkernel.go is `le setup docker-kernel`: the Docker daemon's kernel must
// carry every feature Ze enrolls (owner D-4). `check` asks the daemon in hand.
// `install` puts Ze's runtime kernel on a Linux Docker host and makes it GRUB's
// default, through sudo, one stated step at a time, and never reboots (D-6). It
// runs no root step until the operator confirms the release by name, and it
// installs only a release carrying the CONFIG_LOCALVERSION suffix the runtime
// kernel config declares, so a reinstall replaces Ze's own tree and nothing
// else. On
// macOS the labs run inside the Ze-kernel QEMU guest instead, so `install`
// refuses there and names that route.

package setup

import (
	"context"
	"debug/elf"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testdeployment "github.com/ze-software/ze/internal/le/test/deployment"
	testqemu "github.com/ze-software/ze/internal/le/test/qemu"
)

// dockerKernelArea is the name this area is typed as.
const dockerKernelArea = "setup docker-kernel"

const (
	dockerKernelCheckVerb   = "check"
	dockerKernelInstallVerb = "install"
	dockerKernelZeKeyword   = "ze"
	// dockerKernelConfirmKeyword takes the release the operator is about to
	// install. Passwordless sudo asks nothing, so this is the one question
	// every root step waits on.
	dockerKernelConfirmKeyword = "confirm"
	// dockerKernelCheckTimeout bounds the release query and the probe
	// container, whose own bound is two minutes.
	dockerKernelCheckTimeout = 3 * time.Minute
	grubConfigPath           = "/boot/grub/grub.cfg"
	grubDefaultsPath         = "/etc/default/grub"
)

var dockerKernelActions = leaction.New(dockerKernelArea,
	leaction.Action{
		Verb: dockerKernelCheckVerb,
		Why: "ask the Docker daemon in hand whether its kernel carries every feature Ze enrolls," +
			" probing with the linux ze at `ze`, or without `ze` with the one it cross-builds for the" +
			" daemon's architecture at tmp/qemu/linux-<arch>/ze; exits 1 naming each missing feature",
		Parameters: []leaction.Parameter{
			{Keyword: dockerKernelZeKeyword, Value: "linux-ze-path", Requirement: leaction.Optional},
		},
		AnswerArgs: runDockerKernelCheck,
	},
	leaction.Action{
		Verb:   dockerKernelInstallVerb,
		Writes: true,
		Why: "Linux only: install Ze's cached runtime kernel for this host's architecture under /boot" +
			" and /lib/modules, rebuild the initramfs and the GRUB menu, and make it GRUB's saved" +
			" default by title. Without `confirm <release>` it prints the steps and runs none; every step" +
			" runs through sudo and is printed first; it never reboots. " +
			interoplab.DockerKernelRoute("darwin"),
		Parameters: []leaction.Parameter{
			{Keyword: dockerKernelConfirmKeyword, Value: "release", Requirement: leaction.Optional},
		},
		AnswerArgs: runDockerKernelInstall,
	},
	leaction.Action{
		Verb:   dockerKernelAppArmorVerb,
		Writes: true,
		Why: "Linux only: install and load Ze's AppArmor profile " + kernelcap.ProbeAppArmorProfileName +
			", which the Docker kernel check runs its probe under on a daemon applying AppArmor (docker-default" +
			" denies the probe's mount and /proc/sys write). Without `confirm " + kernelcap.ProbeAppArmorProfileName +
			"` it prints the steps and runs none; every step runs through sudo and is printed first",
		Parameters: []leaction.Parameter{
			{Keyword: dockerKernelConfirmKeyword, Value: "profile", Requirement: leaction.Optional},
		},
		AnswerArgs: runDockerKernelAppArmor,
	},
)

// DockerKernelActions answers the area's command surface as data.
func DockerKernelActions() leaction.List { return dockerKernelActions.Actions() }

// DockerKernelSubs is the one-line hint help renders under the command.
func DockerKernelSubs() string { return dockerKernelActions.Subs() }

// DockerKernelAnswer is the `le setup docker-kernel` command.
func DockerKernelAnswer(args []string) (any, int) { return dockerKernelActions.Answer(args) }

// runDockerKernelCheck runs the one Docker kernel check every Docker run that
// runs Ze makes, against the daemon this process reaches, probing with a linux
// ze for that daemon's architecture.
func runDockerKernelCheck(args leaction.Arguments) (any, int) {
	docker := interoplab.NewDocker()
	ze, err := dockerKernelCheckZe(docker, args.One(dockerKernelZeKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerKernelCheckTimeout)
	defer cancel()

	if err := interoplab.DockerKernel(ze)(ctx, docker); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{"verdict": "pass", "ze": ze}, 0
}

// dockerKernelCheckZe asks the daemon its architecture, then answers the ze to
// mount. The build runs outside the check's own bound, because a cold
// cross-compile can outlast it.
func dockerKernelCheckZe(docker *interoplab.Docker, named string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerKernelCheckTimeout)
	defer cancel()

	goarch, err := docker.ServerArchitecture(ctx)
	if err != nil {
		return "", err
	}
	return dockerKernelProbeZe(named, goarch, buildLinuxZe)
}

// buildLinuxZe is the producer docker-lab uses, bound to this checkout.
func buildLinuxZe(goarch string) (string, error) {
	root, err := lepath.Root()
	if err != nil {
		return "", err
	}
	return testqemu.BuildLinuxZe(root, goarch)
}

// dockerKernelProbeZe answers the ze the probe container mounts: named when the
// operator gave one, else what build writes for goarch. Either way the file must
// be a linux executable for goarch, because the container runs it on the
// daemon's kernel and anything else answers only exit 127 or "exec format
// error" (owner's Linux run, 2026-10-10: the checkout itself was mounted).
func dockerKernelProbeZe(named, goarch string, build func(goarch string) (string, error)) (string, error) {
	ze := named
	if ze == "" {
		built, err := build(goarch)
		if err != nil {
			return "", err
		}
		ze = built
	}
	absolute, err := filepath.Abs(ze)
	if err != nil {
		return "", err
	}
	if err := linuxExecutableFor(absolute, goarch); err != nil {
		var refusal textbuf.Buffer
		return "", errors.New(refusal.Str("the Docker kernel check mounts ").Str(absolute).
			Str(" as the ze it probes with, and ").Err(err).
			Str("; omit `ze` and the check builds ").Str(testqemu.GuestZeRel(goarch)).
			Str(" for the daemon's ").Str(goarch).String())
	}
	return absolute, nil
}

// elfMachines maps the GOARCH a Docker daemon reports to the ELF machine a Go
// build for it carries. An architecture missing here is refused by name, never
// passed unchecked.
var elfMachines = map[string]elf.Machine{
	"386":     elf.EM_386,
	"amd64":   elf.EM_X86_64,
	"arm":     elf.EM_ARM,
	"arm64":   elf.EM_AARCH64,
	"loong64": elf.EM_LOONGARCH,
	"ppc64le": elf.EM_PPC64,
	"riscv64": elf.EM_RISCV,
	"s390x":   elf.EM_S390,
}

// linuxExecutableFor answers why path is not an ELF executable for goarch, or
// nil when it is one.
func linuxExecutableFor(path, goarch string) error {
	want, known := elfMachines[goarch]
	if !known {
		var unknown textbuf.Buffer
		return errors.New(unknown.Str("the daemon's architecture ").Str(goarch).
			Str(" has no ELF machine this check knows").String())
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("it is a directory, not a program")
	}
	file, err := elf.Open(path)
	if err != nil {
		var unread textbuf.Buffer
		return errors.New(unread.Str("it is not an ELF executable (").Err(err).Str(")").String())
	}
	defer file.Close() //nolint:errcheck // read-only; nothing to flush

	if file.Machine == want {
		return nil
	}
	var other textbuf.Buffer
	return errors.New(other.Str("it is built for ").Str(file.Machine.String()).Str(", not the daemon's ").
		Str(goarch).Str(" (").Str(want.String()).Str(")").String())
}

// installStep is one root step of the install, with the reason printed before
// it runs.
type installStep struct {
	Why  string
	Argv []string
}

// runDockerKernelInstall refuses off Linux, without GRUB_DEFAULT=saved, for a
// cache entry whose release lacks Ze's suffix, and without the release
// confirmed, before any step; then it places the kernel, rebuilds the boot
// files, and selects the new GRUB entry by title. The reboot is the operator's.
func runDockerKernelInstall(args leaction.Arguments) (any, int) {
	if err := dockerKernelInstallPlatform(runtime.GOOS); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	defaults, err := os.ReadFile(grubDefaultsPath)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := grubDefaultSaved(string(defaults)); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	cache, err := testdeployment.RuntimeKernelCacheDir(root, runtime.GOARCH, os.Stderr)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	release, err := cachedKernelRelease(cache)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	steps := dockerKernelInstallSteps(cache, release)
	if err := installConfirmed(args, release); err != nil {
		if planErr := printInstallPlan(steps); planErr != nil {
			leaction.ReportError(planErr)
		}
		leaction.ReportError(err)
		return nil, 1
	}
	for _, step := range steps {
		if err := runInstallStep(step); err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
	}
	grubCfg, err := os.ReadFile(grubConfigPath)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	step, err := grubDefaultStep(string(grubCfg), release)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if err := runInstallStep(step); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{
		"release": release,
		"next":    "reboot when ready, then: ./le setup docker-kernel check",
	}, 0
}

// dockerKernelInstallPlatform refuses every platform but Linux, naming the
// route the Mac takes instead (D-6).
func dockerKernelInstallPlatform(goos string) error {
	if goos == "linux" {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("`le setup docker-kernel install` changes a Linux Docker host's boot kernel and runs on Linux only. ").
		Str(interoplab.DockerKernelRoute(goos)).String())
}

// runtimeKernelRebuild is what every refusal of a cache entry tells the
// operator to run.
const runtimeKernelRebuild = "./ze appliance kernel --target runtime --arch " + runtime.GOARCH

// cachedKernelRelease answers the release a runtime-kernel cache entry holds:
// the one directory under lib/modules, which every installed path is named for.
// The release MUST end with the CONFIG_LOCALVERSION suffix the entry's own
// config declares. A bare upstream release such as 7.2.0 names files another
// kernel of that release owns, and a reinstall removes the trees it replaces.
func cachedKernelRelease(cache string) (string, error) {
	modules := filepath.Join(cache, "lib", "modules")
	entries, err := os.ReadDir(modules)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str(modules).Str(" holds ").Int(int64(len(entries))).
			Str(" entries, want one release directory").String())
	}
	release := entries[0].Name()

	config, err := os.ReadFile(filepath.Join(cache, "config")) //nolint:gosec // a file inside the cache entry this command resolved
	if err != nil {
		return "", err
	}
	suffix, err := kernelLocalVersion(string(config))
	if err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("the runtime kernel in ").Str(cache).Str(" is release ").Str(release).
			Str(": ").Err(err).Str("; installed under a bare release it would replace another kernel of that name.").
			Str(" gokrazy/kernel/runtime.config declares the suffix; rebuild with: ").Str(runtimeKernelRebuild).String())
	}
	if !strings.HasSuffix(release, suffix) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("the runtime kernel in ").Str(cache).Str(" is release ").Str(release).
			Str(", which does not end with its CONFIG_LOCALVERSION ").Str(suffix).
			Str(" (gokrazy/kernel/runtime.config); rebuild with: ").Str(runtimeKernelRebuild).String())
	}
	return release, nil
}

// kernelLocalVersion answers the CONFIG_LOCALVERSION a kernel config sets, and
// refuses a config that sets none or sets it empty: the suffix is what makes
// the release Ze's own.
func kernelLocalVersion(config string) (string, error) {
	for line := range strings.SplitSeq(config, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "CONFIG_LOCALVERSION=")
		if !ok {
			continue
		}
		suffix := strings.Trim(value, `"`)
		if suffix == "" {
			return "", errors.New("its config sets CONFIG_LOCALVERSION empty")
		}
		return suffix, nil
	}
	return "", errors.New("its config sets no CONFIG_LOCALVERSION")
}

// installConfirmed refuses unless the operator named, after `confirm`, the
// release the install is about to place, and names the command that does.
func installConfirmed(args leaction.Arguments, release string) error {
	if args.One(dockerKernelConfirmKeyword) == release {
		return nil
	}
	var tb textbuf.Buffer
	return errors.New(tb.Str("the steps above, then saving its GRUB menu entry as the default, run as root and none ran;" +
		" to run them, confirm the release: ./le setup docker-kernel install ").
		Str(dockerKernelConfirmKeyword).Byte(' ').Str(release).String())
}

// printInstallPlan prints every step and its reason without running any.
func printInstallPlan(steps []installStep) error {
	for _, step := range steps {
		if err := printInstallStep(step); err != nil {
			return err
		}
	}
	return nil
}

// dockerKernelInstallSteps answers the root steps that place the cached kernel
// and rebuild the boot files (Debian and Ubuntu tools). Selecting the entry is
// a separate step, because its title exists only once update-grub has run.
// Every removal is named for release, which carries Ze's suffix
// (cachedKernelRelease), so it reaches only what an earlier install wrote.
func dockerKernelInstallSteps(cache, release string) []installStep {
	return []installStep{
		{Why: "install the kernel image", Argv: []string{"sudo", "install", "-m", "0644",
			filepath.Join(cache, "vmlinuz"), "/boot/vmlinuz-" + release}},
		{Why: "install its config, which update-initramfs reads", Argv: []string{"sudo", "install", "-m", "0644",
			filepath.Join(cache, "config"), "/boot/config-" + release}},
		{Why: "remove the modules an earlier install of this release left, so the copy replaces them rather than nesting inside them",
			Argv: []string{"sudo", "rm", "-rf", "/lib/modules/" + release}},
		{Why: "install its modules", Argv: []string{"sudo", "cp", "-a",
			filepath.Join(cache, "lib", "modules", release), "/lib/modules/" + release}},
		{Why: "remove the initramfs an earlier install of this release built, which update-initramfs -c does not replace",
			Argv: []string{"sudo", "rm", "-f", "/boot/initrd.img-" + release}},
		{Why: "build its initramfs", Argv: []string{"sudo", "update-initramfs", "-c", "-k", release}},
		{Why: "add it to the GRUB menu", Argv: []string{"sudo", "update-grub"}},
	}
}

// grubDefaultSaved refuses a GRUB_DEFAULT that is not `saved`: a numeric one
// boots whatever entry lands at that index after the next menu change.
func grubDefaultSaved(defaults string) error {
	for line := range strings.SplitSeq(defaults, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "GRUB_DEFAULT=")
		if !ok {
			continue
		}
		if strings.Trim(value, `"'`) == "saved" {
			return nil
		}
	}
	return errors.New("set GRUB_DEFAULT=saved in " + grubDefaultsPath +
		" and run `sudo update-grub`, then rerun: the install selects Ze's kernel by its menu title")
}

// grubDefaultStep finds the menu entry that boots release (not its recovery
// entry) in grub.cfg and answers the step that saves it as the default, by
// title, prefixed by its submenu title when it sits in one.
func grubDefaultStep(grubCfg, release string) (installStep, error) {
	submenu := ""
	for line := range strings.SplitSeq(grubCfg, "\n") {
		if strings.HasPrefix(line, "submenu ") {
			submenu = grubTitle(line)
			continue
		}
		if line == "}" {
			submenu = ""
			continue
		}
		title := ""
		if strings.HasPrefix(strings.TrimSpace(line), "menuentry ") {
			title = grubTitle(strings.TrimSpace(line))
		}
		if !strings.HasSuffix(title, "Linux "+release) {
			continue
		}
		if submenu != "" {
			title = submenu + ">" + title
		}
		return installStep{Why: "make it GRUB's saved default", Argv: []string{"sudo", "grub-set-default", title}}, nil
	}
	return installStep{}, errors.New(grubConfigPath + " has no menu entry ending `Linux " + release + "`")
}

// grubTitle answers the first single-quoted string of a menuentry or submenu
// line, which is its title.
func grubTitle(line string) string {
	_, rest, ok := strings.Cut(line, "'")
	if !ok {
		return ""
	}
	title, _, ok := strings.Cut(rest, "'")
	if !ok {
		return ""
	}
	return title
}

// runInstallStep prints the step and its reason, then runs it with the
// terminal attached, so sudo can ask for a password.
func runInstallStep(step installStep) error {
	if err := printInstallStep(step); err != nil {
		return err
	}
	command := exec.Command(step.Argv[0], step.Argv[1:]...) //nolint:gosec,noctx // argv is this file's own; sudo may wait on a password prompt
	command.Stdin = os.Stdin
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str(strings.Join(step.Argv, " ")).Str(": ").Err(err).String())
	}
	return nil
}

// printInstallStep prints one step and its reason to stderr.
func printInstallStep(step installStep) error {
	var tb textbuf.Buffer
	line := tb.Str("setup docker-kernel: ").Str(step.Why).Str(": ").Str(strings.Join(step.Argv, " ")).Byte('\n').String()
	_, err := os.Stderr.WriteString(line)
	return err
}
