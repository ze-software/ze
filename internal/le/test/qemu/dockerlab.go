// Design: docs/architecture/testing/qemu-integration.md -- Docker labs in the Ze-kernel guest
// Related: run.go -- the guest boot this action plans
// Related: guestle.go -- the guest le that runs the check and the lab
// Related: ../deployment/gokrazyimage.go -- RuntimeKernelCacheDir, where the booted kernel is cached
// Related: ../../setup/dockerkernel.go -- `setup docker-kernel check`, run inside the guest
//
// dockerlab.go is the one path the Mac (HVF, arm64) and the scheduled nightly
// (KVM, amd64) share for a Docker lab that runs Ze (owner D-6): boot the Alpine
// guest on Ze's runtime kernel for the guest's architecture, start Docker in it,
// run the Docker kernel check there, then run the lab with the guest le.

package testqemu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/linuxle"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	testdeployment "github.com/ze-software/ze/internal/le/test/deployment"
)

const (
	dockerLabVerb = "docker-lab"
	keywordLab    = "lab"
	// keywordEnv carries NAME=value assignments the lab runs under, such as
	// IPSEC_INTEROP_SCENARIO=mobike-initiator: the suites select one scenario
	// from the environment, and the host's environment never reaches the guest.
	keywordEnv = "env"
	// dockerLabPackages are the guest's Alpine packages: docker carries dockerd
	// and its OpenRC service, iproute2 the full `ip` the labs' setup uses.
	dockerLabPackages = "docker iproute2"
	// dockerLabKernelFile is the kernel image inside a runtime-kernel cache
	// entry, beside kernel.version, config and lib/modules.
	dockerLabKernelFile = "vmlinuz"
	// dockerReadySeconds bounds the wait for dockerd to answer after OpenRC
	// starts it.
	dockerReadySeconds = "60"
)

// runDockerLabHere boots the guest on the cached runtime kernel and runs the lab
// named by `lab` inside it. With no lab it stops after the kernel check, which
// is the proof that the guest boots, Docker starts and the kernel passes.
func runDockerLabHere(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	goarch := GuestArch()
	cache, err := testdeployment.RuntimeKernelCacheDir(root, goarch, os.Stderr)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	kernel, err := dockerLabKernel(cache, goarch)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	lab := strings.Fields(args.One(keywordLab))
	environment, err := dockerLabEnvironment(args.One(keywordEnv), len(lab) > 0)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	runArgs := leaction.Arguments{
		keywordCommand:  {dockerLabCommand(goarch, lab, environment)},
		keywordKernel:   {kernel},
		keywordPackages: {dockerLabPackages},
	}
	if timeout := args.One(keywordTimeout); timeout != "" {
		runArgs[keywordTimeout] = []string{timeout}
	}
	options, err := parseRunArguments(runArgs)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	// A lab under TCG measures the emulator and times out; HVF or KVM or nothing.
	options.HardwareOnly = true
	if err := buildGuestLe(root, goarch); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	if _, err := BuildLinuxZe(root, goarch); err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	ctx, cancel := guestContext()
	defer cancel()

	report, err := NewRun(root, options).Execute(ctx)
	if err != nil {
		leaction.ReportError(err)
		return &report, 1
	}
	return &report, runExitCode(&report)
}

// dockerLabKernel answers the kernel image of the runtime-kernel cache entry,
// which the host ze names for the guest's architecture, and refuses an entry
// that holds none, naming the build that writes it. Nothing falls back to the
// Alpine kernel: its verdict would describe Alpine, not Ze.
func dockerLabKernel(cache, goarch string) (string, error) {
	kernel := filepath.Join(cache, dockerLabKernelFile)
	if _, err := os.Stat(kernel); err != nil {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("the Docker lab guest boots Ze's ").Str(goarch).
			Str(" runtime kernel from its cache entry, and ").Str(kernel).Str(" is not there (").Err(err).
			Str("); build it with: ./ze appliance kernel --target runtime --arch ").Str(goarch).String())
	}
	return kernel, nil
}

// GuestZeRel answers where the host writes the linux ze the Docker kernel check
// probes with, relative to the checkout the guest mounts at GuestWorkspace,
// beside the guest le. The nightly's informational check on the runner's own
// Docker probes with the same file, and so does `le setup docker-kernel check`
// on a Linux host when it is given no ze.
func GuestZeRel(goarch string) string {
	return filepath.Join("tmp", "qemu", "linux-"+goarch, "ze")
}

// BuildLinuxZe cross-builds the linux ze for goarch at GuestZeRel(goarch),
// through job admission, with the daemon's feature tags, and answers its
// absolute path. It is the one producer of the ze the Docker kernel check
// probes with: docker-lab builds it for the guest, and `le setup docker-kernel
// check` builds it for the daemon in hand. The check runs
// `ze doctor kernel-capabilities` in a container, and the guest le cannot stand
// in: started under the name ze, a ze_le build answers "unknown command:
// doctor" (first HVF boot, 2026-10-10).
func BuildLinuxZe(root, goarch string) (string, error) {
	toolchain, err := gotoolchain.New(root)
	if err != nil {
		return "", err
	}
	tags, err := repofeaturetags.DaemonBuildTags(root, repofeaturetags.DaemonBase)
	if err != nil {
		return "", err
	}
	admission, err := job.NewIn(root)
	if err != nil {
		return "", err
	}
	admission.Out = os.Stderr

	output := filepath.Join(root, GuestZeRel(goarch))
	environment := append(toolchain.Environment(gotoolchain.EnvOptions{GOOS: "linux", GOARCH: goarch}),
		linuxle.Overrides(goarch)...)
	argv := []string{"go", "build", tagsFlag, tags, "-o", output, zeMainPackage}
	if _, code := admission.Run("qemu-build-ze", argv, root, environment); code != 0 {
		return "", fmt.Errorf("build the linux ze for linux/%s exited %d", goarch, code)
	}
	return output, nil
}

// dockerLabEnvironment answers the NAME=value assignments of the `env` value,
// separated by spaces, and refuses an assignment whose name is not an upper-case
// shell variable name, or any assignment when no lab runs to read it.
func dockerLabEnvironment(value string, lab bool) ([]string, error) {
	assignments := strings.Fields(value)
	if len(assignments) == 0 {
		return nil, nil
	}
	if !lab {
		return nil, errors.New("docker-lab env names the environment of a lab, and no lab is named to run under it")
	}
	for _, assignment := range assignments {
		name, _, found := strings.Cut(assignment, "=")
		if !found || !environmentName(name) {
			return nil, fmt.Errorf("docker-lab env takes NAME=value with an upper-case NAME, not %q", assignment)
		}
	}
	return assignments, nil
}

// environmentName reports whether name is an upper-case shell variable name:
// a letter or underscore, then letters, digits or underscores.
func environmentName(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// dockerLabCommand is the shell line the guest runs from GuestWorkspace: start
// dockerd, wait a bounded time for it to answer, run the Docker kernel check
// probing with the guest ze, then run the lab as `le <lab>`, under `env` with
// the lab's assignments when it has any.
// `set -e` stops the line at the first step that fails, so no lab runs on a
// daemon that never answered or a kernel that failed the check.
func dockerLabCommand(goarch string, lab, environment []string) string {
	guestLe := shellQuote(filepath.ToSlash(filepath.Join(GuestWorkspace, GuestLeRel(goarch))))
	guestZe := shellQuote(filepath.ToSlash(filepath.Join(GuestWorkspace, GuestZeRel(goarch))))
	var b textbuf.Buffer
	b.Str("set -e; rc-service docker start; ").
		Str("timeout ").Str(dockerReadySeconds).
		Str(" sh -c 'until docker info >/dev/null 2>&1; do sleep 1; done' || ").
		Str("{ echo 'dockerd did not answer within ").Str(dockerReadySeconds).Str("s' >&2; exit 1; }; ").
		Str(guestLe).Str(" setup docker-kernel check ze ").Str(guestZe)
	if len(lab) == 0 {
		return b.String()
	}
	b.Str("; ")
	if len(environment) > 0 {
		b.Str("env")
		for _, assignment := range environment {
			b.Byte(' ').Str(shellQuote(assignment))
		}
		b.Byte(' ')
	}
	b.Str(guestLe)
	for _, word := range lab {
		b.Byte(' ').Str(shellQuote(word))
	}
	return b.String()
}
