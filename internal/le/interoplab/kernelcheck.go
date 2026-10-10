// Design: docs/architecture/testing/interop.md -- the Docker host kernel check
// Related: docker.go -- the Docker client the probe runs through
//
// A Docker lab runs Ze on the Docker daemon's kernel, so a kernel feature Ze
// needs and the daemon's kernel lacks fails scenarios for a host fact. The
// required set is not listed here: the staged ze answers it, by running
// `ze doctor --json kernel-capabilities` on the daemon's kernel, which probes
// every capability Ze enrolls (internal/component/kernelcap). A capability a
// package enrolls later is required with no change to this file.

package interoplab

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// kernelProbeImage only carries the staged static ze; any Linux userland
	// that runs a static binary answers the same, because the probes ask the
	// kernel, not the image.
	kernelProbeImage   = "alpine:3.21"
	kernelProbeTimeout = 2 * time.Minute
	// kernelProbeMount is where the staged ze appears inside the probe container.
	kernelProbeMount = "/ze"
)

// DockerKernelRoute answers the next step a refusal names for a reader on goos
// (owner D-6). On Linux the labs run on the host's Docker, so the step is a
// kernel with the feature or Ze's own (D-5) through the install action, which
// never reboots. Elsewhere the labs run inside the Alpine QEMU guest booted on
// Ze's runtime kernel, and the Linux-only install would refuse.
func DockerKernelRoute(goos string) string {
	if goos == "linux" {
		return "Run Docker on a kernel with every feature, or install Ze's kernel with " +
			"`./le setup docker-kernel install` and reboot."
	}
	return "On " + goos + ", run the lab inside the Ze-kernel QEMU guest: " +
		"`./le test qemu docker-lab lab \"<le words>\"`."
}

// kernelProbeCommand is what the probe container runs. It is named in every
// refusal, so an operator can run the same question by hand.
var kernelProbeCommand = []string{kernelProbeMount, "doctor", "--json", "kernel-capabilities"}

// kernelRow is one row of `ze doctor --json kernel-capabilities`, read at this
// boundary as JSON. The producer is kernelcap.Row (internal/component/kernelcap).
type kernelRow struct {
	Subsystem string `json:"subsystem"`
	Kernel    string `json:"kernel"`
	State     string `json:"state"`
	Reason    string `json:"reason"`
}

type kernelAnswer struct {
	Ready        bool        `json:"ready"`
	Capabilities []kernelRow `json:"capabilities"`
}

// DockerKernel returns the preflight that refuses a Docker daemon whose kernel
// lacks any kernel feature Ze enrolls. zePath is the staged linux ze for the
// daemon's architecture, so it runs after StageBinaries.
func DockerKernel(zePath string) PreflightCheck {
	return func(ctx context.Context, docker *Docker) error {
		return checkDockerKernel(ctx, docker, zePath)
	}
}

// StagedZePath answers the absolute path of the binary named ze among the
// binaries a lab stages, which is what Suite.StagedZe names. A lab that stages
// no ze answers empty, and Suite.Run refuses that answer rather than skipping.
func StagedZePath(root string, binaries []LabBinary) string {
	for _, binary := range binaries {
		if binary.Name == "ze" {
			return filepath.Join(root, binary.Output)
		}
	}
	return ""
}

// kernelProbeArgv is the probe container's command line. It grants NET_ADMIN
// for the netlink probes and SYS_ADMIN for the xfrm-interface probe, which
// unshares a network namespace, and gives the container its own network so
// nothing it creates reaches the host's.
func kernelProbeArgv(zePath string) []string {
	argv := make([]string, 0, 12+len(kernelProbeCommand))
	argv = append(argv, dockerExecutable, "run", "--rm",
		"--cap-add", "NET_ADMIN", "--cap-add", "SYS_ADMIN",
		"--network", "none",
		"-v", zePath+":"+kernelProbeMount+":ro",
		kernelProbeImage)
	return append(argv, kernelProbeCommand...)
}

// checkDockerKernel runs the probe and returns nil only when the answer holds at
// least one row and every row is present. Unknown is a refusal (owner D-4: a
// wrong kernel "should not be possible - fail"), and so is a probe that did not
// answer: the check never proceeds on a question it could not ask.
func checkDockerKernel(ctx context.Context, docker *Docker, zePath string) error {
	if docker == nil {
		return errors.New("the Docker kernel check needs a Docker client")
	}
	if zePath == "" {
		return errors.New("the Docker kernel check needs the staged ze to probe with, and this suite names none (Suite.StagedZe)")
	}

	release := dockerKernelRelease(ctx, docker)
	argv := kernelProbeArgv(zePath)
	result, err := docker.runner.Run(ctx, processCommand{Arguments: argv, Timeout: kernelProbeTimeout})

	var problem textbuf.Buffer
	problem.Str("the Docker daemon's kernel (").Str(release).Str(") could not be checked: `").
		Str(strings.Join(argv, " ")).Str("` ")
	if err != nil {
		return errors.New(problem.Str("failed: ").Err(err).String())
	}
	var answer kernelAnswer
	if jsonErr := json.Unmarshal([]byte(result.Stdout), &answer); jsonErr != nil {
		return errors.New(problem.Str("exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(", unreadable answer (").Err(jsonErr).Str("): stdout ").Str(strings.TrimSpace(result.Stdout)).
			Str("; stderr ").Str(strings.TrimSpace(result.Stderr)).String())
	}
	if len(answer.Capabilities) == 0 {
		return errors.New(problem.Str("answered no capability, so nothing about the kernel was asked").String())
	}

	var missing textbuf.Buffer
	count := 0
	for _, row := range answer.Capabilities {
		if row.State == "present" {
			continue
		}
		missing.Str("\n  ").Str(row.Subsystem).Str(" (").Str(row.Kernel).Str("): ").Str(row.State)
		if row.Reason != "" {
			missing.Str(": ").Str(row.Reason)
		}
		count++
	}
	if count == 0 {
		// Every row reads present, so the probe's own verdict must agree: the
		// producer (doctor writeKernelCapabilities) exits 0 with ready true only
		// then. A disagreement is a probe this check cannot read, never a pass.
		if answer.Ready && result.ExitCode == 0 {
			return nil
		}
		return errors.New(problem.Str("answered every row present but exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(", ready ").Str(strconv.FormatBool(answer.Ready)).Str(", so its verdict disagrees with its rows").String())
	}
	var refusal textbuf.Buffer
	refusal.Str("the Docker daemon's kernel ").Str(release).Str(" lacks ").Str(strconv.Itoa(count)).
		Str(" kernel feature(s) Ze needs (`").Str(strings.Join(kernelProbeCommand, " ")).Str("` answered):").
		Str(missing.String()).
		Byte('\n').Str(DockerKernelRoute(runtime.GOOS))
	return errors.New(refusal.String())
}

// dockerKernelRelease reads the daemon's kernel release for the refusal. It is
// context for the reader, never a verdict, so a failed read is spelled out in
// its place rather than stopping the check.
func dockerKernelRelease(ctx context.Context, docker *Docker) string {
	result, err := docker.runner.Run(ctx, processCommand{
		Arguments: []string{dockerExecutable, "info", "--format", "{{.KernelVersion}}"},
		Timeout:   dockerInfoTimeout,
	})
	if err != nil {
		var unread textbuf.Buffer
		return unread.Str("release unread: ").Err(err).String()
	}
	release := strings.TrimSpace(result.Stdout)
	if release == "" {
		return "release unread"
	}
	return release
}
