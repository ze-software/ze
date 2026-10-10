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
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testdeployment "github.com/ze-software/ze/internal/le/test/deployment"
)

const (
	dockerLabVerb = "docker-lab"
	keywordLab    = "lab"
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
	runArgs := leaction.Arguments{
		keywordCommand:  {dockerLabCommand(goarch, strings.Fields(args.One(keywordLab)))},
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

// dockerLabCommand is the shell line the guest runs from GuestWorkspace: start
// dockerd, wait a bounded time for it to answer, run the Docker kernel check
// with the guest le as the ze it probes with, then run the lab as `le <lab>`.
// `set -e` stops the line at the first step that fails, so no lab runs on a
// daemon that never answered or a kernel that failed the check.
func dockerLabCommand(goarch string, lab []string) string {
	guestLe := shellQuote(filepath.ToSlash(filepath.Join(GuestWorkspace, GuestLeRel(goarch))))
	var b textbuf.Buffer
	b.Str("set -e; rc-service docker start; ").
		Str("timeout ").Str(dockerReadySeconds).
		Str(" sh -c 'until docker info >/dev/null 2>&1; do sleep 1; done' || ").
		Str("{ echo 'dockerd did not answer within ").Str(dockerReadySeconds).Str("s' >&2; exit 1; }; ").
		Str(guestLe).Str(" setup docker-kernel check ze ").Str(guestLe)
	if len(lab) == 0 {
		return b.String()
	}
	b.Str("; ").Str(guestLe)
	for _, word := range lab {
		b.Byte(' ').Str(shellQuote(word))
	}
	return b.String()
}
