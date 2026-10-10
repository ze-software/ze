// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: register.go -- registers this area beside `setup`
// Related: ../interoplab/kernelcheck.go -- DockerKernel, the check `check` runs
// Related: ../test/qemu/dockerlab.go -- the macOS route, which runs `check` in the guest
// Related: ../test/deployment/gokrazyimage.go -- RuntimeKernelCacheDir, the kernel `install` places
//
// dockerkernel.go is `le setup docker-kernel`: the Docker daemon's kernel must
// carry every feature Ze enrolls (owner D-4). `check` asks the daemon in hand.
// `install` puts Ze's runtime kernel on a Linux Docker host and makes it GRUB's
// default, through sudo, one stated step at a time, and never reboots (D-6). On
// macOS the labs run inside the Ze-kernel QEMU guest instead, so `install`
// refuses there and names that route.

package setup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testdeployment "github.com/ze-software/ze/internal/le/test/deployment"
)

// dockerKernelArea is the name this area is typed as.
const dockerKernelArea = "setup docker-kernel"

const (
	dockerKernelCheckVerb   = "check"
	dockerKernelInstallVerb = "install"
	dockerKernelZeKeyword   = "ze"
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
			" probing with the linux ze at `ze`; exits 1 naming each missing feature",
		Parameters: []leaction.Parameter{
			{Keyword: dockerKernelZeKeyword, Value: "linux-ze-path", Requirement: leaction.Required},
		},
		AnswerArgs: runDockerKernelCheck,
	},
	leaction.Action{
		Verb:   dockerKernelInstallVerb,
		Writes: true,
		Why: "Linux only: install Ze's cached runtime kernel for this host's architecture under /boot" +
			" and /lib/modules, rebuild the initramfs and the GRUB menu, and make it GRUB's saved" +
			" default by title. Every step runs through sudo and is printed first; it never reboots. " +
			interoplab.DockerKernelRoute("darwin"),
		Answer: runDockerKernelInstall,
	},
)

// DockerKernelActions answers the area's command surface as data.
func DockerKernelActions() leaction.List { return dockerKernelActions.Actions() }

// DockerKernelSubs is the one-line hint help renders under the command.
func DockerKernelSubs() string { return dockerKernelActions.Subs() }

// DockerKernelAnswer is the `le setup docker-kernel` command.
func DockerKernelAnswer(args []string) (any, int) { return dockerKernelActions.Answer(args) }

// runDockerKernelCheck runs the one Docker kernel check every Docker run that
// runs Ze makes, against the daemon this process reaches.
func runDockerKernelCheck(args leaction.Arguments) (any, int) {
	ze, err := filepath.Abs(args.One(dockerKernelZeKeyword))
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerKernelCheckTimeout)
	defer cancel()

	if err := interoplab.DockerKernel(ze)(ctx, interoplab.NewDocker()); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return map[string]string{"verdict": "pass", "ze": ze}, 0
}

// installStep is one root step of the install, with the reason printed before
// it runs.
type installStep struct {
	Why  string
	Argv []string
}

// runDockerKernelInstall refuses off Linux and without GRUB_DEFAULT=saved,
// before any step; then it places the kernel, rebuilds the boot files, and
// selects the new GRUB entry by title. The reboot is the operator's.
func runDockerKernelInstall() (any, int) {
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
	for _, step := range dockerKernelInstallSteps(cache, release) {
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
		"next":    "reboot when ready, then: ./le setup docker-kernel check ze <linux ze>",
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

// cachedKernelRelease answers the release a runtime-kernel cache entry holds:
// the one directory under lib/modules, which every installed path is named for.
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
	return entries[0].Name(), nil
}

// dockerKernelInstallSteps answers the root steps that place the cached kernel
// and rebuild the boot files (Debian and Ubuntu tools). Selecting the entry is
// a separate step, because its title exists only once update-grub has run.
func dockerKernelInstallSteps(cache, release string) []installStep {
	return []installStep{
		{Why: "install the kernel image", Argv: []string{"sudo", "install", "-m", "0644",
			filepath.Join(cache, "vmlinuz"), "/boot/vmlinuz-" + release}},
		{Why: "install its config, which update-initramfs reads", Argv: []string{"sudo", "install", "-m", "0644",
			filepath.Join(cache, "config"), "/boot/config-" + release}},
		{Why: "install its modules", Argv: []string{"sudo", "cp", "-a",
			filepath.Join(cache, "lib", "modules", release), "/lib/modules/" + release}},
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
	var tb textbuf.Buffer
	line := tb.Str("setup docker-kernel: ").Str(step.Why).Str(": ").Str(strings.Join(step.Argv, " ")).Byte('\n').String()
	if _, err := os.Stderr.WriteString(line); err != nil {
		return err
	}
	command := exec.Command(step.Argv[0], step.Argv[1:]...) //nolint:gosec,noctx // argv is this file's own; sudo may wait on a password prompt
	command.Stdin = os.Stdin
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		tb.Reset()
		return errors.New(tb.Str(strings.Join(step.Argv, " ")).Str(": ").Err(err).String())
	}
	return nil
}
