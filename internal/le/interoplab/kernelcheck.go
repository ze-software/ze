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
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/kernelcap"
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

// dockerSecurityOptionsFormat asks the daemon which security modules it applies
// to a container; a daemon applying AppArmor lists "name=apparmor".
const dockerSecurityOptionsFormat = "{{json .SecurityOptions}}"

// appArmorProfilesPath lists the AppArmor profiles a Linux kernel has loaded,
// one "name (mode)" per line.
const appArmorProfilesPath = "/sys/kernel/security/apparmor/profiles"

// appArmorProfiles answers the local kernel's loaded profile list, and false
// when it cannot be read: off Linux, or without AppArmor. A var so a unit test
// fakes the host.
var appArmorProfiles = readAppArmorProfiles

func readAppArmorProfiles() (string, bool) {
	if runtime.GOOS != "linux" {
		return "", false
	}
	data, err := os.ReadFile(appArmorProfilesPath)
	if err != nil {
		return "", false
	}
	return string(data), true
}

// appArmorProfileLoaded reports whether profiles, in the kernel's
// "name (mode)" lines, lists name.
func appArmorProfileLoaded(profiles, name string) bool {
	for line := range strings.SplitSeq(profiles, "\n") {
		if strings.HasPrefix(line, name+" (") {
			return true
		}
	}
	return false
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
// for the netlink probes and SYS_ADMIN for the probes that unshare a network
// namespace, and gives the container its own network so nothing it creates
// reaches the host's. On a daemon applying AppArmor it runs under Ze's profile,
// because docker-default denies the MPLS probe's mount and sysctl write (D-7).
func kernelProbeArgv(zePath string, appArmor bool) []string {
	argv := make([]string, 0, 14+len(kernelProbeCommand))
	argv = append(argv, dockerExecutable, "run", "--rm",
		"--cap-add", "NET_ADMIN", "--cap-add", "SYS_ADMIN",
		"--network", "none")
	if appArmor {
		argv = append(argv, "--security-opt", "apparmor="+kernelcap.ProbeAppArmorProfileName)
	}
	argv = append(argv, "-v", zePath+":"+kernelProbeMount+":ro", kernelProbeImage)
	return append(argv, kernelProbeCommand...)
}

// dockerAppArmor asks whether the daemon applies AppArmor to its containers. A
// question it cannot answer is an error: the probe's argv depends on it.
func dockerAppArmor(ctx context.Context, docker *Docker) (bool, error) {
	argv := []string{dockerExecutable, "info", "--format", dockerSecurityOptionsFormat}
	result, err := docker.runner.Run(ctx, processCommand{Arguments: argv, Timeout: dockerInfoTimeout})
	var problem textbuf.Buffer
	problem.Str("the Docker kernel check could not read the daemon's security options: `").
		Str(strings.Join(argv, " ")).Str("` ")
	if err != nil {
		return false, errors.New(problem.Str("failed: ").Err(err).String())
	}
	if result.ExitCode != 0 {
		return false, errors.New(problem.Str("exit ").Str(strconv.Itoa(result.ExitCode)).
			Str(": ").Str(strings.TrimSpace(result.Stderr)).String())
	}
	var options []string
	if jsonErr := json.Unmarshal([]byte(result.Stdout), &options); jsonErr != nil {
		return false, errors.New(problem.Str("answered unreadable JSON (").Err(jsonErr).Str("): ").
			Str(strings.TrimSpace(result.Stdout)).String())
	}
	for _, option := range options {
		if option == "name=apparmor" || strings.HasPrefix(option, "name=apparmor,") {
			return true, nil
		}
	}
	return false, nil
}

// appArmorProfileMissing answers the refusal for a Linux host whose kernel
// lists its loaded profiles and lacks Ze's, or nil. An unreadable list is no
// answer either way (apparmorfs refuses it to a reader without policy-view
// privilege, such as a non-root user), so it is never read as "loaded": the
// probe runs, and a daemon that cannot apply the profile refuses
// through appArmorProfileUnapplied instead.
func appArmorProfileMissing(release string) error {
	profiles, readable := appArmorProfiles()
	if !readable {
		return nil
	}
	if appArmorProfileLoaded(profiles, kernelcap.ProbeAppArmorProfileName) {
		return nil
	}
	return appArmorRefusal(release, appArmorProfilesPath+" does not list")
}

// runcAppArmorUnapplied is the phrase runc writes when the daemon applies
// AppArmor and cannot put the container under the profile it was asked for,
// which is what a profile the daemon's kernel has not loaded produces
// ("... unable to apply apparmor profile: ... attr/apparmor/exec: no such
// file or directory", Ubuntu 6.8, 2026-10-10).
const runcAppArmorUnapplied = "unable to apply apparmor profile"

// appArmorProfileUnapplied reports whether a probe that exited exit with
// stderr was refused by the daemon because it could not apply Ze's profile.
func appArmorProfileUnapplied(exit int, stderr string) bool {
	if exit == 0 {
		return false
	}
	return strings.Contains(strings.ToLower(stderr), runcAppArmorUnapplied)
}

// appArmorRefusal names the profile, what showed it is not in force, the
// command that loads it, and the kernel route. Loading the profile lets the
// probe answer and changes no kernel feature, so the operator reads the whole
// path at once rather than one step per run.
func appArmorRefusal(release, finding string) error {
	var refusal textbuf.Buffer
	return errors.New(refusal.Str("the Docker daemon applies AppArmor, and the kernel probe runs under Ze's profile ").
		Str(kernelcap.ProbeAppArmorProfileName).Str(", which ").Str(finding).
		Str(". Docker's docker-default profile denies the probe's mount and its /proc/sys write, so load Ze's: ").
		Str(AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)).
		Str("\nThe profile lets the probe answer and adds no kernel feature: every feature the daemon's kernel (").
		Str(release).Str(") lacks, MOBIKE's xfrm migrate for example, is then named, and the route to a kernel carrying them all is: ").
		Str(DockerKernelRoute(runtime.GOOS)).String())
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
	// The probe bind-mounts zePath, and Docker answers a missing source by
	// creating a root-owned directory there, which a later staging then cannot
	// replace. So the file is checked before any docker command runs.
	if missing := stagedZeMissing(zePath); missing != nil {
		return missing
	}

	appArmor, err := dockerAppArmor(ctx, docker)
	if err != nil {
		return err
	}
	release := dockerKernelRelease(ctx, docker)
	if appArmor {
		if missing := appArmorProfileMissing(release); missing != nil {
			return missing
		}
	}
	argv := kernelProbeArgv(zePath, appArmor)
	result, err := docker.runner.Run(ctx, processCommand{Arguments: argv, Timeout: kernelProbeTimeout})

	var problem textbuf.Buffer
	problem.Str("the Docker daemon's kernel (").Str(release).Str(") could not be checked: `").
		Str(strings.Join(argv, " ")).Str("` ")
	if err != nil {
		return errors.New(problem.Str("failed: ").Err(err).String())
	}
	if appArmor && appArmorProfileUnapplied(result.ExitCode, result.Stderr) {
		return appArmorRefusal(release, "the daemon could not apply ("+strings.TrimSpace(result.Stderr)+")")
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
	denied := false
	for _, row := range answer.Capabilities {
		if row.State == "present" {
			continue
		}
		if row.State == "denied" {
			denied = true
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
	// A denied row is the host's security policy, not the kernel: the fix is
	// Ze's profile, loaded or reloaded by one command (D-7).
	if denied {
		refusal.Str("\nA denied row is the host's security policy refusing the probe, not the kernel: load or reload Ze's AppArmor profile with ").
			Str(AppArmorLoadCommandFor(kernelcap.ProbeAppArmorProfileName)).Str(".")
	}
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

// stagedZeMissing answers the refusal for a staged ze path that holds no
// regular file, or nil. A run that skipped its build (NO_BUILD=1) on a checkout
// that never staged one is the usual cause, so the fix is that build.
func stagedZeMissing(zePath string) error {
	info, err := os.Stat(zePath)
	var problem textbuf.Buffer
	problem.Str("the Docker kernel check cannot run: the linux ze it probes with is not at ").Str(zePath)
	if err != nil {
		return errors.New(problem.Str(" (").Err(err).
			Str(").\nThe check mounts that file into the probe container, so without it nothing is checked.").
			Str("\nBuild it: run the same command again without NO_BUILD=1.").String())
	}
	if !info.Mode().IsRegular() {
		return errors.New(problem.Str(": that path is a directory, not the binary").
			Str(" (Docker creates one when it mounts a path that does not exist).").
			Str("\nThe check mounts that file into the probe container, so without it nothing is checked.").
			Str("\nRemove the directory, then run the same command again without NO_BUILD=1:\n").
			Str("  sudo rm -r ").Str(zePath).String())
	}
	return nil
}
